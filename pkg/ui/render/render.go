// Package render provides rendering utilities for the TUI.
//
// Source: ink/renderer.ts, ink/render-border.ts, ink/render-to-screen.ts
//
// In TS, Ink has a full Yoga → cell buffer → diff → terminal output pipeline.
// In Go/bubbletea, rendering is string-based via View(). This package provides
// the supporting utilities: frame rate control, view composition, border
// rendering, and output formatting.
package render

import (
	"strings"
	"sync"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/projectbarks/gopher-code/pkg/ui/theme"
)

// ---------------------------------------------------------------------------
// Frame rate control — Source: ink/renderer.ts (render throttling)
// ---------------------------------------------------------------------------

// FrameRate controls render frequency to avoid excessive terminal writes.
type FrameRate struct {
	mu       sync.Mutex
	interval time.Duration
	lastRender time.Time
	dirty    bool
}

// NewFrameRate creates a frame rate controller.
// Default is ~60fps (16ms between frames).
func NewFrameRate(fps int) *FrameRate {
	if fps <= 0 {
		fps = 60
	}
	return &FrameRate{
		interval: time.Second / time.Duration(fps),
	}
}

// ShouldRender returns true if enough time has passed since the last render.
func (f *FrameRate) ShouldRender() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	now := time.Now()
	if now.Sub(f.lastRender) >= f.interval {
		f.lastRender = now
		f.dirty = false
		return true
	}
	f.dirty = true
	return false
}

// MarkDirty indicates that the view needs re-rendering.
func (f *FrameRate) MarkDirty() {
	f.mu.Lock()
	f.dirty = true
	f.mu.Unlock()
}

// IsDirty returns true if a render is pending.
func (f *FrameRate) IsDirty() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.dirty
}

// ---------------------------------------------------------------------------
// View composition — combining multiple rendered sections
// ---------------------------------------------------------------------------

// Compose joins multiple view sections vertically with optional separators.
func Compose(sections ...string) string {
	var nonEmpty []string
	for _, s := range sections {
		if s != "" {
			nonEmpty = append(nonEmpty, s)
		}
	}
	return strings.Join(nonEmpty, "\n")
}

// ComposeWithDivider joins sections with a horizontal divider between them.
func ComposeWithDivider(width int, sections ...string) string {
	colors := theme.Current().Colors()
	divider := lipgloss.NewStyle().
		Foreground(lipgloss.Color(colors.BorderSubtle)).
		Render(strings.Repeat("─", width))

	var nonEmpty []string
	for _, s := range sections {
		if s != "" {
			nonEmpty = append(nonEmpty, s)
		}
	}
	return strings.Join(nonEmpty, "\n"+divider+"\n")
}

// ---------------------------------------------------------------------------
// Border rendering — Source: ink/render-border.ts
// ---------------------------------------------------------------------------

// BorderStyle describes a border appearance.
type BorderStyle struct {
	TopLeft     string
	TopRight    string
	BottomLeft  string
	BottomRight string
	Horizontal  string
	Vertical    string
}

// DefaultBorder is the standard rounded border.
var DefaultBorder = BorderStyle{
	TopLeft:     "╭",
	TopRight:    "╮",
	BottomLeft:  "╰",
	BottomRight: "╯",
	Horizontal:  "─",
	Vertical:    "│",
}

// SharpBorder is a sharp-cornered border.
var SharpBorder = BorderStyle{
	TopLeft:     "┌",
	TopRight:    "┐",
	BottomLeft:  "└",
	BottomRight: "┘",
	Horizontal:  "─",
	Vertical:    "│",
}

// HeavyDivider is a prominent section divider.
var HeavyDivider = "▔"

// RenderBorder draws a border around content.
func RenderBorder(content string, width int, style BorderStyle, color string) string {
	borderStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(color))
	lines := strings.Split(content, "\n")

	innerWidth := width - 2
	if innerWidth < 0 {
		innerWidth = 0
	}

	var b strings.Builder

	// Top border
	b.WriteString(borderStyle.Render(style.TopLeft))
	b.WriteString(borderStyle.Render(strings.Repeat(style.Horizontal, innerWidth)))
	b.WriteString(borderStyle.Render(style.TopRight))
	b.WriteString("\n")

	// Content with side borders
	for _, line := range lines {
		b.WriteString(borderStyle.Render(style.Vertical))
		b.WriteString(padToWidth(line, innerWidth))
		b.WriteString(borderStyle.Render(style.Vertical))
		b.WriteString("\n")
	}

	// Bottom border
	b.WriteString(borderStyle.Render(style.BottomLeft))
	b.WriteString(borderStyle.Render(strings.Repeat(style.Horizontal, innerWidth)))
	b.WriteString(borderStyle.Render(style.BottomRight))

	return b.String()
}

// RenderDivider draws a horizontal divider line.
func RenderDivider(width int, char string) string {
	if char == "" {
		char = "─"
	}
	colors := theme.Current().Colors()
	style := lipgloss.NewStyle().Foreground(lipgloss.Color(colors.BorderSubtle))
	return style.Render(strings.Repeat(char, width))
}

// ---------------------------------------------------------------------------
// Output formatting
// ---------------------------------------------------------------------------

// TruncateView ensures a view fits within the terminal height.
func TruncateView(view string, maxHeight int) string {
	if maxHeight <= 0 {
		return view
	}
	lines := strings.Split(view, "\n")
	if len(lines) <= maxHeight {
		return view
	}
	return strings.Join(lines[:maxHeight], "\n")
}

// PadView ensures a view fills exactly the specified height.
func PadView(view string, targetHeight int) string {
	lines := strings.Split(view, "\n")
	for len(lines) < targetHeight {
		lines = append(lines, "")
	}
	if len(lines) > targetHeight {
		lines = lines[:targetHeight]
	}
	return strings.Join(lines, "\n")
}

// ClearAndRender returns the view with a screen clear prefix.
// Used for full-screen redraws.
func ClearAndRender(view string) string {
	return "\x1b[2J\x1b[H" + view
}

func padToWidth(s string, width int) string {
	// Simple padding — doesn't account for ANSI codes
	if len(s) >= width {
		return s[:width]
	}
	return s + strings.Repeat(" ", width-len(s))
}
