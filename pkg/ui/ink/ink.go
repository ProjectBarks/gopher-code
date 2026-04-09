// Package ink provides Go equivalents of Ink's React components.
//
// Source: ink/components/Box.tsx, Text.tsx, Link.tsx, Button.tsx,
//         Spacer.tsx, Newline.tsx, NoSelect.tsx, RawAnsi.tsx,
//         AlternateScreen.tsx
//
// In TS, Ink uses React components (Box, Text, Link) for terminal UI.
// In Go, these map to lipgloss styling functions and string builders.
// This package provides the rendering equivalents.
package ink

import (
	"fmt"
	"os"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/projectbarks/gopher-code/pkg/ui/termio"
	"github.com/projectbarks/gopher-code/pkg/ui/theme"
)

// ---------------------------------------------------------------------------
// Link — Source: ink/components/Link.tsx
// ---------------------------------------------------------------------------

// Link renders a clickable hyperlink if the terminal supports it,
// otherwise falls back to plain text.
func Link(url, text string) string {
	if text == "" {
		text = url
	}
	if SupportsHyperlinks() {
		return termio.Hyperlink(url, text)
	}
	return text
}

// LinkWithFallback renders a hyperlink or fallback text.
func LinkWithFallback(url, text, fallback string) string {
	if SupportsHyperlinks() {
		if text == "" {
			text = url
		}
		return termio.Hyperlink(url, text)
	}
	if fallback != "" {
		return fallback
	}
	if text != "" {
		return text
	}
	return url
}

// SupportsHyperlinks checks if the terminal supports OSC 8 hyperlinks.
func SupportsHyperlinks() bool {
	term := os.Getenv("TERM_PROGRAM")
	switch term {
	case "iTerm.app", "WezTerm", "WarpTerminal":
		return true
	}
	if os.Getenv("TERM") == "xterm-ghostty" || os.Getenv("TERM") == "xterm-kitty" {
		return true
	}
	// VS Code terminal supports hyperlinks
	if term == "vscode" || term == "cursor" || term == "windsurf" {
		return true
	}
	return false
}

// ---------------------------------------------------------------------------
// Button — Source: ink/components/Button.tsx
// ---------------------------------------------------------------------------

// ButtonState describes the interactive state of a button.
type ButtonState struct {
	Focused bool
	Hovered bool
	Active  bool
}

// RenderButton renders a button with state-dependent styling.
func RenderButton(label string, state ButtonState) string {
	colors := theme.Current().Colors()

	style := lipgloss.NewStyle().
		Padding(0, 1).
		Border(lipgloss.RoundedBorder())

	if state.Active {
		style = style.
			Foreground(lipgloss.Color(colors.TextInverse)).
			Background(lipgloss.Color(colors.Primary)).
			BorderForeground(lipgloss.Color(colors.Primary))
	} else if state.Focused {
		style = style.
			Foreground(lipgloss.Color(colors.Primary)).
			BorderForeground(lipgloss.Color(colors.BorderFocused))
	} else {
		style = style.
			Foreground(lipgloss.Color(colors.TextPrimary)).
			BorderForeground(lipgloss.Color(colors.Border))
	}

	return style.Render(label)
}

// RenderTextButton renders a minimal text button (no border).
func RenderTextButton(label string, focused bool) string {
	colors := theme.Current().Colors()
	if focused {
		return lipgloss.NewStyle().
			Foreground(lipgloss.Color(colors.Primary)).
			Bold(true).
			Render(label)
	}
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(colors.TextSecondary)).
		Render(label)
}

// ---------------------------------------------------------------------------
// Spacer — Source: ink/components/Spacer.tsx
// ---------------------------------------------------------------------------

// Spacer returns empty space that fills available width.
// In Ink, Spacer uses flexGrow. In Go, we just return spaces.
func Spacer(width int) string {
	if width <= 0 {
		return ""
	}
	return strings.Repeat(" ", width)
}

// ---------------------------------------------------------------------------
// Newline — Source: ink/components/Newline.tsx
// ---------------------------------------------------------------------------

// Newline returns N newline characters.
func Newline(count int) string {
	if count <= 0 {
		count = 1
	}
	return strings.Repeat("\n", count)
}

// ---------------------------------------------------------------------------
// RawAnsi — Source: ink/components/RawAnsi.tsx
// ---------------------------------------------------------------------------

// RawAnsi passes through raw ANSI escape sequences unchanged.
// In Ink, this is a special component that bypasses text processing.
// In Go, we just return the string as-is.
func RawAnsi(s string) string {
	return s
}

// ---------------------------------------------------------------------------
// AlternateScreen — Source: ink/components/AlternateScreen.tsx
// ---------------------------------------------------------------------------

// EnterAlternateScreen switches to the alternate screen buffer.
func EnterAlternateScreen() string {
	return termio.EnableAlternateScreen
}

// ExitAlternateScreen switches back to the main screen buffer.
func ExitAlternateScreen() string {
	return termio.DisableAlternateScreen
}

// ---------------------------------------------------------------------------
// Box — Source: ink/components/Box.tsx (simplified)
// In Go, Box is just lipgloss styling. These helpers provide common patterns.
// ---------------------------------------------------------------------------

// HBox renders children horizontally (flexDirection: row).
func HBox(parts ...string) string {
	return lipgloss.JoinHorizontal(lipgloss.Top, parts...)
}

// VBox renders children vertically (flexDirection: column).
func VBox(parts ...string) string {
	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

// Bordered wraps content in a rounded border.
func Bordered(content string, borderColor string) string {
	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(borderColor)).
		Padding(0, 1)
	return style.Render(content)
}

// Padded adds padding around content.
func Padded(content string, top, right, bottom, left int) string {
	style := lipgloss.NewStyle().
		PaddingTop(top).
		PaddingRight(right).
		PaddingBottom(bottom).
		PaddingLeft(left)
	return style.Render(content)
}

// ---------------------------------------------------------------------------
// Text — Source: ink/components/Text.tsx (simplified)
// In Go, Text is just lipgloss styling applied to a string.
// ---------------------------------------------------------------------------

// Bold renders text in bold.
func Bold(s string) string {
	return lipgloss.NewStyle().Bold(true).Render(s)
}

// Dim renders text with reduced intensity.
func Dim(s string) string {
	return lipgloss.NewStyle().Faint(true).Render(s)
}

// Italic renders text in italic.
func Italic(s string) string {
	return lipgloss.NewStyle().Italic(true).Render(s)
}

// Colored renders text in the specified color.
func Colored(s, color string) string {
	return lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Render(s)
}

// ThemedText renders text with a named theme color.
func ThemedText(s, themeColor string) string {
	colors := theme.Current().Colors()
	color := resolveThemeColor(themeColor, colors)
	return lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Render(s)
}

func resolveThemeColor(name string, c theme.ColorScheme) string {
	switch name {
	case "primary":
		return c.Primary
	case "accent":
		return c.Accent
	case "success":
		return c.Success
	case "warning":
		return c.Warning
	case "error":
		return c.Error
	case "info":
		return c.Info
	case "muted":
		return c.TextMuted
	case "secondary":
		return c.TextSecondary
	default:
		return name // treat as raw color
	}
}

// ---------------------------------------------------------------------------
// NoSelect — Source: ink/components/NoSelect.tsx
// Used to mark regions that shouldn't be text-selectable. In Go this is a
// no-op since we don't have Ink's selection system.
// ---------------------------------------------------------------------------

// NoSelect wraps content that should not be text-selectable.
// In the Go TUI, this is a no-op — just returns the content.
func NoSelect(s string) string {
	return s
}

// ---------------------------------------------------------------------------
// Figures — common terminal figures
// ---------------------------------------------------------------------------

// Figure constants matching the 'figures' npm package.
const (
	Tick     = "✓"
	Cross    = "✗"
	Pointer  = "❯"
	Circle   = "○"
	Bullet   = "●"
	Play     = "▶"
	Warning  = "⚠"
	Info     = "ℹ"
	Ellipsis = "…"
	Heart    = "♥"
	Star     = "★"
)

// FigureWithColor renders a figure symbol with a theme color.
func FigureWithColor(figure, themeColor string) string {
	return ThemedText(figure, themeColor)
}

// StatusFigure returns an appropriate icon for a status string.
func StatusFigure(status string) string {
	switch status {
	case "ok", "success", "pass":
		return Tick
	case "error", "fail", "failed":
		return Cross
	case "warning", "warn":
		return Warning
	case "info":
		return Info
	case "running", "active":
		return Play
	case "idle", "waiting":
		return Ellipsis
	default:
		return Bullet
	}
}

// FormatKeyValue renders a key-value pair with styled key.
func FormatKeyValue(key, value string) string {
	return fmt.Sprintf("%s %s", Bold(key+":"), value)
}
