package audit

import (
	"encoding/json"

	"github.com/rs/zerolog/log"
	"gorm.io/gorm"
)

type Service struct {
	db *gorm.DB
}

func NewService(db *gorm.DB) *Service {
	return &Service{db: db}
}

// Write persists an audit log entry asynchronously (fire-and-forget).
func (s *Service) Write(e Entry) {
	go func() {
		record := Log{
			Username:     e.Username,
			Role:         e.Role,
			Action:       e.Action,
			ResourceType: e.ResourceType,
			ResourceID:   e.ResourceID,
			IPAddress:    e.IPAddress,
			UserAgent:    e.UserAgent,
		}
		if e.UserID != nil {
			record.UserID = e.UserID
		}
		if e.Details != nil {
			if b, err := json.Marshal(e.Details); err == nil {
				record.Details = b
			}
		}
		if err := s.db.Create(&record).Error; err != nil {
			log.Error().Err(err).Str("action", e.Action).Msg("audit log write failed")
		}
	}()
}
