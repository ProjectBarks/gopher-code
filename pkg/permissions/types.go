package permissions

type PermissionMode int

const (
	AutoApprove PermissionMode = iota
	Interactive
	Deny
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
