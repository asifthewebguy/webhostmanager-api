package server

import "time"

// Config is the server connection configuration (single row).
type Config struct {
	ID                   int       `gorm:"primaryKey;autoIncrement:false"`
	ConnectionMode       string    `gorm:"column:connection_mode;not null;default:'local'"`
	SSHHost              string    `gorm:"column:ssh_host;not null;default:''"`
	SSHPort              int       `gorm:"column:ssh_port;not null;default:22"`
	SSHUser              string    `gorm:"column:ssh_user;not null;default:''"`
	SSHAuthType          string    `gorm:"column:ssh_auth_type;not null;default:'key'"`
	SSHKeyEncrypted      string    `gorm:"column:ssh_key_encrypted;not null;default:''"`
	SSHPasswordEncrypted string    `gorm:"column:ssh_password_encrypted;not null;default:''"`
	UpdatedAt            time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

func (Config) TableName() string { return "server_config" }

// MetricsCache stores the last collected server metrics snapshot.
type MetricsCache struct {
	MetricName  string    `gorm:"primaryKey;column:metric_name"`
	Value       string    `gorm:"column:value;not null;default:''"`
	CollectedAt time.Time `gorm:"column:collected_at;not null"`
}

func (MetricsCache) TableName() string { return "metrics_cache" }

// Metrics is the live server performance snapshot.
type Metrics struct {
	CPUPercent  float64   `json:"cpu_percent"`
	RAMPercent  float64   `json:"ram_percent"`
	DiskPercent float64   `json:"disk_percent"`
	RAMUsedMB   int64     `json:"ram_used_mb"`
	RAMTotalMB  int64     `json:"ram_total_mb"`
	DiskUsedGB  float64   `json:"disk_used_gb"`
	DiskTotalGB float64   `json:"disk_total_gb"`
	Uptime      string    `json:"uptime"`
	CollectedAt time.Time `json:"collected_at"`
}

// ServerInfo holds static information about the server.
type ServerInfo struct {
	Hostname       string `json:"hostname"`
	OS             string `json:"os"`
	Kernel         string `json:"kernel"`
	ConnectionMode string `json:"connection_mode"`
}

// Summary holds counts of all managed resources.
type Summary struct {
	Domains       int `json:"domains"`
	SSLCerts      int `json:"ssl_certs"`
	WordPress     int `json:"wordpress"`
	EmailAccounts int `json:"email_accounts"`
}

// ConnectionRequest is the body for POST /server/connection.
type ConnectionRequest struct {
	ConnectionMode string `json:"connection_mode" binding:"required,oneof=local ssh"`
	SSHHost        string `json:"ssh_host"`
	SSHPort        int    `json:"ssh_port"`
	SSHUser        string `json:"ssh_user"`
	SSHAuthType    string `json:"ssh_auth_type"` // "key" or "password"
	SSHKey         string `json:"ssh_key"`
	SSHPassword    string `json:"ssh_password"`
}
