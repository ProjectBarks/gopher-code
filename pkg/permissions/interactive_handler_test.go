package permissions

import (
	"testing"
	"time"
)

func TestInteractiveHandler_RequestPermission(t *testing.T) {
	logger := &CollectingLogger{}
	pc := NewPermissionContext(logger)
	h := &InteractiveHandler{PermCtx: pc}

	cmd, resultCh := h.RequestPermission("Bash", "tu-1", "Run ls", nil, "default")

	// The tool-use should be pending.
	if !pc.IsPending("tu-1") {
		t.Fatal("tu-1 should be pending after RequestPermission")
	}

	// Execute the cmd to get the message.
	msg := cmd()
	req, ok := msg.(PermissionRequestMsg)
	if !ok {
		t.Fatalf("expected PermissionRequestMsg, got %T", msg)
	}
	if req.ToolName != "Bash" {
		t.Errorf("expected tool 'Bash', got %q", req.ToolName)
	}
	if req.ToolUseID != "tu-1" {
		t.Errorf("expected tool-use ID 'tu-1', got %q", req.ToolUseID)
	}
	if req.Description != "Run ls" {
		t.Errorf("expected description 'Run ls', got %q", req.Description)
	}

	// Simulate user allow.
	h.HandleAllow(req, nil, nil, "", false)

	// Result should arrive on the channel.
	select {
	case decision := <-resultCh:
		if _, ok := decision.(AllowDecision); !ok {
			t.Fatalf("expected AllowDecision, got %T", decision)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for decision")
	}

	// Tool should no longer be pending.
	if pc.IsPending("tu-1") {
		t.Fatal("tu-1 should not be pending after allow")
	}

	// Decision should be recorded.
	decisions := pc.GetDecisions()
	if len(decisions) != 1 {
		t.Fatalf("expected 1 decision, got %d", len(decisions))
	}
	if decisions[0].Decision != "accept" {
		t.Errorf("expected decision 'accept', got %q", decisions[0].Decision)
	}

	// Logger should have it.
	if len(logger.Decisions) != 1 {
		t.Fatalf("expected 1 logged decision, got %d", len(logger.Decisions))
	}
}

func TestInteractiveHandler_Deny(t *testing.T) {
	logger := &CollectingLogger{}
	pc := NewPermissionContext(logger)
	h := &InteractiveHandler{PermCtx: pc}

	cmd, resultCh := h.RequestPermission("Bash", "tu-2", "rm -rf /", nil, "default")
	msg := cmd()
	req := msg.(PermissionRequestMsg)

	h.HandleDeny(req, "too dangerous")

	select {
	case decision := <-resultCh:
		d, ok := decision.(DenyDecision)
		if !ok {
			t.Fatalf("expected DenyDecision, got %T", decision)
		}
		if d.Reason != "too dangerous" {
			t.Errorf("expected reason 'too dangerous', got %q", d.Reason)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for decision")
	}

	// Should be session-denied.
	reason, ok := pc.IsSessionDenied("Bash")
	if !ok {
		t.Fatal("Bash should be session-denied")
	}
	if reason != "user denied permission" {
		t.Errorf("expected reason 'user denied permission', got %q", reason)
	}

	// Decision record should show reject.
	decisions := pc.GetDecisions()
	if len(decisions) != 1 {
		t.Fatalf("expected 1 decision, got %d", len(decisions))
	}
	if decisions[0].Decision != "reject" {
		t.Errorf("expected decision 'reject', got %q", decisions[0].Decision)
	}
	if decisions[0].Source != "user_reject" {
		t.Errorf("expected source 'user_reject', got %q", decisions[0].Source)
	}
}

func TestInteractiveHandler_DenyNoFeedback(t *testing.T) {
	pc := NewPermissionContext(nil)
	h := &InteractiveHandler{PermCtx: pc}

	cmd, resultCh := h.RequestPermission("Write", "tu-3", "Write file", nil, "default")
	msg := cmd()
	req := msg.(PermissionRequestMsg)

	h.HandleDeny(req, "")

	select {
	case decision := <-resultCh:
		d, ok := decision.(DenyDecision)
		if !ok {
			t.Fatalf("expected DenyDecision, got %T", decision)
		}
		if d.Reason != "user denied permission" {
			t.Errorf("expected default reason, got %q", d.Reason)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for decision")
	}
}

func TestInteractiveHandler_Abort(t *testing.T) {
	logger := &CollectingLogger{}
	pc := NewPermissionContext(logger)
	h := &InteractiveHandler{PermCtx: pc}

	cmd, resultCh := h.RequestPermission("Bash", "tu-4", "Run something", nil, "default")
	msg := cmd()
	req := msg.(PermissionRequestMsg)

	h.HandleAbort(req)

	select {
	case decision := <-resultCh:
		d, ok := decision.(DenyDecision)
		if !ok {
			t.Fatalf("expected DenyDecision, got %T", decision)
		}
		if d.Reason != "user aborted" {
			t.Errorf("expected 'user aborted', got %q", d.Reason)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for decision")
	}

	// Logger should have cancelled event.
	if len(logger.Cancelled) != 1 {
		t.Fatalf("expected 1 cancelled event, got %d", len(logger.Cancelled))
	}
	if logger.Cancelled[0] != "tu-4" {
		t.Errorf("expected cancelled tu-4, got %q", logger.Cancelled[0])
	}
}

func TestInteractiveHandler_DoubleAllowIgnored(t *testing.T) {
	// Source: PermissionContext.ts — resolve-once guard prevents double-resolve
	pc := NewPermissionContext(nil)
	h := &InteractiveHandler{PermCtx: pc}

	cmd, resultCh := h.RequestPermission("Bash", "tu-5", "echo", nil, "default")
	msg := cmd()
	req := msg.(PermissionRequestMsg)

	// First allow wins.
	h.HandleAllow(req, nil, nil, "", false)

	// Second allow is a no-op (claim fails).
	h.HandleAllow(req, nil, nil, "", true)

	// Only one decision should arrive.
	select {
	case <-resultCh:
		// good
	case <-time.After(time.Second):
		t.Fatal("timed out")
	}

	// Only one decision recorded.
	if len(pc.GetDecisions()) != 1 {
		t.Fatalf("expected 1 decision, got %d", len(pc.GetDecisions()))
	}
}

func TestInteractiveHandler_AllowThenDenyIgnored(t *testing.T) {
	pc := NewPermissionContext(nil)
	h := &InteractiveHandler{PermCtx: pc}

	cmd, resultCh := h.RequestPermission("Bash", "tu-6", "echo", nil, "default")
	msg := cmd()
	req := msg.(PermissionRequestMsg)

	h.HandleAllow(req, nil, nil, "", false)
	h.HandleDeny(req, "should be ignored") // claim fails

	select {
	case decision := <-resultCh:
		if _, ok := decision.(AllowDecision); !ok {
			t.Fatalf("expected AllowDecision (first wins), got %T", decision)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out")
	}
}

func TestShouldIgnoreInteraction(t *testing.T) {
	// Source: interactiveHandler.ts — 200ms grace period
	now := time.Now()
	// Within grace period: should ignore.
	if !ShouldIgnoreInteraction(now) {
		t.Error("interaction within grace period should be ignored")
	}

	// After grace period: should NOT ignore.
	old := time.Now().Add(-500 * time.Millisecond)
	if ShouldIgnoreInteraction(old) {
		t.Error("interaction after grace period should not be ignored")
	}
}

func TestInteractiveHandler_AllowRecordsSessionAllowed(t *testing.T) {
	pc := NewPermissionContext(nil)
	h := &InteractiveHandler{PermCtx: pc}

	cmd, _ := h.RequestPermission("Edit", "tu-7", "Edit file", nil, "default")
	msg := cmd()
	req := msg.(PermissionRequestMsg)

	h.HandleAllow(req, nil, nil, "", true)

	if !pc.IsSessionAllowed("Edit") {
		t.Fatal("Edit should be session-allowed after allow")
	}
}
