package components

import (
	"strings"

	"charm.land/glamour/v2"
)

// Source: components/Markdown.tsx — renders markdown text with terminal styling.
// In Go, we use Glamour (Charm's markdown renderer) instead of a custom parser.

// RenderMarkdown converts markdown text to terminal-styled output.
// Uses Glamour with dark theme by default. Returns the input unchanged on error.
func RenderMarkdown(text string, width int) string {
	if strings.TrimSpace(text) == "" {
		return ""
	}
	if width < 20 {
		width = 80
	}
	r, err := glamour.NewTermRenderer(
		glamour.WithStandardStyle("dark"),
		glamour.WithWordWrap(width),
	)
	if err != nil {
		return text
	}
	out, err := r.Render(text)
	if err != nil {
		return text
	}
	return strings.TrimRight(out, "\n")
}

// RenderMarkdownDark renders markdown with explicit dark theme.
func RenderMarkdownDark(text string, width int) string {
	if strings.TrimSpace(text) == "" {
		return ""
	}
	if width < 20 {
		width = 80
	}
	r, err := glamour.NewTermRenderer(
		glamour.WithStandardStyle("dark"),
		glamour.WithWordWrap(width),
	)
	if err != nil {
		return text
	}
	out, err := r.Render(text)
	if err != nil {
		return text
	}
	return strings.TrimRight(out, "\n")
}
