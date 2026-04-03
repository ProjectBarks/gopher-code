package session

import (
	"context"
	"sync"
	"testing"
)

// Source: utils/teammateContext.ts, utils/swarm/teammateLayoutManager.ts, tools/AgentTool/agentColorManager.ts

func TestAgentColors(t *testing.T) {
	// Source: tools/AgentTool/agentColorManager.ts:14-23
	if len(AgentColors) != 8 {
		t.Errorf("expected 8 agent colors, got %d", len(AgentColors))
	}
	expected := []AgentColorName{ColorRed, ColorBlue, ColorGreen, ColorYellow, ColorPurple, ColorOrange, ColorPink, ColorCyan}
	for i, c := range expected {
		if AgentColors[i] != c {
			t.Errorf("AgentColors[%d] = %q, want %q", i, AgentColors[i], c)
		}
	}
}

func TestAgentColorConstants(t *testing.T) {
	// Source: agentColorManager.ts:3-13
	if ColorRed != "red" { t.Error("wrong") }
	if ColorBlue != "blue" { t.Error("wrong") }
	if ColorGreen != "green" { t.Error("wrong") }
	if ColorYellow != "yellow" { t.Error("wrong") }
	if ColorPurple != "purple" { t.Error("wrong") }
	if ColorOrange != "orange" { t.Error("wrong") }
	if ColorPink != "pink" { t.Error("wrong") }
	if ColorCyan != "cyan" { t.Error("wrong") }
}

func TestTeammateColorManager_RoundRobin(t *testing.T) {
	// Source: teammateLayoutManager.ts:22-33 — round-robin assignment
	m := NewTeammateColorManager()

	c1 := m.AssignColor("agent-1")
	c2 := m.AssignColor("agent-2")
	c3 := m.AssignColor("agent-3")

	if c1 != ColorRed {
		t.Errorf("first color should be red, got %q", c1)
	}
	if c2 != ColorBlue {
		t.Errorf("second color should be blue, got %q", c2)
	}
	if c3 != ColorGreen {
		t.Errorf("third color should be green, got %q", c3)
	}
}

func TestTeammateColorManager_Idempotent(t *testing.T) {
	// Source: teammateLayoutManager.ts:23-26 — same ID returns same color
	m := NewTeammateColorManager()

	c1 := m.AssignColor("agent-1")
	c2 := m.AssignColor("agent-1") // same ID

	if c1 != c2 {
		t.Errorf("same ID should get same color: %q != %q", c1, c2)
	}
	if m.Count() != 1 {
		t.Errorf("should only have 1 assignment, got %d", m.Count())
	}
}

func TestTeammateColorManager_WrapsAround(t *testing.T) {
	// Source: teammateLayoutManager.ts:28 — colorIndex % AGENT_COLORS.length
	m := NewTeammateColorManager()

	// Assign all 8 colors
	for i := 0; i < 8; i++ {
		m.AssignColor(string(rune('a' + i)))
	}

	// 9th should wrap to red
	c9 := m.AssignColor("ninth")
	if c9 != ColorRed {
		t.Errorf("9th color should wrap to red, got %q", c9)
	}
}

func TestTeammateColorManager_GetColor(t *testing.T) {
	// Source: teammateLayoutManager.ts:38-42
	m := NewTeammateColorManager()

	_, ok := m.GetColor("unknown")
	if ok {
		t.Error("should return false for unassigned")
	}

	m.AssignColor("test")
	color, ok := m.GetColor("test")
	if !ok {
		t.Error("should return true for assigned")
	}
	if color != ColorRed {
		t.Errorf("expected red, got %q", color)
	}
}

func TestTeammateColorManager_Clear(t *testing.T) {
	// Source: teammateLayoutManager.ts:48-51
	m := NewTeammateColorManager()
	m.AssignColor("a")
	m.AssignColor("b")
	if m.Count() != 2 {
		t.Fatalf("expected 2 before clear, got %d", m.Count())
	}

	m.Clear()
	if m.Count() != 0 {
		t.Errorf("expected 0 after clear, got %d", m.Count())
	}

	// After clear, should start from red again
	c := m.AssignColor("new")
	if c != ColorRed {
		t.Errorf("after clear should restart at red, got %q", c)
	}
}

func TestTeammateColorManager_Concurrent(t *testing.T) {
	m := NewTeammateColorManager()
	var wg sync.WaitGroup

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			name := string(rune('A' + id%26))
			m.AssignColor(name)
		}(i)
	}
	wg.Wait()

	// Should have assigned some colors without panic
	if m.Count() == 0 {
		t.Error("should have assignments after concurrent access")
	}
}

func TestCreateTeammateContext(t *testing.T) {
	// Source: utils/teammateContext.ts:83-96
	tc := CreateTeammateContext(
		"researcher@my-team",
		"researcher",
		"my-team",
		ColorBlue,
		true,
		"session-123",
	)

	if tc.AgentID != "researcher@my-team" {
		t.Errorf("agentId = %q", tc.AgentID)
	}
	if tc.AgentName != "researcher" {
		t.Errorf("agentName = %q", tc.AgentName)
	}
	if tc.TeamName != "my-team" {
		t.Errorf("teamName = %q", tc.TeamName)
	}
	if tc.Color != ColorBlue {
		t.Errorf("color = %q", tc.Color)
	}
	if !tc.PlanModeRequired {
		t.Error("planModeRequired should be true")
	}
	if tc.ParentSessionID != "session-123" {
		t.Errorf("parentSessionId = %q", tc.ParentSessionID)
	}
	if !tc.IsInProcess {
		t.Error("isInProcess should always be true")
	}
}

func TestTeammateContextInContext(t *testing.T) {
	// Source: utils/teammateContext.ts:47-72

	t.Run("no_context", func(t *testing.T) {
		ctx := context.Background()
		if GetTeammateContext(ctx) != nil {
			t.Error("should be nil without context")
		}
		if IsInProcessTeammate(ctx) {
			t.Error("should not be in-process without context")
		}
	})

	t.Run("with_context", func(t *testing.T) {
		tc := CreateTeammateContext("a@t", "a", "t", ColorRed, false, "s1")
		ctx := WithTeammateContext(context.Background(), tc)

		got := GetTeammateContext(ctx)
		if got == nil {
			t.Fatal("should find context")
		}
		if got.AgentID != "a@t" {
			t.Errorf("agentId = %q", got.AgentID)
		}
		if !IsInProcessTeammate(ctx) {
			t.Error("should be in-process")
		}
	})

	t.Run("child_context_inherits", func(t *testing.T) {
		tc := CreateTeammateContext("b@t", "b", "t", ColorGreen, false, "s2")
		parent := WithTeammateContext(context.Background(), tc)
		child, cancel := context.WithCancel(parent)
		defer cancel()

		got := GetTeammateContext(child)
		if got == nil || got.AgentID != "b@t" {
			t.Error("child context should inherit teammate context")
		}
	})
}

func TestAgentColorToThemeColor(t *testing.T) {
	// Source: agentColorManager.ts:25-34
	if len(AgentColorToThemeColor) != 8 {
		t.Errorf("expected 8 mappings, got %d", len(AgentColorToThemeColor))
	}
	for _, c := range AgentColors {
		theme, ok := AgentColorToThemeColor[c]
		if !ok {
			t.Errorf("missing theme mapping for %q", c)
		}
		if theme == "" {
			t.Errorf("empty theme color for %q", c)
		}
	}
	// Verify specific mapping
	if AgentColorToThemeColor[ColorRed] != "red_FOR_SUBAGENTS_ONLY" {
		t.Errorf("red mapping = %q", AgentColorToThemeColor[ColorRed])
	}
}

func TestGetAgentThemeColor(t *testing.T) {
	// Source: agentColorManager.ts:36-50

	t.Run("general_purpose_no_color", func(t *testing.T) {
		// Source: agentColorManager.ts:37-39
		m := NewTeammateColorManager()
		if GetAgentThemeColor("general-purpose", m) != "" {
			t.Error("general-purpose should have no color")
		}
	})

	t.Run("nil_manager", func(t *testing.T) {
		if GetAgentThemeColor("some-agent", nil) != "" {
			t.Error("nil manager should return empty")
		}
	})

	t.Run("assigned_color", func(t *testing.T) {
		m := NewTeammateColorManager()
		m.AssignColor("explorer")
		theme := GetAgentThemeColor("explorer", m)
		if theme != "red_FOR_SUBAGENTS_ONLY" {
			t.Errorf("expected red theme, got %q", theme)
		}
	})

	t.Run("unassigned_agent", func(t *testing.T) {
		m := NewTeammateColorManager()
		if GetAgentThemeColor("unknown", m) != "" {
			t.Error("unassigned should return empty")
		}
	})
}
