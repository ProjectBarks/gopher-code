package theme

import (
	"image/color"

	"charm.land/lipgloss/v2"
)

// light.go — Light theme for users who prefer light terminal backgrounds.
//
// White/light gray backgrounds with deeper blue accents. Contrast ratios are
// tuned so text remains legible on light surfaces.

func init() {
	Register(&lightTheme{})
}

type lightTheme struct{}

func (l *lightTheme) Name() ThemeName { return ThemeLight }

func (l *lightTheme) Colors() ColorScheme {
	return ColorScheme{
		// Surfaces
		Background:      White,
		Surface:         Gray50,
		SurfaceElevated: White,
		SurfaceOverlay:  White,

		// Text
		TextPrimary:   Gray950,
		TextSecondary: Gray600,
		TextMuted:     Gray400,
		TextInverse:   White,

		// Borders
		Border:        Gray200,
		BorderFocused: Blue400,
		BorderSubtle:  Gray100,

		// Primary action
		Primary:      Blue400,
		PrimaryHover: Blue300,
		PrimaryMuted: Blue50,

		// Accent
		Accent:      Cyan700,
		AccentMuted: Cyan50,

		// Semantic
		Success:      Green600,
		SuccessMuted: Green100,
		Warning:      Yellow600,
		WarningMuted: Yellow100,
		Error:        Red600,
		ErrorMuted:   Red100,
		Info:         Info600,
		InfoMuted:    Info100,

		// Diff
		DiffAdded:   Green600,
		DiffRemoved: Red600,
		DiffContext:  Gray600,

		// Spinner
		Spinner: Blue400,

		// Selection
		Cursor:    Blue400,
		Selection: Blue50,

		// Components
		ToolName:    Blue400,
		ToolBorder:  Gray200,
		Prompt:      Blue400,
		StatusBarBg: Gray50,
		StatusBarFg: Gray600,
		TabActive:   Blue400,
		TabInactive: Gray400,
	}
}

// ---------------------------------------------------------------------------
// Text styles
// ---------------------------------------------------------------------------

func (l *lightTheme) BaseStyle() lipgloss.Style {
	c := l.Colors()
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(c.TextPrimary))
}

func (l *lightTheme) TextPrimary() lipgloss.Style {
	c := l.Colors()
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(c.TextPrimary))
}

func (l *lightTheme) TextSecondary() lipgloss.Style {
	c := l.Colors()
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(c.TextSecondary))
}

func (l *lightTheme) TextAccent() lipgloss.Style {
	c := l.Colors()
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(c.Accent)).
		Bold(true)
}

func (l *lightTheme) TextSuccess() lipgloss.Style {
	c := l.Colors()
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(c.Success))
}

func (l *lightTheme) TextWarning() lipgloss.Style {
	c := l.Colors()
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(c.Warning))
}

func (l *lightTheme) TextError() lipgloss.Style {
	c := l.Colors()
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(c.Error)).
		Bold(true)
}

func (l *lightTheme) TextInfo() lipgloss.Style {
	c := l.Colors()
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(c.Info))
}

// ---------------------------------------------------------------------------
// Box / panel styles
// ---------------------------------------------------------------------------

func (l *lightTheme) Box() lipgloss.Style {
	c := l.Colors()
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(c.Border)).
		Padding(DefaultSpacing.PadV, DefaultSpacing.PadH)
}

func (l *lightTheme) BoxFocused() lipgloss.Style {
	c := l.Colors()
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(c.BorderFocused)).
		Padding(DefaultSpacing.PadV, DefaultSpacing.PadH)
}

func (l *lightTheme) Panel() lipgloss.Style {
	return lipgloss.NewStyle().
		Padding(DefaultSpacing.PadV, DefaultSpacing.PadH)
}

// ---------------------------------------------------------------------------
// Component-specific styles
// ---------------------------------------------------------------------------

func (l *lightTheme) ToolCallHeader() lipgloss.Style {
	c := l.Colors()
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(c.ToolName)).
		Bold(true)
}

func (l *lightTheme) ToolResultSuccess() lipgloss.Style {
	c := l.Colors()
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(c.Success))
}

func (l *lightTheme) ToolResultError() lipgloss.Style {
	c := l.Colors()
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(c.Error)).
		Bold(true)
}

func (l *lightTheme) StatusBar() lipgloss.Style {
	c := l.Colors()
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(c.StatusBarFg)).
		Background(lipgloss.Color(c.StatusBarBg)).
		Padding(0, CompactSpacing.PadH)
}

func (l *lightTheme) PromptChar() lipgloss.Style {
	c := l.Colors()
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(c.Prompt)).
		Bold(true)
}

func (l *lightTheme) SpinnerStyle() lipgloss.Style {
	c := l.Colors()
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(c.Spinner))
}

func (l *lightTheme) DiffAdded() lipgloss.Style {
	c := l.Colors()
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(c.DiffAdded))
}

func (l *lightTheme) DiffRemoved() lipgloss.Style {
	c := l.Colors()
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(c.DiffRemoved))
}

// ---------------------------------------------------------------------------
// Focus helpers
// ---------------------------------------------------------------------------

func (l *lightTheme) BorderColor(focused bool) color.Color {
	c := l.Colors()
	if focused {
		return lipgloss.Color(c.BorderFocused)
	}
	return lipgloss.Color(c.Border)
}

func (l *lightTheme) AgentColor(name string) color.Color {
	if hex, ok := AgentColorMap[name]; ok {
		return lipgloss.Color(hex)
	}
	return lipgloss.Color(Gray600)
}
