package permissions

// PermissionMode matches the TS permission modes.
// Source: types/permissions.ts:16-29
type PermissionMode string

const (
	// External modes (user-facing)
	// Source: types/permissions.ts:16-22
	ModeDefault          PermissionMode = "default"
	ModeAcceptEdits      PermissionMode = "acceptEdits"
	ModeBypassPermissions PermissionMode = "bypassPermissions"
	ModeDontAsk          PermissionMode = "dontAsk"
	ModePlan             PermissionMode = "plan"

	// Internal modes
	// Source: types/permissions.ts:28
	ModeAuto   PermissionMode = "auto"

	// Legacy aliases for backward compatibility with existing Go code
	AutoApprove = ModeBypassPermissions
	Interactive = ModeDefault
	Deny        PermissionMode = "deny" // Not in TS, used internally
)

// PermissionDecision is a sealed interface for permission check results.
type PermissionDecision interface {
	isPermissionDecision()
}

type AllowDecision struct{}

func (AllowDecision) isPermissionDecision() {}

type DenyDecision struct {
	Reason string
}

func (DenyDecision) isPermissionDecision() {}

type AskDecision struct {
	Message string
}

func (AskDecision) isPermissionDecision() {}
