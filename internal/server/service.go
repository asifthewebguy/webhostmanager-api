package server

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/asifthewebguy/webhostmanager-api/pkg/crypto"
)

const metricsKey = "server_metrics"
const metricsTTL = 2 * time.Minute // treat cached metrics as stale after 2 minutes

// Service handles server metrics collection and connection config.
type Service struct {
	db     *gorm.DB
	encKey string
}

func NewService(db *gorm.DB, encKey string) *Service {
	return &Service{db: db, encKey: encKey}
}

// GetConfig reads the single-row server_config record.
func (s *Service) GetConfig() (*Config, error) {
	var cfg Config
	if err := s.db.First(&cfg).Error; err != nil {
		return nil, err
	}
	return &cfg, nil
}

// UpdateConfig persists a new server connection configuration.
func (s *Service) UpdateConfig(req *ConnectionRequest) error {
	cfg := Config{ID: 1}
	s.db.FirstOrCreate(&cfg, Config{ID: 1})

	cfg.ConnectionMode = req.ConnectionMode
	if req.ConnectionMode == "ssh" {
		cfg.SSHHost = req.SSHHost
		port := req.SSHPort
		if port == 0 {
			port = 22
		}
		cfg.SSHPort = port
		cfg.SSHUser = req.SSHUser
		cfg.SSHAuthType = req.SSHAuthType
		if req.SSHKey != "" {
			enc, err := crypto.Encrypt(req.SSHKey, s.encKey)
			if err != nil {
				return fmt.Errorf("encrypt ssh key: %w", err)
			}
			cfg.SSHKeyEncrypted = enc
		}
		if req.SSHPassword != "" {
			enc, err := crypto.Encrypt(req.SSHPassword, s.encKey)
			if err != nil {
				return fmt.Errorf("encrypt ssh password: %w", err)
			}
			cfg.SSHPasswordEncrypted = enc
		}
	}
	return s.db.Save(&cfg).Error
}

// TestConnection verifies the connection without persisting any changes.
func (s *Service) TestConnection(req *ConnectionRequest) error {
	exec, err := s.buildExecutorFromRequest(req)
	if err != nil {
		return err
	}
	defer exec.Close()

	out, err := exec.Run("echo ok")
	if err != nil {
		return err
	}
	if strings.TrimSpace(out) != "ok" {
		return fmt.Errorf("unexpected response: %q", out)
	}
	return nil
}

// CollectAndCache gathers live metrics and writes them to the metrics_cache table.
// Called by the scheduler every 30 seconds.
func (s *Service) CollectAndCache() error {
	exec, err := s.newExecutor()
	if err != nil {
		return fmt.Errorf("build executor: %w", err)
	}
	defer exec.Close()

	m, err := s.collectMetrics(exec)
	if err != nil {
		return fmt.Errorf("collect metrics: %w", err)
	}
	m.CollectedAt = time.Now()

	data, err := json.Marshal(m)
	if err != nil {
		return err
	}
	return s.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "metric_name"}},
		DoUpdates: clause.AssignmentColumns([]string{"value", "collected_at"}),
	}).Create(&MetricsCache{
		MetricName:  metricsKey,
		Value:       string(data),
		CollectedAt: time.Now(),
	}).Error
}

// GetMetrics returns metrics from cache, refreshing if stale or absent.
func (s *Service) GetMetrics() (*Metrics, error) {
	var cache MetricsCache
	err := s.db.Where("metric_name = ?", metricsKey).First(&cache).Error

	if err != nil || time.Since(cache.CollectedAt) > metricsTTL {
		if collectErr := s.CollectAndCache(); collectErr != nil {
			if err != nil {
				return nil, fmt.Errorf("no cached metrics and collection failed: %w", collectErr)
			}
			// collection failed but stale cache exists — fall through and return stale data
		} else {
			s.db.Where("metric_name = ?", metricsKey).First(&cache)
		}
	}

	var m Metrics
	if unmarshalErr := json.Unmarshal([]byte(cache.Value), &m); unmarshalErr != nil {
		return nil, fmt.Errorf("unmarshal cached metrics: %w", unmarshalErr)
	}
	return &m, nil
}

// GetServerInfo collects static server information (hostname, OS, kernel).
func (s *Service) GetServerInfo() (*ServerInfo, error) {
	exec, err := s.newExecutor()
	if err != nil {
		return nil, err
	}
	defer exec.Close()

	mode := "local"
	if cfg, e := s.GetConfig(); e == nil {
		mode = cfg.ConnectionMode
	}

	info := &ServerInfo{ConnectionMode: mode}
	if v, e := exec.Run("hostname"); e == nil {
		info.Hostname = v
	}
	if v, e := exec.Run("uname -r"); e == nil {
		info.Kernel = v
	}
	if v, e := exec.Run(`grep '^PRETTY_NAME' /etc/os-release | cut -d= -f2 | tr -d '"'`); e == nil && v != "" {
		info.OS = v
	} else if v, e := exec.Run("uname -s"); e == nil {
		info.OS = v
	}
	return info, nil
}

// GetSummary returns resource counts (populated by later phases).
func (s *Service) GetSummary() *Summary {
	return &Summary{}
}

// --- internal helpers ---

func (s *Service) newExecutor() (Executor, error) {
	cfg, err := s.GetConfig()
	if err != nil || cfg.ConnectionMode != "ssh" {
		return &LocalExecutor{}, nil
	}

	key := ""
	if cfg.SSHKeyEncrypted != "" {
		key, _ = crypto.Decrypt(cfg.SSHKeyEncrypted, s.encKey)
	}
	password := ""
	if cfg.SSHPasswordEncrypted != "" {
		password, _ = crypto.Decrypt(cfg.SSHPasswordEncrypted, s.encKey)
	}
	port := cfg.SSHPort
	if port == 0 {
		port = 22
	}
	return NewSSHExecutor(cfg.SSHHost, port, cfg.SSHUser, cfg.SSHAuthType, key, password)
}

func (s *Service) buildExecutorFromRequest(req *ConnectionRequest) (Executor, error) {
	if req.ConnectionMode == "local" {
		return &LocalExecutor{}, nil
	}
	port := req.SSHPort
	if port == 0 {
		port = 22
	}
	return NewSSHExecutor(req.SSHHost, port, req.SSHUser, req.SSHAuthType, req.SSHKey, req.SSHPassword)
}

func (s *Service) collectMetrics(exec Executor) (*Metrics, error) {
	m := &Metrics{}

	// CPU: two /proc/stat samples 1 second apart
	stat1, err1 := exec.Run("cat /proc/stat | head -1")
	time.Sleep(1 * time.Second)
	stat2, err2 := exec.Run("cat /proc/stat | head -1")
	if err1 == nil && err2 == nil {
		if cpu, err := parseCPU(stat1, stat2); err == nil {
			m.CPUPercent = roundTo1(cpu)
		}
	}

	// RAM
	if out, err := exec.Run("cat /proc/meminfo"); err == nil {
		if used, total, pct, err := parseMeminfo(out); err == nil {
			m.RAMUsedMB = used
			m.RAMTotalMB = total
			m.RAMPercent = roundTo1(pct)
		}
	}

	// Disk (root filesystem)
	if out, err := exec.Run("df -B1 /"); err == nil {
		if usedGB, totalGB, pct, err := parseDisk(out); err == nil {
			m.DiskUsedGB = roundTo2(usedGB)
			m.DiskTotalGB = roundTo2(totalGB)
			m.DiskPercent = roundTo1(pct)
		}
	}

	// Uptime
	if out, err := exec.Run("cat /proc/uptime"); err == nil {
		m.Uptime = parseUptime(out)
	}

	return m, nil
}

// --- parsers ---

func parseCPU(stat1, stat2 string) (float64, error) {
	extract := func(line string) (total, idle int64, err error) {
		fields := strings.Fields(line)
		if len(fields) < 5 {
			return 0, 0, fmt.Errorf("too few fields in /proc/stat line")
		}
		var v [8]int64
		for i := 1; i < len(fields) && i <= 8; i++ {
			v[i-1], _ = strconv.ParseInt(fields[i], 10, 64)
		}
		// user=0 nice=1 system=2 idle=3 iowait=4 irq=5 softirq=6 steal=7
		idle = v[3] + v[4]
		total = v[0] + v[1] + v[2] + v[3] + v[4] + v[5] + v[6] + v[7]
		return total, idle, nil
	}

	t1, i1, err := extract(stat1)
	if err != nil {
		return 0, err
	}
	t2, i2, err := extract(stat2)
	if err != nil {
		return 0, err
	}

	totalDiff := float64(t2 - t1)
	idleDiff := float64(i2 - i1)
	if totalDiff == 0 {
		return 0, nil
	}
	cpu := (1.0 - idleDiff/totalDiff) * 100.0
	if cpu < 0 {
		cpu = 0
	}
	return cpu, nil
}

func parseMeminfo(output string) (usedMB, totalMB int64, pct float64, err error) {
	var memTotal, memAvailable int64
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		val, e := strconv.ParseInt(fields[1], 10, 64)
		if e != nil {
			continue
		}
		switch strings.TrimSuffix(fields[0], ":") {
		case "MemTotal":
			memTotal = val
		case "MemAvailable":
			memAvailable = val
		}
	}
	if memTotal == 0 {
		return 0, 0, 0, fmt.Errorf("MemTotal not found")
	}
	usedKB := memTotal - memAvailable
	totalMB = memTotal / 1024
	usedMB = usedKB / 1024
	pct = float64(usedKB) / float64(memTotal) * 100.0
	return usedMB, totalMB, pct, nil
}

func parseDisk(output string) (usedGB, totalGB, pct float64, err error) {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) < 2 {
		return 0, 0, 0, fmt.Errorf("unexpected df output")
	}
	fields := strings.Fields(lines[1])
	if len(fields) < 4 {
		return 0, 0, 0, fmt.Errorf("unexpected df columns")
	}
	total, e1 := strconv.ParseInt(fields[1], 10, 64)
	used, e2 := strconv.ParseInt(fields[2], 10, 64)
	if e1 != nil || e2 != nil {
		return 0, 0, 0, fmt.Errorf("parse df values")
	}
	const gb = float64(1024 * 1024 * 1024)
	totalGB = float64(total) / gb
	usedGB = float64(used) / gb
	if total > 0 {
		pct = float64(used) / float64(total) * 100.0
	}
	return usedGB, totalGB, pct, nil
}

func parseUptime(output string) string {
	fields := strings.Fields(output)
	if len(fields) < 1 {
		return "unknown"
	}
	secs, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return "unknown"
	}
	d := time.Duration(int64(secs)) * time.Second
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	mins := int(d.Minutes()) % 60
	switch {
	case days > 0:
		return fmt.Sprintf("%dd %dh %dm", days, hours, mins)
	case hours > 0:
		return fmt.Sprintf("%dh %dm", hours, mins)
	default:
		return fmt.Sprintf("%dm", mins)
	}
}

func roundTo1(f float64) float64 { return float64(int(f*10+0.5)) / 10 }
func roundTo2(f float64) float64 { return float64(int(f*100+0.5)) / 100 }
