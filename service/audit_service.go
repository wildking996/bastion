package service

import (
	"bastion/core"
)

// AuditService handles audit log business logic
type AuditService struct {
	auditor *core.Auditor
}

// NewAuditService constructs an audit service
func NewAuditService(auditor *core.Auditor) *AuditService {
	return &AuditService{auditor: auditor}
}

// GetHTTPLogs returns paginated HTTP logs
func (s *AuditService) GetHTTPLogs(page, pageSize int) ([]*core.HTTPLog, int) {
	return s.auditor.GetHTTPLogs(page, pageSize)
}

// QueryHTTPLogs returns paginated HTTP logs filtered by optional criteria.
func (s *AuditService) QueryHTTPLogs(filter core.HTTPLogFilter, page, pageSize int) ([]*core.HTTPLog, int) {
	return s.auditor.QueryHTTPLogs(filter, page, pageSize)
}

// GetHTTPLogByID retrieves a single HTTP log by ID
func (s *AuditService) GetHTTPLogByID(id int) *core.HTTPLog {
	return s.auditor.GetHTTPLogByID(id)
}

// ClearHTTPLogs removes all HTTP logs
func (s *AuditService) ClearHTTPLogs() {
	s.auditor.ClearHTTPLogs()
}
