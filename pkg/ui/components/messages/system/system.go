// Package system provides system message rendering for the TUI.
// Source: components/messages/SystemTextMessage.tsx
//
// System messages appear between user/assistant turns to convey
// status, errors, duration, memory saves, and other non-conversational info.
package system

import (
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
)

// Subtype classifies system messages for rendering.
// Source: types/message.ts — SystemMessage.subtype
type Subtype string

const (
	SubtypeTurnDuration     Subtype = "turn_duration"
	SubtypeMemorySaved      Subtype = "memory_saved"
	SubtypeAwaySummary      Subtype = "away_summary"
	SubtypeAgentsKilled     Subtype = "agents_killed"
	SubtypeThinking         Subtype = "thinking"
	SubtypeBridgeStatus     Subtype = "bridge_status"
	SubtypeScheduledFire    Subtype = "scheduled_task_fire"
	SubtypePermissionRetry  Subtype = "permission_retry"
	SubtypeStopHookSummary  Subtype = "stop_hook_summary"
	SubtypeAPIError         Subtype = "api_error"
	SubtypeText             Subtype = "text"
)

// Message is a system message with subtype-specific data.
type Message struct {
	Subtype Subtype
	Text    string

	// Subtype-specific fields (only one set per subtype)
	Duration      time.Duration // turn_duration
	MemoryPath    string        // memory_saved
	ErrorCode     string        // api_error
	AgentCount    int           // agents_killed
	HookSummaries []string     // stop_hook_summary
}

// Render returns a styled string for terminal display.
// Source: components/messages/SystemTextMessage.tsx
func Render(msg Message) string {
	switch msg.Subtype {
	case SubtypeTurnDuration:
		return renderDuration(msg)
	case SubtypeMemorySaved:
		return renderMemorySaved(msg)
	case SubtypeAPIError:
		return renderAPIError(msg)
	case SubtypeAgentsKilled:
		return renderAgentsKilled(msg)
	case SubtypeThinking:
		return renderThinking(msg)
	case SubtypeStopHookSummary:
		return renderStopHookSummary(msg)
	default:
		return renderText(msg)
	}
}

func renderDuration(msg Message) string {
	dimStyle := lipgloss.NewStyle().Faint(true)
	secs := msg.Duration.Seconds()
	if secs < 1 {
		return dimStyle.Render(fmt.Sprintf("⏺ Completed in <1s"))
	}
	return dimStyle.Render(fmt.Sprintf("⏺ Completed in %s", formatDuration(msg.Duration)))
}

func renderMemorySaved(msg Message) string {
	icon := "✦"
	text := "Memory saved"
	if msg.MemoryPath != "" {
		text += " → " + msg.MemoryPath
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Render(icon+" "+text)
}

func renderAPIError(msg Message) string {
	errStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	text := "API error"
	if msg.ErrorCode != "" {
		text += " (" + msg.ErrorCode + ")"
	}
	if msg.Text != "" {
		text += ": " + msg.Text
	}
	return errStyle.Render("✗ " + text)
}

func renderAgentsKilled(msg Message) string {
	noun := "agent"
	if msg.AgentCount != 1 {
		noun = "agents"
	}
	return lipgloss.NewStyle().Faint(true).Render(
		fmt.Sprintf("⏺ Stopped %d background %s", msg.AgentCount, noun))
}

func renderThinking(msg Message) string {
	return lipgloss.NewStyle().Faint(true).Italic(true).Render("💭 " + msg.Text)
}

func renderStopHookSummary(msg Message) string {
	if len(msg.HookSummaries) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("Hook results:\n")
	for _, s := range msg.HookSummaries {
		sb.WriteString("  • " + s + "\n")
	}
	return lipgloss.NewStyle().Faint(true).Render(strings.TrimRight(sb.String(), "\n"))
}

func renderText(msg Message) string {
	return lipgloss.NewStyle().Faint(true).Render("⎿ " + msg.Text)
}

func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	m := int(d.Minutes())
	s := int(d.Seconds()) % 60
	if s == 0 {
		return fmt.Sprintf("%dm", m)
	}
	return fmt.Sprintf("%dm%ds", m, s)
}
