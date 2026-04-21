package tools

import (
	"encoding/json"
	"strings"
	"testing"

	"zai-proxy/internal/config"
)

// setTestConfig replaces the global config for the duration of the test.
func setTestConfig(t *testing.T, cfg *config.Config) {
	t.Helper()
	cfg.Sanitize()
	config.SetConfigForTest(cfg)
}

func TestExecuteBuiltinToolExecCommand_Success(t *testing.T) {
	setTestConfig(t, &config.Config{
		EnableExecCommand:          true,
		ExecCommandAllowlist:       []string{"pwd"},
		ExecCommandWorkingDir:      t.TempDir(),
		ExecCommandTimeoutSec:      5,
		ExecCommandMaxOutputBytes:  4096,
		ExecCommandAllowBackground: false,
	})

	raw := ExecuteBuiltinTool("exec_command", `{"command":"pwd"}`)

	var resp struct {
		OK     bool `json:"ok"`
		Result struct {
			Success    bool   `json:"success"`
			ExitCode   int    `json:"exit_code"`
			WorkingDir string `json:"working_dir"`
			Output     string `json:"output"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if !resp.OK {
		t.Fatalf("expected exec_command tool success envelope, got %s", raw)
	}
	if !resp.Result.Success {
		t.Fatalf("expected command success, got %s", raw)
	}
	if resp.Result.ExitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", resp.Result.ExitCode)
	}
	if !strings.Contains(resp.Result.Output, resp.Result.WorkingDir) {
		t.Fatalf("expected output to contain working dir, got %q", resp.Result.Output)
	}
}

func TestExecuteBuiltinToolExecCommand_DeniedByAllowlist(t *testing.T) {
	setTestConfig(t, &config.Config{
		EnableExecCommand:          true,
		ExecCommandAllowlist:       []string{"pwd"},
		ExecCommandWorkingDir:      t.TempDir(),
		ExecCommandTimeoutSec:      5,
		ExecCommandMaxOutputBytes:  4096,
		ExecCommandAllowBackground: false,
	})

	raw := ExecuteBuiltinTool("exec_command", `{"command":"rm -rf ."}`)

	var resp struct {
		OK    bool `json:"ok"`
		Error struct {
			Type string `json:"type"`
		} `json:"error"`
	}
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if resp.OK {
		t.Fatalf("expected allowlist rejection, got %s", raw)
	}
	if resp.Error.Type != "command_not_allowed" {
		t.Fatalf("error.type = %q, want command_not_allowed", resp.Error.Type)
	}
}

func TestExecuteBuiltinToolExecCommand_RejectsShellSyntax(t *testing.T) {
	setTestConfig(t, &config.Config{
		EnableExecCommand:          true,
		ExecCommandAllowlist:       []string{"pwd"},
		ExecCommandWorkingDir:      t.TempDir(),
		ExecCommandTimeoutSec:      5,
		ExecCommandMaxOutputBytes:  4096,
		ExecCommandAllowBackground: false,
	})

	raw := ExecuteBuiltinTool("exec_command", `{"command":"pwd && ls"}`)

	var resp struct {
		OK    bool `json:"ok"`
		Error struct {
			Type string `json:"type"`
		} `json:"error"`
	}
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if resp.OK {
		t.Fatalf("expected shell syntax rejection, got %s", raw)
	}
	if resp.Error.Type != "invalid_command" {
		t.Fatalf("error.type = %q, want invalid_command", resp.Error.Type)
	}
}
