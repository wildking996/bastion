//go:build !windows

package core

import (
	"bufio"
	"bytes"
	"context"
	"net"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

func queryPortListeners(network string, port int) ([]PortListener, DiagnosticsMeta) {
	if network != "tcp" {
		return nil, DiagnosticsMeta{Source: "none", Error: "unsupported network"}
	}

	if runtime.GOOS == "darwin" {
		return queryPortListenersLsof(port)
	}

	listeners, meta := queryPortListenersSS(port)
	if meta.Error == "" {
		return listeners, meta
	}

	listeners2, meta2 := queryPortListenersLsof(port)
	if len(listeners2) > 0 || meta2.Error == "" {
		return listeners2, meta2
	}

	return listeners, meta
}

func queryPortListenersSS(port int) ([]PortListener, DiagnosticsMeta) {
	path, err := exec.LookPath("ss")
	if err != nil {
		return nil, DiagnosticsMeta{Source: "ss", Error: "ss not found"}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 700*time.Millisecond)
	defer cancel()

	// Prefer using a filter expression to keep output small.
	out, runErr := exec.CommandContext(ctx, path, "-H", "-ltnp", "sport", "=", ":"+strconv.Itoa(port)).CombinedOutput()
	if runErr != nil {
		// Fallback without the filter (some ss versions don't support it).
		ctx2, cancel2 := context.WithTimeout(context.Background(), 900*time.Millisecond)
		defer cancel2()
		out2, runErr2 := exec.CommandContext(ctx2, path, "-H", "-ltnp").CombinedOutput()
		if runErr2 != nil {
			return nil, DiagnosticsMeta{Source: "ss", Error: strings.TrimSpace(string(out))}
		}
		out = out2
	}

	lines := scanLines(out)
	listeners := make([]PortListener, 0, 4)

	localRe := regexp.MustCompile(`(?i)(\[[^\]]+\]|[^\s:]+):([0-9]+)$`)
	pidRe := regexp.MustCompile(`pid=([0-9]+)`)    // users:(...,pid=1234,...)
	nameRe := regexp.MustCompile(`\(\"([^\"]+)\"`) // users:(("cmd",pid=...))

	for _, line := range lines {
		fields := strings.Fields(line)
		// Typical ss -H -ltnp columns:
		// State Recv-Q Send-Q Local Address:Port Peer Address:Port Process
		if len(fields) < 4 {
			continue
		}

		local := fields[3]
		m := localRe.FindStringSubmatch(local)
		if m == nil {
			continue
		}
		p, _ := strconv.Atoi(m[2])
		if p != port {
			continue
		}

		ip := strings.Trim(m[1], "[]")
		if ip == "*" {
			ip = "0.0.0.0"
		}

		pid := 0
		if pm := pidRe.FindStringSubmatch(line); pm != nil {
			pid, _ = strconv.Atoi(pm[1])
		}

		name := ""
		if nm := nameRe.FindStringSubmatch(line); nm != nil {
			name = nm[1]
		}

		addr := net.JoinHostPort(ip, strconv.Itoa(p))
		listeners = append(listeners, PortListener{
			Network: "tcp",
			IP:      ip,
			Port:    p,
			Addr:    addr,
			PID:     pid,
			Name:    name,
		})
	}

	return listeners, DiagnosticsMeta{Source: "ss"}
}

func queryPortListenersLsof(port int) ([]PortListener, DiagnosticsMeta) {
	path, err := exec.LookPath("lsof")
	if err != nil {
		return nil, DiagnosticsMeta{Source: "lsof", Error: "lsof not found"}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1200*time.Millisecond)
	defer cancel()

	out, runErr := exec.CommandContext(ctx, path, "-nP", "-iTCP:"+strconv.Itoa(port), "-sTCP:LISTEN").CombinedOutput()
	if runErr != nil {
		return nil, DiagnosticsMeta{Source: "lsof", Error: strings.TrimSpace(string(out))}
	}

	scanner := bufio.NewScanner(bytes.NewReader(out))
	listeners := make([]PortListener, 0, 4)
	first := true

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if first {
			first = false
			if strings.HasPrefix(line, "COMMAND") {
				continue
			}
		}

		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}

		cmd := fields[0]
		pid, _ := strconv.Atoi(fields[1])

		// NAME column typically ends with: "TCP <addr> (LISTEN)"
		addrTok := ""
		if fields[len(fields)-1] == "(LISTEN)" && len(fields) >= 2 {
			addrTok = fields[len(fields)-2]
		} else {
			addrTok = fields[len(fields)-1]
		}
		addrTok = strings.TrimSpace(addrTok)
		addrTok = strings.TrimPrefix(addrTok, "TCP")
		addrTok = strings.TrimSpace(addrTok)

		ip := ""
		if idx := strings.LastIndex(addrTok, ":"); idx >= 0 {
			ip = strings.TrimSpace(addrTok[:idx])
		}
		if ip == "*" {
			ip = "0.0.0.0"
		}

		addr := addrTok
		if ip != "" {
			addr = net.JoinHostPort(ip, strconv.Itoa(port))
		}

		listeners = append(listeners, PortListener{
			Network: "tcp",
			IP:      ip,
			Port:    port,
			Addr:    addr,
			PID:     pid,
			Name:    cmd,
		})
	}

	return listeners, DiagnosticsMeta{Source: "lsof"}
}

func scanLines(b []byte) []string {
	scanner := bufio.NewScanner(bytes.NewReader(b))
	out := []string{}
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		out = append(out, line)
	}
	return out
}
