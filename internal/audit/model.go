package audit

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Log is the GORM model for the audit_logs table.
type Log struct {
	ID           uuid.UUID       `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	UserID       *uuid.UUID      `gorm:"type:uuid"                                      json:"user_id,omitempty"`
	Username     string          `                                                       json:"username"`
	Role         string          `                                                       json:"role"`
	Action       string          `gorm:"not null"                                       json:"action"`
	ResourceType string          `                                                       json:"resource_type"`
	ResourceID   string          `                                                       json:"resource_id"`
	Details      json.RawMessage `gorm:"type:jsonb"                                     json:"details,omitempty"`
	IPAddress    string          `                                                       json:"ip_address"`
	UserAgent    string          `                                                       json:"user_agent"`
	CreatedAt    time.Time       `gorm:"not null"                                       json:"created_at"`
}

func (Log) TableName() string { return "audit_logs" }

// Entry is used when writing a log entry from handlers.
type Entry struct {
	UserID       *uuid.UUID
	Username     string
	Role         string
	Action       string
	ResourceType string
	ResourceID   string
	Details      map[string]any
	IPAddress    string
	UserAgent    string
}
