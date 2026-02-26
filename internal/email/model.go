package email

import (
	"time"

	"github.com/google/uuid"
)

// EmailAccount represents a virtual email account on a hosted domain.
type EmailAccount struct {
	ID         uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	DomainID   uuid.UUID `gorm:"type:uuid;not null"                              json:"domain_id"`
	DomainName string    `gorm:"not null"                                        json:"domain_name"`
	Username   string    `gorm:"not null"                                        json:"username"`
	Email      string    `gorm:"not null"                                        json:"email"`
	Password   string    `gorm:"not null;default:''"                             json:"-"` // AES-encrypted
	QuotaMB    int       `gorm:"not null;default:500"                            json:"quota_mb"`
	Status     string    `gorm:"not null;default:'active'"                       json:"status"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func (EmailAccount) TableName() string { return "email_accounts" }

// EmailForwarder represents a Postfix virtual alias (forwarder or catch-all).
type EmailForwarder struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	DomainID    uuid.UUID `gorm:"type:uuid;not null"                              json:"domain_id"`
	DomainName  string    `gorm:"not null"                                        json:"domain_name"`
	Source      string    `gorm:"not null"                                        json:"source"`      // e.g. "info@example.com" or "@example.com"
	Destination string    `gorm:"not null"                                        json:"destination"` // comma-separated
	IsCatchAll  bool      `gorm:"not null;default:false"                          json:"is_catch_all"`
	CreatedAt   time.Time `json:"created_at"`
}

func (EmailForwarder) TableName() string { return "email_forwarders" }

// --- Request / Response DTOs ---

type CreateAccountRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
	QuotaMB  int    `json:"quota_mb"`
}

type ChangePasswordRequest struct {
	Password string `json:"password" binding:"required"`
}

type ChangeQuotaRequest struct {
	QuotaMB int `json:"quota_mb" binding:"required"`
}

type CreateForwarderRequest struct {
	Source      string `json:"source"      binding:"required"`
	Destination string `json:"destination" binding:"required"`
	IsCatchAll  bool   `json:"is_catch_all"`
}

type InstallMailServerRequest struct {
	SpamAssassin bool `json:"spam_assassin"`
	DKIM         bool `json:"dkim"`
}

// MailServerStatus reports which daemons are active on the managed server.
type MailServerStatus struct {
	Postfix      bool `json:"postfix"`
	Dovecot      bool `json:"dovecot"`
	SpamAssassin bool `json:"spam_assassin"`
	DKIM         bool `json:"dkim"`
	Installed    bool `json:"installed"` // true when Postfix + Dovecot are both active
}

// InstallProgress is the payload returned by the progress-poll endpoint.
type InstallProgress struct {
	Running bool     `json:"running"`
	Done    bool     `json:"done"`
	Error   string   `json:"error,omitempty"`
	Logs    []string `json:"logs"`
}

// EmailConfig holds display-only IMAP/SMTP connection details for a mailbox.
type EmailConfig struct {
	IMAPHost string `json:"imap_host"`
	IMAPPort int    `json:"imap_port"`
	SMTPHost string `json:"smtp_host"`
	SMTPPort int    `json:"smtp_port"`
	Username string `json:"username"` // full email address
}
