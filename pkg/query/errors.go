package query

import "fmt"

// AgentErrorKind identifies the category of agent error.
type AgentErrorKind int

const (
	ErrProvider AgentErrorKind = iota
	ErrMaxTurnsExceeded
	ErrContextTooLong
	ErrAborted
)

// AgentError represents an error from the agent loop.
type AgentError struct {
	Kind    AgentErrorKind
	Wrapped error
	Detail  string
}

func (e *AgentError) Error() string {
	switch e.Kind {
	case ErrMaxTurnsExceeded:
		return fmt.Sprintf("max turns exceeded: %s", e.Detail)
	case ErrContextTooLong:
		return "context too long after compaction"
	case ErrAborted:
		return "query aborted"
	case ErrProvider:
		if e.Wrapped != nil {
			return fmt.Sprintf("provider error: %v", e.Wrapped)
		}
		return "provider error"
	default:
		return "unknown agent error"
	}
}

func (e *AgentError) Unwrap() error {
	return e.Wrapped
}
