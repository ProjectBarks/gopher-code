package theme

import (
	"image/color"

	"charm.land/lipgloss/v2"
)

// highcontrast.go — High-contrast accessibility theme.
//
// Designed for users who need maximum contrast. Uses pure black/white
// backgrounds, bright saturated colors, and liberal use of bold and
// underline to convey state without relying solely on color (supporting
// color-blind users).
//
// Accessibility approach:
//   - All text meets WCAG AAA contrast ratio (7:1) against its background.
//   - Focus/active states use BOTH color change AND bold/underline.
//   - Semantic states (success, error, warning) use distinct hues that
//     remain distinguishable under protanopia, deuteranopia, and tritanopia.
//   - Status is also communicated via prefix icons (check, cross, warning).

func init() {
	Register(&highContrastTheme{})
}

type highContrastTheme struct{}

func (h *highContrastTheme) Name() ThemeName { return ThemeHighContrast }

func (h *highContrastTheme) Colors() ColorScheme {
	return ColorScheme{
		// Surfaces — pure black/white for maximum contrast
		Background:      Black,
		Surface:         Black,
		SurfaceElevated: Gray900,
		SurfaceOverlay:  Gray900,

		// Text — pure white on black
		TextPrimary:   White,
		TextSecondary: Gray200,
		TextMuted:     Gray300,
		TextInverse:   Black,

		// Borders — bright white borders, cyan focus
		Border:        Gray300,
		BorderFocused: Cyan300,
		BorderSubtle:  Gray700,

		// Primary action — bright, high-saturation blue
		Primary:      "#00afff",
		PrimaryHover: "#5fd7ff",
		PrimaryMuted: Blue600,

		// Accent — brightest cyan
		Accent:      Cyan300,
		AccentMuted: Cyan600,

		// Semantic — high-saturation, distinguishable under color blindness
		Success:      "#00ff87", // Bright green — distinct hue from error
		SuccessMuted: Green700,
		Warning:      "#ffff00", // Pure yellow — high visibility
		WarningMuted: Yellow700,
		Error:        "#ff5f5f", // Bright red
		ErrorMuted:   Red700,
		Info:         "#87afff", // Periwinkle blue — distinct from cyan accent
		InfoMuted:    Info700,

		// Diff
		DiffAdded:   "#00ff87",
		DiffRemoved: "#ff5f5f",
		DiffContext:  Gray300,

		// Spinner
		Spinner: Cyan300,

		// Selection
		Cursor:    White,
		Selection: Blue600,

		// Components
		ToolName:    Cyan300,
		ToolBorder:  Gray300,
		Prompt:      Cyan300,
		StatusBarBg: Gray900,
		StatusBarFg: White,
		TabActive:   Cyan300,
		TabInactive: Gray400,
	}
}

// ---------------------------------------------------------------------------
// Text styles — high contrast uses bold more aggressively
// ---------------------------------------------------------------------------

func (h *highContrastTheme) BaseStyle() lipgloss.Style {
	c := h.Colors()
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(c.TextPrimary))
}

func (h *highContrastTheme) TextPrimary() lipgloss.Style {
	c := h.Colors()
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(c.TextPrimary))
}

func (h *highContrastTheme) TextSecondary() lipgloss.Style {
	c := h.Colors()
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(c.TextSecondary))
}

func (h *highContrastTheme) TextAccent() lipgloss.Style {
	c := h.Colors()
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(c.Accent)).
		Bold(true).
		Underline(true) // Underline in addition to color for color-blind users
}

func (h *highContrastTheme) TextSuccess() lipgloss.Style {
	c := h.Colors()
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(c.Success)).
		Bold(true)
}

func (h *highContrastTheme) TextWarning() lipgloss.Style {
	c := h.Colors()
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(c.Warning)).
		Bold(true)
}

func (h *highContrastTheme) TextError() lipgloss.Style {
	c := h.Colors()
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(c.Error)).
		Bold(true).
		Underline(true) // Double emphasis for errors
}

func (h *highContrastTheme) TextInfo() lipgloss.Style {
	c := h.Colors()
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(c.Info)).
		Bold(true)
}

// ---------------------------------------------------------------------------
// Box / panel styles — thicker visual borders
// ---------------------------------------------------------------------------

func (h *highContrastTheme) Box() lipgloss.Style {
	c := h.Colors()
	return lipgloss.NewStyle().
		Border(lipgloss.ThickBorder()). // Thicker border for visibility
		BorderForeground(lipgloss.Color(c.Border)).
		Padding(DefaultSpacing.PadV, DefaultSpacing.PadH)
}

func (h *highContrastTheme) BoxFocused() lipgloss.Style {
	c := h.Colors()
	return lipgloss.NewStyle().
		Border(lipgloss.DoubleBorder()). // Double border for focused state
		BorderForeground(lipgloss.Color(c.BorderFocused)).
		Padding(DefaultSpacing.PadV, DefaultSpacing.PadH)
}

func (h *highContrastTheme) Panel() lipgloss.Style {
	return lipgloss.NewStyle().
		Padding(DefaultSpacing.PadV, DefaultSpacing.PadH)
}

// ---------------------------------------------------------------------------
// Component-specific styles
// ---------------------------------------------------------------------------

func (h *highContrastTheme) ToolCallHeader() lipgloss.Style {
	c := h.Colors()
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(c.ToolName)).
		Bold(true).
		Underline(true)
}

func (h *highContrastTheme) ToolResultSuccess() lipgloss.Style {
	c := h.Colors()
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(c.Success)).
		Bold(true)
}

func (h *highContrastTheme) ToolResultError() lipgloss.Style {
	c := h.Colors()
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(c.Error)).
		Bold(true).
		Underline(true)
}

func (h *highContrastTheme) StatusBar() lipgloss.Style {
	c := h.Colors()
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(c.StatusBarFg)).
		Background(lipgloss.Color(c.StatusBarBg)).
		Bold(true).
		Padding(0, CompactSpacing.PadH)
}

func (h *highContrastTheme) PromptChar() lipgloss.Style {
	c := h.Colors()
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(c.Prompt)).
		Bold(true)
}

func (h *highContrastTheme) SpinnerStyle() lipgloss.Style {
	c := h.Colors()
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(c.Spinner)).
		Bold(true)
}

func (h *highContrastTheme) DiffAdded() lipgloss.Style {
	c := h.Colors()
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(c.DiffAdded)).
		Bold(true)
}

func (h *highContrastTheme) DiffRemoved() lipgloss.Style {
	c := h.Colors()
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(c.DiffRemoved)).
		Bold(true)
}

// ---------------------------------------------------------------------------
// Focus helpers
// ---------------------------------------------------------------------------

func (h *highContrastTheme) BorderColor(focused bool) color.Color {
	c := h.Colors()
	if focused {
		return lipgloss.Color(c.BorderFocused)
	}
	return lipgloss.Color(c.Border)
}

func (h *highContrastTheme) AgentColor(name string) color.Color {
	if hex, ok := AgentColorMap[name]; ok {
		return lipgloss.Color(hex)
	}
	return lipgloss.Color(White)
}
