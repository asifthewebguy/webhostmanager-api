package setup

import (
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Service struct {
	db            *gorm.DB
	encryptionKey string
}

func NewService(db *gorm.DB, encryptionKey string) *Service {
	return &Service{db: db, encryptionKey: encryptionKey}
}

// GetStatus returns the current setup state.
func (s *Service) GetStatus() (*StatusResponse, error) {
	var state State
	if err := s.db.First(&state, 1).Error; err != nil {
		return nil, fmt.Errorf("setup: get status: %w", err)
	}
	return &StatusResponse{
		IsComplete:  state.IsComplete,
		CurrentStep: state.CurrentStep,
		TotalSteps:  8,
	}, nil
}

// IsComplete returns true if setup has been finalized.
func (s *Service) IsComplete() bool {
	var state State
	if err := s.db.First(&state, 1).Error; err != nil {
		return false
	}
	return state.IsComplete
}

// AdvanceStep marks the given step as done (idempotent; only advances forward).
func (s *Service) AdvanceStep(step int) error {
	return s.db.Model(&State{}).Where("id = 1 AND current_step < ?", step).
		Update("current_step", step).Error
}

// Finalize marks setup as complete.
func (s *Service) Finalize() error {
	return s.db.Model(&State{}).Where("id = 1").
		Updates(map[string]any{"is_complete": true}).Error
}

// SaveSetting upserts a key-value pair in the settings table.
func (s *Service) SaveSetting(key, value string, encrypted bool) error {
	setting := Setting{Key: key, Value: value, IsEncrypted: encrypted}
	return s.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "key"}},
		DoUpdates: clause.AssignmentColumns([]string{"value", "is_encrypted", "updated_at"}),
	}).Create(&setting).Error
}

// GetSetting retrieves a setting value by key.
func (s *Service) GetSetting(key string) (string, bool, error) {
	var setting Setting
	if err := s.db.Where("key = ?", key).First(&setting).Error; err != nil {
		return "", false, err
	}
	return setting.Value, setting.IsEncrypted, nil
}
