package core

import (
	"testing"
	"time"
)

func TestAuditor_QueryHTTPLogs_FilterAndPagination(t *testing.T) {
	a := &Auditor{
		httpLogs:    make([]*HTTPLog, 0, 10),
		httpLogsMap: make(map[int]*HTTPLog),
		maxLogs:     10,
	}

	now := time.Now()
	a.saveHTTPLog(&HTTPLog{
		Timestamp:  now.Add(-3 * time.Hour),
		ConnID:     "c1",
		Method:     "GET",
		Host:       "example.com",
		URL:        "/foo",
		Protocol:   "HTTP/1.1",
		StatusCode: 200,
		Request:    "GET /foo HTTP/1.1",
		Response:   "HTTP/1.1 200 OK",
	})
	a.saveHTTPLog(&HTTPLog{
		Timestamp:  now.Add(-2 * time.Hour),
		ConnID:     "c2",
		Method:     "POST",
		Host:       "api.example.com",
		URL:        "/bar",
		Protocol:   "HTTP/1.1",
		StatusCode: 500,
		Request:    "POST /bar HTTP/1.1",
		Response:   "HTTP/1.1 500 Internal Server Error",
	})
	a.saveHTTPLog(&HTTPLog{
		Timestamp:  now.Add(-1 * time.Hour),
		ConnID:     "c3",
		Method:     "GET",
		Host:       "example.com",
		URL:        "/baz",
		Protocol:   "HTTP/1.1",
		StatusCode: 404,
		Request:    "GET /baz HTTP/1.1",
		Response:   "HTTP/1.1 404 Not Found",
	})

	since := now.Add(-90 * time.Minute)
	filter := HTTPLogFilter{
		Method: "GET",
		Since:  &since,
	}

	logs, total := a.QueryHTTPLogs(filter, 1, 20)
	if total != 1 {
		t.Fatalf("expected total 1, got %d", total)
	}
	if len(logs) != 1 {
		t.Fatalf("expected 1 log, got %d", len(logs))
	}
	if logs[0].URL != "/baz" {
		t.Fatalf("expected /baz, got %q", logs[0].URL)
	}

	// Keyword search across host/url
	logs, total = a.QueryHTTPLogs(HTTPLogFilter{Query: "api.example.com"}, 1, 20)
	if total != 1 || len(logs) != 1 || logs[0].ConnID != "c2" {
		t.Fatalf("unexpected keyword search result: total=%d len=%d", total, len(logs))
	}

	// Keyword search should include decompressed response body
	a.saveHTTPLog(&HTTPLog{
		Timestamp:       now.Add(-30 * time.Minute),
		ConnID:          "c4",
		Method:          "GET",
		Host:            "example.com",
		URL:             "/gzip",
		Protocol:        "HTTP/1.1",
		StatusCode:      200,
		Request:         "GET /gzip HTTP/1.1",
		Response:        "HTTP/1.1 200 OK",
		ResponseDecoded: "hello from decoded gzip body",
		IsGzipped:       true,
	})
	logs, total = a.QueryHTTPLogs(HTTPLogFilter{Query: "decoded gzip"}, 1, 20)
	if total != 1 || len(logs) != 1 || logs[0].ConnID != "c4" {
		t.Fatalf("unexpected decoded-body search result: total=%d len=%d", total, len(logs))
	}

	// Pagination on all logs (latest first)
	logs, total = a.QueryHTTPLogs(HTTPLogFilter{}, 1, 2)
	if total != 4 || len(logs) != 2 {
		t.Fatalf("unexpected pagination result: total=%d len=%d", total, len(logs))
	}
	if logs[0].ConnID != "c4" || logs[1].ConnID != "c3" {
		t.Fatalf("unexpected ordering: got %q then %q", logs[0].ConnID, logs[1].ConnID)
	}
}

func TestHTTPPairMatcher_StatusCodeParsed(t *testing.T) {
	matcher := NewHTTPPairMatcher(nil)
	now := time.Now()

	req := &HTTPMessage{
		Type:      HTTPRequest,
		Timestamp: now,
		Data:      []byte("GET /hello HTTP/1.1\r\nHost: example.com\r\n\r\n"),
	}
	resp := &HTTPMessage{
		Type:      HTTPResponse,
		Timestamp: now.Add(10 * time.Millisecond),
		Data:      []byte("HTTP/1.1 404 Not Found\r\nContent-Length: 0\r\n\r\n"),
	}

	httpLog := matcher.createHTTPLog("127.0.0.1:1->127.0.0.1:2", req, resp)
	if httpLog.StatusCode != 404 {
		t.Fatalf("expected status_code=404, got %d", httpLog.StatusCode)
	}
}
