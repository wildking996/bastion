//go:build windows

package core

import (
	"encoding/binary"
	"errors"
	"net"
	"path/filepath"
	"strconv"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	mibTcpStateListen     = 2
	tcpTableOwnerPidAll   = 5
	getExtendedTcpTableOK = 0
)

type mibTCPRowOwnerPID struct {
	State      uint32
	LocalAddr  uint32
	LocalPort  uint32
	RemoteAddr uint32
	RemotePort uint32
	OwningPid  uint32
}

type mibTCPTableOwnerPID struct {
	NumEntries uint32
	Table      [1]mibTCPRowOwnerPID
}

type mibTCP6RowOwnerPID struct {
	LocalAddr     [16]byte
	LocalScopeID  uint32
	LocalPort     uint32
	RemoteAddr    [16]byte
	RemoteScopeID uint32
	RemotePort    uint32
	State         uint32
	OwningPid     uint32
}

type mibTCP6TableOwnerPID struct {
	NumEntries uint32
	Table      [1]mibTCP6RowOwnerPID
}

var (
	modiphlpapi             = windows.NewLazySystemDLL("iphlpapi.dll")
	procGetExtendedTcpTable = modiphlpapi.NewProc("GetExtendedTcpTable")
)

func getExtendedTcpTable(buf *byte, size *uint32, order bool, af uint32) error {
	var sorted uintptr
	if order {
		sorted = 1
	}

	r1, _, _ := procGetExtendedTcpTable.Call(
		uintptr(unsafe.Pointer(buf)),
		uintptr(unsafe.Pointer(size)),
		sorted,
		uintptr(af),
		uintptr(tcpTableOwnerPidAll),
		0,
	)
	if r1 == getExtendedTcpTableOK {
		return nil
	}
	return syscall.Errno(r1)
}

func ntohs(v uint16) uint16 {
	return (v << 8) | (v >> 8)
}

func portFromDWORD(v uint32) int {
	p := ntohs(uint16(v))
	return int(p)
}

func ipFromDWORD(addr uint32) string {
	var b [4]byte
	binary.LittleEndian.PutUint32(b[:], addr)
	return net.IPv4(b[0], b[1], b[2], b[3]).String()
}

func getProcessImagePath(pid uint32) string {
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return ""
	}
	defer windows.CloseHandle(h)

	buf := make([]uint16, 32768)
	size := uint32(len(buf))
	if err := windows.QueryFullProcessImageName(h, 0, &buf[0], &size); err != nil {
		return ""
	}
	return windows.UTF16ToString(buf[:size])
}

func queryPortListeners(network string, port int) ([]PortListener, DiagnosticsMeta) {
	if network != "tcp" {
		return nil, DiagnosticsMeta{Source: "winapi", Error: "unsupported network"}
	}

	listeners := make([]PortListener, 0, 8)

	// IPv4
	{
		var size uint32
		err := getExtendedTcpTable(nil, &size, false, windows.AF_INET)
		if err != nil && !errors.Is(err, windows.ERROR_INSUFFICIENT_BUFFER) {
			return nil, DiagnosticsMeta{Source: "winapi", Error: err.Error()}
		}
		buf := make([]byte, size)
		err = getExtendedTcpTable(&buf[0], &size, false, windows.AF_INET)
		if err != nil {
			return nil, DiagnosticsMeta{Source: "winapi", Error: err.Error()}
		}

		table := (*mibTCPTableOwnerPID)(unsafe.Pointer(&buf[0]))
		rows := unsafe.Slice(&table.Table[0], table.NumEntries)
		for _, r := range rows {
			if r.State != mibTcpStateListen {
				continue
			}
			p := portFromDWORD(r.LocalPort)
			if p != port {
				continue
			}
			ip := ipFromDWORD(r.LocalAddr)
			pid := int(r.OwningPid)
			exe := getProcessImagePath(r.OwningPid)
			name := ""
			if exe != "" {
				name = filepath.Base(exe)
			}
			listeners = append(listeners, PortListener{
				Network: "tcp",
				IP:      ip,
				Port:    p,
				Addr:    net.JoinHostPort(ip, strconv.Itoa(p)),
				PID:     pid,
				Name:    name,
				Exe:     exe,
			})
		}
	}

	// IPv6
	{
		var size uint32
		err := getExtendedTcpTable(nil, &size, false, windows.AF_INET6)
		if err != nil && !errors.Is(err, windows.ERROR_INSUFFICIENT_BUFFER) {
			return listeners, DiagnosticsMeta{Source: "winapi", Error: err.Error()}
		}
		buf := make([]byte, size)
		err = getExtendedTcpTable(&buf[0], &size, false, windows.AF_INET6)
		if err != nil {
			return listeners, DiagnosticsMeta{Source: "winapi", Error: err.Error()}
		}

		table := (*mibTCP6TableOwnerPID)(unsafe.Pointer(&buf[0]))
		rows := unsafe.Slice(&table.Table[0], table.NumEntries)
		for _, r := range rows {
			if r.State != mibTcpStateListen {
				continue
			}
			p := portFromDWORD(r.LocalPort)
			if p != port {
				continue
			}
			ip := net.IP(r.LocalAddr[:]).String()
			pid := int(r.OwningPid)
			exe := getProcessImagePath(r.OwningPid)
			name := ""
			if exe != "" {
				name = filepath.Base(exe)
			}
			listeners = append(listeners, PortListener{
				Network: "tcp",
				IP:      ip,
				Port:    p,
				Addr:    net.JoinHostPort(ip, strconv.Itoa(p)),
				PID:     pid,
				Name:    name,
				Exe:     exe,
			})
		}
	}

	return listeners, DiagnosticsMeta{Source: "winapi"}
}
