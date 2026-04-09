// Package messages provides message rendering types shared across message subtypes.
// Source: components/messages/ — CompactBoundaryMessage, AttachmentMessage,
//         RateLimitMessage, ShutdownMessage, HookProgressMessage
package messages

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
)

// MessageType classifies messages for rendering.
type MessageType string

const (
	TypeUser          MessageType = "user"
	TypeAssistant     MessageType = "assistant"
	TypeSystem        MessageType = "system"
	TypeTool          MessageType = "tool"
	TypeError         MessageType = "error"
	TypeCompact       MessageType = "compact_boundary"
	TypeAttachment    MessageType = "attachment"
	TypeRateLimit     MessageType = "rate_limit"
	TypeShutdown      MessageType = "shutdown"
	TypeHookProgress  MessageType = "hook_progress"
	TypePlanApproval  MessageType = "plan_approval"
)

// RenderCompactBoundary renders the compaction boundary marker.
// Source: components/messages/CompactBoundaryMessage.tsx
func RenderCompactBoundary() string {
	style := lipgloss.NewStyle().Faint(true)
	return style.Render("✻ Conversation compacted (ctrl+o for history)")
}

// RenderAttachment renders an attachment message (file, image, etc.)
// Source: components/messages/AttachmentMessage.tsx
func RenderAttachment(filename, mediaType string, size int) string {
	icon := "📎"
	if strings.HasPrefix(mediaType, "image/") {
		icon = "🖼"
	}
	sizeStr := formatSize(size)
	return fmt.Sprintf("%s %s (%s)", icon, filename, sizeStr)
}

// RenderRateLimit renders a rate limit warning.
// Source: components/messages/RateLimitMessage.tsx
func RenderRateLimit(retryAfterSecs int) string {
	warnStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
	msg := "Rate limit reached."
	if retryAfterSecs > 0 {
		msg += fmt.Sprintf(" Retrying in %ds...", retryAfterSecs)
	} else {
		msg += " Waiting for capacity..."
	}
	return warnStyle.Render("⚠ " + msg)
}

// RenderShutdown renders a session shutdown message.
// Source: components/messages/ShutdownMessage.tsx
func RenderShutdown(reason string) string {
	style := lipgloss.NewStyle().Faint(true)
	if reason == "" {
		return style.Render("Session ended")
	}
	return style.Render("Session ended: " + reason)
}

// RenderHookProgress renders a hook execution progress indicator.
// Source: components/messages/HookProgressMessage.tsx
func RenderHookProgress(hookName string, isRunning bool) string {
	if isRunning {
		return lipgloss.NewStyle().Faint(true).Render("⏺ Running hook: " + hookName + "...")
	}
	return lipgloss.NewStyle().Faint(true).Render("✓ Hook completed: " + hookName)
}

// RenderPlanApproval renders a plan mode approval prompt.
// Source: components/messages/PlanApprovalMessage.tsx
func RenderPlanApproval(planSummary string) string {
	titleStyle := lipgloss.NewStyle().Bold(true)
	var sb strings.Builder
	sb.WriteString(titleStyle.Render("Plan requires approval"))
	sb.WriteString("\n\n")
	sb.WriteString(planSummary)
	sb.WriteString("\n\n")
	sb.WriteString(lipgloss.NewStyle().Faint(true).Render("Press Enter to approve, Escape to reject"))
	return sb.String()
}

func formatSize(bytes int) string {
	if bytes < 1024 {
		return fmt.Sprintf("%d B", bytes)
	}
	if bytes < 1024*1024 {
		return fmt.Sprintf("%.1f KB", float64(bytes)/1024)
	}
	return fmt.Sprintf("%.1f MB", float64(bytes)/(1024*1024))
}
