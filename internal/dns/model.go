package dns

import (
	"time"

	"github.com/google/uuid"
)

// DNSRecord represents a single DNS record synced from / managed via Cloudflare.
type DNSRecord struct {
	ID                 uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	DomainID           uuid.UUID `gorm:"type:uuid;not null"                             json:"domain_id"`
	DomainName         string    `gorm:"not null"                                       json:"domain_name"`
	CloudflareZoneID   string    `gorm:"not null;default:''"                            json:"-"`
	CloudflareRecordID string    `gorm:"not null;default:''"                            json:"cloudflare_record_id"`
	Type               string    `gorm:"not null"                                       json:"type"`
	Name               string    `gorm:"not null"                                       json:"name"`
	Content            string    `gorm:"not null"                                       json:"content"`
	TTL                int       `gorm:"not null;default:1"                             json:"ttl"`
	Proxied            bool      `gorm:"not null;default:false"                         json:"proxied"`
	Priority           int       `gorm:"not null;default:0"                             json:"priority"`
	CreatedAt          time.Time `                                                       json:"created_at"`
	UpdatedAt          time.Time `                                                       json:"updated_at"`
}

func (DNSRecord) TableName() string { return "dns_records" }

// CreateRecordRequest is the payload for POST /dns/domain/:id/records.
type CreateRecordRequest struct {
	Type     string `json:"type"    binding:"required"`
	Name     string `json:"name"    binding:"required"`
	Content  string `json:"content" binding:"required"`
	TTL      int    `json:"ttl"`
	Proxied  bool   `json:"proxied"`
	Priority int    `json:"priority"`
}

// UpdateRecordRequest is the payload for PUT /dns/records/:id.
type UpdateRecordRequest struct {
	Type     string `json:"type"    binding:"required"`
	Name     string `json:"name"    binding:"required"`
	Content  string `json:"content" binding:"required"`
	TTL      int    `json:"ttl"`
	Proxied  bool   `json:"proxied"`
	Priority int    `json:"priority"`
}

// ToggleProxyRequest is the payload for PATCH /dns/records/:id/proxy.
type ToggleProxyRequest struct {
	Proxied bool `json:"proxied"`
}
