package handlers

import (
	"bastion/config"
	"bastion/core"
	"bastion/database"
	"bastion/models"
	"bastion/service"
	"bastion/state"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

func ListBastionsV2(c *gin.Context) {
	bastions, err := service.GlobalServices.Bastion.List()
	if err != nil {
		errV2(c, http.StatusInternalServerError, CodeInternal, "Failed to list bastions", err.Error())
		return
	}
	okV2(c, bastions)
}

func CreateBastionV2(c *gin.Context) {
	var req models.BastionCreate
	if err := c.ShouldBindJSON(&req); err != nil {
		errV2(c, http.StatusBadRequest, CodeInvalidRequest, "Invalid request", err.Error())
		return
	}

	bastion, err := service.GlobalServices.Bastion.Create(req)
	if err != nil {
		errV2(c, http.StatusBadRequest, CodeInvalidRequest, "Failed to create bastion", err.Error())
		return
	}

	okV2(c, gin.H{"id": bastion.ID})
}

func UpdateBastionV2(c *gin.Context) {
	id := c.Param("id")
	bastionID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		errV2(c, http.StatusBadRequest, CodeInvalidRequest, "Invalid bastion id", "invalid bastion id")
		return
	}

	var req models.BastionCreate
	if err := c.ShouldBindJSON(&req); err != nil {
		errV2(c, http.StatusBadRequest, CodeInvalidRequest, "Invalid request", err.Error())
		return
	}

	existingBastion, err := service.GlobalServices.Bastion.Get(uint(bastionID))
	if err != nil {
		errV2(c, http.StatusBadRequest, CodeInvalidRequest, "Failed to load bastion", err.Error())
		return
	}
	if req.Name != "" && req.Name != existingBastion.Name {
		errV2(c, http.StatusBadRequest, CodeInvalidRequest, "bastion name is immutable", "bastion name is immutable")
		return
	}
	if req.Host != "" && req.Host != existingBastion.Host {
		errV2(c, http.StatusBadRequest, CodeInvalidRequest, "bastion host is immutable", "bastion host is immutable")
		return
	}
	if req.Port != 0 && req.Port != existingBastion.Port {
		errV2(c, http.StatusBadRequest, CodeInvalidRequest, "bastion port is immutable", "bastion port is immutable")
		return
	}

	state.Global.RLock()
	running := make(map[string]bool, len(state.Global.Sessions))
	for mappingID := range state.Global.Sessions {
		running[mappingID] = true
	}
	state.Global.RUnlock()

	_, runningMappings, _, checkErr := service.GlobalServices.Bastion.CheckInUse(existingBastion.Name, running)
	if checkErr != nil {
		errV2(c, http.StatusInternalServerError, CodeInternal, "Failed to check bastion usage", checkErr.Error())
		return
	}
	if len(runningMappings) > 0 {
		respondV2(c, http.StatusConflict, CodeConflict, "Bastion is referenced by running mapping(s)", gin.H{
			"running_mappings": runningMappings,
		})
		return
	}

	bastion, err := service.GlobalServices.Bastion.Update(uint(bastionID), req)
	if err != nil {
		errV2(c, http.StatusBadRequest, CodeInvalidRequest, "Failed to update bastion", err.Error())
		return
	}
	okV2(c, gin.H{"id": bastion.ID})
}

func DeleteBastionV2(c *gin.Context) {
	id := c.Param("id")
	bastionID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		errV2(c, http.StatusBadRequest, CodeInvalidRequest, "Invalid bastion id", "invalid bastion id")
		return
	}

	if err := service.GlobalServices.Bastion.Delete(uint(bastionID)); err != nil {
		errV2(c, http.StatusBadRequest, CodeInvalidRequest, "Failed to delete bastion", err.Error())
		return
	}
	okV2(c, gin.H{"ok": true})
}

func ListMappingsV2(c *gin.Context) {
	mappings, err := service.GlobalServices.Mapping.List()
	if err != nil {
		errV2(c, http.StatusInternalServerError, CodeInternal, "Failed to list mappings", err.Error())
		return
	}
	okV2(c, mappings)
}

func CreateMappingV2(c *gin.Context) {
	var req models.MappingCreate
	if err := c.ShouldBindJSON(&req); err != nil {
		errV2(c, http.StatusBadRequest, CodeInvalidRequest, "Invalid request", err.Error())
		return
	}

	mapping, err := service.GlobalServices.Mapping.Create(req)
	if err != nil {
		if errors.Is(err, service.ErrMappingAlreadyExists) {
			errV2(c, http.StatusConflict, CodeConflict, "Mapping already exists", err.Error())
			return
		}
		errV2(c, http.StatusBadRequest, CodeInvalidRequest, "Failed to create mapping", err.Error())
		return
	}

	okV2(c, gin.H{"id": mapping.ID})
}

func UpdateMappingV2(c *gin.Context) {
	id := c.Param("id")

	var req models.MappingCreate
	if err := c.ShouldBindJSON(&req); err != nil {
		errV2(c, http.StatusBadRequest, CodeInvalidRequest, "Invalid request", err.Error())
		return
	}

	mapping, err := service.GlobalServices.Mapping.Update(id, req)
	if err != nil {
		if errors.Is(err, service.ErrMappingRunning) {
			errV2(c, http.StatusConflict, CodeConflict, "Mapping is running", err.Error())
			return
		}
		if errors.Is(err, service.ErrMappingNotFound) {
			errV2(c, http.StatusNotFound, CodeNotFound, "Mapping not found", err.Error())
			return
		}
		errV2(c, http.StatusBadRequest, CodeInvalidRequest, "Failed to update mapping", err.Error())
		return
	}

	okV2(c, gin.H{"id": mapping.ID})
}

func DeleteMappingV2(c *gin.Context) {
	id := c.Param("id")
	if state.Global.SessionExists(id) {
		errV2(c, http.StatusConflict, CodeConflict, "Mapping is running", "mapping is running")
		return
	}

	if err := service.GlobalServices.Mapping.Delete(id); err != nil {
		errV2(c, http.StatusInternalServerError, CodeInternal, "Failed to delete mapping", err.Error())
		return
	}
	okV2(c, gin.H{"ok": true})
}

func StartMappingV2(c *gin.Context) {
	id := c.Param("id")

	if err := service.GlobalServices.Mapping.Start(id); err != nil {
		if errors.Is(err, service.ErrMappingAlreadyRunning) {
			okV2(c, gin.H{"ok": true, "already_running": true})
			return
		}
		if errors.Is(err, service.ErrMappingNotFound) {
			errV2(c, http.StatusNotFound, CodeNotFound, "Mapping not found", err.Error())
			return
		}
		var be *core.BastionError
		if errors.As(err, &be) && be.Code == http.StatusConflict {
			addr := ""
			if m, getErr := service.GlobalServices.Mapping.Get(id); getErr == nil {
				addr = net.JoinHostPort(m.LocalHost, strconv.Itoa(m.LocalPort))
			}
			respondV2(c, http.StatusConflict, CodeResourceBusy, "Local address is already in use", gin.H{
				"detail": err.Error(),
				"addr":   addr,
			})
			return
		}
		// Keep status aligned with v1 (which uses 502 for other start failures)
		errV2(c, http.StatusBadGateway, CodeBadGateway, "Failed to start mapping", err.Error())
		return
	}

	okV2(c, gin.H{"ok": true})
}

func StopMappingV2(c *gin.Context) {
	id := c.Param("id")
	if err := service.GlobalServices.Mapping.Stop(id); err != nil {
		okV2(c, gin.H{"ok": true, "stopped": false, "reason": "not_found_or_already_stopped"})
		return
	}
	okV2(c, gin.H{"ok": true, "stopped": true})
}

func GetStatsV2(c *gin.Context) {
	statsMap := service.GlobalServices.Mapping.GetStats()

	result := make(map[string]gin.H)
	for id, s := range statsMap {
		result[id] = gin.H{
			"up_bytes":    s.BytesUp,
			"down_bytes":  s.BytesDown,
			"connections": s.ActiveConns,
		}
	}

	okV2(c, result)
}

func GetHTTPLogsV2(c *gin.Context) {
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
				errV2(c, http.StatusBadRequest, CodeInvalidRequest, "Invalid regex flag", "invalid regex flag")
				return
			}
			if useRegex {
				re, err := regexp.Compile(q)
				if err != nil {
					errV2(c, http.StatusBadRequest, CodeInvalidRequest, "Invalid regex pattern", "invalid regex pattern")
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
			errV2(c, http.StatusBadRequest, CodeInvalidRequest, "Invalid local_port", "invalid local_port")
			return
		}
		filter.LocalPort = &p
	}
	if statusStr := strings.TrimSpace(c.Query("status")); statusStr != "" {
		code, err := strconv.Atoi(statusStr)
		if err != nil || code < 0 {
			errV2(c, http.StatusBadRequest, CodeInvalidRequest, "Invalid status code", "invalid status")
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
			errV2(c, http.StatusBadRequest, CodeInvalidRequest, "Invalid since timestamp", "invalid since")
			return
		}
		filter.Since = tm
	}
	if untilStr := c.Query("until"); untilStr != "" {
		tm, err := parseTime(untilStr)
		if err != nil {
			errV2(c, http.StatusBadRequest, CodeInvalidRequest, "Invalid until timestamp", "invalid until")
			return
		}
		filter.Until = tm
	}

	logs, total := service.GlobalServices.Audit.QueryHTTPLogs(filter, page, pageSize)
	okV2(c, gin.H{
		"items":     logs,
		"page":      page,
		"page_size": pageSize,
		"total":     total,
	})
}

func GetHTTPLogDetailV2(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		errV2(c, http.StatusBadRequest, CodeInvalidRequest, "Invalid id", "invalid id")
		return
	}

	if partStr := strings.TrimSpace(c.Query("part")); partStr != "" {
		part := core.HTTPLogPart(partStr)

		opts := core.HTTPLogPartOptions{}
		if decodeStr := strings.TrimSpace(c.Query("decode")); decodeStr != "" {
			if !strings.EqualFold(decodeStr, "gzip") {
				errV2(c, http.StatusBadRequest, CodeInvalidRequest, "Invalid decode value", "invalid decode")
				return
			}
			opts.DecodeGzip = true
		}

		result, err := service.GlobalServices.Audit.GetHTTPLogPart(id, part, opts)
		if err != nil {
			if errors.Is(err, core.ErrHTTPLogNotFound) {
				errV2(c, http.StatusNotFound, CodeNotFound, "Log not found", "log not found")
				return
			}
			if errors.Is(err, core.ErrInvalidHTTPLogPart) || errors.Is(err, core.ErrNotGzippedResponse) {
				errV2(c, http.StatusBadRequest, CodeInvalidRequest, "Invalid request", err.Error())
				return
			}
			if errors.Is(err, core.ErrGzipDecodeNotAllowed) {
				errV2(c, http.StatusBadRequest, CodeInvalidRequest, "Invalid request", "decode is only supported for part=response_body")
				return
			}
			errV2(c, http.StatusInternalServerError, CodeInternal, "Failed to fetch log detail", err.Error())
			return
		}

		okV2(c, result)
		return
	}

	log := service.GlobalServices.Audit.GetHTTPLogByID(id)
	if log == nil {
		errV2(c, http.StatusNotFound, CodeNotFound, "Log not found", "log not found")
		return
	}

	okV2(c, log)
}

func ClearHTTPLogsV2(c *gin.Context) {
	service.GlobalServices.Audit.ClearHTTPLogs()
	okV2(c, gin.H{"ok": true})
}

func GetErrorLogsV2(c *gin.Context) {
	okV2(c, core.ErrorLoggerInstance.GetErrorLogs())
}

func ClearErrorLogsV2(c *gin.Context) {
	core.ErrorLoggerInstance.ClearErrorLogs()
	okV2(c, gin.H{"ok": true})
}

func GenerateShutdownCodeV2(c *gin.Context) {
	shutdownMgr.mu.Lock()
	defer shutdownMgr.mu.Unlock()

	n, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		errV2(c, http.StatusInternalServerError, CodeInternal, "Failed to generate code", "Failed to generate code")
		return
	}

	shutdownMgr.code = fmt.Sprintf("%06d", n.Int64())
	shutdownMgr.expiresAt = time.Now().Add(5 * time.Minute)

	okV2(c, gin.H{"code": shutdownMgr.code, "expires_at": shutdownMgr.expiresAt.Unix()})
}

func VerifyAndShutdownV2(c *gin.Context) {
	var req struct {
		Code string `json:"code" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		errV2(c, http.StatusBadRequest, CodeInvalidRequest, "Invalid request", "Invalid request")
		return
	}

	shutdownMgr.mu.RLock()
	storedCode := shutdownMgr.code
	expiresAt := shutdownMgr.expiresAt
	shutdownMgr.mu.RUnlock()

	if storedCode == "" {
		errV2(c, http.StatusBadRequest, CodeInvalidRequest, "No shutdown code generated", "No shutdown code generated")
		return
	}

	if time.Now().After(expiresAt) {
		shutdownMgr.mu.Lock()
		shutdownMgr.code = ""
		shutdownMgr.mu.Unlock()
		errV2(c, http.StatusBadRequest, CodeInvalidRequest, "Shutdown code expired", "Shutdown code expired")
		return
	}

	if req.Code != storedCode {
		errV2(c, http.StatusBadRequest, CodeInvalidRequest, "Invalid shutdown code", "Invalid shutdown code")
		return
	}

	shutdownMgr.mu.Lock()
	shutdownMgr.code = ""
	shutdownMgr.mu.Unlock()

	okV2(c, gin.H{"ok": true})

	go func() {
		time.Sleep(500 * time.Millisecond)
		core.LogErrorWithDetail("System", "Shutdown requested via API", "User initiated shutdown with confirmation code")
		if shutdownChan != nil {
			shutdownChan <- true
		}
	}()
}

func HealthCheckV2(c *gin.Context) {
	state.Global.RLock()
	sessionCount := len(state.Global.Sessions)
	state.Global.RUnlock()

	sqlDB, err := database.DB.DB()
	dbHealthy := true
	if err != nil {
		dbHealthy = false
	} else if err := sqlDB.Ping(); err != nil {
		dbHealthy = false
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
		respondV2(c, http.StatusServiceUnavailable, CodeInternal, "Service degraded", health)
		return
	}

	okV2(c, health)
}

func GetMetricsV2(c *gin.Context) {
	// reuse existing helper
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

	okV2(c, metrics)
}
