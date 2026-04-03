package theme

import (
	"image/color"

	"charm.land/lipgloss/v2"
)

// dark.go — Dark theme: the primary theme for Gopher terminal UI.
//
// Deep navy backgrounds with bright cyan accents. Designed for typical
// dark terminal emulators (iTerm2, Alacritty, kitty, Windows Terminal).

func init() {
	Register(&darkTheme{})
}

type darkTheme struct{}

func (d *darkTheme) Name() ThemeName { return ThemeDark }

func (d *darkTheme) Colors() ColorScheme {
	return ColorScheme{
		// Surfaces
		Background:      Blue900,
		Surface:         Blue800,
		SurfaceElevated: Blue700,
		SurfaceOverlay:  Blue700,

		// Text
		TextPrimary:   Gray100,
		TextSecondary: Gray400,
		TextMuted:     Gray600,
		TextInverse:   Blue900,

		// Borders
		Border:        Blue600,
		BorderFocused: Cyan300,
		BorderSubtle:  Blue700,

		// Primary action
		Primary:      AccentBlue,
		PrimaryHover: Blue300,
		PrimaryMuted: Blue500,

		// Accent
		Accent:      Cyan300,
		AccentMuted: Cyan700,

		// Semantic
		Success:      Green400,
		SuccessMuted: Green700,
		Warning:      Yellow400,
		WarningMuted: Yellow700,
		Error:        Red400,
		ErrorMuted:   Red700,
		Info:         Info400,
		InfoMuted:    Info700,

		// Diff
		DiffAdded:   Green400,
		DiffRemoved: Red400,
		DiffContext:  Gray400,

		// Spinner
		Spinner: Cyan300,

		// Selection
		Cursor:    Cyan300,
		Selection: Blue500,

		// Components
		ToolName:    Cyan300,
		ToolBorder:  Blue600,
		Prompt:      Cyan300,
		StatusBarBg: Blue800,
		StatusBarFg: Gray300,
		TabActive:   Cyan300,
		TabInactive: Gray600,
	}
}

// ---------------------------------------------------------------------------
// Text styles
// ---------------------------------------------------------------------------

func (d *darkTheme) BaseStyle() lipgloss.Style {
	c := d.Colors()
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(c.TextPrimary))
}

func (d *darkTheme) TextPrimary() lipgloss.Style {
	c := d.Colors()
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(c.TextPrimary))
}

func (d *darkTheme) TextSecondary() lipgloss.Style {
	c := d.Colors()
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(c.TextSecondary))
}

func (d *darkTheme) TextAccent() lipgloss.Style {
	c := d.Colors()
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(c.Accent)).
		Bold(true)
}

func (d *darkTheme) TextSuccess() lipgloss.Style {
	c := d.Colors()
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(c.Success))
}

func (d *darkTheme) TextWarning() lipgloss.Style {
	c := d.Colors()
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(c.Warning))
}

func (d *darkTheme) TextError() lipgloss.Style {
	c := d.Colors()
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(c.Error)).
		Bold(true)
}

func (d *darkTheme) TextInfo() lipgloss.Style {
	c := d.Colors()
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(c.Info))
}

// ---------------------------------------------------------------------------
// Box / panel styles
// ---------------------------------------------------------------------------

func (d *darkTheme) Box() lipgloss.Style {
	c := d.Colors()
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(c.Border)).
		Padding(DefaultSpacing.PadV, DefaultSpacing.PadH)
}

func (d *darkTheme) BoxFocused() lipgloss.Style {
	c := d.Colors()
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(c.BorderFocused)).
		Padding(DefaultSpacing.PadV, DefaultSpacing.PadH)
}

func (d *darkTheme) Panel() lipgloss.Style {
	return lipgloss.NewStyle().
		Padding(DefaultSpacing.PadV, DefaultSpacing.PadH)
}

// ---------------------------------------------------------------------------
// Component-specific styles
// ---------------------------------------------------------------------------

func (d *darkTheme) ToolCallHeader() lipgloss.Style {
	c := d.Colors()
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(c.ToolName)).
		Bold(true)
}

func (d *darkTheme) ToolResultSuccess() lipgloss.Style {
	c := d.Colors()
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(c.Success))
}

func (d *darkTheme) ToolResultError() lipgloss.Style {
	c := d.Colors()
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(c.Error)).
		Bold(true)
}

func (d *darkTheme) StatusBar() lipgloss.Style {
	c := d.Colors()
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(c.StatusBarFg)).
		Background(lipgloss.Color(c.StatusBarBg)).
		Padding(0, CompactSpacing.PadH)
}

func (d *darkTheme) PromptChar() lipgloss.Style {
	c := d.Colors()
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(c.Prompt)).
		Bold(true)
}

func (d *darkTheme) SpinnerStyle() lipgloss.Style {
	c := d.Colors()
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(c.Spinner))
}

func (d *darkTheme) DiffAdded() lipgloss.Style {
	c := d.Colors()
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(c.DiffAdded))
}

func (d *darkTheme) DiffRemoved() lipgloss.Style {
	c := d.Colors()
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(c.DiffRemoved))
}

// ---------------------------------------------------------------------------
// Focus helpers
// ---------------------------------------------------------------------------

func (d *darkTheme) BorderColor(focused bool) color.Color {
	c := d.Colors()
	if focused {
		return lipgloss.Color(c.BorderFocused)
	}
	return lipgloss.Color(c.Border)
}

func (d *darkTheme) AgentColor(name string) color.Color {
	if hex, ok := AgentColorMap[name]; ok {
		return lipgloss.Color(hex)
	}
	return lipgloss.Color(Gray400)
}
