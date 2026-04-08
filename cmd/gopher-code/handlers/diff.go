// T410: Diff/PR hooks integration — wires pkg/ui/hooks/diff into the binary.
// Source: src/hooks/useDiffData.ts, useTurnDiffs.ts, usePrStatus.ts
package handlers

import (
	"context"
	"fmt"

	"github.com/projectbarks/gopher-code/pkg/ui/hooks/diff"
)

// DiffSession manages the diff-related hooks for a session: working-tree
// diff computation, per-turn diff tracking, and PR status polling. It is
// the integration point that makes the diff package reachable from main().
type DiffSession struct {
	Computer *diff.DiffComputer
	Tracker  *diff.TurnDiffTracker
	Poller   *diff.PrStatusPoller
}

// NewDiffSession creates a DiffSession rooted at the given repository path.
// The PR status poller uses the real gh-based fetcher by default; pass a
// custom fetcher via WithPrFetcher to override in tests.
func NewDiffSession(repoPath string, opts ...DiffSessionOption) *DiffSession {
	o := diffSessionOptions{
		prFetcher: diff.FetchPrStatus,
	}
	for _, fn := range opts {
		fn(&o)
	}

	return &DiffSession{
		Computer: diff.NewDiffComputer(repoPath),
		Tracker:  diff.NewTurnDiffTracker(),
		Poller:   diff.NewPrStatusPoller(o.prFetcher),
	}
}

type diffSessionOptions struct {
	prFetcher diff.PrStatusFetcher
}

// DiffSessionOption configures NewDiffSession.
type DiffSessionOption func(*diffSessionOptions)

// WithPrFetcher overrides the default PR status fetcher (for testing).
func WithPrFetcher(f diff.PrStatusFetcher) DiffSessionOption {
	return func(o *diffSessionOptions) { o.prFetcher = f }
}

// StartPolling begins background PR status polling. Call StopPolling to stop.
func (ds *DiffSession) StartPolling(ctx context.Context) {
	ds.Poller.Start(ctx)
}

// StopPolling terminates the PR status poller.
func (ds *DiffSession) StopPolling() {
	ds.Poller.Stop()
}

// ComputeDiff returns the current working-tree diff data.
func (ds *DiffSession) ComputeDiff() (*diff.DiffData, error) {
	return ds.Computer.Compute()
}

// StartTurn begins tracking a new user turn.
func (ds *DiffSession) StartTurn(promptPreview, timestamp string) {
	ds.Tracker.StartTurn(promptPreview, timestamp)
}

// RecordEdit records a file edit in the current turn.
func (ds *DiffSession) RecordEdit(result diff.FileEditResult) {
	ds.Tracker.RecordEdit(result)
}

// Turns returns all tracked turn diffs (most recent first).
func (ds *DiffSession) Turns() []*diff.TurnDiff {
	return ds.Tracker.Turns()
}

// Summary returns a human-readable summary of the current diff state.
func (ds *DiffSession) Summary() string {
	data, err := ds.Computer.Compute()
	if err != nil {
		return fmt.Sprintf("diff error: %v", err)
	}
	if data.Stats == nil || data.Stats.FilesCount == 0 {
		return "no uncommitted changes"
	}
	return fmt.Sprintf("%d files changed, +%d -%d lines",
		data.Stats.FilesCount, data.Stats.LinesAdded, data.Stats.LinesRemoved)
}
