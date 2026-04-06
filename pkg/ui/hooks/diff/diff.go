// Package diff provides turn-based diff tracking, git diff computation,
// PR status polling, and IDE diff integration. Ported from the TS hooks
// useDiffData, useTurnDiffs, usePrStatus, and useDiffInIDE.
package diff

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// ---------------------------------------------------------------------------
// Constants matching TS source
// ---------------------------------------------------------------------------

const (
	MaxLinesPerFile    = 400
	MaxFiles           = 50
	MaxFilesForDetails = 500

	prPollInterval    = 60 * time.Second
	prSlowThreshold   = 4 * time.Second
	prIdleStopTimeout = 60 * time.Minute
	ghTimeout         = 5 * time.Second
)

// ---------------------------------------------------------------------------
// DiffFile / DiffData — mirrors useDiffData.ts
// ---------------------------------------------------------------------------

// DiffFile describes a single changed file in the working tree.
type DiffFile struct {
	Path         string
	LinesAdded   int
	LinesRemoved int
	IsBinary     bool
	IsLargeFile  bool
	IsTruncated  bool
	IsNewFile    bool
	IsUntracked  bool
}

// DiffStats holds aggregate counts.
type DiffStats struct {
	FilesCount   int
	LinesAdded   int
	LinesRemoved int
}

// Hunk mirrors the StructuredPatchHunk from the TS `diff` library.
type Hunk struct {
	OldStart int
	OldLines int
	NewStart int
	NewLines int
	Lines    []string
}

// DiffData is the result of computing the current git diff.
type DiffData struct {
	Stats   *DiffStats
	Files   []DiffFile
	Hunks   map[string][]Hunk
	Loading bool
}

// ---------------------------------------------------------------------------
// DiffComputer — computes working-tree diffs via go-git
// ---------------------------------------------------------------------------

// DiffComputer fetches git diff data for a repository at repoPath.
type DiffComputer struct {
	repoPath string
}

// NewDiffComputer creates a DiffComputer for the given repository root.
func NewDiffComputer(repoPath string) *DiffComputer {
	return &DiffComputer{repoPath: repoPath}
}

// Compute returns the current diff data (stats + per-file info + hunks).
// It uses go-git to compare the worktree against HEAD.
func (dc *DiffComputer) Compute() (*DiffData, error) {
	repo, err := git.PlainOpen(dc.repoPath)
	if err != nil {
		return nil, fmt.Errorf("open repo: %w", err)
	}

	// Check for transient git state (merge/rebase/cherry-pick/revert).
	if isTransient, _ := isInTransientGitState(dc.repoPath); isTransient {
		return &DiffData{}, nil
	}

	head, err := repo.Head()
	if err != nil {
		// No HEAD yet (empty repo) — treat everything as added.
		return dc.computeInitialCommit(repo)
	}

	commit, err := repo.CommitObject(head.Hash())
	if err != nil {
		return nil, fmt.Errorf("get HEAD commit: %w", err)
	}

	headTree, err := commit.Tree()
	if err != nil {
		return nil, fmt.Errorf("get HEAD tree: %w", err)
	}

	wt, err := repo.Worktree()
	if err != nil {
		return nil, fmt.Errorf("get worktree: %w", err)
	}

	status, err := wt.Status()
	if err != nil {
		return nil, fmt.Errorf("worktree status: %w", err)
	}

	stats := &DiffStats{}
	files := make([]DiffFile, 0, len(status))
	hunks := make(map[string][]Hunk)

	for path, st := range status {
		if st.Worktree == git.Unmodified && st.Staging == git.Unmodified {
			continue
		}
		if len(files) >= MaxFiles {
			stats.FilesCount++
			continue
		}

		df := DiffFile{Path: path}

		if st.Worktree == git.Untracked {
			df.IsUntracked = true
			df.IsNewFile = true
			files = append(files, df)
			stats.FilesCount++
			continue
		}

		// Compute per-file diff using go-git tree diff.
		added, removed, fileHunks, isBin := dc.diffFileAgainstTree(headTree, path)
		df.LinesAdded = added
		df.LinesRemoved = removed
		df.IsBinary = isBin

		totalLines := added + removed
		df.IsLargeFile = !isBin && totalLines == 0 && !df.IsUntracked && fileHunks == nil
		df.IsTruncated = !df.IsLargeFile && !isBin && totalLines > MaxLinesPerFile

		if fileHunks != nil {
			hunks[path] = fileHunks
		}

		stats.FilesCount++
		stats.LinesAdded += added
		stats.LinesRemoved += removed
		files = append(files, df)
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].Path < files[j].Path
	})

	return &DiffData{Stats: stats, Files: files, Hunks: hunks}, nil
}

// computeInitialCommit handles the case where there is no HEAD commit yet.
func (dc *DiffComputer) computeInitialCommit(repo *git.Repository) (*DiffData, error) {
	wt, err := repo.Worktree()
	if err != nil {
		return nil, err
	}
	status, err := wt.Status()
	if err != nil {
		return nil, err
	}
	stats := &DiffStats{}
	files := make([]DiffFile, 0, len(status))
	for path, st := range status {
		if st.Worktree == git.Unmodified && st.Staging == git.Unmodified {
			continue
		}
		files = append(files, DiffFile{
			Path:        path,
			IsNewFile:   true,
			IsUntracked: st.Worktree == git.Untracked,
		})
		stats.FilesCount++
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	return &DiffData{Stats: stats, Files: files, Hunks: map[string][]Hunk{}}, nil
}

// diffFileAgainstTree computes added/removed line counts and hunks for a
// single file compared to the HEAD tree using go-git.
func (dc *DiffComputer) diffFileAgainstTree(headTree *object.Tree, path string) (added, removed int, hunks []Hunk, isBinary bool) {
	// Read current file from working tree.
	absPath := filepath.Join(dc.repoPath, path)
	newBytes, err := os.ReadFile(absPath)
	if err != nil {
		return 0, 0, nil, false
	}

	// Get old content from HEAD tree.
	var oldContent string
	entry, err := headTree.FindEntry(path)
	if err == nil {
		blob, err2 := headTree.TreeEntryFile(entry)
		if err2 == nil {
			content, err3 := blob.Contents()
			if err3 == nil {
				oldContent = content
			}
		}
	}

	newContent := string(newBytes)

	// Quick binary check.
	if isBinaryContent(newBytes) {
		return 0, 0, nil, true
	}

	// Compute unified diff hunks.
	hunks = computeUnifiedDiffHunks(oldContent, newContent)

	for _, h := range hunks {
		for _, line := range h.Lines {
			if strings.HasPrefix(line, "+") {
				added++
			} else if strings.HasPrefix(line, "-") {
				removed++
			}
		}
	}

	return added, removed, hunks, false
}

// isBinaryContent does a quick heuristic check for NUL bytes.
func isBinaryContent(data []byte) bool {
	limit := 8000
	if len(data) < limit {
		limit = len(data)
	}
	for i := 0; i < limit; i++ {
		if data[i] == 0 {
			return true
		}
	}
	return false
}

// computeUnifiedDiffHunks produces unified-diff-style hunks using a simple
// line-by-line diff (Myers-like via go-git's merkletrie isn't line-level,
// so we do a straightforward LCS-based approach).
func computeUnifiedDiffHunks(oldContent, newContent string) []Hunk {
	oldLines := splitLines(oldContent)
	newLines := splitLines(newContent)

	ops := myersDiff(oldLines, newLines)
	if len(ops) == 0 {
		return nil
	}

	// Group edit operations into hunks with 3 lines of context.
	const contextLines = 3
	return groupIntoHunks(oldLines, newLines, ops, contextLines)
}

// ---------------------------------------------------------------------------
// TurnDiff — mirrors useTurnDiffs.ts
// ---------------------------------------------------------------------------

// TurnFileDiff describes changes to a single file within a turn.
type TurnFileDiff struct {
	FilePath     string
	Hunks        []Hunk
	IsNewFile    bool
	LinesAdded   int
	LinesRemoved int
}

// TurnDiffStats holds per-turn aggregate stats.
type TurnDiffStats struct {
	FilesChanged int
	LinesAdded   int
	LinesRemoved int
}

// TurnDiff captures all file changes in one user turn.
type TurnDiff struct {
	TurnIndex         int
	UserPromptPreview string
	Timestamp         string
	Files             map[string]*TurnFileDiff
	Stats             TurnDiffStats
}

// FileEditResult is the minimal info extracted from a tool result for diff tracking.
type FileEditResult struct {
	FilePath  string
	Hunks     []Hunk
	IsNewFile bool
	Content   string // only set for new-file creates (no hunks)
}

// TurnDiffTracker accumulates per-turn diffs incrementally.
type TurnDiffTracker struct {
	mu                 sync.Mutex
	completedTurns     []*TurnDiff
	currentTurn        *TurnDiff
	lastProcessedIndex int
	lastTurnIndex      int
}

// NewTurnDiffTracker creates an empty tracker.
func NewTurnDiffTracker() *TurnDiffTracker {
	return &TurnDiffTracker{}
}

// StartTurn begins a new turn with the given user prompt preview and timestamp.
// If the current turn has file changes, it is finalized and added to completed.
func (t *TurnDiffTracker) StartTurn(promptPreview, timestamp string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.currentTurn != nil && len(t.currentTurn.Files) > 0 {
		computeTurnStats(t.currentTurn)
		t.completedTurns = append(t.completedTurns, t.currentTurn)
	}

	t.lastTurnIndex++
	t.currentTurn = &TurnDiff{
		TurnIndex:         t.lastTurnIndex,
		UserPromptPreview: truncatePrompt(promptPreview, 30),
		Timestamp:         timestamp,
		Files:             make(map[string]*TurnFileDiff),
	}
}

// RecordEdit records a file edit in the current turn.
func (t *TurnDiffTracker) RecordEdit(result FileEditResult) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.currentTurn == nil {
		return
	}

	entry, ok := t.currentTurn.Files[result.FilePath]
	if !ok {
		entry = &TurnFileDiff{
			FilePath:  result.FilePath,
			IsNewFile: result.IsNewFile,
		}
		t.currentTurn.Files[result.FilePath] = entry
	}

	if result.IsNewFile && len(result.Hunks) == 0 && result.Content != "" {
		// Synthetic hunk for newly created files.
		lines := strings.Split(result.Content, "\n")
		syntheticHunk := Hunk{
			OldStart: 0, OldLines: 0,
			NewStart: 1, NewLines: len(lines),
			Lines: make([]string, len(lines)),
		}
		for i, l := range lines {
			syntheticHunk.Lines[i] = "+" + l
		}
		entry.Hunks = append(entry.Hunks, syntheticHunk)
		entry.LinesAdded += len(lines)
	} else {
		entry.Hunks = append(entry.Hunks, result.Hunks...)
		for _, h := range result.Hunks {
			for _, line := range h.Lines {
				if strings.HasPrefix(line, "+") {
					entry.LinesAdded++
				} else if strings.HasPrefix(line, "-") {
					entry.LinesRemoved++
				}
			}
		}
	}

	if result.IsNewFile {
		entry.IsNewFile = true
	}
}

// Turns returns completed + current turn diffs in reverse chronological order.
func (t *TurnDiffTracker) Turns() []*TurnDiff {
	t.mu.Lock()
	defer t.mu.Unlock()

	result := make([]*TurnDiff, len(t.completedTurns))
	copy(result, t.completedTurns)

	if t.currentTurn != nil && len(t.currentTurn.Files) > 0 {
		snap := *t.currentTurn
		computeTurnStats(&snap)
		result = append(result, &snap)
	}

	// Reverse: most recent first.
	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}
	return result
}

// Reset clears all tracked state (e.g. if messages are rewound).
func (t *TurnDiffTracker) Reset() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.completedTurns = nil
	t.currentTurn = nil
	t.lastProcessedIndex = 0
	t.lastTurnIndex = 0
}

func computeTurnStats(td *TurnDiff) {
	var added, removed int
	for _, f := range td.Files {
		added += f.LinesAdded
		removed += f.LinesRemoved
	}
	td.Stats = TurnDiffStats{
		FilesChanged: len(td.Files),
		LinesAdded:   added,
		LinesRemoved: removed,
	}
}

func truncatePrompt(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "\u2026" // ellipsis
}

// ---------------------------------------------------------------------------
// PrStatus — mirrors usePrStatus.ts
// ---------------------------------------------------------------------------

// PrReviewState maps to the TS PrReviewState union type.
type PrReviewState string

const (
	PrApproved         PrReviewState = "approved"
	PrPending          PrReviewState = "pending"
	PrChangesRequested PrReviewState = "changes_requested"
	PrDraft            PrReviewState = "draft"
	PrMerged           PrReviewState = "merged"
	PrClosed           PrReviewState = "closed"
)

// PrStatusState holds the cached PR status for the current branch.
type PrStatusState struct {
	Number      int           `json:"number"`
	URL         string        `json:"url"`
	ReviewState PrReviewState `json:"reviewState"`
	LastUpdated time.Time     `json:"lastUpdated"`
}

// DeriveReviewState maps GitHub API values to PrReviewState.
// Draft PRs always show as "draft" regardless of reviewDecision.
func DeriveReviewState(isDraft bool, reviewDecision string) PrReviewState {
	if isDraft {
		return PrDraft
	}
	switch reviewDecision {
	case "APPROVED":
		return PrApproved
	case "CHANGES_REQUESTED":
		return PrChangesRequested
	default:
		return PrPending
	}
}

// ghPrViewResult is the JSON shape from `gh pr view --json ...`.
type ghPrViewResult struct {
	Number         int    `json:"number"`
	URL            string `json:"url"`
	ReviewDecision string `json:"reviewDecision"`
	IsDraft        bool   `json:"isDraft"`
	HeadRefName    string `json:"headRefName"`
	State          string `json:"state"`
}

// PrStatusFetcher is the function signature for fetching PR status.
// It can be replaced in tests.
type PrStatusFetcher func(ctx context.Context) (*PrStatusState, error)

// FetchPrStatus runs `gh pr view` and returns the current PR status,
// or nil if there is no open PR for the current branch.
func FetchPrStatus(ctx context.Context) (*PrStatusState, error) {
	// Check if we're in a git repo and not on the default branch.
	branch, defaultBranch, err := detectBranches(ctx)
	if err != nil {
		return nil, nil // not a git repo or error — silently skip
	}
	if branch == defaultBranch {
		return nil, nil
	}

	ctx2, cancel := context.WithTimeout(ctx, ghTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx2, "gh", "pr", "view",
		"--json", "number,url,reviewDecision,isDraft,headRefName,state")
	out, err := cmd.Output()
	if err != nil {
		return nil, nil // gh not installed or no PR
	}
	if len(out) == 0 {
		return nil, nil
	}

	var data ghPrViewResult
	if err := json.Unmarshal(out, &data); err != nil {
		return nil, nil
	}

	// Skip PRs from default branch or merged/closed.
	if data.HeadRefName == defaultBranch ||
		data.HeadRefName == "main" ||
		data.HeadRefName == "master" {
		return nil, nil
	}
	if data.State == "MERGED" || data.State == "CLOSED" {
		return nil, nil
	}

	return &PrStatusState{
		Number:      data.Number,
		URL:         data.URL,
		ReviewState: DeriveReviewState(data.IsDraft, data.ReviewDecision),
		LastUpdated: time.Now(),
	}, nil
}

// PrStatusPoller polls PR status on a timer, caching the result.
type PrStatusPoller struct {
	mu      sync.Mutex
	state   *PrStatusState
	fetcher PrStatusFetcher

	disabled          bool
	lastFetch         time.Time
	lastInteraction   time.Time
	stopCh            chan struct{}
	stopped           bool
}

// NewPrStatusPoller creates a poller that uses the given fetcher.
func NewPrStatusPoller(fetcher PrStatusFetcher) *PrStatusPoller {
	return &PrStatusPoller{
		fetcher:         fetcher,
		lastInteraction: time.Now(),
		stopCh:          make(chan struct{}),
	}
}

// Start begins the polling loop. Call Stop to terminate.
func (p *PrStatusPoller) Start(ctx context.Context) {
	go p.loop(ctx)
}

// Stop terminates the polling loop.
func (p *PrStatusPoller) Stop() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.stopped {
		p.stopped = true
		close(p.stopCh)
	}
}

// State returns the latest cached PR status (may be nil).
func (p *PrStatusPoller) State() *PrStatusState {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.state
}

// RecordInteraction resets the idle timer.
func (p *PrStatusPoller) RecordInteraction() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.lastInteraction = time.Now()
}

func (p *PrStatusPoller) loop(ctx context.Context) {
	// Initial fetch.
	p.poll(ctx)

	ticker := time.NewTicker(prPollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-p.stopCh:
			return
		case <-ticker.C:
			p.mu.Lock()
			disabled := p.disabled
			idle := time.Since(p.lastInteraction)
			p.mu.Unlock()

			if disabled || idle >= prIdleStopTimeout {
				return
			}
			p.poll(ctx)
		}
	}
}

func (p *PrStatusPoller) poll(ctx context.Context) {
	start := time.Now()
	result, err := p.fetcher(ctx)

	p.mu.Lock()
	defer p.mu.Unlock()

	p.lastFetch = start
	if err != nil {
		return
	}
	if result != nil {
		p.state = result
	}

	if time.Since(start) > prSlowThreshold {
		p.disabled = true
	}
}

// ---------------------------------------------------------------------------
// IDEDiff — mirrors useDiffInIDE.ts (struct-only, RPC is out of scope here)
// ---------------------------------------------------------------------------

// IDEDiffRequest contains the parameters needed to open a diff in an IDE.
type IDEDiffRequest struct {
	FilePath    string
	OldContent  string
	NewContent  string
	TabName     string
}

// IDEDiffResult describes the outcome of showing a diff in the IDE.
type IDEDiffResult struct {
	Accepted   bool
	NewContent string
}

// IDEDiffOpener is an interface for opening diffs in an IDE via MCP/RPC.
type IDEDiffOpener interface {
	OpenDiff(ctx context.Context, req IDEDiffRequest) (*IDEDiffResult, error)
	CloseTab(ctx context.Context, tabName string) error
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// isInTransientGitState checks for merge/rebase/cherry-pick/revert markers.
func isInTransientGitState(repoPath string) (bool, error) {
	gitDir := filepath.Join(repoPath, ".git")
	markers := []string{"MERGE_HEAD", "REBASE_HEAD", "CHERRY_PICK_HEAD", "REVERT_HEAD"}
	for _, m := range markers {
		if _, err := os.Stat(filepath.Join(gitDir, m)); err == nil {
			return true, nil
		}
	}
	return false, nil
}

// detectBranches returns the current branch and the default branch name.
func detectBranches(ctx context.Context) (current, defaultBr string, err error) {
	ctx2, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	out, err := exec.CommandContext(ctx2, "git", "rev-parse", "--abbrev-ref", "HEAD").Output()
	if err != nil {
		return "", "", err
	}
	current = strings.TrimSpace(string(out))

	// Try to detect default branch via git remote.
	ctx3, cancel2 := context.WithTimeout(ctx, 3*time.Second)
	defer cancel2()
	out2, err2 := exec.CommandContext(ctx3, "git", "symbolic-ref", "refs/remotes/origin/HEAD").Output()
	if err2 == nil {
		ref := strings.TrimSpace(string(out2))
		if idx := strings.LastIndex(ref, "/"); idx >= 0 {
			defaultBr = ref[idx+1:]
		}
	}
	if defaultBr == "" {
		defaultBr = "main" // fallback
	}
	return current, defaultBr, nil
}

// splitLines splits content into lines, preserving the behavior of strings.Split
// but filtering out a trailing empty string from a final newline.
func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	lines := strings.Split(s, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

// ---------------------------------------------------------------------------
// LCS-based line diff implementation
// ---------------------------------------------------------------------------

type editOp int

const (
	opEqual  editOp = iota
	opInsert
	opDelete
)

type edit struct {
	op   editOp
	oldI int // index in old (for delete/equal)
	newI int // index in new (for insert/equal)
}

// myersDiff computes edit operations between old and new line slices
// using an LCS (longest common subsequence) approach.
func myersDiff(oldLines, newLines []string) []edit {
	n := len(oldLines)
	m := len(newLines)
	if n == 0 && m == 0 {
		return nil
	}

	// Build LCS table.
	dp := make([][]int, n+1)
	for i := range dp {
		dp[i] = make([]int, m+1)
	}
	for i := 1; i <= n; i++ {
		for j := 1; j <= m; j++ {
			if oldLines[i-1] == newLines[j-1] {
				dp[i][j] = dp[i-1][j-1] + 1
			} else if dp[i-1][j] >= dp[i][j-1] {
				dp[i][j] = dp[i-1][j]
			} else {
				dp[i][j] = dp[i][j-1]
			}
		}
	}

	// Backtrack to produce edit script.
	var ops []edit
	i, j := n, m
	for i > 0 || j > 0 {
		if i > 0 && j > 0 && oldLines[i-1] == newLines[j-1] {
			ops = append(ops, edit{op: opEqual, oldI: i - 1, newI: j - 1})
			i--
			j--
		} else if j > 0 && (i == 0 || dp[i][j-1] >= dp[i-1][j]) {
			ops = append(ops, edit{op: opInsert, newI: j - 1})
			j--
		} else {
			ops = append(ops, edit{op: opDelete, oldI: i - 1})
			i--
		}
	}

	// Reverse (we built it backwards).
	for a, b := 0, len(ops)-1; a < b; a, b = a+1, b-1 {
		ops[a], ops[b] = ops[b], ops[a]
	}

	// Check if everything is equal.
	allEqual := true
	for _, op := range ops {
		if op.op != opEqual {
			allEqual = false
			break
		}
	}
	if allEqual {
		return nil
	}

	return ops
}

// groupIntoHunks groups edit operations into unified diff hunks with context.
func groupIntoHunks(oldLines, newLines []string, ops []edit, ctx int) []Hunk {
	// Find ranges of non-equal ops.
	type changeRange struct{ start, end int }
	var changes []changeRange
	inChange := false
	for i, op := range ops {
		if op.op != opEqual {
			if !inChange {
				changes = append(changes, changeRange{start: i})
				inChange = true
			}
			changes[len(changes)-1].end = i + 1
		} else {
			inChange = false
		}
	}

	if len(changes) == 0 {
		return nil
	}

	// Merge nearby changes (within 2*ctx of each other).
	var merged []changeRange
	merged = append(merged, changes[0])
	for i := 1; i < len(changes); i++ {
		prev := &merged[len(merged)-1]
		if changes[i].start-prev.end <= 2*ctx {
			prev.end = changes[i].end
		} else {
			merged = append(merged, changes[i])
		}
	}

	// Build hunks.
	var hunks []Hunk
	for _, cr := range merged {
		start := cr.start - ctx
		if start < 0 {
			start = 0
		}
		end := cr.end + ctx
		if end > len(ops) {
			end = len(ops)
		}

		h := Hunk{}
		// Determine old/new start from first op in range.
		firstOp := ops[start]
		switch firstOp.op {
		case opEqual, opDelete:
			h.OldStart = firstOp.oldI + 1
		default:
			if start > 0 {
				h.OldStart = ops[start-1].oldI + 2
			} else {
				h.OldStart = 1
			}
		}
		switch firstOp.op {
		case opEqual, opInsert:
			h.NewStart = firstOp.newI + 1
		default:
			if start > 0 {
				h.NewStart = ops[start-1].newI + 2
			} else {
				h.NewStart = 1
			}
		}

		for i := start; i < end; i++ {
			op := ops[i]
			switch op.op {
			case opEqual:
				h.Lines = append(h.Lines, " "+oldLines[op.oldI])
				h.OldLines++
				h.NewLines++
			case opDelete:
				h.Lines = append(h.Lines, "-"+oldLines[op.oldI])
				h.OldLines++
			case opInsert:
				h.Lines = append(h.Lines, "+"+newLines[op.newI])
				h.NewLines++
			}
		}
		hunks = append(hunks, h)
	}
	return hunks
}

// ParseShortstat parses `git diff --shortstat` output.
var shortstatRe = regexp.MustCompile(`(\d+)\s+files?\s+changed(?:,\s+(\d+)\s+insertions?\(\+\))?(?:,\s+(\d+)\s+deletions?\(-\))?`)

func ParseShortstat(stdout string) *DiffStats {
	m := shortstatRe.FindStringSubmatch(stdout)
	if m == nil {
		return nil
	}
	files, _ := strconv.Atoi(m[1])
	added, _ := strconv.Atoi(m[2])
	removed, _ := strconv.Atoi(m[3])
	return &DiffStats{FilesCount: files, LinesAdded: added, LinesRemoved: removed}
}
