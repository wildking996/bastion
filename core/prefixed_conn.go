package core

import (
	"net"
	"time"
)

type prefixedConn struct {
	net.Conn
	prefix []byte
	off    int
}

func newPrefixedConn(conn net.Conn, prefix []byte) net.Conn {
	if len(prefix) == 0 {
		return conn
	}
	return &prefixedConn{Conn: conn, prefix: prefix}
}

func (c *prefixedConn) Read(p []byte) (int, error) {
	if c.off < len(c.prefix) {
		n := copy(p, c.prefix[c.off:])
		c.off += n
		return n, nil
	}
	return c.Conn.Read(p)
}

func (c *prefixedConn) SetDeadline(t time.Time) error {
	return c.Conn.SetDeadline(t)
}

func (c *prefixedConn) SetReadDeadline(t time.Time) error {
	return c.Conn.SetReadDeadline(t)
}

func (c *prefixedConn) SetWriteDeadline(t time.Time) error {
	return c.Conn.SetWriteDeadline(t)
}
