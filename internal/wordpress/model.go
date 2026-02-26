package wordpress

import (
	"time"

	"github.com/google/uuid"
)

// WPInstall represents a WordPress installation on a hosted domain.
type WPInstall struct {
	ID         uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	DomainID   uuid.UUID `gorm:"type:uuid;not null;uniqueIndex"                 json:"domain_id"`
	DomainName string    `gorm:"not null"                                       json:"domain_name"`
	DBName     string    `gorm:"not null;default:''"                            json:"db_name"`
	DBUser     string    `gorm:"not null;default:''"                            json:"db_user"`
	DBPassword string    `gorm:"not null;default:''"                            json:"-"` // encrypted, never exposed
	WPVersion  string    `gorm:"not null;default:''"                            json:"wp_version"`
	WPURL      string    `gorm:"not null;default:''"                            json:"wp_url"`
	AdminUser  string    `gorm:"not null;default:''"                            json:"admin_user"`
	AdminEmail string    `gorm:"not null;default:''"                            json:"admin_email"`
	DebugMode  bool      `gorm:"not null;default:false"                         json:"debug_mode"`
	Status     string    `gorm:"not null;default:'installed'"                   json:"status"`
	LastError  string    `gorm:"not null;default:''"                            json:"last_error,omitempty"`
	CreatedAt  time.Time `                                                       json:"created_at"`
	UpdatedAt  time.Time `                                                       json:"updated_at"`
}

func (WPInstall) TableName() string { return "wordpress_installs" }

// InstallRequest holds the parameters for installing WordPress on a domain.
type InstallRequest struct {
	AdminUser  string `json:"admin_user"  binding:"required"`
	AdminPass  string `json:"admin_pass"  binding:"required"`
	AdminEmail string `json:"admin_email" binding:"required"`
}

// Plugin represents a WordPress plugin as returned by WP-CLI.
type Plugin struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Version string `json:"version"`
	Update  string `json:"update"`
}

// DebugRequest toggles WP_DEBUG on or off.
type DebugRequest struct {
	Enable bool `json:"enable"`
}

// ProvisionRequest is the shape of the SSL provision request (IsWildcard).
// (copied here to avoid a cross-package import — kept minimal)
type ProvisionRequest struct {
	IsWildcard bool `json:"is_wildcard"`
}
