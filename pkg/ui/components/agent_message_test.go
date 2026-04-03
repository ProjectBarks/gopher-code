package components

import (
	"strings"
	"testing"

	"github.com/projectbarks/gopher-code/pkg/message"
	"github.com/projectbarks/gopher-code/pkg/ui/theme"
)

func TestAgentMessageRendererCreation(t *testing.T) {
	th := theme.Current()
	amr := NewAgentMessageRenderer(th)

	if amr == nil {
		t.Fatal("AgentMessageRenderer should not be nil")
	}

	if len(amr.agentColors) == 0 {
		t.Error("Should have default colors")
	}
}

func TestAgentMessageRendererDefaultColors(t *testing.T) {
	th := theme.Current()
	amr := NewAgentMessageRenderer(th)

	// Check default colors are set
	if !amr.HasAgent("user") {
		t.Error("Should have 'user' agent color")
	}

	if !amr.HasAgent("assistant") {
		t.Error("Should have 'assistant' agent color")
	}

	if !amr.HasAgent("system") {
		t.Error("Should have 'system' agent color")
	}
}

func TestAgentMessageRendererGetColor(t *testing.T) {
	th := theme.Current()
	amr := NewAgentMessageRenderer(th)

	userColor := amr.GetAgentColor("user")
	if userColor == "" {
		t.Error("Should return color for user agent")
	}

	assistantColor := amr.GetAgentColor("assistant")
	if assistantColor == "" {
		t.Error("Should return color for assistant agent")
	}

	// Colors should be different
	if userColor == assistantColor {
		t.Error("User and assistant colors should be different")
	}
}

func TestAgentMessageRendererGetColorCaseInsensitive(t *testing.T) {
	th := theme.Current()
	amr := NewAgentMessageRenderer(th)

	color1 := amr.GetAgentColor("user")
	color2 := amr.GetAgentColor("USER")
	color3 := amr.GetAgentColor("User")

	if color1 != color2 || color1 != color3 {
		t.Error("Color lookup should be case-insensitive")
	}
}

func TestAgentMessageRendererSetCustomColor(t *testing.T) {
	th := theme.Current()
	amr := NewAgentMessageRenderer(th)

	originalColor := amr.GetAgentColor("custom-agent")

	amr.SetAgentColor("custom-agent", "123456")
	newColor := amr.GetAgentColor("custom-agent")

	if newColor != "123456" {
		t.Errorf("Expected '123456', got '%s'", newColor)
	}

	if newColor == originalColor {
		t.Error("Color should have changed")
	}
}

func TestAgentMessageRendererGetColorForMessage(t *testing.T) {
	th := theme.Current()
	amr := NewAgentMessageRenderer(th)

	userMsg := message.UserMessage("hello")
	userColor := amr.GetAgentColorForMessage(&userMsg)

	if userColor == "" {
		t.Error("Should return color for user message")
	}

	assistantMsg := message.Message{
		Role:    message.RoleAssistant,
		Content: []message.ContentBlock{{Type: message.ContentText, Text: "hi"}},
	}
	assistantColor := amr.GetAgentColorForMessage(&assistantMsg)

	if assistantColor == "" {
		t.Error("Should return color for assistant message")
	}

	if userColor == assistantColor {
		t.Error("Colors should differ for different message roles")
	}
}

func TestAgentMessageRendererApplyAgentStyling(t *testing.T) {
	th := theme.Current()
	amr := NewAgentMessageRenderer(th)

	msg := message.UserMessage("test content")
	styled := amr.ApplyAgentStyling(&msg, "test content")

	if styled == "" {
		t.Error("Styled content should not be empty")
	}

	if !strings.Contains(styled, "test content") {
		t.Error("Styled content should contain original content")
	}
}

func TestAgentMessageRendererBorderColor(t *testing.T) {
	th := theme.Current()
	amr := NewAgentMessageRenderer(th)

	color := amr.GetBorderColor("user")

	if color == "" {
		t.Error("Should return border color")
	}
}

func TestAgentMessageRendererBackgroundColor(t *testing.T) {
	th := theme.Current()
	amr := NewAgentMessageRenderer(th)

	bgColor := amr.GetBackgroundColor("assistant")

	if bgColor == "" {
		t.Error("Should return background color")
	}
}

func TestAgentMessageRendererStyleMessageBubble(t *testing.T) {
	th := theme.Current()
	amr := NewAgentMessageRenderer(th)

	msg := message.UserMessage("test")
	styled := amr.StyleMessageBubble(&msg, "bubble content")

	if styled == "" {
		t.Error("Styled bubble should not be empty")
	}

	if !strings.Contains(styled, "bubble content") {
		t.Error("Styled bubble should contain content")
	}
}

func TestAgentMessageRendererGetAgentList(t *testing.T) {
	th := theme.Current()
	amr := NewAgentMessageRenderer(th)

	agents := amr.GetAgentList()

	if len(agents) == 0 {
		t.Error("Should have agents in list")
	}

	// Check for some expected agents
	hasUser := false
	hasAssistant := false

	for _, agent := range agents {
		if agent == "user" {
			hasUser = true
		}
		if agent == "assistant" {
			hasAssistant = true
		}
	}

	if !hasUser {
		t.Error("Should have 'user' in agent list")
	}

	if !hasAssistant {
		t.Error("Should have 'assistant' in agent list")
	}
}

func TestAgentMessageRendererHasAgent(t *testing.T) {
	th := theme.Current()
	amr := NewAgentMessageRenderer(th)

	if !amr.HasAgent("user") {
		t.Error("Should have 'user' agent")
	}

	if amr.HasAgent("nonexistent") {
		t.Error("Should not have 'nonexistent' agent")
	}

	amr.SetAgentColor("custom", "ffffff")
	if !amr.HasAgent("custom") {
		t.Error("Should have custom agent after setting")
	}
}

func TestAgentMessageRendererRemoveAgent(t *testing.T) {
	th := theme.Current()
	amr := NewAgentMessageRenderer(th)

	amr.SetAgentColor("test-agent", "123456")
	if !amr.HasAgent("test-agent") {
		t.Error("Should have agent before removal")
	}

	amr.RemoveAgent("test-agent")
	if amr.HasAgent("test-agent") {
		t.Error("Should not have agent after removal")
	}
}

func TestAgentMessageRendererResetToDefaults(t *testing.T) {
	th := theme.Current()
	amr := NewAgentMessageRenderer(th)

	// Modify colors
	amr.SetAgentColor("user", "ffffff")
	amr.RemoveAgent("system")

	// Reset
	amr.ResetToDefaults()

	if !amr.HasAgent("system") {
		t.Error("System agent should be restored")
	}

	if amr.GetAgentColor("user") == "ffffff" {
		t.Error("User color should be reset")
	}
}

func TestAgentMessageRendererCreateHighlightStyle(t *testing.T) {
	th := theme.Current()
	amr := NewAgentMessageRenderer(th)

	style := amr.CreateHighlightStyle("user")
	rendered := style.Render("test")

	if rendered == "" {
		t.Error("Should return valid style")
	}
}

func TestAgentMessageRendererCreateMutedStyle(t *testing.T) {
	th := theme.Current()
	amr := NewAgentMessageRenderer(th)

	style := amr.CreateMutedStyle("assistant")
	rendered := style.Render("test")

	if rendered == "" {
		t.Error("Should return valid style")
	}
}

func TestAgentMessageRendererCreateBadgeStyle(t *testing.T) {
	th := theme.Current()
	amr := NewAgentMessageRenderer(th)

	style := amr.CreateBadgeStyle("user")
	rendered := style.Render("test")

	if rendered == "" {
		t.Error("Should return valid style")
	}
}

func TestAgentMessageRendererGetColorScheme(t *testing.T) {
	th := theme.Current()
	amr := NewAgentMessageRenderer(th)

	cs := amr.GetColorScheme()

	if cs.TextPrimary == "" {
		t.Error("Should return valid color scheme")
	}
}

func TestAgentMessageRendererMultipleCustomColors(t *testing.T) {
	th := theme.Current()
	amr := NewAgentMessageRenderer(th)

	amr.SetAgentColor("agent-1", "111111")
	amr.SetAgentColor("agent-2", "222222")
	amr.SetAgentColor("agent-3", "333333")

	if amr.GetAgentColor("agent-1") != "111111" {
		t.Error("Should get correct color for agent-1")
	}

	if amr.GetAgentColor("agent-2") != "222222" {
		t.Error("Should get correct color for agent-2")
	}

	if amr.GetAgentColor("agent-3") != "333333" {
		t.Error("Should get correct color for agent-3")
	}
}

func TestAgentMessageRendererUnknownAgent(t *testing.T) {
	th := theme.Current()
	amr := NewAgentMessageRenderer(th)

	color := amr.GetAgentColor("unknown-agent-xyz")

	if color == "" {
		t.Error("Should return fallback color for unknown agent")
	}

	// Should be assistant color by default
	assistantColor := amr.GetAgentColor("assistant")
	if color != assistantColor {
		t.Errorf("Unknown agent should default to assistant color")
	}
}

func TestAgentMessageRendererStyleConsistency(t *testing.T) {
	th := theme.Current()
	amr := NewAgentMessageRenderer(th)

	msg1 := message.UserMessage("test1")
	msg2 := message.UserMessage("test2")

	styled1 := amr.StyleMessageBubble(&msg1, "content1")
	styled2 := amr.StyleMessageBubble(&msg2, "content2")

	// Both should be styled (non-empty)
	if styled1 == "" || styled2 == "" {
		t.Error("Both styled messages should be non-empty")
	}
}

func TestAgentMessageRendererThemeIntegration(t *testing.T) {
	th := theme.Current()
	amr := NewAgentMessageRenderer(th)

	cs := amr.GetColorScheme()

	// Verify colors from theme are used
	userColor := amr.GetAgentColor("user")
	if userColor != cs.Primary {
		t.Errorf("User color should match theme primary, got %s vs %s", userColor, cs.Primary)
	}

	assistantColor := amr.GetAgentColor("assistant")
	if assistantColor != cs.Accent {
		t.Errorf("Assistant color should match theme accent, got %s vs %s", assistantColor, cs.Accent)
	}
}

func TestAgentMessageRendererAgentListAfterAdding(t *testing.T) {
	th := theme.Current()
	amr := NewAgentMessageRenderer(th)

	initialCount := len(amr.GetAgentList())

	amr.SetAgentColor("new-agent", "ffffff")

	newCount := len(amr.GetAgentList())

	if newCount <= initialCount {
		t.Error("Agent list should grow after adding")
	}
}

func TestAgentMessageRendererCaseSensitiveColorSet(t *testing.T) {
	th := theme.Current()
	amr := NewAgentMessageRenderer(th)

	amr.SetAgentColor("MyAgent", "111111")

	// Should be retrievable with different cases
	if amr.GetAgentColor("myagent") != "111111" {
		t.Error("Should retrieve color regardless of case")
	}

	if amr.GetAgentColor("MYAGENT") != "111111" {
		t.Error("Should retrieve color with uppercase")
	}

	if amr.GetAgentColor("MyAgent") != "111111" {
		t.Error("Should retrieve color with original case")
	}
}

func TestAgentMessageRendererEmptyContentStyling(t *testing.T) {
	th := theme.Current()
	amr := NewAgentMessageRenderer(th)

	msg := message.UserMessage("")
	styled := amr.ApplyAgentStyling(&msg, "")

	// Should handle empty content gracefully
	if styled == "" {
		t.Error("Should return styled output even with empty content")
	}
}

func TestAgentMessageRendererStyledContentNotEmpty(t *testing.T) {
	th := theme.Current()
	amr := NewAgentMessageRenderer(th)

	msg := message.Message{
		Role:    message.RoleAssistant,
		Content: []message.ContentBlock{{Type: message.ContentText, Text: "content"}},
	}

	styled := amr.StyleMessageBubble(&msg, "test")

	if styled == "" {
		t.Error("Styled message bubble should not be empty")
	}
}
