package diff

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// DiffComputer — test diff computation with a real git repo
// ---------------------------------------------------------------------------

func initTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	repo, err := git.PlainInit(dir, false)
	require.NoError(t, err)

	// Write and commit an initial file.
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

func TestDiffComputer_DetectsModifiedFile(t *testing.T) {
	dir := initTestRepo(t)

	// Modify the committed file.
	fp := filepath.Join(dir, "hello.txt")
	require.NoError(t, os.WriteFile(fp, []byte("line1\nchanged\nline3\n"), 0644))

	dc := NewDiffComputer(dir)
	data, err := dc.Compute()
	require.NoError(t, err)
	require.NotNil(t, data.Stats)

	assert.GreaterOrEqual(t, data.Stats.FilesCount, 1)
	assert.GreaterOrEqual(t, data.Stats.LinesAdded, 1)
	assert.GreaterOrEqual(t, data.Stats.LinesRemoved, 1)

	// Should have at least one file.
	require.NotEmpty(t, data.Files)
	found := false
	for _, f := range data.Files {
		if f.Path == "hello.txt" {
			found = true
			assert.Greater(t, f.LinesAdded, 0)
			assert.Greater(t, f.LinesRemoved, 0)
			assert.False(t, f.IsBinary)
		}
	}
	assert.True(t, found, "hello.txt should appear in diff files")

	// Should have hunks for hello.txt.
	hunks, ok := data.Hunks["hello.txt"]
	assert.True(t, ok, "hunks should exist for hello.txt")
	assert.NotEmpty(t, hunks)
}

func TestDiffComputer_DetectsNewUntrackedFile(t *testing.T) {
	dir := initTestRepo(t)

	// Add an untracked file.
	fp := filepath.Join(dir, "newfile.txt")
	require.NoError(t, os.WriteFile(fp, []byte("brand new\n"), 0644))

	dc := NewDiffComputer(dir)
	data, err := dc.Compute()
	require.NoError(t, err)

	found := false
	for _, f := range data.Files {
		if f.Path == "newfile.txt" {
			found = true
			assert.True(t, f.IsUntracked)
			assert.True(t, f.IsNewFile)
		}
	}
	assert.True(t, found, "newfile.txt should appear as untracked")
}

func TestDiffComputer_SkipsDuringTransientGitState(t *testing.T) {
	dir := initTestRepo(t)

	// Simulate a merge in progress.
	gitDir := filepath.Join(dir, ".git")
	require.NoError(t, os.WriteFile(filepath.Join(gitDir, "MERGE_HEAD"), []byte("abc123"), 0644))

	dc := NewDiffComputer(dir)
	data, err := dc.Compute()
	require.NoError(t, err)
	// Should return empty data, not an error.
	assert.Nil(t, data.Stats)
	assert.Empty(t, data.Files)
}

// ---------------------------------------------------------------------------
// TurnDiffTracker — test turn diff accumulation
// ---------------------------------------------------------------------------

func TestTurnDiffTracker_AccumulatesEdits(t *testing.T) {
	tracker := NewTurnDiffTracker()

	// Start first turn.
	tracker.StartTurn("fix the bug", "2025-01-01T00:00:00Z")
	tracker.RecordEdit(FileEditResult{
		FilePath: "main.go",
		Hunks: []Hunk{
			{Lines: []string{"+added line", "-removed line", " context"}},
		},
	})
	tracker.RecordEdit(FileEditResult{
		FilePath: "main.go",
		Hunks: []Hunk{
			{Lines: []string{"+another add"}},
		},
	})

	turns := tracker.Turns()
	require.Len(t, turns, 1)
	assert.Equal(t, 1, turns[0].TurnIndex)
	assert.Equal(t, "fix the bug", turns[0].UserPromptPreview)

	fileDiff, ok := turns[0].Files["main.go"]
	require.True(t, ok)
	assert.Equal(t, 2, fileDiff.LinesAdded)   // "+added line" + "+another add"
	assert.Equal(t, 1, fileDiff.LinesRemoved)  // "-removed line"
	assert.Len(t, fileDiff.Hunks, 2)

	assert.Equal(t, 1, turns[0].Stats.FilesChanged)
	assert.Equal(t, 2, turns[0].Stats.LinesAdded)
	assert.Equal(t, 1, turns[0].Stats.LinesRemoved)
}

func TestTurnDiffTracker_MultipleTurnsReverseOrder(t *testing.T) {
	tracker := NewTurnDiffTracker()

	tracker.StartTurn("turn 1", "t1")
	tracker.RecordEdit(FileEditResult{
		FilePath: "a.go",
		Hunks:    []Hunk{{Lines: []string{"+line"}}},
	})

	tracker.StartTurn("turn 2", "t2")
	tracker.RecordEdit(FileEditResult{
		FilePath: "b.go",
		Hunks:    []Hunk{{Lines: []string{"+line", "+line2"}}},
	})

	turns := tracker.Turns()
	require.Len(t, turns, 2)
	// Most recent first.
	assert.Equal(t, 2, turns[0].TurnIndex)
	assert.Equal(t, 1, turns[1].TurnIndex)
}

func TestTurnDiffTracker_NewFileCreation(t *testing.T) {
	tracker := NewTurnDiffTracker()
	tracker.StartTurn("create file", "t0")
	tracker.RecordEdit(FileEditResult{
		FilePath:  "new.go",
		IsNewFile: true,
		Content:   "package main\n\nfunc main() {}",
	})

	turns := tracker.Turns()
	require.Len(t, turns, 1)

	fileDiff := turns[0].Files["new.go"]
	require.NotNil(t, fileDiff)
	assert.True(t, fileDiff.IsNewFile)
	assert.Equal(t, 3, fileDiff.LinesAdded) // 3 lines from content split
	require.Len(t, fileDiff.Hunks, 1)
	assert.Equal(t, 3, fileDiff.Hunks[0].NewLines)
}

func TestTurnDiffTracker_Reset(t *testing.T) {
	tracker := NewTurnDiffTracker()
	tracker.StartTurn("x", "t")
	tracker.RecordEdit(FileEditResult{
		FilePath: "a.go",
		Hunks:    []Hunk{{Lines: []string{"+line"}}},
	})
	require.Len(t, tracker.Turns(), 1)

	tracker.Reset()
	assert.Empty(t, tracker.Turns())
}

func TestTurnDiffTracker_EmptyTurnsExcluded(t *testing.T) {
	tracker := NewTurnDiffTracker()
	tracker.StartTurn("no edits", "t0")
	// No RecordEdit — this turn should not appear.
	assert.Empty(t, tracker.Turns())
}

// ---------------------------------------------------------------------------
// DeriveReviewState
// ---------------------------------------------------------------------------

func TestDeriveReviewState(t *testing.T) {
	tests := []struct {
		isDraft        bool
		reviewDecision string
		want           PrReviewState
	}{
		{true, "", PrDraft},
		{true, "APPROVED", PrDraft}, // draft always wins
		{false, "APPROVED", PrApproved},
		{false, "CHANGES_REQUESTED", PrChangesRequested},
		{false, "REVIEW_REQUIRED", PrPending},
		{false, "", PrPending},
	}
	for _, tt := range tests {
		got := DeriveReviewState(tt.isDraft, tt.reviewDecision)
		assert.Equal(t, tt.want, got, "isDraft=%v decision=%q", tt.isDraft, tt.reviewDecision)
	}
}

// ---------------------------------------------------------------------------
// PrStatusPoller — test caching with mock fetcher
// ---------------------------------------------------------------------------

func TestPrStatusPoller_CachesResult(t *testing.T) {
	calls := 0
	fetcher := func(ctx context.Context) (*PrStatusState, error) {
		calls++
		return &PrStatusState{
			Number:      42,
			URL:         "https://github.com/org/repo/pull/42",
			ReviewState: PrPending,
			LastUpdated: time.Now(),
		}, nil
	}

	poller := NewPrStatusPoller(fetcher)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	poller.Start(ctx)

	// Give the initial poll time to complete.
	require.Eventually(t, func() bool {
		return poller.State() != nil
	}, 2*time.Second, 10*time.Millisecond)

	st := poller.State()
	assert.Equal(t, 42, st.Number)
	assert.Equal(t, "https://github.com/org/repo/pull/42", st.URL)
	assert.Equal(t, PrPending, st.ReviewState)
	assert.GreaterOrEqual(t, calls, 1)

	poller.Stop()
}

// ---------------------------------------------------------------------------
// ParseShortstat
// ---------------------------------------------------------------------------

func TestParseShortstat(t *testing.T) {
	tests := []struct {
		input string
		want  *DiffStats
	}{
		{
			" 3 files changed, 10 insertions(+), 5 deletions(-)\n",
			&DiffStats{FilesCount: 3, LinesAdded: 10, LinesRemoved: 5},
		},
		{
			" 1 file changed, 2 insertions(+)\n",
			&DiffStats{FilesCount: 1, LinesAdded: 2, LinesRemoved: 0},
		},
		{
			" 1 file changed, 3 deletions(-)\n",
			&DiffStats{FilesCount: 1, LinesAdded: 0, LinesRemoved: 3},
		},
		{"no match here", nil},
	}
	for _, tt := range tests {
		got := ParseShortstat(tt.input)
		assert.Equal(t, tt.want, got, "input=%q", tt.input)
	}
}

// ---------------------------------------------------------------------------
// Unified diff hunk computation
// ---------------------------------------------------------------------------

func TestComputeUnifiedDiffHunks_BasicChange(t *testing.T) {
	old := "line1\nline2\nline3\n"
	new := "line1\nchanged\nline3\n"

	hunks := computeUnifiedDiffHunks(old, new)
	require.NotEmpty(t, hunks)

	// Should contain both a deletion and an addition.
	var hasAdd, hasDel bool
	for _, h := range hunks {
		for _, l := range h.Lines {
			if l == "+changed" {
				hasAdd = true
			}
			if l == "-line2" {
				hasDel = true
			}
		}
	}
	assert.True(t, hasAdd, "should have +changed")
	assert.True(t, hasDel, "should have -line2")
}

func TestComputeUnifiedDiffHunks_NoChange(t *testing.T) {
	content := "same\ncontent\n"
	hunks := computeUnifiedDiffHunks(content, content)
	assert.Empty(t, hunks)
}

func TestComputeUnifiedDiffHunks_AllNew(t *testing.T) {
	hunks := computeUnifiedDiffHunks("", "new\nlines\n")
	require.NotEmpty(t, hunks)
	addCount := 0
	for _, h := range hunks {
		for _, l := range h.Lines {
			if l[0] == '+' {
				addCount++
			}
		}
	}
	assert.Equal(t, 2, addCount)
}

// ---------------------------------------------------------------------------
// Prompt truncation
// ---------------------------------------------------------------------------

func TestTruncatePrompt(t *testing.T) {
	assert.Equal(t, "short", truncatePrompt("short", 30))
	long := "this is a very long prompt that exceeds thirty characters"
	result := truncatePrompt(long, 30)
	// 29 ASCII chars + 3-byte UTF-8 ellipsis = 32 bytes, 30 runes.
	assert.Equal(t, 32, len(result)) // byte length
	runes := []rune(result)
	assert.Equal(t, 30, len(runes)) // rune length
}

// ---------------------------------------------------------------------------
// IDEDiffRequest / IDEDiffResult struct smoke test
// ---------------------------------------------------------------------------

func TestIDEDiffRequestFields(t *testing.T) {
	req := IDEDiffRequest{
		FilePath:   "/tmp/test.go",
		OldContent: "old",
		NewContent: "new",
		TabName:    "test-tab",
	}
	assert.Equal(t, "/tmp/test.go", req.FilePath)
	assert.Equal(t, "old", req.OldContent)
	assert.Equal(t, "new", req.NewContent)
	assert.Equal(t, "test-tab", req.TabName)
}
