package session

import (
	"math"
	"strings"
	"testing"
	"time"

	"github.com/projectbarks/gopher-code/pkg/message"
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

func TestFirstUserPreview(t *testing.T) {
	// Source: ResumeConversation.tsx — first user message preview

	t.Run("extracts_first_user_text", func(t *testing.T) {
		s := New(DefaultConfig(), "/tmp")
		s.PushMessage(message.Message{
			Role:    message.RoleAssistant,
			Content: []message.ContentBlock{message.TextBlock("hello")},
		})
		s.PushMessage(message.UserMessage("fix the bug in main.go"))
		s.PushMessage(message.UserMessage("second question"))

		got := s.FirstUserPreview()
		if got != "fix the bug in main.go" {
			t.Errorf("FirstUserPreview() = %q, want %q", got, "fix the bug in main.go")
		}
	})

	t.Run("truncates_long_messages", func(t *testing.T) {
		s := New(DefaultConfig(), "/tmp")
		long := strings.Repeat("x", 200)
		s.PushMessage(message.UserMessage(long))

		got := s.FirstUserPreview()
		if len(got) > PreviewMaxLen {
			t.Errorf("preview len = %d, want <= %d", len(got), PreviewMaxLen)
		}
		if !strings.HasSuffix(got, "...") {
			t.Errorf("truncated preview should end with ..., got %q", got[len(got)-10:])
		}
	})

	t.Run("empty_when_no_user_messages", func(t *testing.T) {
		s := New(DefaultConfig(), "/tmp")
		if got := s.FirstUserPreview(); got != "" {
			t.Errorf("FirstUserPreview() = %q, want empty", got)
		}
	})
}

func TestSavePreservesPreview(t *testing.T) {
	setupTestHome(t)

	s := New(DefaultConfig(), "/tmp/project")
	s.Name = "test session"
	s.PushMessage(message.UserMessage("what is the meaning of life"))

	if err := s.Save(); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	metas, err := ListSessions()
	if err != nil {
		t.Fatalf("ListSessions() error: %v", err)
	}
	if len(metas) != 1 {
		t.Fatalf("expected 1 session, got %d", len(metas))
	}
	if metas[0].Preview != "what is the meaning of life" {
		t.Errorf("Preview = %q, want %q", metas[0].Preview, "what is the meaning of life")
	}
	if metas[0].Name != "test session" {
		t.Errorf("Name = %q, want %q", metas[0].Name, "test session")
	}
}

// T108: totalAPIDurationWithoutRetries
func TestAPIDurationWithoutRetries(t *testing.T) {
	s := New(DefaultConfig(), "/tmp/test")
	if s.TotalAPIDurationWithoutRetries != 0 {
		t.Errorf("initial TotalAPIDurationWithoutRetries = %f, want 0", s.TotalAPIDurationWithoutRetries)
	}
	s.AddAPIDurationWithoutRetries(100.5)
	s.AddAPIDurationWithoutRetries(200.3)
	if !approxEqual(s.TotalAPIDurationWithoutRetries, 300.8, 0.01) {
		t.Errorf("TotalAPIDurationWithoutRetries = %f, want ~300.8", s.TotalAPIDurationWithoutRetries)
	}
	s.ResetAPIDurationWithoutRetries()
	if s.TotalAPIDurationWithoutRetries != 0 {
		t.Errorf("after reset TotalAPIDurationWithoutRetries = %f, want 0", s.TotalAPIDurationWithoutRetries)
	}
}

// T109: turnHookDurationMs / turnToolDurationMs / turnClassifierDurationMs
func TestTurnDurationMetrics(t *testing.T) {
	s := New(DefaultConfig(), "/tmp/test")

	s.AddTurnToolDuration(50)
	s.AddTurnToolDuration(30)
	if !approxEqual(s.TurnToolDurationMs, 80, 0.01) {
		t.Errorf("TurnToolDurationMs = %f, want 80", s.TurnToolDurationMs)
	}
	if s.TurnToolCount != 2 {
		t.Errorf("TurnToolCount = %d, want 2", s.TurnToolCount)
	}
	s.ResetTurnToolMetrics()
	if s.TurnToolDurationMs != 0 || s.TurnToolCount != 0 {
		t.Error("ResetTurnToolMetrics should zero both fields")
	}

	s.AddTurnHookDuration(10)
	s.AddTurnHookDuration(20)
	s.AddTurnHookDuration(30)
	if !approxEqual(s.TurnHookDurationMs, 60, 0.01) {
		t.Errorf("TurnHookDurationMs = %f, want 60", s.TurnHookDurationMs)
	}
	if s.TurnHookCount != 3 {
		t.Errorf("TurnHookCount = %d, want 3", s.TurnHookCount)
	}
	s.ResetTurnHookMetrics()
	if s.TurnHookDurationMs != 0 || s.TurnHookCount != 0 {
		t.Error("ResetTurnHookMetrics should zero both fields")
	}

	s.AddTurnClassifierDuration(5)
	if !approxEqual(s.TurnClassifierDurationMs, 5, 0.01) {
		t.Errorf("TurnClassifierDurationMs = %f, want 5", s.TurnClassifierDurationMs)
	}
	if s.TurnClassifierCount != 1 {
		t.Errorf("TurnClassifierCount = %d, want 1", s.TurnClassifierCount)
	}
	s.ResetTurnClassifierMetrics()
	if s.TurnClassifierDurationMs != 0 || s.TurnClassifierCount != 0 {
		t.Error("ResetTurnClassifierMetrics should zero both fields")
	}
}

// T110: turnToolCount / turnHookCount / turnClassifierCount — tested above in TestTurnDurationMetrics

// T111: startTime is set on creation
func TestStartTime(t *testing.T) {
	before := time.Now()
	s := New(DefaultConfig(), "/tmp/test")
	after := time.Now()

	if s.StartTime.Before(before) || s.StartTime.After(after) {
		t.Errorf("StartTime = %v, want between %v and %v", s.StartTime, before, after)
	}
	if s.StartTime.IsZero() {
		t.Error("StartTime should not be zero")
	}
}

// T112: lastInteractionTime + updateLastInteractionTime + flushInteractionTime
func TestInteractionTime(t *testing.T) {
	s := New(DefaultConfig(), "/tmp/test")
	initial := s.LastInteractionTime

	if initial.IsZero() {
		t.Fatal("LastInteractionTime should be set on creation")
	}

	// Deferred update: dirty flag set but timestamp unchanged
	s.UpdateLastInteractionTime(false)
	if s.LastInteractionTime != initial {
		t.Error("deferred UpdateLastInteractionTime should not change timestamp immediately")
	}

	// Flush applies the deferred update
	time.Sleep(time.Millisecond) // ensure clock moves
	s.FlushInteractionTime()
	if !s.LastInteractionTime.After(initial) {
		t.Error("FlushInteractionTime should update the timestamp")
	}

	// Flush with no dirty flag is a no-op
	afterFlush := s.LastInteractionTime
	s.FlushInteractionTime()
	if s.LastInteractionTime != afterFlush {
		t.Error("FlushInteractionTime without dirty flag should be a no-op")
	}

	// Immediate update
	time.Sleep(time.Millisecond)
	s.UpdateLastInteractionTime(true)
	if !s.LastInteractionTime.After(afterFlush) {
		t.Error("immediate UpdateLastInteractionTime should update the timestamp")
	}
}

// T113: hasUnknownModelCost
func TestHasUnknownModelCost(t *testing.T) {
	s := New(DefaultConfig(), "/tmp/test")
	if s.HasUnknownModelCost {
		t.Error("HasUnknownModelCost should default to false")
	}
	s.HasUnknownModelCost = true
	if !s.HasUnknownModelCost {
		t.Error("HasUnknownModelCost should be settable to true")
	}
}

// T114: mainLoopModelOverride / initialMainLoopModel
func TestModelOverride(t *testing.T) {
	s := New(DefaultConfig(), "/tmp/test")
	if s.MainLoopModelOverride != "" {
		t.Errorf("MainLoopModelOverride = %q, want empty", s.MainLoopModelOverride)
	}
	if s.InitialMainLoopModel != "" {
		t.Errorf("InitialMainLoopModel = %q, want empty", s.InitialMainLoopModel)
	}

	s.MainLoopModelOverride = "claude-opus-4-6"
	s.InitialMainLoopModel = "claude-sonnet-4-6"

	if s.MainLoopModelOverride != "claude-opus-4-6" {
		t.Errorf("MainLoopModelOverride = %q, want claude-opus-4-6", s.MainLoopModelOverride)
	}
	if s.InitialMainLoopModel != "claude-sonnet-4-6" {
		t.Errorf("InitialMainLoopModel = %q, want claude-sonnet-4-6", s.InitialMainLoopModel)
	}
}

// T116: kairosActive flag
func TestKairosActive(t *testing.T) {
	s := New(DefaultConfig(), "/tmp/test")
	if s.KairosActive {
		t.Error("KairosActive should default to false")
	}
	s.KairosActive = true
	if !s.KairosActive {
		t.Error("KairosActive should be settable to true")
	}
}

// T117: strictToolResultPairing flag
func TestStrictToolResultPairing(t *testing.T) {
	s := New(DefaultConfig(), "/tmp/test")
	if s.StrictToolResultPairing {
		t.Error("StrictToolResultPairing should default to false")
	}
	s.StrictToolResultPairing = true
	if !s.StrictToolResultPairing {
		t.Error("StrictToolResultPairing should be settable to true")
	}
}

// T118: sdkAgentProgressSummariesEnabled
func TestSDKAgentProgressSummariesEnabled(t *testing.T) {
	s := New(DefaultConfig(), "/tmp/test")
	if s.SDKAgentProgressSummariesEnabled {
		t.Error("SDKAgentProgressSummariesEnabled should default to false")
	}
	s.SDKAgentProgressSummariesEnabled = true
	if !s.SDKAgentProgressSummariesEnabled {
		t.Error("SDKAgentProgressSummariesEnabled should be settable to true")
	}
}

// T119: userMsgOptIn
func TestUserMsgOptIn(t *testing.T) {
	s := New(DefaultConfig(), "/tmp/test")
	if s.UserMsgOptIn {
		t.Error("UserMsgOptIn should default to false")
	}
	s.UserMsgOptIn = true
	if !s.UserMsgOptIn {
		t.Error("UserMsgOptIn should be settable to true")
	}
}

// T120: clientType
func TestClientType(t *testing.T) {
	s := New(DefaultConfig(), "/tmp/test")
	if s.ClientType != "cli" {
		t.Errorf("ClientType = %q, want %q", s.ClientType, "cli")
	}

	s.SetClientType("agent")
	if s.ClientType != "agent" {
		t.Errorf("ClientType = %q, want %q", s.ClientType, "agent")
	}

	s.SetClientType("sdk")
	if s.ClientType != "sdk" {
		t.Errorf("ClientType = %q, want %q", s.ClientType, "sdk")
	}
}

// T115: modelStrings cache
func TestModelStrings(t *testing.T) {
	s := New(DefaultConfig(), "/tmp/test")
	if s.ModelStrings != nil {
		t.Error("ModelStrings should default to nil")
	}

	cache := map[string]string{"claude-sonnet-4-6": "Sonnet"}
	s.SetModelStrings(cache)
	if s.ModelStrings["claude-sonnet-4-6"] != "Sonnet" {
		t.Errorf("ModelStrings[claude-sonnet-4-6] = %q, want Sonnet", s.ModelStrings["claude-sonnet-4-6"])
	}

	s.ClearModelStrings()
	if s.ModelStrings != nil {
		t.Error("ClearModelStrings should set ModelStrings to nil")
	}
}

// T121: sessionSource
func TestSessionSource(t *testing.T) {
	s := New(DefaultConfig(), "/tmp/test")
	if s.SessionSource != "" {
		t.Errorf("SessionSource = %q, want empty", s.SessionSource)
	}

	s.SetSessionSource("cli")
	if s.SessionSource != "cli" {
		t.Errorf("SessionSource = %q, want %q", s.SessionSource, "cli")
	}

	s.SetSessionSource("sdk")
	if s.SessionSource != "sdk" {
		t.Errorf("SessionSource = %q, want %q", s.SessionSource, "sdk")
	}

	s.SetSessionSource("bridge")
	if s.SessionSource != "bridge" {
		t.Errorf("SessionSource = %q, want %q", s.SessionSource, "bridge")
	}
}

// T122: questionPreviewFormat
func TestQuestionPreviewFormat(t *testing.T) {
	s := New(DefaultConfig(), "/tmp/test")
	if s.QuestionPreviewFormat != "" {
		t.Errorf("QuestionPreviewFormat = %q, want empty", s.QuestionPreviewFormat)
	}

	s.SetQuestionPreviewFormat("markdown")
	if s.QuestionPreviewFormat != "markdown" {
		t.Errorf("QuestionPreviewFormat = %q, want %q", s.QuestionPreviewFormat, "markdown")
	}

	s.SetQuestionPreviewFormat("html")
	if s.QuestionPreviewFormat != "html" {
		t.Errorf("QuestionPreviewFormat = %q, want %q", s.QuestionPreviewFormat, "html")
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
