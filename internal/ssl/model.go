package ssl

import (
	"time"

	"github.com/google/uuid"
)

// Cert is the GORM model for the ssl_certs table.
type Cert struct {
	ID            uuid.UUID  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	DomainID      uuid.UUID  `gorm:"type:uuid;not null"                             json:"domain_id"`
	DomainName    string     `gorm:"not null"                                       json:"domain_name"`
	Status        string     `gorm:"not null;default:'pending'"                     json:"status"`
	CertPath      string     `gorm:"not null;default:''"                            json:"cert_path"`
	KeyPath       string     `gorm:"not null;default:''"                            json:"key_path"`
	IsWildcard    bool       `gorm:"not null;default:false"                         json:"is_wildcard"`
	RedirectHTTPS bool       `gorm:"not null;default:false"                         json:"redirect_https"`
	IssuedAt      *time.Time `                                                       json:"issued_at,omitempty"`
	ExpiresAt     *time.Time `                                                       json:"expires_at,omitempty"`
	LastRenewedAt *time.Time `                                                       json:"last_renewed_at,omitempty"`
	LastError     string     `gorm:"not null;default:''"                            json:"last_error,omitempty"`
	CreatedAt     time.Time  `                                                       json:"created_at"`
	UpdatedAt     time.Time  `                                                       json:"updated_at"`
}

func (Cert) TableName() string { return "ssl_certs" }

// ProvisionRequest is the body for POST /ssl/domain/:domain_id/provision.
type ProvisionRequest struct {
	IsWildcard bool `json:"is_wildcard"`
}

// RedirectRequest is the body for PATCH /ssl/:id/redirect.
type RedirectRequest struct {
	Enabled bool `json:"enabled"`
}

// DaysUntilExpiry returns the number of whole days until the cert expires.
// Returns -1 if ExpiresAt is nil.
func (c *Cert) DaysUntilExpiry() int {
	if c.ExpiresAt == nil {
		return -1
	}
	d := time.Until(*c.ExpiresAt)
	return int(d.Hours() / 24)
}
