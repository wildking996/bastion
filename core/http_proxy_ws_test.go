package core

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"bastion/config"
	"bastion/models"

	"github.com/gorilla/websocket"
)

func TestHTTPProxy_WebSocketUpgrade_ProxiesBidirectional(t *testing.T) {
	prevAudit := config.Settings.AuditEnabled
	prevLogLevel := config.Settings.LogLevel
	prevHandshakeTimeout := config.Settings.Socks5HandshakeTimeoutSeconds
	t.Cleanup(func() {
		config.Settings.AuditEnabled = prevAudit
		config.Settings.LogLevel = prevLogLevel
		config.Settings.Socks5HandshakeTimeoutSeconds = prevHandshakeTimeout
	})

	// Avoid needing AuditorInstance in tests.
	config.Settings.AuditEnabled = false
	config.Settings.LogLevel = "ERROR"
	config.Settings.Socks5HandshakeTimeoutSeconds = 2

	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		for {
			mt, msg, err := conn.ReadMessage()
			if err != nil {
				return
			}
			if err := conn.WriteMessage(mt, msg); err != nil {
				return
			}
		}
	}))
	t.Cleanup(backend.Close)

	mapping := &models.Mapping{
		ID:        "test-http-proxy-ws",
		LocalHost: "127.0.0.1",
		LocalPort: 0,
		Type:      "http",
	}
	session := NewHTTPProxySession(mapping, nil)
	if err := session.Start(); err != nil {
		t.Fatalf("Start HTTP proxy session: %v", err)
	}
	t.Cleanup(session.Stop)

	proxyURL, err := url.Parse("http://" + session.listener.Addr().String())
	if err != nil {
		t.Fatalf("parse proxy URL: %v", err)
	}

	wsURL := "ws" + strings.TrimPrefix(backend.URL, "http") + "/echo"
	dialer := websocket.Dialer{
		Proxy: http.ProxyURL(proxyURL),
	}

	c, _, err := dialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial ws via proxy: %v", err)
	}
	defer c.Close()

	if err := c.WriteMessage(websocket.TextMessage, []byte("ping")); err != nil {
		t.Fatalf("write ws message: %v", err)
	}

	_ = c.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, got, err := c.ReadMessage()
	if err != nil {
		t.Fatalf("read ws message: %v", err)
	}
	if string(got) != "ping" {
		t.Fatalf("unexpected echo: %q", string(got))
	}
}
