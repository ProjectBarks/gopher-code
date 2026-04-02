package tools

import (
	"context"
	"encoding/json"

	"github.com/projectbarks/gopher-code/pkg/permissions"
)

// HookRunner is the interface for pre/post tool execution hooks.
// This is defined here (rather than importing pkg/hooks) to avoid import cycles.
type HookRunner interface {
	RunForOrchestrator(ctx context.Context, hookType string, toolName string, toolInput json.RawMessage) (blocked bool, message string, err error)
}

// ToolContext provides context for tool execution.
type ToolContext struct {
	CWD         string
	Permissions permissions.PermissionPolicy
	SessionID   string
	Hooks       HookRunner // optional hook runner for pre/post tool hooks
}
