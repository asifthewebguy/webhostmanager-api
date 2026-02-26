package dns

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/asifthewebguy/webhostmanager-api/internal/server"
	"github.com/asifthewebguy/webhostmanager-api/pkg/crypto"
)

const cfBaseURL = "https://api.cloudflare.com/client/v4"

// Service handles DNS record management via the Cloudflare API v4.
type Service struct {
	db        *gorm.DB
	serverSvc *server.Service
	encKey    string
}

func NewService(db *gorm.DB, serverSvc *server.Service, encKey string) *Service {
	return &Service{db: db, serverSvc: serverSvc, encKey: encKey}
}

// --- Read operations ---

// ListByDomain returns all DNS records for a domain from the local DB.
func (s *Service) ListByDomain(domainID string) ([]DNSRecord, error) {
	var records []DNSRecord
	err := s.db.Where("domain_id = ?", domainID).Order("type ASC, name ASC").Find(&records).Error
	return records, err
}

// --- Sync ---

// SyncFromCloudflare pulls all DNS records from Cloudflare for the given domain,
// upserts them into dns_records, removes stale records, and returns the updated list.
func (s *Service) SyncFromCloudflare(domainID string) ([]DNSRecord, error) {
	domName, err := s.domainName(domainID)
	if err != nil {
		return nil, fmt.Errorf("domain not found: %w", err)
	}

	token, err := s.cloudflareToken()
	if err != nil {
		return nil, err
	}

	zoneID, err := s.zoneIDForDomain(token, domName)
	if err != nil {
		return nil, err
	}

	domUID, err := uuid.Parse(domainID)
	if err != nil {
		return nil, fmt.Errorf("invalid domain id: %w", err)
	}

	// Fetch all records from CF (handle pagination)
	var cfRecords []map[string]any
	page := 1
	for {
		resp, err := s.cfDo("GET", fmt.Sprintf("/zones/%s/dns_records?per_page=100&page=%d", zoneID, page), token, nil)
		if err != nil {
			return nil, fmt.Errorf("cloudflare list records: %w", err)
		}
		result, _ := resp["result"].([]any)
		for _, r := range result {
			if rec, ok := r.(map[string]any); ok {
				cfRecords = append(cfRecords, rec)
			}
		}
		// Check if there are more pages
		ri, _ := resp["result_info"].(map[string]any)
		totalPages := int(toFloat(ri["total_pages"]))
		if page >= totalPages || totalPages == 0 {
			break
		}
		page++
	}

	// Build set of CF record IDs we just fetched
	fetchedIDs := make(map[string]struct{}, len(cfRecords))
	for _, rec := range cfRecords {
		cfID, _ := rec["id"].(string)
		fetchedIDs[cfID] = struct{}{}

		record := DNSRecord{
			DomainID:           domUID,
			DomainName:         domName,
			CloudflareZoneID:   zoneID,
			CloudflareRecordID: cfID,
			Type:               strVal(rec, "type"),
			Name:               strVal(rec, "name"),
			Content:            strVal(rec, "content"),
			TTL:                int(toFloat(rec["ttl"])),
			Proxied:            boolVal(rec, "proxied"),
			Priority:           int(toFloat(rec["priority"])),
		}
		if record.TTL == 0 {
			record.TTL = 1
		}

		// Upsert by cloudflare_record_id
		s.db.Where("cloudflare_record_id = ?", cfID).First(&record)
		if record.ID == uuid.Nil {
			s.db.Create(&record)
		} else {
			s.db.Model(&record).Updates(map[string]any{
				"type":     record.Type,
				"name":     record.Name,
				"content":  record.Content,
				"ttl":      record.TTL,
				"proxied":  record.Proxied,
				"priority": record.Priority,
			})
		}
	}

	// Delete DB records that are no longer in Cloudflare
	var existing []DNSRecord
	s.db.Where("domain_id = ? AND cloudflare_record_id <> ''", domainID).Find(&existing)
	for _, ex := range existing {
		if _, found := fetchedIDs[ex.CloudflareRecordID]; !found {
			s.db.Delete(&ex)
		}
	}

	return s.ListByDomain(domainID)
}

// --- Write operations ---

// CreateRecord creates a DNS record in Cloudflare and stores it in the DB.
func (s *Service) CreateRecord(domainID string, req CreateRecordRequest) (*DNSRecord, error) {
	domName, err := s.domainName(domainID)
	if err != nil {
		return nil, fmt.Errorf("domain not found: %w", err)
	}

	token, err := s.cloudflareToken()
	if err != nil {
		return nil, err
	}

	zoneID, err := s.zoneIDForDomain(token, domName)
	if err != nil {
		return nil, err
	}

	domUID, _ := uuid.Parse(domainID)

	ttl := req.TTL
	if ttl == 0 {
		ttl = 1
	}

	body := map[string]any{
		"type":    req.Type,
		"name":    req.Name,
		"content": req.Content,
		"ttl":     ttl,
		"proxied": req.Proxied,
	}
	if req.Priority > 0 {
		body["priority"] = req.Priority
	}

	resp, err := s.cfDo("POST", "/zones/"+zoneID+"/dns_records", token, body)
	if err != nil {
		return nil, fmt.Errorf("cloudflare create record: %w", err)
	}

	result, _ := resp["result"].(map[string]any)
	cfID, _ := result["id"].(string)

	record := &DNSRecord{
		DomainID:           domUID,
		DomainName:         domName,
		CloudflareZoneID:   zoneID,
		CloudflareRecordID: cfID,
		Type:               req.Type,
		Name:               req.Name,
		Content:            req.Content,
		TTL:                ttl,
		Proxied:            req.Proxied,
		Priority:           req.Priority,
	}
	if err := s.db.Create(record).Error; err != nil {
		return nil, fmt.Errorf("store record: %w", err)
	}
	return record, nil
}

// UpdateRecord updates a DNS record in Cloudflare and in the DB.
func (s *Service) UpdateRecord(id string, req UpdateRecordRequest) (*DNSRecord, error) {
	var record DNSRecord
	if err := s.db.Where("id = ?", id).First(&record).Error; err != nil {
		return nil, fmt.Errorf("record not found")
	}

	token, err := s.cloudflareToken()
	if err != nil {
		return nil, err
	}

	ttl := req.TTL
	if ttl == 0 {
		ttl = 1
	}

	body := map[string]any{
		"type":    req.Type,
		"name":    req.Name,
		"content": req.Content,
		"ttl":     ttl,
		"proxied": req.Proxied,
	}
	if req.Priority > 0 {
		body["priority"] = req.Priority
	}

	if _, err := s.cfDo("PATCH", "/zones/"+record.CloudflareZoneID+"/dns_records/"+record.CloudflareRecordID, token, body); err != nil {
		return nil, fmt.Errorf("cloudflare update record: %w", err)
	}

	record.Type = req.Type
	record.Name = req.Name
	record.Content = req.Content
	record.TTL = ttl
	record.Proxied = req.Proxied
	record.Priority = req.Priority
	record.UpdatedAt = time.Now()
	if err := s.db.Save(&record).Error; err != nil {
		return nil, fmt.Errorf("update record: %w", err)
	}
	return &record, nil
}

// DeleteRecord deletes a DNS record from Cloudflare and from the DB.
func (s *Service) DeleteRecord(id string) (*DNSRecord, error) {
	var record DNSRecord
	if err := s.db.Where("id = ?", id).First(&record).Error; err != nil {
		return nil, fmt.Errorf("record not found")
	}

	token, err := s.cloudflareToken()
	if err != nil {
		return nil, err
	}

	if _, err := s.cfDo("DELETE", "/zones/"+record.CloudflareZoneID+"/dns_records/"+record.CloudflareRecordID, token, nil); err != nil {
		return nil, fmt.Errorf("cloudflare delete record: %w", err)
	}

	if err := s.db.Delete(&record).Error; err != nil {
		return nil, fmt.Errorf("delete record: %w", err)
	}
	return &record, nil
}

// ToggleProxy updates only the proxied field of a DNS record.
func (s *Service) ToggleProxy(id string, req ToggleProxyRequest) (*DNSRecord, error) {
	var record DNSRecord
	if err := s.db.Where("id = ?", id).First(&record).Error; err != nil {
		return nil, fmt.Errorf("record not found")
	}

	token, err := s.cloudflareToken()
	if err != nil {
		return nil, err
	}

	body := map[string]any{
		"type":    record.Type,
		"name":    record.Name,
		"content": record.Content,
		"ttl":     record.TTL,
		"proxied": req.Proxied,
	}

	if _, err := s.cfDo("PATCH", "/zones/"+record.CloudflareZoneID+"/dns_records/"+record.CloudflareRecordID, token, body); err != nil {
		return nil, fmt.Errorf("cloudflare toggle proxy: %w", err)
	}

	record.Proxied = req.Proxied
	record.UpdatedAt = time.Now()
	if err := s.db.Save(&record).Error; err != nil {
		return nil, fmt.Errorf("update record: %w", err)
	}
	return &record, nil
}

// --- Cloudflare helpers ---

// cloudflareToken decrypts and returns the Cloudflare API token from settings.
func (s *Service) cloudflareToken() (string, error) {
	var encrypted string
	if err := s.db.Table("settings").Where("key = ?", "cloudflare.api_token").
		Pluck("value", &encrypted).Error; err != nil || encrypted == "" {
		return "", fmt.Errorf("cloudflare API token not configured")
	}
	return crypto.Decrypt(encrypted, s.encKey)
}

// zoneIDForDomain queries the Cloudflare Zones API to find the zone ID for the
// given domain name. It strips subdomains to find the apex zone.
func (s *Service) zoneIDForDomain(token, domainName string) (string, error) {
	apex := apexDomain(domainName)
	resp, err := s.cfDo("GET", "/zones?name="+apex, token, nil)
	if err != nil {
		return "", fmt.Errorf("cloudflare zone lookup: %w", err)
	}
	result, _ := resp["result"].([]any)
	if len(result) == 0 {
		return "", fmt.Errorf("no Cloudflare zone found for domain %q", apex)
	}
	zone, _ := result[0].(map[string]any)
	id, _ := zone["id"].(string)
	if id == "" {
		return "", fmt.Errorf("empty zone id from Cloudflare")
	}
	return id, nil
}

// cfDo executes a Cloudflare API v4 request.
func (s *Service) cfDo(method, path string, token string, body any) (map[string]any, error) {
	var reqBody io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reqBody = bytes.NewReader(b)
	}

	req, err := http.NewRequest(method, cfBaseURL+path, reqBody)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	client := &http.Client{Timeout: 30 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	var out map[string]any
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("cloudflare response decode: %w", err)
	}

	success, _ := out["success"].(bool)
	if !success {
		errs, _ := out["errors"].([]any)
		if len(errs) > 0 {
			if e, ok := errs[0].(map[string]any); ok {
				return nil, fmt.Errorf("cloudflare error: %v", e["message"])
			}
		}
		return nil, fmt.Errorf("cloudflare request failed (status %d)", res.StatusCode)
	}

	return out, nil
}

// --- DB helpers ---

func (s *Service) domainName(domainID string) (string, error) {
	var name string
	return name, s.db.Table("domains").Where("id = ?", domainID).Pluck("name", &name).Error
}

// --- Utility ---

// apexDomain strips subdomains to return the registrable apex domain.
// e.g. "sub.example.com" → "example.com"
func apexDomain(domain string) string {
	parts := strings.Split(domain, ".")
	if len(parts) <= 2 {
		return domain
	}
	return strings.Join(parts[len(parts)-2:], ".")
}

func strVal(m map[string]any, key string) string {
	v, _ := m[key].(string)
	return v
}

func boolVal(m map[string]any, key string) bool {
	v, _ := m[key].(bool)
	return v
}

func toFloat(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case int:
		return float64(n)
	}
	return 0
}
