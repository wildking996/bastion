//go:build !windows

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
	deadline := time.Now().Add(timeout)
	for {
		if time.Now().After(deadline) {
			log.Printf("update-helper: parent pid %d still running after %s; continuing", pid, timeout)
			return
		}
		if err := syscall.Kill(pid, 0); err != nil {
			log.Printf("update-helper: parent pid %d exited", pid)
			return
		}
		time.Sleep(250 * time.Millisecond)
	}
}
