package hooks

import (
	"testing"
)

// Source: utils/hooks/hooksConfigManager.ts, utils/hooks/hooksSettings.ts

func TestGetHookEventMetadata_AllEventsPresent(t *testing.T) {
	// Source: hooksConfigManager.ts:26-267 — 27 events
	meta := GetHookEventMetadata([]string{"Bash", "Read", "Write"})
	if len(meta) != 27 {
		t.Errorf("expected 27 events, got %d", len(meta))
	}
	for _, ev := range AllHookEvents {
		if _, ok := meta[ev]; !ok {
			t.Errorf("missing metadata for event %s", ev)
		}
	}
}

func TestGetHookEventMetadata_Summaries(t *testing.T) {
	// Source: hooksConfigManager.ts — exact summary strings
	meta := GetHookEventMetadata(nil)

	tests := []struct {
		event   HookEvent
		summary string
	}{
		{PreToolUse, "Before tool execution"},
		{PostToolUse, "After tool execution"},
		{Stop, "Right before Claude concludes its response"},
		{SessionStart, "When a new session is started"},
		{SessionEnd, "When a session is ending"},
		{UserPromptSubmit, "When the user submits a prompt"},
		{Notification, "When notifications are sent"},
		{Setup, "Repo setup hooks for init and maintenance"},
		{TeammateIdle, "When a teammate is about to go idle"},
		{FileChanged, "When a watched file changes"},
		{CwdChanged, "After the working directory changes"},
		{WorktreeCreate, "Create an isolated worktree for VCS-agnostic isolation"},
		{WorktreeRemove, "Remove a previously created worktree"},
		{ConfigChange, "When configuration files change during a session"},
		{InstructionsLoaded, "When an instruction file (CLAUDE.md or rule) is loaded"},
	}

	for _, tc := range tests {
		t.Run(string(tc.event), func(t *testing.T) {
			if meta[tc.event].Summary != tc.summary {
				t.Errorf("summary = %q, want %q", meta[tc.event].Summary, tc.summary)
			}
		})
	}
}

func TestGetHookEventMetadata_MatcherFields(t *testing.T) {
	// Source: hooksConfigManager.ts — matcher field names
	tools := []string{"Bash", "Read"}
	meta := GetHookEventMetadata(tools)

	tests := []struct {
		event HookEvent
		field string
	}{
		{PreToolUse, "tool_name"},
		{PostToolUse, "tool_name"},
		{PostToolUseFailure, "tool_name"},
		{PermissionDenied, "tool_name"},
		{PermissionRequest, "tool_name"},
		{Notification, "notification_type"},
		{SessionStart, "source"},
		{StopFailure, "error"},
		{SubagentStart, "agent_type"},
		{SubagentStop, "agent_type"},
		{PreCompact, "trigger"},
		{PostCompact, "trigger"},
		{SessionEnd, "reason"},
		{Setup, "trigger"},
		{Elicitation, "mcp_server_name"},
		{ElicitationResult, "mcp_server_name"},
		{ConfigChange, "source"},
		{InstructionsLoaded, "load_reason"},
	}

	for _, tc := range tests {
		t.Run(string(tc.event), func(t *testing.T) {
			m := meta[tc.event].MatcherMetadata
			if m == nil {
				t.Fatalf("missing matcher metadata for %s", tc.event)
			}
			if m.FieldToMatch != tc.field {
				t.Errorf("fieldToMatch = %q, want %q", m.FieldToMatch, tc.field)
			}
		})
	}
}

func TestGetHookEventMetadata_NoMatcher(t *testing.T) {
	// Source: hooksConfigManager.ts — events without matchers
	meta := GetHookEventMetadata(nil)

	noMatcher := []HookEvent{
		UserPromptSubmit, Stop, TeammateIdle, TaskCreated, TaskCompleted,
		WorktreeCreate, WorktreeRemove, CwdChanged, FileChanged,
	}

	for _, ev := range noMatcher {
		t.Run(string(ev), func(t *testing.T) {
			if meta[ev].MatcherMetadata != nil {
				t.Errorf("expected no matcher metadata for %s", ev)
			}
		})
	}
}

func TestGetHookEventMetadata_ToolNamesPassthrough(t *testing.T) {
	// Source: hooksConfigManager.ts — tool names passed to tool_name matcher values
	tools := []string{"Bash", "Read", "Write", "FileRead"}
	meta := GetHookEventMetadata(tools)

	m := meta[PreToolUse].MatcherMetadata
	if m == nil {
		t.Fatal("missing matcher metadata")
	}
	if len(m.Values) != 4 {
		t.Errorf("expected 4 tool names, got %d", len(m.Values))
	}
	if m.Values[0] != "Bash" || m.Values[3] != "FileRead" {
		t.Errorf("values = %v", m.Values)
	}
}

func TestGetHookEventMetadata_NotificationTypes(t *testing.T) {
	// Source: hooksConfigManager.ts — notification types
	meta := GetHookEventMetadata(nil)
	m := meta[Notification].MatcherMetadata
	if m == nil {
		t.Fatal("missing matcher metadata")
	}
	expected := []string{
		"permission_prompt", "idle_prompt", "auth_success",
		"elicitation_dialog", "elicitation_complete", "elicitation_response",
	}
	if len(m.Values) != len(expected) {
		t.Fatalf("expected %d notification types, got %d", len(expected), len(m.Values))
	}
	for i, v := range expected {
		if m.Values[i] != v {
			t.Errorf("values[%d] = %q, want %q", i, m.Values[i], v)
		}
	}
}

func TestIsHookEqual(t *testing.T) {
	// Source: hooksSettings.ts:33-65

	t.Run("same_command", func(t *testing.T) {
		a := HookCommand{Type: HookCommandTypeBash, Command: "echo hi"}
		b := HookCommand{Type: HookCommandTypeBash, Command: "echo hi"}
		if !IsHookEqual(a, b) {
			t.Error("same command should be equal")
		}
	})

	t.Run("different_command", func(t *testing.T) {
		a := HookCommand{Type: HookCommandTypeBash, Command: "echo hi"}
		b := HookCommand{Type: HookCommandTypeBash, Command: "echo bye"}
		if IsHookEqual(a, b) {
			t.Error("different commands should not be equal")
		}
	})

	t.Run("different_type", func(t *testing.T) {
		a := HookCommand{Type: HookCommandTypeBash, Command: "echo hi"}
		b := HookCommand{Type: HookCommandTypeHTTP, URL: "echo hi"}
		if IsHookEqual(a, b) {
			t.Error("different types should not be equal")
		}
	})

	t.Run("shell_default_bash", func(t *testing.T) {
		a := HookCommand{Type: HookCommandTypeBash, Command: "echo hi", Shell: ""}
		b := HookCommand{Type: HookCommandTypeBash, Command: "echo hi", Shell: "bash"}
		if !IsHookEqual(a, b) {
			t.Error("empty shell should equal 'bash'")
		}
	})

	t.Run("different_shell", func(t *testing.T) {
		a := HookCommand{Type: HookCommandTypeBash, Command: "echo hi", Shell: "bash"}
		b := HookCommand{Type: HookCommandTypeBash, Command: "echo hi", Shell: "powershell"}
		if IsHookEqual(a, b) {
			t.Error("different shells should not be equal")
		}
	})

	t.Run("different_if", func(t *testing.T) {
		a := HookCommand{Type: HookCommandTypeBash, Command: "echo hi", If: "Bash(git *)"}
		b := HookCommand{Type: HookCommandTypeBash, Command: "echo hi", If: "Bash(npm *)"}
		if IsHookEqual(a, b) {
			t.Error("different if conditions should not be equal")
		}
	})

	t.Run("prompt_equal", func(t *testing.T) {
		a := HookCommand{Type: HookCommandTypePrompt, Prompt: "check safety"}
		b := HookCommand{Type: HookCommandTypePrompt, Prompt: "check safety"}
		if !IsHookEqual(a, b) {
			t.Error("same prompts should be equal")
		}
	})

	t.Run("agent_equal", func(t *testing.T) {
		a := HookCommand{Type: HookCommandTypeAgent, Prompt: "verify tests"}
		b := HookCommand{Type: HookCommandTypeAgent, Prompt: "verify tests"}
		if !IsHookEqual(a, b) {
			t.Error("same agent prompts should be equal")
		}
	})

	t.Run("http_equal", func(t *testing.T) {
		a := HookCommand{Type: HookCommandTypeHTTP, URL: "https://example.com/hook"}
		b := HookCommand{Type: HookCommandTypeHTTP, URL: "https://example.com/hook"}
		if !IsHookEqual(a, b) {
			t.Error("same URLs should be equal")
		}
	})

	t.Run("timeout_ignored", func(t *testing.T) {
		a := HookCommand{Type: HookCommandTypeBash, Command: "echo hi", Timeout: 10}
		b := HookCommand{Type: HookCommandTypeBash, Command: "echo hi", Timeout: 30}
		if !IsHookEqual(a, b) {
			t.Error("timeout should be ignored in equality")
		}
	})
}

func TestGetHookDisplayText(t *testing.T) {
	// Source: hooksSettings.ts:68-90

	t.Run("command", func(t *testing.T) {
		h := HookCommand{Type: HookCommandTypeBash, Command: "echo hello"}
		if GetHookDisplayText(h) != "echo hello" {
			t.Errorf("got %q", GetHookDisplayText(h))
		}
	})

	t.Run("prompt", func(t *testing.T) {
		h := HookCommand{Type: HookCommandTypePrompt, Prompt: "check safety"}
		if GetHookDisplayText(h) != "check safety" {
			t.Errorf("got %q", GetHookDisplayText(h))
		}
	})

	t.Run("agent", func(t *testing.T) {
		h := HookCommand{Type: HookCommandTypeAgent, Prompt: "verify tests"}
		if GetHookDisplayText(h) != "verify tests" {
			t.Errorf("got %q", GetHookDisplayText(h))
		}
	})

	t.Run("http", func(t *testing.T) {
		h := HookCommand{Type: HookCommandTypeHTTP, URL: "https://example.com"}
		if GetHookDisplayText(h) != "https://example.com" {
			t.Errorf("got %q", GetHookDisplayText(h))
		}
	})

	t.Run("status_message_override", func(t *testing.T) {
		h := HookCommand{Type: HookCommandTypeBash, Command: "echo hi", StatusMessage: "Running check..."}
		if GetHookDisplayText(h) != "Running check..." {
			t.Errorf("got %q", GetHookDisplayText(h))
		}
	})
}

func TestHookSourceDescription(t *testing.T) {
	// Source: hooksSettings.ts:170-189
	tests := []struct {
		source   HookSource
		expected string
	}{
		{HookSourceUserSettings, "User settings (~/.claude/settings.json)"},
		{HookSourceProjectSettings, "Project settings (.claude/settings.json)"},
		{HookSourceLocalSettings, "Local settings (.claude/settings.local.json)"},
		{HookSourcePluginHook, "Plugin hooks (~/.claude/plugins/*/hooks/hooks.json)"},
		{HookSourceSessionHook, "Session hooks (in-memory, temporary)"},
		{HookSourceBuiltinHook, "Built-in hooks (registered internally by Claude Code)"},
	}
	for _, tc := range tests {
		t.Run(string(tc.source), func(t *testing.T) {
			if HookSourceDescription(tc.source) != tc.expected {
				t.Errorf("got %q, want %q", HookSourceDescription(tc.source), tc.expected)
			}
		})
	}
}

func TestHookSourceHeader(t *testing.T) {
	// Source: hooksSettings.ts:192-208
	if HookSourceHeader(HookSourceUserSettings) != "User Settings" {
		t.Error("wrong")
	}
	if HookSourceHeader(HookSourcePluginHook) != "Plugin Hooks" {
		t.Error("wrong")
	}
}

func TestHookSourceInline(t *testing.T) {
	// Source: hooksSettings.ts:211-228
	if HookSourceInline(HookSourceUserSettings) != "User" {
		t.Error("wrong")
	}
	if HookSourceInline(HookSourceProjectSettings) != "Project" {
		t.Error("wrong")
	}
	if HookSourceInline(HookSourceLocalSettings) != "Local" {
		t.Error("wrong")
	}
	if HookSourceInline(HookSourcePluginHook) != "Plugin" {
		t.Error("wrong")
	}
	if HookSourceInline(HookSourceSessionHook) != "Session" {
		t.Error("wrong")
	}
	if HookSourceInline(HookSourceBuiltinHook) != "Built-in" {
		t.Error("wrong")
	}
}

func TestGroupHooksByEventAndMatcher(t *testing.T) {
	// Source: hooksConfigManager.ts:270-365
	hooks := []IndividualHookConfig{
		{Event: PreToolUse, Config: HookCommand{Type: HookCommandTypeBash, Command: "echo pre"}, Matcher: "Bash", Source: HookSourceUserSettings},
		{Event: PreToolUse, Config: HookCommand{Type: HookCommandTypeBash, Command: "echo pre2"}, Matcher: "Read", Source: HookSourceProjectSettings},
		{Event: Stop, Config: HookCommand{Type: HookCommandTypeBash, Command: "echo stop"}, Source: HookSourceUserSettings},
		{Event: SessionStart, Config: HookCommand{Type: HookCommandTypeBash, Command: "echo start"}, Matcher: "startup", Source: HookSourceUserSettings},
	}

	grouped := GroupHooksByEventAndMatcher(hooks, []string{"Bash", "Read"})

	// PreToolUse should have 2 matcher keys
	if len(grouped[PreToolUse]) != 2 {
		t.Errorf("expected 2 PreToolUse matchers, got %d", len(grouped[PreToolUse]))
	}
	if len(grouped[PreToolUse]["Bash"]) != 1 {
		t.Error("expected 1 hook for Bash matcher")
	}
	if len(grouped[PreToolUse]["Read"]) != 1 {
		t.Error("expected 1 hook for Read matcher")
	}

	// Stop has no matcher metadata, so hooks go under ""
	if len(grouped[Stop][""]) != 1 {
		t.Errorf("expected 1 Stop hook under empty matcher, got %d", len(grouped[Stop][""]))
	}

	// SessionStart has matcher metadata, so uses the matcher value
	if len(grouped[SessionStart]["startup"]) != 1 {
		t.Errorf("expected 1 SessionStart hook under 'startup', got %d", len(grouped[SessionStart]["startup"]))
	}
}

func TestGetSortedMatchersForEvent(t *testing.T) {
	// Source: hooksConfigManager.ts:368-377
	hooks := []IndividualHookConfig{
		{Event: PreToolUse, Config: HookCommand{Type: HookCommandTypeBash, Command: "a"}, Matcher: "Write", Source: HookSourceProjectSettings},
		{Event: PreToolUse, Config: HookCommand{Type: HookCommandTypeBash, Command: "b"}, Matcher: "Bash", Source: HookSourceUserSettings},
		{Event: PreToolUse, Config: HookCommand{Type: HookCommandTypeBash, Command: "c"}, Matcher: "Read", Source: HookSourcePluginHook},
	}

	grouped := GroupHooksByEventAndMatcher(hooks, []string{"Bash", "Read", "Write"})
	sorted := GetSortedMatchersForEvent(grouped, PreToolUse)

	if len(sorted) != 3 {
		t.Fatalf("expected 3 matchers, got %d", len(sorted))
	}
	// UserSettings (0) < ProjectSettings (1) < PluginHook (999)
	if sorted[0] != "Bash" {
		t.Errorf("expected Bash first (user settings), got %q", sorted[0])
	}
	if sorted[1] != "Write" {
		t.Errorf("expected Write second (project settings), got %q", sorted[1])
	}
	if sorted[2] != "Read" {
		t.Errorf("expected Read third (plugin), got %q", sorted[2])
	}
}

func TestGetHooksForMatcher(t *testing.T) {
	// Source: hooksConfigManager.ts:380-392
	hooks := []IndividualHookConfig{
		{Event: PreToolUse, Config: HookCommand{Type: HookCommandTypeBash, Command: "a"}, Matcher: "Bash", Source: HookSourceUserSettings},
		{Event: PreToolUse, Config: HookCommand{Type: HookCommandTypeBash, Command: "b"}, Matcher: "Bash", Source: HookSourceProjectSettings},
		{Event: PreToolUse, Config: HookCommand{Type: HookCommandTypeBash, Command: "c"}, Matcher: "Read", Source: HookSourceUserSettings},
	}

	grouped := GroupHooksByEventAndMatcher(hooks, []string{"Bash", "Read"})

	bashHooks := GetHooksForMatcher(grouped, PreToolUse, "Bash")
	if len(bashHooks) != 2 {
		t.Errorf("expected 2 Bash hooks, got %d", len(bashHooks))
	}

	readHooks := GetHooksForMatcher(grouped, PreToolUse, "Read")
	if len(readHooks) != 1 {
		t.Errorf("expected 1 Read hook, got %d", len(readHooks))
	}

	// Non-existent matcher
	noneHooks := GetHooksForMatcher(grouped, PreToolUse, "Write")
	if len(noneHooks) != 0 {
		t.Errorf("expected 0 Write hooks, got %d", len(noneHooks))
	}
}

func TestGetMatcherMetadata(t *testing.T) {
	// Source: hooksConfigManager.ts:395-399
	tools := []string{"Bash", "Read"}

	m := GetMatcherMetadata(PreToolUse, tools)
	if m == nil {
		t.Fatal("expected matcher metadata for PreToolUse")
	}
	if m.FieldToMatch != "tool_name" {
		t.Errorf("fieldToMatch = %q", m.FieldToMatch)
	}

	m = GetMatcherMetadata(Stop, tools)
	if m != nil {
		t.Error("expected nil matcher metadata for Stop")
	}
}

func TestSortMatchersByPriority_SameSourceAlphabetical(t *testing.T) {
	// Source: hooksSettings.ts:230-271 — same priority sorts alphabetically
	hooks := []IndividualHookConfig{
		{Event: PreToolUse, Config: HookCommand{Type: HookCommandTypeBash, Command: "a"}, Matcher: "Write", Source: HookSourceUserSettings},
		{Event: PreToolUse, Config: HookCommand{Type: HookCommandTypeBash, Command: "b"}, Matcher: "Bash", Source: HookSourceUserSettings},
		{Event: PreToolUse, Config: HookCommand{Type: HookCommandTypeBash, Command: "c"}, Matcher: "Read", Source: HookSourceUserSettings},
	}

	grouped := GroupHooksByEventAndMatcher(hooks, []string{"Bash", "Read", "Write"})
	sorted := GetSortedMatchersForEvent(grouped, PreToolUse)

	if sorted[0] != "Bash" || sorted[1] != "Read" || sorted[2] != "Write" {
		t.Errorf("expected alphabetical order, got %v", sorted)
	}
}

func TestIndividualHookConfig_Fields(t *testing.T) {
	// Source: hooksSettings.ts:22-28
	h := IndividualHookConfig{
		Event:      PreToolUse,
		Config:     HookCommand{Type: HookCommandTypeBash, Command: "echo"},
		Matcher:    "Bash",
		Source:     HookSourceUserSettings,
		PluginName: "",
	}
	if h.Event != PreToolUse {
		t.Error("wrong event")
	}
	if h.Source != HookSourceUserSettings {
		t.Error("wrong source")
	}
}

func TestSettingsSources_Order(t *testing.T) {
	// Source: utils/settings/constants.ts SOURCES
	if len(SettingsSources) != 3 {
		t.Fatalf("expected 3 settings sources, got %d", len(SettingsSources))
	}
	if SettingsSources[0] != HookSourceUserSettings {
		t.Error("first should be userSettings")
	}
	if SettingsSources[1] != HookSourceProjectSettings {
		t.Error("second should be projectSettings")
	}
	if SettingsSources[2] != HookSourceLocalSettings {
		t.Error("third should be localSettings")
	}
}
