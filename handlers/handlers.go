package handlers

import (
	"bastion/config"
	"bastion/core"
	"bastion/database"
	"bastion/models"
	"bastion/service"
	"bastion/state"
	"bastion/version"
	"bytes"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// ShutdownManager manages shutdown confirmation codes
type ShutdownManager struct {
	code      string
	expiresAt time.Time
	mu        sync.RWMutex
}

var shutdownMgr = &ShutdownManager{}

// ListBastions lists all bastions
func ListBastions(c *gin.Context) {
	bastions, err := service.GlobalServices.Bastion.List()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": err.Error()})
		return
	}
	c.JSON(http.StatusOK, bastions)
}

// CreateBastion creates a bastion host
func CreateBastion(c *gin.Context) {
	var req models.BastionCreate
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
		return
	}

	bastion, err := service.GlobalServices.Bastion.Create(req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ok": true, "id": bastion.ID})
}

// UpdateBastion updates a bastion host
func UpdateBastion(c *gin.Context) {
	id := c.Param("id")
	bastionID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "Invalid bastion ID"})
		return
	}

	var req models.BastionCreate
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
		return
	}

	// Enforce immutability of bastion name: mappings reference bastions by name.
	existingBastion, err := service.GlobalServices.Bastion.Get(uint(bastionID))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
		return
	}
	if req.Name != "" && req.Name != existingBastion.Name {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "bastion name is immutable"})
		return
	}
	if req.Host != "" && req.Host != existingBastion.Host {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "bastion host is immutable"})
		return
	}
	if req.Port != 0 && req.Port != existingBastion.Port {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "bastion port is immutable"})
		return
	}

	// Disallow updates when referenced by any running mapping session.
	state.Global.RLock()
	running := make(map[string]bool, len(state.Global.Sessions))
	for mappingID := range state.Global.Sessions {
		running[mappingID] = true
	}
	state.Global.RUnlock()

	_, runningMappings, _, checkErr := service.GlobalServices.Bastion.CheckInUse(existingBastion.Name, running)
	if checkErr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": checkErr.Error()})
		return
	}
	if len(runningMappings) > 0 {
		c.JSON(http.StatusConflict, gin.H{
			"detail":           "bastion is referenced by running mapping(s); stop them before updating",
			"running_mappings": runningMappings,
		})
		return
	}

	bastion, err := service.GlobalServices.Bastion.Update(uint(bastionID), req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ok": true, "id": bastion.ID})
}

// DeleteBastion deletes a bastion host
func DeleteBastion(c *gin.Context) {
	id := c.Param("id")
	bastionID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "Invalid bastion ID"})
		return
	}

	if err := service.GlobalServices.Bastion.Delete(uint(bastionID)); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// ListMappings lists all mappings
func ListMappings(c *gin.Context) {
	mappings, err := service.GlobalServices.Mapping.List()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": err.Error()})
		return
	}
	c.JSON(http.StatusOK, mappings)
}

// CreateMapping creates a mapping
func CreateMapping(c *gin.Context) {
	var req models.MappingCreate
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
		return
	}

	mapping, err := service.GlobalServices.Mapping.Create(req)
	if err != nil {
		if errors.Is(err, service.ErrMappingAlreadyExists) {
			c.JSON(http.StatusConflict, gin.H{"detail": "mapping already exists; use PUT /api/mappings/:id to update (stopped only)"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ok": true, "id": mapping.ID})
}

// UpdateMapping updates a mapping when it is not running.
// Immutable fields: local/remote host/port and type.
func UpdateMapping(c *gin.Context) {
	id := c.Param("id")

	var req models.MappingCreate
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
		return
	}

	mapping, err := service.GlobalServices.Mapping.Update(id, req)
	if err != nil {
		if errors.Is(err, service.ErrMappingRunning) {
			c.JSON(http.StatusConflict, gin.H{"detail": "mapping is running; stop it before updating"})
			return
		}
		if err.Error() == "mapping not found: "+id {
			c.JSON(http.StatusNotFound, gin.H{"detail": err.Error()})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ok": true, "id": mapping.ID})
}

// DeleteMapping deletes a mapping
func DeleteMapping(c *gin.Context) {
	id := c.Param("id")

	// Disallow deleting an enabled (running) mapping to keep runtime state and DB data consistent.
	if state.Global.SessionExists(id) {
		c.JSON(http.StatusConflict, gin.H{"detail": "mapping is running; stop it before deleting"})
		return
	}

	if err := service.GlobalServices.Mapping.Delete(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// StartMapping starts a mapping
func StartMapping(c *gin.Context) {
	id := c.Param("id")

	if err := service.GlobalServices.Mapping.Start(id); err != nil {
		// Return different status codes based on the error type
		if err.Error() == "mapping is already running" {
			c.JSON(http.StatusOK, gin.H{"ok": true, "msg": "Already running"})
		} else if err.Error() == "mapping not found: "+id {
			c.JSON(http.StatusNotFound, gin.H{"detail": err.Error()})
		} else {
			c.JSON(http.StatusBadGateway, gin.H{"detail": err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// StopMapping stops a mapping
func StopMapping(c *gin.Context) {
	id := c.Param("id")

	if err := service.GlobalServices.Mapping.Stop(id); err != nil {
		c.JSON(http.StatusOK, gin.H{"ok": true, "msg": "Session not found or already stopped"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ok": true, "msg": "Session stopped"})
}

// GetStats returns mapping statistics
func GetStats(c *gin.Context) {
	statsMap := service.GlobalServices.Mapping.GetStats()

	result := make(map[string]gin.H)
	for id, s := range statsMap {
		result[id] = gin.H{
			"up_bytes":    s.BytesUp,
			"down_bytes":  s.BytesDown,
			"connections": s.ActiveConns,
		}
	}

	c.JSON(http.StatusOK, result)
}

// GetHTTPLogs retrieves paginated HTTP logs
func GetHTTPLogs(c *gin.Context) {
	page := 1
	pageSize := 20

	if pageStr := c.Query("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	if sizeStr := c.Query("page_size"); sizeStr != "" {
		if s, err := strconv.Atoi(sizeStr); err == nil && s > 0 {
			pageSize = s
		}
	}

	filter := core.HTTPLogFilter{}

	if q := strings.TrimSpace(c.Query("q")); q != "" {
		filter.Query = q
		if regexStr := c.Query("regex"); regexStr != "" {
			useRegex, err := strconv.ParseBool(regexStr)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"detail": "Invalid regex flag"})
				return
			}
			if useRegex {
				re, err := regexp.Compile(q)
				if err != nil {
					c.JSON(http.StatusBadRequest, gin.H{"detail": "Invalid regex pattern"})
					return
				}
				filter.QueryRegex = re
			}
		}
	}

	if method := strings.TrimSpace(c.Query("method")); method != "" {
		filter.Method = method
	}
	if host := strings.TrimSpace(c.Query("host")); host != "" {
		filter.Host = host
	}
	if urlStr := strings.TrimSpace(c.Query("url")); urlStr != "" {
		filter.URL = urlStr
	}
	if bastion := strings.TrimSpace(c.Query("bastion")); bastion != "" {
		filter.Bastion = bastion
	}
	if localPortStr := strings.TrimSpace(c.Query("local_port")); localPortStr != "" {
		p, err := strconv.Atoi(localPortStr)
		if err != nil || p <= 0 || p > 65535 {
			c.JSON(http.StatusBadRequest, gin.H{"detail": "Invalid local_port"})
			return
		}
		filter.LocalPort = &p
	}
	if statusStr := strings.TrimSpace(c.Query("status")); statusStr != "" {
		code, err := strconv.Atoi(statusStr)
		if err != nil || code < 0 {
			c.JSON(http.StatusBadRequest, gin.H{"detail": "Invalid status code"})
			return
		}
		filter.StatusCode = code
	}

	parseTime := func(value string) (*time.Time, error) {
		value = strings.TrimSpace(value)
		if value == "" {
			return nil, nil
		}
		if unix, err := strconv.ParseInt(value, 10, 64); err == nil {
			tm := time.Unix(unix, 0)
			return &tm, nil
		}
		tm, err := time.Parse(time.RFC3339, value)
		if err != nil {
			return nil, err
		}
		return &tm, nil
	}

	if sinceStr := c.Query("since"); sinceStr != "" {
		tm, err := parseTime(sinceStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"detail": "Invalid since timestamp"})
			return
		}
		filter.Since = tm
	}
	if untilStr := c.Query("until"); untilStr != "" {
		tm, err := parseTime(untilStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"detail": "Invalid until timestamp"})
			return
		}
		filter.Until = tm
	}

	logs, total := service.GlobalServices.Audit.QueryHTTPLogs(filter, page, pageSize)

	c.JSON(http.StatusOK, gin.H{
		"data":      logs,
		"page":      page,
		"page_size": pageSize,
		"total":     total,
	})
}

// GetHTTPLogDetail retrieves details for a single HTTP log entry.
//
// Optional query params:
// - part=request_header|request_body|response_header|response_body
// - decode=gzip (only for part=response_body)
func GetHTTPLogDetail(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "Invalid ID"})
		return
	}

	if partStr := strings.TrimSpace(c.Query("part")); partStr != "" {
		part := core.HTTPLogPart(partStr)

		opts := core.HTTPLogPartOptions{}
		if decodeStr := strings.TrimSpace(c.Query("decode")); decodeStr != "" {
			if !strings.EqualFold(decodeStr, "gzip") {
				c.JSON(http.StatusBadRequest, gin.H{"detail": "Invalid decode value"})
				return
			}
			opts.DecodeGzip = true
		}

		result, err := service.GlobalServices.Audit.GetHTTPLogPart(id, part, opts)
		if err != nil {
			if errors.Is(err, core.ErrHTTPLogNotFound) {
				c.JSON(http.StatusNotFound, gin.H{"detail": "Log not found"})
				return
			}
			if errors.Is(err, core.ErrInvalidHTTPLogPart) || errors.Is(err, core.ErrNotGzippedResponse) {
				c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
				return
			}
			if errors.Is(err, core.ErrGzipDecodeNotAllowed) {
				c.JSON(http.StatusBadRequest, gin.H{"detail": "decode is only supported for part=response_body"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"detail": err.Error()})
			return
		}

		c.JSON(http.StatusOK, result)
		return
	}

	log := service.GlobalServices.Audit.GetHTTPLogByID(id)
	if log == nil {
		c.JSON(http.StatusNotFound, gin.H{"detail": "Log not found"})
		return
	}

	c.JSON(http.StatusOK, log)
}

// ClearHTTPLogs removes all HTTP logs
func ClearHTTPLogs(c *gin.Context) {
	service.GlobalServices.Audit.ClearHTTPLogs()
	c.JSON(http.StatusOK, gin.H{"ok": true, "message": "HTTP logs cleared"})
}

// HealthCheck health endpoint
func HealthCheck(c *gin.Context) {
	state.Global.RLock()
	sessionCount := len(state.Global.Sessions)
	state.Global.RUnlock()

	// Check database connectivity
	sqlDB, err := database.DB.DB()
	dbHealthy := true
	if err != nil {
		dbHealthy = false
	} else {
		if err := sqlDB.Ping(); err != nil {
			dbHealthy = false
		}
	}

	health := gin.H{
		"status":        "healthy",
		"timestamp":     time.Now().Unix(),
		"sessions":      sessionCount,
		"db_healthy":    dbHealthy,
		"audit_enabled": config.Settings.AuditEnabled,
	}

	if !dbHealthy {
		health["status"] = "degraded"
		c.JSON(http.StatusServiceUnavailable, health)
		return
	}

	c.JSON(http.StatusOK, health)
}

type metricsSnapshot struct {
	timestamp        int64
	sessionCount     int
	totalConnections int32
	totalBytesUp     int64
	totalBytesDown   int64
	httpLogCount     int
	mem              runtime.MemStats
}

func collectMetricsSnapshot() metricsSnapshot {
	state.Global.RLock()
	sessionCount := len(state.Global.Sessions)

	var totalConnections int32
	var totalBytesUp, totalBytesDown int64

	for _, session := range state.Global.Sessions {
		stats := session.GetStats()
		totalConnections += stats.ActiveConns
		totalBytesUp += stats.BytesUp
		totalBytesDown += stats.BytesDown
	}
	state.Global.RUnlock()

	_, httpLogCount := service.GlobalServices.Audit.GetHTTPLogs(1, 1)

	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	return metricsSnapshot{
		timestamp:        time.Now().Unix(),
		sessionCount:     sessionCount,
		totalConnections: totalConnections,
		totalBytesUp:     totalBytesUp,
		totalBytesDown:   totalBytesDown,
		httpLogCount:     httpLogCount,
		mem:              mem,
	}
}

// GetMetrics gathers system metrics
func GetMetrics(c *gin.Context) {
	s := collectMetricsSnapshot()

	metrics := gin.H{
		"timestamp": s.timestamp,
		"audit": gin.H{
			"queue_len":      service.GlobalServices.Audit.AuditQueueLen(),
			"queue_capacity": service.GlobalServices.Audit.AuditQueueCap(),
			"dropped_total":  service.GlobalServices.Audit.AuditDroppedTotal(),
		},
		"ssh_pool": gin.H{
			"connections":        core.Pool.SSHPoolConnections(),
			"active_conns":       core.Pool.SSHPoolActiveConns(),
			"keepalive_failures": core.Pool.SSHKeepaliveFailuresTotal(),
			"idle_closed_total":  core.Pool.SSHIdleClosedTotal(),
		},
		"sessions": gin.H{
			"total":       s.sessionCount,
			"connections": s.totalConnections,
		},
		"traffic": gin.H{
			"bytes_up":   s.totalBytesUp,
			"bytes_down": s.totalBytesDown,
			"total":      s.totalBytesUp + s.totalBytesDown,
		},
		"http_logs": gin.H{
			"total": s.httpLogCount,
		},
		"system": gin.H{
			"goroutines":   runtime.NumGoroutine(),
			"memory_alloc": s.mem.Alloc,
			"memory_total": s.mem.TotalAlloc,
			"memory_sys":   s.mem.Sys,
			"gc_runs":      s.mem.NumGC,
		},
	}

	c.JSON(http.StatusOK, metrics)
}

func promLabelEscape(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	s = strings.ReplaceAll(s, "\n", "\\n")
	return s
}

// GetPrometheusMetrics writes Prometheus-formatted metrics to the HTTP response for scraping.
// It collects runtime and application metrics — including build info, SQLite connectivity and error counters, session and connection counts, traffic byte totals, HTTP log count, goroutine and memory statistics, and GC runs — and returns them using the Prometheus exposition content type.
func GetPrometheusMetrics(c *gin.Context) {
	s := collectMetricsSnapshot()

	var buf bytes.Buffer

	buf.WriteString("# HELP bastion_build_info Build information.\n")
	buf.WriteString("# TYPE bastion_build_info gauge\n")
	fmt.Fprintf(
		&buf,
		"bastion_build_info{version=\"%s\",commit=\"%s\",build_time=\"%s\"} 1\n",
		promLabelEscape(version.Version),
		promLabelEscape(version.CommitHash),
		promLabelEscape(version.BuildTime),
	)

	buf.WriteString("# HELP bastion_sqlite_up SQLite connectivity (1=up, 0=down).\n")
	buf.WriteString("# TYPE bastion_sqlite_up gauge\n")
	if database.SQLiteUp(c.Request.Context()) {
		buf.WriteString("bastion_sqlite_up 1\n")
	} else {
		buf.WriteString("bastion_sqlite_up 0\n")
	}

	buf.WriteString("# HELP bastion_sqlite_busy_errors_total Total SQLite busy errors observed.\n")
	buf.WriteString("# TYPE bastion_sqlite_busy_errors_total counter\n")
	fmt.Fprintf(&buf, "bastion_sqlite_busy_errors_total %d\n", database.SQLiteBusyErrorsTotal())

	buf.WriteString("# HELP bastion_sqlite_locked_errors_total Total SQLite locked errors observed.\n")
	buf.WriteString("# TYPE bastion_sqlite_locked_errors_total counter\n")
	fmt.Fprintf(&buf, "bastion_sqlite_locked_errors_total %d\n", database.SQLiteLockedErrorsTotal())

	buf.WriteString("# HELP bastion_sessions_total Number of configured/running sessions.\n")
	buf.WriteString("# TYPE bastion_sessions_total gauge\n")
	fmt.Fprintf(&buf, "bastion_sessions_total %d\n", s.sessionCount)

	buf.WriteString("# HELP bastion_sessions_connections Active connections across sessions.\n")
	buf.WriteString("# TYPE bastion_sessions_connections gauge\n")
	fmt.Fprintf(&buf, "bastion_sessions_connections %d\n", s.totalConnections)

	buf.WriteString("# HELP bastion_http_audit_queue_capacity HTTP audit queue capacity.\n")
	buf.WriteString("# TYPE bastion_http_audit_queue_capacity gauge\n")
	fmt.Fprintf(&buf, "bastion_http_audit_queue_capacity %d\n", service.GlobalServices.Audit.AuditQueueCap())

	buf.WriteString("# HELP bastion_http_audit_queue_len Current number of buffered audit messages.\n")
	buf.WriteString("# TYPE bastion_http_audit_queue_len gauge\n")
	fmt.Fprintf(&buf, "bastion_http_audit_queue_len %d\n", service.GlobalServices.Audit.AuditQueueLen())

	buf.WriteString("# HELP bastion_http_audit_dropped_total Total audit messages dropped due to backpressure.\n")
	buf.WriteString("# TYPE bastion_http_audit_dropped_total counter\n")
	fmt.Fprintf(&buf, "bastion_http_audit_dropped_total %d\n", service.GlobalServices.Audit.AuditDroppedTotal())

	buf.WriteString("# HELP bastion_ssh_pool_connections Number of pooled SSH client connections.\n")
	buf.WriteString("# TYPE bastion_ssh_pool_connections gauge\n")
	fmt.Fprintf(&buf, "bastion_ssh_pool_connections %d\n", core.Pool.SSHPoolConnections())

	buf.WriteString("# HELP bastion_ssh_pool_active_conns Active tunneled connections opened via the SSH pool.\n")
	buf.WriteString("# TYPE bastion_ssh_pool_active_conns gauge\n")
	fmt.Fprintf(&buf, "bastion_ssh_pool_active_conns %d\n", core.Pool.SSHPoolActiveConns())

	buf.WriteString("# HELP bastion_ssh_pool_keepalive_failures_total Total pooled SSH keepalive probe failures.\n")
	buf.WriteString("# TYPE bastion_ssh_pool_keepalive_failures_total counter\n")
	fmt.Fprintf(&buf, "bastion_ssh_pool_keepalive_failures_total %d\n", core.Pool.SSHKeepaliveFailuresTotal())

	buf.WriteString("# HELP bastion_ssh_pool_idle_closed_total Total pooled SSH connections closed due to idleness or eviction.\n")
	buf.WriteString("# TYPE bastion_ssh_pool_idle_closed_total counter\n")
	fmt.Fprintf(&buf, "bastion_ssh_pool_idle_closed_total %d\n", core.Pool.SSHIdleClosedTotal())

	buf.WriteString("# HELP bastion_traffic_bytes_up_total Total uploaded bytes.\n")
	buf.WriteString("# TYPE bastion_traffic_bytes_up_total counter\n")
	fmt.Fprintf(&buf, "bastion_traffic_bytes_up_total %d\n", s.totalBytesUp)

	buf.WriteString("# HELP bastion_traffic_bytes_down_total Total downloaded bytes.\n")
	buf.WriteString("# TYPE bastion_traffic_bytes_down_total counter\n")
	fmt.Fprintf(&buf, "bastion_traffic_bytes_down_total %d\n", s.totalBytesDown)

	buf.WriteString("# HELP bastion_http_logs_total Total HTTP audit log entries kept in memory.\n")
	buf.WriteString("# TYPE bastion_http_logs_total gauge\n")
	fmt.Fprintf(&buf, "bastion_http_logs_total %d\n", s.httpLogCount)

	buf.WriteString("# HELP bastion_go_goroutines Number of goroutines.\n")
	buf.WriteString("# TYPE bastion_go_goroutines gauge\n")
	fmt.Fprintf(&buf, "bastion_go_goroutines %d\n", runtime.NumGoroutine())

	buf.WriteString("# HELP bastion_memory_alloc_bytes Bytes of allocated heap objects.\n")
	buf.WriteString("# TYPE bastion_memory_alloc_bytes gauge\n")
	fmt.Fprintf(&buf, "bastion_memory_alloc_bytes %d\n", s.mem.Alloc)

	buf.WriteString("# HELP bastion_memory_sys_bytes Bytes obtained from the OS.\n")
	buf.WriteString("# TYPE bastion_memory_sys_bytes gauge\n")
	fmt.Fprintf(&buf, "bastion_memory_sys_bytes %d\n", s.mem.Sys)

	buf.WriteString("# HELP bastion_gc_runs_total Number of completed GC cycles.\n")
	buf.WriteString("# TYPE bastion_gc_runs_total counter\n")
	fmt.Fprintf(&buf, "bastion_gc_runs_total %d\n", s.mem.NumGC)

	c.Data(http.StatusOK, "text/plain; version=0.0.4; charset=utf-8", buf.Bytes())
}

// GetErrorLogs returns recent error logs
func GetErrorLogs(c *gin.Context) {
	logs := core.ErrorLoggerInstance.GetErrorLogs()
	c.JSON(http.StatusOK, logs)
}

// ClearErrorLogs wipes error logs
func ClearErrorLogs(c *gin.Context) {
	core.ErrorLoggerInstance.ClearErrorLogs()
	c.JSON(http.StatusOK, gin.H{"ok": true, "message": "Error logs cleared"})
}

// GenerateShutdownCode creates a shutdown confirmation code
func GenerateShutdownCode(c *gin.Context) {
	shutdownMgr.mu.Lock()
	defer shutdownMgr.mu.Unlock()

	// Generate a 6-digit random number
	n, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "Failed to generate code"})
		return
	}

	shutdownMgr.code = fmt.Sprintf("%06d", n.Int64())
	shutdownMgr.expiresAt = time.Now().Add(5 * time.Minute) // 5-minute expiration

	c.JSON(http.StatusOK, gin.H{
		"code":       shutdownMgr.code,
		"expires_at": shutdownMgr.expiresAt.Unix(),
	})
}

// VerifyAndShutdown validates the confirmation code and shuts the app down
func VerifyAndShutdown(c *gin.Context) {
	var req struct {
		Code string `json:"code" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "Invalid request"})
		return
	}

	shutdownMgr.mu.RLock()
	storedCode := shutdownMgr.code
	expiresAt := shutdownMgr.expiresAt
	shutdownMgr.mu.RUnlock()

	// Ensure a code was issued
	if storedCode == "" {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "No shutdown code generated. Please generate one first."})
		return
	}

	// Check expiration
	if time.Now().After(expiresAt) {
		shutdownMgr.mu.Lock()
		shutdownMgr.code = ""
		shutdownMgr.mu.Unlock()
		c.JSON(http.StatusBadRequest, gin.H{"detail": "Shutdown code expired. Please generate a new one."})
		return
	}

	// Validate the code value
	if req.Code != storedCode {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "Invalid shutdown code"})
		return
	}

	// Clear the stored code
	shutdownMgr.mu.Lock()
	shutdownMgr.code = ""
	shutdownMgr.mu.Unlock()

	// Respond success
	c.JSON(http.StatusOK, gin.H{"ok": true, "message": "Shutdown initiated"})

	// Perform graceful shutdown in the background
	go func() {
		time.Sleep(500 * time.Millisecond) // Give clients time to receive the response
		core.LogErrorWithDetail("System", "Shutdown requested via API", "User initiated shutdown with confirmation code")
		// Send interrupt to trigger graceful shutdown
		// Note: main.go must initialize the global shutdown channel
		if shutdownChan != nil {
			shutdownChan <- true
		}
	}()
}

// Global shutdown channel (must be initialized in main.go)
var shutdownChan chan bool

// SetShutdownChannel sets the shutdown channel
func SetShutdownChannel(ch chan bool) {
	shutdownChan = ch
}
