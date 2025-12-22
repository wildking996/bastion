package core

import (
	"net"
	"time"
)

// DeadlineConn wraps a net.Conn and applies per-operation read/write deadlines.
// Each Read/Write refreshes the corresponding deadline to now+timeout when timeout > 0.
type DeadlineConn struct {
	conn         net.Conn
	readTimeout  time.Duration
	writeTimeout time.Duration
}

// NewDeadlineConn wraps conn and applies read/write deadlines on each Read/Write.
func NewDeadlineConn(conn net.Conn, readTimeout, writeTimeout time.Duration) *DeadlineConn {
	return &DeadlineConn{
		conn:         conn,
		readTimeout:  readTimeout,
		writeTimeout: writeTimeout,
	}
}

// SetTimeouts updates the per-operation read/write timeouts.
func (c *DeadlineConn) SetTimeouts(readTimeout, writeTimeout time.Duration) {
	c.readTimeout = readTimeout
	c.writeTimeout = writeTimeout
}

func (c *DeadlineConn) Read(p []byte) (int, error) {
	if c.readTimeout > 0 {
		_ = c.conn.SetReadDeadline(time.Now().Add(c.readTimeout))
	} else {
		_ = c.conn.SetReadDeadline(time.Time{})
	}
	return c.conn.Read(p)
}

func (c *DeadlineConn) Write(p []byte) (int, error) {
	if c.writeTimeout > 0 {
		_ = c.conn.SetWriteDeadline(time.Now().Add(c.writeTimeout))
	} else {
		_ = c.conn.SetWriteDeadline(time.Time{})
	}
	return c.conn.Write(p)
}

func (c *DeadlineConn) Close() error                       { return c.conn.Close() }
func (c *DeadlineConn) LocalAddr() net.Addr                { return c.conn.LocalAddr() }
func (c *DeadlineConn) RemoteAddr() net.Addr               { return c.conn.RemoteAddr() }
func (c *DeadlineConn) SetDeadline(t time.Time) error      { return c.conn.SetDeadline(t) }
func (c *DeadlineConn) SetReadDeadline(t time.Time) error  { return c.conn.SetReadDeadline(t) }
func (c *DeadlineConn) SetWriteDeadline(t time.Time) error { return c.conn.SetWriteDeadline(t) }

func (c *DeadlineConn) CloseWrite() error {
	if cw, ok := c.conn.(interface{ CloseWrite() error }); ok {
		return cw.CloseWrite()
	}
	return nil
}

func (c *DeadlineConn) CloseRead() error {
	if cr, ok := c.conn.(interface{ CloseRead() error }); ok {
		return cr.CloseRead()
	}
	return nil
}
