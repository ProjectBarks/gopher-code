package components

import (
	"charm.land/lipgloss/v2"
)

// Source: components/SearchBox.tsx

// SearchBoxConfig configures the search box appearance.
type SearchBoxConfig struct {
	Query       string
	Placeholder string // default "Search…"
	Prefix      string // default "⌕"
	IsFocused   bool
	Width       int
	Borderless  bool
}

// RenderSearchBox renders a search box as a styled string.
// This is the Go equivalent of the TS SearchBox React component.
func RenderSearchBox(cfg SearchBoxConfig) string {
	if cfg.Placeholder == "" {
		cfg.Placeholder = "Search…"
	}
	if cfg.Prefix == "" {
		cfg.Prefix = "⌕"
	}
	if cfg.Width == 0 {
		cfg.Width = 40
	}

	text := cfg.Query
	if text == "" {
		text = cfg.Placeholder
	}

	content := cfg.Prefix + " " + text

	style := lipgloss.NewStyle().Width(cfg.Width)

	if !cfg.Borderless {
		style = style.
			Border(lipgloss.RoundedBorder()).
			Padding(0, 1)
	}

	if cfg.IsFocused {
		style = style.BorderForeground(lipgloss.Color("12")) // blue
	} else {
		style = style.BorderForeground(lipgloss.Color("240")) // dim
	}

	return style.Render(content)
}
