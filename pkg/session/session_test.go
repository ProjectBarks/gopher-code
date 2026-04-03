package session

import (
	"math"
	"testing"

	"github.com/projectbarks/gopher-code/pkg/provider"
)

func approxEqual(a, b, tolerance float64) bool {
	return math.Abs(a-b) < tolerance
}

func TestNew_InitializesNewFields(t *testing.T) {
	s := New(DefaultConfig(), "/tmp/test")

	if s.OriginalCWD != "/tmp/test" {
		t.Errorf("OriginalCWD = %q, want %q", s.OriginalCWD, "/tmp/test")
	}
	if s.ProjectRoot != "/tmp/test" {
		t.Errorf("ProjectRoot = %q, want %q", s.ProjectRoot, "/tmp/test")
	}
	if s.ModelUsage == nil {
		t.Error("ModelUsage should be initialized (non-nil)")
	}
	if s.TotalCostUSD != 0 {
		t.Errorf("TotalCostUSD = %f, want 0", s.TotalCostUSD)
	}
	if s.ParentSessionID != "" {
		t.Errorf("ParentSessionID = %q, want empty", s.ParentSessionID)
	}
	if s.IsInteractive {
		t.Error("IsInteractive should default to false")
	}
}

func TestAddCost(t *testing.T) {
	s := New(DefaultConfig(), "/tmp/test")

	usage1 := provider.TokenUsage{
		InputTokens:  1000,
		OutputTokens: 500,
	}
	s.AddCost("claude-sonnet-4-6", 0.01, usage1)

	if s.TotalCostUSD != 0.01 {
		t.Errorf("TotalCostUSD = %f, want 0.01", s.TotalCostUSD)
	}

	entry, ok := s.ModelUsage["claude-sonnet-4-6"]
	if !ok {
		t.Fatal("ModelUsage missing entry for claude-sonnet-4-6")
	}
	if entry.InputTokens != 1000 {
		t.Errorf("ModelUsage.InputTokens = %d, want 1000", entry.InputTokens)
	}
	if entry.OutputTokens != 500 {
		t.Errorf("ModelUsage.OutputTokens = %d, want 500", entry.OutputTokens)
	}
	if entry.CostUSD != 0.01 {
		t.Errorf("ModelUsage.CostUSD = %f, want 0.01", entry.CostUSD)
	}

	// Second call accumulates
	usage2 := provider.TokenUsage{
		InputTokens:  2000,
		OutputTokens: 1000,
	}
	s.AddCost("claude-sonnet-4-6", 0.02, usage2)

	if s.TotalCostUSD != 0.03 {
		t.Errorf("TotalCostUSD = %f, want 0.03", s.TotalCostUSD)
	}
	if entry.InputTokens != 3000 {
		t.Errorf("Accumulated InputTokens = %d, want 3000", entry.InputTokens)
	}
	if entry.CostUSD != 0.03 {
		t.Errorf("Accumulated CostUSD = %f, want 0.03", entry.CostUSD)
	}
}

func TestAddCost_MultipleModels(t *testing.T) {
	s := New(DefaultConfig(), "/tmp/test")

	s.AddCost("claude-sonnet-4-6", 0.01, provider.TokenUsage{InputTokens: 100})
	s.AddCost("claude-opus-4-6", 0.05, provider.TokenUsage{InputTokens: 200})

	if len(s.ModelUsage) != 2 {
		t.Errorf("ModelUsage has %d entries, want 2", len(s.ModelUsage))
	}
	if !approxEqual(s.TotalCostUSD, 0.06, 0.001) {
		t.Errorf("TotalCostUSD = %f, want ~0.06", s.TotalCostUSD)
	}
}

func TestAddLinesChanged(t *testing.T) {
	s := New(DefaultConfig(), "/tmp/test")

	s.AddLinesChanged(50, 10)
	s.AddLinesChanged(30, 20)

	if s.TotalLinesAdded != 80 {
		t.Errorf("TotalLinesAdded = %d, want 80", s.TotalLinesAdded)
	}
	if s.TotalLinesRemoved != 30 {
		t.Errorf("TotalLinesRemoved = %d, want 30", s.TotalLinesRemoved)
	}
}

func TestRegenerateSessionID(t *testing.T) {
	s := New(DefaultConfig(), "/tmp/test")
	originalID := s.ID

	// Without setting parent
	newID := s.RegenerateSessionID(false)
	if newID == originalID {
		t.Error("RegenerateSessionID should produce a new ID")
	}
	if s.ID != newID {
		t.Errorf("s.ID = %q, want %q", s.ID, newID)
	}
	if s.ParentSessionID != "" {
		t.Errorf("ParentSessionID should remain empty, got %q", s.ParentSessionID)
	}

	// With setting parent
	prevID := s.ID
	s.RegenerateSessionID(true)
	if s.ParentSessionID != prevID {
		t.Errorf("ParentSessionID = %q, want %q", s.ParentSessionID, prevID)
	}
	if s.ID == prevID {
		t.Error("ID should change after regeneration")
	}
}

func TestSaveAndLoad_NewFields(t *testing.T) {
	setupTestHome(t)

	s := New(DefaultConfig(), "/tmp/project")
	s.OriginalCWD = "/tmp/original"
	s.ProjectRoot = "/tmp/root"
	s.ParentSessionID = "parent-123"
	s.TotalAPIDuration = 5000
	s.TotalToolDuration = 3000
	s.TotalLinesAdded = 100
	s.TotalLinesRemoved = 50
	s.IsInteractive = true
	s.AddCost("claude-sonnet-4-6", 1.5, provider.TokenUsage{InputTokens: 10000, OutputTokens: 5000})

	if err := s.Save(); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	loaded, err := Load(s.ID)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if loaded.OriginalCWD != "/tmp/original" {
		t.Errorf("OriginalCWD = %q, want /tmp/original", loaded.OriginalCWD)
	}
	if loaded.ProjectRoot != "/tmp/root" {
		t.Errorf("ProjectRoot = %q, want /tmp/root", loaded.ProjectRoot)
	}
	if loaded.ParentSessionID != "parent-123" {
		t.Errorf("ParentSessionID = %q, want parent-123", loaded.ParentSessionID)
	}
	if loaded.TotalCostUSD != 1.5 {
		t.Errorf("TotalCostUSD = %f, want 1.5", loaded.TotalCostUSD)
	}
	if loaded.TotalAPIDuration != 5000 {
		t.Errorf("TotalAPIDuration = %f, want 5000", loaded.TotalAPIDuration)
	}
	if loaded.TotalLinesAdded != 100 {
		t.Errorf("TotalLinesAdded = %d, want 100", loaded.TotalLinesAdded)
	}
	if loaded.TotalLinesRemoved != 50 {
		t.Errorf("TotalLinesRemoved = %d, want 50", loaded.TotalLinesRemoved)
	}
	if !loaded.IsInteractive {
		t.Error("IsInteractive should be true")
	}
	if loaded.ModelUsage == nil {
		t.Fatal("ModelUsage should be non-nil after load")
	}
	entry, ok := loaded.ModelUsage["claude-sonnet-4-6"]
	if !ok {
		t.Fatal("ModelUsage missing claude-sonnet-4-6 after load")
	}
	if entry.InputTokens != 10000 {
		t.Errorf("ModelUsage.InputTokens = %d, want 10000", entry.InputTokens)
	}
}
