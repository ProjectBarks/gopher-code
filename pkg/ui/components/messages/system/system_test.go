package system

import (
	"strings"
	"testing"
	"time"
)

func TestRender_TurnDuration(t *testing.T) {
	got := Render(Message{Subtype: SubtypeTurnDuration, Duration: 5 * time.Second})
	if !strings.Contains(got, "5s") {
		t.Errorf("should contain '5s', got %q", got)
	}
}

func TestRender_TurnDuration_SubSecond(t *testing.T) {
	got := Render(Message{Subtype: SubtypeTurnDuration, Duration: 500 * time.Millisecond})
	if !strings.Contains(got, "<1s") {
		t.Errorf("should contain '<1s', got %q", got)
	}
}

func TestRender_MemorySaved(t *testing.T) {
	got := Render(Message{Subtype: SubtypeMemorySaved, MemoryPath: "memory/feedback_testing.md"})
	if !strings.Contains(got, "Memory saved") {
		t.Error("should contain 'Memory saved'")
	}
	if !strings.Contains(got, "feedback_testing.md") {
		t.Error("should contain file path")
	}
}

func TestRender_APIError(t *testing.T) {
	got := Render(Message{Subtype: SubtypeAPIError, ErrorCode: "429", Text: "rate limited"})
	if !strings.Contains(got, "429") {
		t.Error("should contain error code")
	}
	if !strings.Contains(got, "rate limited") {
		t.Error("should contain error text")
	}
}

func TestRender_AgentsKilled(t *testing.T) {
	got := Render(Message{Subtype: SubtypeAgentsKilled, AgentCount: 3})
	if !strings.Contains(got, "3") {
		t.Error("should contain count")
	}
	if !strings.Contains(got, "agents") {
		t.Error("should use plural")
	}
}

func TestRender_AgentsKilled_Singular(t *testing.T) {
	got := Render(Message{Subtype: SubtypeAgentsKilled, AgentCount: 1})
	if strings.Contains(got, "agents") {
		t.Error("should use singular 'agent'")
	}
}

func TestRender_Thinking(t *testing.T) {
	got := Render(Message{Subtype: SubtypeThinking, Text: "Analyzing the code..."})
	if !strings.Contains(got, "Analyzing") {
		t.Error("should contain thinking text")
	}
}

func TestRender_StopHookSummary(t *testing.T) {
	got := Render(Message{
		Subtype:       SubtypeStopHookSummary,
		HookSummaries: []string{"lint passed", "tests passed"},
	})
	if !strings.Contains(got, "lint passed") {
		t.Error("should contain hook summaries")
	}
}

func TestRender_Text(t *testing.T) {
	got := Render(Message{Subtype: SubtypeText, Text: "some info"})
	if !strings.Contains(got, "some info") {
		t.Error("should contain text")
	}
}
