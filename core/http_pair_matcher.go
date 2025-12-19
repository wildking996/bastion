package core

import (
	"bytes"
	"strconv"
	"strings"
	"sync"
	"time"
)

type PendingRequest struct {
	Message   *HTTPMessage
	Timestamp time.Time
	Ctx       AuditContext
}

type HTTPPairMatcher struct {
	pendingRequests map[string][]*PendingRequest // connID -> pending requests
	mu              sync.RWMutex
	onPairComplete  func(*HTTPLog)
}

func NewHTTPPairMatcher(onComplete func(*HTTPLog)) *HTTPPairMatcher {
	return &HTTPPairMatcher{
		pendingRequests: make(map[string][]*PendingRequest),
		onPairComplete:  onComplete,
	}
}

// AddRequest enqueues a request awaiting response pairing
func (m *HTTPPairMatcher) AddRequest(ctx AuditContext, connID string, msg *HTTPMessage) {
	m.mu.Lock()
	defer m.mu.Unlock()

	pending := &PendingRequest{
		Message:   msg,
		Timestamp: msg.Timestamp,
		Ctx:       ctx,
	}

	m.pendingRequests[connID] = append(m.pendingRequests[connID], pending)
}

// MatchResponse pairs a response to the earliest pending request
func (m *HTTPPairMatcher) MatchResponse(connID string, response *HTTPMessage) {
	m.mu.Lock()
	defer m.mu.Unlock()

	queue := m.pendingRequests[connID]
	if len(queue) == 0 {
		// No pending requests; response arrived first (rare)
		return
	}

	// FIFO: take earliest request
	request := queue[0]
	m.pendingRequests[connID] = queue[1:]

	// Build full log entry
	httpLog := m.createHTTPLog(request.Ctx, connID, request.Message, response)

	// Callback to persist
	if m.onPairComplete != nil {
		m.onPairComplete(httpLog)
	}
}

// createHTTPLog builds an HTTP log entry
func (m *HTTPPairMatcher) createHTTPLog(ctx AuditContext, connID string, request, response *HTTPMessage) *HTTPLog {
	// Parse request
	method, url, protocol, host := parseRequest(request.Data)

	// Extract client info from connID
	clientInfo := extractClientInfo(connID)
	enhancedConnID := connID + clientInfo

	// Parse response
	responseStr := ""
	isGzipped := false
	respSize := 0
	statusCode := 0
	var durationMs int64 = 0

	if response != nil {
		responseStr = string(response.Data)
		respSize = len(response.Data)
		isGzipped = httpMessageHasGzipEncoding(response.Data)
		statusCode = parseResponseStatusCode(response.Data)

		// Compute latency in milliseconds
		durationMs = response.Timestamp.Sub(request.Timestamp).Milliseconds()
	}

	return &HTTPLog{
		Timestamp:    request.Timestamp,
		ConnID:       enhancedConnID,
		MappingID:    ctx.MappingID,
		LocalPort:    ctx.LocalPort,
		BastionChain: ctx.BastionChain,
		Method:       method,
		URL:          url,
		Host:         host,
		Protocol:     protocol,
		StatusCode:   statusCode,
		Request:      string(request.Data),
		Response:     responseStr,
		ReqSize:      len(request.Data),
		RespSize:     respSize,
		IsGzipped:    isGzipped,
		DurationMs:   durationMs,
	}
}

func parseResponseStatusCode(data []byte) int {
	lines := bytes.SplitN(data, []byte("\r\n"), 2)
	if len(lines) == 0 {
		return 0
	}

	// Expected: HTTP/1.1 200 OK
	parts := bytes.SplitN(lines[0], []byte(" "), 3)
	if len(parts) < 2 {
		return 0
	}

	code, err := strconv.Atoi(string(parts[1]))
	if err != nil {
		return 0
	}
	return code
}

// extractClientInfo extracts client info from connID
func extractClientInfo(connID string) string {
	// connID format: "127.0.0.1:54321->10.25.236.153:9880"
	parts := strings.Split(connID, "->")
	if len(parts) > 0 {
		return " [client=" + parts[0] + "]"
	}
	return ""
}

// CleanupStale removes stale unmatched requests
func (m *HTTPPairMatcher) CleanupStale(maxAge time.Duration) int {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	cleaned := 0

	for connID, queue := range m.pendingRequests {
		var remaining []*PendingRequest

		for _, req := range queue {
			if now.Sub(req.Timestamp) > maxAge {
				// Timed out; save as an incomplete request
				httpLog := m.createHTTPLog(req.Ctx, connID, req.Message, nil)
				if m.onPairComplete != nil {
					m.onPairComplete(httpLog)
				}
				cleaned++
			} else {
				remaining = append(remaining, req)
			}
		}

		if len(remaining) > 0 {
			m.pendingRequests[connID] = remaining
		} else {
			delete(m.pendingRequests, connID)
		}
	}

	return cleaned
}

// parseRequest parses the HTTP request bytes
func parseRequest(data []byte) (method, url, protocol, host string) {
	lines := bytes.Split(data, []byte("\r\n"))

	if len(lines) > 0 {
		parts := bytes.SplitN(lines[0], []byte(" "), 3)
		if len(parts) >= 3 {
			method = string(parts[0])
			url = string(parts[1])
			protocol = string(parts[2])
		}
	}

	// Extract Host header
	for _, line := range lines {
		if bytes.HasPrefix(bytes.ToLower(line), []byte("host:")) {
			host = string(bytes.TrimSpace(line[5:]))
			break
		}
	}

	return
}
