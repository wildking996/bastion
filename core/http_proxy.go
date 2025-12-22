package core

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"bastion/config"
	"bastion/models"
)

// HTTPProxySession handles HTTP forward-proxy sessions
type HTTPProxySession struct {
	BaseSession
}

// NewHTTPProxySession creates an HTTP proxy session
func NewHTTPProxySession(mapping *models.Mapping, bastions []models.Bastion) *HTTPProxySession {
	ipACL, _ := NewIPAccessControl(mapping.GetAllowCIDRs(), mapping.GetDenyCIDRs())
	chain := make([]string, 0, len(bastions))
	for _, b := range bastions {
		chain = append(chain, b.Name)
	}
	return &HTTPProxySession{
		BaseSession: BaseSession{
			Mapping:        mapping,
			Bastions:       bastions,
			stopChan:       make(chan struct{}),
			maxConnections: int32(config.Settings.MaxSessionConnections),
			httpParsers:    make(map[string]*HTTPStreamParser),
			ipACL:          ipACL,
			auditCtx: AuditContext{
				MappingID:    mapping.ID,
				LocalPort:    mapping.LocalPort,
				BastionChain: chain,
			},
		},
	}
}

// Start starts the HTTP proxy session
func (s *HTTPProxySession) Start() error {
	addr := fmt.Sprintf("%s:%d", s.Mapping.LocalHost, s.Mapping.LocalPort)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return NewResourceBusyError(fmt.Sprintf("Port %d is already in use", s.Mapping.LocalPort))
	}

	s.listener = listener
	log.Printf("HTTP Proxy started: %s", addr)

	s.wg.Add(1)
	go s.acceptLoop()

	return nil
}

// acceptLoop accepts incoming HTTP proxy connections
func (s *HTTPProxySession) acceptLoop() {
	defer s.wg.Done()

	for {
		select {
		case <-s.stopChan:
			return
		default:
		}

		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.stopChan:
				return
			default:
				log.Printf("Accept error: %v", err)
				continue
			}
		}

		if !s.shouldAcceptClient(conn) {
			if config.Settings.LogLevel == "DEBUG" {
				log.Printf("[HTTP] Rejected client %s by IP ACL", conn.RemoteAddr().String())
			}
			conn.Close()
			continue
		}

		// Enforce connection limit
		if atomic.LoadInt32(&s.activeConns) >= s.maxConnections {
			log.Printf("Connection limit reached (%d), rejecting new connection", s.maxConnections)
			conn.Close()
			continue
		}

		s.wg.Add(1)
		go s.handleHTTPClientWithRecover(conn)
	}
}

// handleHTTPClientWithRecover processes a client with panic recovery
func (s *HTTPProxySession) handleHTTPClientWithRecover(conn net.Conn) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Recovered from panic in handleHTTPClient: %v", r)
		}
	}()
	s.handleHTTPClient(conn)
}

// handleHTTPClient handles an HTTP proxy client connection
func (s *HTTPProxySession) handleHTTPClient(clientConn net.Conn) {
	defer s.wg.Done()
	defer clientConn.Close()

	atomic.AddInt32(&s.activeConns, 1)
	defer atomic.AddInt32(&s.activeConns, -1)

	clientAddr := clientConn.RemoteAddr().String()
	localAddr := clientConn.LocalAddr().String()

	// Handshake timeout (shared with SOCKS5 settings)
	if err := clientConn.SetDeadline(time.Now().Add(time.Duration(config.Settings.Socks5HandshakeTimeoutSeconds) * time.Second)); err != nil {
		log.Printf("[HTTP] Failed to set handshake deadline for client %s: %v", clientAddr, err)
	}

	reader := bufio.NewReader(clientConn)
	req, err := http.ReadRequest(reader)
	if err != nil {
		log.Printf("[HTTP] Failed to parse request from %s: %v", clientAddr, err)
		sendSimpleHTTPError(clientConn, http.StatusBadRequest, "Bad Request")
		return
	}

	targetHost, targetPort, err := parseProxyTarget(req)
	if err != nil {
		log.Printf("[HTTP] Invalid target from %s: %v", clientAddr, err)
		sendSimpleHTTPError(clientConn, http.StatusBadRequest, "Bad Request")
		return
	}

	remoteAddr := net.JoinHostPort(targetHost, strconv.Itoa(targetPort))
	connID := fmt.Sprintf("%s->%s", clientAddr, remoteAddr)

	if config.Settings.LogLevel == "DEBUG" {
		log.Printf("[HTTP] New connection: client=%s, local=%s, method=%s, target=%s",
			clientAddr, localAddr, req.Method, remoteAddr)
	}

	var remoteConn net.Conn

	// Connect via bastion chain or directly
	if len(s.Bastions) == 0 {
		remoteConn, err = net.DialTimeout("tcp", remoteAddr, 10*time.Second)
	} else {
		bastionChain := getBastionChainNames(s.Bastions)
		remoteConn, err = s.dialWithRetry(remoteAddr, clientAddr, bastionChain)
	}
	if err != nil {
		log.Printf("[HTTP] Failed to dial remote %s from client %s: %v", remoteAddr, clientAddr, err)
		sendSimpleHTTPError(clientConn, http.StatusBadGateway, "Bad Gateway")
		return
	}
	defer remoteConn.Close()

	// Handshake complete; switch to session timeout
	if err := clientConn.SetDeadline(time.Now().Add(time.Duration(config.Settings.SessionIdleTimeoutHours) * time.Hour)); err != nil {
		log.Printf("[HTTP] Failed to set session deadline for client %s: %v", clientAddr, err)
	}
	if err := remoteConn.SetDeadline(time.Now().Add(time.Duration(config.Settings.SessionIdleTimeoutHours) * time.Hour)); err != nil {
		log.Printf("[HTTP] Failed to set session deadline for remote %s: %v", remoteAddr, err)
	}

	if strings.EqualFold(req.Method, http.MethodConnect) {
		if _, err := clientConn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n")); err != nil {
			return
		}
		s.pipe(clientConn, remoteConn, connID)
		return
	}

	// Standard HTTP request: forward once and copy response
	if req.Host == "" {
		req.Host = net.JoinHostPort(targetHost, strconv.Itoa(targetPort))
	}
	req.URL.Scheme = ""
	req.URL.Host = ""
	req.RequestURI = ""
	req.Close = true

	isWebSocket := isWebSocketUpgradeRequest(req)
	if isWebSocket {
		// WebSocket upgrade requires a long-lived full-duplex connection.
		req.Close = false
	}

	trackingWriter := &proxyTrackingWriter{
		dst:       remoteConn,
		session:   &s.BaseSession,
		direction: "request",
		connID:    connID,
	}

	if err := req.Write(trackingWriter); err != nil {
		log.Printf("[HTTP] Failed to write request to %s: %v", remoteAddr, err)
		return
	}

	if isWebSocket {
		remoteReader := bufio.NewReader(remoteConn)
		resp, err := http.ReadResponse(remoteReader, req)
		if err != nil {
			log.Printf("[HTTP] Failed to read response from %s: %v", remoteAddr, err)
			return
		}

		respWriter := &proxyTrackingWriter{
			dst:       clientConn,
			session:   &s.BaseSession,
			direction: "response",
			connID:    connID,
		}

		// For WebSocket, we forward the 101 response first, then switch to raw TCP proxying
		// (no HTTP audit parsing for subsequent WS frames).
		if err := resp.Write(respWriter); err != nil {
			log.Printf("[HTTP] Failed to write response to client %s: %v", clientAddr, err)
			_ = resp.Body.Close()
			return
		}

		// Do not close resp.Body for 101 Switching Protocols; it may wrap the underlying connection.
		if resp.StatusCode != http.StatusSwitchingProtocols {
			_ = resp.Body.Close()
			return
		}

		s.pipeRaw(clientConn, reader, remoteConn, remoteReader, connID)
		return
	}

	// Copy response while updating stats and audit logs
	s.copyData(clientConn, remoteConn, "response", connID)
}

func isWebSocketUpgradeRequest(req *http.Request) bool {
	upgrade := strings.ToLower(req.Header.Get("Upgrade"))
	if upgrade != "websocket" {
		return false
	}
	conn := strings.ToLower(req.Header.Get("Connection"))
	return strings.Contains(conn, "upgrade")
}

// pipeRaw proxies bidirectional bytes without feeding the HTTP audit parser.
// clientReader is the bufio.Reader used for parsing the initial request, which may contain buffered bytes.
func (s *HTTPProxySession) pipeRaw(clientConn net.Conn, clientReader *bufio.Reader, remoteConn net.Conn, remoteReader *bufio.Reader, connID string) {
	var wg sync.WaitGroup
	wg.Add(2)

	once := &sync.Once{}
	closeConns := func() {
		_ = clientConn.Close()
		_ = remoteConn.Close()
	}

	go func() {
		defer wg.Done()
		defer once.Do(closeConns)
		s.copyRaw(remoteConn, clientReader, "request", connID)
	}()

	go func() {
		defer wg.Done()
		defer once.Do(closeConns)
		s.copyRaw(clientConn, remoteReader, "response", connID)
	}()

	wg.Wait()
}

func (s *HTTPProxySession) copyRaw(dst net.Conn, src io.Reader, direction, connID string) {
	pool := getForwardBufferPool()
	bufPtr := pool.Get(pool.InitialSize())
	buf := *bufPtr
	defer pool.Put(bufPtr)

	defer func() {
		if r := recover(); r != nil {
			log.Printf("Recovered from panic in copyRaw [%s]: %v", direction, r)
		}
		if tcpConn, ok := dst.(*net.TCPConn); ok {
			_ = tcpConn.CloseWrite()
		}
	}()

	for {
		n, err := src.Read(buf)
		if n > 0 {
			if direction == "request" {
				atomic.AddInt64(&s.bytesUp, int64(n))
			} else {
				atomic.AddInt64(&s.bytesDown, int64(n))
			}

			written := 0
			for written < n {
				w, werr := dst.Write(buf[written:n])
				if werr != nil {
					return
				}
				written += w
			}

			if n == len(buf) {
				if next, ok := pool.NextSize(len(buf)); ok && next > len(buf) {
					pool.Put(bufPtr)
					bufPtr = pool.Get(next)
					buf = *bufPtr
				}
			}
		}
		if err != nil {
			if err != io.EOF && config.Settings.LogLevel == "DEBUG" {
				log.Printf("Copy raw error [%s] (%s): %v", direction, connID, err)
			}
			return
		}
	}
}

// proxyTrackingWriter updates stats and audit data while writing
type proxyTrackingWriter struct {
	dst       io.Writer
	session   *BaseSession
	direction string
	connID    string
}

func (w *proxyTrackingWriter) Write(p []byte) (int, error) {
	if w.session != nil {
		if w.direction == "request" {
			atomic.AddInt64(&w.session.bytesUp, int64(len(p)))
		} else {
			atomic.AddInt64(&w.session.bytesDown, int64(len(p)))
		}
		if config.Settings.AuditEnabled {
			w.session.feedHTTPParser(p, w.direction, w.connID)
		}
	}
	return w.dst.Write(p)
}

// parseProxyTarget parses the proxy target from the request
func parseProxyTarget(req *http.Request) (string, int, error) {
	hostPort := req.Host
	if hostPort == "" && req.URL != nil {
		hostPort = req.URL.Host
	}
	if hostPort == "" {
		return "", 0, fmt.Errorf("missing Host")
	}

	host, portStr, err := net.SplitHostPort(hostPort)
	if err != nil {
		// Add default port if missing
		if strings.Contains(err.Error(), "missing port in address") {
			host = hostPort
			if strings.EqualFold(req.Method, http.MethodConnect) || req.URL.Scheme == "https" {
				return host, 443, nil
			}
			return host, 80, nil
		}
		return "", 0, err
	}

	port, err := strconv.Atoi(portStr)
	if err != nil {
		return "", 0, fmt.Errorf("invalid port: %w", err)
	}

	return host, port, nil
}

// sendSimpleHTTPError writes a simple HTTP error response
func sendSimpleHTTPError(conn net.Conn, status int, message string) {
	body := message + "\r\n"
	resp := fmt.Sprintf("HTTP/1.1 %d %s\r\nContent-Type: text/plain\r\nContent-Length: %d\r\nConnection: close\r\n\r\n%s",
		status, http.StatusText(status), len(body), body)
	_, _ = conn.Write([]byte(resp))
}
