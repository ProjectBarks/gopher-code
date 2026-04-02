package cli

import (
	"charm.land/glamour/v2"
)

// Source: components — Glamour renders markdown in tool output

// RenderMarkdown renders a markdown string with terminal styling.
// Uses Glamour's environment config to detect terminal capabilities.
func RenderMarkdown(input string) (string, error) {
	r, err := glamour.NewTermRenderer(
		glamour.WithEnvironmentConfig(),
		glamour.WithWordWrap(100),
	)
	if err != nil {
		return input, err // Fall back to raw text
	}
	return r.Render(input)
}

// RenderMarkdownWithStyle renders markdown with a specific style.
func RenderMarkdownWithStyle(input, style string) (string, error) {
	r, err := glamour.NewTermRenderer(
		glamour.WithStandardStyle(style),
		glamour.WithWordWrap(100),
	)
	if err != nil {
		return input, err
	}
	return r.Render(input)
}
