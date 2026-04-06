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

// HookEvent identifies when a hook fires.
// Source: entrypoints/sdk/coreTypes.ts:25-53
type HookEvent string

const (
	PreToolUse        HookEvent = "PreToolUse"
	PostToolUse       HookEvent = "PostToolUse"
	PostToolUseFailure HookEvent = "PostToolUseFailure"
	Notification      HookEvent = "Notification"
	UserPromptSubmit  HookEvent = "UserPromptSubmit"
	SessionStart      HookEvent = "SessionStart"
	SessionEnd        HookEvent = "SessionEnd"
	Stop              HookEvent = "Stop"
	StopFailure       HookEvent = "StopFailure"
	SubagentStart     HookEvent = "SubagentStart"
	SubagentStop      HookEvent = "SubagentStop"
	PreCompact        HookEvent = "PreCompact"
	PostCompact       HookEvent = "PostCompact"
	PermissionRequest HookEvent = "PermissionRequest"
	PermissionDenied  HookEvent = "PermissionDenied"
	Setup             HookEvent = "Setup"
	TeammateIdle      HookEvent = "TeammateIdle"
	TaskCreated       HookEvent = "TaskCreated"
	TaskCompleted     HookEvent = "TaskCompleted"
	Elicitation       HookEvent = "Elicitation"
	ElicitationResult HookEvent = "ElicitationResult"
	ConfigChange      HookEvent = "ConfigChange"
	WorktreeCreate    HookEvent = "WorktreeCreate"
	WorktreeRemove    HookEvent = "WorktreeRemove"
	InstructionsLoaded HookEvent = "InstructionsLoaded"
	CwdChanged        HookEvent = "CwdChanged"
	FileChanged       HookEvent = "FileChanged"
)

// AllHookEvents lists all recognized hook events in order.
// Source: entrypoints/sdk/coreTypes.ts:25-53
var AllHookEvents = []HookEvent{
	PreToolUse, PostToolUse, PostToolUseFailure, Notification,
	UserPromptSubmit, SessionStart, SessionEnd, Stop, StopFailure,
	SubagentStart, SubagentStop, PreCompact, PostCompact,
	PermissionRequest, PermissionDenied, Setup, TeammateIdle,
	TaskCreated, TaskCompleted, Elicitation, ElicitationResult,
	ConfigChange, WorktreeCreate, WorktreeRemove, InstructionsLoaded,
	CwdChanged, FileChanged,
}

// IsHookEvent checks if a string is a valid hook event.
// Source: types/hooks.ts:22-24
func IsHookEvent(s string) bool {
	for _, e := range AllHookEvents {
		if string(e) == s {
			return true
		}
	}
	return false
}

// HookCommandType identifies the execution method for a hook.
// Source: schemas/hooks.ts:32-163
type HookCommandType string

const (
	HookCommandTypeBash   HookCommandType = "command"
	HookCommandTypePrompt HookCommandType = "prompt"
	HookCommandTypeAgent  HookCommandType = "agent"
	HookCommandTypeHTTP   HookCommandType = "http"
)

// HookCommand defines a single hook action. The Type field discriminates the
// union; only fields relevant to the chosen type are populated.
// Source: schemas/hooks.ts:176-189
type HookCommand struct {
	Type          HookCommandType   `json:"type"`
	Command       string            `json:"command,omitempty"`       // type=command
	Prompt        string            `json:"prompt,omitempty"`        // type=prompt or type=agent
	URL           string            `json:"url,omitempty"`           // type=http
	If            string            `json:"if,omitempty"`            // permission rule filter
	Shell         string            `json:"shell,omitempty"`         // "bash" or "powershell"
	Timeout       int               `json:"timeout,omitempty"`       // seconds
	StatusMessage string            `json:"statusMessage,omitempty"` // custom spinner text
	Once          bool              `json:"once,omitempty"`          // run once then remove
	Async         bool              `json:"async,omitempty"`         // fire-and-forget (type=command)
	AsyncRewake   bool              `json:"asyncRewake,omitempty"`   // async + rewake on exit 2
	Model         string            `json:"model,omitempty"`         // type=prompt or type=agent
	Headers       map[string]string `json:"headers,omitempty"`       // type=http
	AllowedEnvVars []string         `json:"allowedEnvVars,omitempty"` // type=http
}

// HookMatcher pairs a pattern with hook commands.
// Source: schemas/hooks.ts:194-204
type HookMatcher struct {
	Matcher string        `json:"matcher,omitempty"` // tool name, file path, etc.
	Hooks   []HookCommand `json:"hooks"`
}

// HooksSettings maps hook events to their matchers.
// Source: schemas/hooks.ts:211-213
type HooksSettings map[HookEvent][]HookMatcher

// HookConfig is the simplified hook config used by HookRunner.
// Retained for backward compatibility with existing callers.
type HookConfig struct {
	Type    HookEvent `json:"type"`
	Matcher string    `json:"matcher,omitempty"`
	Command string    `json:"command"`
	Timeout int       `json:"timeout,omitempty"`
}

// BaseHookInput is the common input passed to all hooks.
// Source: entrypoints/sdk/coreSchemas.ts:387-411
type BaseHookInput struct {
	SessionID      string `json:"session_id"`
	TranscriptPath string `json:"transcript_path"`
	Cwd            string `json:"cwd"`
	PermissionMode string `json:"permission_mode,omitempty"`
	AgentID        string `json:"agent_id,omitempty"`
	AgentType      string `json:"agent_type,omitempty"`
	HookEventName  string `json:"hook_event_name"`
}

// HookInput extends BaseHookInput with event-specific fields.
// Callers set HookEventName and the relevant fields for the event.
type HookInput struct {
	BaseHookInput

	// PreToolUse, PostToolUse, PostToolUseFailure, PermissionRequest, PermissionDenied
	ToolName  string          `json:"tool_name,omitempty"`
	ToolInput json.RawMessage `json:"tool_input,omitempty"`
	ToolUseID string          `json:"tool_use_id,omitempty"`

	// PostToolUse
	ToolResponse json.RawMessage `json:"tool_response,omitempty"`

	// PostToolUseFailure
	Error       string `json:"error,omitempty"`
	IsInterrupt *bool  `json:"is_interrupt,omitempty"`

	// PermissionDenied
	Reason string `json:"reason,omitempty"`

	// Notification
	Message          string `json:"message,omitempty"`
	Title            string `json:"title,omitempty"`
	NotificationType string `json:"notification_type,omitempty"`

	// UserPromptSubmit
	Prompt string `json:"prompt,omitempty"`

	// SessionStart
	Source string `json:"source,omitempty"` // "startup", "resume", "clear", "compact"
	Model  string `json:"model,omitempty"`

	// Stop, StopFailure, SubagentStop
	StopHookActive       bool   `json:"stop_hook_active,omitempty"`
	LastAssistantMessage string `json:"last_assistant_message,omitempty"`

	// SubagentStart, SubagentStop
	AgentTranscriptPath string `json:"agent_transcript_path,omitempty"`

	// PreCompact, PostCompact
	Trigger            string `json:"trigger,omitempty"` // "manual" or "auto"
	CustomInstructions string `json:"custom_instructions,omitempty"`
	CompactSummary     string `json:"compact_summary,omitempty"`

	// Setup
	SetupTrigger string `json:"setup_trigger,omitempty"` // "init" or "maintenance"

	// SessionEnd
	SessionEndReason string `json:"session_end_reason,omitempty"`

	// TeammateIdle
	TeammateName string `json:"teammate_name,omitempty"`
	TeamName     string `json:"team_name,omitempty"`

	// TaskCreated, TaskCompleted
	TaskID          string `json:"task_id,omitempty"`
	TaskSubject     string `json:"task_subject,omitempty"`
	TaskDescription string `json:"task_description,omitempty"`

	// PermissionRequest
	PermissionSuggestions json.RawMessage `json:"permission_suggestions,omitempty"`

	// ConfigChange
	ConfigSource string `json:"config_source,omitempty"`
	FilePath     string `json:"file_path,omitempty"`

	// InstructionsLoaded
	MemoryType      string   `json:"memory_type,omitempty"`
	LoadReason      string   `json:"load_reason,omitempty"`
	Globs           []string `json:"globs,omitempty"`
	TriggerFilePath string   `json:"trigger_file_path,omitempty"`
	ParentFilePath  string   `json:"parent_file_path,omitempty"`

	// WorktreeCreate, WorktreeRemove
	Name          string `json:"name,omitempty"`
	WorktreePath  string `json:"worktree_path,omitempty"`

	// CwdChanged
	OldCwd string `json:"old_cwd,omitempty"`
	NewCwd string `json:"new_cwd,omitempty"`

	// FileChanged
	FileChangedEvent string `json:"event,omitempty"` // "change", "add", "unlink"
}

// HookJSONOutput is the structured JSON output from a hook.
// Source: types/hooks.ts:169-176
type HookJSONOutput struct {
	// Async mode
	IsAsync      bool `json:"async,omitempty"`
	AsyncTimeout int  `json:"asyncTimeout,omitempty"`

	// Sync mode fields — Source: types/hooks.ts:50-166
	Continue       *bool  `json:"continue,omitempty"`       // default true
	SuppressOutput bool   `json:"suppressOutput,omitempty"` // hide stdout from transcript
	StopReason     string `json:"stopReason,omitempty"`     // message when continue=false
	Decision       string `json:"decision,omitempty"`       // "approve" or "block"
	Reason         string `json:"reason,omitempty"`         // explanation
	SystemMessage  string `json:"systemMessage,omitempty"`  // warning to user

	// Event-specific output
	HookSpecificOutput json.RawMessage `json:"hookSpecificOutput,omitempty"`
}

// IsSyncResponse returns true if the output is a synchronous response.
// Source: types/hooks.ts:184-186
func (h *HookJSONOutput) IsSyncResponse() bool {
	return !h.IsAsync
}

// ShouldContinue returns whether the hook allows continuation (default true).
func (h *HookJSONOutput) ShouldContinue() bool {
	if h.Continue == nil {
		return true
	}
	return *h.Continue
}

// HookOutcome categorizes the hook execution result.
// Source: types/hooks.ts:264
type HookOutcome string

const (
	OutcomeSuccess          HookOutcome = "success"
	OutcomeBlocking         HookOutcome = "blocking"
	OutcomeNonBlockingError HookOutcome = "non_blocking_error"
	OutcomeCancelled        HookOutcome = "cancelled"
)

// HookBlockingError pairs a blocking error message with the hook command.
// Source: types/hooks.ts:243-246
type HookBlockingError struct {
	BlockingError string `json:"blockingError"`
	Command       string `json:"command"`
}

// HookResult is the outcome of running a hook.
// Source: types/hooks.ts:260-275
type HookResult struct {
	ExitCode int
	Stdout   string
	Stderr   string
	Blocked  bool   // true if hook returned exit 2 on PreToolUse
	Message  string // optional message from the hook

	Outcome                     HookOutcome
	PreventContinuation         bool
	StopReason                  string
	PermissionBehavior          string // "ask", "deny", "allow", "passthrough"
	HookPermissionDecisionReason string
	AdditionalContext           string
	InitialUserMessage          string
	BlockingError               *HookBlockingError
	SystemMessage               string
	JSONOutput                  *HookJSONOutput // parsed structured output
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
// Exit code semantics — Source: utils/hooks/hooksConfigManager.ts:
//   - Exit 0: success
//   - Exit 2: blocking error (show stderr to model, prevent continuation)
//   - Other non-zero: non-blocking error (show stderr to user only)
func (r *HookRunner) Run(ctx context.Context, hookType HookEvent, toolName string, toolInput json.RawMessage) (*HookResult, error) {
	for _, h := range r.hooks {
		if HookEvent(h.Type) != hookType {
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
			"HOOK_EVENT_NAME="+string(hookType),
			"HOOK_TYPE="+string(hookType), // backward compat
			"TOOL_NAME="+toolName,
			"TOOL_INPUT="+string(toolInput),
		)

		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		err := cmd.Run()
		exitCode := 0
		if err != nil {
			if hookCtx.Err() != nil {
				return &HookResult{
					ExitCode: -1,
					Stderr:   hookCtx.Err().Error(),
					Message:  hookCtx.Err().Error(),
					Outcome:  OutcomeCancelled,
				}, hookCtx.Err()
			}
			if exitErr, ok := err.(*exec.ExitError); ok {
				exitCode = exitErr.ExitCode()
			} else {
				return &HookResult{
					ExitCode: -1,
					Stderr:   err.Error(),
					Message:  err.Error(),
					Outcome:  OutcomeNonBlockingError,
				}, err
			}
		}

		result := &HookResult{
			ExitCode: exitCode,
			Stdout:   stdout.String(),
			Stderr:   stderr.String(),
		}

		// Try to parse structured JSON output from stdout
		result.JSONOutput = parseHookJSONOutput(stdout.String())

		// Apply exit code semantics
		// Source: utils/hooks/hooksConfigManager.ts exit code docs
		switch {
		case exitCode == 0:
			result.Outcome = OutcomeSuccess
			result.Message = strings.TrimSpace(stderr.String())
			// Apply JSON output fields if present
			if result.JSONOutput != nil {
				applyJSONOutput(result, h.Command)
			}
		case exitCode == 2:
			// Exit code 2 = blocking error
			result.Outcome = OutcomeBlocking
			result.Blocked = true
			result.Message = strings.TrimSpace(stderr.String())
			result.PreventContinuation = true
			result.BlockingError = &HookBlockingError{
				BlockingError: strings.TrimSpace(stderr.String()),
				Command:       h.Command,
			}
		default:
			// Other non-zero = non-blocking error
			result.Outcome = OutcomeNonBlockingError
			result.Message = strings.TrimSpace(stderr.String())
			// For backward compat: PreToolUse with non-zero still blocks
			if hookType == PreToolUse {
				result.Blocked = true
			}
		}

		// Return early if hook blocks or has meaningful state to propagate
		if result.Blocked || result.PreventContinuation || result.Outcome != OutcomeSuccess {
			return result, nil
		}
	}
	return &HookResult{Outcome: OutcomeSuccess}, nil
}

// parseHookJSONOutput attempts to parse structured JSON output from hook stdout.
func parseHookJSONOutput(stdout string) *HookJSONOutput {
	trimmed := strings.TrimSpace(stdout)
	if trimmed == "" || trimmed[0] != '{' {
		return nil
	}
	var output HookJSONOutput
	if err := json.Unmarshal([]byte(trimmed), &output); err != nil {
		return nil
	}
	return &output
}

// applyJSONOutput applies structured JSON output to the hook result.
// Handles both PreToolUse decisions ("approve"/"block") and Stop hook
// decisions ("continue"/"stop").
// Source: types/hooks.ts:50-166, query/stopHooks.ts decision resolution
func applyJSONOutput(result *HookResult, command string) {
	out := result.JSONOutput
	if out == nil {
		return
	}

	// "continue" field: explicit false prevents continuation (default true).
	// Source: types/hooks.ts:50
	if !out.ShouldContinue() {
		result.PreventContinuation = true
		result.StopReason = out.StopReason
	}

	// "decision" field: handles both PreToolUse and Stop hook semantics.
	// PreToolUse: "approve" or "block"
	// Stop hooks: "continue" or "stop"
	// Source: query/stopHooks.ts — decision='continue'/'stop' resolution (T52)
	switch out.Decision {
	case "block":
		result.Blocked = true
		result.Outcome = OutcomeBlocking
		result.BlockingError = &HookBlockingError{
			BlockingError: out.Reason,
			Command:       command,
		}
	case "stop":
		result.PreventContinuation = true
		if out.Reason != "" {
			result.StopReason = out.Reason
		} else if out.StopReason != "" {
			result.StopReason = out.StopReason
		}
	case "continue":
		// Explicit continue: ensure continuation is not prevented.
		result.PreventContinuation = false
	}

	if out.SystemMessage != "" {
		result.SystemMessage = out.SystemMessage
	}
}

// RunForOrchestrator adapts Run to the tools.HookRunner interface.
// Returns (blocked, message, error).
func (r *HookRunner) RunForOrchestrator(ctx context.Context, hookType string, toolName string, toolInput json.RawMessage) (bool, string, error) {
	result, err := r.Run(ctx, HookEvent(hookType), toolName, toolInput)
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
