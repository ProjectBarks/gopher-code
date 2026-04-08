package query

import (
	"errors"
	"fmt"
)

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
	Kind        AgentErrorKind
	Wrapped     error
	Detail      string
	UserMessage string // User-facing message from provider.APIError.UserFacingMessage()
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

// FallbackTriggeredError is thrown when 529 retries are exhausted and a fallback model is available.
// Source: services/api/withRetry.ts:160-168
type FallbackTriggeredError struct {
	OriginalModel string
	FallbackModel string
}

func (e *FallbackTriggeredError) Error() string {
	return fmt.Sprintf("Model fallback triggered: %s -> %s", e.OriginalModel, e.FallbackModel)
}

// IsFallbackTriggered checks if an error is a FallbackTriggeredError.
func IsFallbackTriggered(err error) (*FallbackTriggeredError, bool) {
	var fte *FallbackTriggeredError
	if errors.As(err, &fte) {
		return fte, true
	}
	return nil, false
}
