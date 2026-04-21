package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig_MissingDefaultFileFallsBackToDefaults(t *testing.T) {
	t.Setenv("PORT", "")
	t.Setenv("LISTEN", "")
	t.Setenv("LOG_LEVEL", "")
	t.Setenv("ALLOWED_ORIGINS", "")
	t.Setenv("ENABLE_CORS", "")
	t.Setenv("ENABLE_STATUS_PAGE", "")
	t.Setenv("READ_HEADER_TIMEOUT_SEC", "")
	t.Setenv("READ_TIMEOUT_SEC", "")
	t.Setenv("WRITE_TIMEOUT_SEC", "")
	t.Setenv("IDLE_TIMEOUT_SEC", "")
	t.Setenv("SHUTDOWN_TIMEOUT_SEC", "")
	t.Setenv("ENABLE_EXEC_COMMAND", "")
	t.Setenv("EXEC_COMMAND_ALLOWLIST", "")
	t.Setenv("EXEC_COMMAND_WORKING_DIR", "")
	t.Setenv("EXEC_COMMAND_TIMEOUT_SEC", "")
	t.Setenv("EXEC_COMMAND_MAX_OUTPUT_BYTES", "")
	t.Setenv("EXEC_COMMAND_ALLOW_BACKGROUND", "")
	t.Setenv("CONFIG_FILE", "")

	if err := LoadConfig(filepath.Join(t.TempDir(), "config.yaml")); err != nil {
		t.Fatalf("LoadConfig returned error: %v", err)
	}

	if Cfg.Listen != ":8000" {
		t.Fatalf("expected default listen :8000, got %q", Cfg.Listen)
	}
	if Cfg.LogLevel != "info" {
		t.Fatalf("expected default log level info, got %q", Cfg.LogLevel)
	}
	if !Cfg.EnableStatusPage {
		t.Fatalf("expected status page enabled by default")
	}
	if !Cfg.EnableExecCommand {
		t.Fatalf("expected exec command enabled by default")
	}
	if len(Cfg.ExecCommandAllowlist) == 0 {
		t.Fatalf("expected non-empty exec command allowlist")
	}
}

func TestLoadConfig_YAMLAndEnvOverride(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "custom.yaml")
	content := []byte(`
listen: ":9000"
log-level: "warn"
enable-cors: false
allowed-origins:
  - "https://one.example"
enable-status-page: false
read-header-timeout-sec: 3
shutdown-timeout-sec: 4
enable-exec-command: false
exec-command-allowlist:
  - "pwd"
exec-command-working-dir: "."
exec-command-timeout-sec: 7
exec-command-max-output-bytes: 2048
exec-command-allow-background: false
`)
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	t.Setenv("CONFIG_FILE", "")
	t.Setenv("PORT", "9100")
	t.Setenv("LISTEN", "")
	t.Setenv("LOG_LEVEL", "debug")
	t.Setenv("ALLOWED_ORIGINS", "https://two.example, https://three.example")
	t.Setenv("ENABLE_CORS", "true")
	t.Setenv("ENABLE_STATUS_PAGE", "true")
	t.Setenv("READ_HEADER_TIMEOUT_SEC", "9")
	t.Setenv("SHUTDOWN_TIMEOUT_SEC", "11")
	t.Setenv("ENABLE_EXEC_COMMAND", "true")
	t.Setenv("EXEC_COMMAND_ALLOWLIST", "pwd,git status")
	t.Setenv("EXEC_COMMAND_WORKING_DIR", dir)
	t.Setenv("EXEC_COMMAND_TIMEOUT_SEC", "21")
	t.Setenv("EXEC_COMMAND_MAX_OUTPUT_BYTES", "4096")
	t.Setenv("EXEC_COMMAND_ALLOW_BACKGROUND", "true")

	if err := LoadConfig(path); err != nil {
		t.Fatalf("LoadConfig returned error: %v", err)
	}

	if Cfg.Listen != ":9100" {
		t.Fatalf("expected env port override, got %q", Cfg.Listen)
	}
	if Cfg.LogLevel != "debug" {
		t.Fatalf("expected env log level override, got %q", Cfg.LogLevel)
	}
	if !Cfg.EnableCORS {
		t.Fatalf("expected env enable-cors override to win")
	}
	if !Cfg.EnableStatusPage {
		t.Fatalf("expected env enable-status-page override to win")
	}
	if Cfg.ReadHeaderTimeoutSec != 9 {
		t.Fatalf("expected env read-header-timeout override, got %d", Cfg.ReadHeaderTimeoutSec)
	}
	if Cfg.ShutdownTimeoutSec != 11 {
		t.Fatalf("expected env shutdown-timeout override, got %d", Cfg.ShutdownTimeoutSec)
	}
	if len(Cfg.AllowedOrigins) != 2 {
		t.Fatalf("expected 2 allowed origins, got %d", len(Cfg.AllowedOrigins))
	}
	if !Cfg.EnableExecCommand {
		t.Fatalf("expected env enable-exec-command override to win")
	}
	if Cfg.ExecCommandTimeoutSec != 21 {
		t.Fatalf("expected env exec timeout override, got %d", Cfg.ExecCommandTimeoutSec)
	}
	if Cfg.ExecCommandMaxOutputBytes != 4096 {
		t.Fatalf("expected env exec max output override, got %d", Cfg.ExecCommandMaxOutputBytes)
	}
	if !Cfg.ExecCommandAllowBackground {
		t.Fatalf("expected env exec background override to win")
	}
	if Cfg.ExecCommandWorkingDir != dir {
		t.Fatalf("expected env exec working dir override, got %q", Cfg.ExecCommandWorkingDir)
	}
	if len(Cfg.ExecCommandAllowlist) != 2 {
		t.Fatalf("expected 2 exec prefixes, got %d", len(Cfg.ExecCommandAllowlist))
	}
}
