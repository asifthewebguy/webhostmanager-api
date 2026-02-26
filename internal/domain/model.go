package domain

import (
	"time"

	"github.com/google/uuid"
)

// Domain is the GORM model for the domains table.
type Domain struct {
	ID         uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	Name       string    `gorm:"uniqueIndex;not null"                           json:"name"`
	Status     string    `gorm:"not null;default:'active'"                      json:"status"`
	DiskUsedMB int64     `gorm:"not null;default:0"                             json:"disk_used_mb"`
	WebRoot    string    `gorm:"not null;default:''"                            json:"web_root"`
	CreatedAt  time.Time `                                                       json:"created_at"`
	UpdatedAt  time.Time `                                                       json:"updated_at"`
}

func (Domain) TableName() string { return "domains" }

// AddRequest is the body for POST /api/v1/domains.
type AddRequest struct {
	Name string `json:"name" binding:"required"`
}
