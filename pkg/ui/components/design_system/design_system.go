// Package design_system provides shared UI primitives for the TUI.
// Source: components/design-system/ (16 files)
//
// In TS these are React components. In Go they're lipgloss render functions
// and bubbletea message types.
package design_system

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
)

// --- Pane ---
// A region bounded by a colored top border line with padding.
// Source: design-system/Pane.tsx

// RenderPane wraps content in a pane with a colored top border.
func RenderPane(content string, borderColor string, width int) string {
	border := lipgloss.NewStyle().
		Foreground(lipgloss.Color(borderColor)).
		Render(strings.Repeat("─", width))
	return border + "\n" + content
}

// --- Dialog ---
// A bordered dialog with title, content, and cancel/confirm footer.
// Source: design-system/Dialog.tsx

// RenderDialog wraps content in a titled dialog frame.
func RenderDialog(title, content string, width int) string {
	titleStyle := lipgloss.NewStyle().Bold(true)
	frameStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(1, 2).
		Width(width)

	header := titleStyle.Render(title)
	return frameStyle.Render(header + "\n\n" + content)
}

// --- Divider ---
// A horizontal line separator.
// Source: design-system/Divider.tsx

// RenderDivider renders a horizontal divider line.
func RenderDivider(width int, label string) string {
	style := lipgloss.NewStyle().Faint(true)
	if label == "" {
		return style.Render(strings.Repeat("─", width))
	}
	// Label in middle
	pad := (width - len(label) - 2) / 2
	if pad < 0 {
		pad = 0
	}
	return style.Render(strings.Repeat("─", pad) + " " + label + " " + strings.Repeat("─", pad))
}

// --- Badge ---
// An inline colored label.

// RenderBadge renders a colored inline badge.
func RenderBadge(text, color string) string {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(color)).
		Bold(true).
		Render("[" + text + "]")
}

// --- StatusIcon ---
// Renders ✓, ✗, ⚠, ⏺ icons with appropriate coloring.
// Source: design-system/StatusIcon.tsx

// StatusType classifies status icons.
type StatusType string

const (
	StatusSuccess StatusType = "success"
	StatusError   StatusType = "error"
	StatusWarning StatusType = "warning"
	StatusInfo    StatusType = "info"
	StatusPending StatusType = "pending"
)

// RenderStatusIcon returns a colored status icon.
func RenderStatusIcon(status StatusType) string {
	switch status {
	case StatusSuccess:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Render("✓")
	case StatusError:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render("✗")
	case StatusWarning:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Render("⚠")
	case StatusInfo:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Render("ℹ")
	case StatusPending:
		return lipgloss.NewStyle().Faint(true).Render("⏺")
	default:
		return "•"
	}
}

// --- ProgressBar ---
// Source: design-system/ProgressBar.tsx

// RenderProgressBar renders a text-based progress bar.
func RenderProgressBar(percent float64, width int) string {
	if percent < 0 {
		percent = 0
	}
	if percent > 1 {
		percent = 1
	}
	filled := int(float64(width) * percent)
	empty := width - filled

	bar := strings.Repeat("█", filled) + strings.Repeat("░", empty)
	label := fmt.Sprintf(" %.0f%%", percent*100)

	return lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Render(bar) + label
}

// --- KeyboardShortcutHint ---
// Source: design-system/KeyboardShortcutHint.tsx

// RenderShortcutHint renders a keyboard shortcut hint like "Escape to cancel".
func RenderShortcutHint(key, action string) string {
	keyStyle := lipgloss.NewStyle().Bold(true)
	dimStyle := lipgloss.NewStyle().Faint(true)
	return dimStyle.Render(keyStyle.Render(key) + " " + action)
}

// --- Tabs ---
// Source: design-system/Tabs.tsx

// RenderTabs renders a horizontal tab bar with the active tab highlighted.
func RenderTabs(tabs []string, active int) string {
	activeStyle := lipgloss.NewStyle().Bold(true).Underline(true).Padding(0, 1)
	inactiveStyle := lipgloss.NewStyle().Faint(true).Padding(0, 1)

	var parts []string
	for i, tab := range tabs {
		if i == active {
			parts = append(parts, activeStyle.Render(tab))
		} else {
			parts = append(parts, inactiveStyle.Render(tab))
		}
	}
	return strings.Join(parts, " ")
}

// --- ListItem ---
// Source: design-system/ListItem.tsx

// RenderListItem renders a list item with optional icon and indent.
func RenderListItem(text, icon string, indent int) string {
	prefix := strings.Repeat("  ", indent)
	if icon != "" {
		return prefix + icon + " " + text
	}
	return prefix + "• " + text
}

// --- LoadingState ---
// Source: design-system/LoadingState.tsx

// RenderLoadingState renders a loading indicator with message.
func RenderLoadingState(message string) string {
	return lipgloss.NewStyle().Faint(true).Render("⏺ " + message + "...")
}
