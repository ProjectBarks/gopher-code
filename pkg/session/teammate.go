package session

import (
	"context"
	"sync"
)

// Source: utils/teammateContext.ts, utils/swarm/teammateLayoutManager.ts, tools/AgentTool/agentColorManager.ts

// AgentColorName represents a color that can be assigned to a teammate.
// Source: tools/AgentTool/agentColorManager.ts:3-13
type AgentColorName string

const (
	ColorRed    AgentColorName = "red"
	ColorBlue   AgentColorName = "blue"
	ColorGreen  AgentColorName = "green"
	ColorYellow AgentColorName = "yellow"
	ColorPurple AgentColorName = "purple"
	ColorOrange AgentColorName = "orange"
	ColorPink   AgentColorName = "pink"
	ColorCyan   AgentColorName = "cyan"
)

// AgentColors is the ordered palette for teammate color assignment.
// Source: tools/AgentTool/agentColorManager.ts:14-23
var AgentColors = []AgentColorName{
	ColorRed, ColorBlue, ColorGreen, ColorYellow,
	ColorPurple, ColorOrange, ColorPink, ColorCyan,
}

// TeammateContext is the runtime context for an in-process teammate.
// Source: utils/teammateContext.ts:22-39
type TeammateContext struct {
	AgentID          string         `json:"agentId"`          // e.g., "researcher@my-team"
	AgentName        string         `json:"agentName"`        // e.g., "researcher"
	TeamName         string         `json:"teamName"`
	Color            AgentColorName `json:"color,omitempty"`
	PlanModeRequired bool           `json:"planModeRequired"`
	ParentSessionID  string         `json:"parentSessionId"`
	IsInProcess      bool           `json:"isInProcess"`      // always true
}

// CreateTeammateContext creates a TeammateContext from spawn configuration.
// Source: utils/teammateContext.ts:83-96
func CreateTeammateContext(
	agentID, agentName, teamName string,
	color AgentColorName,
	planModeRequired bool,
	parentSessionID string,
) *TeammateContext {
	return &TeammateContext{
		AgentID:          agentID,
		AgentName:        agentName,
		TeamName:         teamName,
		Color:            color,
		PlanModeRequired: planModeRequired,
		ParentSessionID:  parentSessionID,
		IsInProcess:      true,
	}
}

// TeammateColorManager assigns unique colors to teammates in round-robin order.
// Source: utils/swarm/teammateLayoutManager.ts:7-51
type TeammateColorManager struct {
	mu          sync.Mutex
	assignments map[string]AgentColorName
	colorIndex  int
}

// NewTeammateColorManager creates a new color manager.
func NewTeammateColorManager() *TeammateColorManager {
	return &TeammateColorManager{
		assignments: make(map[string]AgentColorName),
	}
}

// AssignColor assigns a unique color to a teammate from the available palette.
// Colors are assigned in round-robin order. Idempotent — same ID gets same color.
// Source: utils/swarm/teammateLayoutManager.ts:22-33
func (m *TeammateColorManager) AssignColor(teammateID string) AgentColorName {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Idempotent: return existing assignment
	if existing, ok := m.assignments[teammateID]; ok {
		return existing
	}

	// Round-robin assignment
	color := AgentColors[m.colorIndex%len(AgentColors)]
	m.assignments[teammateID] = color
	m.colorIndex++

	return color
}

// GetColor returns the assigned color for a teammate, if any.
// Source: utils/swarm/teammateLayoutManager.ts:38-42
func (m *TeammateColorManager) GetColor(teammateID string) (AgentColorName, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	c, ok := m.assignments[teammateID]
	return c, ok
}

// Clear resets all color assignments.
// Source: utils/swarm/teammateLayoutManager.ts:48-51
func (m *TeammateColorManager) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.assignments = make(map[string]AgentColorName)
	m.colorIndex = 0
}

// Count returns the number of assigned colors.
func (m *TeammateColorManager) Count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.assignments)
}

// Context key for teammate context (Go equivalent of AsyncLocalStorage).
type teammateContextKey struct{}

// WithTeammateContext returns a new context with the teammate context attached.
// Source: utils/teammateContext.ts:59-64
func WithTeammateContext(ctx context.Context, tc *TeammateContext) context.Context {
	return context.WithValue(ctx, teammateContextKey{}, tc)
}

// GetTeammateContext retrieves the teammate context from a context, if present.
// Source: utils/teammateContext.ts:47-49
func GetTeammateContext(ctx context.Context) *TeammateContext {
	tc, _ := ctx.Value(teammateContextKey{}).(*TeammateContext)
	return tc
}

// IsInProcessTeammate checks if the current context is running as an in-process teammate.
// Source: utils/teammateContext.ts:70-72
func IsInProcessTeammate(ctx context.Context) bool {
	return GetTeammateContext(ctx) != nil
}

// AgentColorToThemeColor maps agent color names to theme color keys.
// Source: tools/AgentTool/agentColorManager.ts:25-34
var AgentColorToThemeColor = map[AgentColorName]string{
	ColorRed:    "red_FOR_SUBAGENTS_ONLY",
	ColorBlue:   "blue_FOR_SUBAGENTS_ONLY",
	ColorGreen:  "green_FOR_SUBAGENTS_ONLY",
	ColorYellow: "yellow_FOR_SUBAGENTS_ONLY",
	ColorPurple: "purple_FOR_SUBAGENTS_ONLY",
	ColorOrange: "orange_FOR_SUBAGENTS_ONLY",
	ColorPink:   "pink_FOR_SUBAGENTS_ONLY",
	ColorCyan:   "cyan_FOR_SUBAGENTS_ONLY",
}

// GetAgentThemeColor returns the theme color key for an agent type.
// Returns empty string for general-purpose agents (no color).
// Source: tools/AgentTool/agentColorManager.ts:36-50
func GetAgentThemeColor(agentType string, colorManager *TeammateColorManager) string {
	if agentType == "general-purpose" {
		return ""
	}
	if colorManager == nil {
		return ""
	}
	color, ok := colorManager.GetColor(agentType)
	if !ok {
		return ""
	}
	if themeColor, ok := AgentColorToThemeColor[color]; ok {
		return themeColor
	}
	return ""
}
