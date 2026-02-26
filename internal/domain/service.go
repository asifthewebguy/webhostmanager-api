package domain

import (
	"encoding/base64"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"gorm.io/gorm"

	"github.com/asifthewebguy/webhostmanager-api/internal/server"
)

// domainRe validates a fully-qualified domain name (supports subdomains).
var domainRe = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?)+$`)

// Service handles domain provisioning and lifecycle.
type Service struct {
	db        *gorm.DB
	serverSvc *server.Service
}

func NewService(db *gorm.DB, serverSvc *server.Service) *Service {
	return &Service{db: db, serverSvc: serverSvc}
}

// List returns all domains ordered by newest first.
func (s *Service) List() ([]Domain, error) {
	var domains []Domain
	err := s.db.Order("created_at DESC").Find(&domains).Error
	return domains, err
}

// GetByID returns a single domain by UUID.
func (s *Service) GetByID(id string) (*Domain, error) {
	var d Domain
	err := s.db.Where("id = ?", id).First(&d).Error
	return &d, err
}

// Count returns the total number of hosted domains.
func (s *Service) Count() int64 {
	var count int64
	s.db.Model(&Domain{}).Count(&count)
	return count
}

// Create provisions a new domain: web root + virtual host + DB record.
func (s *Service) Create(name string) (*Domain, error) {
	name = strings.ToLower(strings.TrimSpace(name))
	if !domainRe.MatchString(name) {
		return nil, fmt.Errorf("invalid domain name %q — use a valid fully-qualified domain (e.g. example.com)", name)
	}

	var count int64
	s.db.Model(&Domain{}).Where("name = ?", name).Count(&count)
	if count > 0 {
		return nil, fmt.Errorf("domain %q already exists", name)
	}

	exec, err := s.serverSvc.NewExecutor()
	if err != nil {
		return nil, fmt.Errorf("build executor: %w", err)
	}
	defer exec.Close()

	webRoot := "/var/www/" + name + "/public_html"

	// Create directory tree
	if _, err := exec.Run("mkdir -p " + webRoot); err != nil {
		return nil, fmt.Errorf("create web root: %w", err)
	}
	exec.Run("chown -R www-data:www-data /var/www/" + name)
	exec.Run("chmod 755 /var/www/" + name)

	// Placeholder index.html (base64-encoded to avoid quoting issues)
	placeholder := fmt.Sprintf("<html><body><h1>%s</h1><p>Hosted by WebHostManager.</p></body></html>", name)
	enc := base64.StdEncoding.EncodeToString([]byte(placeholder))
	exec.Run(fmt.Sprintf("echo '%s' | base64 -d > %s/index.html", enc, webRoot))

	// Virtual host config
	proxyType := s.getProxyType()
	if err := s.createVhost(exec, name, proxyType); err != nil {
		exec.Run("rm -rf /var/www/" + name)
		return nil, fmt.Errorf("create virtual host: %w", err)
	}

	d := &Domain{
		Name:    name,
		Status:  "active",
		WebRoot: "/var/www/" + name,
	}
	if err := s.db.Create(d).Error; err != nil {
		s.removeVhost(exec, name, proxyType)
		exec.Run("rm -rf /var/www/" + name)
		return nil, fmt.Errorf("save domain: %w", err)
	}
	return d, nil
}

// Delete removes the virtual host, web root, and DB record for a domain.
func (s *Service) Delete(id string) error {
	d, err := s.GetByID(id)
	if err != nil {
		return fmt.Errorf("domain not found")
	}

	exec, err := s.serverSvc.NewExecutor()
	if err != nil {
		return fmt.Errorf("build executor: %w", err)
	}
	defer exec.Close()

	proxyType := s.getProxyType()
	s.removeVhost(exec, d.Name, proxyType)
	exec.Run("rm -rf /var/www/" + d.Name)

	return s.db.Delete(&Domain{}, "id = ?", id).Error
}

// RefreshDiskUsage queries the server for current disk usage and persists it.
func (s *Service) RefreshDiskUsage(d *Domain) error {
	exec, err := s.serverSvc.NewExecutor()
	if err != nil {
		return err
	}
	defer exec.Close()

	out, err := exec.Run(fmt.Sprintf("du -sm /var/www/%s 2>/dev/null | awk '{print $1}'", d.Name))
	if err != nil {
		return err
	}
	mb, err := strconv.ParseInt(strings.TrimSpace(out), 10, 64)
	if err != nil {
		return nil // non-fatal — directory may be empty
	}
	return s.db.Model(d).Update("disk_used_mb", mb).Error
}

// --- internal helpers ---

// getProxyType reads the proxy_type setting from the settings table.
func (s *Service) getProxyType() string {
	var v string
	s.db.Table("settings").Where("key = ?", "proxy.type").Pluck("value", &v)
	if v == "" {
		return "nginx"
	}
	return v
}

func (s *Service) createVhost(exec server.Executor, name, proxyType string) error {
	var content, path string
	switch proxyType {
	case "apache":
		content = generateApacheConfig(name)
		path = "/etc/apache2/sites-available/" + name + ".conf"
	default: // nginx
		content = generateNginxConfig(name)
		path = "/etc/nginx/sites-available/" + name
	}

	// Write config file (base64-encoded to avoid shell quoting issues)
	enc := base64.StdEncoding.EncodeToString([]byte(content))
	if _, err := exec.Run(fmt.Sprintf("echo '%s' | base64 -d | tee %s > /dev/null", enc, path)); err != nil {
		return fmt.Errorf("write vhost config: %w", err)
	}

	// Enable site and reload
	switch proxyType {
	case "apache":
		exec.Run("a2ensite " + name + ".conf")
		exec.Run("systemctl reload apache2 2>/dev/null || service apache2 reload 2>/dev/null || true")
	default:
		exec.Run(fmt.Sprintf("ln -sf %s /etc/nginx/sites-enabled/%s", path, name))
		exec.Run("nginx -s reload 2>/dev/null || systemctl reload nginx 2>/dev/null || true")
	}
	return nil
}

func (s *Service) removeVhost(exec server.Executor, name, proxyType string) {
	switch proxyType {
	case "apache":
		exec.Run("a2dissite " + name + ".conf 2>/dev/null || true")
		exec.Run("rm -f /etc/apache2/sites-available/" + name + ".conf")
		exec.Run("systemctl reload apache2 2>/dev/null || service apache2 reload 2>/dev/null || true")
	default:
		exec.Run("rm -f /etc/nginx/sites-enabled/" + name)
		exec.Run("rm -f /etc/nginx/sites-available/" + name)
		exec.Run("nginx -s reload 2>/dev/null || systemctl reload nginx 2>/dev/null || true")
	}
}

func generateNginxConfig(name string) string {
	return fmt.Sprintf(`server {
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
`, name, name, name)
}

func generateApacheConfig(name string) string {
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
`, name, name, name, name, name, name)
}
