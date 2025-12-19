package core

import (
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"bastion/config"
)

// HTTPLogPart selects which slice of a stored HTTP message to return.
type HTTPLogPart string

const (
	// HTTPLogPartRequestHeader returns the request headers (without the body).
	HTTPLogPartRequestHeader HTTPLogPart = "request_header"
	// HTTPLogPartRequestBody returns the request body (without the headers).
	HTTPLogPartRequestBody HTTPLogPart = "request_body"
	// HTTPLogPartResponseHeader returns the response headers (without the body).
	HTTPLogPartResponseHeader HTTPLogPart = "response_header"
	// HTTPLogPartResponseBody returns the response body (without the headers).
	HTTPLogPartResponseBody HTTPLogPart = "response_body"
)

// HTTPLogPartOptions configures HTTP log part retrieval.
type HTTPLogPartOptions struct {
	// DecodeGzip enables on-demand gzip decoding for response bodies only.
	DecodeGzip bool
}

// HTTPLogPartResult is the response payload for a single log part.
type HTTPLogPartResult struct {
	Data            string `json:"data"`
	Truncated       bool   `json:"truncated"`
	TruncatedReason string `json:"truncated_reason,omitempty"`
}

var (
	// ErrHTTPLogNotFound indicates a missing HTTP log by ID.
	ErrHTTPLogNotFound = errors.New("http log not found")
	// ErrInvalidHTTPLogPart indicates an unknown part selector.
	ErrInvalidHTTPLogPart = errors.New("invalid http log part")
	// ErrGzipDecodeNotAllowed indicates an attempt to gzip-decode a non-response-body part.
	ErrGzipDecodeNotAllowed = errors.New("gzip decode not allowed for this part")
	// ErrNotGzippedResponse indicates the stored response is not gzip-compressed.
	ErrNotGzippedResponse = errors.New("response is not gzipped")
)

type gzipDecodedBodyCacheEntry struct {
	result     HTTPLogPartResult
	lastAccess time.Time
	expiresAt  time.Time
}

func httpMessageHasGzipEncoding(data []byte) bool {
	headers, _, _ := splitHTTPMessage(data)
	return httpHeadersContain(headers, "content-encoding", "gzip")
}

func splitHTTPMessage(data []byte) (headers []byte, body []byte, ok bool) {
	sep := []byte("\r\n\r\n")
	idx := bytes.Index(data, sep)
	if idx == -1 {
		return data, nil, false
	}
	return data[:idx], data[idx+len(sep):], true
}

func httpHeadersContain(headers []byte, headerKey, token string) bool {
	key := []byte(strings.ToLower(headerKey) + ":")
	want := []byte(strings.ToLower(token))

	for _, line := range bytes.Split(headers, []byte("\r\n")) {
		lower := bytes.ToLower(line)
		if !bytes.HasPrefix(lower, key) {
			continue
		}
		return bytes.Contains(lower, want)
	}
	return false
}

// GetHTTPLogPart returns a specific part of a stored HTTP log message.
//
// Supported parts:
// - request_header, request_body
// - response_header, response_body
//
// When opts.DecodeGzip is true, only response_body is supported and the result may be truncated with a reason.
func (a *Auditor) GetHTTPLogPart(id int, part HTTPLogPart, opts HTTPLogPartOptions) (*HTTPLogPartResult, error) {
	a.httpMu.RLock()
	log := a.httpLogsMap[id]
	a.httpMu.RUnlock()

	if log == nil {
		return nil, ErrHTTPLogNotFound
	}

	if opts.DecodeGzip && part != HTTPLogPartResponseBody {
		return nil, ErrGzipDecodeNotAllowed
	}

	switch part {
	case HTTPLogPartRequestHeader:
		headers, _, _ := splitHTTPMessage([]byte(log.Request))
		return &HTTPLogPartResult{Data: string(headers)}, nil
	case HTTPLogPartRequestBody:
		_, body, ok := splitHTTPMessage([]byte(log.Request))
		if !ok {
			return &HTTPLogPartResult{Data: ""}, nil
		}
		return &HTTPLogPartResult{Data: string(body)}, nil
	case HTTPLogPartResponseHeader:
		headers, _, _ := splitHTTPMessage([]byte(log.Response))
		return &HTTPLogPartResult{Data: string(headers)}, nil
	case HTTPLogPartResponseBody:
		return a.getHTTPLogResponseBody(id, []byte(log.Response), opts)
	default:
		return nil, ErrInvalidHTTPLogPart
	}
}

func (a *Auditor) getHTTPLogResponseBody(id int, rawResponse []byte, opts HTTPLogPartOptions) (*HTTPLogPartResult, error) {
	headers, body, ok := splitHTTPMessage(rawResponse)
	if !ok {
		return &HTTPLogPartResult{Data: ""}, nil
	}

	if !opts.DecodeGzip {
		return &HTTPLogPartResult{Data: string(body)}, nil
	}

	if !httpHeadersContain(headers, "content-encoding", "gzip") {
		return nil, ErrNotGzippedResponse
	}

	ttl := time.Duration(config.Settings.HTTPGzipDecodeCacheSeconds) * time.Second
	now := time.Now()

	if ttl > 0 {
		if cached := a.getCachedGzipDecodedBody(id, now, ttl); cached != nil {
			return cached, nil
		}
	}

	if httpHeadersContain(headers, "transfer-encoding", "chunked") {
		body = dechunkBodyData(body)
		if body == nil {
			return &HTTPLogPartResult{
				Data:            "",
				Truncated:       true,
				TruncatedReason: "invalid_chunked",
			}, nil
		}
	}

	maxBytes := config.Settings.HTTPGzipDecodeMaxBytes
	if maxBytes <= 0 {
		maxBytes = 1048576
	}
	timeout := time.Duration(config.Settings.HTTPGzipDecodeTimeoutMS) * time.Millisecond
	if timeout < 0 {
		timeout = 0
	}

	decoded, truncated, reason := decodeGzipBodyPreview(body, maxBytes, timeout)
	result := &HTTPLogPartResult{
		Data:            string(decoded),
		Truncated:       truncated,
		TruncatedReason: reason,
	}

	if ttl > 0 && (reason == "" || reason == "max_bytes") {
		a.setCachedGzipDecodedBody(id, now, ttl, result)
	}

	return result, nil
}

func (a *Auditor) getCachedGzipDecodedBody(id int, now time.Time, ttl time.Duration) *HTTPLogPartResult {
	a.gzipDecodeMu.Lock()
	defer a.gzipDecodeMu.Unlock()

	a.sweepGzipDecodedBodyCacheLocked(now, ttl)

	entry := a.gzipDecodedBodyCache[id]
	if entry == nil || now.After(entry.expiresAt) {
		delete(a.gzipDecodedBodyCache, id)
		return nil
	}

	entry.lastAccess = now
	entry.expiresAt = now.Add(ttl)
	copied := entry.result
	return &copied
}

func (a *Auditor) setCachedGzipDecodedBody(id int, now time.Time, ttl time.Duration, result *HTTPLogPartResult) {
	a.gzipDecodeMu.Lock()
	defer a.gzipDecodeMu.Unlock()

	a.gzipDecodedBodyCache[id] = &gzipDecodedBodyCacheEntry{
		result:     *result,
		lastAccess: now,
		expiresAt:  now.Add(ttl),
	}
}

func (a *Auditor) sweepGzipDecodedBodyCacheLocked(now time.Time, ttl time.Duration) {
	if ttl <= 0 {
		a.gzipDecodedBodyCache = make(map[int]*gzipDecodedBodyCacheEntry)
		a.gzipCacheLastSweep = now
		return
	}

	const sweepInterval = 30 * time.Second
	if !a.gzipCacheLastSweep.IsZero() && now.Sub(a.gzipCacheLastSweep) < sweepInterval {
		return
	}
	a.gzipCacheLastSweep = now

	for id, entry := range a.gzipDecodedBodyCache {
		if entry == nil || now.After(entry.expiresAt) {
			delete(a.gzipDecodedBodyCache, id)
		}
	}
}

func decodeGzipBodyPreview(compressed []byte, maxBytes int, timeout time.Duration) (preview []byte, truncated bool, reason string) {
	if maxBytes < 0 {
		maxBytes = 0
	}
	if len(compressed) == 0 {
		return nil, false, ""
	}

	if len(compressed) < 2 || compressed[0] != 0x1f || compressed[1] != 0x8b {
		return previewBytes(compressed, maxBytes), len(compressed) > maxBytes, "invalid_gzip"
	}

	reader, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		return previewBytes(compressed, maxBytes), len(compressed) > maxBytes, "invalid_gzip"
	}
	defer reader.Close()

	start := time.Now()
	var out bytes.Buffer
	tmp := make([]byte, 32*1024)

	for {
		if timeout > 0 && time.Since(start) > timeout {
			return out.Bytes(), true, "timeout"
		}

		n, readErr := reader.Read(tmp)
		if n > 0 {
			remaining := maxBytes - out.Len()
			if remaining <= 0 {
				return out.Bytes(), true, "max_bytes"
			}
			if n > remaining {
				out.Write(tmp[:remaining])
				return out.Bytes(), true, "max_bytes"
			}
			out.Write(tmp[:n])
		}

		if readErr == io.EOF {
			return out.Bytes(), false, ""
		}
		if readErr != nil {
			if out.Len() > 0 {
				return out.Bytes(), true, "gzip_read_error"
			}
			return previewBytes(compressed, maxBytes), len(compressed) > maxBytes, "gzip_read_error"
		}
	}
}

func previewBytes(data []byte, maxBytes int) []byte {
	if maxBytes <= 0 {
		return nil
	}
	if len(data) <= maxBytes {
		return data
	}
	return data[:maxBytes]
}

func dechunkBodyData(data []byte) []byte {
	var result bytes.Buffer
	reader := bytes.NewReader(data)

	for {
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

		sizeStr := strings.TrimSpace(string(chunkSizeLine[:len(chunkSizeLine)-2]))
		if idx := strings.Index(sizeStr, ";"); idx > 0 {
			sizeStr = sizeStr[:idx]
		}

		var chunkSize int
		_, err := fmt.Sscanf(sizeStr, "%x", &chunkSize)
		if err != nil {
			return nil
		}

		if chunkSize == 0 {
			break
		}

		chunkData := make([]byte, chunkSize)
		n, err := reader.Read(chunkData)
		if err != nil || n != chunkSize {
			return nil
		}
		result.Write(chunkData)

		if _, err := reader.ReadByte(); err != nil {
			return nil
		}
		if _, err := reader.ReadByte(); err != nil {
			return nil
		}
	}

	return result.Bytes()
}
