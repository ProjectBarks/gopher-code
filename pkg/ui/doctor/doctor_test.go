package doctor

import (
	"errors"
	"strings"
	"testing"
)

func TestRenderDistTags_Success(t *testing.T) {
	tags := &DistTags{Stable: "1.0.5", Latest: "1.1.0"}
	out := RenderDistTags(tags, nil, "enabled", "latest")

	if !strings.Contains(out, "Updates") {
		t.Error("expected Updates header")
	}
	if !strings.Contains(out, "Stable version: 1.0.5") {
		t.Error("expected stable version")
	}
	if !strings.Contains(out, "Latest version: 1.1.0") {
		t.Error("expected latest version")
	}
	if !strings.Contains(out, "Auto-updates: enabled") {
		t.Error("expected auto-updates status")
	}
	if !strings.Contains(out, "Auto-update channel: latest") {
		t.Error("expected channel")
	}
}

func TestRenderDistTags_FetchError(t *testing.T) {
	out := RenderDistTags(nil, errors.New("network error"), "enabled", "latest")
	if !strings.Contains(out, "Failed to fetch versions") {
		t.Error("expected failure message on error")
	}
}

func TestRenderDistTags_NoStable(t *testing.T) {
	tags := &DistTags{Latest: "2.0.0"}
	out := RenderDistTags(tags, nil, "enabled", "latest")
	if strings.Contains(out, "Stable version") {
		t.Error("should not show stable when empty")
	}
	if !strings.Contains(out, "Latest version: 2.0.0") {
		t.Error("expected latest version")
	}
}

func TestRenderContextWarnings_Nil(t *testing.T) {
	out := RenderContextWarnings(nil)
	if out != "" {
		t.Error("nil warnings should produce empty output")
	}
}

func TestRenderContextWarnings_Empty(t *testing.T) {
	out := RenderContextWarnings(&ContextWarnings{})
	if out != "" {
		t.Error("empty warnings should produce empty output")
	}
}

func TestRenderContextWarnings_ClaudeMD(t *testing.T) {
	cw := &ContextWarnings{
		ClaudeMDWarning: &ContextWarning{
			Message: "CLAUDE.md files use 45% of context",
			Details: []string{"./CLAUDE.md (12k tokens)"},
		},
	}
	out := RenderContextWarnings(cw)
	if !strings.Contains(out, "Context Usage Warnings") {
		t.Error("expected section header")
	}
	if !strings.Contains(out, "CLAUDE.md files use 45% of context") {
		t.Error("expected warning message")
	}
	if !strings.Contains(out, "./CLAUDE.md (12k tokens)") {
		t.Error("expected detail")
	}
}

func TestRenderContextWarnings_UnreachableRules(t *testing.T) {
	cw := &ContextWarnings{
		UnreachableRulesWarning: &ContextWarning{
			Message: "2 rules will never match",
			Details: []string{"rule1", "rule2"},
		},
	}
	out := RenderContextWarnings(cw)
	if !strings.Contains(out, "Unreachable Permission Rules") {
		t.Error("expected unreachable rules header")
	}
	if !strings.Contains(out, "2 rules will never match") {
		t.Error("expected warning message")
	}
}

func TestRenderPIDLocks_Nil(t *testing.T) {
	out := RenderPIDLocks(nil)
	if out != "" {
		t.Error("nil should produce empty output")
	}
}

func TestRenderPIDLocks_Disabled(t *testing.T) {
	out := RenderPIDLocks(&VersionLockInfo{Enabled: false})
	if out != "" {
		t.Error("disabled should produce empty output")
	}
}

func TestRenderPIDLocks_NoLocks(t *testing.T) {
	out := RenderPIDLocks(&VersionLockInfo{Enabled: true})
	if !strings.Contains(out, "Version Locks") {
		t.Error("expected header")
	}
	if !strings.Contains(out, "No active version locks") {
		t.Error("expected no-locks message")
	}
}

func TestRenderPIDLocks_WithLocks(t *testing.T) {
	info := &VersionLockInfo{
		Enabled:           true,
		StaleLocksCleaned: 2,
		Locks: []LockInfo{
			{Version: "1.0.0", PID: 1234, IsProcessRunning: true},
			{Version: "1.0.1", PID: 5678, IsProcessRunning: false},
		},
	}
	out := RenderPIDLocks(info)
	if !strings.Contains(out, "Cleaned 2 stale lock(s)") {
		t.Error("expected stale locks cleaned message")
	}
	if !strings.Contains(out, "1.0.0: PID 1234") {
		t.Error("expected first lock")
	}
	if !strings.Contains(out, "(running)") {
		t.Error("expected running status")
	}
	if !strings.Contains(out, "(stale)") {
		t.Error("expected stale status")
	}
}

func TestRenderAgents_Nil(t *testing.T) {
	out := RenderAgents(nil)
	if out != "" {
		t.Error("nil should produce empty output")
	}
}

func TestRenderAgents_WithAgents(t *testing.T) {
	info := &AgentInfo{
		ActiveAgents: []AgentEntry{
			{AgentType: "custom", Source: "user"},
			{AgentType: "built-in", Source: "built-in"},
		},
		UserAgentsDir:    "/home/user/.claude/agents",
		ProjectAgentsDir: "/project/.claude/agents",
		UserDirExists:    true,
		ProjectDirExists: false,
	}
	out := RenderAgents(info)
	if !strings.Contains(out, "Active agents: 2") {
		t.Error("expected agent count")
	}
	if !strings.Contains(out, "custom (user)") {
		t.Error("expected custom agent entry")
	}
	if !strings.Contains(out, "(exists)") {
		t.Error("expected user dir exists")
	}
	if !strings.Contains(out, "(not found)") {
		t.Error("expected project dir not found")
	}
}

func TestRenderAgents_WithParseErrors(t *testing.T) {
	info := &AgentInfo{
		FailedFiles: []FailedAgentFile{
			{Path: "/agents/bad.yml", Error: "invalid yaml"},
		},
	}
	out := RenderAgents(info)
	if !strings.Contains(out, "Agent Parse Errors") {
		t.Error("expected parse errors header")
	}
	if !strings.Contains(out, "/agents/bad.yml: invalid yaml") {
		t.Error("expected error detail")
	}
}
