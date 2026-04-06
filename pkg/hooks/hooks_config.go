package hooks

// Source: utils/hooks/hooksConfigManager.ts, utils/hooks/hooksSettings.ts

// HookSource identifies where a hook config was loaded from.
// Source: hooksSettings.ts:15-20
type HookSource string

const (
	HookSourceUserSettings    HookSource = "userSettings"
	HookSourceProjectSettings HookSource = "projectSettings"
	HookSourceLocalSettings   HookSource = "localSettings"
	HookSourcePolicySettings  HookSource = "policySettings"
	HookSourcePluginHook      HookSource = "pluginHook"
	HookSourceSessionHook     HookSource = "sessionHook"
	HookSourceBuiltinHook     HookSource = "builtinHook"
)

// SettingsSources is the priority-ordered list of editable settings sources.
// Source: utils/settings/constants.ts SOURCES
var SettingsSources = []HookSource{
	HookSourceUserSettings,
	HookSourceProjectSettings,
	HookSourceLocalSettings,
}

// IndividualHookConfig is a single hook paired with its source and event.
// Source: hooksSettings.ts:22-28
type IndividualHookConfig struct {
	Event      HookEvent   `json:"event"`
	Config     HookCommand `json:"config"`
	Matcher    string      `json:"matcher,omitempty"`
	Source     HookSource  `json:"source"`
	PluginName string      `json:"pluginName,omitempty"`
}

// MatcherMetadata describes the field a hook event matches on and its valid values.
// Source: hooksConfigManager.ts:11-14
type MatcherMetadata struct {
	FieldToMatch string   `json:"fieldToMatch"`
	Values       []string `json:"values"`
}

// HookEventMetadata provides human-readable descriptions for a hook event.
// Source: hooksConfigManager.ts:16-20
type HookEventMetadata struct {
	Summary         string           `json:"summary"`
	Description     string           `json:"description"`
	MatcherMetadata *MatcherMetadata `json:"matcherMetadata,omitempty"`
}

// GetHookEventMetadata returns metadata for all 27 hook events.
// The toolNames parameter populates matcher values for tool-related events.
// Source: hooksConfigManager.ts:26-267
func GetHookEventMetadata(toolNames []string) map[HookEvent]HookEventMetadata {
	return map[HookEvent]HookEventMetadata{
		PreToolUse: {
			Summary:     "Before tool execution",
			Description: "Input to command is JSON of tool call arguments.\nExit code 0 - stdout/stderr not shown\nExit code 2 - show stderr to model and block tool call\nOther exit codes - show stderr to user only but continue with tool call",
			MatcherMetadata: &MatcherMetadata{
				FieldToMatch: "tool_name",
				Values:       toolNames,
			},
		},
		PostToolUse: {
			Summary:     "After tool execution",
			Description: "Input to command is JSON with fields \"inputs\" (tool call arguments) and \"response\" (tool call response).\nExit code 0 - stdout shown in transcript mode (ctrl+o)\nExit code 2 - show stderr to model immediately\nOther exit codes - show stderr to user only",
			MatcherMetadata: &MatcherMetadata{
				FieldToMatch: "tool_name",
				Values:       toolNames,
			},
		},
		PostToolUseFailure: {
			Summary:     "After tool execution fails",
			Description: "Input to command is JSON with tool_name, tool_input, tool_use_id, error, error_type, is_interrupt, and is_timeout.\nExit code 0 - stdout shown in transcript mode (ctrl+o)\nExit code 2 - show stderr to model immediately\nOther exit codes - show stderr to user only",
			MatcherMetadata: &MatcherMetadata{
				FieldToMatch: "tool_name",
				Values:       toolNames,
			},
		},
		PermissionDenied: {
			Summary:     "After auto mode classifier denies a tool call",
			Description: "Input to command is JSON with tool_name, tool_input, tool_use_id, and reason.\nReturn {\"hookSpecificOutput\":{\"hookEventName\":\"PermissionDenied\",\"retry\":true}} to tell the model it may retry.\nExit code 0 - stdout shown in transcript mode (ctrl+o)\nOther exit codes - show stderr to user only",
			MatcherMetadata: &MatcherMetadata{
				FieldToMatch: "tool_name",
				Values:       toolNames,
			},
		},
		Notification: {
			Summary:     "When notifications are sent",
			Description: "Input to command is JSON with notification message and type.\nExit code 0 - stdout/stderr not shown\nOther exit codes - show stderr to user only",
			MatcherMetadata: &MatcherMetadata{
				FieldToMatch: "notification_type",
				Values: []string{
					"permission_prompt", "idle_prompt", "auth_success",
					"elicitation_dialog", "elicitation_complete", "elicitation_response",
				},
			},
		},
		UserPromptSubmit: {
			Summary:     "When the user submits a prompt",
			Description: "Input to command is JSON with original user prompt text.\nExit code 0 - stdout shown to Claude\nExit code 2 - block processing, erase original prompt, and show stderr to user only\nOther exit codes - show stderr to user only",
		},
		SessionStart: {
			Summary:     "When a new session is started",
			Description: "Input to command is JSON with session start source.\nExit code 0 - stdout shown to Claude\nBlocking errors are ignored\nOther exit codes - show stderr to user only",
			MatcherMetadata: &MatcherMetadata{
				FieldToMatch: "source",
				Values:       []string{"startup", "resume", "clear", "compact"},
			},
		},
		Stop: {
			Summary:     "Right before Claude concludes its response",
			Description: "Exit code 0 - stdout/stderr not shown\nExit code 2 - show stderr to model and continue conversation\nOther exit codes - show stderr to user only",
		},
		StopFailure: {
			Summary:     "When the turn ends due to an API error",
			Description: "Fires instead of Stop when an API error (rate limit, auth failure, etc.) ended the turn. Fire-and-forget \u2014 hook output and exit codes are ignored.",
			MatcherMetadata: &MatcherMetadata{
				FieldToMatch: "error",
				Values: []string{
					"rate_limit", "authentication_failed", "billing_error",
					"invalid_request", "server_error", "max_output_tokens", "unknown",
				},
			},
		},
		SubagentStart: {
			Summary:     "When a subagent (Agent tool call) is started",
			Description: "Input to command is JSON with agent_id and agent_type.\nExit code 0 - stdout shown to subagent\nBlocking errors are ignored\nOther exit codes - show stderr to user only",
			MatcherMetadata: &MatcherMetadata{
				FieldToMatch: "agent_type",
				Values:       []string{},
			},
		},
		SubagentStop: {
			Summary:     "Right before a subagent (Agent tool call) concludes its response",
			Description: "Input to command is JSON with agent_id, agent_type, and agent_transcript_path.\nExit code 0 - stdout/stderr not shown\nExit code 2 - show stderr to subagent and continue having it run\nOther exit codes - show stderr to user only",
			MatcherMetadata: &MatcherMetadata{
				FieldToMatch: "agent_type",
				Values:       []string{},
			},
		},
		PreCompact: {
			Summary:     "Before conversation compaction",
			Description: "Input to command is JSON with compaction details.\nExit code 0 - stdout appended as custom compact instructions\nExit code 2 - block compaction\nOther exit codes - show stderr to user only but continue with compaction",
			MatcherMetadata: &MatcherMetadata{
				FieldToMatch: "trigger",
				Values:       []string{"manual", "auto"},
			},
		},
		PostCompact: {
			Summary:     "After conversation compaction",
			Description: "Input to command is JSON with compaction details and the summary.\nExit code 0 - stdout shown to user\nOther exit codes - show stderr to user only",
			MatcherMetadata: &MatcherMetadata{
				FieldToMatch: "trigger",
				Values:       []string{"manual", "auto"},
			},
		},
		SessionEnd: {
			Summary:     "When a session is ending",
			Description: "Input to command is JSON with session end reason.\nExit code 0 - command completes successfully\nOther exit codes - show stderr to user only",
			MatcherMetadata: &MatcherMetadata{
				FieldToMatch: "reason",
				Values:       []string{"clear", "logout", "prompt_input_exit", "other"},
			},
		},
		PermissionRequest: {
			Summary:     "When a permission dialog is displayed",
			Description: "Input to command is JSON with tool_name, tool_input, and tool_use_id.\nOutput JSON with hookSpecificOutput containing decision to allow or deny.\nExit code 0 - use hook decision if provided\nOther exit codes - show stderr to user only",
			MatcherMetadata: &MatcherMetadata{
				FieldToMatch: "tool_name",
				Values:       toolNames,
			},
		},
		Setup: {
			Summary:     "Repo setup hooks for init and maintenance",
			Description: "Input to command is JSON with trigger (init or maintenance).\nExit code 0 - stdout shown to Claude\nBlocking errors are ignored\nOther exit codes - show stderr to user only",
			MatcherMetadata: &MatcherMetadata{
				FieldToMatch: "trigger",
				Values:       []string{"init", "maintenance"},
			},
		},
		TeammateIdle: {
			Summary:     "When a teammate is about to go idle",
			Description: "Input to command is JSON with teammate_name and team_name.\nExit code 0 - stdout/stderr not shown\nExit code 2 - show stderr to teammate and prevent idle (teammate continues working)\nOther exit codes - show stderr to user only",
		},
		TaskCreated: {
			Summary:     "When a task is being created",
			Description: "Input to command is JSON with task_id, task_subject, task_description, teammate_name, and team_name.\nExit code 0 - stdout/stderr not shown\nExit code 2 - show stderr to model and prevent task creation\nOther exit codes - show stderr to user only",
		},
		TaskCompleted: {
			Summary:     "When a task is being marked as completed",
			Description: "Input to command is JSON with task_id, task_subject, task_description, teammate_name, and team_name.\nExit code 0 - stdout/stderr not shown\nExit code 2 - show stderr to model and prevent task completion\nOther exit codes - show stderr to user only",
		},
		Elicitation: {
			Summary:     "When an MCP server requests user input (elicitation)",
			Description: "Input to command is JSON with mcp_server_name, message, and requested_schema.\nOutput JSON with hookSpecificOutput containing action (accept/decline/cancel) and optional content.\nExit code 0 - use hook response if provided\nExit code 2 - deny the elicitation\nOther exit codes - show stderr to user only",
			MatcherMetadata: &MatcherMetadata{
				FieldToMatch: "mcp_server_name",
				Values:       []string{},
			},
		},
		ElicitationResult: {
			Summary:     "After a user responds to an MCP elicitation",
			Description: "Input to command is JSON with mcp_server_name, action, content, mode, and elicitation_id.\nOutput JSON with hookSpecificOutput containing optional action and content to override the response.\nExit code 0 - use hook response if provided\nExit code 2 - block the response (action becomes decline)\nOther exit codes - show stderr to user only",
			MatcherMetadata: &MatcherMetadata{
				FieldToMatch: "mcp_server_name",
				Values:       []string{},
			},
		},
		ConfigChange: {
			Summary:     "When configuration files change during a session",
			Description: "Input to command is JSON with source (user_settings, project_settings, local_settings, policy_settings, skills) and file_path.\nExit code 0 - allow the change\nExit code 2 - block the change from being applied to the session\nOther exit codes - show stderr to user only",
			MatcherMetadata: &MatcherMetadata{
				FieldToMatch: "source",
				Values: []string{
					"user_settings", "project_settings", "local_settings",
					"policy_settings", "skills",
				},
			},
		},
		InstructionsLoaded: {
			Summary:     "When an instruction file (CLAUDE.md or rule) is loaded",
			Description: "Input to command is JSON with file_path, memory_type (User, Project, Local, Managed), load_reason (session_start, nested_traversal, path_glob_match, include, compact), globs (optional), trigger_file_path (optional), and parent_file_path (optional).\nExit code 0 - command completes successfully\nOther exit codes - show stderr to user only\nThis hook is observability-only and does not support blocking.",
			MatcherMetadata: &MatcherMetadata{
				FieldToMatch: "load_reason",
				Values: []string{
					"session_start", "nested_traversal", "path_glob_match",
					"include", "compact",
				},
			},
		},
		WorktreeCreate: {
			Summary:     "Create an isolated worktree for VCS-agnostic isolation",
			Description: "Input to command is JSON with name (suggested worktree slug).\nStdout should contain the absolute path to the created worktree directory.\nExit code 0 - worktree created successfully\nOther exit codes - worktree creation failed",
		},
		WorktreeRemove: {
			Summary:     "Remove a previously created worktree",
			Description: "Input to command is JSON with worktree_path (absolute path to worktree).\nExit code 0 - worktree removed successfully\nOther exit codes - show stderr to user only",
		},
		CwdChanged: {
			Summary:     "After the working directory changes",
			Description: "Input to command is JSON with old_cwd and new_cwd.\nCLAUDE_ENV_FILE is set \u2014 write bash exports there to apply env to subsequent BashTool commands.\nHook output can include hookSpecificOutput.watchPaths (array of absolute paths) to register with the FileChanged watcher.\nExit code 0 - command completes successfully\nOther exit codes - show stderr to user only",
		},
		FileChanged: {
			Summary:     "When a watched file changes",
			Description: "Input to command is JSON with file_path and event (change, add, unlink).\nCLAUDE_ENV_FILE is set \u2014 write bash exports there to apply env to subsequent BashTool commands.\nThe matcher field specifies filenames to watch in the current directory (e.g. \".envrc|.env\").\nHook output can include hookSpecificOutput.watchPaths (array of absolute paths) to dynamically update the watch list.\nExit code 0 - command completes successfully\nOther exit codes - show stderr to user only",
		},
	}
}

// IsHookEqual compares two hook commands by type and content (not timeout).
// Source: hooksSettings.ts:33-65
func IsHookEqual(a, b HookCommand) bool {
	if a.Type != b.Type {
		return false
	}
	sameIf := (a.If == b.If)
	switch a.Type {
	case HookCommandTypeBash:
		aShell := a.Shell
		if aShell == "" {
			aShell = "bash"
		}
		bShell := b.Shell
		if bShell == "" {
			bShell = "bash"
		}
		return b.Type == HookCommandTypeBash && a.Command == b.Command && aShell == bShell && sameIf
	case HookCommandTypePrompt:
		return b.Type == HookCommandTypePrompt && a.Prompt == b.Prompt && sameIf
	case HookCommandTypeAgent:
		return b.Type == HookCommandTypeAgent && a.Prompt == b.Prompt && sameIf
	case HookCommandTypeHTTP:
		return b.Type == HookCommandTypeHTTP && a.URL == b.URL && sameIf
	}
	return false
}

// GetHookDisplayText returns a human-readable display text for a hook command.
// Source: hooksSettings.ts:68-90
func GetHookDisplayText(hook HookCommand) string {
	if hook.StatusMessage != "" {
		return hook.StatusMessage
	}
	switch hook.Type {
	case HookCommandTypeBash:
		return hook.Command
	case HookCommandTypePrompt, HookCommandTypeAgent:
		return hook.Prompt
	case HookCommandTypeHTTP:
		return hook.URL
	}
	return ""
}

// HookSourceDescription returns a human-readable description of the source.
// Source: hooksSettings.ts:170-189
func HookSourceDescription(source HookSource) string {
	switch source {
	case HookSourceUserSettings:
		return "User settings (~/.claude/settings.json)"
	case HookSourceProjectSettings:
		return "Project settings (.claude/settings.json)"
	case HookSourceLocalSettings:
		return "Local settings (.claude/settings.local.json)"
	case HookSourcePluginHook:
		return "Plugin hooks (~/.claude/plugins/*/hooks/hooks.json)"
	case HookSourceSessionHook:
		return "Session hooks (in-memory, temporary)"
	case HookSourceBuiltinHook:
		return "Built-in hooks (registered internally by Claude Code)"
	default:
		return string(source)
	}
}

// HookSourceHeader returns a short header label for the source.
// Source: hooksSettings.ts:192-208
func HookSourceHeader(source HookSource) string {
	switch source {
	case HookSourceUserSettings:
		return "User Settings"
	case HookSourceProjectSettings:
		return "Project Settings"
	case HookSourceLocalSettings:
		return "Local Settings"
	case HookSourcePluginHook:
		return "Plugin Hooks"
	case HookSourceSessionHook:
		return "Session Hooks"
	case HookSourceBuiltinHook:
		return "Built-in Hooks"
	default:
		return string(source)
	}
}

// HookSourceInline returns a short inline label for the source.
// Source: hooksSettings.ts:211-228
func HookSourceInline(source HookSource) string {
	switch source {
	case HookSourceUserSettings:
		return "User"
	case HookSourceProjectSettings:
		return "Project"
	case HookSourceLocalSettings:
		return "Local"
	case HookSourcePluginHook:
		return "Plugin"
	case HookSourceSessionHook:
		return "Session"
	case HookSourceBuiltinHook:
		return "Built-in"
	default:
		return string(source)
	}
}

// GroupHooksByEventAndMatcher groups a flat list of hooks by event and matcher key.
// Source: hooksConfigManager.ts:270-365
func GroupHooksByEventAndMatcher(hooks []IndividualHookConfig, toolNames []string) map[HookEvent]map[string][]IndividualHookConfig {
	metadata := GetHookEventMetadata(toolNames)

	grouped := make(map[HookEvent]map[string][]IndividualHookConfig, len(AllHookEvents))
	for _, ev := range AllHookEvents {
		grouped[ev] = make(map[string][]IndividualHookConfig)
	}

	for _, hook := range hooks {
		eventGroup, ok := grouped[hook.Event]
		if !ok {
			continue
		}
		matcherKey := ""
		if meta, ok := metadata[hook.Event]; ok && meta.MatcherMetadata != nil {
			matcherKey = hook.Matcher
		}
		eventGroup[matcherKey] = append(eventGroup[matcherKey], hook)
	}

	return grouped
}

// GetSortedMatchersForEvent returns matchers for an event sorted by source priority.
// Source: hooksConfigManager.ts:368-377
func GetSortedMatchersForEvent(grouped map[HookEvent]map[string][]IndividualHookConfig, event HookEvent) []string {
	eventGroup, ok := grouped[event]
	if !ok {
		return nil
	}
	matchers := make([]string, 0, len(eventGroup))
	for m := range eventGroup {
		matchers = append(matchers, m)
	}
	return SortMatchersByPriority(matchers, grouped, event)
}

// GetHooksForMatcher returns hooks for a specific event and matcher.
// Source: hooksConfigManager.ts:380-392
func GetHooksForMatcher(grouped map[HookEvent]map[string][]IndividualHookConfig, event HookEvent, matcher string) []IndividualHookConfig {
	eventGroup, ok := grouped[event]
	if !ok {
		return nil
	}
	return eventGroup[matcher]
}

// GetMatcherMetadata returns the matcher metadata for a specific event.
// Source: hooksConfigManager.ts:395-399
func GetMatcherMetadata(event HookEvent, toolNames []string) *MatcherMetadata {
	meta, ok := GetHookEventMetadata(toolNames)[event]
	if !ok {
		return nil
	}
	return meta.MatcherMetadata
}

// SortMatchersByPriority sorts matchers by highest-priority source first.
// Source: hooksSettings.ts:230-271
func SortMatchersByPriority(matchers []string, grouped map[HookEvent]map[string][]IndividualHookConfig, event HookEvent) []string {
	sourcePriority := make(map[HookSource]int, len(SettingsSources))
	for i, s := range SettingsSources {
		sourcePriority[s] = i
	}

	getSourcePriority := func(source HookSource) int {
		if source == HookSourcePluginHook || source == HookSourceBuiltinHook {
			return 999
		}
		if p, ok := sourcePriority[source]; ok {
			return p
		}
		return 500
	}

	getHighestPriority := func(matcher string) int {
		hooks := grouped[event][matcher]
		best := 9999
		for _, h := range hooks {
			p := getSourcePriority(h.Source)
			if p < best {
				best = p
			}
		}
		return best
	}

	sorted := make([]string, len(matchers))
	copy(sorted, matchers)

	// Bubble sort for small N
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			pi := getHighestPriority(sorted[i])
			pj := getHighestPriority(sorted[j])
			if pj < pi || (pj == pi && sorted[j] < sorted[i]) {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	return sorted
}
