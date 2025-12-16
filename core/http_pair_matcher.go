package core

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"
)

type PendingRequest struct {
	Message   *HTTPMessage
	Timestamp time.Time
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
func (m *HTTPPairMatcher) AddRequest(connID string, msg *HTTPMessage) {
	m.mu.Lock()
	defer m.mu.Unlock()

	pending := &PendingRequest{
		Message:   msg,
		Timestamp: msg.Timestamp,
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
	httpLog := m.createHTTPLog(connID, request.Message, response)

	// Callback to persist
	if m.onPairComplete != nil {
		m.onPairComplete(httpLog)
	}
}

// createHTTPLog builds an HTTP log entry
func (m *HTTPPairMatcher) createHTTPLog(connID string, request, response *HTTPMessage) *HTTPLog {
	// Parse request
	method, url, protocol, host := parseRequest(request.Data)

	// Extract client info from connID
	clientInfo := extractClientInfo(connID)
	enhancedConnID := connID + clientInfo

	// Parse response
	responseStr := ""
	responseDecoded := ""
	isGzipped := false
	respSize := 0
	var durationMs int64 = 0

	if response != nil {
		responseStr = string(response.Data)
		respSize = len(response.Data)
		isGzipped, responseDecoded = tryDecompressGzipData(response.Data)

		// Compute latency in milliseconds
		durationMs = response.Timestamp.Sub(request.Timestamp).Milliseconds()
	}

	return &HTTPLog{
		Timestamp:       request.Timestamp,
		ConnID:          enhancedConnID,
		Method:          method,
		URL:             url,
		Host:            host,
		Protocol:        protocol,
		Request:         string(request.Data),
		Response:        responseStr,
		ResponseDecoded: responseDecoded,
		ReqSize:         len(request.Data),
		RespSize:        respSize,
		IsGzipped:       isGzipped,
		DurationMs:      durationMs,
	}
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
				httpLog := m.createHTTPLog(connID, req.Message, nil)
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

// tryDecompressGzipData attempts to decompress gzip content
func tryDecompressGzipData(data []byte) (bool, string) {
	// Check response headers for Content-Encoding: gzip
	lines := bytes.Split(data, []byte("\r\n"))
	hasGzipEncoding := false
	hasChunkedEncoding := false
	headerEndIndex := 0

	for i, line := range lines {
		if len(line) == 0 {
			headerEndIndex = i
			break
		}
		lowerLine := bytes.ToLower(line)
		if bytes.Contains(lowerLine, []byte("content-encoding")) &&
			bytes.Contains(lowerLine, []byte("gzip")) {
			hasGzipEncoding = true
		}
		if bytes.Contains(lowerLine, []byte("transfer-encoding")) &&
			bytes.Contains(lowerLine, []byte("chunked")) {
			hasChunkedEncoding = true
		}
	}

	if !hasGzipEncoding {
		return false, ""
	}

	// Extract body
	if headerEndIndex >= len(lines)-1 {
		return true, "[No body or body parsing failed]"
	}

	bodyStart := bytes.Index(data, []byte("\r\n\r\n"))
	if bodyStart == -1 {
		return true, "[Body not found]"
	}
	bodyStart += 4

	if bodyStart >= len(data) {
		return true, "[Empty body]"
	}

	bodyData := data[bodyStart:]

	// Decode chunked body if needed
	if hasChunkedEncoding {
		bodyData = dechunkBodyData(bodyData)
		if bodyData == nil {
			return true, "[Chunked decoding failed]"
		}
	}

	// Ensure this is actually gzip data
	if len(bodyData) < 2 || bodyData[0] != 0x1f || bodyData[1] != 0x8b {
		return true, "[Not valid gzip data]"
	}

	// Attempt gzip decompress
	reader, err := gzip.NewReader(bytes.NewReader(bodyData))
	if err != nil {
		return true, "[Gzip decompression failed: " + err.Error() + "]"
	}
	defer reader.Close()

	decompressed, err := io.ReadAll(reader)
	if err != nil {
		return true, "[Gzip read failed: " + err.Error() + "]"
	}

	// Drop Content-Encoding, Content-Length, and Transfer-Encoding headers
	var newHeaders []string
	for _, line := range lines[:headerEndIndex] {
		lineStr := string(line)
		lowerLine := strings.ToLower(lineStr)
		if !strings.Contains(lowerLine, "content-encoding") &&
			!strings.Contains(lowerLine, "content-length") &&
			!strings.Contains(lowerLine, "transfer-encoding") {
			newHeaders = append(newHeaders, lineStr)
		}
	}

	result := strings.Join(newHeaders, "\r\n") + "\r\n\r\n" + string(decompressed)
	return true, result
}

// dechunkBodyData decodes chunked transfer encoding
func dechunkBodyData(data []byte) []byte {
	var result bytes.Buffer
	reader := bytes.NewReader(data)

	for {
		// Read chunk size line
		var chunkSizeLine []byte
		for {
			b, err := reader.ReadByte()
			if err != nil {
				return nil
			}
			chunkSizeLine = append(chunkSizeLine, b)
			if len(chunkSizeLine) >= 2 &&
				chunkSizeLine[len(chunkSizeLine)-2] == '\r' &&
				chunkSizeLine[len(chunkSizeLine)-1] == '\n' {
				break
			}
		}

		// Parse chunk size
		sizeStr := strings.TrimSpace(string(chunkSizeLine[:len(chunkSizeLine)-2]))
		if idx := strings.Index(sizeStr, ";"); idx > 0 {
			sizeStr = sizeStr[:idx]
		}

		var chunkSize int
		_, err := fmt.Sscanf(sizeStr, "%x", &chunkSize)
		if err != nil {
			return nil
		}

		// Chunk size 0 means end
		if chunkSize == 0 {
			break
		}

		// Read chunk data
		chunkData := make([]byte, chunkSize)
		n, err := reader.Read(chunkData)
		if err != nil || n != chunkSize {
			return nil
		}
		result.Write(chunkData)

		// Read and discard trailing \r\n
		if _, err := reader.ReadByte(); err != nil {
			return nil
		}
		if _, err := reader.ReadByte(); err != nil {
			return nil
		}
	}

	return result.Bytes()
}
