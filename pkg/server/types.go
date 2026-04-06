// Package server — types for the persistent session daemon (direct-connect mode).
// Source: src/server/types.ts
package server

// ---------------------------------------------------------------------------
// T90: SessionState — 5-state session lifecycle enum
// Source: src/server/types.ts:26-31
// ---------------------------------------------------------------------------

// SessionState represents the lifecycle state of a server-managed session.
// States: starting -> running -> (detached <-> running) -> stopping -> stopped
type SessionState string

const (
	// SessionStarting is the initial state while the session subprocess is launching.
	SessionStarting SessionState = "starting"
	// SessionRunning means the session is active and connected.
	SessionRunning SessionState = "running"
	// SessionDetached means the client disconnected but the session is still alive.
	SessionDetached SessionState = "detached"
	// SessionStopping means the session is in the process of shutting down.
	SessionStopping SessionState = "stopping"
	// SessionStopped means the session has terminated.
	SessionStopped SessionState = "stopped"
)

// AllSessionStates returns all valid SessionState values.
func AllSessionStates() []SessionState {
	return []SessionState{
		SessionStarting,
		SessionRunning,
		SessionDetached,
		SessionStopping,
		SessionStopped,
	}
}

// IsTerminal returns true if the state represents a finished session.
func (s SessionState) IsTerminal() bool {
	return s == SessionStopped
}

// IsActive returns true if the session is alive (not stopped/stopping).
func (s SessionState) IsActive() bool {
	return s == SessionStarting || s == SessionRunning || s == SessionDetached
}

// Valid returns true if s is one of the five defined states.
func (s SessionState) Valid() bool {
	switch s {
	case SessionStarting, SessionRunning, SessionDetached, SessionStopping, SessionStopped:
		return true
	}
	return false
}

// ---------------------------------------------------------------------------
// T89: ServerConfig — daemon configuration
// Source: src/server/types.ts:13-24
// ---------------------------------------------------------------------------

// ServerConfig holds configuration for the persistent session daemon.
// Used by the `claude serve` subcommand for direct-connect mode.
type ServerConfig struct {
	// Port is the TCP port to listen on.
	Port int `json:"port"`
	// Host is the bind address (e.g. "127.0.0.1", "0.0.0.0").
	Host string `json:"host"`
	// AuthToken is the bearer token required for all API requests.
	AuthToken string `json:"authToken"`
	// Unix is an optional Unix domain socket path. When set, the server
	// listens on this socket instead of (or in addition to) TCP.
	Unix string `json:"unix,omitempty"`
	// IdleTimeoutMs is the idle timeout for detached sessions in milliseconds.
	// 0 means sessions never expire due to inactivity.
	IdleTimeoutMs int `json:"idleTimeoutMs,omitempty"`
	// MaxSessions is the maximum number of concurrent sessions.
	// 0 means unlimited.
	MaxSessions int `json:"maxSessions,omitempty"`
	// Workspace is the default working directory for sessions that don't
	// specify their own cwd.
	Workspace string `json:"workspace,omitempty"`
}

// ConnectResponse is the response shape returned when a client creates or
// attaches to a session. Matches connectResponseSchema in TS.
// Source: src/server/types.ts:5-11
type ConnectResponse struct {
	// SessionID is the server-assigned session identifier.
	SessionID string `json:"session_id"`
	// WSURL is the WebSocket URL to connect to for streaming.
	WSURL string `json:"ws_url"`
	// WorkDir is the resolved working directory (optional).
	WorkDir string `json:"work_dir,omitempty"`
}

// SessionInfo is the in-memory state tracked by the server for each session.
// Source: src/server/types.ts:33-40
type SessionInfo struct {
	// ID is the session identifier.
	ID string `json:"id"`
	// Status is the current lifecycle state.
	Status SessionState `json:"status"`
	// CreatedAt is the Unix timestamp (milliseconds) when the session was created.
	CreatedAt int64 `json:"createdAt"`
	// WorkDir is the working directory of the session.
	WorkDir string `json:"workDir"`
	// SessionKey is an optional key for session resume across restarts.
	SessionKey string `json:"sessionKey,omitempty"`
	// Process is not serialized — it's the OS process handle (nil when detached).
	// In Go this would be *os.Process, but we keep it as an interface for flexibility.
}

// SessionIndexEntry is the persisted metadata for a single session.
// Stored in ~/.claude/server-sessions.json keyed by session key.
// Source: src/server/types.ts:46-55
type SessionIndexEntry struct {
	// SessionID is the server-assigned session ID.
	SessionID string `json:"sessionId"`
	// TranscriptSessionID is the claude transcript session ID for --resume.
	// Same as SessionID for direct sessions.
	TranscriptSessionID string `json:"transcriptSessionId"`
	// CWD is the working directory of the session.
	CWD string `json:"cwd"`
	// PermissionMode is the permission mode (e.g. "auto", "interactive", "deny").
	PermissionMode string `json:"permissionMode,omitempty"`
	// CreatedAt is the Unix timestamp (milliseconds) when the session was created.
	CreatedAt int64 `json:"createdAt"`
	// LastActiveAt is the Unix timestamp (milliseconds) of the last activity.
	LastActiveAt int64 `json:"lastActiveAt"`
}

// SessionIndex maps session keys to their index entries.
// Persisted to ~/.claude/server-sessions.json.
// Source: src/server/types.ts:57
type SessionIndex map[string]SessionIndexEntry
