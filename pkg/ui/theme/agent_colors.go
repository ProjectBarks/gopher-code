package theme

import "sync"

// agent_colors.go — Agent color palette and round-robin index.
//
// T129: agentColorMap / agentColorIndex
// Source: bootstrap/state.ts — agentColorMap, agentColorIndex
//
// AgentColorMap (the static palette) is defined in palette.go.
// This file adds the dynamic color index that assigns colors to agents
// at the theme level, complementing session.TeammateColorManager.

// AgentColorIndex is the ordered list of agent color names, used for
// round-robin assignment. Matches the order in session.AgentColors.
var AgentColorIndex = []string{
	"red", "blue", "green", "yellow",
	"purple", "orange", "pink", "cyan",
}

// AgentColorAssigner assigns unique colors to agent IDs in round-robin order.
// Thread-safe. This is the theme-level equivalent of session.TeammateColorManager,
// suitable for UI components that need color assignment without a session reference.
// Source: bootstrap/state.ts — agentColorMap, agentColorIndex
type AgentColorAssigner struct {
	mu          sync.Mutex
	assignments map[string]string // agent ID -> color hex
	index       int
}

// NewAgentColorAssigner creates a new assigner with an empty mapping.
func NewAgentColorAssigner() *AgentColorAssigner {
	return &AgentColorAssigner{
		assignments: make(map[string]string),
	}
}

// Assign returns the hex color for an agent ID, assigning one if needed.
// Colors are assigned in round-robin order from AgentColorIndex.
func (a *AgentColorAssigner) Assign(agentID string) string {
	a.mu.Lock()
	defer a.mu.Unlock()

	if hex, ok := a.assignments[agentID]; ok {
		return hex
	}

	colorName := AgentColorIndex[a.index%len(AgentColorIndex)]
	hex := AgentColorMap[colorName]
	a.assignments[agentID] = hex
	a.index++
	return hex
}

// Get returns the hex color for an agent ID without assigning.
// Returns empty string if the agent has no assignment.
func (a *AgentColorAssigner) Get(agentID string) string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.assignments[agentID]
}

// Count returns the number of assigned agents.
func (a *AgentColorAssigner) Count() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return len(a.assignments)
}

// Clear resets all assignments.
func (a *AgentColorAssigner) Clear() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.assignments = make(map[string]string)
	a.index = 0
}
