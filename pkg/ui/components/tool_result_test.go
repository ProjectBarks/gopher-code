package components

import (
	"strings"
	"testing"

	"github.com/projectbarks/gopher-code/pkg/ui/theme"
)

func TestToolResultDisplayCreation(t *testing.T) {
	th := theme.Current()
	trd := NewToolResultDisplay("tool-1", "search", th)

	if trd == nil {
		t.Error("Expected non-nil ToolResultDisplay")
	}
	if trd.toolUseID != "tool-1" {
		t.Errorf("Expected toolUseID 'tool-1', got %q", trd.toolUseID)
	}
	if trd.toolName != "search" {
		t.Errorf("Expected toolName 'search', got %q", trd.toolName)
	}
}

func TestToolResultDisplaySetTextContent(t *testing.T) {
	th := theme.Current()
	trd := NewToolResultDisplay("tool-1", "bash", th)

	textContent := "Command output here"
	trd.SetContent(textContent, false)

	if trd.Content() != textContent {
		t.Errorf("Expected content %q, got %q", textContent, trd.Content())
	}
	if trd.Type() != ToolResultText {
		t.Errorf("Expected type Text, got %v", trd.Type())
	}
	if trd.IsError() {
		t.Error("Expected IsError to be false")
	}
}

func TestToolResultDisplayTextRendering(t *testing.T) {
	th := theme.Current()
	trd := NewToolResultDisplay("tool-1", "bash", th)

	trd.SetContent("hello world", false)

	view := trd.View()
	if !strings.Contains(view.Content, "hello world") {
		t.Errorf("Expected 'hello world' in view, got %q", view.Content)
	}
	if !strings.Contains(view.Content, "Text Result") {
		t.Errorf("Expected 'Text Result' in view, got %q", view.Content)
	}
}

func TestToolResultDisplayJSONDetection(t *testing.T) {
	th := theme.Current()
	trd := NewToolResultDisplay("tool-1", "api", th)

	jsonContent := `{"status": "ok", "data": [1, 2, 3]}`
	trd.SetContent(jsonContent, false)

	if trd.Type() != ToolResultJSON {
		t.Errorf("Expected type JSON, got %v", trd.Type())
	}
	if trd.IsError() {
		t.Error("Expected IsError to be false")
	}
}

func TestToolResultDisplayJSONRendering(t *testing.T) {
	th := theme.Current()
	trd := NewToolResultDisplay("tool-1", "api", th)

	jsonContent := `{"status":"ok","data":[1,2,3]}`
	trd.SetContent(jsonContent, false)

	view := trd.View()
	if !strings.Contains(view.Content, "JSON Result") {
		t.Errorf("Expected 'JSON Result' in view, got %q", view.Content)
	}
	// Pretty-printed JSON should have newlines
	if !strings.Contains(view.Content, "\n") {
		t.Errorf("Expected formatted JSON with newlines, got %q", view.Content)
	}
}

func TestToolResultDisplayJSONArrayDetection(t *testing.T) {
	th := theme.Current()
	trd := NewToolResultDisplay("tool-1", "api", th)

	jsonContent := `[1, 2, 3, 4]`
	trd.SetContent(jsonContent, false)

	if trd.Type() != ToolResultJSON {
		t.Errorf("Expected type JSON for array, got %v", trd.Type())
	}
}

func TestToolResultDisplayInvalidJSONDetection(t *testing.T) {
	th := theme.Current()
	trd := NewToolResultDisplay("tool-1", "bash", th)

	// Content looks like JSON but isn't valid
	invalidJSON := `{not valid json}`
	trd.SetContent(invalidJSON, false)

	if trd.Type() != ToolResultText {
		t.Errorf("Expected type Text for invalid JSON, got %v", trd.Type())
	}
}

func TestToolResultDisplayErrorContent(t *testing.T) {
	th := theme.Current()
	trd := NewToolResultDisplay("tool-1", "bash", th)

	errorMsg := "command not found"
	trd.SetContent(errorMsg, true)

	if trd.Type() != ToolResultError {
		t.Errorf("Expected type Error, got %v", trd.Type())
	}
	if !trd.IsError() {
		t.Error("Expected IsError to be true")
	}
}

func TestToolResultDisplayErrorRendering(t *testing.T) {
	th := theme.Current()
	trd := NewToolResultDisplay("tool-1", "bash", th)

	trd.SetContent("File not found", true)

	view := trd.View()
	if !strings.Contains(view.Content, "Error") {
		t.Errorf("Expected 'Error' in view, got %q", view.Content)
	}
	if !strings.Contains(view.Content, "File not found") {
		t.Errorf("Expected error message in view, got %q", view.Content)
	}
}

func TestToolResultDisplayLongTextTruncation(t *testing.T) {
	th := theme.Current()
	trd := NewToolResultDisplay("tool-1", "bash", th)

	longText := strings.Repeat("x", 1000)
	trd.SetContent(longText, false)

	view := trd.View()
	if strings.Count(view.Content, "x") > 550 {
		t.Error("Expected long text to be truncated")
	}
}

func TestToolResultDisplayLongJSONTruncation(t *testing.T) {
	th := theme.Current()
	trd := NewToolResultDisplay("tool-1", "api", th)

	// Create a large JSON object
	jsonContent := `{"data": "` + strings.Repeat("x", 600) + `"}`
	trd.SetContent(jsonContent, false)

	view := trd.View()
	if strings.Count(view.Content, "x") > 550 {
		t.Error("Expected long JSON to be truncated")
	}
}

func TestToolResultDisplayLongErrorTruncation(t *testing.T) {
	th := theme.Current()
	trd := NewToolResultDisplay("tool-1", "bash", th)

	longError := strings.Repeat("e", 500)
	trd.SetContent(longError, true)

	view := trd.View()
	if strings.Count(view.Content, "e") > 350 {
		t.Error("Expected long error to be truncated")
	}
}

func TestToolResultDisplayEmptyContent(t *testing.T) {
	th := theme.Current()
	trd := NewToolResultDisplay("tool-1", "bash", th)

	trd.SetContent("", false)

	if trd.Content() != "" {
		t.Error("Expected empty content")
	}

	view := trd.View()
	if !strings.Contains(view.Content, "Text Result") {
		t.Error("Expected header even with empty content")
	}
}

func TestToolResultDisplayEmptyError(t *testing.T) {
	th := theme.Current()
	trd := NewToolResultDisplay("tool-1", "bash", th)

	trd.SetContent("", true)

	view := trd.View()
	if !strings.Contains(view.Content, "Error") {
		t.Error("Expected Error header for empty error")
	}
}

func TestToolResultDisplayContentAccessor(t *testing.T) {
	th := theme.Current()
	trd := NewToolResultDisplay("tool-1", "bash", th)

	testContent := "test content"
	trd.SetContent(testContent, false)

	if trd.Content() != testContent {
		t.Errorf("Expected Content() to return %q, got %q", testContent, trd.Content())
	}
}

func TestToolResultDisplayTypeAccessor(t *testing.T) {
	th := theme.Current()
	trd := NewToolResultDisplay("tool-1", "api", th)

	jsonContent := `{"key": "value"}`
	trd.SetContent(jsonContent, false)

	if trd.Type() != ToolResultJSON {
		t.Errorf("Expected Type() to return JSON, got %v", trd.Type())
	}
}

func TestToolResultDisplayIsErrorAccessor(t *testing.T) {
	th := theme.Current()
	trd := NewToolResultDisplay("tool-1", "bash", th)

	trd.SetContent("error message", true)
	if !trd.IsError() {
		t.Error("Expected IsError() to return true")
	}

	trd.SetContent("success text", false)
	if trd.IsError() {
		t.Error("Expected IsError() to return false")
	}
}

func TestToolResultDisplayInit(t *testing.T) {
	th := theme.Current()
	trd := NewToolResultDisplay("tool-1", "bash", th)

	cmd := trd.Init()
	if cmd != nil {
		t.Error("Expected Init() to return nil")
	}
}

func TestToolResultDisplaySetSize(t *testing.T) {
	th := theme.Current()
	trd := NewToolResultDisplay("tool-1", "bash", th)

	trd.SetSize(100, 24)
	if trd.width != 100 {
		t.Errorf("Expected width 100, got %d", trd.width)
	}
}

func TestToolResultDisplayUpdate(t *testing.T) {
	th := theme.Current()
	trd := NewToolResultDisplay("tool-1", "bash", th)

	trd.SetContent("test", false)

	// Update should return self with nil command
	updated, cmd := trd.Update(struct{}{})

	if cmd != nil {
		t.Error("Expected nil command")
	}
	if updated != trd {
		t.Error("Expected Update to return self")
	}
}

func TestToolResultDisplayComplexJSON(t *testing.T) {
	th := theme.Current()
	trd := NewToolResultDisplay("tool-1", "api", th)

	// Complex JSON (compact format for detection)
	trd.SetContent(`{"status":"ok","data":{"users":[{"id":1,"name":"Alice"},{"id":2,"name":"Bob"}],"count":2}}`, false)

	if trd.Type() != ToolResultJSON {
		t.Error("Expected complex JSON to be detected as JSON")
	}

	view := trd.View()
	if !strings.Contains(view.Content, "JSON Result") {
		t.Error("Expected JSON result header")
	}
}

func TestToolResultDisplayMultilineText(t *testing.T) {
	th := theme.Current()
	trd := NewToolResultDisplay("tool-1", "bash", th)

	multilineText := "Line 1\nLine 2\nLine 3"
	trd.SetContent(multilineText, false)

	view := trd.View()
	if !strings.Contains(view.Content, "Line 1") {
		t.Error("Expected 'Line 1' in view")
	}
	if !strings.Contains(view.Content, "Line 3") {
		t.Error("Expected 'Line 3' in view")
	}
}

func TestToolResultDisplayWithDifferentThemes(t *testing.T) {
	themes := []theme.ThemeName{
		theme.ThemeDark,
		theme.ThemeLight,
		theme.ThemeHighContrast,
	}

	for _, themeName := range themes {
		theme.SetTheme(themeName)
		defer theme.SetTheme(theme.ThemeDark)

		th := theme.Current()
		trd := NewToolResultDisplay("tool-1", "bash", th)

		trd.SetContent("Test output", false)
		view := trd.View()

		if !strings.Contains(view.Content, "Test output") {
			t.Errorf("Expected output in view with theme %s", themeName)
		}
	}
}

func TestToolResultDisplayViewConsistency(t *testing.T) {
	th := theme.Current()
	trd := NewToolResultDisplay("tool-1", "bash", th)

	trd.SetContent("Consistent output", false)

	view1 := trd.View()
	view2 := trd.View()

	if !strings.Contains(view1.Content, "Consistent output") {
		t.Error("Expected output in view1")
	}
	if !strings.Contains(view2.Content, "Consistent output") {
		t.Error("Expected output in view2")
	}
}

func TestToolResultDisplayToolNameInOutput(t *testing.T) {
	th := theme.Current()

	names := []string{"bash", "python", "grep", "curl"}

	for _, name := range names {
		trd := NewToolResultDisplay("tool-1", name, th)
		trd.SetContent("output", false)

		view := trd.View()
		if !strings.Contains(view.Content, name) {
			t.Errorf("Expected tool name %q in view", name)
		}
	}
}

func TestToolResultDisplayTypeChangeOnReset(t *testing.T) {
	th := theme.Current()
	trd := NewToolResultDisplay("tool-1", "api", th)

	// First set JSON
	trd.SetContent(`{"key":"value"}`, false)
	if trd.Type() != ToolResultJSON {
		t.Error("Expected JSON type initially")
	}

	// Then set text
	trd.SetContent("plain text", false)
	if trd.Type() != ToolResultText {
		t.Error("Expected Text type after change")
	}
}

func TestToolResultDisplayErrorOverridesType(t *testing.T) {
	th := theme.Current()
	trd := NewToolResultDisplay("tool-1", "api", th)

	// Set JSON content but mark as error
	trd.SetContent(`{"error":"not found"}`, true)

	// Should be Error type, not JSON
	if trd.Type() != ToolResultError {
		t.Error("Expected Error type when isError=true")
	}
}

func TestToolResultDisplayJSONWithWhitespace(t *testing.T) {
	th := theme.Current()
	trd := NewToolResultDisplay("tool-1", "api", th)

	jsonWithSpaces := `   {"key":"value"}   `
	trd.SetContent(jsonWithSpaces, false)

	if trd.Type() != ToolResultJSON {
		t.Error("Expected JSON type even with whitespace")
	}
}
