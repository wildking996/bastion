package core

import "testing"

func TestClassifyMixedProtocol(t *testing.T) {
	cases := []struct {
		name   string
		prefix []byte
		want   string
	}{
		{"socks5", []byte{0x05, 0x01, 0x00}, "socks5"},
		{"http_get", []byte("GET http://example.com/ HTTP/1.1\r\n"), "http"},
		{"http_connect", []byte("CONNECT example.com:443 HTTP/1.1\r\n"), "http"},
		{"leading_ws", []byte("  \r\nPOST / HTTP/1.1\r\n"), "http"},
		{"unknown", []byte{0x01, 0x02, 0x03}, ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := classifyMixedProtocol(tc.prefix); got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}
