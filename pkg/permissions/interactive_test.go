package permissions_test

import (
	"context"
	"strings"
	"testing"

	"github.com/projectbarks/gopher-code/pkg/permissions"
)

func TestInteractivePolicy_Yes(t *testing.T) {
	r := strings.NewReader("y\n")
	policy := permissions.NewInteractivePolicyWithReader(r)

	decision := policy.Check(context.Background(), "bash", "t1")
	if _, ok := decision.(permissions.AllowDecision); !ok {
		t.Fatalf("expected AllowDecision for 'y', got %T", decision)
	}
}

func TestInteractivePolicy_EmptyDefault(t *testing.T) {
	r := strings.NewReader("\n")
	policy := permissions.NewInteractivePolicyWithReader(r)

	decision := policy.Check(context.Background(), "bash", "t1")
	if _, ok := decision.(permissions.AllowDecision); !ok {
		t.Fatalf("expected AllowDecision for empty (default), got %T", decision)
	}
}

func TestInteractivePolicy_No(t *testing.T) {
	r := strings.NewReader("n\n")
	policy := permissions.NewInteractivePolicyWithReader(r)

	decision := policy.Check(context.Background(), "bash", "t1")
	d, ok := decision.(permissions.DenyDecision)
	if !ok {
		t.Fatalf("expected DenyDecision for 'n', got %T", decision)
	}
	if d.Reason != "user denied permission" {
		t.Errorf("expected reason 'user denied permission', got %q", d.Reason)
	}
}

func TestInteractivePolicy_Always(t *testing.T) {
	// "always\n" for first call, then no more input needed for second call
	r := strings.NewReader("always\n")
	policy := permissions.NewInteractivePolicyWithReader(r)

	// First call: should read "always" and allow
	decision := policy.Check(context.Background(), "bash", "t1")
	if _, ok := decision.(permissions.AllowDecision); !ok {
		t.Fatalf("expected AllowDecision for 'always', got %T", decision)
	}

	// Second call: should be auto-allowed without reading input
	decision = policy.Check(context.Background(), "bash", "t2")
	if _, ok := decision.(permissions.AllowDecision); !ok {
		t.Fatalf("expected AllowDecision for remembered 'always', got %T", decision)
	}
}

func TestInteractivePolicy_AlwaysPerTool(t *testing.T) {
	// "always" for bash, then "n" for write
	r := strings.NewReader("always\nn\n")
	policy := permissions.NewInteractivePolicyWithReader(r)

	// Allow bash always
	decision := policy.Check(context.Background(), "bash", "t1")
	if _, ok := decision.(permissions.AllowDecision); !ok {
		t.Fatalf("expected AllowDecision for bash 'always', got %T", decision)
	}

	// Deny write tool
	decision = policy.Check(context.Background(), "write", "t2")
	if _, ok := decision.(permissions.DenyDecision); !ok {
		t.Fatalf("expected DenyDecision for write 'n', got %T", decision)
	}

	// bash should still be allowed (remembered)
	decision = policy.Check(context.Background(), "bash", "t3")
	if _, ok := decision.(permissions.AllowDecision); !ok {
		t.Fatalf("expected AllowDecision for bash (remembered), got %T", decision)
	}
}

func TestInteractivePolicy_ShortAlways(t *testing.T) {
	r := strings.NewReader("a\n")
	policy := permissions.NewInteractivePolicyWithReader(r)

	decision := policy.Check(context.Background(), "bash", "t1")
	if _, ok := decision.(permissions.AllowDecision); !ok {
		t.Fatalf("expected AllowDecision for 'a', got %T", decision)
	}

	// Should be remembered
	decision = policy.Check(context.Background(), "bash", "t2")
	if _, ok := decision.(permissions.AllowDecision); !ok {
		t.Fatalf("expected AllowDecision for remembered 'a', got %T", decision)
	}
}

func TestInteractivePolicy_CaseInsensitive(t *testing.T) {
	r := strings.NewReader("YES\n")
	policy := permissions.NewInteractivePolicyWithReader(r)

	decision := policy.Check(context.Background(), "bash", "t1")
	if _, ok := decision.(permissions.AllowDecision); !ok {
		t.Fatalf("expected AllowDecision for 'YES', got %T", decision)
	}
}

func TestInteractivePolicy_NoInput(t *testing.T) {
	// Empty reader simulates EOF / no input
	r := strings.NewReader("")
	policy := permissions.NewInteractivePolicyWithReader(r)

	decision := policy.Check(context.Background(), "bash", "t1")
	d, ok := decision.(permissions.DenyDecision)
	if !ok {
		t.Fatalf("expected DenyDecision for EOF, got %T", decision)
	}
	if d.Reason != "no input received" {
		t.Errorf("expected reason 'no input received', got %q", d.Reason)
	}
}
