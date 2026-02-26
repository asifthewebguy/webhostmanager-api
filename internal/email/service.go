package email

import (
	"encoding/base64"
	"fmt"
	"strings"
	"sync"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/asifthewebguy/webhostmanager-api/internal/server"
	"github.com/asifthewebguy/webhostmanager-api/pkg/crypto"
)

const (
	dovecotUsersFile  = "/etc/dovecot/users"
	postfixMboxDoms   = "/etc/postfix/virtual_mailbox_domains"
	postfixMboxMaps   = "/etc/postfix/virtual_mailbox_maps"
	postfixAliasMaps  = "/etc/postfix/virtual_alias_maps"
	mailVhostsBase    = "/var/mail/vhosts"
)

// installState tracks a running mail-server installation.
type installState struct {
	mu      sync.Mutex
	running bool
	done    bool
	errMsg  string
	logs    []string
}

func (s *installState) log(msg string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.logs = append(s.logs, msg)
}

func (s *installState) finish(errMsg string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.running = false
	s.done = true
	s.errMsg = errMsg
}

func (s *installState) snapshot() InstallProgress {
	s.mu.Lock()
	defer s.mu.Unlock()
	logs := make([]string, len(s.logs))
	copy(logs, s.logs)
	return InstallProgress{
		Running: s.running,
		Done:    s.done,
		Error:   s.errMsg,
		Logs:    logs,
	}
}

// Service handles email account management and mail server installation.
type Service struct {
	db        *gorm.DB
	serverSvc *server.Service
	encKey    string
	install   installState
}

func NewService(db *gorm.DB, serverSvc *server.Service, encKey string) *Service {
	return &Service{db: db, serverSvc: serverSvc, encKey: encKey}
}

// --- Mail server ---

// CheckMailServerStatus checks whether Postfix and Dovecot are active.
func (s *Service) CheckMailServerStatus() (*MailServerStatus, error) {
	exec, err := s.serverSvc.NewExecutor()
	if err != nil {
		return nil, fmt.Errorf("build executor: %w", err)
	}
	defer exec.Close()

	isActive := func(svc string) bool {
		out, _ := exec.Run(fmt.Sprintf("systemctl is-active %s 2>/dev/null || echo inactive", svc))
		return strings.TrimSpace(out) == "active"
	}

	postfix := isActive("postfix")
	dovecot := isActive("dovecot")
	sa      := isActive("spamassassin")
	dkim    := isActive("opendkim")

	return &MailServerStatus{
		Postfix:      postfix,
		Dovecot:      dovecot,
		SpamAssassin: sa,
		DKIM:         dkim,
		Installed:    postfix && dovecot,
	}, nil
}

// InstallMailServer launches a background goroutine that installs Postfix + Dovecot.
// It is a no-op if an installation is already running.
func (s *Service) InstallMailServer(opts InstallMailServerRequest) error {
	s.install.mu.Lock()
	if s.install.running {
		s.install.mu.Unlock()
		return fmt.Errorf("installation already in progress")
	}
	s.install = installState{running: true} // reset state
	s.install.mu.Unlock()

	go func() {
		exec, err := s.serverSvc.NewExecutor()
		if err != nil {
			s.install.log("ERROR: " + err.Error())
			s.install.finish(err.Error())
			return
		}
		defer exec.Close()

		run := func(desc, cmd string) error {
			s.install.log(">>> " + desc)
			out, err := exec.Run(cmd)
			if out != "" {
				for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
					s.install.log(line)
				}
			}
			if err != nil {
				msg := fmt.Sprintf("FAILED: %s — %v", desc, err)
				s.install.log(msg)
				return fmt.Errorf("%s", msg)
			}
			s.install.log("OK: " + desc)
			return nil
		}

		// 1. Install packages
		pkgs := "postfix dovecot-core dovecot-imapd dovecot-pop3d"
		if opts.SpamAssassin {
			pkgs += " spamassassin spamc"
		}
		if opts.DKIM {
			pkgs += " opendkim opendkim-tools"
		}
		if err := run("Update package list", "DEBIAN_FRONTEND=noninteractive apt-get update -qq"); err != nil {
			s.install.finish(err.Error()); return
		}
		if err := run("Install mail server packages",
			"DEBIAN_FRONTEND=noninteractive apt-get install -y -qq "+pkgs); err != nil {
			s.install.finish(err.Error()); return
		}

		// 2. Create virtual mailbox directory structure
		if err := run("Create virtual mailbox base directory",
			"mkdir -p "+mailVhostsBase+" && chown -R vmail:vmail "+mailVhostsBase+" 2>/dev/null || "+
				"useradd -r -u 150 -g mail -d "+mailVhostsBase+" -s /sbin/nologin vmail 2>/dev/null; "+
				"mkdir -p "+mailVhostsBase+" && chown vmail: "+mailVhostsBase); err != nil {
			s.install.finish(err.Error()); return
		}

		// 3. Configure Postfix for virtual mailboxes
		postfixMain := `# WebHostManager — virtual mailbox config
virtual_mailbox_domains = /etc/postfix/virtual_mailbox_domains
virtual_mailbox_base = /var/mail/vhosts
virtual_mailbox_maps = hash:/etc/postfix/virtual_mailbox_maps
virtual_minimum_uid = 100
virtual_uid_maps = static:150
virtual_gid_maps = static:8
virtual_alias_maps = hash:/etc/postfix/virtual_alias_maps
`
		enc := base64.StdEncoding.EncodeToString([]byte(postfixMain))
		if err := run("Configure Postfix virtual mailbox settings",
			fmt.Sprintf("echo '%s' | base64 -d >> /etc/postfix/main.cf", enc)); err != nil {
			s.install.finish(err.Error()); return
		}

		// 4. Initialize empty Postfix map files if they don't exist
		initFiles := []string{postfixMboxDoms, postfixMboxMaps, postfixAliasMaps}
		for _, f := range initFiles {
			if err := run("Initialize "+f,
				fmt.Sprintf("touch %s && postmap %s 2>/dev/null || true", f, f)); err != nil {
				s.install.log("WARN: could not init " + f)
			}
		}

		// 5. Configure Dovecot for passwd-file auth
		dovecotConf := `# WebHostManager — virtual user auth
passdb {
  driver = passwd-file
  args = scheme=SHA512-CRYPT /etc/dovecot/users
}
userdb {
  driver = passwd-file
  args = /etc/dovecot/users
  default_fields = uid=vmail gid=mail home=/var/mail/vhosts/%d/%n
}
`
		enc = base64.StdEncoding.EncodeToString([]byte(dovecotConf))
		if err := run("Configure Dovecot virtual user authentication",
			fmt.Sprintf("echo '%s' | base64 -d > /etc/dovecot/conf.d/99-whm-virtual.conf", enc)); err != nil {
			s.install.finish(err.Error()); return
		}

		// Initialize empty users file
		run("Initialize Dovecot users file", "touch "+dovecotUsersFile+" && chmod 640 "+dovecotUsersFile)

		// 6. Enable and start services
		for _, svc := range []string{"postfix", "dovecot"} {
			if err := run("Enable and start "+svc,
				"systemctl enable "+svc+" && systemctl restart "+svc); err != nil {
				s.install.finish(err.Error()); return
			}
		}

		// 7. Optional: SpamAssassin
		if opts.SpamAssassin {
			run("Enable SpamAssassin", "systemctl enable spamassassin && systemctl start spamassassin")
		}

		// 8. Optional: OpenDKIM
		if opts.DKIM {
			run("Enable OpenDKIM", "systemctl enable opendkim && systemctl start opendkim")
		}

		s.install.log("Mail server installation complete.")
		s.install.finish("")
	}()

	return nil
}

// GetInstallProgress returns the current mail server installation progress.
func (s *Service) GetInstallProgress() InstallProgress {
	return s.install.snapshot()
}

// --- Account management ---

// ListAll returns all email accounts, newest first.
func (s *Service) ListAll() ([]EmailAccount, error) {
	var accounts []EmailAccount
	err := s.db.Order("created_at DESC").Find(&accounts).Error
	return accounts, err
}

// ListByDomain returns accounts for a specific domain.
func (s *Service) ListByDomain(domainID string) ([]EmailAccount, error) {
	var accounts []EmailAccount
	err := s.db.Where("domain_id = ?", domainID).Order("username ASC").Find(&accounts).Error
	return accounts, err
}

// Count returns the total number of email accounts.
func (s *Service) Count() int64 {
	var count int64
	s.db.Model(&EmailAccount{}).Count(&count)
	return count
}

// domainName looks up a domain's name by ID (same pattern as wordpress service).
func (s *Service) domainName(domainID string) (string, error) {
	var name string
	err := s.db.Table("domains").Where("id = ?", domainID).Pluck("name", &name).Error
	if err != nil || name == "" {
		return "", fmt.Errorf("domain %s not found", domainID)
	}
	return name, nil
}

// CreateAccount creates a new virtual email account on the mail server.
func (s *Service) CreateAccount(domainID string, req CreateAccountRequest) (*EmailAccount, error) {
	domainName, err := s.domainName(domainID)
	if err != nil {
		return nil, err
	}
	if req.QuotaMB <= 0 {
		req.QuotaMB = 500
	}
	fullEmail := req.Username + "@" + domainName

	// Encrypt password for DB storage
	encPass, err := crypto.Encrypt(req.Password, s.encKey)
	if err != nil {
		return nil, fmt.Errorf("encrypt password: %w", err)
	}

	domUID, err := uuid.Parse(domainID)
	if err != nil {
		return nil, fmt.Errorf("invalid domain id: %w", err)
	}

	account := &EmailAccount{
		DomainID:   domUID,
		DomainName: domainName,
		Username:   req.Username,
		Email:      fullEmail,
		Password:   encPass,
		QuotaMB:    req.QuotaMB,
		Status:     "active",
	}
	if err := s.db.Create(account).Error; err != nil {
		return nil, fmt.Errorf("create account record: %w", err)
	}

	// Provision on the mail server
	if provErr := s.provisionAccount(account, req.Password); provErr != nil {
		// Roll back DB record to avoid orphaned entries
		s.db.Delete(account)
		return nil, fmt.Errorf("provision account on server: %w", provErr)
	}

	return account, nil
}

func (s *Service) provisionAccount(account *EmailAccount, plainPass string) error {
	exec, err := s.serverSvc.NewExecutor()
	if err != nil {
		return fmt.Errorf("build executor: %w", err)
	}
	defer exec.Close()

	// Hash password with SHA-512 for Dovecot
	hashOut, err := exec.Run(fmt.Sprintf("openssl passwd -6 '%s'", shellQuote(plainPass)))
	if err != nil || strings.TrimSpace(hashOut) == "" {
		return fmt.Errorf("hash password: %w", err)
	}
	passHash := strings.TrimSpace(hashOut)

	// Create Maildir
	maildir := fmt.Sprintf("%s/%s/%s/Maildir", mailVhostsBase, account.DomainName, account.Username)
	if _, err := exec.Run(fmt.Sprintf("mkdir -p %s/{cur,new,tmp} && chown -R vmail: %s", maildir, maildir)); err != nil {
		return fmt.Errorf("create maildir: %w", err)
	}

	// Append to Dovecot users file: email:hash::
	userLine := fmt.Sprintf("%s:%s::", account.Email, passHash)
	enc := base64.StdEncoding.EncodeToString([]byte(userLine + "\n"))
	if _, err := exec.Run(fmt.Sprintf("echo '%s' | base64 -d >> %s", enc, dovecotUsersFile)); err != nil {
		return fmt.Errorf("add dovecot user: %w", err)
	}

	// Ensure domain is in virtual_mailbox_domains
	if _, err := exec.Run(fmt.Sprintf(
		"grep -qxF '%s' %s || echo '%s' >> %s",
		account.DomainName, postfixMboxDoms, account.DomainName, postfixMboxDoms,
	)); err != nil {
		return fmt.Errorf("register mailbox domain: %w", err)
	}
	exec.Run(fmt.Sprintf("postmap %s 2>/dev/null || true", postfixMboxDoms))

	// Append to virtual_mailbox_maps: email domain/username/Maildir/
	mapLine := fmt.Sprintf("%s %s/%s/Maildir/\n", account.Email, account.DomainName, account.Username)
	enc = base64.StdEncoding.EncodeToString([]byte(mapLine))
	if _, err := exec.Run(fmt.Sprintf("echo '%s' | base64 -d >> %s", enc, postfixMboxMaps)); err != nil {
		return fmt.Errorf("add postfix mailbox map: %w", err)
	}
	if _, err := exec.Run(fmt.Sprintf("postmap %s && postfix reload 2>/dev/null || true", postfixMboxMaps)); err != nil {
		return fmt.Errorf("reload postfix: %w", err)
	}

	exec.Run("doveadm reload 2>/dev/null || true")
	return nil
}

// DeleteAccount removes an email account from the DB and mail server.
func (s *Service) DeleteAccount(id string) (*EmailAccount, error) {
	var account EmailAccount
	if err := s.db.Where("id = ?", id).First(&account).Error; err != nil {
		return nil, fmt.Errorf("account not found: %w", err)
	}

	// Remove from mail server
	s.deprovisionAccount(&account)

	if err := s.db.Delete(&account).Error; err != nil {
		return nil, fmt.Errorf("delete account record: %w", err)
	}
	return &account, nil
}

func (s *Service) deprovisionAccount(account *EmailAccount) {
	exec, err := s.serverSvc.NewExecutor()
	if err != nil {
		return
	}
	defer exec.Close()

	// Remove from Dovecot users
	exec.Run(fmt.Sprintf("sed -i '/^%s:/d' %s", strings.ReplaceAll(account.Email, ".", "\\."), dovecotUsersFile))

	// Remove from Postfix mailbox maps
	exec.Run(fmt.Sprintf("sed -i '/^%s /d' %s", strings.ReplaceAll(account.Email, ".", "\\."), postfixMboxMaps))
	exec.Run(fmt.Sprintf("postmap %s && postfix reload 2>/dev/null || true", postfixMboxMaps))

	// Remove Maildir
	maildir := fmt.Sprintf("%s/%s/%s", mailVhostsBase, account.DomainName, account.Username)
	exec.Run("rm -rf " + maildir)

	exec.Run("doveadm reload 2>/dev/null || true")
}

// ChangePassword updates the encrypted password in DB and rehashes in Dovecot.
func (s *Service) ChangePassword(id string, req ChangePasswordRequest) error {
	var account EmailAccount
	if err := s.db.Where("id = ?", id).First(&account).Error; err != nil {
		return fmt.Errorf("account not found: %w", err)
	}

	encPass, err := crypto.Encrypt(req.Password, s.encKey)
	if err != nil {
		return fmt.Errorf("encrypt password: %w", err)
	}

	exec, err := s.serverSvc.NewExecutor()
	if err != nil {
		return fmt.Errorf("build executor: %w", err)
	}
	defer exec.Close()

	hashOut, err := exec.Run(fmt.Sprintf("openssl passwd -6 '%s'", shellQuote(req.Password)))
	if err != nil || strings.TrimSpace(hashOut) == "" {
		return fmt.Errorf("hash password: %w", err)
	}
	passHash := strings.TrimSpace(hashOut)

	// Replace the hash in Dovecot users file
	escapedEmail := strings.ReplaceAll(account.Email, ".", "\\.")
	exec.Run(fmt.Sprintf(
		"sed -i 's|^%s:.*::|%s:%s::|' %s",
		escapedEmail, account.Email, passHash, dovecotUsersFile,
	))
	exec.Run("doveadm reload 2>/dev/null || true")

	return s.db.Model(&account).Update("password", encPass).Error
}

// ChangeQuota updates the storage quota for an email account.
func (s *Service) ChangeQuota(id string, req ChangeQuotaRequest) error {
	if req.QuotaMB <= 0 {
		return fmt.Errorf("quota must be greater than 0 MB")
	}
	var account EmailAccount
	if err := s.db.Where("id = ?", id).First(&account).Error; err != nil {
		return fmt.Errorf("account not found: %w", err)
	}
	return s.db.Model(&account).Update("quota_mb", req.QuotaMB).Error
}

// --- Forwarder management ---

// ListForwarders returns all forwarders for a domain.
func (s *Service) ListForwarders(domainID string) ([]EmailForwarder, error) {
	var fwds []EmailForwarder
	err := s.db.Where("domain_id = ?", domainID).Order("created_at ASC").Find(&fwds).Error
	return fwds, err
}

// CreateForwarder adds a Postfix virtual alias and stores it in the DB.
func (s *Service) CreateForwarder(domainID string, req CreateForwarderRequest) (*EmailForwarder, error) {
	domainName, err := s.domainName(domainID)
	if err != nil {
		return nil, err
	}
	domUID, err := uuid.Parse(domainID)
	if err != nil {
		return nil, fmt.Errorf("invalid domain id: %w", err)
	}

	fwd := &EmailForwarder{
		DomainID:    domUID,
		DomainName:  domainName,
		Source:      req.Source,
		Destination: req.Destination,
		IsCatchAll:  req.IsCatchAll,
	}
	if err := s.db.Create(fwd).Error; err != nil {
		return nil, fmt.Errorf("create forwarder record: %w", err)
	}

	exec, err := s.serverSvc.NewExecutor()
	if err != nil {
		s.db.Delete(fwd)
		return nil, fmt.Errorf("build executor: %w", err)
	}
	defer exec.Close()

	aliasLine := fmt.Sprintf("%s %s\n", req.Source, req.Destination)
	enc := base64.StdEncoding.EncodeToString([]byte(aliasLine))
	if _, err := exec.Run(fmt.Sprintf("echo '%s' | base64 -d >> %s", enc, postfixAliasMaps)); err != nil {
		s.db.Delete(fwd)
		return nil, fmt.Errorf("add alias map entry: %w", err)
	}
	exec.Run(fmt.Sprintf("postmap %s && postfix reload 2>/dev/null || true", postfixAliasMaps))

	return fwd, nil
}

// DeleteForwarder removes a forwarder from the DB and alias map.
func (s *Service) DeleteForwarder(id string) (*EmailForwarder, error) {
	var fwd EmailForwarder
	if err := s.db.Where("id = ?", id).First(&fwd).Error; err != nil {
		return nil, fmt.Errorf("forwarder not found: %w", err)
	}

	exec, err := s.serverSvc.NewExecutor()
	if err == nil {
		defer exec.Close()
		escapedSrc := strings.ReplaceAll(fwd.Source, ".", "\\.")
		exec.Run(fmt.Sprintf("sed -i '/^%s /d' %s", escapedSrc, postfixAliasMaps))
		exec.Run(fmt.Sprintf("postmap %s && postfix reload 2>/dev/null || true", postfixAliasMaps))
	}

	if err := s.db.Delete(&fwd).Error; err != nil {
		return nil, fmt.Errorf("delete forwarder record: %w", err)
	}
	return &fwd, nil
}

// --- Config display ---

// GetConfig returns IMAP/SMTP connection details for display.
func (s *Service) GetConfig(email string) (*EmailConfig, error) {
	cfg, err := s.serverSvc.GetConfig()
	if err != nil {
		return nil, fmt.Errorf("get server config: %w", err)
	}

	host := cfg.SSHHost
	if host == "" {
		// co-located: use the configured hostname or a sensible default
		host = "mail.your-domain.com"
	}

	return &EmailConfig{
		IMAPHost: host,
		IMAPPort: 993,
		SMTPHost: host,
		SMTPPort: 587,
		Username: email,
	}, nil
}

// shellQuote escapes a string for safe single-quote shell usage.
func shellQuote(s string) string {
	return strings.ReplaceAll(s, "'", "'\\''")
}
