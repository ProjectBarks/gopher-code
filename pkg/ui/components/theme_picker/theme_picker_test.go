package theme_picker

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/projectbarks/gopher-code/pkg/ui/theme"
)

func TestNew(t *testing.T) {
	m := New(theme.ThemeDark)
	if m.cursor != 0 {
		t.Errorf("cursor = %d, want 0 (dark is first)", m.cursor)
	}
	if m.current != theme.ThemeDark {
		t.Errorf("current = %q", m.current)
	}
}

func TestNew_LightCurrent(t *testing.T) {
	m := New(theme.ThemeLight)
	if m.cursor != 1 {
		t.Errorf("cursor = %d, want 1 (light is second)", m.cursor)
	}
}

func TestNew_HighContrastCurrent(t *testing.T) {
	m := New(theme.ThemeHighContrast)
	if m.cursor != 2 {
		t.Errorf("cursor = %d, want 2 (high contrast is third)", m.cursor)
	}
}

func TestDefaultOptions(t *testing.T) {
	opts := DefaultOptions()
	if len(opts) != 3 {
		t.Fatalf("expected 3 options, got %d", len(opts))
	}
	if opts[0].Name != theme.ThemeDark {
		t.Error("first should be dark")
	}
	if opts[1].Name != theme.ThemeLight {
		t.Error("second should be light")
	}
	if opts[2].Name != theme.ThemeHighContrast {
		t.Error("third should be high-contrast")
	}
}

func TestModel_Navigation(t *testing.T) {
	m := New(theme.ThemeDark)

	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if m.cursor != 1 {
		t.Errorf("cursor = %d, want 1", m.cursor)
	}

	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if m.cursor != 2 {
		t.Errorf("cursor = %d, want 2", m.cursor)
	}

	// Can't go past end
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if m.cursor != 2 {
		t.Error("should not go past end")
	}

	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	if m.cursor != 1 {
		t.Errorf("cursor = %d, want 1", m.cursor)
	}
}

func TestModel_Select(t *testing.T) {
	m := New(theme.ThemeDark)
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown}) // light

	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter should return cmd")
	}
	msg := cmd()
	sel, ok := msg.(ThemeSelectedMsg)
	if !ok {
		t.Fatalf("expected ThemeSelectedMsg, got %T", msg)
	}
	if sel.Theme != theme.ThemeLight {
		t.Errorf("selected = %q, want light", sel.Theme)
	}
}

func TestModel_Cancel(t *testing.T) {
	m := New(theme.ThemeDark)
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if cmd == nil {
		t.Fatal("Esc should return cmd")
	}
	msg := cmd()
	if _, ok := msg.(CancelledMsg); !ok {
		t.Fatalf("expected CancelledMsg, got %T", msg)
	}
}

func TestModel_View(t *testing.T) {
	m := New(theme.ThemeDark)
	v := m.View()

	if !strings.Contains(v, "Dark") {
		t.Error("should show Dark option")
	}
	if !strings.Contains(v, "Light") {
		t.Error("should show Light option")
	}
	if !strings.Contains(v, "High Contrast") {
		t.Error("should show High Contrast option")
	}
	if !strings.Contains(v, "(current)") {
		t.Error("should mark current theme")
	}
}

func TestModel_View_IntroText(t *testing.T) {
	m := New(theme.ThemeDark).WithIntroText(true)
	v := m.View()
	if !strings.Contains(v, "Choose your theme") {
		t.Error("intro text should show 'Choose your theme'")
	}
}

func TestModel_View_HelpText(t *testing.T) {
	m := New(theme.ThemeDark).WithHelpText("Custom help")
	v := m.View()
	if !strings.Contains(v, "Custom help") {
		t.Error("should show custom help text")
	}
}

func TestSelectedTheme(t *testing.T) {
	m := New(theme.ThemeDark)
	if m.SelectedTheme() != theme.ThemeDark {
		t.Error("should return dark initially")
	}

	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if m.SelectedTheme() != theme.ThemeLight {
		t.Error("should return light after down")
	}
}

func TestModel_VimKeys(t *testing.T) {
	m := New(theme.ThemeDark)
	m, _ = m.Update(tea.KeyPressMsg{Code: 'j'})
	if m.cursor != 1 {
		t.Error("j should move down")
	}
	m, _ = m.Update(tea.KeyPressMsg{Code: 'k'})
	if m.cursor != 0 {
		t.Error("k should move up")
	}
}
