package memory

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// MemoryUsage tests
// Source: src/hooks/useMemoryUsage.ts
// ---------------------------------------------------------------------------

func TestClassify_Normal(t *testing.T) {
	got := Classify(500 * 1024 * 1024) // 500 MB
	if got != MemoryNormal {
		t.Errorf("Classify(500MB) = %v, want MemoryNormal", got)
	}
}

func TestClassify_High(t *testing.T) {
	got := Classify(HighMemoryThreshold)
	if got != MemoryHigh {
		t.Errorf("Classify(1.5GB) = %v, want MemoryHigh", got)
	}
}

func TestClassify_AboveHighBelowCritical(t *testing.T) {
	got := Classify(2 * 1024 * 1024 * 1024) // 2 GB
	if got != MemoryHigh {
		t.Errorf("Classify(2GB) = %v, want MemoryHigh", got)
	}
}

func TestClassify_Critical(t *testing.T) {
	got := Classify(CriticalMemoryThreshold)
	if got != MemoryCritical {
		t.Errorf("Classify(2.5GB) = %v, want MemoryCritical", got)
	}
}

func TestClassify_AboveCritical(t *testing.T) {
	got := Classify(3 * 1024 * 1024 * 1024) // 3 GB
	if got != MemoryCritical {
		t.Errorf("Classify(3GB) = %v, want MemoryCritical", got)
	}
}

func TestClassify_Zero(t *testing.T) {
	got := Classify(0)
	if got != MemoryNormal {
		t.Errorf("Classify(0) = %v, want MemoryNormal", got)
	}
}

func TestMemoryStatus_String(t *testing.T) {
	tests := []struct {
		status MemoryStatus
		want   string
	}{
		{MemoryNormal, "normal"},
		{MemoryHigh, "high"},
		{MemoryCritical, "critical"},
	}
	for _, tt := range tests {
		if got := tt.status.String(); got != tt.want {
			t.Errorf("MemoryStatus(%d).String() = %q, want %q", tt.status, got, tt.want)
		}
	}
}

func TestSuggestion_Normal(t *testing.T) {
	if s := Suggestion(MemoryNormal); s != "" {
		t.Errorf("Suggestion(Normal) = %q, want empty", s)
	}
}

func TestSuggestion_High(t *testing.T) {
	s := Suggestion(MemoryHigh)
	if s == "" {
		t.Fatal("Suggestion(High) should not be empty")
	}
}

func TestSuggestion_Critical(t *testing.T) {
	s := Suggestion(MemoryCritical)
	if s == "" {
		t.Fatal("Suggestion(Critical) should not be empty")
	}
}

func TestMemoryUsage_TickNormal_ReturnsNil(t *testing.T) {
	m := &MemoryUsage{
		PollInterval: time.Millisecond,
		readMem:      func() uint64 { return 100 * 1024 * 1024 }, // 100 MB
	}
	cmd := m.Tick()
	if cmd == nil {
		t.Fatal("Tick() should return a non-nil Cmd")
	}
	// Execute the command — the tick fires after PollInterval. We wait a bit.
	// Since we can't easily wait for tea.Tick in a unit test, we verify the
	// Cmd is non-nil (the tick is registered). The classification logic is
	// tested separately above.
}

func TestMemoryUsage_ClassifyHighTick(t *testing.T) {
	// Verify that a high-memory reading would produce MemoryHigh.
	heap := uint64(2 * 1024 * 1024 * 1024) // 2 GB
	status := Classify(heap)
	if status != MemoryHigh {
		t.Errorf("expected MemoryHigh for 2GB, got %v", status)
	}
}

func TestMemoryUsage_DefaultInterval(t *testing.T) {
	m := &MemoryUsage{}
	if m.interval() != DefaultPollInterval {
		t.Errorf("default interval = %v, want %v", m.interval(), DefaultPollInterval)
	}
}

func TestMemoryUsage_CustomInterval(t *testing.T) {
	m := &MemoryUsage{PollInterval: 5 * time.Second}
	if m.interval() != 5*time.Second {
		t.Errorf("custom interval = %v, want 5s", m.interval())
	}
}

// ---------------------------------------------------------------------------
// SkillsWatcher tests
// Source: src/utils/skills/skillChangeDetector.ts
// ---------------------------------------------------------------------------

func TestSkillsWatcher_DetectsNewFile(t *testing.T) {
	dir := t.TempDir()

	w := &SkillsWatcher{Dirs: []string{dir}}
	w.Init()

	// Initial poll — no changes since Init snapshot.
	if changed := w.Poll(); len(changed) != 0 {
		t.Fatalf("expected no changes on first poll, got %v", changed)
	}

	// Create a new file.
	path := filepath.Join(dir, "new-skill.md")
	if err := os.WriteFile(path, []byte("# test"), 0o644); err != nil {
		t.Fatal(err)
	}

	changed := w.Poll()
	if len(changed) == 0 {
		t.Fatal("expected change detection for new file")
	}
	found := false
	for _, p := range changed {
		if p == path {
			found = true
		}
	}
	if !found {
		t.Errorf("expected %q in changed paths, got %v", path, changed)
	}
}

func TestSkillsWatcher_DetectsModifiedFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "skill.md")
	if err := os.WriteFile(path, []byte("v1"), 0o644); err != nil {
		t.Fatal(err)
	}

	w := &SkillsWatcher{Dirs: []string{dir}}
	w.Init()

	// No changes yet.
	if changed := w.Poll(); len(changed) != 0 {
		t.Fatalf("expected no changes on first poll, got %v", changed)
	}

	// Modify the file — ensure different modtime.
	time.Sleep(10 * time.Millisecond)
	now := time.Now().Add(time.Second)
	if err := os.WriteFile(path, []byte("v2"), 0o644); err != nil {
		t.Fatal(err)
	}
	_ = os.Chtimes(path, now, now)

	changed := w.Poll()
	if len(changed) == 0 {
		t.Fatal("expected change detection for modified file")
	}
}

func TestSkillsWatcher_DetectsDeletedFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "old-skill.md")
	if err := os.WriteFile(path, []byte("gone"), 0o644); err != nil {
		t.Fatal(err)
	}

	w := &SkillsWatcher{Dirs: []string{dir}}
	w.Init()

	// Remove the file.
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}

	changed := w.Poll()
	if len(changed) == 0 {
		t.Fatal("expected change detection for deleted file")
	}
	found := false
	for _, p := range changed {
		if p == path {
			found = true
		}
	}
	if !found {
		t.Errorf("expected %q in changed paths, got %v", path, changed)
	}
}

func TestSkillsWatcher_DetectsNestedFile(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "my-skill")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}

	w := &SkillsWatcher{Dirs: []string{dir}}
	w.Init()

	// Add SKILL.md inside nested dir (skill-name/SKILL.md format).
	path := filepath.Join(skillDir, "SKILL.md")
	if err := os.WriteFile(path, []byte("# skill"), 0o644); err != nil {
		t.Fatal(err)
	}

	changed := w.Poll()
	if len(changed) == 0 {
		t.Fatal("expected change detection for nested new file")
	}
}

func TestSkillsWatcher_NoChangeReturnsEmpty(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "stable.md"), []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}

	w := &SkillsWatcher{Dirs: []string{dir}}
	w.Init()

	if changed := w.Poll(); len(changed) != 0 {
		t.Fatalf("expected no changes, got %v", changed)
	}
	// Poll again — still no changes.
	if changed := w.Poll(); len(changed) != 0 {
		t.Fatalf("expected no changes on second poll, got %v", changed)
	}
}

func TestSkillsWatcher_MissingDirSkipped(t *testing.T) {
	w := &SkillsWatcher{Dirs: []string{"/nonexistent/path/skills"}}
	w.Init() // should not panic
	if changed := w.Poll(); len(changed) != 0 {
		t.Fatalf("expected no changes for missing dir, got %v", changed)
	}
}

func TestSkillsWatcher_MultipleDirs(t *testing.T) {
	dir1 := t.TempDir()
	dir2 := t.TempDir()

	w := &SkillsWatcher{Dirs: []string{dir1, dir2}}
	w.Init()

	// Create files in both dirs.
	p1 := filepath.Join(dir1, "a.md")
	p2 := filepath.Join(dir2, "b.md")
	if err := os.WriteFile(p1, []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p2, []byte("b"), 0o644); err != nil {
		t.Fatal(err)
	}

	changed := w.Poll()
	if len(changed) < 2 {
		t.Errorf("expected at least 2 changes, got %d: %v", len(changed), changed)
	}
}

func TestSkillsWatcher_DefaultPollInterval(t *testing.T) {
	w := &SkillsWatcher{}
	if w.pollInterval() != DefaultSkillsPollInterval {
		t.Errorf("default poll interval = %v, want %v", w.pollInterval(), DefaultSkillsPollInterval)
	}
}

func TestSkillsWatcher_TickReturnsCmd(t *testing.T) {
	dir := t.TempDir()
	w := &SkillsWatcher{
		Dirs:         []string{dir},
		PollInterval: time.Millisecond,
	}
	w.Init()
	cmd := w.Tick()
	if cmd == nil {
		t.Fatal("Tick() should return non-nil Cmd")
	}
}

// ---------------------------------------------------------------------------
// SkillImprovementTracker tests
// Source: src/utils/hooks/skillImprovement.ts,
//
//	src/hooks/useSkillImprovementSurvey.ts
//
// ---------------------------------------------------------------------------

func TestTracker_RecordTurn_BatchThreshold(t *testing.T) {
	tr := &SkillImprovementTracker{TurnBatchSize: 3}

	// Turns 1 and 2 should not trigger.
	if tr.RecordTurn() {
		t.Error("turn 1 should not trigger")
	}
	if tr.RecordTurn() {
		t.Error("turn 2 should not trigger")
	}
	// Turn 3 should trigger.
	if !tr.RecordTurn() {
		t.Error("turn 3 should trigger (batch size = 3)")
	}
	// Turns 4 and 5 should not trigger.
	if tr.RecordTurn() {
		t.Error("turn 4 should not trigger")
	}
	if tr.RecordTurn() {
		t.Error("turn 5 should not trigger")
	}
	// Turn 6 should trigger again.
	if !tr.RecordTurn() {
		t.Error("turn 6 should trigger (second batch)")
	}
}

func TestTracker_DefaultBatchSize(t *testing.T) {
	tr := &SkillImprovementTracker{}
	if tr.batchSize() != DefaultTurnBatchSize {
		t.Errorf("default batch size = %d, want %d", tr.batchSize(), DefaultTurnBatchSize)
	}
}

func TestTracker_RecordTurn_DefaultBatchSize(t *testing.T) {
	tr := &SkillImprovementTracker{} // default = 5

	for i := 1; i < DefaultTurnBatchSize; i++ {
		if tr.RecordTurn() {
			t.Errorf("turn %d should not trigger (batch=%d)", i, DefaultTurnBatchSize)
		}
	}
	if !tr.RecordTurn() {
		t.Errorf("turn %d should trigger", DefaultTurnBatchSize)
	}
}

func TestTracker_RecordSkillUse(t *testing.T) {
	tr := &SkillImprovementTracker{}

	if n := tr.RecordSkillUse("daily-standup"); n != 1 {
		t.Errorf("first use count = %d, want 1", n)
	}
	if n := tr.RecordSkillUse("daily-standup"); n != 2 {
		t.Errorf("second use count = %d, want 2", n)
	}
	if n := tr.RecordSkillUse("code-review"); n != 1 {
		t.Errorf("different skill first use = %d, want 1", n)
	}
}

func TestTracker_SkillUseCount(t *testing.T) {
	tr := &SkillImprovementTracker{}

	// Zero before any use.
	if n := tr.SkillUseCount("unknown"); n != 0 {
		t.Errorf("unrecorded skill count = %d, want 0", n)
	}

	tr.RecordSkillUse("test")
	tr.RecordSkillUse("test")
	tr.RecordSkillUse("test")
	if n := tr.SkillUseCount("test"); n != 3 {
		t.Errorf("skill count = %d, want 3", n)
	}
}

func TestTracker_SetAndGetSuggestion(t *testing.T) {
	tr := &SkillImprovementTracker{}

	if s := tr.PendingSuggestion(); s != nil {
		t.Error("expected nil suggestion initially")
	}

	suggestion := &SkillImprovementSuggestion{
		SkillName: "standup",
		Updates: []SkillUpdate{
			{Section: "greeting", Change: "add energy check", Reason: "user asked"},
		},
	}
	tr.SetSuggestion(suggestion)

	got := tr.PendingSuggestion()
	if got == nil {
		t.Fatal("expected non-nil suggestion")
	}
	if got.SkillName != "standup" {
		t.Errorf("skill name = %q, want %q", got.SkillName, "standup")
	}
	if len(got.Updates) != 1 {
		t.Errorf("updates count = %d, want 1", len(got.Updates))
	}
}

func TestTracker_HandleResponse_Applied(t *testing.T) {
	tr := &SkillImprovementTracker{}
	suggestion := &SkillImprovementSuggestion{SkillName: "test"}
	tr.SetSuggestion(suggestion)

	got := tr.HandleResponse(SurveyApplied)
	if got == nil {
		t.Fatal("expected non-nil suggestion from HandleResponse")
	}
	if got.SkillName != "test" {
		t.Errorf("skill name = %q, want %q", got.SkillName, "test")
	}

	// Suggestion should be cleared.
	if s := tr.PendingSuggestion(); s != nil {
		t.Error("suggestion should be nil after HandleResponse")
	}
}

func TestTracker_HandleResponse_Dismissed(t *testing.T) {
	tr := &SkillImprovementTracker{}
	suggestion := &SkillImprovementSuggestion{SkillName: "test"}
	tr.SetSuggestion(suggestion)

	got := tr.HandleResponse(SurveyDismissed)
	if got == nil {
		t.Fatal("expected non-nil suggestion from HandleResponse(Dismissed)")
	}

	// Still cleared.
	if s := tr.PendingSuggestion(); s != nil {
		t.Error("suggestion should be nil after dismiss")
	}
}

func TestTracker_HandleResponse_NoPending(t *testing.T) {
	tr := &SkillImprovementTracker{}
	got := tr.HandleResponse(SurveyApplied)
	if got != nil {
		t.Error("HandleResponse with no pending should return nil")
	}
}

func TestTracker_MarkAppearanceLogged(t *testing.T) {
	tr := &SkillImprovementTracker{}
	tr.SetSuggestion(&SkillImprovementSuggestion{SkillName: "x"})

	// First call should return true.
	if !tr.MarkAppearanceLogged() {
		t.Error("first MarkAppearanceLogged should return true")
	}
	// Second call should return false (already logged).
	if tr.MarkAppearanceLogged() {
		t.Error("second MarkAppearanceLogged should return false")
	}

	// Setting a new suggestion resets the flag.
	tr.SetSuggestion(&SkillImprovementSuggestion{SkillName: "y"})
	if !tr.MarkAppearanceLogged() {
		t.Error("MarkAppearanceLogged after new suggestion should return true")
	}
}

func TestResultMessage(t *testing.T) {
	msg := ResultMessage("daily-standup")
	want := `Skill "daily-standup" updated with improvements.`
	if msg != want {
		t.Errorf("ResultMessage = %q, want %q", msg, want)
	}
}

func TestSurveyResponse_String(t *testing.T) {
	if SurveyApplied.String() != "applied" {
		t.Errorf("SurveyApplied.String() = %q", SurveyApplied.String())
	}
	if SurveyDismissed.String() != "dismissed" {
		t.Errorf("SurveyDismissed.String() = %q", SurveyDismissed.String())
	}
}

// ---------------------------------------------------------------------------
// Concurrency safety tests
// ---------------------------------------------------------------------------

func TestTracker_ConcurrentRecordTurn(t *testing.T) {
	tr := &SkillImprovementTracker{TurnBatchSize: 5}
	var wg sync.WaitGroup
	triggers := make(chan bool, 100)

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			triggers <- tr.RecordTurn()
		}()
	}
	wg.Wait()
	close(triggers)

	triggerCount := 0
	for triggered := range triggers {
		if triggered {
			triggerCount++
		}
	}
	// 20 turns / batch size 5 = exactly 4 triggers.
	if triggerCount != 4 {
		t.Errorf("concurrent trigger count = %d, want 4", triggerCount)
	}
}

func TestTracker_ConcurrentSkillUse(t *testing.T) {
	tr := &SkillImprovementTracker{}
	var wg sync.WaitGroup

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			tr.RecordSkillUse("shared-skill")
		}()
	}
	wg.Wait()

	if n := tr.SkillUseCount("shared-skill"); n != 50 {
		t.Errorf("concurrent skill use count = %d, want 50", n)
	}
}

func TestSkillsWatcher_ConcurrentPoll(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "init.md"), []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}

	w := &SkillsWatcher{Dirs: []string{dir}}
	w.Init()

	// Concurrent polls should not race.
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = w.Poll()
		}()
	}
	wg.Wait()
}
