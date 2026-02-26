package ssl

import (
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/asifthewebguy/webhostmanager-api/internal/server"
	"github.com/asifthewebguy/webhostmanager-api/pkg/crypto"
)

// Service handles SSL certificate provisioning, renewal, and status tracking.
type Service struct {
	db        *gorm.DB
	serverSvc *server.Service
	encKey    string
}

func NewService(db *gorm.DB, serverSvc *server.Service, encKey string) *Service {
	return &Service{db: db, serverSvc: serverSvc, encKey: encKey}
}

// --- Read operations ---

// List returns all SSL certs ordered by domain name.
func (s *Service) List() ([]Cert, error) {
	var certs []Cert
	err := s.db.Order("domain_name ASC").Find(&certs).Error
	return certs, err
}

// GetByID returns a cert by its UUID.
func (s *Service) GetByID(id string) (*Cert, error) {
	var c Cert
	err := s.db.Where("id = ?", id).First(&c).Error
	return &c, err
}

// GetByDomainID returns the cert for a specific domain.
func (s *Service) GetByDomainID(domainID string) (*Cert, error) {
	var c Cert
	err := s.db.Where("domain_id = ?", domainID).First(&c).Error
	return &c, err
}

// Count returns total number of SSL certs with status 'valid' or 'expiring_soon'.
func (s *Service) Count() int64 {
	var count int64
	s.db.Model(&Cert{}).Where("status IN ?", []string{"valid", "expiring_soon"}).Count(&count)
	return count
}

// --- Write operations ---

// Provision requests a new Let's Encrypt certificate for a domain.
// Creates a DB record if one doesn't exist, then runs certbot.
func (s *Service) Provision(domainID string, isWildcard bool) (*Cert, error) {
	// Resolve domain
	domainName, err := s.domainName(domainID)
	if err != nil {
		return nil, fmt.Errorf("domain not found: %w", err)
	}

	// Upsert cert record in DB (pending state)
	cert := &Cert{}
	uid, _ := uuid.Parse(domainID)
	s.db.Where("domain_id = ?", domainID).First(cert)
	if cert.ID == uuid.Nil {
		cert = &Cert{
			DomainID:   uid,
			DomainName: domainName,
			Status:     "pending",
			IsWildcard: isWildcard,
		}
		if err := s.db.Create(cert).Error; err != nil {
			return nil, fmt.Errorf("create cert record: %w", err)
		}
	}

	exec, err := s.serverSvc.NewExecutor()
	if err != nil {
		return nil, fmt.Errorf("build executor: %w", err)
	}
	defer exec.Close()

	email := s.adminEmail()
	if email == "" {
		return nil, fmt.Errorf("no admin email found — set one in your account settings")
	}

	// Run certbot
	var certbotErr error
	if isWildcard {
		certbotErr = s.runWildcardCertbot(exec, domainName, email)
	} else {
		certbotErr = s.runWebRootCertbot(exec, domainName, email)
	}

	if certbotErr != nil {
		errMsg := certbotErr.Error()
		if len(errMsg) > 512 {
			errMsg = errMsg[:512]
		}
		s.db.Model(cert).Updates(map[string]any{
			"status":     "failed",
			"last_error": errMsg,
		})
		return nil, fmt.Errorf("certbot: %w", certbotErr)
	}

	// Read cert metadata from the issued certificate
	expiresAt, issuedAt := s.readCertDates(exec, domainName)
	certPath := fmt.Sprintf("/etc/letsencrypt/live/%s/fullchain.pem", domainName)
	keyPath := fmt.Sprintf("/etc/letsencrypt/live/%s/privkey.pem", domainName)
	status := computeStatus(expiresAt)
	now := time.Now()

	s.db.Model(cert).Updates(map[string]any{
		"status":           status,
		"cert_path":        certPath,
		"key_path":         keyPath,
		"is_wildcard":      isWildcard,
		"issued_at":        issuedAt,
		"expires_at":       expiresAt,
		"last_renewed_at":  now,
		"last_error":       "",
	})

	// Update nginx/apache vhost with SSL block
	proxyType := s.proxyType()
	s.updateVhostSSL(exec, domainName, certPath, keyPath, cert.RedirectHTTPS, proxyType)

	// Reload DB record
	s.db.Where("id = ?", cert.ID).First(cert)
	return cert, nil
}

// Renew force-renews an existing certificate.
func (s *Service) Renew(id string) error {
	cert, err := s.GetByID(id)
	if err != nil {
		return fmt.Errorf("cert not found")
	}

	exec, err := s.serverSvc.NewExecutor()
	if err != nil {
		return fmt.Errorf("build executor: %w", err)
	}
	defer exec.Close()

	out, runErr := exec.Run(fmt.Sprintf(
		"certbot renew --cert-name %s --non-interactive --force-renewal 2>&1",
		cert.DomainName,
	))

	if runErr != nil {
		errMsg := out
		if errMsg == "" {
			errMsg = runErr.Error()
		}
		if len(errMsg) > 512 {
			errMsg = errMsg[:512]
		}
		s.db.Model(cert).Updates(map[string]any{
			"status":     "failed",
			"last_error": errMsg,
		})
		return fmt.Errorf("certbot renew: %w", runErr)
	}

	expiresAt, issuedAt := s.readCertDates(exec, cert.DomainName)
	now := time.Now()
	s.db.Model(cert).Updates(map[string]any{
		"status":          computeStatus(expiresAt),
		"issued_at":       issuedAt,
		"expires_at":      expiresAt,
		"last_renewed_at": now,
		"last_error":      "",
	})
	return nil
}

// RenewDue runs `certbot renew` for all certs on the system then refreshes DB statuses.
// Called by the daily scheduler job.
func (s *Service) RenewDue() error {
	exec, err := s.serverSvc.NewExecutor()
	if err != nil {
		return fmt.Errorf("build executor: %w", err)
	}
	defer exec.Close()

	// Let certbot decide which certs need renewal (within 30-day window)
	exec.Run("certbot renew --non-interactive 2>&1")

	// Refresh all cert statuses in our DB
	return s.refreshAllStatuses(exec)
}

// ToggleRedirect enables or disables HTTP→HTTPS redirect for a domain.
func (s *Service) ToggleRedirect(id string, enable bool) error {
	cert, err := s.GetByID(id)
	if err != nil {
		return fmt.Errorf("cert not found")
	}
	if cert.Status != "valid" && cert.Status != "expiring_soon" {
		return fmt.Errorf("certificate is not active — provision it first")
	}

	exec, err := s.serverSvc.NewExecutor()
	if err != nil {
		return fmt.Errorf("build executor: %w", err)
	}
	defer exec.Close()

	proxyType := s.proxyType()
	s.updateVhostSSL(exec, cert.DomainName, cert.CertPath, cert.KeyPath, enable, proxyType)

	return s.db.Model(cert).Update("redirect_https", enable).Error
}

// RefreshAllStatuses re-reads cert expiry dates from disk and updates DB statuses.
func (s *Service) RefreshAllStatuses() error {
	exec, err := s.serverSvc.NewExecutor()
	if err != nil {
		return err
	}
	defer exec.Close()
	return s.refreshAllStatuses(exec)
}

// --- Internal helpers ---

func (s *Service) refreshAllStatuses(exec server.Executor) error {
	var certs []Cert
	if err := s.db.Where("cert_path != ''").Find(&certs).Error; err != nil {
		return err
	}
	for i := range certs {
		expiresAt, issuedAt := s.readCertDates(exec, certs[i].DomainName)
		if expiresAt != nil {
			status := computeStatus(expiresAt)
			s.db.Model(&certs[i]).Updates(map[string]any{
				"status":     status,
				"expires_at": expiresAt,
				"issued_at":  issuedAt,
				"last_error": "",
			})
		}
	}
	return nil
}

func (s *Service) runWebRootCertbot(exec server.Executor, domain, email string) error {
	cmd := fmt.Sprintf(
		"certbot certonly --webroot -w /var/www/%s/public_html -d %s -d www.%s --non-interactive --agree-tos --email %s 2>&1",
		domain, domain, domain, email,
	)
	_, err := exec.Run(cmd)
	return err
}

func (s *Service) runWildcardCertbot(exec server.Executor, domain, email string) error {
	token, err := s.cloudflareToken()
	if err != nil {
		return fmt.Errorf("cloudflare token not configured: %w", err)
	}

	// Write credentials file (base64-encoded to avoid quoting issues)
	credsPath := fmt.Sprintf("/etc/letsencrypt/cloudflare-%s.ini", domain)
	credsContent := fmt.Sprintf("dns_cloudflare_api_token = %s\n", token)
	enc := base64.StdEncoding.EncodeToString([]byte(credsContent))
	exec.Run(fmt.Sprintf("echo '%s' | base64 -d > %s && chmod 600 %s", enc, credsPath, credsPath))

	cmd := fmt.Sprintf(
		"certbot certonly --dns-cloudflare --dns-cloudflare-credentials %s -d %s -d '*.%s' --non-interactive --agree-tos --email %s 2>&1",
		credsPath, domain, domain, email,
	)
	_, runErr := exec.Run(cmd)

	// Always clean up credentials file
	exec.Run("rm -f " + credsPath)
	return runErr
}

// readCertDates extracts the expiry and issue dates from the cert file on disk.
func (s *Service) readCertDates(exec server.Executor, domain string) (expiresAt *time.Time, issuedAt *time.Time) {
	certPath := fmt.Sprintf("/etc/letsencrypt/live/%s/fullchain.pem", domain)

	if out, err := exec.Run(fmt.Sprintf(
		"openssl x509 -enddate -noout -in %s 2>/dev/null", certPath,
	)); err == nil {
		// "notAfter=Mar 27 12:00:00 2026 GMT"
		out = strings.TrimSpace(strings.TrimPrefix(out, "notAfter="))
		if t, err := time.Parse("Jan _2 15:04:05 2006 MST", out); err == nil {
			expiresAt = &t
		}
	}

	if out, err := exec.Run(fmt.Sprintf(
		"openssl x509 -startdate -noout -in %s 2>/dev/null", certPath,
	)); err == nil {
		out = strings.TrimSpace(strings.TrimPrefix(out, "notBefore="))
		if t, err := time.Parse("Jan _2 15:04:05 2006 MST", out); err == nil {
			issuedAt = &t
		}
	}
	return
}

// updateVhostSSL regenerates the vhost config to include SSL directives.
func (s *Service) updateVhostSSL(exec server.Executor, domain, certPath, keyPath string, redirect bool, proxyType string) {
	var content, path string
	switch proxyType {
	case "apache":
		content = generateApacheSSLConfig(domain, certPath, keyPath, redirect)
		path = fmt.Sprintf("/etc/apache2/sites-available/%s.conf", domain)
		exec.Run(fmt.Sprintf("echo '%s' | base64 -d | tee %s > /dev/null",
			base64.StdEncoding.EncodeToString([]byte(content)), path))
		exec.Run("systemctl reload apache2 2>/dev/null || service apache2 reload 2>/dev/null || true")
	default:
		content = generateNginxSSLConfig(domain, certPath, keyPath, redirect)
		path = fmt.Sprintf("/etc/nginx/sites-available/%s", domain)
		exec.Run(fmt.Sprintf("echo '%s' | base64 -d | tee %s > /dev/null",
			base64.StdEncoding.EncodeToString([]byte(content)), path))
		exec.Run("nginx -s reload 2>/dev/null || systemctl reload nginx 2>/dev/null || true")
	}
}

// domainName looks up a domain's name by ID from the domains table.
func (s *Service) domainName(domainID string) (string, error) {
	var name string
	err := s.db.Table("domains").Where("id = ?", domainID).Pluck("name", &name).Error
	if err != nil || name == "" {
		return "", fmt.Errorf("domain %s not found", domainID)
	}
	return name, nil
}

// adminEmail fetches the first super_admin's email from the users table.
func (s *Service) adminEmail() string {
	var email string
	s.db.Table("users").Where("role = ?", "super_admin").
		Order("created_at ASC").Limit(1).Pluck("email", &email)
	return email
}

// proxyType reads the proxy type from the settings table.
func (s *Service) proxyType() string {
	var v string
	s.db.Table("settings").Where("key = ?", "proxy.type").Pluck("value", &v)
	if v == "" {
		return "nginx"
	}
	return v
}

// cloudflareToken decrypts and returns the Cloudflare API token.
func (s *Service) cloudflareToken() (string, error) {
	var encrypted string
	if err := s.db.Table("settings").Where("key = ?", "cloudflare.api_token").
		Pluck("value", &encrypted).Error; err != nil || encrypted == "" {
		return "", fmt.Errorf("cloudflare API token not configured")
	}
	return crypto.Decrypt(encrypted, s.encKey)
}

// computeStatus derives a status string from the expiry time.
func computeStatus(expiresAt *time.Time) string {
	if expiresAt == nil {
		return "pending"
	}
	now := time.Now()
	if expiresAt.Before(now) {
		return "expired"
	}
	if expiresAt.Before(now.Add(30 * 24 * time.Hour)) {
		return "expiring_soon"
	}
	return "valid"
}

// --- vhost templates with SSL ---

func generateNginxSSLConfig(domain, certPath, keyPath string, redirect bool) string {
	sslBlock := fmt.Sprintf(`server {
    listen 443 ssl http2;
    server_name %s www.%s;
    root /var/www/%s/public_html;
    index index.html index.htm index.php;

    ssl_certificate %s;
    ssl_certificate_key %s;
    include /etc/letsencrypt/options-ssl-nginx.conf;
    ssl_dhparam /etc/letsencrypt/ssl-dhparams.pem;

    location / {
        try_files $uri $uri/ /index.php?$query_string;
    }

    location ~ \.php$ {
        include snippets/fastcgi-php.conf;
        fastcgi_pass unix:/run/php/php-fpm.sock;
    }

    location ~ /\.ht {
        deny all;
    }
}
`, domain, domain, domain, certPath, keyPath)

	if redirect {
		return fmt.Sprintf(`server {
    listen 80;
    server_name %s www.%s;
    return 301 https://$host$request_uri;
}

`, domain, domain) + sslBlock
	}

	// No redirect — serve both HTTP and HTTPS
	httpBlock := fmt.Sprintf(`server {
    listen 80;
    server_name %s www.%s;
    root /var/www/%s/public_html;
    index index.html index.htm index.php;

    location / {
        try_files $uri $uri/ /index.php?$query_string;
    }

    location ~ \.php$ {
        include snippets/fastcgi-php.conf;
        fastcgi_pass unix:/run/php/php-fpm.sock;
    }

    location ~ /\.ht {
        deny all;
    }
}

`, domain, domain, domain)
	return httpBlock + sslBlock
}

func generateApacheSSLConfig(domain, certPath, keyPath string, redirect bool) string {
	if redirect {
		return fmt.Sprintf(`<VirtualHost *:80>
    ServerName %s
    ServerAlias www.%s
    Redirect permanent / https://%s/
</VirtualHost>

<VirtualHost *:443>
    ServerName %s
    ServerAlias www.%s
    DocumentRoot /var/www/%s/public_html

    SSLEngine on
    SSLCertificateFile %s
    SSLCertificateKeyFile %s

    <Directory /var/www/%s/public_html>
        Options Indexes FollowSymLinks
        AllowOverride All
        Require all granted
    </Directory>

    ErrorLog /var/log/apache2/%s-error.log
    CustomLog /var/log/apache2/%s-access.log combined
</VirtualHost>
`, domain, domain, domain, domain, domain, domain, certPath, keyPath, domain, domain, domain)
	}

	return fmt.Sprintf(`<VirtualHost *:80>
    ServerName %s
    ServerAlias www.%s
    DocumentRoot /var/www/%s/public_html

    <Directory /var/www/%s/public_html>
        Options Indexes FollowSymLinks
        AllowOverride All
        Require all granted
    </Directory>

    ErrorLog /var/log/apache2/%s-error.log
    CustomLog /var/log/apache2/%s-access.log combined
</VirtualHost>

<VirtualHost *:443>
    ServerName %s
    ServerAlias www.%s
    DocumentRoot /var/www/%s/public_html

    SSLEngine on
    SSLCertificateFile %s
    SSLCertificateKeyFile %s

    <Directory /var/www/%s/public_html>
        Options Indexes FollowSymLinks
        AllowOverride All
        Require all granted
    </Directory>

    ErrorLog /var/log/apache2/%s-error.log
    CustomLog /var/log/apache2/%s-access.log combined
</VirtualHost>
`, domain, domain, domain, domain, domain, domain, domain, domain, domain, certPath, keyPath, domain, domain, domain)
}
