package core

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"
)

type HTTPMessageType int

const (
	HTTPRequest HTTPMessageType = iota
	HTTPResponse
)

type HTTPMessage struct {
	Type      HTTPMessageType
	Data      []byte
	Timestamp time.Time
}

type HTTPStreamParser struct {
	connID         string
	direction      string
	buffer         *bytes.Buffer
	contentLength  int
	isChunked      bool
	headerComplete bool
	mu             sync.Mutex
}

func NewHTTPStreamParser(connID, direction string) *HTTPStreamParser {
	return &HTTPStreamParser{
		connID:        connID,
		direction:     direction,
		buffer:        &bytes.Buffer{},
		contentLength: -1,
	}
}

// Feed ingests data and returns complete HTTP messages if available
func (p *HTTPStreamParser) Feed(data []byte) []*HTTPMessage {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.buffer.Write(data)

	var messages []*HTTPMessage

	// Extract complete messages (handles keep-alive)
	for {
		msg := p.tryExtractMessage()
		if msg == nil {
			break
		}
		messages = append(messages, msg)
	}

	return messages
}

// tryExtractMessage attempts to pull a complete message from the buffer
func (p *HTTPStreamParser) tryExtractMessage() *HTTPMessage {
	data := p.buffer.Bytes()

	if len(data) == 0 {
		return nil
	}

	// Locate header terminator
	headerEnd := bytes.Index(data, []byte("\r\n\r\n"))
	if headerEnd == -1 {
		return nil // header incomplete
	}

	if !p.headerComplete {
		p.parseHeaders(data[:headerEnd])
		p.headerComplete = true
	}

	bodyStart := headerEnd + 4

	// Determine completeness for different body types
	var messageEnd int

	if p.isChunked {
		messageEnd = p.findChunkedEnd(data, bodyStart)
	} else if p.contentLength >= 0 {
		messageEnd = bodyStart + p.contentLength
	} else {
		// No body or response terminated by connection close
		messageEnd = bodyStart
	}

	if messageEnd == -1 || messageEnd > len(data) {
		return nil // message incomplete
	}

	// Extract full message
	messageData := make([]byte, messageEnd)
	copy(messageData, data[:messageEnd])

	// Remove extracted bytes from buffer
	p.buffer.Next(messageEnd)

	// Reset state for next message
	p.reset()

	// Identify message type
	msgType := p.detectMessageType(messageData)

	return &HTTPMessage{
		Type:      msgType,
		Data:      messageData,
		Timestamp: time.Now(),
	}
}

// parseHeaders parses HTTP headers
func (p *HTTPStreamParser) parseHeaders(headerData []byte) {
	lines := bytes.Split(headerData, []byte("\r\n"))

	for _, line := range lines {
		lowerLine := bytes.ToLower(line)

		// Parse Content-Length
		if bytes.HasPrefix(lowerLine, []byte("content-length:")) {
			parts := bytes.SplitN(line, []byte(":"), 2)
			if len(parts) == 2 {
				lengthStr := strings.TrimSpace(string(parts[1]))
				if length, err := strconv.Atoi(lengthStr); err == nil {
					p.contentLength = length
				}
			}
		}

		// Check Transfer-Encoding: chunked
		if bytes.HasPrefix(lowerLine, []byte("transfer-encoding:")) &&
			bytes.Contains(lowerLine, []byte("chunked")) {
			p.isChunked = true
		}
	}
}

// findChunkedEnd finds the end of a chunked-encoded body
func (p *HTTPStreamParser) findChunkedEnd(data []byte, start int) int {
	pos := start

	for {
		if pos >= len(data) {
			return -1 // insufficient data
		}

		// Find end of chunk-size line
		lineEnd := bytes.Index(data[pos:], []byte("\r\n"))
		if lineEnd == -1 {
			return -1
		}
		lineEnd += pos

		// Parse chunk size
		sizeStr := strings.TrimSpace(string(data[pos:lineEnd]))
		if idx := strings.Index(sizeStr, ";"); idx > 0 {
			sizeStr = sizeStr[:idx]
		}

		var chunkSize int
		_, err := fmt.Sscanf(sizeStr, "%x", &chunkSize)
		if err != nil {
			return -1
		}

		// Chunk size 0 means finished
		if chunkSize == 0 {
			// Skip final \r\n
			return lineEnd + 4 // \r\n + \r\n
		}

		// Skip chunk size line, data, and trailing \r\n
		pos = lineEnd + 2 + chunkSize + 2
	}
}

// detectMessageType detects whether data is a request or response
func (p *HTTPStreamParser) detectMessageType(data []byte) HTTPMessageType {
	if len(data) < 4 {
		return HTTPRequest // default
	}

	// Check for HTTP/ prefix (response)
	if bytes.HasPrefix(data, []byte("HTTP/")) {
		return HTTPResponse
	}

	return HTTPRequest
}

// reset clears parser state
func (p *HTTPStreamParser) reset() {
	p.contentLength = -1
	p.isChunked = false
	p.headerComplete = false
}

// Flush forces out remaining buffered data (e.g., on connection close)
func (p *HTTPStreamParser) Flush() *HTTPMessage {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.buffer.Len() == 0 {
		return nil
	}

	data := p.buffer.Bytes()
	msgType := p.detectMessageType(data)

	msg := &HTTPMessage{
		Type:      msgType,
		Data:      make([]byte, len(data)),
		Timestamp: time.Now(),
	}
	copy(msg.Data, data)

	p.buffer.Reset()
	p.reset()

	return msg
}
