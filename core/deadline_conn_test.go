package core

import (
	"errors"
	"net"
	"testing"
	"time"
)

func TestDeadlineConnReadTimeout(t *testing.T) {
	t.Parallel()

	server, client := net.Pipe()
	t.Cleanup(func() { _ = server.Close() })
	t.Cleanup(func() { _ = client.Close() })

	conn := NewDeadlineConn(server, 50*time.Millisecond, 0)
	buf := make([]byte, 8)

	start := time.Now()
	_, err := conn.Read(buf)
	if err == nil {
		t.Fatalf("expected read timeout error, got nil")
	}
	var nerr net.Error
	if !errors.As(err, &nerr) || !nerr.Timeout() {
		t.Fatalf("expected net.Error timeout, got %T: %v", err, err)
	}
	if time.Since(start) > 2*time.Second {
		t.Fatalf("read took too long: %s", time.Since(start))
	}
}

func TestDeadlineConnWriteTimeout(t *testing.T) {
	t.Parallel()

	server, client := net.Pipe()
	t.Cleanup(func() { _ = server.Close() })
	t.Cleanup(func() { _ = client.Close() })

	conn := NewDeadlineConn(server, 0, 50*time.Millisecond)

	done := make(chan error, 1)
	go func() {
		_, err := conn.Write([]byte("hello"))
		done <- err
	}()

	select {
	case err := <-done:
		if err == nil {
			t.Fatalf("expected write timeout error, got nil")
		}
		var nerr net.Error
		if !errors.As(err, &nerr) || !nerr.Timeout() {
			t.Fatalf("expected net.Error timeout, got %T: %v", err, err)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("write did not return within timeout")
	}
}

func TestSocks5HandshakeTimeoutOnSlowClient(t *testing.T) {
	t.Parallel()

	server, client := net.Pipe()
	t.Cleanup(func() { _ = server.Close() })
	t.Cleanup(func() { _ = client.Close() })

	conn := NewDeadlineConn(server, 50*time.Millisecond, 50*time.Millisecond)
	handshake := &Socks5Handshake{}

	done := make(chan error, 1)
	go func() {
		_, _, err := handshake.Handshake(conn)
		done <- err
	}()

	// Send only the SOCKS5 version byte, then stall so the header read times out.
	_, _ = client.Write([]byte{0x05})
	time.Sleep(120 * time.Millisecond)

	select {
	case err := <-done:
		if err == nil {
			t.Fatalf("expected timeout error, got nil")
		}
		var nerr net.Error
		if !errors.As(err, &nerr) || !nerr.Timeout() {
			t.Fatalf("expected net.Error timeout, got %T: %v", err, err)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("handshake did not return within timeout")
	}
}
