package wordpress

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"
	"regexp"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/asifthewebguy/webhostmanager-api/internal/server"
	"github.com/asifthewebguy/webhostmanager-api/pkg/crypto"
)

// nonAlphaNum replaces any non-alphanumeric run with a single underscore.
var nonAlphaNum = regexp.MustCompile(`[^a-z0-9]+`)

// Service handles WordPress installation lifecycle.
type Service struct {
	db        *gorm.DB
	serverSvc *server.Service
	encKey    string
}

func NewService(db *gorm.DB, serverSvc *server.Service, encKey string) *Service {
	return &Service{db: db, serverSvc: serverSvc, encKey: encKey}
}

// --- Read operations ---

// List returns all WordPress installs ordered by newest first.
func (s *Service) List() ([]WPInstall, error) {
	var installs []WPInstall
	err := s.db.Order("created_at DESC").Find(&installs).Error
	return installs, err
}

// GetByDomainID returns the WP install for a domain, or an error if not installed.
func (s *Service) GetByDomainID(domainID string) (*WPInstall, error) {
	var install WPInstall
	err := s.db.Where("domain_id = ?", domainID).First(&install).Error
	if err != nil {
		return nil, err
	}
	return &install, nil
}

// GetPlugins returns the plugin list for a WordPress installation.
func (s *Service) GetPlugins(domainID string) ([]Plugin, error) {
	install, err := s.GetByDomainID(domainID)
	if err != nil {
		return nil, fmt.Errorf("WordPress not installed on this domain")
	}

	exec, err := s.serverSvc.NewExecutor()
	if err != nil {
		return nil, fmt.Errorf("build executor: %w", err)
	}
	defer exec.Close()

	webRoot := fmt.Sprintf("/var/www/%s/public_html", install.DomainName)
	out, err := exec.Run(fmt.Sprintf(
		"wp plugin list --format=json --path=%s --allow-root 2>/dev/null",
		webRoot,
	))
	if err != nil || strings.TrimSpace(out) == "" {
		return []Plugin{}, nil
	}

	var plugins []Plugin
	if jsonErr := json.Unmarshal([]byte(strings.TrimSpace(out)), &plugins); jsonErr != nil {
		return []Plugin{}, nil
	}
	return plugins, nil
}

// Count returns the total number of WordPress installations.
func (s *Service) Count() int64 {
	var count int64
	s.db.Model(&WPInstall{}).Count(&count)
	return count
}

// --- Write operations ---

// Install provisions a WordPress site on the given domain.
func (s *Service) Install(domainID, adminUser, adminPass, adminEmail string) (*WPInstall, error) {
	// Resolve domain name
	domainName, err := s.domainName(domainID)
	if err != nil {
		return nil, fmt.Errorf("domain not found")
	}

	// Prevent double-install
	var count int64
	s.db.Model(&WPInstall{}).Where("domain_id = ?", domainID).Count(&count)
	if count > 0 {
		return nil, fmt.Errorf("WordPress is already installed on %s", domainName)
	}

	exec, err := s.serverSvc.NewExecutor()
	if err != nil {
		return nil, fmt.Errorf("build executor: %w", err)
	}
	defer exec.Close()

	webRoot := fmt.Sprintf("/var/www/%s/public_html", domainName)

	// Ensure WP-CLI is available
	if err := s.ensureWPCLI(exec); err != nil {
		return nil, fmt.Errorf("install WP-CLI: %w", err)
	}

	// Generate MySQL credentials (alphanumeric — safe in shell without quoting)
	dbName := "wp_" + sanitizeName(domainName, 59) // MySQL max DB name = 64 chars
	dbUser := "wp_" + sanitizeName(domainName, 28) // keep user ≤ 32 chars (MySQL 5.7 limit)
	if len(dbUser) > 32 {
		dbUser = dbUser[:32]
	}
	dbPass := generatePassword(24)

	// Create MySQL database and user
	if err := s.createDatabase(exec, dbName, dbUser, dbPass); err != nil {
		return nil, fmt.Errorf("create database: %w", err)
	}

	// Download WordPress core
	if _, dlErr := exec.Run(fmt.Sprintf(
		"wp core download --path=%s --allow-root --force 2>&1", webRoot,
	)); dlErr != nil {
		s.dropDatabase(exec, dbName, dbUser)
		return nil, fmt.Errorf("download WordPress: %w", dlErr)
	}

	// Create wp-config.php
	configCmd := fmt.Sprintf(
		"wp config create --dbname=%s --dbuser=%s --dbpass=%s --dbhost=localhost --path=%s --allow-root --force 2>&1",
		dbName, dbUser, dbPass, webRoot,
	)
	if _, cfgErr := exec.Run(configCmd); cfgErr != nil {
		s.dropDatabase(exec, dbName, dbUser)
		return nil, fmt.Errorf("create wp-config: %w", cfgErr)
	}

	// Run WordPress installation
	installCmd := fmt.Sprintf(
		"wp core install --url=%s --title=%s --admin_user=%s --admin_password=%s --admin_email=%s --path=%s --allow-root --skip-email 2>&1",
		shellQuote("https://"+domainName),
		shellQuote(domainName),
		shellQuote(adminUser),
		shellQuote(adminPass),
		shellQuote(adminEmail),
		webRoot,
	)
	if _, instErr := exec.Run(installCmd); instErr != nil {
		s.dropDatabase(exec, dbName, dbUser)
		return nil, fmt.Errorf("install WordPress: %w", instErr)
	}

	// Fix file ownership
	exec.Run(fmt.Sprintf("chown -R www-data:www-data %s", webRoot))

	// Get installed WP version
	wpVersion := ""
	if v, vErr := exec.Run(fmt.Sprintf(
		"wp core version --path=%s --allow-root 2>/dev/null", webRoot,
	)); vErr == nil {
		wpVersion = strings.TrimSpace(v)
	}

	// Encrypt DB password before storing
	encPass, encErr := crypto.Encrypt(dbPass, s.encKey)
	if encErr != nil {
		return nil, fmt.Errorf("encrypt db password: %w", encErr)
	}

	uid, _ := uuid.Parse(domainID)
	install := &WPInstall{
		DomainID:   uid,
		DomainName: domainName,
		DBName:     dbName,
		DBUser:     dbUser,
		DBPassword: encPass,
		WPVersion:  wpVersion,
		WPURL:      "https://" + domainName,
		AdminUser:  adminUser,
		AdminEmail: adminEmail,
		DebugMode:  false,
		Status:     "installed",
	}
	if saveErr := s.db.Create(install).Error; saveErr != nil {
		return nil, fmt.Errorf("save install record: %w", saveErr)
	}
	return install, nil
}

// Uninstall removes WordPress files and database, then deletes the DB record.
func (s *Service) Uninstall(domainID string) error {
	install, err := s.GetByDomainID(domainID)
	if err != nil {
		return fmt.Errorf("WordPress not installed on this domain")
	}

	exec, err := s.serverSvc.NewExecutor()
	if err != nil {
		return fmt.Errorf("build executor: %w", err)
	}
	defer exec.Close()

	// Drop MySQL database and user
	s.dropDatabase(exec, install.DBName, install.DBUser)

	// Remove WordPress files (keep public_html dir, restore placeholder)
	webRoot := fmt.Sprintf("/var/www/%s/public_html", install.DomainName)
	exec.Run(fmt.Sprintf("rm -rf %s/*", webRoot))
	exec.Run(fmt.Sprintf("rm -rf %s/.htaccess %s/wp-config.php", webRoot, webRoot))

	// Restore placeholder index.html
	placeholder := fmt.Sprintf(
		"<html><body><h1>%s</h1><p>Hosted by WebHostManager.</p></body></html>",
		install.DomainName,
	)
	exec.Run(fmt.Sprintf("echo %s > %s/index.html", shellQuote(placeholder), webRoot))

	return s.db.Delete(&WPInstall{}, "id = ?", install.ID).Error
}

// UpdateCore runs `wp core update` for a domain.
func (s *Service) UpdateCore(domainID string) error {
	install, err := s.GetByDomainID(domainID)
	if err != nil {
		return fmt.Errorf("WordPress not installed on this domain")
	}

	exec, err := s.serverSvc.NewExecutor()
	if err != nil {
		return fmt.Errorf("build executor: %w", err)
	}
	defer exec.Close()

	webRoot := fmt.Sprintf("/var/www/%s/public_html", install.DomainName)
	if _, runErr := exec.Run(fmt.Sprintf(
		"wp core update --path=%s --allow-root 2>&1", webRoot,
	)); runErr != nil {
		return fmt.Errorf("wp core update: %w", runErr)
	}

	// Refresh version in DB
	if v, vErr := exec.Run(fmt.Sprintf(
		"wp core version --path=%s --allow-root 2>/dev/null", webRoot,
	)); vErr == nil {
		s.db.Model(install).Update("wp_version", strings.TrimSpace(v))
	}
	return nil
}

// UpdatePlugin runs `wp plugin update` for a specific plugin.
func (s *Service) UpdatePlugin(domainID, plugin string) error {
	install, err := s.GetByDomainID(domainID)
	if err != nil {
		return fmt.Errorf("WordPress not installed on this domain")
	}

	exec, err := s.serverSvc.NewExecutor()
	if err != nil {
		return fmt.Errorf("build executor: %w", err)
	}
	defer exec.Close()

	webRoot := fmt.Sprintf("/var/www/%s/public_html", install.DomainName)
	if _, runErr := exec.Run(fmt.Sprintf(
		"wp plugin update %s --path=%s --allow-root 2>&1",
		shellQuote(plugin), webRoot,
	)); runErr != nil {
		return fmt.Errorf("wp plugin update: %w", runErr)
	}
	return nil
}

// ToggleDebug enables or disables WordPress debug mode via WP-CLI.
func (s *Service) ToggleDebug(domainID string, enable bool) error {
	install, err := s.GetByDomainID(domainID)
	if err != nil {
		return fmt.Errorf("WordPress not installed on this domain")
	}

	exec, err := s.serverSvc.NewExecutor()
	if err != nil {
		return fmt.Errorf("build executor: %w", err)
	}
	defer exec.Close()

	webRoot := fmt.Sprintf("/var/www/%s/public_html", install.DomainName)
	val := "false"
	if enable {
		val = "true"
	}
	if _, runErr := exec.Run(fmt.Sprintf(
		"wp config set WP_DEBUG %s --raw --path=%s --allow-root 2>&1",
		val, webRoot,
	)); runErr != nil {
		return fmt.Errorf("set WP_DEBUG: %w", runErr)
	}

	return s.db.Model(install).Update("debug_mode", enable).Error
}

// --- Internal helpers ---

// ensureWPCLI checks for wp-cli and installs it if absent.
func (s *Service) ensureWPCLI(exec server.Executor) error {
	if _, err := exec.Run("command -v wp >/dev/null 2>&1"); err == nil {
		return nil // already installed
	}
	_, err := exec.Run(
		"curl -sL https://raw.githubusercontent.com/wp-cli/builds/gh-pages/phar/wp-cli.phar" +
			" -o /usr/local/bin/wp && chmod +x /usr/local/bin/wp 2>&1",
	)
	return err
}

// createDatabase creates a MySQL database and user with full privileges.
func (s *Service) createDatabase(exec server.Executor, dbName, dbUser, dbPass string) error {
	cmds := []string{
		fmt.Sprintf("mysql -u root -e %s 2>&1",
			shellQuote(fmt.Sprintf("CREATE DATABASE IF NOT EXISTS `%s`;", dbName))),
		fmt.Sprintf("mysql -u root -e %s 2>&1",
			shellQuote(fmt.Sprintf("CREATE USER IF NOT EXISTS '%s'@'localhost' IDENTIFIED BY '%s';", dbUser, dbPass))),
		fmt.Sprintf("mysql -u root -e %s 2>&1",
			shellQuote(fmt.Sprintf("GRANT ALL PRIVILEGES ON `%s`.* TO '%s'@'localhost';", dbName, dbUser))),
		"mysql -u root -e 'FLUSH PRIVILEGES;' 2>&1",
	}
	for _, cmd := range cmds {
		if _, err := exec.Run(cmd); err != nil {
			return err
		}
	}
	return nil
}

// dropDatabase removes the MySQL database and user.
func (s *Service) dropDatabase(exec server.Executor, dbName, dbUser string) {
	exec.Run(fmt.Sprintf("mysql -u root -e %s 2>/dev/null || true",
		shellQuote(fmt.Sprintf("DROP DATABASE IF EXISTS `%s`;", dbName))))
	exec.Run(fmt.Sprintf("mysql -u root -e %s 2>/dev/null || true",
		shellQuote(fmt.Sprintf("DROP USER IF EXISTS '%s'@'localhost';", dbUser))))
}

// domainName looks up a domain's name by ID.
func (s *Service) domainName(domainID string) (string, error) {
	var name string
	err := s.db.Table("domains").Where("id = ?", domainID).Pluck("name", &name).Error
	if err != nil || name == "" {
		return "", fmt.Errorf("domain %s not found", domainID)
	}
	return name, nil
}

// sanitizeName converts a domain name to a safe MySQL identifier segment.
func sanitizeName(domain string, maxLen int) string {
	s := strings.ToLower(domain)
	s = nonAlphaNum.ReplaceAllString(s, "_")
	s = strings.Trim(s, "_")
	if len(s) > maxLen {
		s = s[:maxLen]
	}
	return s
}

// generatePassword returns a random alphanumeric string of the given length.
func generatePassword(length int) string {
	const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	for i := range b {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(chars))))
		b[i] = chars[n.Int64()]
	}
	return string(b)
}

// shellQuote wraps a string in single quotes, escaping any embedded single quotes.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}
