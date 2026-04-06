package permissions

import (
	"context"
	"testing"
)

// Source: src/remote/remotePermissionBridge.ts — FallbackPermissionRequest routing

func TestFallbackPolicy_DefaultMode_Asks(t *testing.T) {
	fp := NewFallbackPolicy(ModeDefault)
	decision := fp.Check(context.Background(), "UnknownTool", "")

	ask, ok := decision.(AskDecision)
	if !ok {
		t.Fatalf("expected AskDecision, got %T", decision)
	}
	if ask.Message == "" {
		t.Error("ask message should not be empty")
	}
}

func TestFallbackPolicy_BypassMode_Allows(t *testing.T) {
	fp := NewFallbackPolicy(ModeBypassPermissions)
	decision := fp.Check(context.Background(), "UnknownTool", "")

	if _, ok := decision.(AllowDecision); !ok {
		t.Errorf("expected AllowDecision in bypass mode, got %T", decision)
	}
}

func TestFallbackPolicy_DontAskMode_Allows(t *testing.T) {
	fp := NewFallbackPolicy(ModeDontAsk)
	decision := fp.Check(context.Background(), "UnknownTool", "")

	if _, ok := decision.(AllowDecision); !ok {
		t.Errorf("expected AllowDecision in dontAsk mode, got %T", decision)
	}
}

func TestFallbackPolicy_AcceptEditsMode_Asks(t *testing.T) {
	fp := NewFallbackPolicy(ModeAcceptEdits)
	decision := fp.Check(context.Background(), "UnknownTool", "")

	if _, ok := decision.(AskDecision); !ok {
		t.Errorf("expected AskDecision in acceptEdits mode, got %T", decision)
	}
}

func TestFallbackPolicy_PlanMode_Asks(t *testing.T) {
	fp := NewFallbackPolicy(ModePlan)
	decision := fp.Check(context.Background(), "UnknownTool", "")

	if _, ok := decision.(AskDecision); !ok {
		t.Errorf("expected AskDecision in plan mode, got %T", decision)
	}
}

func TestFallbackPolicy_ImplementsPolicy(t *testing.T) {
	// Verify FallbackPolicy satisfies PermissionPolicy interface.
	var _ PermissionPolicy = (*FallbackPolicy)(nil)
}
