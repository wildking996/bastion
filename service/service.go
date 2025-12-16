package service

import (
	"bastion/core"
	"bastion/state"

	"gorm.io/gorm"
)

// Services is the global service container
type Services struct {
	Bastion *BastionService
	Mapping *MappingService
	Audit   *AuditService
}

// GlobalServices is the global service instance
var GlobalServices *Services

// InitServices initializes all services
func InitServices(db *gorm.DB, appState *state.AppState, auditor *core.Auditor) {
	bastionSvc := NewBastionService(db)
	mappingSvc := NewMappingService(db, appState, bastionSvc)
	auditSvc := NewAuditService(auditor)

	GlobalServices = &Services{
		Bastion: bastionSvc,
		Mapping: mappingSvc,
		Audit:   auditSvc,
	}
}
