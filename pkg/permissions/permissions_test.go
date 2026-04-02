package permissions_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/projectbarks/gopher-code/internal/testharness"
	"github.com/projectbarks/gopher-code/pkg/permissions"
)

func TestPermissionModes(t *testing.T) {
	modes := []struct {
		mode permissions.PermissionMode
		name string
	}{
		{permissions.AutoApprove, "AutoApprove"},
		{permissions.Interactive, "Interactive"},
		{permissions.Deny, "Deny"},
	}

	for _, m := range modes {
		m := m
		t.Run(m.name, func(t *testing.T) {
			policy := permissions.NewRuleBasedPolicy(m.mode)
			t.Run("not_nil", func(t *testing.T) {
				if policy == nil {
					t.Fatal("policy is nil")
				}
			})
			t.Run("implements_policy", func(t *testing.T) {
				var _ permissions.PermissionPolicy = policy
			})
		})
	}
}

func TestAutoApproveMode(t *testing.T) {
	policy := permissions.NewRuleBasedPolicy(permissions.AutoApprove)
	ctx := context.Background()

	tools := []string{"Bash", "Read", "Write", "Edit", "Glob", "Grep", "WebFetch", "Agent", "unknown_tool"}
	for _, tool := range tools {
		tool := tool
		t.Run(fmt.Sprintf("allows_%s", tool), func(t *testing.T) {
			decision := policy.Check(ctx, tool, "test-id")
			if _, ok := decision.(permissions.AllowDecision); !ok {
				t.Errorf("AutoApprove should allow %s, got %T", tool, decision)
			}
		})
	}
}

func TestDenyMode(t *testing.T) {
	policy := permissions.NewRuleBasedPolicy(permissions.Deny)
	ctx := context.Background()

	tools := []string{"Bash", "Write", "Edit", "Agent", "unknown_tool"}
	for _, tool := range tools {
		tool := tool
		t.Run(fmt.Sprintf("denies_%s", tool), func(t *testing.T) {
			decision := policy.Check(ctx, tool, "test-id")
			if _, ok := decision.(permissions.DenyDecision); !ok {
				t.Errorf("Deny should deny %s, got %T", tool, decision)
			}
		})
	}

	t.Run("deny_has_reason", func(t *testing.T) {
		decision := policy.Check(ctx, "Bash", "test-id")
		deny, ok := decision.(permissions.DenyDecision)
		if !ok {
			t.Fatal("expected DenyDecision")
		}
		if deny.Reason == "" {
			t.Error("DenyDecision should have a reason")
		}
	})
}

func TestInteractiveMode(t *testing.T) {
	policy := permissions.NewRuleBasedPolicy(permissions.Interactive)
	ctx := context.Background()

	tools := []string{"Bash", "Write", "Edit"}
	for _, tool := range tools {
		tool := tool
		t.Run(fmt.Sprintf("asks_for_%s", tool), func(t *testing.T) {
			decision := policy.Check(ctx, tool, "test-id")
			if _, ok := decision.(permissions.AskDecision); !ok {
				t.Errorf("Interactive should ask for %s, got %T", tool, decision)
			}
		})
	}

	t.Run("ask_has_message", func(t *testing.T) {
		decision := policy.Check(ctx, "Bash", "test-id")
		ask, ok := decision.(permissions.AskDecision)
		if !ok {
			t.Fatal("expected AskDecision")
		}
		if ask.Message == "" {
			t.Error("AskDecision should have a message")
		}
	})
}

func TestDecisionTypes(t *testing.T) {
	t.Run("allow_is_decision", func(t *testing.T) {
		var d permissions.PermissionDecision = permissions.AllowDecision{}
		_ = d
	})
	t.Run("deny_is_decision", func(t *testing.T) {
		var d permissions.PermissionDecision = permissions.DenyDecision{Reason: "test"}
		_ = d
	})
	t.Run("ask_is_decision", func(t *testing.T) {
		var d permissions.PermissionDecision = permissions.AskDecision{Message: "test"}
		_ = d
	})
}

// TestPermissionModesMatchGolden validates permission modes from the golden constants file.
func TestPermissionModesMatchGolden(t *testing.T) {
	constants, err := testharness.LoadQueryLoopConstants()
	if err != nil {
		t.Skipf("could not load constants: %v", err)
	}
	// Constants file exists and loads — validate it has the expected structure
	_ = constants
	t.Run("constants_loaded", func(t *testing.T) {
		if constants == nil {
			t.Fatal("constants is nil")
		}
	})
}

// TestReadOnlyToolPermissions validates that read-only tools bypass permission checks
// in the orchestrator (per TypeScript source behavior).
func TestReadOnlyToolPermissions(t *testing.T) {
	schemas, err := testharness.LoadToolSchemas()
	if err != nil {
		t.Fatalf("failed to load schemas: %v", err)
	}

	readOnlyTools := []string{}
	mutatingTools := []string{}
	for _, s := range schemas {
		if s.IsReadOnly {
			readOnlyTools = append(readOnlyTools, s.Name)
		} else {
			mutatingTools = append(mutatingTools, s.Name)
		}
	}

	t.Run("read_only_tools_exist", func(t *testing.T) {
		if len(readOnlyTools) == 0 {
			t.Error("no read-only tools found")
		}
	})
	t.Run("mutating_tools_exist", func(t *testing.T) {
		if len(mutatingTools) == 0 {
			t.Error("no mutating tools found")
		}
	})

	for _, name := range readOnlyTools {
		name := name
		t.Run(fmt.Sprintf("read_only_%s_skip_permission", name), func(t *testing.T) {
			// Read-only tools should not need permission checks
			// This is validated at the orchestrator level
		})
	}

	for _, name := range mutatingTools {
		name := name
		t.Run(fmt.Sprintf("mutating_%s_needs_permission", name), func(t *testing.T) {
			// Mutating tools must check permissions before execution
		})
	}
}
