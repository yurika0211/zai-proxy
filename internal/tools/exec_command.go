package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"zai-proxy/internal/config"
)

type execCommandRequest struct {
	Command         string   `json:"command"`
	Args            []string `json:"args"`
	Description     string   `json:"description"`
	Workdir         string   `json:"workdir"`
	TimeoutSec      int      `json:"timeout_sec"`
	RunInBackground bool     `json:"run_in_background"`
}

type backgroundProcessInfo struct {
	PID        int
	Command    []string
	WorkingDir string
	LogPath    string
	StartedAt  time.Time
}

type cappedOutputBuffer struct {
	limit int
	total int
	buf   []byte
}

var backgroundProcesses sync.Map

func executeExecCommandTool(args map[string]interface{}) string {
	settings := config.CurrentExecCommandSettings()
	if !settings.Enabled {
		return marshalDisabledTool("exec_command", "exec_command 已在当前代理配置中禁用。")
	}

	req, err := decodeExecCommandRequest(args)
	if err != nil {
		return marshalToolExecutionResponse(toolExecutionResponse{
			OK:   false,
			Tool: "exec_command",
			Error: &toolExecutionError{
				Type:    "invalid_exec_request",
				Message: err.Error(),
			},
		})
	}

	argv, err := buildExecCommandArgv(req)
	if err != nil {
		return marshalToolExecutionResponse(toolExecutionResponse{
			OK:   false,
			Tool: "exec_command",
			Error: &toolExecutionError{
				Type:    "invalid_command",
				Message: err.Error(),
			},
		})
	}

	matchedPrefix, allowed := matchAllowedCommandPrefix(argv, settings.Allowlist)
	if !allowed {
		return marshalToolExecutionResponse(toolExecutionResponse{
			OK:   false,
			Tool: "exec_command",
			Error: &toolExecutionError{
				Type:    "command_not_allowed",
				Message: fmt.Sprintf("command is not in allowlist; allowed prefixes: %s", strings.Join(settings.Allowlist, ", ")),
			},
		})
	}

	workdir, err := resolveExecCommandWorkdir(req.Workdir, settings.WorkingDir)
	if err != nil {
		return marshalToolExecutionResponse(toolExecutionResponse{
			OK:   false,
			Tool: "exec_command",
			Error: &toolExecutionError{
				Type:    "invalid_workdir",
				Message: err.Error(),
			},
		})
	}

	resolvedCommand, err := exec.LookPath(argv[0])
	if err != nil {
		return marshalToolExecutionResponse(toolExecutionResponse{
			OK:   false,
			Tool: "exec_command",
			Error: &toolExecutionError{
				Type:    "command_not_found",
				Message: err.Error(),
			},
		})
	}

	if req.RunInBackground {
		if !settings.AllowBackground {
			return marshalToolExecutionResponse(toolExecutionResponse{
				OK:   false,
				Tool: "exec_command",
				Error: &toolExecutionError{
					Type:    "background_not_allowed",
					Message: "background execution is disabled in the current proxy config",
				},
			})
		}
		return startBackgroundExecCommand(req, argv, resolvedCommand, matchedPrefix, workdir)
	}

	return runForegroundExecCommand(req, argv, resolvedCommand, matchedPrefix, workdir, settings)
}

func decodeExecCommandRequest(args map[string]interface{}) (execCommandRequest, error) {
	data, err := json.Marshal(args)
	if err != nil {
		return execCommandRequest{}, err
	}

	var req execCommandRequest
	if err := json.Unmarshal(data, &req); err != nil {
		return execCommandRequest{}, err
	}

	if strings.TrimSpace(req.Command) == "" {
		return execCommandRequest{}, fmt.Errorf("command is required")
	}

	return req, nil
}

func buildExecCommandArgv(req execCommandRequest) ([]string, error) {
	if len(req.Args) > 0 {
		parts, err := splitExecCommandLine(req.Command)
		if err != nil {
			return nil, err
		}
		if len(parts) != 1 {
			return nil, fmt.Errorf("when args is provided, command must contain only the executable name")
		}
		if err := validateExecutableName(parts[0]); err != nil {
			return nil, err
		}
		argv := append([]string{parts[0]}, req.Args...)
		return argv, nil
	}

	parts, err := splitExecCommandLine(req.Command)
	if err != nil {
		return nil, err
	}
	if err := validateExecutableName(parts[0]); err != nil {
		return nil, err
	}
	return parts, nil
}

func splitExecCommandLine(command string) ([]string, error) {
	var args []string
	var current strings.Builder
	inSingle := false
	inDouble := false
	escaped := false

	flush := func() {
		if current.Len() == 0 {
			return
		}
		args = append(args, current.String())
		current.Reset()
	}

	for _, r := range command {
		switch {
		case escaped:
			current.WriteRune(r)
			escaped = false
		case inSingle:
			if r == '\'' {
				inSingle = false
			} else {
				current.WriteRune(r)
			}
		case inDouble:
			if r == '"' {
				inDouble = false
			} else if r == '\\' {
				escaped = true
			} else if isUnsupportedShellRune(r) {
				return nil, fmt.Errorf("shell operators are not supported in exec_command")
			} else {
				current.WriteRune(r)
			}
		default:
			if isUnsupportedShellRune(r) {
				return nil, fmt.Errorf("shell operators are not supported in exec_command")
			}
			if r == '\\' {
				escaped = true
				continue
			}
			if r == '\'' {
				inSingle = true
				continue
			}
			if r == '"' {
				inDouble = true
				continue
			}
			if r == ' ' || r == '\t' {
				flush()
				continue
			}
			current.WriteRune(r)
		}
	}

	if escaped || inSingle || inDouble {
		return nil, fmt.Errorf("unterminated quoting or escaping in command")
	}

	flush()
	if len(args) == 0 {
		return nil, fmt.Errorf("command is required")
	}
	return args, nil
}

func isUnsupportedShellRune(r rune) bool {
	switch r {
	case '|', '&', ';', '<', '>', '`', '\n', '\r':
		return true
	default:
		return false
	}
}

func validateExecutableName(name string) error {
	if strings.Contains(name, "/") || strings.Contains(name, `\\`) {
		return fmt.Errorf("executable must be a bare command name, not a path")
	}
	return nil
}

func matchAllowedCommandPrefix(argv []string, allowlist []string) (string, bool) {
	normalizedArgv := make([]string, len(argv))
	for i, arg := range argv {
		normalizedArgv[i] = strings.ToLower(arg)
	}

	for _, prefix := range allowlist {
		parts := strings.Fields(strings.ToLower(prefix))
		if len(parts) == 0 || len(parts) > len(normalizedArgv) {
			continue
		}
		matched := true
		for i := range parts {
			if normalizedArgv[i] != parts[i] {
				matched = false
				break
			}
		}
		if matched {
			return prefix, true
		}
	}

	return "", false
}

func resolveExecCommandWorkdir(requested, base string) (string, error) {
	if strings.TrimSpace(base) == "" {
		base = "."
	}
	baseAbs, err := filepath.Abs(base)
	if err != nil {
		return "", err
	}

	candidate := strings.TrimSpace(requested)
	if candidate == "" {
		candidate = baseAbs
	} else if !filepath.IsAbs(candidate) {
		candidate = filepath.Join(baseAbs, candidate)
	}

	candidateAbs, err := filepath.Abs(candidate)
	if err != nil {
		return "", err
	}
	if !isWithinBaseDir(baseAbs, candidateAbs) {
		return "", fmt.Errorf("workdir must stay within %s", baseAbs)
	}
	info, err := os.Stat(candidateAbs)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("workdir %q is not a directory", candidateAbs)
	}
	return candidateAbs, nil
}

func isWithinBaseDir(baseDir, candidate string) bool {
	rel, err := filepath.Rel(baseDir, candidate)
	if err != nil {
		return false
	}
	if rel == "." {
		return true
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator))
}

func runForegroundExecCommand(req execCommandRequest, argv []string, resolvedCommand, matchedPrefix, workdir string, settings config.ExecCommandSettings) string {
	timeoutSec := req.TimeoutSec
	if timeoutSec <= 0 {
		timeoutSec = settings.TimeoutSec
	}
	if timeoutSec > 600 {
		timeoutSec = 600
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSec)*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, resolvedCommand, argv[1:]...)
	cmd.Dir = workdir

	output := &cappedOutputBuffer{limit: settings.MaxOutputBytes}
	cmd.Stdout = output
	cmd.Stderr = output

	runErr := cmd.Run()
	timedOut := errors.Is(ctx.Err(), context.DeadlineExceeded)
	if runErr != nil && cmd.ProcessState == nil && !timedOut {
		return marshalToolExecutionResponse(toolExecutionResponse{
			OK:   false,
			Tool: "exec_command",
			Error: &toolExecutionError{
				Type:    "command_start_failed",
				Message: runErr.Error(),
			},
		})
	}

	exitCode := 0
	if cmd.ProcessState != nil {
		exitCode = cmd.ProcessState.ExitCode()
	} else if timedOut {
		exitCode = -1
	}

	result := map[string]interface{}{
		"command":                 req.Command,
		"argv":                    argv,
		"resolved_command":        resolvedCommand,
		"matched_allowlist":       matchedPrefix,
		"working_dir":             workdir,
		"background":              false,
		"success":                 runErr == nil,
		"exit_code":               exitCode,
		"timed_out":               timedOut,
		"timeout_sec":             timeoutSec,
		"output":                  output.String(),
		"output_truncated":        output.Truncated(),
		"captured_output_bytes":   len(output.buf),
		"total_output_bytes_seen": output.total,
	}
	if req.Description != "" {
		result["description"] = req.Description
	}
	if runErr != nil {
		result["command_error"] = runErr.Error()
	}

	return marshalToolExecutionResponse(toolExecutionResponse{
		OK:     true,
		Tool:   "exec_command",
		Result: result,
	})
}

func startBackgroundExecCommand(req execCommandRequest, argv []string, resolvedCommand, matchedPrefix, workdir string) string {
	logFile, err := os.CreateTemp("", "zai-proxy-exec-*.log")
	if err != nil {
		return marshalToolExecutionResponse(toolExecutionResponse{
			OK:   false,
			Tool: "exec_command",
			Error: &toolExecutionError{
				Type:    "background_log_error",
				Message: err.Error(),
			},
		})
	}

	cmd := exec.Command(resolvedCommand, argv[1:]...)
	cmd.Dir = workdir
	cmd.Stdout = logFile
	cmd.Stderr = logFile

	if err := cmd.Start(); err != nil {
		_ = logFile.Close()
		_ = os.Remove(logFile.Name())
		return marshalToolExecutionResponse(toolExecutionResponse{
			OK:   false,
			Tool: "exec_command",
			Error: &toolExecutionError{
				Type:    "command_start_failed",
				Message: err.Error(),
			},
		})
	}

	info := backgroundProcessInfo{
		PID:        cmd.Process.Pid,
		Command:    argv,
		WorkingDir: workdir,
		LogPath:    logFile.Name(),
		StartedAt:  time.Now().UTC(),
	}
	backgroundProcesses.Store(info.PID, info)
	go func(pid int) {
		_ = cmd.Wait()
		backgroundProcesses.Delete(pid)
		_ = logFile.Close()
	}(info.PID)

	result := map[string]interface{}{
		"command":           req.Command,
		"argv":              argv,
		"resolved_command":  resolvedCommand,
		"matched_allowlist": matchedPrefix,
		"working_dir":       workdir,
		"background":        true,
		"pid":               info.PID,
		"log_path":          info.LogPath,
		"started_at":        info.StartedAt.Format(time.RFC3339),
	}
	if req.Description != "" {
		result["description"] = req.Description
	}

	return marshalToolExecutionResponse(toolExecutionResponse{
		OK:     true,
		Tool:   "exec_command",
		Result: result,
	})
}

func (b *cappedOutputBuffer) Write(p []byte) (int, error) {
	b.total += len(p)
	remaining := b.limit - len(b.buf)
	if remaining > 0 {
		if len(p) > remaining {
			b.buf = append(b.buf, p[:remaining]...)
		} else {
			b.buf = append(b.buf, p...)
		}
	}
	return len(p), nil
}

func (b *cappedOutputBuffer) String() string {
	return string(b.buf)
}

func (b *cappedOutputBuffer) Truncated() bool {
	return b.total > len(b.buf)
}
