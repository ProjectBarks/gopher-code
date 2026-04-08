package handlers

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/projectbarks/gopher-code/pkg/ui/hooks/diff"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// initTestRepo creates a temporary git repo with one committed file.
func initTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	repo, err := git.PlainInit(dir, false)
	require.NoError(t, err)

	fp := filepath.Join(dir, "hello.txt")
	require.NoError(t, os.WriteFile(fp, []byte("line1\nline2\nline3\n"), 0644))

	wt, err := repo.Worktree()
	require.NoError(t, err)
	_, err = wt.Add("hello.txt")
	require.NoError(t, err)
	_, err = wt.Commit("initial", &git.CommitOptions{
		Author: &object.Signature{Name: "test", Email: "t@t.com", When: time.Now()},
	})
	require.NoError(t, err)
	return dir
}

// TestDiffSession_Integration exercises the full DiffSession code path that
// is wired into the binary via main.go -> handlers.NewDiffSession.
func TestDiffSession_Integration(t *testing.T) {
	dir := initTestRepo(t)

	// Modify a file to create a diff.
	fp := filepath.Join(dir, "hello.txt")
	require.NoError(t, os.WriteFile(fp, []byte("line1\nchanged\nline3\n"), 0644))

	ds := NewDiffSession(dir)
	require.NotNil(t, ds)
	require.NotNil(t, ds.Computer)
	require.NotNil(t, ds.Tracker)
	require.NotNil(t, ds.Poller)

	// Test ComputeDiff — exercises DiffComputer through the handler layer.
	data, err := ds.ComputeDiff()
	require.NoError(t, err)
	require.NotNil(t, data.Stats)
	assert.GreaterOrEqual(t, data.Stats.FilesCount, 1)
	assert.Greater(t, data.Stats.LinesAdded, 0)
	assert.Greater(t, data.Stats.LinesRemoved, 0)

	// Test Summary.
	summary := ds.Summary()
	assert.Contains(t, summary, "files changed")
	assert.Contains(t, summary, "+")
	assert.Contains(t, summary, "-")

	// Test turn tracking — exercises TurnDiffTracker through the handler layer.
	ds.StartTurn("fix the bug", time.Now().Format(time.RFC3339))
	ds.RecordEdit(diff.FileEditResult{
		FilePath: "hello.txt",
		Hunks: []diff.Hunk{
			{Lines: []string{"+changed", "-line2"}},
		},
	})

	turns := ds.Turns()
	require.Len(t, turns, 1)
	assert.Equal(t, "fix the bug", turns[0].UserPromptPreview)
	assert.Equal(t, 1, turns[0].Stats.FilesChanged)
	assert.Equal(t, 1, turns[0].Stats.LinesAdded)
	assert.Equal(t, 1, turns[0].Stats.LinesRemoved)
}

// TestDiffSession_PrPollerWithMockFetcher exercises the PR status poller
// through the handler integration layer with a mock fetcher.
func TestDiffSession_PrPollerWithMockFetcher(t *testing.T) {
	dir := initTestRepo(t)

	fetcher := func(ctx context.Context) (*diff.PrStatusState, error) {
		return &diff.PrStatusState{
			Number:      99,
			URL:         "https://github.com/org/repo/pull/99",
			ReviewState: diff.PrApproved,
			LastUpdated: time.Now(),
		}, nil
	}

	ds := NewDiffSession(dir, WithPrFetcher(fetcher))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ds.StartPolling(ctx)

	// Wait for the initial poll to complete.
	require.Eventually(t, func() bool {
		return ds.Poller.State() != nil
	}, 2*time.Second, 10*time.Millisecond)

	st := ds.Poller.State()
	assert.Equal(t, 99, st.Number)
	assert.Equal(t, diff.PrApproved, st.ReviewState)

	ds.StopPolling()
}

// TestDiffSession_NoChanges verifies that Summary reports no changes on a
// clean working tree.
func TestDiffSession_NoChanges(t *testing.T) {
	dir := initTestRepo(t)

	ds := NewDiffSession(dir)
	summary := ds.Summary()
	assert.Equal(t, "no uncommitted changes", summary)
}
