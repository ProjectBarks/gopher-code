// Package spinner provides spinner variants for different UI contexts.
//
// Source: components/Spinner.tsx, components/Spinner/*.tsx, design-system/LoadingState.tsx
//
// The base ThinkingSpinner is in spinner_verbs.go (same components package parent).
// This package provides additional spinner modes and variants used across the TUI:
// tool use spinners, agent spinners, loading states, and shimmer animations.
package spinner

import (
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/projectbarks/gopher-code/pkg/ui/theme"
)

// Mode describes what the spinner is waiting for.
// Source: components/Spinner/types.ts
type Mode string

const (
	ModeThinking   Mode = "thinking"   // model is generating
	ModeToolUse    Mode = "tool_use"   // tool is executing
	ModeAgent      Mode = "agent"      // sub-agent is running
	ModeLoading    Mode = "loading"    // generic loading
	ModeProcessing Mode = "processing" // processing input
)

// Glyphs are the default spinner animation characters.
// Platform-aware — see spinner_verbs.go for the real glyph set.
var Glyphs = []string{"·", "✢", "✳", "✶", "✻", "✽"}

// GlyphFrames is the full bounce cycle: forward + reverse.
var GlyphFrames []string

func init() {
	GlyphFrames = make([]string, 0, len(Glyphs)*2)
	GlyphFrames = append(GlyphFrames, Glyphs...)
	reversed := make([]string, len(Glyphs))
	for i, g := range Glyphs {
		reversed[len(Glyphs)-1-i] = g
	}
	GlyphFrames = append(GlyphFrames, reversed...)
}

// FrameAt returns the glyph for a given animation frame index.
func FrameAt(frame int) string {
	if len(GlyphFrames) == 0 {
		return "·"
	}
	return GlyphFrames[frame%len(GlyphFrames)]
}

// ToolUseSpinner renders a spinner for tool execution.
// Shows: "✻ Running {toolName}…"
type ToolUseSpinner struct {
	ToolName string
	frame    int
	active   bool
	start    time.Time
}

// NewToolUseSpinner creates a spinner for a tool invocation.
func NewToolUseSpinner(toolName string) *ToolUseSpinner {
	return &ToolUseSpinner{
		ToolName: toolName,
		active:   true,
		start:    time.Now(),
	}
}

// Tick advances the frame.
func (s *ToolUseSpinner) Tick() { s.frame++ }

// Stop marks the spinner as complete.
func (s *ToolUseSpinner) Stop() { s.active = false }

// IsActive returns true if the spinner is running.
func (s *ToolUseSpinner) IsActive() bool { return s.active }

// Elapsed returns time since start.
func (s *ToolUseSpinner) Elapsed() time.Duration { return time.Since(s.start) }

// View renders the tool use spinner line.
func (s *ToolUseSpinner) View() string {
	colors := theme.Current().Colors()
	glyphStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(colors.Accent))
	toolStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(colors.ToolName)).Bold(true)
	dimStyle := lipgloss.NewStyle().Faint(true)

	glyph := FrameAt(s.frame)

	if s.active {
		return glyphStyle.Render(glyph) + " Running " + toolStyle.Render(s.ToolName) + "…"
	}

	secs := int(s.Elapsed().Seconds())
	if secs < 1 {
		secs = 1
	}
	return glyphStyle.Render(glyph) + " " + toolStyle.Render(s.ToolName) +
		" " + dimStyle.Render(fmt.Sprintf("(%ds)", secs))
}

// AgentSpinner renders a spinner for a sub-agent.
// Shows: "✻ Agent {agentType} working…"
type AgentSpinner struct {
	AgentType string
	Color     string // agent color name
	frame     int
	active    bool
	start     time.Time
}

// NewAgentSpinner creates a spinner for a sub-agent.
func NewAgentSpinner(agentType, color string) *AgentSpinner {
	return &AgentSpinner{
		AgentType: agentType,
		Color:     color,
		active:    true,
		start:     time.Now(),
	}
}

// Tick advances the frame.
func (s *AgentSpinner) Tick() { s.frame++ }

// Stop marks the spinner as complete.
func (s *AgentSpinner) Stop() { s.active = false }

// IsActive returns true if the spinner is running.
func (s *AgentSpinner) IsActive() bool { return s.active }

// View renders the agent spinner line.
func (s *AgentSpinner) View() string {
	colors := theme.Current().Colors()
	glyphStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(colors.Accent))
	nameStyle := lipgloss.NewStyle().Bold(true)
	if s.Color != "" {
		nameStyle = nameStyle.Foreground(lipgloss.Color(s.Color))
	}

	glyph := FrameAt(s.frame)

	if s.active {
		return glyphStyle.Render(glyph) + " Agent " + nameStyle.Render(s.AgentType) + " working…"
	}

	secs := int(time.Since(s.start).Seconds())
	if secs < 1 {
		secs = 1
	}
	dimStyle := lipgloss.NewStyle().Faint(true)
	return glyphStyle.Render(glyph) + " Agent " + nameStyle.Render(s.AgentType) +
		" " + dimStyle.Render(fmt.Sprintf("finished in %ds", secs))
}

// LoadingState renders a generic loading indicator with a message.
// Source: design-system/LoadingState.tsx
type LoadingState struct {
	Message string
	frame   int
}

// NewLoadingState creates a loading indicator.
func NewLoadingState(message string) *LoadingState {
	return &LoadingState{Message: message}
}

// Tick advances the frame.
func (s *LoadingState) Tick() { s.frame++ }

// View renders the loading state.
func (s *LoadingState) View() string {
	colors := theme.Current().Colors()
	glyphStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(colors.Spinner))
	glyph := FrameAt(s.frame)
	return glyphStyle.Render(glyph) + " " + s.Message
}

// ShimmerText creates a shimmer effect on text by highlighting one character
// at a time. Used for the input placeholder during processing.
// Source: components/Spinner/useShimmerAnimation.ts
func ShimmerText(text string, frame int, highlightColor string) string {
	runes := []rune(text)
	if len(runes) == 0 {
		return text
	}

	idx := frame % len(runes)
	highlightStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(highlightColor))

	var b strings.Builder
	for i, r := range runes {
		if i == idx {
			b.WriteString(highlightStyle.Render(string(r)))
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}
