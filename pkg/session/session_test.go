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

// T130: lastAPIRequest / lastAPIRequestMessages
func TestLastAPIRequest(t *testing.T) {
	s := New(DefaultConfig(), "/tmp/test")

	// Defaults to nil
	if s.LastAPIRequest != nil {
		t.Error("LastAPIRequest should default to nil")
	}
	if s.LastAPIRequestMessages != nil {
		t.Error("LastAPIRequestMessages should default to nil")
	}

	// Set via method
	msgs := []provider.RequestMessage{
		{Role: "user", Content: []provider.RequestContent{{Type: "text", Text: "hello"}}},
	}
	s.SetLastAPIRequest(map[string]string{"model": "test"}, msgs)

	if s.LastAPIRequest == nil {
		t.Error("LastAPIRequest should be non-nil after set")
	}
	if len(s.LastAPIRequestMessages) != 1 {
		t.Errorf("LastAPIRequestMessages len = %d, want 1", len(s.LastAPIRequestMessages))
	}
	if s.LastAPIRequestMessages[0].Role != "user" {
		t.Errorf("LastAPIRequestMessages[0].Role = %q, want user", s.LastAPIRequestMessages[0].Role)
	}

	// Clear
	s.ClearLastAPIRequest()
	if s.LastAPIRequest != nil {
		t.Error("LastAPIRequest should be nil after clear")
	}
	if s.LastAPIRequestMessages != nil {
		t.Error("LastAPIRequestMessages should be nil after clear")
	}
}

// T131: lastClassifierRequests
func TestLastClassifierRequests(t *testing.T) {
	s := New(DefaultConfig(), "/tmp/test")

	if s.LastClassifierRequests != nil {
		t.Error("LastClassifierRequests should default to nil")
	}

	// Add requests
	s.AddClassifierRequest(map[string]string{"model": "classifier-v1"})
	s.AddClassifierRequest(map[string]string{"model": "classifier-v2"})

	if len(s.LastClassifierRequests) != 2 {
		t.Fatalf("LastClassifierRequests len = %d, want 2", len(s.LastClassifierRequests))
	}

	// Clear
	s.ClearClassifierRequests()
	if s.LastClassifierRequests != nil {
		t.Error("LastClassifierRequests should be nil after clear")
	}
}

// T132: cachedClaudeMdContent
func TestCachedClaudeMdContent(t *testing.T) {
	s := New(DefaultConfig(), "/tmp/test")

	if s.GetCachedClaudeMdContent() != "" {
		t.Error("CachedClaudeMdContent should default to empty")
	}

	content := "# CLAUDE.md\nAlways use gofmt."
	s.SetCachedClaudeMdContent(content)
	if got := s.GetCachedClaudeMdContent(); got != content {
		t.Errorf("GetCachedClaudeMdContent() = %q, want %q", got, content)
	}

	// Overwrite
	s.SetCachedClaudeMdContent("")
	if got := s.GetCachedClaudeMdContent(); got != "" {
		t.Errorf("GetCachedClaudeMdContent() after clear = %q, want empty", got)
	}
}

// T133: inMemoryErrorLog
func TestInMemoryErrorLog(t *testing.T) {
	s := New(DefaultConfig(), "/tmp/test")

	if s.InMemoryErrorLog != nil {
		t.Error("InMemoryErrorLog should default to nil")
	}

	// Append errors
	s.AppendError("connection timeout")
	s.AppendError("rate limited")
	s.AppendError("invalid response")

	if len(s.InMemoryErrorLog) != 3 {
		t.Fatalf("InMemoryErrorLog len = %d, want 3", len(s.InMemoryErrorLog))
	}
	if s.InMemoryErrorLog[0] != "connection timeout" {
		t.Errorf("InMemoryErrorLog[0] = %q, want %q", s.InMemoryErrorLog[0], "connection timeout")
	}

	// GetErrorLog returns a copy
	log := s.GetErrorLog()
	if len(log) != 3 {
		t.Fatalf("GetErrorLog() len = %d, want 3", len(log))
	}
	log[0] = "mutated"
	if s.InMemoryErrorLog[0] == "mutated" {
		t.Error("GetErrorLog should return a copy, not a reference")
	}

	// GetErrorLog on empty session
	s2 := New(DefaultConfig(), "/tmp/test")
	if got := s2.GetErrorLog(); got != nil {
		t.Errorf("GetErrorLog() on empty session = %v, want nil", got)
	}
}

// T135: sessionBypassPermissionsMode
func TestSessionBypassPermissionsMode(t *testing.T) {
	s := New(DefaultConfig(), "/tmp/test")

	if s.SessionBypassPermissionsMode {
		t.Error("SessionBypassPermissionsMode should default to false")
	}

	s.SetBypassPermissionsMode(true)
	if !s.SessionBypassPermissionsMode {
		t.Error("SessionBypassPermissionsMode should be true after SetBypassPermissionsMode(true)")
	}

	s.SetBypassPermissionsMode(false)
	if s.SessionBypassPermissionsMode {
		t.Error("SessionBypassPermissionsMode should be false after SetBypassPermissionsMode(false)")
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

// T136: scheduledTasksEnabled + sessionCronTasks + SessionCronTask
func TestScheduledTasksAndCronTasks(t *testing.T) {
	s := New(DefaultConfig(), "/tmp/test")

	// Defaults
	if s.ScheduledTasksEnabled {
		t.Error("ScheduledTasksEnabled should default to false")
	}
	if len(s.SessionCronTasks) != 0 {
		t.Errorf("SessionCronTasks len = %d, want 0", len(s.SessionCronTasks))
	}

	// Toggle enabled flag
	s.SetScheduledTasksEnabled(true)
	if !s.GetScheduledTasksEnabled() {
		t.Error("GetScheduledTasksEnabled should return true after set")
	}
	s.SetScheduledTasksEnabled(false)
	if s.GetScheduledTasksEnabled() {
		t.Error("GetScheduledTasksEnabled should return false after unset")
	}

	// Add cron tasks
	s.AddSessionCronTask(SessionCronTask{ID: "cron-1", Cron: "*/5 * * * *", Prompt: "check status", CreatedAt: 1000, Recurring: true})
	s.AddSessionCronTask(SessionCronTask{ID: "cron-2", Cron: "0 9 * * *", Prompt: "morning report", CreatedAt: 2000})
	s.AddSessionCronTask(SessionCronTask{ID: "cron-3", Cron: "0 17 * * 1-5", Prompt: "eod summary", CreatedAt: 3000, AgentID: "agent-1"})

	tasks := s.GetSessionCronTasks()
	if len(tasks) != 3 {
		t.Fatalf("GetSessionCronTasks len = %d, want 3", len(tasks))
	}
	if tasks[0].ID != "cron-1" || tasks[0].Recurring != true {
		t.Errorf("task[0] = %+v, want cron-1 recurring", tasks[0])
	}
	if tasks[2].AgentID != "agent-1" {
		t.Errorf("task[2].AgentID = %q, want agent-1", tasks[2].AgentID)
	}

	// Remove by ID — returns count
	removed := s.RemoveSessionCronTasks([]string{"cron-1", "cron-3"})
	if removed != 2 {
		t.Errorf("RemoveSessionCronTasks returned %d, want 2", removed)
	}
	if len(s.SessionCronTasks) != 1 {
		t.Fatalf("after remove, len = %d, want 1", len(s.SessionCronTasks))
	}
	if s.SessionCronTasks[0].ID != "cron-2" {
		t.Errorf("remaining task ID = %q, want cron-2", s.SessionCronTasks[0].ID)
	}

	// Remove non-existent returns 0
	if n := s.RemoveSessionCronTasks([]string{"no-such-id"}); n != 0 {
		t.Errorf("RemoveSessionCronTasks(non-existent) = %d, want 0", n)
	}

	// Remove empty slice returns 0
	if n := s.RemoveSessionCronTasks(nil); n != 0 {
		t.Errorf("RemoveSessionCronTasks(nil) = %d, want 0", n)
	}
}

// T137: sessionCreatedTeams
func TestSessionCreatedTeams(t *testing.T) {
	s := New(DefaultConfig(), "/tmp/test")

	// Default: initialized empty set
	teams := s.GetSessionCreatedTeams()
	if len(teams) != 0 {
		t.Errorf("SessionCreatedTeams len = %d, want 0", len(teams))
	}

	// Add teams
	s.AddSessionCreatedTeam("team-alpha")
	s.AddSessionCreatedTeam("team-beta")
	s.AddSessionCreatedTeam("team-alpha") // duplicate is a no-op

	if len(s.SessionCreatedTeams) != 2 {
		t.Errorf("SessionCreatedTeams len = %d, want 2", len(s.SessionCreatedTeams))
	}
	if _, ok := s.SessionCreatedTeams["team-alpha"]; !ok {
		t.Error("team-alpha should be in SessionCreatedTeams")
	}

	// Remove team
	s.RemoveSessionCreatedTeam("team-alpha")
	if _, ok := s.SessionCreatedTeams["team-alpha"]; ok {
		t.Error("team-alpha should be removed from SessionCreatedTeams")
	}
	if len(s.SessionCreatedTeams) != 1 {
		t.Errorf("after remove, len = %d, want 1", len(s.SessionCreatedTeams))
	}

	// Remove non-existent is safe
	s.RemoveSessionCreatedTeam("no-such-team")
}

// T138: sessionTrustAccepted
func TestSessionTrustAccepted(t *testing.T) {
	s := New(DefaultConfig(), "/tmp/test")

	if s.GetSessionTrustAccepted() {
		t.Error("SessionTrustAccepted should default to false")
	}

	s.SetSessionTrustAccepted(true)
	if !s.GetSessionTrustAccepted() {
		t.Error("SessionTrustAccepted should be true after set")
	}

	s.SetSessionTrustAccepted(false)
	if s.GetSessionTrustAccepted() {
		t.Error("SessionTrustAccepted should be false after unset")
	}
}

// T139: sessionPersistenceDisabled
func TestSessionPersistenceDisabled(t *testing.T) {
	s := New(DefaultConfig(), "/tmp/test")

	if s.IsSessionPersistenceDisabled() {
		t.Error("SessionPersistenceDisabled should default to false")
	}

	s.SetSessionPersistenceDisabled(true)
	if !s.IsSessionPersistenceDisabled() {
		t.Error("SessionPersistenceDisabled should be true after set")
	}

	s.SetSessionPersistenceDisabled(false)
	if s.IsSessionPersistenceDisabled() {
		t.Error("SessionPersistenceDisabled should be false after unset")
	}
}

// T140: plan mode transitions
func TestPlanModeTransitions(t *testing.T) {
	s := New(DefaultConfig(), "/tmp/test")

	// Defaults
	if s.HasExitedPlanModeInSession() {
		t.Error("HasExitedPlanMode should default to false")
	}
	if s.GetNeedsPlanModeExitAttachment() {
		t.Error("NeedsPlanModeExitAttachment should default to false")
	}

	// Entering plan mode from normal: clears exit attachment
	s.SetNeedsPlanModeExitAttachment(true) // simulate stale flag
	s.HandlePlanModeTransition("normal", "plan")
	if s.GetNeedsPlanModeExitAttachment() {
		t.Error("entering plan mode should clear NeedsPlanModeExitAttachment")
	}

	// Exiting plan mode to normal: triggers exit attachment
	s.HandlePlanModeTransition("plan", "normal")
	if !s.GetNeedsPlanModeExitAttachment() {
		t.Error("exiting plan mode should set NeedsPlanModeExitAttachment")
	}

	// plan -> plan: no-op (both conditions false)
	s.SetNeedsPlanModeExitAttachment(false)
	s.HandlePlanModeTransition("plan", "plan")
	if s.GetNeedsPlanModeExitAttachment() {
		t.Error("plan->plan should not set NeedsPlanModeExitAttachment")
	}

	// normal -> normal: no-op
	s.HandlePlanModeTransition("normal", "normal")
	if s.GetNeedsPlanModeExitAttachment() {
		t.Error("normal->normal should not set NeedsPlanModeExitAttachment")
	}

	// Quick toggle: enter then exit plan mode
	s.HandlePlanModeTransition("normal", "plan")
	s.HandlePlanModeTransition("plan", "auto")
	if !s.GetNeedsPlanModeExitAttachment() {
		t.Error("plan->auto should set NeedsPlanModeExitAttachment")
	}

	// SetHasExitedPlanMode
	s.SetHasExitedPlanMode(true)
	if !s.HasExitedPlanModeInSession() {
		t.Error("HasExitedPlanMode should be true after set")
	}
}

// T141: auto mode transitions
func TestAutoModeTransitions(t *testing.T) {
	s := New(DefaultConfig(), "/tmp/test")

	// Default: no exit attachment pending
	if s.GetNeedsAutoModeExitAttachment() {
		t.Error("NeedsAutoModeExitAttachment should default to false")
	}

	// Entering auto mode from normal: clears exit attachment
	s.SetNeedsAutoModeExitAttachment(true) // simulate stale flag
	s.HandleAutoModeTransition("normal", "auto")
	if s.GetNeedsAutoModeExitAttachment() {
		t.Error("entering auto mode should clear NeedsAutoModeExitAttachment")
	}

	// Exiting auto mode to normal: triggers exit attachment
	s.HandleAutoModeTransition("auto", "normal")
	if !s.GetNeedsAutoModeExitAttachment() {
		t.Error("exiting auto mode should set NeedsAutoModeExitAttachment")
	}

	// auto -> plan: skipped (handled by plan mode logic)
	s.SetNeedsAutoModeExitAttachment(false)
	s.HandleAutoModeTransition("auto", "plan")
	if s.GetNeedsAutoModeExitAttachment() {
		t.Error("auto->plan should be skipped (no-op)")
	}

	// plan -> auto: skipped
	s.HandleAutoModeTransition("plan", "auto")
	if s.GetNeedsAutoModeExitAttachment() {
		t.Error("plan->auto should be skipped (no-op)")
	}

	// auto -> auto: no-op (both fromIsAuto and toIsAuto, enter branch is false)
	s.HandleAutoModeTransition("auto", "auto")
	if s.GetNeedsAutoModeExitAttachment() {
		t.Error("auto->auto should not set NeedsAutoModeExitAttachment")
	}

	// normal -> normal: no-op
	s.HandleAutoModeTransition("normal", "normal")
	if s.GetNeedsAutoModeExitAttachment() {
		t.Error("normal->normal should not set NeedsAutoModeExitAttachment")
	}

	// Quick toggle: enter then exit auto mode
	s.HandleAutoModeTransition("normal", "auto")
	s.HandleAutoModeTransition("auto", "normal")
	if !s.GetNeedsAutoModeExitAttachment() {
		t.Error("auto->normal should set NeedsAutoModeExitAttachment")
	}
}

// T142: lspRecommendationShownThisSession
func TestLspRecommendationShownThisSession(t *testing.T) {
	s := New(DefaultConfig(), "/tmp/test")

	if s.HasShownLspRecommendationThisSession() {
		t.Error("LspRecommendationShownThisSession should default to false")
	}

	s.SetLspRecommendationShownThisSession(true)
	if !s.HasShownLspRecommendationThisSession() {
		t.Error("LspRecommendationShownThisSession should be true after set")
	}

	s.SetLspRecommendationShownThisSession(false)
	if s.HasShownLspRecommendationThisSession() {
		t.Error("LspRecommendationShownThisSession should be false after unset")
	}
}

// T143: SDK init state (initJsonSchema / resetSdkInitState)
func TestSdkInitState(t *testing.T) {
	s := New(DefaultConfig(), "/tmp/test")

	// Defaults
	if s.GetInitJsonSchema() != nil {
		t.Error("InitJsonSchema should default to nil")
	}
	if s.SdkInitialized {
		t.Error("SdkInitialized should default to false")
	}

	// Set schema
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"name": map[string]string{"type": "string"},
		},
	}
	s.SetInitJsonSchema(schema)
	if s.GetInitJsonSchema() == nil {
		t.Error("InitJsonSchema should be non-nil after set")
	}
	if !s.SdkInitialized {
		t.Error("SdkInitialized should be true after SetInitJsonSchema")
	}

	// Verify schema content
	got, ok := s.GetInitJsonSchema().(map[string]interface{})
	if !ok {
		t.Fatal("InitJsonSchema should be a map")
	}
	if got["type"] != "object" {
		t.Errorf("schema type = %v, want object", got["type"])
	}

	// Reset
	s.ResetSdkInitState()
	if s.GetInitJsonSchema() != nil {
		t.Error("InitJsonSchema should be nil after reset")
	}
	if s.SdkInitialized {
		t.Error("SdkInitialized should be false after reset")
	}
}

// T145: planSlugCache
func TestPlanSlugCache(t *testing.T) {
	s := New(DefaultConfig(), "/tmp/test")

	// Default: empty map
	if s.PlanSlugCache == nil {
		t.Fatal("PlanSlugCache should be initialized")
	}
	if len(s.PlanSlugCache) != 0 {
		t.Errorf("PlanSlugCache len = %d, want 0", len(s.PlanSlugCache))
	}

	// Set and get
	s.SetPlanSlug("session-1", "fluffy-cat")
	slug, ok := s.GetPlanSlug("session-1")
	if !ok || slug != "fluffy-cat" {
		t.Errorf("GetPlanSlug(session-1) = (%q, %v), want (fluffy-cat, true)", slug, ok)
	}

	// Missing key
	_, ok = s.GetPlanSlug("no-such-session")
	if ok {
		t.Error("GetPlanSlug should return false for missing key")
	}

	// Overwrite
	s.SetPlanSlug("session-1", "happy-dog")
	slug, _ = s.GetPlanSlug("session-1")
	if slug != "happy-dog" {
		t.Errorf("GetPlanSlug after overwrite = %q, want happy-dog", slug)
	}

	// Delete
	s.DeletePlanSlug("session-1")
	_, ok = s.GetPlanSlug("session-1")
	if ok {
		t.Error("GetPlanSlug should return false after delete")
	}

	// Delete non-existent is safe
	s.DeletePlanSlug("no-such-session")
}

// T146: teleportedSessionInfo
func TestTeleportedSessionInfo(t *testing.T) {
	s := New(DefaultConfig(), "/tmp/test")

	if s.TeleportedSessionInfo != nil {
		t.Error("TeleportedSessionInfo should default to nil")
	}

	info := map[string]string{"origin": "remote-host", "session_id": "abc-123"}
	s.TeleportedSessionInfo = info

	got, ok := s.TeleportedSessionInfo.(map[string]string)
	if !ok {
		t.Fatal("TeleportedSessionInfo should be a map[string]string")
	}
	if got["origin"] != "remote-host" {
		t.Errorf("TeleportedSessionInfo[origin] = %q, want remote-host", got["origin"])
	}
	if got["session_id"] != "abc-123" {
		t.Errorf("TeleportedSessionInfo[session_id] = %q, want abc-123", got["session_id"])
	}

	// Can be cleared
	s.TeleportedSessionInfo = nil
	if s.TeleportedSessionInfo != nil {
		t.Error("TeleportedSessionInfo should be nil after clear")
	}
}

// T147: invokedSkills
func TestInvokedSkills(t *testing.T) {
	s := New(DefaultConfig(), "/tmp/test")

	// Default: initialized empty map
	if s.InvokedSkills == nil {
		t.Fatal("InvokedSkills should be initialized")
	}
	if len(s.InvokedSkills) != 0 {
		t.Errorf("InvokedSkills len = %d, want 0", len(s.InvokedSkills))
	}

	// Mark and check
	s.MarkSkillInvoked("agent-1", "commit")
	if !s.HasInvokedSkill("agent-1", "commit") {
		t.Error("HasInvokedSkill(agent-1, commit) should be true")
	}
	if s.HasInvokedSkill("agent-1", "review") {
		t.Error("HasInvokedSkill(agent-1, review) should be false")
	}
	if s.HasInvokedSkill("agent-2", "commit") {
		t.Error("HasInvokedSkill(agent-2, commit) should be false (different agent)")
	}

	// Mark multiple skills
	s.MarkSkillInvoked("agent-1", "review")
	s.MarkSkillInvoked("agent-2", "commit")

	if !s.HasInvokedSkill("agent-1", "review") {
		t.Error("HasInvokedSkill(agent-1, review) should be true after mark")
	}
	if !s.HasInvokedSkill("agent-2", "commit") {
		t.Error("HasInvokedSkill(agent-2, commit) should be true after mark")
	}

	// ClearInvokedSkills with preserved agent IDs
	s.ClearInvokedSkills([]string{"agent-1"})
	if !s.HasInvokedSkill("agent-1", "commit") {
		t.Error("agent-1:commit should be preserved")
	}
	if !s.HasInvokedSkill("agent-1", "review") {
		t.Error("agent-1:review should be preserved")
	}
	if s.HasInvokedSkill("agent-2", "commit") {
		t.Error("agent-2:commit should be cleared")
	}

	// ClearInvokedSkills with no preserved IDs clears all
	s.ClearInvokedSkills(nil)
	if len(s.InvokedSkills) != 0 {
		t.Errorf("InvokedSkills should be empty after clear-all, got %d", len(s.InvokedSkills))
	}

	// Duplicate mark is idempotent
	s.MarkSkillInvoked("agent-1", "commit")
	s.MarkSkillInvoked("agent-1", "commit")
	if len(s.InvokedSkills) != 1 {
		t.Errorf("duplicate mark should be idempotent, got %d entries", len(s.InvokedSkills))
	}
}

// T148: slowOperations
func TestSlowOperations(t *testing.T) {
	s := New(DefaultConfig(), "/tmp/test")

	// Default: initialized empty map
	if s.SlowOperations == nil {
		t.Fatal("SlowOperations should be initialized")
	}
	if len(s.SlowOperations) != 0 {
		t.Errorf("SlowOperations len = %d, want 0", len(s.SlowOperations))
	}

	// Record and get
	s.RecordSlowOperation("mcp-connect", 2500*time.Millisecond)
	s.RecordSlowOperation("tool-execute", 1200*time.Millisecond)

	ops := s.GetSlowOperations()
	if len(ops) != 2 {
		t.Fatalf("GetSlowOperations len = %d, want 2", len(ops))
	}
	if ops["mcp-connect"] != 2500*time.Millisecond {
		t.Errorf("mcp-connect duration = %v, want 2500ms", ops["mcp-connect"])
	}
	if ops["tool-execute"] != 1200*time.Millisecond {
		t.Errorf("tool-execute duration = %v, want 1200ms", ops["tool-execute"])
	}

	// GetSlowOperations returns a copy
	ops["mcp-connect"] = 0
	if s.SlowOperations["mcp-connect"] != 2500*time.Millisecond {
		t.Error("GetSlowOperations should return a copy, not a reference")
	}

	// Overwrite existing operation
	s.RecordSlowOperation("mcp-connect", 5000*time.Millisecond)
	if s.SlowOperations["mcp-connect"] != 5000*time.Millisecond {
		t.Errorf("mcp-connect after overwrite = %v, want 5000ms", s.SlowOperations["mcp-connect"])
	}

	// GetSlowOperations on nil returns nil
	s2 := New(DefaultConfig(), "/tmp/test")
	s2.SlowOperations = nil
	if got := s2.GetSlowOperations(); got != nil {
		t.Errorf("GetSlowOperations on nil = %v, want nil", got)
	}

	// RecordSlowOperation on nil map initializes it
	s2.RecordSlowOperation("late-init", time.Second)
	if s2.SlowOperations == nil {
		t.Error("RecordSlowOperation should initialize nil map")
	}
	if s2.SlowOperations["late-init"] != time.Second {
		t.Errorf("late-init = %v, want 1s", s2.SlowOperations["late-init"])
	}
}

// T149: sdkBetas
func TestSdkBetas(t *testing.T) {
	s := New(DefaultConfig(), "/tmp/test")

	if s.SdkBetas != nil {
		t.Error("SdkBetas should default to nil")
	}

	s.SdkBetas = []string{"prompt-caching-2024-07-31", "max-tokens-3-5-sonnet-2024-07-15"}
	if len(s.SdkBetas) != 2 {
		t.Fatalf("SdkBetas len = %d, want 2", len(s.SdkBetas))
	}
	if s.SdkBetas[0] != "prompt-caching-2024-07-31" {
		t.Errorf("SdkBetas[0] = %q, want prompt-caching-2024-07-31", s.SdkBetas[0])
	}
	if s.SdkBetas[1] != "max-tokens-3-5-sonnet-2024-07-15" {
		t.Errorf("SdkBetas[1] = %q, want max-tokens-3-5-sonnet-2024-07-15", s.SdkBetas[1])
	}

	// Can be cleared
	s.SdkBetas = nil
	if s.SdkBetas != nil {
		t.Error("SdkBetas should be nil after clear")
	}
}

// T150: mainThreadAgentType
func TestMainThreadAgentType(t *testing.T) {
	s := New(DefaultConfig(), "/tmp/test")

	if s.MainThreadAgentType != "" {
		t.Errorf("MainThreadAgentType = %q, want empty", s.MainThreadAgentType)
	}

	s.MainThreadAgentType = "coordinator"
	if s.MainThreadAgentType != "coordinator" {
		t.Errorf("MainThreadAgentType = %q, want coordinator", s.MainThreadAgentType)
	}

	s.MainThreadAgentType = "normal"
	if s.MainThreadAgentType != "normal" {
		t.Errorf("MainThreadAgentType = %q, want normal", s.MainThreadAgentType)
	}
}

// T146-T150: New() initializes maps for new fields
func TestNew_InitializesT146_T150Fields(t *testing.T) {
	s := New(DefaultConfig(), "/tmp/test")

	if s.TeleportedSessionInfo != nil {
		t.Error("TeleportedSessionInfo should default to nil")
	}
	if s.InvokedSkills == nil {
		t.Error("InvokedSkills should be initialized (non-nil)")
	}
	if s.SlowOperations == nil {
		t.Error("SlowOperations should be initialized (non-nil)")
	}
	if s.SdkBetas != nil {
		t.Error("SdkBetas should default to nil")
	}
	if s.MainThreadAgentType != "" {
		t.Errorf("MainThreadAgentType = %q, want empty", s.MainThreadAgentType)
	}
	// T151
	if s.IsRemoteMode {
		t.Error("IsRemoteMode should default to false")
	}
	// T152
	if s.DirectConnectServerUrl != "" {
		t.Errorf("DirectConnectServerUrl = %q, want empty", s.DirectConnectServerUrl)
	}
	// T154
	if s.LastEmittedDate != "" {
		t.Errorf("LastEmittedDate = %q, want empty", s.LastEmittedDate)
	}
	// T155
	if s.AdditionalDirectoriesForClaudeMd != nil {
		t.Error("AdditionalDirectoriesForClaudeMd should default to nil")
	}
}

// ---------------------------------------------------------------------------
// T151: Remote mode
// ---------------------------------------------------------------------------

func TestIsRemoteMode(t *testing.T) {
	s := New(DefaultConfig(), "/tmp/test")

	if s.GetIsRemoteMode() {
		t.Error("GetIsRemoteMode should default to false")
	}

	s.SetIsRemoteMode(true)
	if !s.GetIsRemoteMode() {
		t.Error("GetIsRemoteMode should be true after SetIsRemoteMode(true)")
	}

	s.SetIsRemoteMode(false)
	if s.GetIsRemoteMode() {
		t.Error("GetIsRemoteMode should be false after SetIsRemoteMode(false)")
	}
}

// ---------------------------------------------------------------------------
// T152: Direct connect server URL
// ---------------------------------------------------------------------------

func TestDirectConnectServerUrl(t *testing.T) {
	s := New(DefaultConfig(), "/tmp/test")

	if s.GetDirectConnectServerUrl() != "" {
		t.Errorf("GetDirectConnectServerUrl should default to empty, got %q", s.GetDirectConnectServerUrl())
	}

	s.SetDirectConnectServerUrl("https://example.com:8443")
	if got := s.GetDirectConnectServerUrl(); got != "https://example.com:8443" {
		t.Errorf("GetDirectConnectServerUrl = %q, want %q", got, "https://example.com:8443")
	}

	s.SetDirectConnectServerUrl("")
	if got := s.GetDirectConnectServerUrl(); got != "" {
		t.Errorf("GetDirectConnectServerUrl = %q, want empty", got)
	}
}

// ---------------------------------------------------------------------------
// T154: Last emitted date
// ---------------------------------------------------------------------------

func TestLastEmittedDate(t *testing.T) {
	s := New(DefaultConfig(), "/tmp/test")

	if s.GetLastEmittedDate() != "" {
		t.Errorf("GetLastEmittedDate should default to empty, got %q", s.GetLastEmittedDate())
	}

	s.SetLastEmittedDate("2026-04-05")
	if got := s.GetLastEmittedDate(); got != "2026-04-05" {
		t.Errorf("GetLastEmittedDate = %q, want %q", got, "2026-04-05")
	}

	// Update to new date
	s.SetLastEmittedDate("2026-04-06")
	if got := s.GetLastEmittedDate(); got != "2026-04-06" {
		t.Errorf("GetLastEmittedDate = %q, want %q", got, "2026-04-06")
	}
}

// ---------------------------------------------------------------------------
// T155: Additional directories for CLAUDE.md
// ---------------------------------------------------------------------------

func TestAdditionalDirectoriesForClaudeMd(t *testing.T) {
	s := New(DefaultConfig(), "/tmp/test")

	if s.GetAdditionalDirectoriesForClaudeMd() != nil {
		t.Error("GetAdditionalDirectoriesForClaudeMd should default to nil")
	}

	dirs := []string{"/home/user/project-a", "/home/user/project-b"}
	s.SetAdditionalDirectoriesForClaudeMd(dirs)

	got := s.GetAdditionalDirectoriesForClaudeMd()
	if len(got) != 2 {
		t.Fatalf("GetAdditionalDirectoriesForClaudeMd len = %d, want 2", len(got))
	}
	if got[0] != "/home/user/project-a" {
		t.Errorf("got[0] = %q, want %q", got[0], "/home/user/project-a")
	}
	if got[1] != "/home/user/project-b" {
		t.Errorf("got[1] = %q, want %q", got[1], "/home/user/project-b")
	}

	// Clear
	s.SetAdditionalDirectoriesForClaudeMd(nil)
	if s.GetAdditionalDirectoriesForClaudeMd() != nil {
		t.Error("GetAdditionalDirectoriesForClaudeMd should be nil after clearing")
	}
}
