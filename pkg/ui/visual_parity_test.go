package ui

import (
	"regexp"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/projectbarks/gopher-code/pkg/message"
	"github.com/projectbarks/gopher-code/pkg/query"
	"github.com/projectbarks/gopher-code/pkg/session"
	"github.com/projectbarks/gopher-code/pkg/ui/components"
	"github.com/projectbarks/gopher-code/pkg/ui/theme"
)

// stripANSI removes ANSI escape sequences for text comparison.
var ansiRegex = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func strip(s string) string {
	return ansiRegex.ReplaceAllString(s, "")
}

// TestVisualParity_StartupShowsWelcome verifies the welcome screen on startup.
func TestVisualParity_StartupShowsWelcome(t *testing.T) {
	config := session.DefaultConfig()
	config.Model = "claude-opus-4-6"
	sess := session.New(config, "/Users/test/project")
	app := NewAppModel(sess, nil)
	app.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

	view := app.View()
	plain := strip(view.Content)

	// Welcome screen should be visible
	if !strings.Contains(plain, "Welcome") {
		t.Error("Startup should show Welcome screen")
	}
	if !strings.Contains(plain, "Gopher") {
		t.Error("Startup should show Gopher branding")
	}
	if !strings.Contains(plain, "Tips") {
		t.Error("Startup should show Tips section")
	}
	// Prompt should be visible below welcome
	if !strings.Contains(plain, "›") {
		t.Error("Prompt › should be visible below welcome")
	}
	// Divider should be visible
	if !strings.Contains(plain, "━") {
		t.Error("Heavy divider should be visible")
	}
}

// TestVisualParity_WelcomeDismissOnSubmit verifies welcome dismisses on input.
func TestVisualParity_WelcomeDismissOnSubmit(t *testing.T) {
	config := session.DefaultConfig()
	sess := session.New(config, "/tmp")
	app := NewAppModel(sess, nil)
	app.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

	// Submit a message
	app.Update(components.SubmitMsg{Text: "hello"})

	view := app.View()
	plain := strip(view.Content)

	// Welcome should be gone
	if strings.Contains(plain, "Welcome") {
		t.Error("Welcome screen should be dismissed after submit")
	}
	// Header should now show
	if !strings.Contains(plain, "Gopher") {
		t.Error("Header should show after welcome dismiss")
	}
}

// TestVisualParity_UserMessageStyling verifies user messages use › prefix and bold.
func TestVisualParity_UserMessageStyling(t *testing.T) {
	config := session.DefaultConfig()
	sess := session.New(config, "/tmp")
	app := NewAppModel(sess, nil)
	app.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	app.Update(components.SubmitMsg{Text: "explain this code"})

	view := app.View()
	plain := strip(view.Content)

	// User message should use › prefix
	if !strings.Contains(plain, "› explain this code") {
		t.Errorf("Expected '› explain this code' in view, got:\n%s", plain)
	}
}

// TestVisualParity_StreamingShowsSpinner verifies spinner during streaming.
func TestVisualParity_StreamingShowsSpinner(t *testing.T) {
	config := session.DefaultConfig()
	sess := session.New(config, "/tmp")
	app := NewAppModel(sess, nil)
	app.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	app.Update(components.SubmitMsg{Text: "test"})

	// Simulate streaming — the spinner should be active
	if !app.spinner.IsActive() {
		t.Error("Spinner should be active after submit")
	}

	view := app.View()
	plain := strip(view.Content)

	// Should show spinner verb in conversation area
	verb := app.spinner.Verb()
	if !strings.Contains(plain, verb) {
		t.Errorf("Expected spinner verb %q in view during streaming", verb)
	}
	if !strings.Contains(plain, "thinking") {
		t.Error("Expected 'thinking' in spinner output")
	}
}

// TestVisualParity_StreamingStatusBar verifies "esc to interrupt" during streaming.
func TestVisualParity_StreamingStatusBar(t *testing.T) {
	config := session.DefaultConfig()
	sess := session.New(config, "/tmp")
	app := NewAppModel(sess, nil)
	app.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	app.Update(components.SubmitMsg{Text: "test"})

	view := app.View()
	plain := strip(view.Content)

	if !strings.Contains(plain, "esc to interrupt") {
		t.Errorf("Expected 'esc to interrupt' in status bar during streaming, got:\n%s", plain)
	}
}

// TestVisualParity_ToolResultUsesConnector verifies "  └ " connector on results.
func TestVisualParity_ToolResultUsesConnector(t *testing.T) {
	// Test MessageBubble directly — the connector appears when rendering
	// a message with tool result content blocks
	mb := components.NewMessageBubble(theme.Current(), 80)
	msg := &message.Message{
		Role: message.RoleAssistant,
		Content: []message.ContentBlock{
			{Type: message.ContentText, Text: "Let me check."},
			{Type: message.ContentToolResult, Content: "file1.txt\nfile2.txt"},
		},
	}
	result := mb.Render(msg)
	if !strings.Contains(result, "└") {
		t.Error("Expected └ connector in tool result rendering")
	}
}

// TestVisualParity_IdleStatusShowsModel verifies idle status bar content.
func TestVisualParity_IdleStatusShowsModel(t *testing.T) {
	config := session.DefaultConfig()
	config.Model = "claude-opus-4-6"
	sess := session.New(config, "/tmp")
	app := NewAppModel(sess, nil)
	app.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

	// Dismiss welcome
	app.Update(components.SubmitMsg{Text: ""})

	view := app.View()
	plain := strip(view.Content)

	if !strings.Contains(plain, "claude-opus-4-6") {
		t.Errorf("Idle status should show model name, got:\n%s", plain)
	}
}

// TestVisualParity_FullConversationFlow tests a complete conversation sequence.
func TestVisualParity_FullConversationFlow(t *testing.T) {
	config := session.DefaultConfig()
	sess := session.New(config, "/tmp")
	app := NewAppModel(sess, nil)
	app.Update(tea.WindowSizeMsg{Width: 80, Height: 30})

	// 1. Start: welcome screen visible
	v1 := strip(app.View().Content)
	if !strings.Contains(v1, "Welcome") {
		t.Error("Step 1: Expected welcome screen")
	}

	// 2. Submit query: welcome dismissed, spinner starts
	app.Update(components.SubmitMsg{Text: "hello world"})
	v2 := strip(app.View().Content)
	if strings.Contains(v2, "Welcome") {
		t.Error("Step 2: Welcome should be gone")
	}
	if !strings.Contains(v2, "› hello world") {
		t.Error("Step 2: Expected user message with › prefix")
	}
	if !strings.Contains(v2, "esc to interrupt") {
		t.Error("Step 2: Expected 'esc to interrupt' during streaming")
	}

	// 3. Text arrives
	app.Update(TextDeltaMsg{Text: "Hi! How can I help?"})
	v3 := strip(app.View().Content)
	if !strings.Contains(v3, "Hi! How can I help?") {
		t.Error("Step 3: Expected streaming text in view")
	}

	// 4. Turn complete
	app.Update(TurnCompleteMsg{})
	v4 := strip(app.View().Content)
	if strings.Contains(v4, "esc to interrupt") {
		t.Error("Step 4: Should not show 'esc to interrupt' after completion")
	}

	// 5. Verify conversation has messages
	if app.conversation.MessageCount() != 2 {
		t.Errorf("Step 5: Expected 2 messages (user + assistant), got %d",
			app.conversation.MessageCount())
	}
}

// TestVisualParity_SlashCommandClear verifies /clear works.
func TestVisualParity_SlashCommandClear(t *testing.T) {
	config := session.DefaultConfig()
	sess := session.New(config, "/tmp")
	app := NewAppModel(sess, nil)
	app.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

	// Add a message then clear
	app.Update(components.SubmitMsg{Text: "hello"})
	app.Update(TurnCompleteMsg{})

	// Submit /clear command
	_, cmd := app.Update(components.SubmitMsg{Text: "/clear"})
	if cmd != nil {
		msg := cmd()
		app.Update(msg)
	}

	if app.conversation.MessageCount() != 0 {
		t.Errorf("Expected 0 messages after /clear, got %d", app.conversation.MessageCount())
	}
}

// TestVisualParity_EffortLevelDisplay verifies effort icons with thinking enabled.
func TestVisualParity_EffortLevelDisplay(t *testing.T) {
	config := session.DefaultConfig()
	config.ThinkingEnabled = true
	config.ThinkingBudget = 20000 // high effort
	sess := session.New(config, "/tmp")
	app := NewAppModel(sess, nil)
	app.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

	app.Update(components.SubmitMsg{Text: "think hard"})

	// Spinner should show effort level
	spinnerView := strip(app.spinner.View())
	if !strings.Contains(spinnerView, "●") {
		t.Errorf("Expected high effort icon ● in spinner, got: %s", spinnerView)
	}
}

// TestVisualParity_DividerSpansFullWidth verifies divider line width.
func TestVisualParity_DividerSpansFullWidth(t *testing.T) {
	config := session.DefaultConfig()
	sess := session.New(config, "/tmp")
	app := NewAppModel(sess, nil)
	app.Update(tea.WindowSizeMsg{Width: 60, Height: 24})

	view := app.View()
	lines := strings.Split(view.Content, "\n")

	foundDivider := false
	for _, line := range lines {
		plain := strip(line)
		if strings.Count(plain, "━") >= 50 {
			foundDivider = true
			break
		}
	}
	if !foundDivider {
		t.Error("Expected divider line of ━ characters spanning width")
	}
}

// TestVisualParity_CtrlCQuitsWhenIdle verifies Ctrl+C quits.
func TestVisualParity_CtrlCQuitsWhenIdle(t *testing.T) {
	app := NewAppModel(nil, nil)
	_, cmd := app.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	if cmd == nil {
		t.Fatal("Ctrl+C should produce quit command")
	}
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Errorf("Expected QuitMsg, got %T", msg)
	}
}

// TestVisualParity_QueryEventFlow verifies full QueryEvent routing.
func TestVisualParity_QueryEventFlow(t *testing.T) {
	config := session.DefaultConfig()
	sess := session.New(config, "/tmp")
	app := NewAppModel(sess, nil)

	events := []query.QueryEvent{
		{Type: query.QEventTextDelta, Text: "Hello "},
		{Type: query.QEventTextDelta, Text: "world"},
		{Type: query.QEventUsage, InputTokens: 50, OutputTokens: 25},
		{Type: query.QEventTurnComplete},
	}

	for _, evt := range events {
		app.Update(QueryEventMsg{Event: evt})
	}

	if app.mode != ModeIdle {
		t.Error("Should be idle after TurnComplete")
	}
	if sess.TotalInputTokens != 50 {
		t.Errorf("Expected 50 input tokens, got %d", sess.TotalInputTokens)
	}
	if app.conversation.MessageCount() != 1 {
		t.Errorf("Expected 1 finalized message, got %d", app.conversation.MessageCount())
	}
}
