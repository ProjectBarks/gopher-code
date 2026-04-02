package testharness

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/projectbarks/gopher-code/pkg/tools"
)

// ResponseFunc is a function that produces a tool output from input.
type ResponseFunc func(input json.RawMessage) *tools.ToolOutput

// SpyTool records all calls and returns configurable responses.
type SpyTool struct {
	name       string
	readOnly   bool
	mu         sync.Mutex
	calls      []json.RawMessage
	responseFn ResponseFunc
}

// NewSpyTool creates a new spy tool with default "ok" response.
func NewSpyTool(name string, readOnly bool) *SpyTool {
	return &SpyTool{
		name:     name,
		readOnly: readOnly,
		responseFn: func(_ json.RawMessage) *tools.ToolOutput {
			return tools.SuccessOutput("ok")
		},
	}
}

// WithResponse sets a custom response function. Returns self for chaining.
func (s *SpyTool) WithResponse(fn ResponseFunc) *SpyTool {
	s.responseFn = fn
	return s
}

// CallCount returns the number of times Execute was called.
func (s *SpyTool) CallCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.calls)
}

// Calls returns a copy of all recorded inputs.
func (s *SpyTool) Calls() []json.RawMessage {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make([]json.RawMessage, len(s.calls))
	copy(result, s.calls)
	return result
}

// --- tools.Tool interface ---

func (s *SpyTool) Name() string        { return s.name }
func (s *SpyTool) Description() string  { return "A spy tool for testing." }
func (s *SpyTool) IsReadOnly() bool     { return s.readOnly }
func (s *SpyTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{"type":"object"}`)
}

func (s *SpyTool) Execute(_ context.Context, _ *tools.ToolContext, input json.RawMessage) (*tools.ToolOutput, error) {
	s.mu.Lock()
	inputCopy := make(json.RawMessage, len(input))
	copy(inputCopy, input)
	s.calls = append(s.calls, inputCopy)
	s.mu.Unlock()
	return s.responseFn(input), nil
}
