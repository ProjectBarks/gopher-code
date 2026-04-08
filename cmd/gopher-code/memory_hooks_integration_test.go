package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	memoryhooks "github.com/projectbarks/gopher-code/pkg/ui/hooks/memory"
)

// TestMemoryHooks_WiredIntoBinary verifies that the memory/skills hook types
// from pkg/ui/hooks/memory are reachable from the binary and function correctly
// when wired the same way main.go constructs them.
func TestMemoryHooks_WiredIntoBinary(t *testing.T) {
	// MemoryUsage — same construction as main.go
	memoryMonitor := &memoryhooks.MemoryUsage{}
	cmd := memoryMonitor.Tick()
	if cmd == nil {
		t.Fatal("MemoryUsage.Tick() should return a non-nil Cmd")
	}

	// Verify classification works through the wired path.
	status := memoryhooks.Classify(500 * 1024 * 1024) // 500 MB — normal
	if status != memoryhooks.MemoryNormal {
		t.Errorf("Classify(500MB) = %v, want MemoryNormal", status)
	}
	status = memoryhooks.Classify(memoryhooks.CriticalMemoryThreshold)
	if status != memoryhooks.MemoryCritical {
		t.Errorf("Classify(critical) = %v, want MemoryCritical", status)
	}
	if s := memoryhooks.Suggestion(memoryhooks.MemoryCritical); s == "" {
		t.Error("Suggestion(Critical) should not be empty")
	}
}

// TestSkillsWatcher_WiredIntoBinary verifies that SkillsWatcher works through
// the same construction path used in main.go (project + home skills dirs).
func TestSkillsWatcher_WiredIntoBinary(t *testing.T) {
	// Create a fake project skills directory (same logic as main.go).
	projectRoot := t.TempDir()
	skillsDir := filepath.Join(projectRoot, ".claude", "skills")
	if err := os.MkdirAll(skillsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	watcher := &memoryhooks.SkillsWatcher{
		Dirs:         []string{skillsDir},
		PollInterval: time.Millisecond,
	}
	watcher.Init()

	// No changes initially.
	if changed := watcher.Poll(); len(changed) != 0 {
		t.Fatalf("expected no changes on first poll, got %v", changed)
	}

	// Add a skill file — watcher should detect it.
	skillFile := filepath.Join(skillsDir, "test-skill.md")
	if err := os.WriteFile(skillFile, []byte("# Test Skill"), 0o644); err != nil {
		t.Fatal(err)
	}

	changed := watcher.Poll()
	if len(changed) == 0 {
		t.Fatal("expected SkillsWatcher to detect new skill file")
	}
	found := false
	for _, p := range changed {
		if p == skillFile {
			found = true
		}
	}
	if !found {
		t.Errorf("expected %q in changed paths, got %v", skillFile, changed)
	}

	// Tick returns a Cmd.
	cmd := watcher.Tick()
	if cmd == nil {
		t.Fatal("SkillsWatcher.Tick() should return a non-nil Cmd")
	}
}

// TestSkillImprovementTracker_WiredIntoBinary verifies that the skill
// improvement tracker works through the same construction path used in main.go.
func TestSkillImprovementTracker_WiredIntoBinary(t *testing.T) {
	tracker := &memoryhooks.SkillImprovementTracker{}

	// Record turns — batch threshold at default (5).
	for i := 0; i < 4; i++ {
		if tracker.RecordTurn() {
			t.Errorf("turn %d should not trigger", i+1)
		}
	}
	if !tracker.RecordTurn() {
		t.Error("turn 5 should trigger (default batch size)")
	}

	// Record skill use.
	if n := tracker.RecordSkillUse("test-skill"); n != 1 {
		t.Errorf("first use = %d, want 1", n)
	}

	// Set and retrieve suggestion.
	suggestion := &memoryhooks.SkillImprovementSuggestion{
		SkillName: "test-skill",
		Updates: []memoryhooks.SkillUpdate{
			{Section: "intro", Change: "add greeting", Reason: "user preference"},
		},
	}
	tracker.SetSuggestion(suggestion)
	if got := tracker.PendingSuggestion(); got == nil || got.SkillName != "test-skill" {
		t.Error("PendingSuggestion should return the set suggestion")
	}

	// Handle response clears suggestion.
	handled := tracker.HandleResponse(memoryhooks.SurveyApplied)
	if handled == nil {
		t.Fatal("HandleResponse should return the suggestion")
	}
	if tracker.PendingSuggestion() != nil {
		t.Error("suggestion should be nil after HandleResponse")
	}

	// ResultMessage produces expected string.
	msg := memoryhooks.ResultMessage("test-skill")
	want := `Skill "test-skill" updated with improvements.`
	if msg != want {
		t.Errorf("ResultMessage = %q, want %q", msg, want)
	}
}
