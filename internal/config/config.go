package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Listen                     string   `yaml:"listen"`
	LogLevel                   string   `yaml:"log-level"`
	EnableCORS                 bool     `yaml:"enable-cors"`
	AllowedOrigins             []string `yaml:"allowed-origins"`
	EnableStatusPage           bool     `yaml:"enable-status-page"`
	ReadHeaderTimeoutSec       int      `yaml:"read-header-timeout-sec"`
	ReadTimeoutSec             int      `yaml:"read-timeout-sec"`
	WriteTimeoutSec            int      `yaml:"write-timeout-sec"`
	IdleTimeoutSec             int      `yaml:"idle-timeout-sec"`
	ShutdownTimeoutSec         int      `yaml:"shutdown-timeout-sec"`
	EnableExecCommand          bool     `yaml:"enable-exec-command"`
	ExecCommandAllowlist       []string `yaml:"exec-command-allowlist"`
	ExecCommandWorkingDir      string   `yaml:"exec-command-working-dir"`
	ExecCommandTimeoutSec      int      `yaml:"exec-command-timeout-sec"`
	ExecCommandMaxOutputBytes  int      `yaml:"exec-command-max-output-bytes"`
	ExecCommandAllowBackground bool     `yaml:"exec-command-allow-background"`
}

type ExecCommandSettings struct {
	Enabled         bool
	Allowlist       []string
	WorkingDir      string
	TimeoutSec      int
	MaxOutputBytes  int
	AllowBackground bool
}

var Cfg *Config

func LoadConfig(path string) error {
	_ = godotenv.Load()

	cfg := defaultConfig()
	configPath := resolveConfigPath(path)
	if err := mergeConfigFile(cfg, configPath); err != nil {
		return err
	}

	applyEnvOverrides(cfg)
	cfg.Sanitize()

	if err := cfg.Validate(); err != nil {
		return err
	}

	Cfg = cfg
	return nil
}

func CurrentExecCommandSettings() ExecCommandSettings {
	if Cfg == nil {
		cfg := defaultConfig()
		cfg.Sanitize()
		return cfg.execCommandSettings()
	}
	return Cfg.execCommandSettings()
}

func defaultConfig() *Config {
	return &Config{
		Listen:                     ":8000",
		LogLevel:                   "info",
		EnableCORS:                 true,
		AllowedOrigins:             []string{"*"},
		EnableStatusPage:           true,
		ReadHeaderTimeoutSec:       10,
		ReadTimeoutSec:             0,
		WriteTimeoutSec:            0,
		IdleTimeoutSec:             120,
		ShutdownTimeoutSec:         10,
		EnableExecCommand:          true,
		ExecCommandAllowlist:       defaultExecCommandAllowlist(),
		ExecCommandWorkingDir:      defaultExecCommandWorkingDir(),
		ExecCommandTimeoutSec:      20,
		ExecCommandMaxOutputBytes:  64 * 1024,
		ExecCommandAllowBackground: true,
	}
}

func resolveConfigPath(path string) string {
	if envPath := strings.TrimSpace(os.Getenv("CONFIG_FILE")); envPath != "" {
		return envPath
	}
	if strings.TrimSpace(path) != "" {
		return path
	}
	return "config.yaml"
}

func mergeConfigFile(cfg *Config, path string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) && filepath.Base(path) == "config.yaml" {
			return nil
		}
		return fmt.Errorf("read config file %q: %w", path, err)
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return fmt.Errorf("parse config file %q: %w", path, err)
	}

	return nil
}

func applyEnvOverrides(cfg *Config) {
	if port := strings.TrimSpace(os.Getenv("PORT")); port != "" {
		cfg.Listen = ":" + port
	}
	if listen := strings.TrimSpace(os.Getenv("LISTEN")); listen != "" {
		cfg.Listen = listen
	}
	if level := strings.TrimSpace(os.Getenv("LOG_LEVEL")); level != "" {
		cfg.LogLevel = level
	}
	if origins := parseCSVEnv("ALLOWED_ORIGINS"); len(origins) > 0 {
		cfg.AllowedOrigins = origins
	}
	if enableCORS, ok := parseBoolEnv("ENABLE_CORS"); ok {
		cfg.EnableCORS = enableCORS
	}
	if enableStatusPage, ok := parseBoolEnv("ENABLE_STATUS_PAGE"); ok {
		cfg.EnableStatusPage = enableStatusPage
	}
	if timeout, ok := parseIntEnv("READ_HEADER_TIMEOUT_SEC"); ok {
		cfg.ReadHeaderTimeoutSec = timeout
	}
	if timeout, ok := parseIntEnv("READ_TIMEOUT_SEC"); ok {
		cfg.ReadTimeoutSec = timeout
	}
	if timeout, ok := parseIntEnv("WRITE_TIMEOUT_SEC"); ok {
		cfg.WriteTimeoutSec = timeout
	}
	if timeout, ok := parseIntEnv("IDLE_TIMEOUT_SEC"); ok {
		cfg.IdleTimeoutSec = timeout
	}
	if timeout, ok := parseIntEnv("SHUTDOWN_TIMEOUT_SEC"); ok {
		cfg.ShutdownTimeoutSec = timeout
	}
	if enabled, ok := parseBoolEnv("ENABLE_EXEC_COMMAND"); ok {
		cfg.EnableExecCommand = enabled
	}
	if prefixes := parseCSVEnv("EXEC_COMMAND_ALLOWLIST"); len(prefixes) > 0 {
		cfg.ExecCommandAllowlist = prefixes
	}
	if workingDir := strings.TrimSpace(os.Getenv("EXEC_COMMAND_WORKING_DIR")); workingDir != "" {
		cfg.ExecCommandWorkingDir = workingDir
	}
	if timeout, ok := parseIntEnv("EXEC_COMMAND_TIMEOUT_SEC"); ok {
		cfg.ExecCommandTimeoutSec = timeout
	}
	if maxOutput, ok := parseIntEnv("EXEC_COMMAND_MAX_OUTPUT_BYTES"); ok {
		cfg.ExecCommandMaxOutputBytes = maxOutput
	}
	if allowBackground, ok := parseBoolEnv("EXEC_COMMAND_ALLOW_BACKGROUND"); ok {
		cfg.ExecCommandAllowBackground = allowBackground
	}
}

func (c *Config) Sanitize() {
	c.Listen = strings.TrimSpace(c.Listen)
	if c.Listen == "" {
		c.Listen = ":8000"
	}
	if !strings.Contains(c.Listen, ":") {
		c.Listen = ":" + c.Listen
	}

	c.LogLevel = strings.ToLower(strings.TrimSpace(c.LogLevel))
	if c.LogLevel == "" {
		c.LogLevel = "info"
	}

	if len(c.AllowedOrigins) == 0 {
		c.AllowedOrigins = []string{"*"}
	} else {
		c.AllowedOrigins = normalizeOrigins(c.AllowedOrigins)
	}

	if c.ReadHeaderTimeoutSec < 0 {
		c.ReadHeaderTimeoutSec = 0
	}
	if c.ReadTimeoutSec < 0 {
		c.ReadTimeoutSec = 0
	}
	if c.WriteTimeoutSec < 0 {
		c.WriteTimeoutSec = 0
	}
	if c.IdleTimeoutSec < 0 {
		c.IdleTimeoutSec = 0
	}
	if c.ShutdownTimeoutSec <= 0 {
		c.ShutdownTimeoutSec = 10
	}

	c.ExecCommandAllowlist = normalizeCommandPrefixes(c.ExecCommandAllowlist)
	if len(c.ExecCommandAllowlist) == 0 {
		c.ExecCommandAllowlist = defaultExecCommandAllowlist()
	}

	c.ExecCommandWorkingDir = strings.TrimSpace(c.ExecCommandWorkingDir)
	if c.ExecCommandWorkingDir == "" {
		c.ExecCommandWorkingDir = defaultExecCommandWorkingDir()
	}
	if absDir, err := filepath.Abs(c.ExecCommandWorkingDir); err == nil {
		c.ExecCommandWorkingDir = absDir
	}

	if c.ExecCommandTimeoutSec <= 0 {
		c.ExecCommandTimeoutSec = 20
	}
	if c.ExecCommandTimeoutSec > 600 {
		c.ExecCommandTimeoutSec = 600
	}
	if c.ExecCommandMaxOutputBytes <= 0 {
		c.ExecCommandMaxOutputBytes = 64 * 1024
	}
	if c.ExecCommandMaxOutputBytes > 1024*1024 {
		c.ExecCommandMaxOutputBytes = 1024 * 1024
	}
}

func (c *Config) Validate() error {
	switch c.LogLevel {
	case "debug", "info", "warn", "error":
	default:
		return fmt.Errorf("invalid log-level %q", c.LogLevel)
	}

	if len(c.AllowedOrigins) == 0 {
		return fmt.Errorf("allowed-origins cannot be empty")
	}

	info, err := os.Stat(c.ExecCommandWorkingDir)
	if err != nil {
		return fmt.Errorf("invalid exec-command-working-dir %q: %w", c.ExecCommandWorkingDir, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("exec-command-working-dir %q is not a directory", c.ExecCommandWorkingDir)
	}

	if len(c.ExecCommandAllowlist) == 0 {
		return fmt.Errorf("exec-command-allowlist cannot be empty")
	}

	return nil
}

func (c *Config) execCommandSettings() ExecCommandSettings {
	settings := ExecCommandSettings{
		Enabled:         c.EnableExecCommand,
		Allowlist:       append([]string(nil), c.ExecCommandAllowlist...),
		WorkingDir:      c.ExecCommandWorkingDir,
		TimeoutSec:      c.ExecCommandTimeoutSec,
		MaxOutputBytes:  c.ExecCommandMaxOutputBytes,
		AllowBackground: c.ExecCommandAllowBackground,
	}
	if len(settings.Allowlist) == 0 {
		settings.Allowlist = defaultExecCommandAllowlist()
	}
	if strings.TrimSpace(settings.WorkingDir) == "" {
		settings.WorkingDir = defaultExecCommandWorkingDir()
	}
	if settings.TimeoutSec <= 0 {
		settings.TimeoutSec = 20
	}
	if settings.MaxOutputBytes <= 0 {
		settings.MaxOutputBytes = 64 * 1024
	}
	return settings
}

func parseBoolEnv(key string) (bool, bool) {
	value, ok := os.LookupEnv(key)
	if !ok {
		return false, false
	}

	parsed, err := strconv.ParseBool(strings.TrimSpace(value))
	if err != nil {
		return false, false
	}

	return parsed, true
}

func parseIntEnv(key string) (int, bool) {
	value, ok := os.LookupEnv(key)
	if !ok {
		return 0, false
	}

	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return 0, false
	}

	return parsed, true
}

func parseCSVEnv(key string) []string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	items := make([]string, 0, len(parts))
	for _, part := range parts {
		item := strings.TrimSpace(part)
		if item == "" {
			continue
		}
		items = append(items, item)
	}
	return items
}

func normalizeOrigins(origins []string) []string {
	seen := make(map[string]struct{}, len(origins))
	normalized := make([]string, 0, len(origins))

	for _, origin := range origins {
		item := strings.TrimSpace(origin)
		if item == "" {
			continue
		}
		if item == "*" {
			return []string{"*"}
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		normalized = append(normalized, item)
	}

	if len(normalized) == 0 {
		return []string{"*"}
	}

	return normalized
}

func normalizeCommandPrefixes(prefixes []string) []string {
	seen := make(map[string]struct{}, len(prefixes))
	normalized := make([]string, 0, len(prefixes))
	for _, prefix := range prefixes {
		item := strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(prefix)), " "))
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		normalized = append(normalized, item)
	}
	return normalized
}

func defaultExecCommandWorkingDir() string {
	wd, err := os.Getwd()
	if err != nil || strings.TrimSpace(wd) == "" {
		return "."
	}
	return wd
}

func defaultExecCommandAllowlist() []string {
	return []string{
		"pwd",
		"ls",
		"cat",
		"head",
		"tail",
		"sed",
		"grep",
		"rg",
		"find",
		"wc",
		"stat",
		"git status",
		"git diff",
		"git log",
		"git show",
		"go test",
		"go run",
		"go env",
		"npm run",
		"npm test",
		"pnpm run",
		"pnpm test",
		"yarn run",
		"yarn test",
	}
}
