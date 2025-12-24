package core

import (
	"bytes"
	"errors"
	"log"
	"net"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"bastion/config"
	"bastion/models"
)

type MixedProxySession struct {
	BaseSession
}

func NewMixedProxySession(mapping *models.Mapping, bastions []models.Bastion) *MixedProxySession {
	ipACL, _ := NewIPAccessControl(mapping.GetAllowCIDRs(), mapping.GetDenyCIDRs())
	chain := make([]string, 0, len(bastions))
	for _, b := range bastions {
		chain = append(chain, b.Name)
	}
	return &MixedProxySession{
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

func (s *MixedProxySession) Start() error {
	addr := net.JoinHostPort(s.Mapping.LocalHost, strconv.Itoa(s.Mapping.LocalPort))
	listener, err := listenTCPWithDiagnostics(s.Mapping)
	if err != nil {
		return err
	}

	s.listener = listener
	log.Printf("MIXED Proxy started: %s", addr)

	s.wg.Add(1)
	go s.acceptLoop()
	return nil
}

func (s *MixedProxySession) acceptLoop() {
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
				log.Printf("[MIXED] Rejected client %s by IP ACL", conn.RemoteAddr().String())
			}
			_ = conn.Close()
			continue
		}

		if atomic.LoadInt32(&s.activeConns) >= s.maxConnections {
			log.Printf("Connection limit reached (%d), rejecting new connection", s.maxConnections)
			_ = conn.Close()
			continue
		}

		s.wg.Add(1)
		go s.handleMixedClientWithRecover(conn)
	}
}

func (s *MixedProxySession) handleMixedClientWithRecover(conn net.Conn) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Recovered from panic in handleMixedClient: %v", r)
		}
	}()
	s.handleMixedClient(conn)
}

func (s *MixedProxySession) handleMixedClient(conn net.Conn) {
	proto, wrapped, err := detectMixedProtocol(conn)
	if err != nil {
		if config.Settings.LogLevel == "DEBUG" {
			log.Printf("[MIXED] Protocol detect failed from %s: %v", conn.RemoteAddr().String(), err)
		}
		_ = conn.Close()
		s.wg.Done()
		return
	}

	if config.Settings.LogLevel == "DEBUG" {
		log.Printf("[MIXED] Detected protocol=%s from %s", proto, wrapped.RemoteAddr().String())
	}

	switch proto {
	case "socks5":
		s.handleSocks5Client(wrapped)
	case "http":
		s.handleHTTPClient(wrapped)
	default:
		_ = wrapped.Close()
		s.wg.Done()
	}
}

var errUnknownProtocol = errors.New("unknown protocol")

func detectMixedProtocol(conn net.Conn) (string, net.Conn, error) {
	const maxPeek = 32
	const timeout = 2 * time.Second

	deadline := time.Now().Add(timeout)
	if err := conn.SetReadDeadline(deadline); err != nil {
		return "", nil, err
	}
	defer func() {
		_ = conn.SetReadDeadline(time.Time{})
	}()

	buf := make([]byte, 0, maxPeek)
	tmp := make([]byte, maxPeek)

	for len(buf) < maxPeek {
		n, err := conn.Read(tmp[:cap(buf)-len(buf)])
		if n > 0 {
			buf = append(buf, tmp[:n]...)
			if proto := classifyMixedProtocol(buf); proto != "" {
				return proto, newPrefixedConn(conn, buf), nil
			}
		}
		if err != nil {
			return "", nil, err
		}
	}

	if proto := classifyMixedProtocol(buf); proto != "" {
		return proto, newPrefixedConn(conn, buf), nil
	}
	return "", nil, errUnknownProtocol
}

func classifyMixedProtocol(prefix []byte) string {
	p := bytes.TrimLeft(prefix, " \t\r\n")
	if len(p) == 0 {
		return ""
	}
	if p[0] == 0x05 {
		return "socks5"
	}
	if looksLikeHTTP(p) {
		return "http"
	}
	return ""
}

var httpMethodTokens = map[string]struct{}{
	"GET":     {},
	"POST":    {},
	"PUT":     {},
	"DELETE":  {},
	"HEAD":    {},
	"OPTIONS": {},
	"PATCH":   {},
	"CONNECT": {},
	"TRACE":   {},
}

func looksLikeHTTP(p []byte) bool {
	if len(p) < 3 {
		return false
	}

	// Read token until space.
	i := 0
	for i < len(p) && ((p[i] >= 'A' && p[i] <= 'Z') || (p[i] >= 'a' && p[i] <= 'z')) {
		i++
	}
	if i == 0 {
		return bytes.HasPrefix(p, []byte("HTTP/"))
	}

	tok := strings.ToUpper(string(p[:i]))
	_, ok := httpMethodTokens[tok]
	return ok
}
