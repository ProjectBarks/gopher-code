package permissions

import "context"

// PermissionPolicy checks whether a tool is allowed to execute.
type PermissionPolicy interface {
	Check(ctx context.Context, toolName string, toolID string) PermissionDecision
}
