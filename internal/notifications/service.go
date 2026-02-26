package notifications

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/smtp"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/asifthewebguy/webhostmanager-api/pkg/crypto"
)

// Service handles notification creation, delivery, and management.
type Service struct {
	db     *gorm.DB
	encKey string
}

func NewService(db *gorm.DB, encKey string) *Service {
	return &Service{db: db, encKey: encKey}
}

// --- Core CRUD ---

// Create inserts a notification into the DB and fans out to enabled delivery channels.
func (s *Service) Create(n Notification) error {
	if err := s.db.Create(&n).Error; err != nil {
		return err
	}
	go s.fanoutNotification(n)
	return nil
}

// ListAll returns notifications from the last 90 days (newest first, max 200).
func (s *Service) ListAll() ([]Notification, error) {
	var records []Notification
	err := s.db.Where("created_at > NOW() - INTERVAL '90 days'").
		Order("created_at DESC").Limit(200).Find(&records).Error
	return records, err
}

// ListUnread returns unread notifications (newest first, max 50).
func (s *Service) ListUnread() ([]Notification, error) {
	var records []Notification
	err := s.db.Where("read_at IS NULL").
		Order("created_at DESC").Limit(50).Find(&records).Error
	return records, err
}

// UnreadCount returns the number of unread notifications.
func (s *Service) UnreadCount() (int64, error) {
	var count int64
	err := s.db.Model(&Notification{}).Where("read_at IS NULL").Count(&count).Error
	return count, err
}

// MarkRead marks a single notification as read.
func (s *Service) MarkRead(id string) error {
	now := time.Now()
	return s.db.Model(&Notification{}).Where("id = ? AND read_at IS NULL", id).
		Update("read_at", now).Error
}

// MarkAllRead marks all unread notifications as read.
func (s *Service) MarkAllRead() error {
	now := time.Now()
	return s.db.Model(&Notification{}).Where("read_at IS NULL").
		Update("read_at", now).Error
}

// --- Channel config ---

// GetChannelConfig returns all channel config rows.
func (s *Service) GetChannelConfig() ([]ChannelConfig, error) {
	var cfg []ChannelConfig
	err := s.db.Order("event_type, channel").Find(&cfg).Error
	return cfg, err
}

// UpdateChannelConfig upserts a channel config row.
func (s *Service) UpdateChannelConfig(req UpdateChannelConfigRequest) error {
	return s.db.Save(&ChannelConfig{
		EventType: req.EventType,
		Channel:   req.Channel,
		Enabled:   req.Enabled,
	}).Error
}

// isChannelEnabled checks if a channel is enabled for a given event type.
func (s *Service) isChannelEnabled(eventType, channel string) bool {
	var cfg ChannelConfig
	if err := s.db.Where("event_type = ? AND channel = ?", eventType, channel).First(&cfg).Error; err != nil {
		return false
	}
	return cfg.Enabled
}

// --- Delivery ---

// fanoutNotification delivers a notification to all enabled external channels.
// in_app is already persisted to DB; we skip it here.
func (s *Service) fanoutNotification(n Notification) {
	if n.EventType == "" {
		return
	}
	if s.isChannelEnabled(n.EventType, ChannelEmail) {
		if err := s.sendEmail(n); err != nil {
			// best-effort; log silently
			_ = err
		}
	}
	if s.isChannelEnabled(n.EventType, ChannelSlack) {
		if err := s.sendSlack(n); err != nil {
			_ = err
		}
	}
	if s.isChannelEnabled(n.EventType, ChannelDiscord) {
		if err := s.sendDiscord(n); err != nil {
			_ = err
		}
	}
}

// sendEmail delivers a notification via SMTP using settings from the DB.
func (s *Service) sendEmail(n Notification) error {
	host := s.settingVal("smtp.host")
	port := s.settingVal("smtp.port")
	from := s.settingVal("smtp.from")
	user := s.settingVal("smtp.user")
	if host == "" || port == "" || from == "" {
		return fmt.Errorf("smtp not configured")
	}

	password, err := s.settingDecrypted("smtp.password")
	if err != nil || password == "" {
		return fmt.Errorf("smtp password not configured")
	}

	to := s.adminEmail()
	if to == "" {
		return fmt.Errorf("no admin email found")
	}

	subject := fmt.Sprintf("[WebHostManager] %s", n.Title)
	body := fmt.Sprintf("Subject: %s\r\nFrom: %s\r\nTo: %s\r\nContent-Type: text/plain; charset=utf-8\r\n\r\n%s\r\n\r\nSeverity: %s\r\nEvent: %s",
		subject, from, to, n.Message, n.Severity, n.EventType)

	auth := smtp.PlainAuth("", user, password, host)
	return smtp.SendMail(host+":"+port, auth, from, []string{to}, []byte(body))
}

// sendSlack delivers a notification to a Slack webhook.
func (s *Service) sendSlack(n Notification) error {
	webhook, err := s.settingDecrypted("notifications.slack_webhook")
	if err != nil || webhook == "" {
		return fmt.Errorf("slack webhook not configured")
	}
	text := fmt.Sprintf("*[%s] %s*\n%s", strings.ToUpper(n.Severity), n.Title, n.Message)
	payload, _ := json.Marshal(map[string]string{"text": text})
	resp, err := http.Post(webhook, "application/json", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("slack returned status %d", resp.StatusCode)
	}
	return nil
}

// sendDiscord delivers a notification to a Discord webhook.
func (s *Service) sendDiscord(n Notification) error {
	webhook, err := s.settingDecrypted("notifications.discord_webhook")
	if err != nil || webhook == "" {
		return fmt.Errorf("discord webhook not configured")
	}
	content := fmt.Sprintf("**[%s] %s**\n%s", strings.ToUpper(n.Severity), n.Title, n.Message)
	payload, _ := json.Marshal(map[string]string{"content": content})
	resp, err := http.Post(webhook, "application/json", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("discord returned status %d", resp.StatusCode)
	}
	return nil
}

// SendTestChannel sends a test notification to the given channel directly,
// bypassing channel config. Used by the test endpoint.
func (s *Service) SendTestChannel(channel string) error {
	n := Notification{
		Title:     "Test notification",
		Message:   "This is a test notification from WebHostManager.",
		Severity:  SeverityInfo,
		EventType: "test",
	}
	switch channel {
	case ChannelEmail:
		return s.sendEmail(n)
	case ChannelSlack:
		return s.sendSlack(n)
	case ChannelDiscord:
		return s.sendDiscord(n)
	default:
		return fmt.Errorf("unknown channel: %s", channel)
	}
}

// --- Scheduler job ---

// CheckSSLExpiry is run daily by the scheduler. It looks for SSL certs expiring
// within 30 days and creates an in-app/delivery notification for each, deduped per day.
func (s *Service) CheckSSLExpiry() error {
	type certRow struct {
		ID         string
		DomainName string
		ExpiresAt  *time.Time
	}
	var certs []certRow
	// Use raw column names from ssl_certs table
	if err := s.db.Table("ssl_certs").
		Select("id, domain_name, expires_at").
		Where("expires_at IS NOT NULL AND expires_at <= NOW() + INTERVAL '30 days' AND status = 'valid'").
		Scan(&certs).Error; err != nil {
		return err
	}

	today := time.Now().Truncate(24 * time.Hour)

	for _, cert := range certs {
		if cert.ExpiresAt == nil {
			continue
		}
		days := int(time.Until(*cert.ExpiresAt).Hours() / 24)

		// Deduplicate: skip if already notified today for this cert
		var existing int64
		s.db.Model(&Notification{}).
			Where("event_type = ? AND resource_id = ? AND created_at >= ?", "ssl.expiring_soon", cert.ID, today).
			Count(&existing)
		if existing > 0 {
			continue
		}

		severity := SeverityWarning
		if days <= 7 {
			severity = SeverityCritical
		}

		n := Notification{
			Title:        fmt.Sprintf("SSL certificate expiring soon: %s", cert.DomainName),
			Message:      fmt.Sprintf("The SSL certificate for %s expires in %d day(s). Please renew it soon.", cert.DomainName, days),
			Severity:     severity,
			EventType:    "ssl.expiring_soon",
			ResourceType: "ssl_cert",
			ResourceID:   cert.ID,
		}
		if err := s.Create(n); err != nil {
			return err
		}
	}
	return nil
}

// --- DB helpers ---

// adminEmail returns the email of the first super_admin user.
func (s *Service) adminEmail() string {
	var email string
	s.db.Table("users").Where("role = 'super_admin'").Limit(1).Pluck("email", &email)
	return email
}

// settingVal reads a plaintext value from the settings table.
func (s *Service) settingVal(key string) string {
	var val string
	s.db.Table("settings").Where("key = ?", key).Pluck("value", &val)
	return val
}

// settingDecrypted reads and decrypts an encrypted value from the settings table.
func (s *Service) settingDecrypted(key string) (string, error) {
	encrypted := s.settingVal(key)
	if encrypted == "" {
		return "", fmt.Errorf("setting %q not found", key)
	}
	return crypto.Decrypt(encrypted, s.encKey)
}
