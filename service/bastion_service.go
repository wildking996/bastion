package service

import (
	"bastion/models"
	"fmt"

	"gorm.io/gorm"
)

// BastionService handles bastion business logic
type BastionService struct {
	db *gorm.DB
}

// NewBastionService constructs a bastion service
func NewBastionService(db *gorm.DB) *BastionService {
	return &BastionService{db: db}
}

// List lists all bastions
func (s *BastionService) List() ([]models.Bastion, error) {
	var bastions []models.Bastion
	if err := s.db.Find(&bastions).Error; err != nil {
		return nil, fmt.Errorf("failed to list bastions: %w", err)
	}
	return bastions, nil
}

// ListPage returns bastions with pagination.
func (s *BastionService) ListPage(page, pageSize int) ([]models.Bastion, int64, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}

	var total int64
	if err := s.db.Model(&models.Bastion{}).Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count bastions: %w", err)
	}

	var bastions []models.Bastion
	offset := (page - 1) * pageSize
	if err := s.db.Order("id desc").Offset(offset).Limit(pageSize).Find(&bastions).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to list bastions: %w", err)
	}
	return bastions, total, nil
}

// Get fetches a bastion by ID
func (s *BastionService) Get(id uint) (*models.Bastion, error) {
	var bastion models.Bastion
	if err := s.db.First(&bastion, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("bastion not found: %d", id)
		}
		return nil, fmt.Errorf("failed to get bastion: %w", err)
	}
	return &bastion, nil
}

// Create creates a new bastion
func (s *BastionService) Create(req models.BastionCreate) (*models.Bastion, error) {
	// Normalize inputs
	req.Normalize()

	// Build bastion model
	bastion := models.Bastion{
		Name:           req.Name,
		Host:           req.Host,
		Port:           req.Port,
		Username:       req.Username,
		Password:       req.Password,
		PkeyPath:       req.PkeyPath,
		PkeyPassphrase: req.PkeyPassphrase,
	}

	// Apply defaults
	if bastion.Port == 0 {
		bastion.Port = 22
	}

	if bastion.Name == "" {
		bastion.Name = fmt.Sprintf("%s:%d", bastion.Host, bastion.Port)
	}

	// Persist to database
	if err := s.db.Create(&bastion).Error; err != nil {
		return nil, fmt.Errorf("failed to create bastion: %w", err)
	}

	return &bastion, nil
}

// Update updates a bastion
func (s *BastionService) Update(id uint, req models.BastionCreate) (*models.Bastion, error) {
	// Look up bastion
	bastion, err := s.Get(id)
	if err != nil {
		return nil, err
	}

	// Normalize inputs
	req.Normalize()

	// Update fields (name/host/port are immutable because mappings reference bastions by name)
	bastion.Username = req.Username
	bastion.Password = req.Password
	bastion.PkeyPath = req.PkeyPath
	bastion.PkeyPassphrase = req.PkeyPassphrase

	// Persist updates
	if err := s.db.Save(bastion).Error; err != nil {
		return nil, fmt.Errorf("failed to update bastion: %w", err)
	}

	return bastion, nil
}

// Delete removes a bastion
func (s *BastionService) Delete(id uint) error {
	// Look up bastion
	bastion, err := s.Get(id)
	if err != nil {
		return err
	}

	// Ensure no mappings reference it
	var count int64
	s.db.Model(&models.Mapping{}).
		Where("chain_json LIKE ?", "%\""+bastion.Name+"\"%").
		Count(&count)

	if count > 0 {
		return fmt.Errorf("cannot delete bastion '%s': used by %d mapping(s)", bastion.Name, count)
	}

	// Delete bastion
	if err := s.db.Delete(bastion).Error; err != nil {
		return fmt.Errorf("failed to delete bastion: %w", err)
	}

	return nil
}

// CheckInUse checks whether a bastion is referenced (including running sessions)
func (s *BastionService) CheckInUse(bastionName string, runningSessions map[string]bool) (inUse bool, runningMappings []string, totalMappings int64, err error) {
	// Count all mappings that reference the bastion
	var count int64
	s.db.Model(&models.Mapping{}).
		Where("chain_json LIKE ?", "%\""+bastionName+"\"%").
		Count(&count)

	if count == 0 {
		return false, nil, 0, nil
	}

	// Check for running sessions
	var mappings []models.Mapping
	if err := s.db.Where("chain_json LIKE ?", "%\""+bastionName+"\"%").Find(&mappings).Error; err != nil {
		return false, nil, 0, fmt.Errorf("failed to query mappings: %w", err)
	}

	var runningMappingIDs []string
	for _, mapping := range mappings {
		if runningSessions[mapping.ID] {
			chainNames := mapping.GetChain()
			for _, name := range chainNames {
				if name == bastionName {
					runningMappingIDs = append(runningMappingIDs, mapping.ID)
					break
				}
			}
		}
	}

	return true, runningMappingIDs, count, nil
}
