package core

import (
	"bytes"
	"compress/gzip"
	"testing"
	"time"

	"bastion/config"
)

func gzipBytes(t *testing.T, input []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	if _, err := w.Write(input); err != nil {
		t.Fatalf("gzip write: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("gzip close: %v", err)
	}
	return buf.Bytes()
}

func TestAuditor_GetHTTPLogPart_ResponseBodyDecode(t *testing.T) {
	oldMax := config.Settings.HTTPGzipDecodeMaxBytes
	oldTimeout := config.Settings.HTTPGzipDecodeTimeoutMS
	oldCache := config.Settings.HTTPGzipDecodeCacheSeconds
	t.Cleanup(func() {
		config.Settings.HTTPGzipDecodeMaxBytes = oldMax
		config.Settings.HTTPGzipDecodeTimeoutMS = oldTimeout
		config.Settings.HTTPGzipDecodeCacheSeconds = oldCache
	})

	config.Settings.HTTPGzipDecodeMaxBytes = 1024
	config.Settings.HTTPGzipDecodeTimeoutMS = 1000
	config.Settings.HTTPGzipDecodeCacheSeconds = 60

	plain := []byte("hello world")
	resp := append(
		[]byte("HTTP/1.1 200 OK\r\nContent-Encoding: gzip\r\n\r\n"),
		gzipBytes(t, plain)...,
	)

	auditor := &Auditor{
		httpLogsMap:          map[int]*HTTPLog{1: {ID: 1, Response: string(resp)}},
		gzipDecodedBodyCache: make(map[int]*gzipDecodedBodyCacheEntry),
	}

	got, err := auditor.GetHTTPLogPart(1, HTTPLogPartResponseBody, HTTPLogPartOptions{DecodeGzip: true})
	if err != nil {
		t.Fatalf("GetHTTPLogPart: %v", err)
	}
	if got.Truncated {
		t.Fatalf("expected not truncated; got truncated=%v reason=%q", got.Truncated, got.TruncatedReason)
	}
	if got.TruncatedReason != "" {
		t.Fatalf("expected empty reason; got %q", got.TruncatedReason)
	}
	if got.Data != string(plain) {
		t.Fatalf("unexpected data: %q", got.Data)
	}
}

func TestAuditor_GetHTTPLogPart_ResponseBodyDecode_MaxBytes(t *testing.T) {
	oldMax := config.Settings.HTTPGzipDecodeMaxBytes
	oldTimeout := config.Settings.HTTPGzipDecodeTimeoutMS
	oldCache := config.Settings.HTTPGzipDecodeCacheSeconds
	t.Cleanup(func() {
		config.Settings.HTTPGzipDecodeMaxBytes = oldMax
		config.Settings.HTTPGzipDecodeTimeoutMS = oldTimeout
		config.Settings.HTTPGzipDecodeCacheSeconds = oldCache
	})

	config.Settings.HTTPGzipDecodeMaxBytes = 5
	config.Settings.HTTPGzipDecodeTimeoutMS = 1000
	config.Settings.HTTPGzipDecodeCacheSeconds = 0

	plain := []byte("hello world")
	resp := append(
		[]byte("HTTP/1.1 200 OK\r\nContent-Encoding: gzip\r\n\r\n"),
		gzipBytes(t, plain)...,
	)

	auditor := &Auditor{
		httpLogsMap:          map[int]*HTTPLog{1: {ID: 1, Response: string(resp)}},
		gzipDecodedBodyCache: make(map[int]*gzipDecodedBodyCacheEntry),
	}

	got, err := auditor.GetHTTPLogPart(1, HTTPLogPartResponseBody, HTTPLogPartOptions{DecodeGzip: true})
	if err != nil {
		t.Fatalf("GetHTTPLogPart: %v", err)
	}
	if !got.Truncated || got.TruncatedReason != "max_bytes" {
		t.Fatalf("expected max_bytes truncation; got truncated=%v reason=%q", got.Truncated, got.TruncatedReason)
	}
	if got.Data != "hello" {
		t.Fatalf("unexpected data: %q", got.Data)
	}
}

func TestAuditor_GetHTTPLogPart_ResponseBodyDecode_Cache(t *testing.T) {
	oldMax := config.Settings.HTTPGzipDecodeMaxBytes
	oldTimeout := config.Settings.HTTPGzipDecodeTimeoutMS
	oldCache := config.Settings.HTTPGzipDecodeCacheSeconds
	t.Cleanup(func() {
		config.Settings.HTTPGzipDecodeMaxBytes = oldMax
		config.Settings.HTTPGzipDecodeTimeoutMS = oldTimeout
		config.Settings.HTTPGzipDecodeCacheSeconds = oldCache
	})

	config.Settings.HTTPGzipDecodeMaxBytes = 1024
	config.Settings.HTTPGzipDecodeTimeoutMS = 1000
	config.Settings.HTTPGzipDecodeCacheSeconds = 60

	plain := []byte("cache me")
	resp := append(
		[]byte("HTTP/1.1 200 OK\r\nContent-Encoding: gzip\r\n\r\n"),
		gzipBytes(t, plain)...,
	)

	auditor := &Auditor{
		httpLogsMap:          map[int]*HTTPLog{1: {ID: 1, Response: string(resp)}},
		gzipDecodedBodyCache: make(map[int]*gzipDecodedBodyCacheEntry),
	}

	got1, err := auditor.GetHTTPLogPart(1, HTTPLogPartResponseBody, HTTPLogPartOptions{DecodeGzip: true})
	if err != nil {
		t.Fatalf("GetHTTPLogPart: %v", err)
	}
	if got1.Data != string(plain) {
		t.Fatalf("unexpected data: %q", got1.Data)
	}

	config.Settings.HTTPGzipDecodeMaxBytes = 1
	got2, err := auditor.GetHTTPLogPart(1, HTTPLogPartResponseBody, HTTPLogPartOptions{DecodeGzip: true})
	if err != nil {
		t.Fatalf("GetHTTPLogPart (cached): %v", err)
	}
	if got2.Data != string(plain) {
		t.Fatalf("expected cached full data; got %q", got2.Data)
	}

	auditor.gzipDecodeMu.Lock()
	entry := auditor.gzipDecodedBodyCache[1]
	if entry == nil {
		auditor.gzipDecodeMu.Unlock()
		t.Fatalf("expected cache entry")
	}
	entry.expiresAt = time.Now().Add(-time.Second)
	auditor.gzipDecodeMu.Unlock()

	got3, err := auditor.GetHTTPLogPart(1, HTTPLogPartResponseBody, HTTPLogPartOptions{DecodeGzip: true})
	if err != nil {
		t.Fatalf("GetHTTPLogPart (expired): %v", err)
	}
	if got3.Data != "c" {
		t.Fatalf("expected re-decode with new max bytes; got %q", got3.Data)
	}
}

func TestAuditor_GetHTTPLogPart_ResponseBodyDecode_InvalidGzip(t *testing.T) {
	oldMax := config.Settings.HTTPGzipDecodeMaxBytes
	oldTimeout := config.Settings.HTTPGzipDecodeTimeoutMS
	oldCache := config.Settings.HTTPGzipDecodeCacheSeconds
	t.Cleanup(func() {
		config.Settings.HTTPGzipDecodeMaxBytes = oldMax
		config.Settings.HTTPGzipDecodeTimeoutMS = oldTimeout
		config.Settings.HTTPGzipDecodeCacheSeconds = oldCache
	})

	config.Settings.HTTPGzipDecodeMaxBytes = 1024
	config.Settings.HTTPGzipDecodeTimeoutMS = 1000
	config.Settings.HTTPGzipDecodeCacheSeconds = 0

	body := []byte("not gzip")
	resp := append(
		[]byte("HTTP/1.1 200 OK\r\nContent-Encoding: gzip\r\n\r\n"),
		body...,
	)

	auditor := &Auditor{
		httpLogsMap:          map[int]*HTTPLog{1: {ID: 1, Response: string(resp)}},
		gzipDecodedBodyCache: make(map[int]*gzipDecodedBodyCacheEntry),
	}

	got, err := auditor.GetHTTPLogPart(1, HTTPLogPartResponseBody, HTTPLogPartOptions{DecodeGzip: true})
	if err != nil {
		t.Fatalf("GetHTTPLogPart: %v", err)
	}
	if got.TruncatedReason != "invalid_gzip" {
		t.Fatalf("expected invalid_gzip reason; got %q", got.TruncatedReason)
	}
	if got.Data != string(body) {
		t.Fatalf("unexpected preview: %q", got.Data)
	}
}
