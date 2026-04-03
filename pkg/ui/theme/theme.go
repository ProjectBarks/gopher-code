package theme

import (
	"image/color"
	"os"
	"strings"
	"sync"

	"charm.land/lipgloss/v2"
)

// theme.go — Theme interface, component styling types, and global registry.
//
// The theme system is designed around three principles:
//   1. Semantic color roles — components reference roles (Primary, Accent, Error)
//      not raw hex values, so themes swap cleanly.
//   2. Lipgloss v2 integration — all styling helpers return lipgloss.Style values
//      that can be further composed.
//   3. Runtime switching — themes can be changed at any time via SetTheme().

// ThemeName identifies a registered theme.
type ThemeName string

const (
	ThemeDark         ThemeName = "dark"
	ThemeLight        ThemeName = "light"
	ThemeHighContrast ThemeName = "high-contrast"
)

// Theme is the interface that all themes must implement.
type Theme interface {
	// Name returns the theme identifier.
	Name() ThemeName

	// Colors returns the full color scheme for this theme.
	Colors() ColorScheme

	// --- Pre-built lipgloss styles for common components ---

	// BaseStyle returns a base style with the theme's default colors applied.
	BaseStyle() lipgloss.Style

	// --- Text styles --------------------------------------------------------

	// TextPrimary returns a style for primary body text.
	TextPrimary() lipgloss.Style
	// TextSecondary returns a style for secondary/muted text.
	TextSecondary() lipgloss.Style
	// TextAccent returns a style for accent-colored text (links, highlights).
	TextAccent() lipgloss.Style
	// TextSuccess returns a style for success text.
	TextSuccess() lipgloss.Style
	// TextWarning returns a style for warning text.
	TextWarning() lipgloss.Style
	// TextError returns a style for error text.
	TextError() lipgloss.Style
	// TextInfo returns a style for informational text.
	TextInfo() lipgloss.Style

	// --- Box / panel styles -------------------------------------------------

	// Box returns a bordered box style with optional title.
	Box() lipgloss.Style
	// BoxFocused returns a box style for the focused/active state.
	BoxFocused() lipgloss.Style
	// Panel returns a panel style (padded surface with no border).
	Panel() lipgloss.Style

	// --- Component-specific styles ------------------------------------------

	// ToolCallHeader returns the style for a tool call header line.
	ToolCallHeader() lipgloss.Style
	// ToolResultSuccess returns the style for a successful tool result.
	ToolResultSuccess() lipgloss.Style
	// ToolResultError returns the style for a failed tool result.
	ToolResultError() lipgloss.Style
	// StatusBar returns the style for the bottom status bar.
	StatusBar() lipgloss.Style
	// PromptChar returns the style for the input prompt character.
	PromptChar() lipgloss.Style
	// SpinnerStyle returns the style for spinner animations.
	SpinnerStyle() lipgloss.Style
	// DiffAdded returns the style for added diff lines.
	DiffAdded() lipgloss.Style
	// DiffRemoved returns the style for removed diff lines.
	DiffRemoved() lipgloss.Style

	// --- Focus helpers ------------------------------------------------------

	// BorderColor returns the appropriate border color for a focus state.
	BorderColor(focused bool) color.Color
	// AgentColor returns the color for a teammate agent by color name.
	AgentColor(name string) color.Color
}

// ---------------------------------------------------------------------------
// ComponentSpacing defines consistent spacing constants across components.
// ---------------------------------------------------------------------------

// Spacing holds padding/margin values used by the theme's component styles.
type Spacing struct {
	// PadH is horizontal padding inside boxes and panels.
	PadH int
	// PadV is vertical padding inside boxes and panels.
	PadV int
	// MarginH is horizontal margin outside boxes.
	MarginH int
	// MarginV is vertical margin outside boxes.
	MarginV int
	// Gap is the gap between adjacent components.
	Gap int
}

// DefaultSpacing is the standard spacing used across themes.
var DefaultSpacing = Spacing{
	PadH:    1,
	PadV:    0,
	MarginH: 0,
	MarginV: 0,
	Gap:     1,
}

// CompactSpacing is a tighter spacing for dense UIs (status bars, pills).
var CompactSpacing = Spacing{
	PadH:    1,
	PadV:    0,
	MarginH: 0,
	MarginV: 0,
	Gap:     0,
}

// ---------------------------------------------------------------------------
// Global theme registry and current theme
// ---------------------------------------------------------------------------

var (
	mu       sync.RWMutex
	current  Theme
	registry = map[ThemeName]Theme{}
)

// Register adds a theme to the global registry.
func Register(t Theme) {
	mu.Lock()
	defer mu.Unlock()
	registry[t.Name()] = t
}

// SetTheme switches the active theme. Returns false if the theme is not
// registered.
func SetTheme(name ThemeName) bool {
	mu.Lock()
	defer mu.Unlock()
	t, ok := registry[name]
	if !ok {
		return false
	}
	current = t
	return true
}

// Current returns the active theme. If none is set, it initializes from the
// GOPHER_THEME environment variable (falling back to dark).
func Current() Theme {
	mu.RLock()
	t := current
	mu.RUnlock()
	if t != nil {
		return t
	}

	// First call — initialize from environment.
	mu.Lock()
	defer mu.Unlock()

	// Double-check after acquiring write lock.
	if current != nil {
		return current
	}

	name := ThemeName(strings.ToLower(os.Getenv("GOPHER_THEME")))
	if t, ok := registry[name]; ok {
		current = t
	} else if t, ok := registry[ThemeDark]; ok {
		current = t
	}
	// If registry is empty (shouldn't happen after init), current stays nil.
	// Callers must handle nil — but in practice init() in dark.go registers it.
	return current
}

// ListThemes returns the names of all registered themes.
func ListThemes() []ThemeName {
	mu.RLock()
	defer mu.RUnlock()
	names := make([]ThemeName, 0, len(registry))
	for n := range registry {
		names = append(names, n)
	}
	return names
}

// C is a shorthand for Current().Colors() — the most common access pattern.
func C() ColorScheme {
	return Current().Colors()
}

// S is a shorthand that returns the current theme (for calling style methods).
func S() Theme {
	return Current()
}
