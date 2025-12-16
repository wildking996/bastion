package models

import "time"

// ErrorLog model for error logs
type ErrorLog struct {
	ID        int       `json:"id"`
	Timestamp time.Time `json:"timestamp"`
	Level     string    `json:"level"`   // ERROR, WARN, FATAL
	Source    string    `json:"source"`  // Error source (module name)
	Message   string    `json:"message"` // Error message
	Detail    string    `json:"detail"`  // Detailed information
	Stack     string    `json:"stack"`   // Stack trace
	Context   string    `json:"context"` // Context information (JSON format)
}
