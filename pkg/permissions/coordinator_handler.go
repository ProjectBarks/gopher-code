package permissions

import (
	"context"
	"fmt"
	"log/slog"
)

// Source: src/hooks/toolPermission/handlers/coordinatorHandler.ts

// HookRunner executes permission-request hooks and returns a decision
// (or nil to fall through). This is the Go equivalent of ctx.runHooks().
type HookRunner func(ctx context.Context, toolName string, toolInput string, mode string) (PermissionDecision, error)

// ClassifierRunner executes the bash classifier and returns a decision
// (or nil to fall through). This is the Go equivalent of ctx.tryClassifier().
type ClassifierRunner func(ctx context.Context, toolName string, toolInput string) (PermissionDecision, error)

// CoordinatorHandler implements the coordinator-worker permission flow.
// For coordinator workers, automated checks (hooks then classifier) are
// awaited sequentially before falling through to the interactive dialog.
//
// Source: src/hooks/toolPermission/handlers/coordinatorHandler.ts
type CoordinatorHandler struct {
	Hooks      HookRunner      // permission-request hook runner (required)
	Classifier ClassifierRunner // bash classifier runner (optional, feature-gated)
	Logger     *slog.Logger
}

// Handle tries automated permission checks (hooks, then classifier).
// Returns a PermissionDecision if automated checks resolved the request,
// or nil if the caller should fall through to the interactive dialog.
//
// Source: coordinatorHandler.ts — handleCoordinatorPermission
func (h *CoordinatorHandler) Handle(ctx context.Context, toolName, toolInput, permissionMode string) PermissionDecision {
	logger := h.Logger
	if logger == nil {
		logger = slog.Default()
	}

	defer func() {
		if r := recover(); r != nil {
			logger.Error("panic in coordinator permission handler",
				slog.String("tool", toolName),
				slog.Any("panic", r),
			)
		}
	}()

	// Step 1: Try permission hooks first (fast, local).
	// Source: coordinatorHandler.ts:33-35
	if h.Hooks != nil {
		decision, err := h.Hooks(ctx, toolName, toolInput, permissionMode)
		if err != nil {
			logger.Error("permission hook failed",
				slog.String("tool", toolName),
				slog.String("error", err.Error()),
			)
			// Fall through to classifier/dialog.
		} else if decision != nil {
			return decision
		}
	}

	// Step 2: Try classifier (slow, inference — bash only).
	// Source: coordinatorHandler.ts:41-46
	if h.Classifier != nil {
		decision, err := h.Classifier(ctx, toolName, toolInput)
		if err != nil {
			// Source: coordinatorHandler.ts:47-57
			// Non-Error throws get a context prefix.
			logger.Error(fmt.Sprintf("Automated permission check failed: %s", err.Error()),
				slog.String("tool", toolName),
			)
			// Fall through to dialog.
		} else if decision != nil {
			return decision
		}
	}

	// Step 3: Neither resolved — fall through to dialog.
	// Source: coordinatorHandler.ts:60-61
	return nil
}
