//go:build windows

package main

import (
	"log"
	"syscall"
	"time"
)

func waitForParentExit(pid int, timeout time.Duration) {
	if pid <= 0 {
		return
	}
	ms := uint32(timeout / time.Millisecond)
	if ms == 0 {
		ms = 1
	}

	h, err := syscall.OpenProcess(syscall.SYNCHRONIZE, false, uint32(pid))
	if err != nil {
		// If we can't open the process handle, assume it's already gone.
		return
	}
	defer func() {
		if err := syscall.CloseHandle(h); err != nil {
			log.Printf("update-helper: close parent handle failed: %v", err)
		}
	}()

	log.Printf("update-helper: waiting parent pid %d to exit (timeout=%s)", pid, timeout)
	status, err := syscall.WaitForSingleObject(h, ms)
	if err != nil {
		log.Printf("update-helper: wait parent pid %d failed: %v", pid, err)
		return
	}
	switch status {
	case syscall.WAIT_OBJECT_0:
		log.Printf("update-helper: parent pid %d exited", pid)
	case syscall.WAIT_TIMEOUT:
		log.Printf("update-helper: parent pid %d still running after %s; continuing", pid, timeout)
	default:
		log.Printf("update-helper: wait parent pid %d returned status=%d; continuing", pid, status)
	}
}
