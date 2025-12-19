package core

import (
	"testing"
	"time"

	"bastion/config"
)

func TestAuditor_EnqueueHTTPMessage_DropsWhenQueueFull(t *testing.T) {
	oldEnabled := config.Settings.AuditEnabled
	t.Cleanup(func() {
		config.Settings.AuditEnabled = oldEnabled
	})
	config.Settings.AuditEnabled = true

	a := &Auditor{
		running:     true,
		httpLogs:    make([]*HTTPLog, 0, 10),
		httpLogsMap: make(map[int]*HTTPLog),
		maxLogs:     10,
	}
	a.auditQueue = make(chan auditEvent, 1)

	msg := &HTTPMessage{Type: HTTPRequest, Timestamp: time.Now(), Data: []byte("GET / HTTP/1.1\r\n\r\n")}
	if ok := a.EnqueueHTTPMessage(AuditContext{}, "c", msg); !ok {
		t.Fatalf("expected enqueue ok")
	}
	if ok := a.EnqueueHTTPMessage(AuditContext{}, "c", msg); ok {
		t.Fatalf("expected enqueue to drop when full")
	}
	if got := a.AuditDroppedTotal(); got != 1 {
		t.Fatalf("expected dropped_total=1, got %d", got)
	}
}

func TestAuditor_AuditQueueProcessesMessages(t *testing.T) {
	oldEnabled := config.Settings.AuditEnabled
	oldQueue := config.Settings.AuditQueueSize
	t.Cleanup(func() {
		config.Settings.AuditEnabled = oldEnabled
		config.Settings.AuditQueueSize = oldQueue
	})
	config.Settings.AuditEnabled = true
	config.Settings.AuditQueueSize = 10

	a := &Auditor{
		httpLogs:             make([]*HTTPLog, 0, 10),
		httpLogsMap:          make(map[int]*HTTPLog),
		maxLogs:              10,
		gzipDecodedBodyCache: make(map[int]*gzipDecodedBodyCacheEntry),
	}
	a.pairMatcher = NewHTTPPairMatcher(func(httpLog *HTTPLog) {
		a.saveHTTPLog(httpLog)
	})

	a.running = true
	a.startAuditQueue()
	t.Cleanup(func() {
		a.Stop()
	})

	connID := "127.0.0.1:1->127.0.0.1:2"
	ctx := AuditContext{MappingID: "m", LocalPort: 7788, BastionChain: []string{"b"}}

	req := &HTTPMessage{
		Type:      HTTPRequest,
		Timestamp: time.Now(),
		Data:      []byte("GET /hello HTTP/1.1\r\nHost: example.com\r\n\r\n"),
	}
	resp := &HTTPMessage{
		Type:      HTTPResponse,
		Timestamp: time.Now().Add(10 * time.Millisecond),
		Data:      []byte("HTTP/1.1 200 OK\r\nContent-Length: 0\r\n\r\n"),
	}

	if ok := a.EnqueueHTTPMessage(ctx, connID, req); !ok {
		t.Fatalf("expected enqueue ok for request")
	}
	if ok := a.EnqueueHTTPMessage(ctx, connID, resp); !ok {
		t.Fatalf("expected enqueue ok for response")
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		_, total := a.GetHTTPLogs(1, 10)
		if total == 1 {
			log := a.GetHTTPLogByID(1)
			if log == nil {
				t.Fatalf("expected log present")
			}
			if log.MappingID != "m" || log.LocalPort != 7788 || len(log.BastionChain) != 1 || log.BastionChain[0] != "b" {
				t.Fatalf("unexpected context fields: %+v", log)
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Fatalf("timeout waiting for audit processing")
}
