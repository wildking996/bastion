package main

import (
	"fmt"
	"log"
	"os"
)

// setupLogging configures file-only logging with a single rotated history file.
// It returns the opened log file so callers can close it on shutdown.
func setupLogging(path string) (*os.File, error) {
	if path == "" {
		return nil, fmt.Errorf("log file path is empty")
	}

	// Remove existing history to keep only one backup
	_ = os.Remove(path + ".1")

	// Rotate current log to .1 if present
	if _, err := os.Stat(path); err == nil {
		if err := os.Rename(path, path+".1"); err != nil {
			return nil, fmt.Errorf("failed to rotate existing log: %w", err)
		}
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file %s: %w", path, err)
	}

	log.SetOutput(f)
	return f, nil
}
