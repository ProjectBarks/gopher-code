package testharness

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/projectbarks/gopher-code/pkg/compact"
	"github.com/projectbarks/gopher-code/pkg/message"
	"github.com/projectbarks/gopher-code/pkg/permissions"
	"github.com/projectbarks/gopher-code/pkg/provider"
	"github.com/projectbarks/gopher-code/pkg/session"
)

// --- Session builders ---

// MakeSession creates a default test session with "hello" user message.
// Uses 1ms retry base delay to keep tests fast.
func MakeSession() *session.SessionState {
	cfg := session.SessionConfig{
		Model:          "test-model",
		MaxTurns:       100,
		TokenBudget:    compact.DefaultBudget(),
		PermissionMode: permissions.AutoApprove,
		RetryBaseDelay: 1 * time.Millisecond,
	}
	s := session.New(cfg, os.TempDir())
	s.PushMessage(message.UserMessage("hello"))
	return s
}

// MakeSessionWithConfig creates a test session with a custom config.
func MakeSessionWithConfig(cfg session.SessionConfig) *session.SessionState {
	s := session.New(cfg, os.TempDir())
	s.PushMessage(message.UserMessage("hello"))
	return s
}

// MakeSessionWithCWD creates a test session with a specific working directory.
func MakeSessionWithCWD(cwd string) *session.SessionState {
	cfg := session.SessionConfig{
		Model:          "test-model",
		MaxTurns:       100,
		TokenBudget:    compact.DefaultBudget(),
		PermissionMode: permissions.AutoApprove,
		RetryBaseDelay: 1 * time.Millisecond,
	}
	s := session.New(cfg, cwd)
	s.PushMessage(message.UserMessage("hello"))
	return s
}

// --- Turn builders ---

// MakeTextTurn creates a scripted text response turn.
func MakeTextTurn(text string, stop provider.StopReason) TurnScript {
	return MakeTextTurnWithUsage(text, stop, provider.Usage{})
}

// MakeTextTurnWithUsage creates a text turn with specific usage stats.
func MakeTextTurnWithUsage(text string, stop provider.StopReason, usage provider.Usage) TurnScript {
	sr := stop
	return TurnScript{
		Events: []provider.StreamResult{
			{Event: &provider.StreamEvent{
				Type: provider.EventTextDelta,
				Text: text,
			}},
			{Event: &provider.StreamEvent{
				Type: provider.EventMessageDone,
				Response: &provider.ModelResponse{
					ID:         fmt.Sprintf("resp-text-%s", shortUUID()),
					Content:    []provider.ResponseContent{{Type: "text", Text: text}},
					StopReason: &sr,
					Usage:      usage,
				},
			}},
		},
	}
}

// MakeToolTurn creates a scripted tool-use turn.
func MakeToolTurn(toolID, toolName string, input json.RawMessage, stop provider.StopReason) TurnScript {
	return MakeToolTurnWithUsage(toolID, toolName, input, stop, provider.Usage{})
}

// MakeToolTurnWithUsage creates a tool turn with specific usage stats.
func MakeToolTurnWithUsage(toolID, toolName string, input json.RawMessage, stop provider.StopReason, usage provider.Usage) TurnScript {
	sr := stop
	return TurnScript{
		Events: []provider.StreamResult{
			{Event: &provider.StreamEvent{
				Type: provider.EventContentBlockStart,
				Content: &provider.ResponseContent{
					Type:  "tool_use",
					ID:    toolID,
					Name:  toolName,
					Input: input,
				},
			}},
			{Event: &provider.StreamEvent{
				Type:        provider.EventInputJsonDelta,
				PartialJSON: string(input),
			}},
			{Event: &provider.StreamEvent{
				Type: provider.EventMessageDone,
				Response: &provider.ModelResponse{
					ID: fmt.Sprintf("resp-tool-%s", shortUUID()),
					Content: []provider.ResponseContent{{
						Type:  "tool_use",
						ID:    toolID,
						Name:  toolName,
						Input: input,
					}},
					StopReason: &sr,
					Usage:      usage,
				},
			}},
		},
	}
}

// ToolSpec defines a tool call for MakeMultiToolTurn.
type ToolSpec struct {
	ID    string
	Name  string
	Input json.RawMessage
}

// MakeMultiToolTurn creates a turn with multiple concurrent tool calls.
func MakeMultiToolTurn(toolSpecs []ToolSpec, stop provider.StopReason) TurnScript {
	var events []provider.StreamResult
	var responseContent []provider.ResponseContent

	for i, spec := range toolSpecs {
		events = append(events, provider.StreamResult{
			Event: &provider.StreamEvent{
				Type:  provider.EventContentBlockStart,
				Index: i,
				Content: &provider.ResponseContent{
					Type:  "tool_use",
					ID:    spec.ID,
					Name:  spec.Name,
					Input: spec.Input,
				},
			},
		})
		events = append(events, provider.StreamResult{
			Event: &provider.StreamEvent{
				Type:        provider.EventInputJsonDelta,
				Index:       i,
				PartialJSON: string(spec.Input),
			},
		})
		responseContent = append(responseContent, provider.ResponseContent{
			Type:  "tool_use",
			ID:    spec.ID,
			Name:  spec.Name,
			Input: spec.Input,
		})
	}

	sr := stop
	events = append(events, provider.StreamResult{
		Event: &provider.StreamEvent{
			Type: provider.EventMessageDone,
			Response: &provider.ModelResponse{
				ID:         fmt.Sprintf("resp-multi-%s", shortUUID()),
				Content:    responseContent,
				StopReason: &sr,
				Usage:      provider.Usage{},
			},
		},
	})

	return TurnScript{Events: events}
}

// MakeChunkedTextTurn creates a text turn split into multiple TextDelta events.
func MakeChunkedTextTurn(chunks []string, stop provider.StopReason) TurnScript {
	var events []provider.StreamResult
	fullText := ""
	for _, chunk := range chunks {
		fullText += chunk
		events = append(events, provider.StreamResult{
			Event: &provider.StreamEvent{
				Type: provider.EventTextDelta,
				Text: chunk,
			},
		})
	}

	sr := stop
	events = append(events, provider.StreamResult{
		Event: &provider.StreamEvent{
			Type: provider.EventMessageDone,
			Response: &provider.ModelResponse{
				ID:         fmt.Sprintf("resp-chunked-%s", shortUUID()),
				Content:    []provider.ResponseContent{{Type: "text", Text: fullText}},
				StopReason: &sr,
				Usage:      provider.Usage{},
			},
		},
	})

	return TurnScript{Events: events}
}

// MakeToolTurnWithJSONChunks creates a tool turn where JSON input arrives in multiple deltas.
func MakeToolTurnWithJSONChunks(toolID, toolName string, jsonChunks []string, fullInput json.RawMessage, stop provider.StopReason) TurnScript {
	var events []provider.StreamResult

	events = append(events, provider.StreamResult{
		Event: &provider.StreamEvent{
			Type: provider.EventContentBlockStart,
			Content: &provider.ResponseContent{
				Type:  "tool_use",
				ID:    toolID,
				Name:  toolName,
				Input: fullInput,
			},
		},
	})

	for _, chunk := range jsonChunks {
		events = append(events, provider.StreamResult{
			Event: &provider.StreamEvent{
				Type:        provider.EventInputJsonDelta,
				PartialJSON: chunk,
			},
		})
	}

	sr := stop
	events = append(events, provider.StreamResult{
		Event: &provider.StreamEvent{
			Type: provider.EventMessageDone,
			Response: &provider.ModelResponse{
				ID: fmt.Sprintf("resp-json-chunks-%s", shortUUID()),
				Content: []provider.ResponseContent{{
					Type:  "tool_use",
					ID:    toolID,
					Name:  toolName,
					Input: fullInput,
				}},
				StopReason: &sr,
				Usage:      provider.Usage{},
			},
		},
	})

	return TurnScript{Events: events}
}

// MakeErrorTurn creates a turn that returns an error from Stream().
func MakeErrorTurn(err error) TurnScript {
	return TurnScript{Err: err}
}

func shortUUID() string {
	return uuid.New().String()[:8]
}
