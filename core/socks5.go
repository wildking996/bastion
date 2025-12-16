package core

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
)

const (
	socks5Version = 0x05
	socks5NoAuth  = 0x00
	socks5Connect = 0x01
	socks5IPv4    = 0x01
	socks5Domain  = 0x03
	socks5IPv6    = 0x04
)

// Socks5Handshake handles SOCKS5 handshakes
type Socks5Handshake struct{}

// Handshake performs a SOCKS5 handshake and returns the target address
func (s *Socks5Handshake) Handshake(conn net.Conn) (string, int, error) {
	// Phase 1: negotiate auth method
	header := make([]byte, 2)
	if _, err := io.ReadFull(conn, header); err != nil {
		return "", 0, fmt.Errorf("invalid SOCKS5 header: %w", err)
	}

	version, nmethods := header[0], header[1]
	if version != socks5Version {
		return "", 0, fmt.Errorf("only SOCKS5 supported")
	}

	// Read supported methods
	methods := make([]byte, nmethods)
	if _, err := io.ReadFull(conn, methods); err != nil {
		return "", 0, err
	}

	// Reply: no authentication required
	if _, err := conn.Write([]byte{socks5Version, socks5NoAuth}); err != nil {
		return "", 0, err
	}

	// Phase 2: read request details
	request := make([]byte, 4)
	if _, err := io.ReadFull(conn, request); err != nil {
		return "", 0, fmt.Errorf("invalid request: %w", err)
	}

	version, cmd, _, atyp := request[0], request[1], request[2], request[3]
	if version != socks5Version {
		return "", 0, fmt.Errorf("invalid version")
	}

	if cmd != socks5Connect {
		return "", 0, fmt.Errorf("only TCP CONNECT supported")
	}

	// Parse target address
	var targetHost string
	switch atyp {
	case socks5IPv4:
		addr := make([]byte, 4)
		if _, err := io.ReadFull(conn, addr); err != nil {
			return "", 0, err
		}
		targetHost = net.IP(addr).String()

	case socks5Domain:
		domainLen := make([]byte, 1)
		if _, err := io.ReadFull(conn, domainLen); err != nil {
			return "", 0, err
		}
		domain := make([]byte, domainLen[0])
		if _, err := io.ReadFull(conn, domain); err != nil {
			return "", 0, err
		}
		targetHost = string(domain)

	case socks5IPv6:
		addr := make([]byte, 16)
		if _, err := io.ReadFull(conn, addr); err != nil {
			return "", 0, err
		}
		targetHost = net.IP(addr).String()

	default:
		return "", 0, fmt.Errorf("unknown address type")
	}

	// Read port
	portBytes := make([]byte, 2)
	if _, err := io.ReadFull(conn, portBytes); err != nil {
		return "", 0, err
	}
	targetPort := int(binary.BigEndian.Uint16(portBytes))

	return targetHost, targetPort, nil
}

// SendReply sends the handshake result
func (s *Socks5Handshake) SendReply(conn net.Conn, success bool) error {
	rep := byte(0x00) // succeeded
	if !success {
		rep = 0x01 // general failure
	}

	// [VER, REP, RSV, ATYP, BND.ADDR(0.0.0.0), BND.PORT(0)]
	reply := []byte{
		socks5Version, rep, 0x00, socks5IPv4,
		0, 0, 0, 0, // 0.0.0.0
		0, 0, // port 0
	}

	_, err := conn.Write(reply)
	return err
}
