package core

import (
	"bastion/config"
	"log"
	"sync"
	"time"
)

// Auditor captures HTTP audit logs
type Auditor struct {
	running      bool
	httpLogs     []*HTTPLog
	httpLogsMap  map[int]*HTTPLog // ID to log quick lookup map
	httpMu       sync.RWMutex
	maxLogs      int
	logIDCounter int
	pairMatcher  *HTTPPairMatcher
	stateMu      sync.RWMutex
}

// HTTPLog represents an HTTP request/response log
type HTTPLog struct {
	ID              int       `json:"id"`
	Timestamp       time.Time `json:"timestamp"`
	ConnID          string    `json:"conn_id"`
	Method          string    `json:"method"`
	URL             string    `json:"url"`
	Host            string    `json:"host"`
	Protocol        string    `json:"protocol"`
	Request         string    `json:"request"`          // Full request (headers and body)
	Response        string    `json:"response"`         // Full response (headers and body)
	ResponseDecoded string    `json:"response_decoded"` // Decompressed response (if gzip)
	ReqSize         int       `json:"req_size"`         // Request size
	RespSize        int       `json:"resp_size"`        // Response size
	IsGzipped       bool      `json:"is_gzipped"`       // Whether response was gzip-compressed
	DurationMs      int64     `json:"duration_ms"`      // Request/response latency in ms
}

var AuditorInstance *Auditor

func init() {
	AuditorInstance = &Auditor{
		httpLogs:    make([]*HTTPLog, 0, config.Settings.MaxHTTPLogs),
		httpLogsMap: make(map[int]*HTTPLog),
		maxLogs:     config.Settings.MaxHTTPLogs,
	}

	// Create matcher
	AuditorInstance.pairMatcher = NewHTTPPairMatcher(func(httpLog *HTTPLog) {
		AuditorInstance.saveHTTPLog(httpLog)
	})
}

// Start begins auditing
func (a *Auditor) Start() {
	if !config.Settings.AuditEnabled {
		return
	}

	a.stateMu.Lock()
	if a.running {
		a.stateMu.Unlock()
		return
	}
	a.running = true
	a.stateMu.Unlock()

	go a.cleanupStalePairs()
}

// Stop halts auditing
func (a *Auditor) Stop() {
	a.stateMu.Lock()
	if !a.running {
		a.stateMu.Unlock()
		return
	}
	a.running = false
	a.stateMu.Unlock()
}

// isRunning safely reads running state
func (a *Auditor) isRunning() bool {
	a.stateMu.RLock()
	defer a.stateMu.RUnlock()
	return a.running
}

// LogHTTPMessage records a full HTTP message
func (a *Auditor) LogHTTPMessage(connID string, msg *HTTPMessage) {
	if !config.Settings.AuditEnabled {
		return
	}

	if msg.Type == HTTPRequest {
		a.pairMatcher.AddRequest(connID, msg)
	} else {
		a.pairMatcher.MatchResponse(connID, msg)
	}
}

// saveHTTPLog stores an HTTP log entry
func (a *Auditor) saveHTTPLog(httpLog *HTTPLog) {
	a.httpMu.Lock()
	defer a.httpMu.Unlock()

	// LRU eviction
	if len(a.httpLogs) >= a.maxLogs {
		oldLog := a.httpLogs[0]
		delete(a.httpLogsMap, oldLog.ID)
		a.httpLogs = a.httpLogs[1:]
	}

	a.logIDCounter++
	httpLog.ID = a.logIDCounter

	a.httpLogs = append(a.httpLogs, httpLog)
	a.httpLogsMap[httpLog.ID] = httpLog
}

// GetHTTPLogs returns paginated HTTP logs
func (a *Auditor) GetHTTPLogs(page, pageSize int) ([]*HTTPLog, int) {
	a.httpMu.RLock()
	defer a.httpMu.RUnlock()

	total := len(a.httpLogs)

	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}

	// Compute pagination
	start := (page - 1) * pageSize
	end := start + pageSize

	if start >= total {
		return []*HTTPLog{}, total
	}

	if end > total {
		end = total
	}

	// Return latest records (descending)
	result := make([]*HTTPLog, end-start)
	for i := 0; i < end-start; i++ {
		result[i] = a.httpLogs[total-1-start-i]
	}

	return result, total
}

// GetHTTPLogByID fetches a single log by ID
func (a *Auditor) GetHTTPLogByID(id int) *HTTPLog {
	a.httpMu.RLock()
	defer a.httpMu.RUnlock()

	// Use map for O(1) lookup
	return a.httpLogsMap[id]
}

// ClearHTTPLogs removes all HTTP logs
func (a *Auditor) ClearHTTPLogs() {
	a.httpMu.Lock()
	defer a.httpMu.Unlock()
	a.httpLogs = make([]*HTTPLog, 0, a.maxLogs)
	a.httpLogsMap = make(map[int]*HTTPLog)
	a.logIDCounter = 0
}

// cleanupStalePairs periodically clears unfinished HTTP pairs to avoid leaks
func (a *Auditor) cleanupStalePairs() {
	ticker := time.NewTicker(time.Duration(config.Settings.HTTPPairCleanupIntervalMinutes) * time.Minute)
	defer ticker.Stop()

	for {
		if !a.isRunning() {
			return
		}

		<-ticker.C
		maxAge := time.Duration(config.Settings.HTTPPairMaxAgeMinutes) * time.Minute
		cleaned := a.pairMatcher.CleanupStale(maxAge)

		if cleaned > 0 && config.Settings.LogLevel == "DEBUG" {
			log.Printf("Cleaned up %d stale HTTP pairs", cleaned)
		}
	}
}
