package permissions

import (
	"sync"
	"time"
)

// Source: src/hooks/toolPermission/PermissionContext.ts

// ApprovalSourceType identifies how a permission was approved.
type ApprovalSourceType string

const (
	ApprovalConfig     ApprovalSourceType = "config"
	ApprovalHook       ApprovalSourceType = "hook"
	ApprovalUser       ApprovalSourceType = "user"
	ApprovalClassifier ApprovalSourceType = "classifier"
)

// ApprovalSource describes who/what approved a permission request.
// Source: PermissionContext.ts — PermissionApprovalSource
type ApprovalSource struct {
	Type      ApprovalSourceType
	Permanent bool // for hook/user: whether the rule was persisted
}

// RejectionSourceType identifies how a permission was rejected.
type RejectionSourceType string

const (
	RejectionHook       RejectionSourceType = "hook"
	RejectionUserAbort  RejectionSourceType = "user_abort"
	RejectionUserReject RejectionSourceType = "user_reject"
	RejectionConfig     RejectionSourceType = "config"
)

// RejectionSource describes who/what rejected a permission request.
// Source: PermissionContext.ts — PermissionRejectionSource
type RejectionSource struct {
	Type        RejectionSourceType
	HasFeedback bool // for user_reject: whether the user provided feedback
}

// DecisionRecord is the outcome of a permission check, stored for audit.
type DecisionRecord struct {
	ToolName  string
	ToolUseID string
	Decision  string // "accept" or "reject"
	Source    string // flattened source label
	Timestamp time.Time
}

// PermissionContext tracks permission state for a session. It is the
// Go equivalent of the TS createPermissionContext factory — a single
// struct used by all handlers (coordinator, interactive, swarm-worker).
//
// Source: src/hooks/toolPermission/PermissionContext.ts
type PermissionContext struct {
	mu sync.RWMutex

	// SessionAllowed tracks tools that have been session-allowed (tool → true).
	SessionAllowed map[string]bool

	// SessionDenied tracks tools that have been session-denied (tool → reason).
	SessionDenied map[string]string

	// Pending tracks tool-use IDs with pending permission requests.
	Pending map[string]bool

	// Decisions is an append-only audit trail of all permission decisions.
	Decisions []DecisionRecord

	// Logger receives every decision for external logging/telemetry.
	Logger PermissionLogger
}

// NewPermissionContext creates a fresh per-session permission context.
func NewPermissionContext(logger PermissionLogger) *PermissionContext {
	if logger == nil {
		logger = &noopLogger{}
	}
	return &PermissionContext{
		SessionAllowed: make(map[string]bool),
		SessionDenied:  make(map[string]string),
		Pending:        make(map[string]bool),
		Logger:         logger,
	}
}

// MarkAllowed records that a tool has been allowed for this session.
func (pc *PermissionContext) MarkAllowed(toolName string) {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	pc.SessionAllowed[toolName] = true
	delete(pc.SessionDenied, toolName)
}

// MarkDenied records that a tool has been denied for this session.
func (pc *PermissionContext) MarkDenied(toolName, reason string) {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	pc.SessionDenied[toolName] = reason
	delete(pc.SessionAllowed, toolName)
}

// IsSessionAllowed checks if a tool was previously allowed this session.
func (pc *PermissionContext) IsSessionAllowed(toolName string) bool {
	pc.mu.RLock()
	defer pc.mu.RUnlock()
	return pc.SessionAllowed[toolName]
}

// IsSessionDenied checks if a tool was previously denied this session.
func (pc *PermissionContext) IsSessionDenied(toolName string) (string, bool) {
	pc.mu.RLock()
	defer pc.mu.RUnlock()
	reason, ok := pc.SessionDenied[toolName]
	return reason, ok
}

// MarkPending adds a tool-use ID as pending permission.
func (pc *PermissionContext) MarkPending(toolUseID string) {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	pc.Pending[toolUseID] = true
}

// ClearPending removes a tool-use ID from pending.
func (pc *PermissionContext) ClearPending(toolUseID string) {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	delete(pc.Pending, toolUseID)
}

// IsPending checks if a tool-use ID has a pending permission request.
func (pc *PermissionContext) IsPending(toolUseID string) bool {
	pc.mu.RLock()
	defer pc.mu.RUnlock()
	return pc.Pending[toolUseID]
}

// RecordDecision appends a decision to the audit trail and notifies the logger.
func (pc *PermissionContext) RecordDecision(toolName, toolUseID, decision, source string) {
	record := DecisionRecord{
		ToolName:  toolName,
		ToolUseID: toolUseID,
		Decision:  decision,
		Source:    source,
		Timestamp: time.Now(),
	}

	pc.mu.Lock()
	pc.Decisions = append(pc.Decisions, record)
	pc.mu.Unlock()

	pc.Logger.LogDecision(record)
}

// GetDecisions returns a copy of all recorded decisions.
func (pc *PermissionContext) GetDecisions() []DecisionRecord {
	pc.mu.RLock()
	defer pc.mu.RUnlock()
	out := make([]DecisionRecord, len(pc.Decisions))
	copy(out, pc.Decisions)
	return out
}

// PendingCount returns the number of pending permission requests.
func (pc *PermissionContext) PendingCount() int {
	pc.mu.RLock()
	defer pc.mu.RUnlock()
	return len(pc.Pending)
}
