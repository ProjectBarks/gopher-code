// Package bridge implements the CCR (Cloud Code Runner) bridge protocol types
// and subsystem for remote control sessions.
// Source: src/bridge/types.ts
package bridge

import "time"

// ---------------------------------------------------------------------------
// Constants — user-visible strings (verbatim match with TS source)
// ---------------------------------------------------------------------------

// DefaultSessionTimeoutMS is the per-session timeout (24 hours).
const DefaultSessionTimeoutMS = 24 * 60 * 60 * 1000 // 86_400_000

// BridgeLoginInstruction is the guidance shown for bridge auth errors.
const BridgeLoginInstruction = "Remote Control is only available with claude.ai subscriptions. Please use `/login` to sign in with your claude.ai account."

// BridgeLoginError is the full error printed when `claude remote-control`
// is run without auth.
const BridgeLoginError = "Error: You must be logged in to use Remote Control.\n\n" + BridgeLoginInstruction

// RemoteControlDisconnectedMsg is shown when the user disconnects Remote Control.
const RemoteControlDisconnectedMsg = "Remote Control disconnected."

// ---------------------------------------------------------------------------
// String enums
// ---------------------------------------------------------------------------

// SessionDoneStatus indicates how a session ended.
type SessionDoneStatus string

const (
	SessionDoneCompleted   SessionDoneStatus = "completed"
	SessionDoneFailed      SessionDoneStatus = "failed"
	SessionDoneInterrupted SessionDoneStatus = "interrupted"
)

// SessionActivityType categorises session activity events.
type SessionActivityType string

const (
	ActivityToolStart SessionActivityType = "tool_start"
	ActivityText      SessionActivityType = "text"
	ActivityResult    SessionActivityType = "result"
	ActivityError     SessionActivityType = "error"
)

// SpawnMode controls how `claude remote-control` chooses session working directories.
type SpawnMode string

const (
	SpawnModeSingleSession SpawnMode = "single-session"
	SpawnModeWorktree      SpawnMode = "worktree"
	SpawnModeSameDir       SpawnMode = "same-dir"
)

// BridgeWorkerType identifies the kind of worker this bridge instance runs.
type BridgeWorkerType string

const (
	WorkerTypeClaudeCode          BridgeWorkerType = "claude_code"
	WorkerTypeClaudeCodeAssistant BridgeWorkerType = "claude_code_assistant"
)

// WorkDataType distinguishes session work from healthchecks.
type WorkDataType string

const (
	WorkDataTypeSession     WorkDataType = "session"
	WorkDataTypeHealthcheck WorkDataType = "healthcheck"
)

// ---------------------------------------------------------------------------
// Protocol structs — JSON wire types for the environments API
// ---------------------------------------------------------------------------

// WorkData is the inner payload of a WorkResponse.
type WorkData struct {
	Type WorkDataType `json:"type"`
	ID   string       `json:"id"`
}

// WorkResponse is the top-level work item returned by poll-for-work.
type WorkResponse struct {
	ID            string   `json:"id"`
	Type          string   `json:"type"`
	EnvironmentID string   `json:"environment_id"`
	State         string   `json:"state"`
	Data          WorkData `json:"data"`
	Secret        string   `json:"secret"` // base64url-encoded JSON
	CreatedAt     string   `json:"created_at"`
}

// GitInfo holds source-repository metadata inside a WorkSecret source entry.
type GitInfo struct {
	Type  string `json:"type"`
	Repo  string `json:"repo"`
	Ref   string `json:"ref,omitempty"`
	Token string `json:"token,omitempty"`
}

// WorkSecretSource is one source entry inside WorkSecret.
type WorkSecretSource struct {
	Type    string   `json:"type"`
	GitInfo *GitInfo `json:"git_info,omitempty"`
}

// WorkSecretAuth is one auth entry inside WorkSecret.
type WorkSecretAuth struct {
	Type  string `json:"type"`
	Token string `json:"token"`
}

// WorkSecret is the decrypted secret payload attached to a work item.
type WorkSecret struct {
	Version              int                `json:"version"`
	SessionIngressToken  string             `json:"session_ingress_token"`
	APIBaseURL           string             `json:"api_base_url"`
	Sources              []WorkSecretSource `json:"sources"`
	Auth                 []WorkSecretAuth   `json:"auth"`
	ClaudeCodeArgs       map[string]string  `json:"claude_code_args,omitempty"`
	MCPConfig            any                `json:"mcp_config,omitempty"`
	EnvironmentVariables map[string]string  `json:"environment_variables,omitempty"`
	UseCodeSessions      *bool              `json:"use_code_sessions,omitempty"`
}

// SessionActivity records a single activity event within a session.
type SessionActivity struct {
	Type      SessionActivityType `json:"type"`
	Summary   string              `json:"summary"`
	Timestamp float64             `json:"timestamp"` // Unix ms (matches TS number)
}

// ---------------------------------------------------------------------------
// BridgeConfig — configuration for a bridge instance
// ---------------------------------------------------------------------------

// BridgeConfig holds all configuration for a bridge instance.
type BridgeConfig struct {
	Dir                string    `json:"dir"`
	MachineName        string    `json:"machine_name"`
	Branch             string    `json:"branch"`
	GitRepoURL         *string   `json:"git_repo_url"`
	MaxSessions        int       `json:"max_sessions"`
	SpawnMode          SpawnMode `json:"spawn_mode"`
	Verbose            bool      `json:"verbose"`
	Sandbox            bool      `json:"sandbox"`
	BridgeID           string    `json:"bridge_id"`
	WorkerType         string    `json:"worker_type"`
	EnvironmentID      string    `json:"environment_id"`
	ReuseEnvironmentID string    `json:"reuse_environment_id,omitempty"`
	APIBaseURL         string    `json:"api_base_url"`
	SessionIngressURL  string    `json:"session_ingress_url"`
	DebugFile          string    `json:"debug_file,omitempty"`
	SessionTimeoutMS   *int      `json:"session_timeout_ms,omitempty"`
}

// NewRemoteControlConfig builds a BridgeConfig with sensible defaults for a
// `claude remote-control` invocation. Later tasks will populate auth/URL
// fields from the credential store and environment.
func NewRemoteControlConfig(dir string, machineName string) BridgeConfig {
	return BridgeConfig{
		Dir:            dir,
		MachineName:    machineName,
		MaxSessions:    1,
		SpawnMode:      SpawnModeSingleSession,
		WorkerType:     string(WorkerTypeClaudeCode),
		SessionTimeoutMS: func() *int { v := DefaultSessionTimeoutMS; return &v }(),
	}
}

// ---------------------------------------------------------------------------
// Permission event
// ---------------------------------------------------------------------------

// PermissionResponse is the inner response payload of a PermissionResponseEvent.
type PermissionResponse struct {
	Subtype   string         `json:"subtype"`
	RequestID string         `json:"request_id"`
	Response  map[string]any `json:"response"`
}

// PermissionResponseEvent is a control_response sent back to a session.
type PermissionResponseEvent struct {
	Type     string             `json:"type"`
	Response PermissionResponse `json:"response"`
}

// ---------------------------------------------------------------------------
// Registration response
// ---------------------------------------------------------------------------

// RegisterEnvironmentResponse is returned by registerBridgeEnvironment.
type RegisterEnvironmentResponse struct {
	EnvironmentID     string `json:"environment_id"`
	EnvironmentSecret string `json:"environment_secret"`
}

// HeartbeatResponse is returned by heartbeatWork.
type HeartbeatResponse struct {
	LeaseExtended bool   `json:"lease_extended"`
	State         string `json:"state"`
}

// ---------------------------------------------------------------------------
// Session spawn options
// ---------------------------------------------------------------------------

// SessionSpawnOpts configures a new session spawn.
type SessionSpawnOpts struct {
	SessionID          string `json:"session_id"`
	SDKURL             string `json:"sdk_url"`
	AccessToken        string `json:"access_token"`
	UseCcrV2           bool   `json:"use_ccr_v2,omitempty"`
	WorkerEpoch        *int   `json:"worker_epoch,omitempty"`
	OnFirstUserMessage func(text string) `json:"-"`
}

// ---------------------------------------------------------------------------
// Interfaces (Go style — defined separately, consumed by later tasks)
// ---------------------------------------------------------------------------

// SessionHandle represents a running session.
type SessionHandle struct {
	SessionID       string
	Done            <-chan SessionDoneStatus
	Kill            func()
	ForceKill       func()
	Activities      []SessionActivity
	CurrentActivity *SessionActivity
	AccessToken     string
	LastStderr      []string
	WriteStdin      func(data string)
	UpdateAccessToken func(token string)
}

// SessionSpawner creates new sessions.
type SessionSpawner interface {
	Spawn(opts SessionSpawnOpts, dir string) *SessionHandle
}

// BridgeLogger abstracts the bridge UI logging surface.
type BridgeLogger interface {
	PrintBanner(config BridgeConfig, environmentID string)
	LogSessionStart(sessionID string, prompt string)
	LogSessionComplete(sessionID string, duration time.Duration)
	LogSessionFailed(sessionID string, err string)
	LogStatus(message string)
	LogVerbose(message string)
	LogError(message string)
	LogReconnected(disconnectedDuration time.Duration)
	UpdateIdleStatus()
	UpdateReconnectingStatus(delayStr string, elapsedStr string)
	UpdateSessionStatus(sessionID string, elapsed string, activity SessionActivity, trail []string)
	ClearStatus()
	SetRepoInfo(repoName string, branch string)
	SetDebugLogPath(path string)
	SetAttached(sessionID string)
	UpdateFailedStatus(err string)
	ToggleQR()
	UpdateSessionCount(active int, max int, mode SpawnMode)
	SetSpawnModeDisplay(mode *SpawnMode)
	AddSession(sessionID string, url string)
	UpdateSessionActivity(sessionID string, activity SessionActivity)
	SetSessionTitle(sessionID string, title string)
	RemoveSession(sessionID string)
	RefreshDisplay()
}

// BridgeAPIClient abstracts the bridge API surface (implemented in T173).
type BridgeAPIClient interface {
	RegisterBridgeEnvironment(config BridgeConfig) (*RegisterEnvironmentResponse, error)
	PollForWork(environmentID string, environmentSecret string, reclaimOlderThanMS *int) (*WorkResponse, error)
	AcknowledgeWork(environmentID string, workID string, sessionToken string) error
	StopWork(environmentID string, workID string, force bool) error
	DeregisterEnvironment(environmentID string) error
	SendPermissionResponseEvent(sessionID string, event PermissionResponseEvent, sessionToken string) error
	ArchiveSession(sessionID string) error
	ReconnectSession(environmentID string, sessionID string) error
	HeartbeatWork(environmentID string, workID string, sessionToken string) (*HeartbeatResponse, error)
}
