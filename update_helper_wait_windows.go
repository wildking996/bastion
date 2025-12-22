//go:build windows

package main

import "time"

func waitForParentExit(pid int, timeout time.Duration) {
	// On Windows we rely on the executable lock to naturally delay replacement/restart.
}
