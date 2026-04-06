package permissions

import (
	"testing"
)

func TestPermissionContext_AllowAndDeny(t *testing.T) {
	logger := &CollectingLogger{}
	pc := NewPermissionContext(logger)

	// Initially nothing is allowed or denied.
	if pc.IsSessionAllowed("Bash") {
		t.Fatal("Bash should not be allowed initially")
	}
	if _, ok := pc.IsSessionDenied("Bash"); ok {
		t.Fatal("Bash should not be denied initially")
	}

	// Allow Bash.
	pc.MarkAllowed("Bash")
	if !pc.IsSessionAllowed("Bash") {
		t.Fatal("Bash should be allowed after MarkAllowed")
	}

	// Deny replaces allow.
	pc.MarkDenied("Bash", "dangerous command")
	if pc.IsSessionAllowed("Bash") {
		t.Fatal("Bash should not be allowed after MarkDenied")
	}
	reason, ok := pc.IsSessionDenied("Bash")
	if !ok {
		t.Fatal("Bash should be denied after MarkDenied")
	}
	if reason != "dangerous command" {
		t.Fatalf("expected reason 'dangerous command', got %q", reason)
	}

	// Allow replaces deny.
	pc.MarkAllowed("Bash")
	if !pc.IsSessionAllowed("Bash") {
		t.Fatal("Bash should be allowed again")
	}
	if _, ok := pc.IsSessionDenied("Bash"); ok {
		t.Fatal("Bash should not be denied after re-allow")
	}
}

func TestPermissionContext_PendingTracking(t *testing.T) {
	pc := NewPermissionContext(nil)

	if pc.IsPending("tu-1") {
		t.Fatal("should not be pending initially")
	}
	if pc.PendingCount() != 0 {
		t.Fatal("pending count should be 0 initially")
	}

	pc.MarkPending("tu-1")
	pc.MarkPending("tu-2")

	if !pc.IsPending("tu-1") {
		t.Fatal("tu-1 should be pending")
	}
	if pc.PendingCount() != 2 {
		t.Fatalf("expected 2 pending, got %d", pc.PendingCount())
	}

	pc.ClearPending("tu-1")
	if pc.IsPending("tu-1") {
		t.Fatal("tu-1 should not be pending after clear")
	}
	if pc.PendingCount() != 1 {
		t.Fatalf("expected 1 pending, got %d", pc.PendingCount())
	}
}

func TestPermissionContext_RecordDecision(t *testing.T) {
	logger := &CollectingLogger{}
	pc := NewPermissionContext(logger)

	pc.RecordDecision("Bash", "tu-1", "accept", "config")
	pc.RecordDecision("Write", "tu-2", "reject", "user_reject")
	pc.RecordDecision("Edit", "tu-3", "accept", "hook")

	decisions := pc.GetDecisions()
	if len(decisions) != 3 {
		t.Fatalf("expected 3 decisions, got %d", len(decisions))
	}

	// Verify first decision.
	if decisions[0].ToolName != "Bash" {
		t.Errorf("expected tool 'Bash', got %q", decisions[0].ToolName)
	}
	if decisions[0].Decision != "accept" {
		t.Errorf("expected decision 'accept', got %q", decisions[0].Decision)
	}
	if decisions[0].Source != "config" {
		t.Errorf("expected source 'config', got %q", decisions[0].Source)
	}

	// Verify reject.
	if decisions[1].Decision != "reject" {
		t.Errorf("expected decision 'reject', got %q", decisions[1].Decision)
	}
	if decisions[1].Source != "user_reject" {
		t.Errorf("expected source 'user_reject', got %q", decisions[1].Source)
	}

	// Logger should have received the same decisions.
	if len(logger.Decisions) != 3 {
		t.Fatalf("logger expected 3 decisions, got %d", len(logger.Decisions))
	}
}

func TestPermissionContext_NilLoggerSafe(t *testing.T) {
	// Passing nil logger should not panic.
	pc := NewPermissionContext(nil)
	pc.RecordDecision("Bash", "tu-1", "accept", "config")

	decisions := pc.GetDecisions()
	if len(decisions) != 1 {
		t.Fatalf("expected 1 decision, got %d", len(decisions))
	}
}

func TestPermissionContext_GetDecisionsIsCopy(t *testing.T) {
	pc := NewPermissionContext(nil)
	pc.RecordDecision("Bash", "tu-1", "accept", "config")

	d1 := pc.GetDecisions()
	d1[0].ToolName = "MUTATED"

	d2 := pc.GetDecisions()
	if d2[0].ToolName != "Bash" {
		t.Fatal("GetDecisions should return a copy, not a reference")
	}
}
