package components

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/projectbarks/gopher-code/pkg/message"
	"github.com/projectbarks/gopher-code/pkg/ui/theme"
)

// AgentMessageRenderer colors messages based on agent identity
type AgentMessageRenderer struct {
	th          theme.Theme
	agentColors map[string]string // Maps agent ID → color
}

// NewAgentMessageRenderer creates a new agent message renderer
func NewAgentMessageRenderer(th theme.Theme) *AgentMessageRenderer {
	amr := &AgentMessageRenderer{
		th:          th,
		agentColors: make(map[string]string),
	}
	amr.initializeDefaultColors()
	return amr
}

// initializeDefaultColors sets up default color mappings
func (amr *AgentMessageRenderer) initializeDefaultColors() {
	cs := amr.th.Colors()

	// Default agent colors
	amr.agentColors["user"] = cs.Primary       // Blue for user messages
	amr.agentColors["assistant"] = cs.Accent  // Cyan for assistant messages
	amr.agentColors["system"] = cs.Warning    // Orange for system messages
	amr.agentColors["tool"] = cs.Info         // Green for tool messages
	amr.agentColors["error"] = cs.Error       // Red for error messages

	// Common agent names
	amr.agentColors["claude"] = cs.Accent
	amr.agentColors["gpt"] = cs.Primary
	amr.agentColors["user-agent"] = cs.Primary
	amr.agentColors["system-agent"] = cs.Warning
}

// SetAgentColor sets a custom color for an agent
func (amr *AgentMessageRenderer) SetAgentColor(agentID, color string) {
	amr.agentColors[strings.ToLower(agentID)] = color
}

// GetAgentColor returns the color for a given agent
func (amr *AgentMessageRenderer) GetAgentColor(agentID string) string {
	agentID = strings.ToLower(agentID)

	// Check exact match
	if color, exists := amr.agentColors[agentID]; exists {
		return color
	}

	// Fallback based on message role
	switch agentID {
	case "user":
		return amr.agentColors["user"]
	case "assistant":
		return amr.agentColors["assistant"]
	default:
		// Default to assistant color for unknown agents
		return amr.agentColors["assistant"]
	}
}

// GetAgentColorForMessage returns the color for a message's agent
func (amr *AgentMessageRenderer) GetAgentColorForMessage(msg *message.Message) string {
	// Map message role to agent ID
	agentID := string(msg.Role)
	return amr.GetAgentColor(agentID)
}

// ApplyAgentStyling applies agent-specific styling to content
func (amr *AgentMessageRenderer) ApplyAgentStyling(msg *message.Message, content string) string {
	color := amr.GetAgentColorForMessage(msg)

	// Create styled content with agent color border
	style := lipgloss.NewStyle().
		BorderLeft(true).
		BorderLeftForeground(lipgloss.Color(color)).
		Padding(0, 1)

	return style.Render(content)
}

// GetBorderColor returns the border color for a message
func (amr *AgentMessageRenderer) GetBorderColor(agentID string) string {
	return amr.GetAgentColor(agentID)
}

// GetBackgroundColor returns a muted background color for an agent
func (amr *AgentMessageRenderer) GetBackgroundColor(agentID string) string {
	color := amr.GetAgentColor(agentID)
	cs := amr.th.Colors()

	// Map color to muted version based on theme colors
	switch color {
	case cs.Primary:
		return cs.PrimaryMuted
	case cs.Accent:
		return cs.AccentMuted
	case cs.Warning:
		return cs.WarningMuted
	case cs.Info:
		return cs.InfoMuted
	case cs.Error:
		return cs.ErrorMuted
	default:
		return cs.Surface
	}
}

// StyleMessageBubble applies agent styling to a message bubble
func (amr *AgentMessageRenderer) StyleMessageBubble(msg *message.Message, bubbleContent string) string {
	color := amr.GetAgentColorForMessage(msg)

	// Create style based on message role
	style := lipgloss.NewStyle().
		BorderLeft(true).
		BorderLeftForeground(lipgloss.Color(color))

	// Add background color for contrast
	if msg.Role == message.RoleAssistant {
		style = style.Background(lipgloss.Color(amr.GetBackgroundColor(string(msg.Role))))
	}

	return style.Render(bubbleContent)
}

// GetAgentList returns all configured agents
func (amr *AgentMessageRenderer) GetAgentList() []string {
	agents := make([]string, 0, len(amr.agentColors))
	for agentID := range amr.agentColors {
		agents = append(agents, agentID)
	}
	return agents
}

// GetColorScheme returns the current theme colors
func (amr *AgentMessageRenderer) GetColorScheme() theme.ColorScheme {
	return amr.th.Colors()
}

// HasAgent checks if an agent is configured
func (amr *AgentMessageRenderer) HasAgent(agentID string) bool {
	_, exists := amr.agentColors[strings.ToLower(agentID)]
	return exists
}

// RemoveAgent removes an agent's custom color mapping
func (amr *AgentMessageRenderer) RemoveAgent(agentID string) {
	delete(amr.agentColors, strings.ToLower(agentID))
}

// ResetToDefaults resets color mappings to defaults
func (amr *AgentMessageRenderer) ResetToDefaults() {
	amr.agentColors = make(map[string]string)
	amr.initializeDefaultColors()
}

// CreateHighlightStyle creates a highlight style for an agent
func (amr *AgentMessageRenderer) CreateHighlightStyle(agentID string) lipgloss.Style {
	color := amr.GetAgentColor(agentID)
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(color)).
		Bold(true)
}

// CreateMutedStyle creates a muted style for an agent
func (amr *AgentMessageRenderer) CreateMutedStyle(agentID string) lipgloss.Style {
	agentID = strings.ToLower(agentID)
	color := amr.GetAgentColor(agentID)
	cs := amr.th.Colors()

	// Use muted version of the color
	var mutedColor string
	switch color {
	case cs.Primary:
		mutedColor = cs.PrimaryMuted
	case cs.Accent:
		mutedColor = cs.AccentMuted
	default:
		mutedColor = cs.TextSecondary
	}

	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(mutedColor))
}

// CreateBadgeStyle creates a badge style for displaying agent name
func (amr *AgentMessageRenderer) CreateBadgeStyle(agentID string) lipgloss.Style {
	color := amr.GetAgentColor(agentID)
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color("255")).
		Background(lipgloss.Color(color)).
		Padding(0, 1).
		Bold(true)
}
