package tools

import "github.com/projectbarks/gopher-code/pkg/permissions"

// ToolContext provides context for tool execution.
type ToolContext struct {
	CWD         string
	Permissions permissions.PermissionPolicy
	SessionID   string
}
