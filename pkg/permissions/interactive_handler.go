package permissions

import (
	"time"

	tea "charm.land/bubbletea/v2"
)

// Source: src/hooks/toolPermission/handlers/interactiveHandler.ts

// InteractiveGracePeriod is how long to ignore user interactions after
// the permission prompt appears, to prevent stale keypresses from
// canceling the classifier prematurely.
// Source: interactiveHandler.ts — GRACE_PERIOD_MS
const InteractiveGracePeriod = 200 * time.Millisecond

// PermissionRequestMsg is a bubbletea message that asks the TUI to
// display a permission prompt for a tool-use call.
type PermissionRequestMsg struct {
	ToolName    string
	ToolUseID   string
	Description string
	Input       map[string]any
	StartTime   time.Time
	Resolve     *ResolveOnce[PermissionDecision]
	Context     *PermissionContext
}

// PermissionResponseMsg is sent back after the user responds.
type PermissionResponseMsg struct {
	ToolUseID string
	Decision  PermissionDecision
	Feedback  string
	Updates   []PermissionUpdate
	Permanent bool
}

// PermissionAutoApprovedMsg is sent when hooks/classifier auto-approve.
type PermissionAutoApprovedMsg struct {
	ToolUseID string
	Source    string // "hook" or "classifier"
}

// InteractiveHandler implements the interactive (main-agent) permission
// flow in a bubbletea-compatible way. Instead of pushing to a React
// queue, it returns tea.Cmd values that emit PermissionRequestMsg.
//
// The interactive handler sets up a ResolveOnce guard and returns a
// tea.Cmd. The TUI model receives the PermissionRequestMsg and shows
// the prompt. User interaction (allow/deny/abort) resolves the guard.
//
// Source: src/hooks/toolPermission/handlers/interactiveHandler.ts
type InteractiveHandler struct {
	// PermCtx is the per-session permission context for state tracking.
	PermCtx *PermissionContext

	// Hooks is the optional coordinator handler for pre-dialog checks.
	Coordinator *CoordinatorHandler
}

// RequestPermission returns a tea.Cmd that will produce a PermissionRequestMsg
// for the TUI to display. The returned channel will receive the final decision.
//
// Source: interactiveHandler.ts — handleInteractivePermission
func (h *InteractiveHandler) RequestPermission(
	toolName, toolUseID, description string,
	input map[string]any,
	permissionMode string,
) (tea.Cmd, <-chan PermissionDecision) {
	resultCh := make(chan PermissionDecision, 1)

	resolve := NewResolveOnce(func(d PermissionDecision) {
		h.PermCtx.ClearPending(toolUseID)
		resultCh <- d
	})

	h.PermCtx.MarkPending(toolUseID)

	cmd := func() tea.Msg {
		return PermissionRequestMsg{
			ToolName:    toolName,
			ToolUseID:   toolUseID,
			Description: description,
			Input:       input,
			StartTime:   time.Now(),
			Resolve:     resolve,
			Context:     h.PermCtx,
		}
	}

	return cmd, resultCh
}

// HandleAllow processes a user "allow" response.
func (h *InteractiveHandler) HandleAllow(
	req PermissionRequestMsg,
	updatedInput map[string]any,
	updates []PermissionUpdate,
	feedback string,
	permanent bool,
) {
	if !req.Resolve.Claim() {
		return
	}

	source := ApprovalSourceToString(ApprovalSource{Type: ApprovalUser, Permanent: permanent})
	req.Context.RecordDecision(req.ToolName, req.ToolUseID, "accept", source)
	req.Context.MarkAllowed(req.ToolName)

	req.Resolve.Resolve(AllowDecision{})
}

// HandleDeny processes a user "deny" response.
func (h *InteractiveHandler) HandleDeny(
	req PermissionRequestMsg,
	feedback string,
) {
	if !req.Resolve.Claim() {
		return
	}

	hasFeedback := feedback != ""
	source := RejectionSourceToString(RejectionSource{Type: RejectionUserReject, HasFeedback: hasFeedback})
	req.Context.RecordDecision(req.ToolName, req.ToolUseID, "reject", source)
	req.Context.MarkDenied(req.ToolName, "user denied permission")

	reason := "user denied permission"
	if feedback != "" {
		reason = feedback
	}
	req.Resolve.Resolve(DenyDecision{Reason: reason})
}

// HandleAbort processes a user "abort" response (Ctrl-C, etc).
func (h *InteractiveHandler) HandleAbort(req PermissionRequestMsg) {
	if !req.Resolve.Claim() {
		return
	}

	source := RejectionSourceToString(RejectionSource{Type: RejectionUserAbort})
	req.Context.RecordDecision(req.ToolName, req.ToolUseID, "reject", source)
	req.Context.Logger.LogCancelled(req.ToolName, req.ToolUseID, "")

	req.Resolve.Resolve(DenyDecision{Reason: "user aborted"})
}

// ShouldIgnoreInteraction returns true if the interaction happened within
// the grace period (200ms) after the prompt appeared.
// Source: interactiveHandler.ts — GRACE_PERIOD_MS
func ShouldIgnoreInteraction(promptStart time.Time) bool {
	return time.Since(promptStart) < InteractiveGracePeriod
}
