package session

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/projectbarks/gopher-code/pkg/compact"
	"github.com/projectbarks/gopher-code/pkg/message"
	"github.com/projectbarks/gopher-code/pkg/permissions"
	"github.com/projectbarks/gopher-code/pkg/provider"
)

// PermissionPolicyProvider is the interface for permission checking.
// Duplicated here to avoid import cycles with the tools package.
type PermissionPolicyProvider interface {
	Check(ctx context.Context, toolName string, toolID string) permissions.PermissionDecision
}

// SessionConfig holds configuration for a session.
type SessionConfig struct {
	Model           string
	SystemPrompt    string
	MaxTurns        int
	TokenBudget     compact.TokenBudget
	PermissionMode  permissions.PermissionMode
	ThinkingEnabled bool
	ThinkingBudget  int     // default 10000
	JSONSchema        string  `json:"json_schema,omitempty"`
	MaxBudgetUSD      float64 `json:"max_budget_usd,omitempty"`
	TokenBudgetTarget int     `json:"token_budget_target,omitempty"` // +500k feature: output token target
	FallbackModel     string  `json:"fallback_model,omitempty"`      // --fallback-model: switch on 529 exhaustion
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() SessionConfig {
	return SessionConfig{
		Model:          "claude-sonnet-4-20250514",
		MaxTurns:       100,
		TokenBudget:    compact.DefaultBudget(),
		PermissionMode: permissions.AutoApprove,
	}
}

// ModelUsageEntry tracks token usage for a specific model.
// Source: bootstrap/state.ts — modelUsage
type ModelUsageEntry struct {
	InputTokens              int     `json:"input_tokens"`
	OutputTokens             int     `json:"output_tokens"`
	CacheCreationInputTokens int     `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int     `json:"cache_read_input_tokens"`
	CostUSD                  float64 `json:"cost_usd"`
}

// SessionState holds the mutable state of a conversation session.
// Source: bootstrap/state.ts — State type
type SessionState struct {
	ID                       string            `json:"id"`
	ParentSessionID          string            `json:"parent_session_id,omitempty"`
	Name                     string            `json:"name,omitempty"`
	Config                   SessionConfig     `json:"config"`
	Messages                 []message.Message `json:"messages"`
	CWD                      string            `json:"cwd"`
	OriginalCWD              string            `json:"original_cwd"`
	ProjectRoot              string            `json:"project_root"`
	TurnCount                int               `json:"turn_count"`
	TotalInputTokens         int               `json:"total_input_tokens"`
	TotalOutputTokens        int               `json:"total_output_tokens"`
	TotalCacheCreationTokens int               `json:"total_cache_creation_tokens"`
	TotalCacheReadTokens     int               `json:"total_cache_read_tokens"`
	LastInputTokens          int               `json:"last_input_tokens"`
	CreatedAt                time.Time         `json:"created_at"`

	// Cost & duration tracking — Source: bootstrap/state.ts lines 51-64
	TotalCostUSD                  float64 `json:"total_cost_usd"`
	TotalAPIDuration              float64 `json:"total_api_duration_ms"`                // cumulative API call time in ms
	TotalAPIDurationWithoutRetries float64 `json:"total_api_duration_without_retries_ms"` // T108: API duration excluding retries
	TotalToolDuration             float64 `json:"total_tool_duration_ms"`                // cumulative tool execution time in ms
	TotalLinesAdded               int     `json:"total_lines_added"`
	TotalLinesRemoved             int     `json:"total_lines_removed"`

	// Per-turn duration tracking — Source: bootstrap/state.ts lines 55-57 (T109)
	TurnHookDurationMs       float64 `json:"turn_hook_duration_ms"`
	TurnToolDurationMs       float64 `json:"turn_tool_duration_ms"`
	TurnClassifierDurationMs float64 `json:"turn_classifier_duration_ms"`

	// Per-turn count tracking — Source: bootstrap/state.ts lines 58-60 (T110)
	TurnToolCount       int `json:"turn_tool_count"`
	TurnHookCount       int `json:"turn_hook_count"`
	TurnClassifierCount int `json:"turn_classifier_count"`

	// T111: Monotonic start time, set once at session creation.
	// Source: bootstrap/state.ts line 61 — startTime
	StartTime time.Time `json:"start_time"`

	// T112: Last interaction time tracking. The dirty flag defers Date.now()
	// calls until a render frame flush, avoiding per-keypress overhead.
	// Source: bootstrap/state.ts lines 62, 665-689
	LastInteractionTime time.Time `json:"last_interaction_time"`
	interactionDirty    bool      `json:"-"`

	// T113: Flag set when the model's pricing isn't in the cost table.
	// Source: bootstrap/state.ts line 65
	HasUnknownModelCost bool `json:"has_unknown_model_cost,omitempty"`

	// T114: Model override for the session main loop.
	// Source: bootstrap/state.ts lines 68-69
	MainLoopModelOverride string `json:"main_loop_model_override,omitempty"`
	InitialMainLoopModel  string `json:"initial_main_loop_model,omitempty"`

	// T115: Cache of resolved model display strings.
	// Source: bootstrap/state.ts line 70
	ModelStrings map[string]string `json:"model_strings,omitempty"`

	// T116: Flag indicating Kairos (Brief) mode is active.
	// Source: bootstrap/state.ts — kairosActive
	KairosActive bool `json:"kairos_active,omitempty"`

	// T117: When true, every tool_use must have a matching tool_result.
	// Source: bootstrap/state.ts — strictToolResultPairing
	StrictToolResultPairing bool `json:"strict_tool_result_pairing,omitempty"`

	// T118: SDK agent progress summaries feature flag.
	// Source: bootstrap/state.ts — sdkAgentProgressSummariesEnabled
	SDKAgentProgressSummariesEnabled bool `json:"sdk_agent_progress_summaries_enabled,omitempty"`

	// T119: User opted into message collection.
	// Source: bootstrap/state.ts — userMsgOptIn
	UserMsgOptIn bool `json:"user_msg_opt_in,omitempty"`

	// T120: Client type, set from --agent flag or defaults to "cli".
	// Source: bootstrap/state.ts — clientType
	ClientType string `json:"client_type,omitempty"`

	// T121: Where the session was initiated from (e.g. "cli", "sdk", "bridge").
	// Source: bootstrap/state.ts — sessionSource
	SessionSource string `json:"session_source,omitempty"`

	// T122: Preview rendering format for AskUserQuestion — "markdown" or "html".
	// Source: bootstrap/state.ts — questionPreviewFormat
	QuestionPreviewFormat string `json:"question_preview_format,omitempty"`

	// Per-model usage tracking — Source: bootstrap/state.ts line 67
	mu         sync.Mutex               `json:"-"`
	ModelUsage map[string]*ModelUsageEntry `json:"model_usage,omitempty"`

	// CoordinatorMode stores the session's coordinator mode for resume reconciliation.
	// Values: "coordinator", "normal", or "" (unset/legacy session).
	// Source: coordinatorMode.ts — sessionMode field
	CoordinatorMode string `json:"coordinator_mode,omitempty"`

	// IsInteractive distinguishes TUI/REPL mode from headless/pipe mode.
	// Source: bootstrap/state.ts line 71
	IsInteractive bool `json:"is_interactive"`

	// PermissionPolicy is the runtime permission policy (not serialized).
	PermissionPolicy interface{} `json:"-"`

	// StopHookRunner is an optional callback that runs after model turns.
	// It can prevent continuation or inject blocking errors.
	// Source: query/stopHooks.ts:60-63
	StopHookRunner interface{} `json:"-"`

	// PostSamplingHooks are fire-and-forget callbacks that run after model
	// sampling completes, before tool execution.
	// Source: utils/hooks/postSamplingHooks.ts:24-33
	PostSamplingHooks []interface{} `json:"-"`

	// T130: Last API request/response for debugging (/share, bug reports).
	// Source: bootstrap/state.ts — lastAPIRequest, lastAPIRequestMessages
	LastAPIRequest         interface{}             `json:"-"` // raw API request body (for debug/share)
	LastAPIRequestMessages []provider.RequestMessage `json:"-"` // messages from the last API request

	// T131: Recent classifier API calls stored for debugging.
	// Source: bootstrap/state.ts — lastClassifierRequests
	LastClassifierRequests []interface{} `json:"-"`

	// T132: Cached CLAUDE.md file content. Avoids re-reading the file on every prompt build.
	// Source: bootstrap/state.ts — cachedClaudeMdContent
	CachedClaudeMdContent string `json:"-"`

	// T133: Append-only in-memory error log for /doctor and bug reports.
	// Source: bootstrap/state.ts — inMemoryErrorLog
	InMemoryErrorLog []string `json:"-"`

	// T135: Runtime permission bypass toggle (session-scoped).
	// Source: bootstrap/state.ts — sessionBypassPermissionsMode
	SessionBypassPermissionsMode bool `json:"-"`

	// T136: Scheduled tasks enabled flag + session-only cron tasks.
	// Source: bootstrap/state.ts — scheduledTasksEnabled, sessionCronTasks
	ScheduledTasksEnabled bool              `json:"-"`
	SessionCronTasks      []SessionCronTask `json:"-"`

	// T137: Teams created during this session, cleaned up on shutdown.
	// Source: bootstrap/state.ts — sessionCreatedTeams
	SessionCreatedTeams map[string]struct{} `json:"-"`

	// T138: Session-only trust flag for home directory (not persisted to disk).
	// Source: bootstrap/state.ts — sessionTrustAccepted
	SessionTrustAccepted bool `json:"-"`

	// T139: Session-only flag to disable session persistence to disk.
	// Source: bootstrap/state.ts — sessionPersistenceDisabled
	SessionPersistenceDisabled bool `json:"-"`

	// T140: Plan mode transition tracking.
	// Source: bootstrap/state.ts — hasExitedPlanMode, needsPlanModeExitAttachment
	HasExitedPlanMode            bool `json:"-"`
	NeedsPlanModeExitAttachment  bool `json:"-"`

	// T141: Auto mode transition tracking.
	// Source: bootstrap/state.ts — needsAutoModeExitAttachment
	NeedsAutoModeExitAttachment bool `json:"-"`

	// T142: LSP recommendation shown once per session.
	// Source: bootstrap/state.ts — lspRecommendationShownThisSession
	LspRecommendationShownThisSession bool `json:"-"`

	// T143: SDK initialization state — JSON schema for structured output.
	// Source: bootstrap/state.ts — initJsonSchema
	InitJsonSchema interface{} `json:"-"`
	// T143: Whether the SDK has been initialized this session.
	SdkInitialized bool `json:"-"`

	// T145: Cache for plan slugs: sessionId -> wordSlug.
	// Source: bootstrap/state.ts — planSlugCache
	PlanSlugCache map[string]string `json:"-"`

	// T146: Info about a teleported (remote) session origin.
	// Source: bootstrap/state.ts — teleportedSessionInfo
	TeleportedSessionInfo interface{} `json:"-"`

	// T147: Tracks which skills have been invoked. Composite key "agentId:skillName".
	// Source: bootstrap/state.ts — invokedSkills
	InvokedSkills map[string]bool `json:"-"`

	// T148: Tracks slow operations for diagnostics.
	// Source: bootstrap/state.ts — slowOperations
	SlowOperations map[string]time.Duration `json:"-"`

	// T149: List of beta headers from SDK.
	// Source: bootstrap/state.ts — sdkBetas
	SdkBetas []string `json:"-"`

	// T150: The agent type for the main conversation thread.
	// Source: bootstrap/state.ts — mainThreadAgentType
	MainThreadAgentType string `json:"-"`

	// T151: Whether running as a remote/bridge session.
	// Source: bootstrap/state.ts — isRemoteMode
	IsRemoteMode bool `json:"-"`

	// T152: URL of the direct-connect server (set in remote/bridge mode).
	// Source: bootstrap/state.ts — directConnectServerUrl
	DirectConnectServerUrl string `json:"-"`

	// T154: The last date string emitted in the system prompt (for date-change detection).
	// Source: bootstrap/state.ts — lastEmittedDate
	LastEmittedDate string `json:"-"`

	// T155: Extra directories to scan for CLAUDE.md files (beyond project root).
	// Source: bootstrap/state.ts — additionalDirectoriesForClaudeMd
	AdditionalDirectoriesForClaudeMd []string `json:"-"`
}

// New creates a new SessionState with the given config and working directory.
// Source: bootstrap/state.ts — getInitialState()
func New(config SessionConfig, cwd string) *SessionState {
	now := time.Now()
	return &SessionState{
		ID:                  uuid.New().String(),
		Config:              config,
		Messages:            make([]message.Message, 0),
		CWD:                 cwd,
		OriginalCWD:         cwd,
		ProjectRoot:         cwd,
		CreatedAt:           now,
		StartTime:           now,
		LastInteractionTime: now,
		ModelUsage:          make(map[string]*ModelUsageEntry),
		ClientType:          "cli",
		SessionCronTasks:    make([]SessionCronTask, 0),
		SessionCreatedTeams: make(map[string]struct{}),
		PlanSlugCache:       make(map[string]string),
		InvokedSkills:       make(map[string]bool),
		SlowOperations:      make(map[string]time.Duration),
	}
}

// AddCost records a cost amount and per-model usage.
// Source: bootstrap/state.ts — addToTotalCostState()
func (s *SessionState) AddCost(model string, costUSD float64, usage provider.TokenUsage) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.TotalCostUSD += costUSD
	entry, ok := s.ModelUsage[model]
	if !ok {
		entry = &ModelUsageEntry{}
		s.ModelUsage[model] = entry
	}
	entry.InputTokens += usage.InputTokens
	entry.OutputTokens += usage.OutputTokens
	entry.CacheCreationInputTokens += usage.CacheCreationInputTokens
	entry.CacheReadInputTokens += usage.CacheReadInputTokens
	entry.CostUSD += costUSD
}

// AddAPIDurationWithoutRetries accumulates API call time excluding retries.
// Source: bootstrap/state.ts — addToTotalAPIDurationWithoutRetries()
func (s *SessionState) AddAPIDurationWithoutRetries(durationMs float64) {
	s.TotalAPIDurationWithoutRetries += durationMs
}

// ResetAPIDurationWithoutRetries zeros the running total.
// Source: bootstrap/state.ts — resetTotalAPIDurationWithoutRetries()
func (s *SessionState) ResetAPIDurationWithoutRetries() {
	s.TotalAPIDurationWithoutRetries = 0
}

// AddTurnToolDuration records tool execution time and increments the tool count.
// Source: bootstrap/state.ts — addToTurnToolDuration()
func (s *SessionState) AddTurnToolDuration(durationMs float64) {
	s.TurnToolDurationMs += durationMs
	s.TurnToolCount++
}

// ResetTurnToolMetrics zeros the per-turn tool duration and count.
// Source: bootstrap/state.ts — resetTurnToolDuration()
func (s *SessionState) ResetTurnToolMetrics() {
	s.TurnToolDurationMs = 0
	s.TurnToolCount = 0
}

// AddTurnHookDuration records hook execution time and increments the hook count.
// Source: bootstrap/state.ts — addToTurnHookDuration()
func (s *SessionState) AddTurnHookDuration(durationMs float64) {
	s.TurnHookDurationMs += durationMs
	s.TurnHookCount++
}

// ResetTurnHookMetrics zeros the per-turn hook duration and count.
// Source: bootstrap/state.ts — resetTurnHookDuration()
func (s *SessionState) ResetTurnHookMetrics() {
	s.TurnHookDurationMs = 0
	s.TurnHookCount = 0
}

// AddTurnClassifierDuration records classifier execution time and increments the count.
// Source: bootstrap/state.ts — addToTurnClassifierDuration()
func (s *SessionState) AddTurnClassifierDuration(durationMs float64) {
	s.TurnClassifierDurationMs += durationMs
	s.TurnClassifierCount++
}

// ResetTurnClassifierMetrics zeros the per-turn classifier duration and count.
// Source: bootstrap/state.ts — resetTurnClassifierDuration()
func (s *SessionState) ResetTurnClassifierMetrics() {
	s.TurnClassifierDurationMs = 0
	s.TurnClassifierCount = 0
}

// AddLinesChanged records lines added/removed by code edits.
// Source: bootstrap/state.ts — addToTotalLinesChanged()
func (s *SessionState) AddLinesChanged(added, removed int) {
	s.TotalLinesAdded += added
	s.TotalLinesRemoved += removed
}

// UpdateLastInteractionTime marks an interaction. When immediate is true the
// timestamp is updated right away; otherwise it is deferred until the next
// FlushInteractionTime call (batching many keypresses into one clock read).
// Source: bootstrap/state.ts — updateLastInteractionTime()
func (s *SessionState) UpdateLastInteractionTime(immediate bool) {
	if immediate {
		s.LastInteractionTime = time.Now()
		s.interactionDirty = false
	} else {
		s.interactionDirty = true
	}
}

// FlushInteractionTime updates the timestamp if an interaction was recorded
// since the last flush. Called by the render loop before each frame.
// Source: bootstrap/state.ts — flushInteractionTime()
func (s *SessionState) FlushInteractionTime() {
	if s.interactionDirty {
		s.LastInteractionTime = time.Now()
		s.interactionDirty = false
	}
}

// SetModelStrings replaces the cached model display strings.
// Source: bootstrap/state.ts — setModelStrings()
func (s *SessionState) SetModelStrings(m map[string]string) {
	s.ModelStrings = m
}

// ClearModelStrings resets the model strings cache to nil.
// Source: bootstrap/state.ts — clearModelStrings()
func (s *SessionState) ClearModelStrings() {
	s.ModelStrings = nil
}

// SetClientType updates the client type (e.g. from --agent flag).
// Source: bootstrap/state.ts — clientType setter
func (s *SessionState) SetClientType(ct string) {
	s.ClientType = ct
}

// SetSessionSource sets the session source (e.g. "cli", "sdk", "bridge").
// Source: bootstrap/state.ts — setSessionSource()
func (s *SessionState) SetSessionSource(source string) {
	s.SessionSource = source
}

// SetQuestionPreviewFormat sets the AskUserQuestion preview format.
// Only "markdown" and "html" are valid values.
// Source: bootstrap/state.ts — setQuestionPreviewFormat()
func (s *SessionState) SetQuestionPreviewFormat(format string) {
	s.QuestionPreviewFormat = format
}

// SetLastAPIRequest stores the raw API request and messages for debugging.
// Source: bootstrap/state.ts — lastAPIRequest, lastAPIRequestMessages
func (s *SessionState) SetLastAPIRequest(request interface{}, messages []provider.RequestMessage) {
	s.LastAPIRequest = request
	s.LastAPIRequestMessages = messages
}

// ClearLastAPIRequest resets the debug request fields.
func (s *SessionState) ClearLastAPIRequest() {
	s.LastAPIRequest = nil
	s.LastAPIRequestMessages = nil
}

// AddClassifierRequest appends a classifier API call to the debug log.
// Source: bootstrap/state.ts — lastClassifierRequests
func (s *SessionState) AddClassifierRequest(req interface{}) {
	s.LastClassifierRequests = append(s.LastClassifierRequests, req)
}

// ClearClassifierRequests resets the classifier request log.
func (s *SessionState) ClearClassifierRequests() {
	s.LastClassifierRequests = nil
}

// SetCachedClaudeMdContent stores the cached CLAUDE.md content.
// Source: bootstrap/state.ts — cachedClaudeMdContent
func (s *SessionState) SetCachedClaudeMdContent(content string) {
	s.CachedClaudeMdContent = content
}

// GetCachedClaudeMdContent returns the cached CLAUDE.md content.
func (s *SessionState) GetCachedClaudeMdContent() string {
	return s.CachedClaudeMdContent
}

// AppendError adds an error message to the in-memory error log.
// Source: bootstrap/state.ts — inMemoryErrorLog
func (s *SessionState) AppendError(errMsg string) {
	s.InMemoryErrorLog = append(s.InMemoryErrorLog, errMsg)
}

// GetErrorLog returns a copy of the in-memory error log.
func (s *SessionState) GetErrorLog() []string {
	if s.InMemoryErrorLog == nil {
		return nil
	}
	out := make([]string, len(s.InMemoryErrorLog))
	copy(out, s.InMemoryErrorLog)
	return out
}

// SetBypassPermissionsMode enables or disables the session-scoped permission bypass.
// Source: bootstrap/state.ts — sessionBypassPermissionsMode
func (s *SessionState) SetBypassPermissionsMode(bypass bool) {
	s.SessionBypassPermissionsMode = bypass
}

// ---------------------------------------------------------------------------
// T136: Cron task state — Source: bootstrap/state.ts
// ---------------------------------------------------------------------------

// SessionCronTask represents a session-only cron task created via CronCreate
// with durable: false. These fire on schedule but are never written to disk.
// Source: bootstrap/state.ts — SessionCronTask type
type SessionCronTask struct {
	ID        string `json:"id"`
	Cron      string `json:"cron"`
	Prompt    string `json:"prompt"`
	CreatedAt int64  `json:"created_at"` // Unix ms
	Recurring bool   `json:"recurring,omitempty"`
	AgentID   string `json:"agent_id,omitempty"` // routes to a subagent instead of main REPL
}

// SetScheduledTasksEnabled enables or disables the scheduled tasks watcher.
// Source: bootstrap/state.ts — setScheduledTasksEnabled
func (s *SessionState) SetScheduledTasksEnabled(enabled bool) {
	s.ScheduledTasksEnabled = enabled
}

// GetScheduledTasksEnabled returns whether scheduled tasks are enabled.
// Source: bootstrap/state.ts — getScheduledTasksEnabled
func (s *SessionState) GetScheduledTasksEnabled() bool {
	return s.ScheduledTasksEnabled
}

// GetSessionCronTasks returns the session-only cron tasks.
// Source: bootstrap/state.ts — getSessionCronTasks
func (s *SessionState) GetSessionCronTasks() []SessionCronTask {
	return s.SessionCronTasks
}

// AddSessionCronTask appends a cron task to the session-only list.
// Source: bootstrap/state.ts — addSessionCronTask
func (s *SessionState) AddSessionCronTask(task SessionCronTask) {
	s.SessionCronTasks = append(s.SessionCronTasks, task)
}

// RemoveSessionCronTasks removes tasks by ID and returns the number removed.
// Source: bootstrap/state.ts — removeSessionCronTasks
func (s *SessionState) RemoveSessionCronTasks(ids []string) int {
	if len(ids) == 0 {
		return 0
	}
	idSet := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		idSet[id] = struct{}{}
	}
	remaining := make([]SessionCronTask, 0, len(s.SessionCronTasks))
	for _, t := range s.SessionCronTasks {
		if _, ok := idSet[t.ID]; !ok {
			remaining = append(remaining, t)
		}
	}
	removed := len(s.SessionCronTasks) - len(remaining)
	if removed > 0 {
		s.SessionCronTasks = remaining
	}
	return removed
}

// ---------------------------------------------------------------------------
// T137: Team tracking — Source: bootstrap/state.ts — sessionCreatedTeams
// ---------------------------------------------------------------------------

// GetSessionCreatedTeams returns the set of teams created during this session.
// Source: bootstrap/state.ts — getSessionCreatedTeams
func (s *SessionState) GetSessionCreatedTeams() map[string]struct{} {
	return s.SessionCreatedTeams
}

// AddSessionCreatedTeam records a team created during this session.
func (s *SessionState) AddSessionCreatedTeam(teamName string) {
	if s.SessionCreatedTeams == nil {
		s.SessionCreatedTeams = make(map[string]struct{})
	}
	s.SessionCreatedTeams[teamName] = struct{}{}
}

// RemoveSessionCreatedTeam removes a team from the session tracking set
// (e.g. when TeamDelete is called, to avoid double-cleanup on shutdown).
func (s *SessionState) RemoveSessionCreatedTeam(teamName string) {
	delete(s.SessionCreatedTeams, teamName)
}

// ---------------------------------------------------------------------------
// T138: Session trust — Source: bootstrap/state.ts
// ---------------------------------------------------------------------------

// SetSessionTrustAccepted sets the session-scoped trust flag.
// Source: bootstrap/state.ts — setSessionTrustAccepted
func (s *SessionState) SetSessionTrustAccepted(accepted bool) {
	s.SessionTrustAccepted = accepted
}

// GetSessionTrustAccepted returns whether trust has been accepted this session.
// Source: bootstrap/state.ts — getSessionTrustAccepted
func (s *SessionState) GetSessionTrustAccepted() bool {
	return s.SessionTrustAccepted
}

// ---------------------------------------------------------------------------
// T139: Session persistence — Source: bootstrap/state.ts
// ---------------------------------------------------------------------------

// SetSessionPersistenceDisabled disables session persistence to disk.
// Source: bootstrap/state.ts — setSessionPersistenceDisabled
func (s *SessionState) SetSessionPersistenceDisabled(disabled bool) {
	s.SessionPersistenceDisabled = disabled
}

// IsSessionPersistenceDisabled returns whether persistence is disabled.
// Source: bootstrap/state.ts — isSessionPersistenceDisabled
func (s *SessionState) IsSessionPersistenceDisabled() bool {
	return s.SessionPersistenceDisabled
}

// ---------------------------------------------------------------------------
// T140: Plan mode transitions — Source: bootstrap/state.ts
// ---------------------------------------------------------------------------

// HasExitedPlanModeInSession returns whether the user has exited plan mode.
// Source: bootstrap/state.ts — hasExitedPlanModeInSession
func (s *SessionState) HasExitedPlanModeInSession() bool {
	return s.HasExitedPlanMode
}

// SetHasExitedPlanMode sets the plan mode exit tracking flag.
// Source: bootstrap/state.ts — setHasExitedPlanMode
func (s *SessionState) SetHasExitedPlanMode(value bool) {
	s.HasExitedPlanMode = value
}

// GetNeedsPlanModeExitAttachment returns whether the exit attachment is pending.
// Source: bootstrap/state.ts — needsPlanModeExitAttachment
func (s *SessionState) GetNeedsPlanModeExitAttachment() bool {
	return s.NeedsPlanModeExitAttachment
}

// SetNeedsPlanModeExitAttachment sets the exit attachment flag.
// Source: bootstrap/state.ts — setNeedsPlanModeExitAttachment
func (s *SessionState) SetNeedsPlanModeExitAttachment(value bool) {
	s.NeedsPlanModeExitAttachment = value
}

// HandlePlanModeTransition processes a mode switch and updates plan mode flags.
// When switching TO plan mode, clears any pending exit attachment.
// When switching FROM plan mode, triggers the exit attachment.
// Source: bootstrap/state.ts — handlePlanModeTransition
func (s *SessionState) HandlePlanModeTransition(fromMode, toMode string) {
	// Entering plan mode: clear pending exit attachment to avoid sending both
	// plan_mode and plan_mode_exit when the user toggles quickly.
	if toMode == "plan" && fromMode != "plan" {
		s.NeedsPlanModeExitAttachment = false
	}
	// Leaving plan mode: trigger the one-time exit attachment.
	if fromMode == "plan" && toMode != "plan" {
		s.NeedsPlanModeExitAttachment = true
	}
}

// ---------------------------------------------------------------------------
// T141: Auto mode transitions — Source: bootstrap/state.ts
// ---------------------------------------------------------------------------

// GetNeedsAutoModeExitAttachment returns whether the auto mode exit attachment is pending.
// Source: bootstrap/state.ts — needsAutoModeExitAttachment
func (s *SessionState) GetNeedsAutoModeExitAttachment() bool {
	return s.NeedsAutoModeExitAttachment
}

// SetNeedsAutoModeExitAttachment sets the auto mode exit attachment flag.
// Source: bootstrap/state.ts — setNeedsAutoModeExitAttachment
func (s *SessionState) SetNeedsAutoModeExitAttachment(value bool) {
	s.NeedsAutoModeExitAttachment = value
}

// HandleAutoModeTransition processes a mode switch and updates auto mode flags.
// Auto<->plan transitions are skipped (handled by plan mode logic). Only direct
// auto transitions trigger the exit attachment.
// Source: bootstrap/state.ts — handleAutoModeTransition
func (s *SessionState) HandleAutoModeTransition(fromMode, toMode string) {
	// Skip auto<->plan transitions — these are handled by plan mode logic.
	if (fromMode == "auto" && toMode == "plan") ||
		(fromMode == "plan" && toMode == "auto") {
		return
	}

	fromIsAuto := fromMode == "auto"
	toIsAuto := toMode == "auto"

	// Entering auto mode: clear pending exit attachment to avoid sending both
	// auto_mode and auto_mode_exit when the user toggles quickly.
	if toIsAuto && !fromIsAuto {
		s.NeedsAutoModeExitAttachment = false
	}

	// Leaving auto mode: trigger the one-time exit attachment.
	if fromIsAuto && !toIsAuto {
		s.NeedsAutoModeExitAttachment = true
	}
}

// ---------------------------------------------------------------------------
// T142: LSP recommendation — Source: bootstrap/state.ts
// ---------------------------------------------------------------------------

// HasShownLspRecommendationThisSession returns whether the LSP recommendation
// has been shown this session.
// Source: bootstrap/state.ts — hasShownLspRecommendationThisSession
func (s *SessionState) HasShownLspRecommendationThisSession() bool {
	return s.LspRecommendationShownThisSession
}

// SetLspRecommendationShownThisSession marks the LSP recommendation as shown.
// Source: bootstrap/state.ts — setLspRecommendationShownThisSession
func (s *SessionState) SetLspRecommendationShownThisSession(value bool) {
	s.LspRecommendationShownThisSession = value
}

// ---------------------------------------------------------------------------
// T143: SDK init state — Source: bootstrap/state.ts
// ---------------------------------------------------------------------------

// SetInitJsonSchema stores the JSON schema for SDK structured output.
// Source: bootstrap/state.ts — setInitJsonSchema
func (s *SessionState) SetInitJsonSchema(schema interface{}) {
	s.InitJsonSchema = schema
	s.SdkInitialized = true
}

// GetInitJsonSchema returns the SDK init JSON schema (may be nil).
// Source: bootstrap/state.ts — getInitJsonSchema
func (s *SessionState) GetInitJsonSchema() interface{} {
	return s.InitJsonSchema
}

// ResetSdkInitState clears the SDK initialization state.
// Source: bootstrap/state.ts — resetSdkInitState
func (s *SessionState) ResetSdkInitState() {
	s.InitJsonSchema = nil
	s.SdkInitialized = false
}

// ---------------------------------------------------------------------------
// T145: Plan slug cache — Source: bootstrap/state.ts
// ---------------------------------------------------------------------------

// GetPlanSlug returns the cached plan slug for a session ID.
// Source: bootstrap/state.ts — getPlanSlugCache
func (s *SessionState) GetPlanSlug(sessionID string) (string, bool) {
	slug, ok := s.PlanSlugCache[sessionID]
	return slug, ok
}

// SetPlanSlug caches a plan slug for a session ID.
func (s *SessionState) SetPlanSlug(sessionID, slug string) {
	if s.PlanSlugCache == nil {
		s.PlanSlugCache = make(map[string]string)
	}
	s.PlanSlugCache[sessionID] = slug
}

// DeletePlanSlug removes a plan slug entry.
func (s *SessionState) DeletePlanSlug(sessionID string) {
	delete(s.PlanSlugCache, sessionID)
}

// ---------------------------------------------------------------------------
// T147: Invoked skills — Source: bootstrap/state.ts — invokedSkills
// ---------------------------------------------------------------------------

// MarkSkillInvoked records that a skill has been invoked for a given agent.
// The composite key is "agentId:skillName".
func (s *SessionState) MarkSkillInvoked(agentID, skillName string) {
	if s.InvokedSkills == nil {
		s.InvokedSkills = make(map[string]bool)
	}
	s.InvokedSkills[agentID+":"+skillName] = true
}

// HasInvokedSkill returns whether a skill has been invoked for a given agent.
func (s *SessionState) HasInvokedSkill(agentID, skillName string) bool {
	return s.InvokedSkills[agentID+":"+skillName]
}

// ClearInvokedSkills removes all invoked skill entries except those belonging
// to the preserved agent IDs.
func (s *SessionState) ClearInvokedSkills(preservedAgentIDs []string) {
	if len(preservedAgentIDs) == 0 {
		s.InvokedSkills = make(map[string]bool)
		return
	}
	preserved := make(map[string]struct{}, len(preservedAgentIDs))
	for _, id := range preservedAgentIDs {
		preserved[id+":"] = struct{}{}
	}
	for key := range s.InvokedSkills {
		keep := false
		for prefix := range preserved {
			if len(key) >= len(prefix) && key[:len(prefix)] == prefix {
				keep = true
				break
			}
		}
		if !keep {
			delete(s.InvokedSkills, key)
		}
	}
}

// ---------------------------------------------------------------------------
// T148: Slow operations — Source: bootstrap/state.ts — slowOperations
// ---------------------------------------------------------------------------

// RecordSlowOperation records a slow operation with the given name and duration.
func (s *SessionState) RecordSlowOperation(name string, duration time.Duration) {
	if s.SlowOperations == nil {
		s.SlowOperations = make(map[string]time.Duration)
	}
	s.SlowOperations[name] = duration
}

// GetSlowOperations returns a copy of the slow operations map.
func (s *SessionState) GetSlowOperations() map[string]time.Duration {
	if s.SlowOperations == nil {
		return nil
	}
	out := make(map[string]time.Duration, len(s.SlowOperations))
	for k, v := range s.SlowOperations {
		out[k] = v
	}
	return out
}

// RegenerateSessionID creates a new session ID, optionally setting the current as parent.
// Source: bootstrap/state.ts — regenerateSessionId()
func (s *SessionState) RegenerateSessionID(setCurrentAsParent bool) string {
	if setCurrentAsParent {
		s.ParentSessionID = s.ID
	}
	s.ID = uuid.New().String()
	return s.ID
}

// FirstUserPreview extracts the first user message text, truncated to
// PreviewMaxLen characters. Used for the resume session picker.
// Source: ResumeConversation.tsx — first user message preview
func (s *SessionState) FirstUserPreview() string {
	for _, m := range s.Messages {
		if m.Role != message.RoleUser {
			continue
		}
		for _, b := range m.Content {
			if b.Type == message.ContentText && b.Text != "" {
				text := b.Text
				if len(text) > PreviewMaxLen {
					text = text[:PreviewMaxLen-3] + "..."
				}
				return text
			}
		}
	}
	return ""
}

// PushMessage appends a message to the session history.
func (s *SessionState) PushMessage(msg message.Message) {
	s.Messages = append(s.Messages, msg)
}

// ToRequestMessages converts session messages to the API request format.
func (s *SessionState) ToRequestMessages() []provider.RequestMessage {
	msgs := make([]provider.RequestMessage, 0, len(s.Messages))
	for _, m := range s.Messages {
		rm := provider.RequestMessage{
			Role:    string(m.Role),
			Content: make([]provider.RequestContent, 0, len(m.Content)),
		}
		for _, b := range m.Content {
			switch b.Type {
			case message.ContentText:
				rm.Content = append(rm.Content, provider.RequestContent{
					Type: "text",
					Text: b.Text,
				})
			case message.ContentToolUse:
				rm.Content = append(rm.Content, provider.RequestContent{
					Type:  "tool_use",
					ID:    b.ID,
					Name:  b.Name,
					Input: b.Input,
				})
			case message.ContentToolResult:
				var errPtr *bool
				if b.IsError {
					t := true
					errPtr = &t
				}
				rm.Content = append(rm.Content, provider.RequestContent{
					Type:      "tool_result",
					ToolUseID: b.ToolUseID,
					Content:   b.Content,
					IsError:   errPtr,
				})
			}
		}
		msgs = append(msgs, rm)
	}
	return msgs
}

// ---------------------------------------------------------------------------
// T151: Remote mode — Source: bootstrap/state.ts — isRemoteMode
// ---------------------------------------------------------------------------

// SetIsRemoteMode sets whether the session is running in remote/bridge mode.
func (s *SessionState) SetIsRemoteMode(remote bool) {
	s.IsRemoteMode = remote
}

// GetIsRemoteMode returns whether the session is in remote mode.
func (s *SessionState) GetIsRemoteMode() bool {
	return s.IsRemoteMode
}

// ---------------------------------------------------------------------------
// T152: Direct connect server URL — Source: bootstrap/state.ts
// ---------------------------------------------------------------------------

// SetDirectConnectServerUrl sets the direct-connect server URL.
func (s *SessionState) SetDirectConnectServerUrl(url string) {
	s.DirectConnectServerUrl = url
}

// GetDirectConnectServerUrl returns the direct-connect server URL.
func (s *SessionState) GetDirectConnectServerUrl() string {
	return s.DirectConnectServerUrl
}

// ---------------------------------------------------------------------------
// T154: Last emitted date — Source: bootstrap/state.ts — lastEmittedDate
// ---------------------------------------------------------------------------

// SetLastEmittedDate stores the last date string emitted in the system prompt.
func (s *SessionState) SetLastEmittedDate(date string) {
	s.LastEmittedDate = date
}

// GetLastEmittedDate returns the last date string emitted in the system prompt.
func (s *SessionState) GetLastEmittedDate() string {
	return s.LastEmittedDate
}

// ---------------------------------------------------------------------------
// T155: Additional directories for CLAUDE.md — Source: bootstrap/state.ts
// ---------------------------------------------------------------------------

// SetAdditionalDirectoriesForClaudeMd sets extra directories to scan for CLAUDE.md.
func (s *SessionState) SetAdditionalDirectoriesForClaudeMd(dirs []string) {
	s.AdditionalDirectoriesForClaudeMd = dirs
}

// GetAdditionalDirectoriesForClaudeMd returns the extra CLAUDE.md directories.
func (s *SessionState) GetAdditionalDirectoriesForClaudeMd() []string {
	return s.AdditionalDirectoriesForClaudeMd
}
