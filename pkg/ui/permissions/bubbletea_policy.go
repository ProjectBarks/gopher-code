package permissions

import (
	"context"
	"time"

	"github.com/projectbarks/gopher-code/pkg/permissions"
	"github.com/projectbarks/gopher-code/pkg/ui/components"
)

// BubbleTeaPolicy implements the PermissionPolicy interface for the Bubbletea UI.
// It bridges the synchronous permission check (called from the query loop goroutine)
// with the asynchronous Bubbletea UI (which shows a modal dialog).
type BubbleTeaPolicy struct {
	// requestCh sends permission requests to the UI
	requestCh chan PermissionRequest
	// timeout is the maximum time to wait for user response
	timeout time.Duration
}

// PermissionRequest carries a permission check into the UI.
type PermissionRequest struct {
	ToolName   string
	ToolID     string
	Message    string
	ResponseCh chan<- permissions.PermissionDecision
}

// NewBubbleTeaPolicy creates a new Bubbletea permission policy.
func NewBubbleTeaPolicy(timeout time.Duration) *BubbleTeaPolicy {
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	return &BubbleTeaPolicy{
		requestCh: make(chan PermissionRequest, 1),
		timeout:   timeout,
	}
}

// Check implements the permission check. Called from the query goroutine.
// Blocks until the user approves/denies or timeout expires.
func (btp *BubbleTeaPolicy) Check(ctx context.Context, toolName string, toolID string) permissions.PermissionDecision {
	responseCh := make(chan permissions.PermissionDecision, 1)

	req := PermissionRequest{
		ToolName:   toolName,
		ToolID:     toolID,
		ResponseCh: responseCh,
	}

	// Send request to UI
	select {
	case btp.requestCh <- req:
	case <-ctx.Done():
		return permissions.DenyDecision{Reason: "context cancelled"}
	}

	// Wait for response with timeout
	select {
	case decision := <-responseCh:
		return decision
	case <-time.After(btp.timeout):
		return permissions.DenyDecision{Reason: "permission timeout"}
	case <-ctx.Done():
		return permissions.DenyDecision{Reason: "context cancelled"}
	}
}

// Requests returns the channel to receive permission requests from.
// The Bubbletea Update loop reads from this channel.
func (btp *BubbleTeaPolicy) Requests() <-chan PermissionRequest {
	return btp.requestCh
}

// HandleApproval converts an ApprovalResult to a PermissionDecision
// and sends it back through the request's response channel.
func HandleApproval(req PermissionRequest, result components.ApprovalResult) {
	var decision permissions.PermissionDecision
	switch result {
	case components.ApprovalApproved:
		decision = permissions.AllowDecision{}
	case components.ApprovalAlways:
		decision = permissions.AllowDecision{}
	default:
		decision = permissions.DenyDecision{Reason: "user denied"}
	}

	select {
	case req.ResponseCh <- decision:
	default:
	}
}
