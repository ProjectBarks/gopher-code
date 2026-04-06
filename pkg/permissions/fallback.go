package permissions

import (
	"context"
	"fmt"
)

// ---------------------------------------------------------------------------
// T77: FallbackPermissionRequest routing
// Source: src/remote/remotePermissionBridge.ts — Tool stub routes to
//         FallbackPermissionRequest for unknown/remote tools.
// ---------------------------------------------------------------------------

// FallbackPolicy is a permission policy for tools that don't match any
// specific handler. It implements the conservative default: always ask
// the user for permission (or deny in non-interactive modes).
//
// Remote CCR tool stubs use this policy because the local CLI doesn't know
// the tool's capabilities. The safe default is to prompt the user.
//
// Source: remotePermissionBridge.ts — stub routes to FallbackPermissionRequest
type FallbackPolicy struct {
	// Mode controls fallback behavior:
	//   - ModeBypassPermissions / ModeDontAsk → allow
	//   - Everything else → ask
	Mode PermissionMode
}

// NewFallbackPolicy creates a FallbackPolicy with the given mode.
func NewFallbackPolicy(mode PermissionMode) *FallbackPolicy {
	return &FallbackPolicy{Mode: mode}
}

// Check evaluates permission for an unknown tool.
// Source: The TS FallbackPermissionRequest path always prompts unless
// bypass mode is active.
func (f *FallbackPolicy) Check(_ context.Context, toolName string, _ string) PermissionDecision {
	// In bypass / dontAsk modes, allow everything.
	if f.Mode == ModeBypassPermissions || f.Mode == ModeDontAsk {
		return AllowDecision{}
	}

	// For all other modes, ask the user.
	return AskDecision{
		Message: fmt.Sprintf("Allow %s? (unknown tool — requires explicit permission)", toolName),
	}
}
