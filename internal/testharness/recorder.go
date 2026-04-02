package testharness

import (
	"context"
	"encoding/json"
	"os"
	"sync"
	"time"

	"github.com/projectbarks/gopher-code/pkg/provider"
)

// RecordedTurn captures one request/response pair from a session.
type RecordedTurn struct {
	Timestamp time.Time              `json:"timestamp"`
	Request   *provider.ModelRequest `json:"request"`
	Events    []provider.StreamEvent `json:"events"`
	Error     string                 `json:"error,omitempty"`
}

// RecordedSession is a complete session trace for replay.
type RecordedSession struct {
	Version   string         `json:"version"`
	Source    string         `json:"source"` // "ts" or "go"
	Turns     []RecordedTurn `json:"turns"`
}

// RecordingProvider wraps a real provider and logs all interactions.
type RecordingProvider struct {
	inner  provider.ModelProvider
	mu     sync.Mutex
	turns  []RecordedTurn
}

// NewRecordingProvider creates a provider that records all interactions.
func NewRecordingProvider(inner provider.ModelProvider) *RecordingProvider {
	return &RecordingProvider{inner: inner}
}

// Stream delegates to inner and records the request/events.
func (rp *RecordingProvider) Stream(ctx context.Context, req provider.ModelRequest) (<-chan provider.StreamResult, error) {
	ch, err := rp.inner.Stream(ctx, req)
	if err != nil {
		rp.mu.Lock()
		rp.turns = append(rp.turns, RecordedTurn{
			Timestamp: time.Now(),
			Request:   &req,
			Error:     err.Error(),
		})
		rp.mu.Unlock()
		return nil, err
	}

	// Wrap channel to capture events
	recordCh := make(chan provider.StreamResult)
	go func() {
		defer close(recordCh)
		var events []provider.StreamEvent
		for result := range ch {
			if result.Event != nil {
				events = append(events, *result.Event)
			}
			recordCh <- result
		}
		rp.mu.Lock()
		rp.turns = append(rp.turns, RecordedTurn{
			Timestamp: time.Now(),
			Request:   &req,
			Events:    events,
		})
		rp.mu.Unlock()
	}()

	return recordCh, nil
}

// Name returns the inner provider name.
func (rp *RecordingProvider) Name() string {
	return "recording:" + rp.inner.Name()
}

// Session returns the recorded session.
func (rp *RecordingProvider) Session(source string) *RecordedSession {
	rp.mu.Lock()
	defer rp.mu.Unlock()
	return &RecordedSession{
		Version: "1.0",
		Source:  source,
		Turns:   append([]RecordedTurn{}, rp.turns...),
	}
}

// SaveSession writes the recorded session to a file.
func (rp *RecordingProvider) SaveSession(path, source string) error {
	session := rp.Session(source)
	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// ReplayProvider plays back a recorded session.
type ReplayProvider struct {
	session  *RecordedSession
	mu       sync.Mutex
	nextTurn int
}

// NewReplayProvider creates a provider from a recorded session.
func NewReplayProvider(session *RecordedSession) *ReplayProvider {
	return &ReplayProvider{session: session}
}

// LoadReplayProvider loads a recorded session from a file.
func LoadReplayProvider(path string) (*ReplayProvider, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var session RecordedSession
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, err
	}
	return NewReplayProvider(&session), nil
}

// Stream replays the next recorded turn.
func (rp *ReplayProvider) Stream(_ context.Context, req provider.ModelRequest) (<-chan provider.StreamResult, error) {
	rp.mu.Lock()
	defer rp.mu.Unlock()

	if rp.nextTurn >= len(rp.session.Turns) {
		return nil, &replayError{msg: "no more recorded turns"}
	}

	turn := rp.session.Turns[rp.nextTurn]
	rp.nextTurn++

	if turn.Error != "" {
		return nil, &replayError{msg: turn.Error}
	}

	ch := make(chan provider.StreamResult, len(turn.Events))
	go func() {
		defer close(ch)
		for _, evt := range turn.Events {
			evt := evt
			ch <- provider.StreamResult{Event: &evt}
		}
	}()

	return ch, nil
}

// Name returns the replay provider name.
func (rp *ReplayProvider) Name() string {
	return "replay:" + rp.session.Source
}

type replayError struct {
	msg string
}

func (e *replayError) Error() string {
	return e.msg
}
