package health

import (
	"context"
	"time"

	"gorm.io/gorm"
)

type Status struct {
	API      string `json:"api"`
	Database string `json:"database"`
}

type Service struct {
	db *gorm.DB
}

func NewService(db *gorm.DB) *Service {
	return &Service{db: db}
}

func (s *Service) Check() Status {
	status := Status{API: "ok"}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	sqlDB, err := s.db.DB()
	if err != nil {
		status.Database = "error: " + err.Error()
		return status
	}

	if err := sqlDB.PingContext(ctx); err != nil {
		status.Database = "error: " + err.Error()
		return status
	}

	status.Database = "ok"
	return status
}
