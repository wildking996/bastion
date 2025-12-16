package handlers

import (
	"bastion/config"
	"bastion/core"
	"bastion/database"
	"bastion/models"
	"bastion/service"
	"bastion/state"
	"crypto/rand"
	"fmt"
	"math/big"
	"net/http"
	"runtime"
	"strconv"
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
		c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ok": true, "id": mapping.ID})
}

// DeleteMapping deletes a mapping
func DeleteMapping(c *gin.Context) {
	id := c.Param("id")

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

	logs, total := service.GlobalServices.Audit.GetHTTPLogs(page, pageSize)

	c.JSON(http.StatusOK, gin.H{
		"data":      logs,
		"page":      page,
		"page_size": pageSize,
		"total":     total,
	})
}

// GetHTTPLogDetail retrieves details for a single HTTP log entry
func GetHTTPLogDetail(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "Invalid ID"})
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

// GetMetrics gathers system metrics
func GetMetrics(c *gin.Context) {
	state.Global.RLock()
	sessionCount := len(state.Global.Sessions)

	// Sum total connections and traffic
	var totalConnections int32
	var totalBytesUp, totalBytesDown int64

	for _, session := range state.Global.Sessions {
		stats := session.GetStats()
		totalConnections += stats.ActiveConns
		totalBytesUp += stats.BytesUp
		totalBytesDown += stats.BytesDown
	}
	state.Global.RUnlock()

	// Get HTTP log totals
	_, httpLogCount := service.GlobalServices.Audit.GetHTTPLogs(1, 1)

	// Collect system resource usage
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	metrics := gin.H{
		"timestamp": time.Now().Unix(),
		"sessions": gin.H{
			"total":       sessionCount,
			"connections": totalConnections,
		},
		"traffic": gin.H{
			"bytes_up":   totalBytesUp,
			"bytes_down": totalBytesDown,
			"total":      totalBytesUp + totalBytesDown,
		},
		"http_logs": gin.H{
			"total": httpLogCount,
		},
		"system": gin.H{
			"goroutines":   runtime.NumGoroutine(),
			"memory_alloc": m.Alloc,
			"memory_total": m.TotalAlloc,
			"memory_sys":   m.Sys,
			"gc_runs":      m.NumGC,
		},
	}

	c.JSON(http.StatusOK, metrics)
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
