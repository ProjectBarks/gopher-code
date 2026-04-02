package hooks

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// HookType identifies when a hook fires.
type HookType string

const (
	PreToolUse  HookType = "PreToolUse"
	PostToolUse HookType = "PostToolUse"
	Notification HookType = "Notification"
	Stop        HookType = "Stop"
)

// HookConfig defines a single hook.
type HookConfig struct {
	Type    HookType `json:"type"`
	Matcher string   `json:"matcher,omitempty"` // tool name pattern (glob)
	Command string   `json:"command"`
	Timeout int      `json:"timeout,omitempty"` // seconds, default 30
}

// HookResult is the outcome of running a hook.
type HookResult struct {
	ExitCode int
	Stdout   string
	Stderr   string
	Blocked  bool   // true if hook returned non-zero on PreToolUse
	Message  string // optional message from the hook
}

// HookRunner manages and executes hooks.
type HookRunner struct {
	hooks []HookConfig
}

// NewHookRunner creates a HookRunner from a list of hook configs.
func NewHookRunner(hooks []HookConfig) *HookRunner {
	return &HookRunner{hooks: hooks}
}

// Run executes all matching hooks of the given type.
// For PreToolUse, if any hook returns non-zero, the tool is blocked.
func (r *HookRunner) Run(ctx context.Context, hookType HookType, toolName string, toolInput json.RawMessage) (*HookResult, error) {
	for _, h := range r.hooks {
		if h.Type != hookType {
			continue
		}
		if h.Matcher != "" && !matchToolName(h.Matcher, toolName) {
			continue
		}

		timeout := h.Timeout
		if timeout <= 0 {
			timeout = 30
		}

		hookCtx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
		defer cancel()

		cmd := exec.CommandContext(hookCtx, "sh", "-c", h.Command)

		// Pass hook context as environment variables
		cmd.Env = append(os.Environ(),
			"HOOK_TYPE="+string(hookType),
			"TOOL_NAME="+toolName,
			"TOOL_INPUT="+string(toolInput),
		)

		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		err := cmd.Run()
		exitCode := 0
		if err != nil {
			// Check for context timeout/cancellation first
			if hookCtx.Err() != nil {
				return &HookResult{
					ExitCode: -1,
					Stderr:   hookCtx.Err().Error(),
					Message:  hookCtx.Err().Error(),
				}, hookCtx.Err()
			}
			if exitErr, ok := err.(*exec.ExitError); ok {
				exitCode = exitErr.ExitCode()
			} else {
				return &HookResult{
					ExitCode: -1,
					Stderr:   err.Error(),
					Message:  err.Error(),
				}, err
			}
		}

		result := &HookResult{
			ExitCode: exitCode,
			Stdout:   stdout.String(),
			Stderr:   stderr.String(),
			Blocked:  hookType == PreToolUse && exitCode != 0,
			Message:  strings.TrimSpace(stderr.String()),
		}

		if result.Blocked {
			return result, nil
		}
	}
	return &HookResult{}, nil
}

// RunForOrchestrator adapts Run to the tools.HookRunner interface.
// Returns (blocked, message, error).
func (r *HookRunner) RunForOrchestrator(ctx context.Context, hookType string, toolName string, toolInput json.RawMessage) (bool, string, error) {
	result, err := r.Run(ctx, HookType(hookType), toolName, toolInput)
	if err != nil {
		return false, err.Error(), err
	}
	if result == nil {
		return false, "", nil
	}
	return result.Blocked, result.Message, nil
}

func matchToolName(pattern, name string) bool {
	matched, _ := filepath.Match(pattern, name)
	return matched
}
