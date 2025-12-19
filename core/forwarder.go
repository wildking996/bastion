package core

import (
	"bastion/config"
	"bastion/models"
	"fmt"
	"io"
	"log"
	"net"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// bufferPool reuses buffers to reduce GC pressure
var bufferPool = sync.Pool{
	New: func() interface{} {
		buf := make([]byte, config.Settings.ForwardBufferSize)
		return &buf
	},
}

// Session represents a generic session
type Session interface {
	Start() error
	Stop()
	GetStats() SessionStats
}

// SessionStats holds session metrics
type SessionStats struct {
	BytesUp     int64
	BytesDown   int64
	ActiveConns int32
}

// BaseSession shared state for sessions
type BaseSession struct {
	Mapping        *models.Mapping
	Bastions       []models.Bastion
	listener       net.Listener
	activeConns    int32
	bytesUp        int64
	bytesDown      int64
	stopChan       chan struct{}
	wg             sync.WaitGroup
	maxConnections int32                        // Concurrency limit
	httpParsers    map[string]*HTTPStreamParser // connID:direction -> parser
	parserMu       sync.Mutex
	ipACL          *IPAccessControl
	auditCtx       AuditContext
}

func (s *BaseSession) shouldAcceptClient(conn net.Conn) bool {
	if s.ipACL == nil {
		return true
	}
	if conn == nil {
		return false
	}
	if tcpAddr, ok := conn.RemoteAddr().(*net.TCPAddr); ok {
		return s.ipACL.Allows(tcpAddr.IP)
	}

	remote := conn.RemoteAddr().String()
	host, _, err := net.SplitHostPort(remote)
	if err != nil {
		return s.ipACL.Allows(net.ParseIP(remote))
	}
	return s.ipACL.Allows(net.ParseIP(host))
}

// TunnelSession TCP tunnel session
type TunnelSession struct {
	BaseSession
}

// Socks5Session SOCKS5 proxy session
type Socks5Session struct {
	BaseSession
}

// NewTunnelSession creates a TCP tunnel session
func NewTunnelSession(mapping *models.Mapping, bastions []models.Bastion) *TunnelSession {
	ipACL, _ := NewIPAccessControl(mapping.GetAllowCIDRs(), mapping.GetDenyCIDRs())
	chain := make([]string, 0, len(bastions))
	for _, b := range bastions {
		chain = append(chain, b.Name)
	}
	return &TunnelSession{
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

// NewSocks5Session creates a SOCKS5 session
func NewSocks5Session(mapping *models.Mapping, bastions []models.Bastion) *Socks5Session {
	ipACL, _ := NewIPAccessControl(mapping.GetAllowCIDRs(), mapping.GetDenyCIDRs())
	chain := make([]string, 0, len(bastions))
	for _, b := range bastions {
		chain = append(chain, b.Name)
	}
	return &Socks5Session{
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

// Start launches the TCP tunnel session
func (s *TunnelSession) Start() error {
	addr := fmt.Sprintf("%s:%d", s.Mapping.LocalHost, s.Mapping.LocalPort)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return NewResourceBusyError(fmt.Sprintf("Port %d is already in use", s.Mapping.LocalPort))
	}

	s.listener = listener
	log.Printf("TCP Tunnel started: %s -> %s:%d", addr, s.Mapping.RemoteHost, s.Mapping.RemotePort)

	s.wg.Add(1)
	go s.acceptLoop()

	return nil
}

// Start launches the SOCKS5 session
func (s *Socks5Session) Start() error {
	addr := fmt.Sprintf("%s:%d", s.Mapping.LocalHost, s.Mapping.LocalPort)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return NewResourceBusyError(fmt.Sprintf("Port %d is already in use", s.Mapping.LocalPort))
	}

	s.listener = listener
	log.Printf("SOCKS5 Proxy started: %s", addr)

	s.wg.Add(1)
	go s.acceptLoop()

	return nil
}

// acceptLoop accepts TCP tunnel connections
func (s *TunnelSession) acceptLoop() {
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
				log.Printf("[TCP] Rejected client %s by IP ACL", conn.RemoteAddr().String())
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
		go s.handleTCPClientWithRecover(conn)
	}
}

// handleTCPClientWithRecover wraps TCP handling with panic recovery
func (s *TunnelSession) handleTCPClientWithRecover(conn net.Conn) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Recovered from panic in handleTCPClient: %v", r)
		}
	}()
	s.handleTCPClient(conn)
}

// acceptLoop accepts SOCKS5 connections
func (s *Socks5Session) acceptLoop() {
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
				log.Printf("[SOCKS5] Rejected client %s by IP ACL", conn.RemoteAddr().String())
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
		go s.handleSocks5ClientWithRecover(conn)
	}
}

// handleSocks5ClientWithRecover wraps SOCKS5 handling with panic recovery
func (s *Socks5Session) handleSocks5ClientWithRecover(conn net.Conn) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Recovered from panic in handleSocks5Client: %v", r)
		}
	}()
	s.handleSocks5Client(conn)
}

// handleTCPClient processes a TCP client connection
func (s *TunnelSession) handleTCPClient(clientConn net.Conn) {
	defer s.wg.Done()
	defer clientConn.Close()

	atomic.AddInt32(&s.activeConns, 1)
	defer atomic.AddInt32(&s.activeConns, -1)

	clientAddr := clientConn.RemoteAddr().String()
	localAddr := clientConn.LocalAddr().String()
	remoteTarget := net.JoinHostPort(s.Mapping.RemoteHost, strconv.Itoa(s.Mapping.RemotePort))
	connID := fmt.Sprintf("%s->%s", clientAddr, remoteTarget)

	// Set idle timeout to avoid resource hogging
	if err := clientConn.SetDeadline(time.Now().Add(time.Duration(config.Settings.SessionIdleTimeoutHours) * time.Hour)); err != nil {
		log.Printf("[TCP] Failed to set deadline for client: %v", err)
	}

	// Detailed logging: record connection source
	if config.Settings.LogLevel == "DEBUG" {
		log.Printf("[TCP] New connection: client=%s, local=%s, target=%s, mapping_id=%s",
			clientAddr, localAddr, remoteTarget, s.Mapping.ID)
	}

	remoteAddr := remoteTarget
	var remoteConn net.Conn
	var err error

	// Check for bastion chain
	if len(s.Bastions) == 0 {
		// Direct connection (no bastions)
		remoteConn, err = net.DialTimeout("tcp", remoteAddr, 10*time.Second)
		if err != nil {
			log.Printf("[TCP] Failed to dial remote %s directly from client %s: %v", remoteAddr, clientAddr, err)
			return
		}
	} else {
		// Connect via bastion chain with retry
		bastionChain := getBastionChainNames(s.Bastions)
		remoteConn, err = s.dialWithRetry(remoteAddr, clientAddr, bastionChain)
		if err != nil {
			log.Printf("[TCP] Failed to dial remote %s via bastion chain [%s] from client %s: %v",
				remoteAddr, bastionChain, clientAddr, err)
			return
		}
	}
	defer remoteConn.Close()

	// Set remote timeout
	if err := remoteConn.SetDeadline(time.Now().Add(time.Duration(config.Settings.SessionIdleTimeoutHours) * time.Hour)); err != nil {
		log.Printf("Failed to set remote deadline: %v", err)
	}

	// Bidirectional forwarding
	s.pipe(clientConn, remoteConn, connID)
}

// handleSocks5Client processes a SOCKS5 client connection
func (s *Socks5Session) handleSocks5Client(clientConn net.Conn) {
	defer s.wg.Done()
	defer clientConn.Close()

	atomic.AddInt32(&s.activeConns, 1)
	defer atomic.AddInt32(&s.activeConns, -1)

	clientAddr := clientConn.RemoteAddr().String()
	localAddr := clientConn.LocalAddr().String()

	// Set handshake timeout
	if err := clientConn.SetDeadline(time.Now().Add(time.Duration(config.Settings.Socks5HandshakeTimeoutSeconds) * time.Second)); err != nil {
		log.Printf("[SOCKS5] Failed to set handshake deadline: %v", err)
		return
	}

	// SOCKS5 handshake
	handshake := &Socks5Handshake{}
	targetHost, targetPort, err := handshake.Handshake(clientConn)
	if err != nil {
		log.Printf("[SOCKS5] Handshake failed from client %s: %v", clientAddr, err)
		if err := handshake.SendReply(clientConn, false); err != nil {
			log.Printf("[SOCKS5] Failed to send failure reply: %v", err)
		}
		return
	}

	remoteTarget := net.JoinHostPort(targetHost, strconv.Itoa(targetPort))
	connID := fmt.Sprintf("%s->%s", clientAddr, remoteTarget)

	// Detailed logging: record source and destination
	if config.Settings.LogLevel == "DEBUG" {
		log.Printf("[SOCKS5] New connection: client=%s, local=%s, target=%s",
			clientAddr, localAddr, remoteTarget)
	}

	// Reset timeout for long-lived session
	if err := clientConn.SetDeadline(time.Now().Add(time.Duration(config.Settings.SessionIdleTimeoutHours) * time.Hour)); err != nil {
		log.Printf("[SOCKS5] Failed to reset deadline: %v", err)
		return
	}

	remoteAddr := remoteTarget
	var remoteConn net.Conn

	// Check for bastion chain
	if len(s.Bastions) == 0 {
		// Direct connection (no bastions)
		remoteConn, err = net.DialTimeout("tcp", remoteAddr, 10*time.Second)
		if err != nil {
			log.Printf("[SOCKS5] Failed to dial remote %s directly from client %s: %v", remoteAddr, clientAddr, err)
			if err := handshake.SendReply(clientConn, false); err != nil {
				log.Printf("[SOCKS5] Failed to send failure reply: %v", err)
			}
			return
		}
	} else {
		// Connect via bastion chain with retry
		bastionChain := getBastionChainNames(s.Bastions)
		remoteConn, err = s.dialWithRetry(remoteAddr, clientAddr, bastionChain)
		if err != nil {
			log.Printf("[SOCKS5] Failed to dial remote %s via bastion chain [%s] from client %s: %v",
				remoteAddr, bastionChain, clientAddr, err)
			if err := handshake.SendReply(clientConn, false); err != nil {
				log.Printf("[SOCKS5] Failed to send failure reply: %v", err)
			}
			return
		}
	}
	defer remoteConn.Close()

	// Set remote timeout
	if err := remoteConn.SetDeadline(time.Now().Add(time.Duration(config.Settings.SessionIdleTimeoutHours) * time.Hour)); err != nil {
		log.Printf("Failed to set remote deadline: %v", err)
	}

	// Send success reply
	if err := handshake.SendReply(clientConn, true); err != nil {
		log.Printf("Failed to send SOCKS5 reply: %v", err)
		return
	}

	// Bidirectional forwarding
	s.pipe(clientConn, remoteConn, connID)
}

// pipe handles bidirectional data forwarding
func (s *BaseSession) pipe(client, remote net.Conn, connID string) {
	var wg sync.WaitGroup
	wg.Add(2)

	// Use a sync.Once to ensure connections are closed exactly once
	once := sync.Once{}
	closeConns := func() {
		client.Close()
		remote.Close()
	}

	// Client -> Remote (Request)
	go func() {
		defer wg.Done()
		defer once.Do(closeConns) // Ensure connections are closed when this goroutine exits
		s.copyData(remote, client, "request", connID)
	}()

	// Remote -> Client (Response)
	go func() {
		defer wg.Done()
		defer once.Do(closeConns) // Ensure connections are closed when this goroutine exits
		s.copyData(client, remote, "response", connID)
	}()

	wg.Wait() // Wait for both copyData goroutines to finish

	// Connection closed, flush any remaining HTTP data
	if config.Settings.AuditEnabled {
		s.flushHTTPParser("request", connID)
		s.flushHTTPParser("response", connID)
	}
}

// copyData copies data between connections
func (s *BaseSession) copyData(dst, src net.Conn, direction, connID string) {
	// Get buffer from pool
	bufAny := bufferPool.Get()
	bufPtr, ok := bufAny.(*[]byte)
	if !ok || bufPtr == nil {
		buf := make([]byte, config.Settings.ForwardBufferSize)
		bufPtr = &buf
	}
	buf := *bufPtr
	defer bufferPool.Put(bufPtr)

	// Ensure the write end of the destination connection is closed when this goroutine exits
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Recovered from panic in copyData [%s]: %v", direction, r)
		}
		if tcpConn, ok := dst.(*net.TCPConn); ok {
			if err := tcpConn.CloseWrite(); err != nil {
				log.Printf("Failed to close write end of TCP connection: %v", err)
			}
		}
	}()

	for {
		n, err := src.Read(buf)
		if n > 0 {
			// Update statistics
			if direction == "request" {
				atomic.AddInt64(&s.bytesUp, int64(n))
			} else {
				atomic.AddInt64(&s.bytesDown, int64(n))
			}

			// HTTP Auditing
			if config.Settings.AuditEnabled {
				s.feedHTTPParser(buf[:n], direction, connID)
			}

			// Write to destination
			written := 0
			for written < n {
				w, writeErr := dst.Write(buf[written:n])
				if writeErr != nil {
					return // Exit on write error
				}
				written += w
			}
		}
		if err != nil {
			if err != io.EOF && config.Settings.LogLevel == "DEBUG" {
				log.Printf("Copy error [%s]: %v", direction, err)
			}
			return // Exit on read error or EOF
		}
	}
}

// feedHTTPParser feeds data into the HTTP parser
func (s *BaseSession) feedHTTPParser(data []byte, direction, connID string) {
	s.parserMu.Lock()

	parserKey := connID + ":" + direction
	parser, exists := s.httpParsers[parserKey]
	if !exists {
		parser = NewHTTPStreamParser(connID, direction)
		s.httpParsers[parserKey] = parser
	}

	s.parserMu.Unlock()

	// Feed data and fetch complete messages
	messages := parser.Feed(data)

	// Send complete messages to the auditor
	for _, msg := range messages {
		AuditorInstance.EnqueueHTTPMessage(s.auditCtx, connID, msg)
	}
}

// flushHTTPParser flushes the HTTP parser
func (s *BaseSession) flushHTTPParser(direction, connID string) {
	s.parserMu.Lock()

	parserKey := connID + ":" + direction
	parser, exists := s.httpParsers[parserKey]
	if exists {
		delete(s.httpParsers, parserKey)
	}

	s.parserMu.Unlock()

	if parser != nil {
		if msg := parser.Flush(); msg != nil {
			AuditorInstance.EnqueueHTTPMessage(s.auditCtx, connID, msg)
		}
	}
}

// Stop stops the session
func (s *BaseSession) Stop() {
	close(s.stopChan)
	if s.listener != nil {
		s.listener.Close()
	}

	// Wait with timeout to avoid blocking indefinitely
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.Printf("Session stopped for mapping: %s", s.Mapping.ID)
	case <-time.After(5 * time.Second):
		log.Printf("Session stop timeout for mapping: %s (forced)", s.Mapping.ID)
	}
}

// dialWithRetry dials via bastion chain with retries
func (s *BaseSession) dialWithRetry(remoteAddr, clientAddr, bastionChain string) (net.Conn, error) {
	maxRetries := 3
	retryDelay := 1 * time.Second

	var lastErr error
	for attempt := 1; attempt <= maxRetries; attempt++ {
		if attempt > 1 {
			if config.Settings.LogLevel == "DEBUG" {
				log.Printf("[Retry] Attempt %d/%d to dial %s via bastion chain [%s] for client %s",
					attempt, maxRetries, remoteAddr, bastionChain, clientAddr)
			}
			time.Sleep(retryDelay)
		}

		remoteConn, err := Pool.Dial(s.Bastions, "tcp", remoteAddr)
		if err != nil {
			lastErr = fmt.Errorf("dial failed: %w", err)
			continue
		}

		// Connection succeeded
		if attempt > 1 {
			log.Printf("[Retry] Successfully connected to %s on attempt %d/%d", remoteAddr, attempt, maxRetries)
		}
		return remoteConn, nil
	}

	return nil, fmt.Errorf("all %d attempts failed, last error: %w", maxRetries, lastErr)
}

// getBastionChainNames returns bastion chain names for logging
func getBastionChainNames(bastions []models.Bastion) string {
	names := make([]string, len(bastions))
	for i, b := range bastions {
		names[i] = b.Name
	}
	return strings.Join(names, "->")
}

// GetStats returns session statistics
func (s *BaseSession) GetStats() SessionStats {
	return SessionStats{
		BytesUp:     atomic.LoadInt64(&s.bytesUp),
		BytesDown:   atomic.LoadInt64(&s.bytesDown),
		ActiveConns: atomic.LoadInt32(&s.activeConns),
	}
}
