package layout

import (
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/projectbarks/gopher-code/pkg/ui/theme"
)

// Source: components/FullscreenLayout.tsx
//
// FullscreenLayout splits the terminal into three vertical regions:
//   - Scrollable: transcript area (messages, tool output) — fills available space
//   - Bottom:     fixed area (prompt, spinner, permissions) — measured height
//   - Modal:      optional overlay that covers both areas
//
// The layout calculates how many rows each region gets based on the terminal
// height and the bottom content height.

// ModalTranscriptPeek is the number of transcript rows visible above a modal.
const ModalTranscriptPeek = 2

// FullscreenLayout renders a split-region terminal layout.
type FullscreenLayout struct {
	Width  int
	Height int

	// StickyHeader is an optional header pinned at the top of the scroll area.
	StickyHeader string
	// ScrollContent is the main scrollable content (messages, tool output).
	ScrollContent string
	// BottomContent is the fixed bottom area (prompt, spinner, permissions).
	BottomContent string
	// ModalContent is optional overlay content. When set, it covers most of
	// the screen, leaving ModalTranscriptPeek rows of transcript visible.
	ModalContent string
	// OverlayContent is rendered inside the scroll area after messages.
	OverlayContent string
	// StatusBar is an optional single-line status bar at the bottom.
	StatusBar string
	// NewMessagePill is an optional "N new messages" pill indicator.
	NewMessagePill string
}

// Render produces the final terminal output string.
func (l *FullscreenLayout) Render() string {
	if l.Width <= 0 || l.Height <= 0 {
		return ""
	}

	colors := theme.Current().Colors()
	dividerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(colors.BorderSubtle))

	// Calculate region heights
	bottomHeight := countLines(l.BottomContent)
	statusHeight := 0
	if l.StatusBar != "" {
		statusHeight = 1
	}
	headerHeight := 0
	if l.StickyHeader != "" {
		headerHeight = countLines(l.StickyHeader)
	}

	// Modal mode: show modal instead of normal layout
	if l.ModalContent != "" {
		return l.renderModal(dividerStyle, headerHeight, statusHeight)
	}

	// Normal mode: scroll area + bottom
	scrollHeight := l.Height - bottomHeight - statusHeight - headerHeight
	if scrollHeight < 1 {
		scrollHeight = 1
	}

	var b strings.Builder

	// Sticky header
	if l.StickyHeader != "" {
		b.WriteString(l.StickyHeader)
		b.WriteString("\n")
	}

	// Scroll content — truncate/pad to scrollHeight
	scrollLines := splitLines(l.ScrollContent)
	if l.OverlayContent != "" {
		scrollLines = append(scrollLines, splitLines(l.OverlayContent)...)
	}

	// Show bottom of scroll content (most recent messages)
	if len(scrollLines) > scrollHeight {
		scrollLines = scrollLines[len(scrollLines)-scrollHeight:]
	}
	for i, line := range scrollLines {
		b.WriteString(line)
		if i < len(scrollLines)-1 {
			b.WriteString("\n")
		}
	}
	// Pad remaining space
	for i := len(scrollLines); i < scrollHeight; i++ {
		b.WriteString("\n")
	}

	// New message pill (floating indicator)
	if l.NewMessagePill != "" {
		b.WriteString("\n")
		b.WriteString(l.NewMessagePill)
	}

	// Bottom content
	if l.BottomContent != "" {
		b.WriteString("\n")
		b.WriteString(l.BottomContent)
	}

	// Status bar
	if l.StatusBar != "" {
		b.WriteString("\n")
		b.WriteString(l.StatusBar)
	}

	return b.String()
}

// renderModal renders the modal overlay with transcript peek above.
func (l *FullscreenLayout) renderModal(dividerStyle lipgloss.Style, headerHeight, statusHeight int) string {
	modalHeight := l.Height - ModalTranscriptPeek - headerHeight - statusHeight - 1 // -1 for divider
	if modalHeight < 3 {
		modalHeight = 3
	}

	var b strings.Builder

	// Transcript peek (top N rows)
	if l.StickyHeader != "" {
		b.WriteString(l.StickyHeader)
		b.WriteString("\n")
	}
	scrollLines := splitLines(l.ScrollContent)
	peekLines := ModalTranscriptPeek
	if len(scrollLines) > peekLines {
		scrollLines = scrollLines[len(scrollLines)-peekLines:]
	}
	for _, line := range scrollLines {
		b.WriteString(line)
		b.WriteString("\n")
	}
	for i := len(scrollLines); i < peekLines; i++ {
		b.WriteString("\n")
	}

	// Divider
	divider := strings.Repeat("▔", l.Width)
	b.WriteString(dividerStyle.Render(divider))
	b.WriteString("\n")

	// Modal content — truncate to modalHeight
	modalLines := splitLines(l.ModalContent)
	if len(modalLines) > modalHeight {
		modalLines = modalLines[:modalHeight]
	}
	for i, line := range modalLines {
		b.WriteString(line)
		if i < len(modalLines)-1 {
			b.WriteString("\n")
		}
	}

	// Status bar
	if l.StatusBar != "" {
		b.WriteString("\n")
		b.WriteString(l.StatusBar)
	}

	return b.String()
}

// ScrollRegionHeight returns the available rows for the scroll content area.
func (l *FullscreenLayout) ScrollRegionHeight() int {
	bottomHeight := countLines(l.BottomContent)
	statusHeight := 0
	if l.StatusBar != "" {
		statusHeight = 1
	}
	headerHeight := 0
	if l.StickyHeader != "" {
		headerHeight = countLines(l.StickyHeader)
	}
	h := l.Height - bottomHeight - statusHeight - headerHeight
	if h < 1 {
		return 1
	}
	return h
}

// ModalRegionHeight returns available rows for modal content.
func (l *FullscreenLayout) ModalRegionHeight() int {
	headerHeight := 0
	if l.StickyHeader != "" {
		headerHeight = countLines(l.StickyHeader)
	}
	h := l.Height - ModalTranscriptPeek - headerHeight - 2 // -1 divider, -1 status
	if h < 3 {
		return 3
	}
	return h
}

func countLines(s string) int {
	if s == "" {
		return 0
	}
	return strings.Count(s, "\n") + 1
}

func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}
