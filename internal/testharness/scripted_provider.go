package testharness

import (
	"context"
	"sync"

	"github.com/projectbarks/gopher-code/pkg/provider"
)

// TurnScript is either a successful sequence of events or an error.
type TurnScript struct {
	Events []provider.StreamResult // scripted stream events (used if Err is nil)
	Err    error                   // if non-nil, Stream() returns this error
}

// ScriptedProvider plays back pre-recorded stream turns for deterministic testing.
type ScriptedProvider struct {
	mu               sync.Mutex
	turns            []TurnScript
	nextTurn         int
	CapturedRequests []*provider.ModelRequest
}

// NewScriptedProvider creates a provider from a sequence of turn scripts.
func NewScriptedProvider(turns ...TurnScript) *ScriptedProvider {
	return &ScriptedProvider{
		turns:            turns,
		CapturedRequests: make([]*provider.ModelRequest, 0),
	}
}

// Stream plays back the next scripted turn. Panics if no more turns are available.
func (sp *ScriptedProvider) Stream(_ context.Context, req provider.ModelRequest) (<-chan provider.StreamResult, error) {
	sp.mu.Lock()
	defer sp.mu.Unlock()

	reqCopy := req
	sp.CapturedRequests = append(sp.CapturedRequests, &reqCopy)

	if sp.nextTurn >= len(sp.turns) {
		panic("ScriptedProvider: no more scripted turns")
	}
	turn := sp.turns[sp.nextTurn]
	sp.nextTurn++

	if turn.Err != nil {
		return nil, turn.Err
	}

	ch := make(chan provider.StreamResult, len(turn.Events))
	go func() {
		defer close(ch)
		for _, event := range turn.Events {
			ch <- event
		}
	}()
	return ch, nil
}

// Name returns the provider name.
func (sp *ScriptedProvider) Name() string {
	return "scripted-test-provider"
}

// Requests returns a copy of all captured requests.
func (sp *ScriptedProvider) Requests() []*provider.ModelRequest {
	sp.mu.Lock()
	defer sp.mu.Unlock()
	result := make([]*provider.ModelRequest, len(sp.CapturedRequests))
	copy(result, sp.CapturedRequests)
	return result
}
