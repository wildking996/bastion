package core

import (
	"encoding/json"
	"fmt"
	"net"
	"path/filepath"
	"sort"
	"strings"
)

type PortAttempt struct {
	Network string `json:"network"`
	Host    string `json:"host"`
	Port    int    `json:"port"`
	Addr    string `json:"addr"`
}

type PortListener struct {
	Network string `json:"network"`
	Addr    string `json:"addr"`
	IP      string `json:"ip"`
	Port    int    `json:"port"`
	PID     int    `json:"pid"`
	Name    string `json:"name,omitempty"`
	Exe     string `json:"exe,omitempty"`
}

type ProcessOwner struct {
	PID  int    `json:"pid"`
	Exe  string `json:"exe,omitempty"`
	Name string `json:"name,omitempty"`
}

type PortConflict struct {
	MappingID string `json:"mapping_id"`
	LocalHost string `json:"local_host"`
	LocalPort int    `json:"local_port"`
	Type      string `json:"type"`
	Running   bool   `json:"running"`
}

type DiagnosticsMeta struct {
	Source string `json:"source"`
	Error  string `json:"error,omitempty"`
}

type PortInUseDetail struct {
	Attempt           PortAttempt     `json:"attempt"`
	ListenError       string          `json:"listen_error"`
	Listeners         []PortListener  `json:"listeners,omitempty"`
	Owners            []ProcessOwner  `json:"owners,omitempty"`
	InternalConflicts []PortConflict  `json:"internal_conflicts,omitempty"`
	Diag              DiagnosticsMeta `json:"diag"`
}

type PortInUseError struct {
	Detail PortInUseDetail
	Cause  error
}

func (e *PortInUseError) Error() string {
	if e == nil {
		return "port is already in use"
	}
	if e.Cause != nil {
		return e.Cause.Error()
	}
	return "port is already in use"
}

func (e *PortInUseError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

func (d PortInUseDetail) String() string {
	b, err := json.Marshal(d)
	if err != nil {
		return fmt.Sprintf("<port_in_use_detail marshal failed: %v>", err)
	}
	return string(b)
}

func DiagnosePortInUse(network, host string, port int) PortInUseDetail {
	addr := net.JoinHostPort(host, fmt.Sprintf("%d", port))
	attempt := PortAttempt{
		Network: network,
		Host:    host,
		Port:    port,
		Addr:    addr,
	}

	listeners, meta := queryPortListeners(network, port)
	owners := dedupeOwners(listeners)

	return PortInUseDetail{
		Attempt:   attempt,
		Listeners: listeners,
		Owners:    owners,
		Diag:      meta,
	}
}

func dedupeOwners(listeners []PortListener) []ProcessOwner {
	seen := map[int]ProcessOwner{}
	for _, l := range listeners {
		if l.PID <= 0 {
			continue
		}
		owner := seen[l.PID]
		owner.PID = l.PID
		if owner.Exe == "" && l.Exe != "" {
			owner.Exe = l.Exe
		}
		if owner.Name == "" {
			if l.Name != "" {
				owner.Name = l.Name
			} else if owner.Exe != "" {
				owner.Name = filepath.Base(owner.Exe)
			}
		}
		seen[l.PID] = owner
	}

	result := make([]ProcessOwner, 0, len(seen))
	for _, v := range seen {
		result = append(result, v)
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].PID != result[j].PID {
			return result[i].PID < result[j].PID
		}
		return strings.Compare(result[i].Exe, result[j].Exe) < 0
	})

	return result
}
