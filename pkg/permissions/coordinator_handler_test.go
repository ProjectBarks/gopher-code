package permissions

import (
	"context"
	"errors"
	"testing"
)

func TestCoordinatorHandler_HookApproves(t *testing.T) {
	// Source: coordinatorHandler.ts:33-35 — hook returns decision
	h := &CoordinatorHandler{
		Hooks: func(_ context.Context, toolName, toolInput, mode string) (PermissionDecision, error) {
			return AllowDecision{}, nil
		},
	}

	decision := h.Handle(context.Background(), "Bash", "ls", "default")
	if _, ok := decision.(AllowDecision); !ok {
		t.Fatalf("expected AllowDecision from hook, got %T", decision)
	}
}

func TestCoordinatorHandler_HookDenies(t *testing.T) {
	h := &CoordinatorHandler{
		Hooks: func(_ context.Context, toolName, toolInput, mode string) (PermissionDecision, error) {
			return DenyDecision{Reason: "denied by hook"}, nil
		},
	}

	decision := h.Handle(context.Background(), "Bash", "rm -rf /", "default")
	d, ok := decision.(DenyDecision)
	if !ok {
		t.Fatalf("expected DenyDecision from hook, got %T", decision)
	}
	if d.Reason != "denied by hook" {
		t.Errorf("expected reason 'denied by hook', got %q", d.Reason)
	}
}

func TestCoordinatorHandler_HookNilFallsToClassifier(t *testing.T) {
	// Source: coordinatorHandler.ts:41-46 — hook returns nil, classifier resolves
	h := &CoordinatorHandler{
		Hooks: func(_ context.Context, _, _, _ string) (PermissionDecision, error) {
			return nil, nil // no decision
		},
		Classifier: func(_ context.Context, _, _ string) (PermissionDecision, error) {
			return AllowDecision{}, nil
		},
	}

	decision := h.Handle(context.Background(), "Bash", "echo hello", "default")
	if _, ok := decision.(AllowDecision); !ok {
		t.Fatalf("expected AllowDecision from classifier, got %T", decision)
	}
}

func TestCoordinatorHandler_BothNilFallsThrough(t *testing.T) {
	// Source: coordinatorHandler.ts:60-61 — neither resolved, return nil
	h := &CoordinatorHandler{
		Hooks: func(_ context.Context, _, _, _ string) (PermissionDecision, error) {
			return nil, nil
		},
		Classifier: func(_ context.Context, _, _ string) (PermissionDecision, error) {
			return nil, nil
		},
	}

	decision := h.Handle(context.Background(), "Bash", "echo hello", "default")
	if decision != nil {
		t.Fatalf("expected nil (fall through to dialog), got %T", decision)
	}
}

func TestCoordinatorHandler_HookErrorFallsThrough(t *testing.T) {
	// Source: coordinatorHandler.ts:47-57 — errors fall through to dialog
	h := &CoordinatorHandler{
		Hooks: func(_ context.Context, _, _, _ string) (PermissionDecision, error) {
			return nil, errors.New("hook crashed")
		},
		Classifier: func(_ context.Context, _, _ string) (PermissionDecision, error) {
			return AllowDecision{}, nil
		},
	}

	// Hook error should be logged but classifier still runs.
	decision := h.Handle(context.Background(), "Bash", "echo hello", "default")
	if _, ok := decision.(AllowDecision); !ok {
		t.Fatalf("expected AllowDecision from classifier after hook error, got %T", decision)
	}
}

func TestCoordinatorHandler_ClassifierErrorFallsThrough(t *testing.T) {
	h := &CoordinatorHandler{
		Hooks: func(_ context.Context, _, _, _ string) (PermissionDecision, error) {
			return nil, nil
		},
		Classifier: func(_ context.Context, _, _ string) (PermissionDecision, error) {
			return nil, errors.New("classifier crashed")
		},
	}

	decision := h.Handle(context.Background(), "Bash", "echo hello", "default")
	if decision != nil {
		t.Fatalf("expected nil after classifier error, got %T", decision)
	}
}

func TestCoordinatorHandler_NoHooksNoClassifier(t *testing.T) {
	// Both nil — falls through immediately.
	h := &CoordinatorHandler{}

	decision := h.Handle(context.Background(), "Bash", "echo hello", "default")
	if decision != nil {
		t.Fatalf("expected nil with no hooks/classifier, got %T", decision)
	}
}
