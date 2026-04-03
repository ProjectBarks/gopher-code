package theme

import (
	"testing"
)

func TestCurrentTheme(t *testing.T) {
	theme := Current()
	if theme == nil {
		t.Fatal("Current() should return a theme")
	}
}

func TestCurrentThemeIsDark(t *testing.T) {
	// Default theme is dark
	theme := Current()
	if theme.Name() != ThemeDark {
		t.Errorf("Default theme should be dark, got %s", theme.Name())
	}
}

func TestSetTheme(t *testing.T) {
	// Switch to light
	ok := SetTheme(ThemeLight)
	if !ok {
		t.Fatal("SetTheme(light) should succeed")
	}
	theme := Current()
	if theme.Name() != ThemeLight {
		t.Errorf("Expected light theme, got %s", theme.Name())
	}

	// Switch to high contrast
	ok = SetTheme(ThemeHighContrast)
	if !ok {
		t.Fatal("SetTheme(high-contrast) should succeed")
	}
	theme = Current()
	if theme.Name() != ThemeHighContrast {
		t.Errorf("Expected high-contrast theme, got %s", theme.Name())
	}

	// Switch back to dark for other tests
	SetTheme(ThemeDark)
}

func TestSetThemeInvalid(t *testing.T) {
	ok := SetTheme("nonexistent")
	if ok {
		t.Error("SetTheme with invalid name should return false")
	}
}

func TestListThemes(t *testing.T) {
	themes := ListThemes()
	if len(themes) < 3 {
		t.Errorf("Expected at least 3 themes, got %d", len(themes))
	}
	// Check all expected themes are present
	found := map[ThemeName]bool{}
	for _, name := range themes {
		found[name] = true
	}
	if !found[ThemeDark] {
		t.Error("Missing dark theme")
	}
	if !found[ThemeLight] {
		t.Error("Missing light theme")
	}
	if !found[ThemeHighContrast] {
		t.Error("Missing high-contrast theme")
	}
}

func TestColorScheme(t *testing.T) {
	cs := C()
	// Verify key colors are non-empty strings
	if cs.TextPrimary == "" {
		t.Error("TextPrimary color should not be empty")
	}
	if cs.Background == "" {
		t.Error("Background color should not be empty")
	}
	if cs.Primary == "" {
		t.Error("Primary color should not be empty")
	}
	if cs.Error == "" {
		t.Error("Error color should not be empty")
	}
}

func TestShorthandS(t *testing.T) {
	theme := S()
	if theme == nil {
		t.Fatal("S() should return current theme")
	}
	if theme != Current() {
		t.Error("S() should return same theme as Current()")
	}
}

func TestThemeStyles(t *testing.T) {
	theme := Current()

	// All style methods should return non-zero styles
	styles := []string{
		theme.TextPrimary().Render("test"),
		theme.TextSecondary().Render("test"),
		theme.TextAccent().Render("test"),
		theme.TextSuccess().Render("test"),
		theme.TextWarning().Render("test"),
		theme.TextError().Render("test"),
		theme.TextInfo().Render("test"),
		theme.Box().Render("test"),
		theme.BoxFocused().Render("test"),
		theme.Panel().Render("test"),
		theme.ToolCallHeader().Render("test"),
		theme.ToolResultSuccess().Render("test"),
		theme.ToolResultError().Render("test"),
		theme.StatusBar().Render("test"),
		theme.PromptChar().Render("test"),
		theme.SpinnerStyle().Render("test"),
		theme.DiffAdded().Render("test"),
		theme.DiffRemoved().Render("test"),
	}

	for i, s := range styles {
		if s == "" {
			t.Errorf("Style method %d returned empty string", i)
		}
	}
}

func TestThemeBorderColor(t *testing.T) {
	theme := Current()
	focused := theme.BorderColor(true)
	unfocused := theme.BorderColor(false)
	if focused == nil || unfocused == nil {
		t.Error("BorderColor should return non-nil colors")
	}
}

func TestThemeAgentColor(t *testing.T) {
	theme := Current()
	color := theme.AgentColor("assistant")
	if color == nil {
		t.Error("AgentColor should return non-nil color")
	}
}

func TestDefaultSpacing(t *testing.T) {
	if DefaultSpacing.PadH < 0 {
		t.Error("PadH should be non-negative")
	}
	if DefaultSpacing.Gap < 0 {
		t.Error("Gap should be non-negative")
	}
}

func TestCompactSpacing(t *testing.T) {
	if CompactSpacing.Gap != 0 {
		t.Error("CompactSpacing gap should be 0")
	}
}

func TestAllThemeStyles(t *testing.T) {
	// Test that all three themes can render all style methods without panicking
	themes := []ThemeName{ThemeDark, ThemeLight, ThemeHighContrast}
	for _, name := range themes {
		SetTheme(name)
		theme := Current()
		if theme == nil {
			t.Errorf("Theme %s should not be nil", name)
			continue
		}

		cs := theme.Colors()
		if cs.TextPrimary == "" {
			t.Errorf("Theme %s: TextPrimary should not be empty", name)
		}

		// Exercise every style method
		_ = theme.BaseStyle().Render("test")
		_ = theme.TextPrimary().Render("test")
		_ = theme.TextSecondary().Render("test")
		_ = theme.TextAccent().Render("test")
		_ = theme.TextSuccess().Render("test")
		_ = theme.TextWarning().Render("test")
		_ = theme.TextError().Render("test")
		_ = theme.TextInfo().Render("test")
		_ = theme.Box().Render("test")
		_ = theme.BoxFocused().Render("test")
		_ = theme.Panel().Render("test")
		_ = theme.ToolCallHeader().Render("test")
		_ = theme.ToolResultSuccess().Render("test")
		_ = theme.ToolResultError().Render("test")
		_ = theme.StatusBar().Render("test")
		_ = theme.PromptChar().Render("test")
		_ = theme.SpinnerStyle().Render("test")
		_ = theme.DiffAdded().Render("test")
		_ = theme.DiffRemoved().Render("test")
		_ = theme.BorderColor(true)
		_ = theme.BorderColor(false)
		_ = theme.AgentColor("assistant")
		_ = theme.AgentColor("user")
		_ = theme.AgentColor("unknown")
	}
	// Reset to dark
	SetTheme(ThemeDark)
}

func TestRegister(t *testing.T) {
	// ListThemes should include registered themes
	themes := ListThemes()
	if len(themes) == 0 {
		t.Error("Registry should have themes from init()")
	}
}
