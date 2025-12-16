package core

import (
	"bastion/models"
	"encoding/json"
	"fmt"
	"runtime"
	"sync"
	"time"
)

// ErrorLogger records error logs (in-memory KV-style store)
type ErrorLogger struct {
	logs      []*models.ErrorLog
	logsMap   map[int]*models.ErrorLog
	mu        sync.RWMutex
	maxLogs   int
	idCounter int
}

var ErrorLoggerInstance *ErrorLogger

func init() {
	ErrorLoggerInstance = &ErrorLogger{
		logs:    make([]*models.ErrorLog, 0, 100),
		logsMap: make(map[int]*models.ErrorLog),
		maxLogs: 100,
	}
}

// LogError records an error log entry
func (e *ErrorLogger) LogError(level, source, message, detail string, contextData map[string]interface{}) {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Capture stack trace (skip first 3 frames)
	stack := e.getStackTrace(3)

	// Serialize context
	contextJSON := ""
	if contextData != nil {
		if data, err := json.Marshal(contextData); err == nil {
			contextJSON = string(data)
		}
	}

	// LRU eviction
	if len(e.logs) >= e.maxLogs {
		oldLog := e.logs[0]
		delete(e.logsMap, oldLog.ID)
		e.logs = e.logs[1:]
	}

	e.idCounter++
	errorLog := &models.ErrorLog{
		ID:        e.idCounter,
		Timestamp: time.Now(),
		Level:     level,
		Source:    source,
		Message:   message,
		Detail:    detail,
		Stack:     stack,
		Context:   contextJSON,
	}

	e.logs = append(e.logs, errorLog)
	e.logsMap[errorLog.ID] = errorLog
}

// GetErrorLogs returns recent error logs (up to 100)
func (e *ErrorLogger) GetErrorLogs() []*models.ErrorLog {
	e.mu.RLock()
	defer e.mu.RUnlock()

	total := len(e.logs)
	result := make([]*models.ErrorLog, total)

	// Return in reverse order (latest first)
	for i := 0; i < total; i++ {
		result[i] = e.logs[total-1-i]
	}

	return result
}

// GetErrorLogByID returns a single error log by ID
func (e *ErrorLogger) GetErrorLogByID(id int) *models.ErrorLog {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.logsMap[id]
}

// ClearErrorLogs removes all error logs
func (e *ErrorLogger) ClearErrorLogs() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.logs = make([]*models.ErrorLog, 0, e.maxLogs)
	e.logsMap = make(map[int]*models.ErrorLog)
	e.idCounter = 0
}

// getStackTrace captures stack trace information
func (e *ErrorLogger) getStackTrace(skip int) string {
	const maxDepth = 10
	var stack string

	for i := skip; i < skip+maxDepth; i++ {
		pc, file, line, ok := runtime.Caller(i)
		if !ok {
			break
		}

		fn := runtime.FuncForPC(pc)
		funcName := "unknown"
		if fn != nil {
			funcName = fn.Name()
		}

		stack += fmt.Sprintf("%s:%d %s\n", file, line, funcName)
	}

	return stack
}

// Helper functions for different log levels

// LogErrorSimple records a simple error
func LogErrorSimple(source, message string) {
	ErrorLoggerInstance.LogError("ERROR", source, message, "", nil)
}

// LogErrorWithDetail records an error with details
func LogErrorWithDetail(source, message, detail string) {
	ErrorLoggerInstance.LogError("ERROR", source, message, detail, nil)
}

// LogErrorWithContext records an error with context
func LogErrorWithContext(source, message, detail string, context map[string]interface{}) {
	ErrorLoggerInstance.LogError("ERROR", source, message, detail, context)
}

// LogWarn records a warning
func LogWarn(source, message, detail string) {
	ErrorLoggerInstance.LogError("WARN", source, message, detail, nil)
}

// LogFatal records a fatal error
func LogFatal(source, message, detail string, context map[string]interface{}) {
	ErrorLoggerInstance.LogError("FATAL", source, message, detail, context)
}
