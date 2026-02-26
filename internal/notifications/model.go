package notifications

import (
	"time"

	"github.com/google/uuid"
)

// Notification represents a single in-app notification.
type Notification struct {
	ID           uuid.UUID  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	Title        string     `gorm:"not null"                                       json:"title"`
	Message      string     `gorm:"not null"                                       json:"message"`
	Severity     string     `gorm:"not null;default:'info'"                        json:"severity"`
	EventType    string     `gorm:"not null;default:''"                            json:"event_type"`
	ResourceType string     `gorm:"not null;default:''"                            json:"resource_type"`
	ResourceID   string     `gorm:"not null;default:''"                            json:"resource_id"`
	ReadAt       *time.Time `                                                       json:"read_at"`
	CreatedAt    time.Time  `                                                       json:"created_at"`
}

func (Notification) TableName() string { return "notifications" }

// ChannelConfig stores per-event-type per-channel enabled flag.
type ChannelConfig struct {
	EventType string `gorm:"primaryKey"            json:"event_type"`
	Channel   string `gorm:"primaryKey"            json:"channel"`
	Enabled   bool   `gorm:"not null;default:true" json:"enabled"`
}

func (ChannelConfig) TableName() string { return "notification_channel_config" }

// UpdateChannelConfigRequest is the body for PUT /notifications/config.
type UpdateChannelConfigRequest struct {
	EventType string `json:"event_type" binding:"required"`
	Channel   string `json:"channel"    binding:"required"`
	Enabled   bool   `json:"enabled"`
}

// Severity constants.
const (
	SeverityInfo     = "info"
	SeverityWarning  = "warning"
	SeverityCritical = "critical"
)

// Channel constants.
const (
	ChannelInApp   = "in_app"
	ChannelEmail   = "email"
	ChannelSlack   = "slack"
	ChannelDiscord = "discord"
)
