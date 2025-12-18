package service

import (
	"bastion/core"
	"bastion/models"
	"bastion/state"
	"errors"
	"fmt"

	"gorm.io/gorm"
)

var ErrMappingAlreadyExists = errors.New("mapping already exists")
var ErrMappingRunning = errors.New("mapping is running")

// MappingService handles mapping business logic
type MappingService struct {
	db         *gorm.DB
	state      *state.AppState
	bastionSvc *BastionService
}

// NewMappingService constructs a mapping service
func NewMappingService(db *gorm.DB, appState *state.AppState, bastionSvc *BastionService) *MappingService {
	return &MappingService{
		db:         db,
		state:      appState,
		bastionSvc: bastionSvc,
	}
}

// List returns all mappings (including runtime status)
func (s *MappingService) List() ([]models.MappingRead, error) {
	var mappings []models.Mapping
	if err := s.db.Find(&mappings).Error; err != nil {
		return nil, fmt.Errorf("failed to list mappings: %w", err)
	}

	// Collect running sessions
	s.state.RLock()
	runningIDs := make(map[string]bool)
	for id := range s.state.Sessions {
		runningIDs[id] = true
	}
	s.state.RUnlock()

	// Build response objects
	result := make([]models.MappingRead, len(mappings))
	for i, m := range mappings {
		result[i] = models.MappingRead{
			ID:         m.ID,
			LocalHost:  m.LocalHost,
			LocalPort:  m.LocalPort,
			RemoteHost: m.RemoteHost,
			RemotePort: m.RemotePort,
			Chain:      m.GetChain(),
			AllowCIDRs: m.GetAllowCIDRs(),
			DenyCIDRs:  m.GetDenyCIDRs(),
			Type:       m.Type,
			AutoStart:  m.AutoStart,
			Running:    runningIDs[m.ID],
		}
	}

	return result, nil
}

// Get fetches a mapping by ID
func (s *MappingService) Get(id string) (*models.Mapping, error) {
	var mapping models.Mapping
	if err := s.db.First(&mapping, "id = ?", id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("mapping not found: %s", id)
		}
		return nil, fmt.Errorf("failed to get mapping: %w", err)
	}
	return &mapping, nil
}

// GetWithStatus fetches a mapping and its runtime status
func (s *MappingService) GetWithStatus(id string) (*models.Mapping, bool, *core.SessionStats, error) {
	mapping, err := s.Get(id)
	if err != nil {
		return nil, false, nil, err
	}

	running := s.state.SessionExists(id)
	var stats *core.SessionStats

	if running {
		if session, exists := s.state.GetSession(id); exists {
			s := session.GetStats()
			stats = &s
		}
	}

	return mapping, running, stats, nil
}

// Create creates a mapping.
// This endpoint is intentionally NOT an upsert: if the mapping already exists, it returns ErrMappingAlreadyExists.
func (s *MappingService) Create(req models.MappingCreate) (*models.Mapping, error) {
	// Normalize inputs
	req.Normalize()

	// Generate ID
	id := req.ID
	if id == "" {
		id = fmt.Sprintf("%s:%d", req.LocalHost, req.LocalPort)
	}

	// Apply defaults
	if req.LocalHost == "" {
		req.LocalHost = "127.0.0.1"
	}

	if req.Type == "" {
		req.Type = "tcp"
	}

	switch req.Type {
	case "tcp", "socks5", "http":
	default:
		return nil, fmt.Errorf("invalid mapping type: %s", req.Type)
	}

	// Ensure mapping does not already exist (no upsert)
	var existing models.Mapping
	if err := s.db.First(&existing, "id = ?", id).Error; err == nil {
		return nil, ErrMappingAlreadyExists
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("failed to check mapping existence: %w", err)
	}

	// Create a new mapping
	mapping := models.Mapping{
		ID:        id,
		LocalHost: req.LocalHost,
		LocalPort: req.LocalPort,
		Type:      req.Type,
		AutoStart: req.AutoStart,
	}
	if req.Type == "tcp" {
		mapping.RemoteHost = req.RemoteHost
		mapping.RemotePort = req.RemotePort
	} else {
		mapping.RemoteHost = "0.0.0.0"
		mapping.RemotePort = 0
	}
	mapping.SetChain(req.Chain)
	mapping.SetAllowCIDRs(req.AllowCIDRs)
	mapping.SetDenyCIDRs(req.DenyCIDRs)

	if _, err := core.NewIPAccessControl(req.AllowCIDRs, req.DenyCIDRs); err != nil {
		return nil, err
	}

	// Persist to database
	if err := s.db.Create(&mapping).Error; err != nil {
		return nil, fmt.Errorf("failed to create mapping: %w", err)
	}

	return &mapping, nil
}

// Update updates a mapping when it is not running.
// Immutable fields: local/remote host/port and type.
func (s *MappingService) Update(id string, req models.MappingCreate) (*models.Mapping, error) {
	// Disallow updates while running
	if s.state.SessionExists(id) {
		return nil, ErrMappingRunning
	}

	req.Normalize()

	// Load existing mapping
	mapping, err := s.Get(id)
	if err != nil {
		return nil, err
	}

	// ID must match (mapping identity is stable)
	if req.ID != "" && req.ID != id {
		return nil, fmt.Errorf("mapping id is immutable")
	}

	// Enforce immutable address fields (both local and remote)
	if req.LocalHost != "" && req.LocalHost != mapping.LocalHost {
		return nil, fmt.Errorf("local_host is immutable")
	}
	if req.LocalPort != 0 && req.LocalPort != mapping.LocalPort {
		return nil, fmt.Errorf("local_port is immutable")
	}

	// Type is immutable (changing it changes runtime semantics)
	if req.Type != "" && req.Type != mapping.Type {
		return nil, fmt.Errorf("type is immutable")
	}

	// Remote host/port are immutable as well (only meaningful for TCP; still enforce if provided).
	if req.RemoteHost != "" && req.RemoteHost != mapping.RemoteHost {
		return nil, fmt.Errorf("remote_host is immutable")
	}
	if req.RemotePort != 0 && req.RemotePort != mapping.RemotePort {
		return nil, fmt.Errorf("remote_port is immutable")
	}

	// For TCP mappings, require remote host/port to be present so we can enforce immutability.
	if mapping.Type == "tcp" {
		if req.RemoteHost == "" || req.RemotePort == 0 {
			return nil, fmt.Errorf("remote_host and remote_port are required for tcp mapping update")
		}
	}

	// Allowed updates
	mapping.AutoStart = req.AutoStart
	mapping.SetChain(req.Chain)
	mapping.SetAllowCIDRs(req.AllowCIDRs)
	mapping.SetDenyCIDRs(req.DenyCIDRs)

	if _, err := core.NewIPAccessControl(req.AllowCIDRs, req.DenyCIDRs); err != nil {
		return nil, err
	}

	if err := s.db.Save(mapping).Error; err != nil {
		return nil, fmt.Errorf("failed to update mapping: %w", err)
	}

	return mapping, nil
}

// Delete removes a mapping (stopping it first if running)
func (s *MappingService) Delete(id string) error {
	// Stop session if running
	s.state.RemoveAndStopSession(id)

	// Delete mapping
	if err := s.db.Delete(&models.Mapping{}, "id = ?", id).Error; err != nil {
		return fmt.Errorf("failed to delete mapping: %w", err)
	}

	return nil
}

// Start starts a mapping session
func (s *MappingService) Start(id string) error {
	// Ensure not already running
	if s.state.SessionExists(id) {
		return fmt.Errorf("mapping is already running")
	}

	// Load mapping configuration
	mapping, err := s.Get(id)
	if err != nil {
		return err
	}

	// Build bastion chain
	chainNames := mapping.GetChain()
	var bastions []models.Bastion

	// Lookup bastions if a chain is provided
	if len(chainNames) > 0 {
		// Query bastions in batch
		var allBastions []models.Bastion
		if err := s.db.Where("name IN ?", chainNames).Find(&allBastions).Error; err != nil {
			return fmt.Errorf("failed to query bastions: %w", err)
		}

		// Build name -> bastion map
		bastionMap := make(map[string]models.Bastion)
		for _, b := range allBastions {
			bastionMap[b.Name] = b
		}

		// Build ordered bastion list according to chain
		bastions = make([]models.Bastion, 0, len(chainNames))
		for _, name := range chainNames {
			bastion, exists := bastionMap[name]
			if !exists {
				return fmt.Errorf("bastion '%s' in chain not found", name)
			}
			bastions = append(bastions, bastion)
		}
	}
	// If no bastions configured, empty slice indicates direct connection

	// Create session
	var session core.Session
	switch mapping.Type {
	case "socks5":
		session = core.NewSocks5Session(mapping, bastions)
	case "http":
		session = core.NewHTTPProxySession(mapping, bastions)
	default:
		session = core.NewTunnelSession(mapping, bastions)
	}

	// Start session
	if err := session.Start(); err != nil {
		return fmt.Errorf("failed to start session: %w", err)
	}

	// Add to state
	s.state.AddSession(id, session)

	return nil
}

// Stop stops a mapping session
func (s *MappingService) Stop(id string) error {
	if !s.state.SessionExists(id) {
		return fmt.Errorf("mapping is not running")
	}

	s.state.RemoveAndStopSession(id)
	return nil
}

// GetStats returns stats for all sessions
func (s *MappingService) GetStats() map[string]core.SessionStats {
	s.state.RLock()
	defer s.state.RUnlock()

	stats := make(map[string]core.SessionStats)
	for id, session := range s.state.Sessions {
		stats[id] = session.GetStats()
	}

	return stats
}

// GetSessionIDs lists all running session IDs
func (s *MappingService) GetSessionIDs() []string {
	s.state.RLock()
	defer s.state.RUnlock()

	ids := make([]string, 0, len(s.state.Sessions))
	for id := range s.state.Sessions {
		ids = append(ids, id)
	}

	return ids
}

// IsRunning checks whether a mapping is running
func (s *MappingService) IsRunning(id string) bool {
	return s.state.SessionExists(id)
}

// StartAutoStartMappings starts all mappings marked as auto-start
func (s *MappingService) StartAutoStartMappings() error {
	var mappings []models.Mapping
	if err := s.db.Where("auto_start = ?", true).Find(&mappings).Error; err != nil {
		return fmt.Errorf("failed to query auto-start mappings: %w", err)
	}

	for _, mapping := range mappings {
		if err := s.Start(mapping.ID); err != nil {
			// Log error but continue with other mappings
			fmt.Printf("Failed to auto-start mapping %s: %v\n", mapping.ID, err)
		}
	}

	return nil
}
