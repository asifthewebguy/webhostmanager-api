package setup

import "time"

// State is the GORM model for the setup_state table (single row).
type State struct {
	ID          int        `gorm:"primaryKey;default:1" json:"id"`
	CurrentStep int        `gorm:"not null;default:0"   json:"current_step"`
	IsComplete  bool       `gorm:"not null;default:false" json:"is_complete"`
	CompletedAt *time.Time `                             json:"completed_at,omitempty"`
}

func (State) TableName() string { return "setup_state" }

// Setting is the GORM model for the settings key-value store.
type Setting struct {
	ID          string    `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"-"`
	Key         string    `gorm:"uniqueIndex;not null"                           json:"key"`
	Value       string    `gorm:"not null;default:''"                            json:"-"`
	IsEncrypted bool      `gorm:"not null;default:false"                         json:"-"`
	CreatedAt   time.Time `                                                       json:"-"`
	UpdatedAt   time.Time `                                                       json:"-"`
}

func (Setting) TableName() string { return "settings" }

// StatusResponse is returned by GET /api/v1/setup/status.
type StatusResponse struct {
	IsComplete  bool `json:"is_complete"`
	CurrentStep int  `json:"current_step"`
	TotalSteps  int  `json:"total_steps"`
}

// --- Step request bodies ---

// Step1Request acknowledges the welcome screen.
type Step1Request struct{}

// Step2Request creates the Super Admin account.
type Step2Request struct {
	Username        string `json:"username"         binding:"required,min=3,max=100"`
	Email           string `json:"email"            binding:"required,email"`
	Password        string `json:"password"         binding:"required,min=8"`
	ConfirmPassword string `json:"confirm_password" binding:"required"`
}

// Step3Request configures the server connection.
type Step3Request struct {
	ConnectionType string `json:"connection_type" binding:"required,oneof=local ssh"`
	SSHHost        string `json:"ssh_host"`
	SSHPort        int    `json:"ssh_port"`
	SSHUser        string `json:"ssh_user"`
	SSHAuthType    string `json:"ssh_auth_type"` // "key" or "password"
	SSHKey         string `json:"ssh_key"`
	SSHPassword    string `json:"ssh_password"`
}

// Step4Request selects the reverse proxy.
type Step4Request struct {
	ProxyType string `json:"proxy_type" binding:"required,oneof=nginx apache"`
}

// Step5Request configures a default domain (optional).
type Step5Request struct {
	Skip          bool   `json:"skip"`
	DefaultDomain string `json:"default_domain"`
}

// Step6Request configures Cloudflare (optional).
type Step6Request struct {
	Skip      bool   `json:"skip"`
	APIToken  string `json:"api_token"`
	ZoneID    string `json:"zone_id"`
	AccountID string `json:"account_id"`
}

// Step7Request configures notification channels (optional).
type Step7Request struct {
	Skip           bool   `json:"skip"`
	SMTPHost       string `json:"smtp_host"`
	SMTPPort       int    `json:"smtp_port"`
	SMTPUser       string `json:"smtp_user"`
	SMTPPassword   string `json:"smtp_password"`
	SMTPFrom       string `json:"smtp_from"`
	SlackWebhook   string `json:"slack_webhook"`
	DiscordWebhook string `json:"discord_webhook"`
}
