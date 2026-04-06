package prompt

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/projectbarks/gopher-code/pkg/output_styles"
	"github.com/projectbarks/gopher-code/pkg/provider"
)

// SYSTEM_PROMPT_DYNAMIC_BOUNDARY separates static (cross-org cacheable) content
// from dynamic content in the system prompt array.
// Source: constants/prompts.ts — SYSTEM_PROMPT_DYNAMIC_BOUNDARY
const SystemPromptDynamicBoundary = "__SYSTEM_PROMPT_DYNAMIC_BOUNDARY__"

// FRONTIER_MODEL_NAME is the current frontier model's marketing name.
// @[MODEL LAUNCH]: Update on each release.
// Source: constants/prompts.ts — FRONTIER_MODEL_NAME
const FrontierModelName = "Claude Opus 4.6"

// ClaudeCodeDocsMapURL is the public docs URL referenced by the prompt.
// Source: constants/prompts.ts — CLAUDE_CODE_DOCS_MAP_URL
const ClaudeCodeDocsMapURL = "https://code.claude.com/docs/en/claude_code_docs_map.md"

// Claude46Or45ModelIDs maps tier names to model IDs.
// @[MODEL LAUNCH]: Update on each release.
// Source: constants/prompts.ts — CLAUDE_4_5_OR_4_6_MODEL_IDS
var Claude46Or45ModelIDs = struct {
	Opus   string
	Sonnet string
	Haiku  string
}{
	Opus:   "claude-opus-4-6",
	Sonnet: "claude-sonnet-4-6",
	Haiku:  "claude-haiku-4-5-20251001",
}

// SummarizeToolResultsSection is appended to prompts when function-result
// clearing is active.
// Source: constants/prompts.ts — SUMMARIZE_TOOL_RESULTS_SECTION
const SummarizeToolResultsSection = "When working with tool results, write down any important information you might need later in your response, as the original tool result may be cleared later."

// DefaultAgentPrompt is the system prompt prefix for sub-agents.
// Source: constants/prompts.ts — DEFAULT_AGENT_PROMPT
const DefaultAgentPrompt = `You are an agent for Claude Code, Anthropic's official CLI for Claude. Given the user's message, you should use the tools available to complete the task. Complete the task fully—don't gold-plate, but don't leave it half-done. When you complete the task, respond with a concise report covering what was done and any key findings — the caller will relay this to the user, so it only needs the essentials.`

// FastModeExplanation is shown in env info.
// Source: constants/prompts.ts — fast-mode text in computeSimpleEnvInfo
var FastModeExplanation = fmt.Sprintf(
	"Fast mode for Claude Code uses the same %s model with faster output. It does NOT switch to a different model. It can be toggled with /fast.",
	FrontierModelName,
)

// HooksSection returns the hooks guidance paragraph.
// Source: constants/prompts.ts — getHooksSection()
func HooksSection() string {
	return "Users may configure 'hooks', shell commands that execute in response to events like tool calls, in settings. Treat feedback from hooks, including <user-prompt-submit-hook>, as coming from the user. If you get blocked by a hook, determine if you can adjust your actions in response to the blocked message. If not, ask the user to check their hooks configuration."
}

// ActionsSection returns the "Executing actions with care" section.
// Source: constants/prompts.ts — getActionsSection()
func ActionsSection() string {
	return `# Executing actions with care

Carefully consider the reversibility and blast radius of actions. Generally you can freely take local, reversible actions like editing files or running tests. But for actions that are hard to reverse, affect shared systems beyond your local environment, or could otherwise be risky or destructive, check with the user before proceeding. The cost of pausing to confirm is low, while the cost of an unwanted action (lost work, unintended messages sent, deleted branches) can be very high. For actions like these, consider the context, the action, and user instructions, and by default transparently communicate the action and ask for confirmation before proceeding. This default can be changed by user instructions - if explicitly asked to operate more autonomously, then you may proceed without confirmation, but still attend to the risks and consequences when taking actions. A user approving an action (like a git push) once does NOT mean that they approve it in all contexts, so unless actions are authorized in advance in durable instructions like CLAUDE.md files, always confirm first. Authorization stands for the scope specified, not beyond. Match the scope of your actions to what was actually requested.

Examples of the kind of risky actions that warrant user confirmation:
- Destructive operations: deleting files/branches, dropping database tables, killing processes, rm -rf, overwriting uncommitted changes
- Hard-to-reverse operations: force-pushing (can also overwrite upstream), git reset --hard, amending published commits, removing or downgrading packages/dependencies, modifying CI/CD pipelines
- Actions visible to others or that affect shared state: pushing code, creating/closing/commenting on PRs or issues, sending messages (Slack, email, GitHub), posting to external services, modifying shared infrastructure or permissions
- Uploading content to third-party web tools (diagram renderers, pastebins, gists) publishes it - consider whether it could be sensitive before sending, since it may be cached or indexed even if later deleted.

When you encounter an obstacle, do not use destructive actions as a shortcut to simply make it go away. For instance, try to identify root causes and fix underlying issues rather than bypassing safety checks (e.g. --no-verify). If you discover unexpected state like unfamiliar files, branches, or configuration, investigate before deleting or overwriting, as it may represent the user's in-progress work. For example, typically resolve merge conflicts rather than discarding changes; similarly, if a lock file exists, investigate what process holds it rather than deleting it. In short: only take risky actions carefully, and when in doubt, ask before acting. Follow both the spirit and letter of these instructions - measure twice, cut once.`
}

// OutputEfficiencySection returns the output efficiency guidance.
// Source: constants/prompts.ts — getOutputEfficiencySection() (external path)
func OutputEfficiencySection() string {
	return `# Output efficiency

IMPORTANT: Go straight to the point. Try the simplest approach first without going in circles. Do not overdo it. Be extra concise.

Keep your text output brief and direct. Lead with the answer or action, not the reasoning. Skip filler words, preamble, and unnecessary transitions. Do not restate what the user said — just do it. When explaining, include only what is necessary for the user to understand.

Focus text output on:
- Decisions that need the user's input
- High-level status updates at natural milestones
- Errors or blockers that change the plan

If you can say it in one sentence, don't use three. Prefer short, direct sentences over long explanations. This does not apply to code or tool calls.`
}

// ToneAndStyleSection returns the tone and style guidance.
// Source: constants/prompts.ts — getSimpleToneAndStyleSection() (external path)
func ToneAndStyleSection() string {
	items := []string{
		"Only use emojis if the user explicitly requests it. Avoid using emojis in all communication unless asked.",
		"Your responses should be short and concise.",
		"When referencing specific functions or pieces of code include the pattern file_path:line_number to allow the user to easily navigate to the source code location.",
		`When referencing GitHub issues or pull requests, use the owner/repo#123 format (e.g. anthropics/claude-code#100) so they render as clickable links.`,
		`Do not use a colon before tool calls. Your tool calls may not be shown directly in the output, so text like "Let me read the file:" followed by a read tool call should just be "Let me read the file." with a period.`,
	}
	return "# Tone and style\n" + strings.Join(PrependBullets(items), "\n")
}

// PrependBullets formats a flat list of strings as " - item" bullets.
// Nested slices (from the TS version) are handled by PrependBulletsNested.
// Source: constants/prompts.ts — prependBullets()
func PrependBullets(items []string) []string {
	out := make([]string, len(items))
	for i, item := range items {
		out[i] = " - " + item
	}
	return out
}

// PrependBulletsNested handles mixed items: plain strings get " - " prefix,
// sub-slices get "  - " (nested indent). Mirrors the TS flatMap logic.
func PrependBulletsNested(items []any) []string {
	var out []string
	for _, item := range items {
		switch v := item.(type) {
		case string:
			out = append(out, " - "+v)
		case []string:
			for _, sub := range v {
				out = append(out, "  - "+sub)
			}
		}
	}
	return out
}

// GetKnowledgeCutoff returns the knowledge cutoff date string for a model ID,
// or "" if unknown.
// Source: constants/prompts.ts — getKnowledgeCutoff()
func GetKnowledgeCutoff(modelID string) string {
	canonical := provider.GetCanonicalName(modelID)

	switch {
	case strings.Contains(canonical, "claude-sonnet-4-6"):
		return "August 2025"
	case strings.Contains(canonical, "claude-opus-4-6"):
		return "May 2025"
	case strings.Contains(canonical, "claude-opus-4-5"):
		return "May 2025"
	case strings.Contains(canonical, "claude-haiku-4"):
		return "February 2025"
	case strings.Contains(canonical, "claude-opus-4"),
		strings.Contains(canonical, "claude-sonnet-4"):
		return "January 2025"
	default:
		return ""
	}
}

// GetShellInfoLine returns the "Shell: ..." line for the env section.
// Source: constants/prompts.ts — getShellInfoLine()
func GetShellInfoLine() string {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "unknown"
	}

	var shellName string
	switch {
	case strings.Contains(shell, "zsh"):
		shellName = "zsh"
	case strings.Contains(shell, "bash"):
		shellName = "bash"
	default:
		shellName = shell
	}

	if runtime.GOOS == "windows" {
		return fmt.Sprintf("Shell: %s (use Unix shell syntax, not Windows — e.g., /dev/null not NUL, forward slashes in paths)", shellName)
	}
	return fmt.Sprintf("Shell: %s", shellName)
}

// GetUnameSR returns the OS type and release, matching `uname -sr` on POSIX.
// Source: constants/prompts.ts — getUnameSR()
func GetUnameSR() string {
	if runtime.GOOS == "windows" {
		// On Windows, use "ver" for a friendly version string.
		out, err := exec.Command("cmd", "/c", "ver").Output()
		if err == nil {
			return strings.TrimSpace(string(out))
		}
		return "Windows"
	}
	out, err := exec.Command("uname", "-sr").Output()
	if err != nil {
		return runtime.GOOS
	}
	return strings.TrimSpace(string(out))
}

// ComputeSimpleEnvInfo builds the "# Environment" section of the system prompt.
// Source: constants/prompts.ts — computeSimpleEnvInfo()
func ComputeSimpleEnvInfo(cwd, modelID string, isGit bool, isWorktree bool, additionalDirs []string) string {
	marketingName := provider.GetMarketingNameForModel(modelID)

	var modelDescription string
	if marketingName != "" {
		modelDescription = fmt.Sprintf("You are powered by the model named %s. The exact model ID is %s.", marketingName, modelID)
	} else {
		modelDescription = fmt.Sprintf("You are powered by the model %s.", modelID)
	}

	cutoff := GetKnowledgeCutoff(modelID)
	var cutoffMsg string
	if cutoff != "" {
		cutoffMsg = fmt.Sprintf("Assistant knowledge cutoff is %s.", cutoff)
	}

	var items []any

	items = append(items, fmt.Sprintf("Primary working directory: %s", cwd))

	if isWorktree {
		items = append(items, "This is a git worktree — an isolated copy of the repository. Run all commands from this directory. Do NOT `cd` to the original repository root.")
	}

	items = append(items, []string{fmt.Sprintf("Is a git repository: %v", isGit)})

	if len(additionalDirs) > 0 {
		items = append(items, "Additional working directories:")
		items = append(items, additionalDirs)
	}

	items = append(items, fmt.Sprintf("Platform: %s", runtime.GOOS))
	items = append(items, GetShellInfoLine())
	items = append(items, fmt.Sprintf("OS Version: %s", GetUnameSR()))

	if modelDescription != "" {
		items = append(items, modelDescription)
	}
	if cutoffMsg != "" {
		items = append(items, cutoffMsg)
	}

	items = append(items, fmt.Sprintf(
		"The most recent Claude model family is Claude 4.5/4.6. Model IDs — Opus 4.6: '%s', Sonnet 4.6: '%s', Haiku 4.5: '%s'. When building AI applications, default to the latest and most capable Claude models.",
		Claude46Or45ModelIDs.Opus, Claude46Or45ModelIDs.Sonnet, Claude46Or45ModelIDs.Haiku,
	))
	items = append(items, "Claude Code is available as a CLI in the terminal, desktop app (Mac/Windows), web app (claude.ai/code), and IDE extensions (VS Code, JetBrains).")
	items = append(items, FastModeExplanation)

	lines := []string{
		"# Environment",
		"You have been invoked in the following environment: ",
	}
	lines = append(lines, PrependBulletsNested(items)...)
	return strings.Join(lines, "\n")
}

// ComputeEnvInfo builds the legacy env-info block (used by enhanceSystemPromptWithEnvDetails).
// Source: constants/prompts.ts — computeEnvInfo()
func ComputeEnvInfo(cwd, modelID string, isGit bool, additionalDirs []string) string {
	marketingName := provider.GetMarketingNameForModel(modelID)

	var modelDescription string
	if marketingName != "" {
		modelDescription = fmt.Sprintf("You are powered by the model named %s. The exact model ID is %s.", marketingName, modelID)
	} else {
		modelDescription = fmt.Sprintf("You are powered by the model %s.", modelID)
	}

	var additionalDirsInfo string
	if len(additionalDirs) > 0 {
		additionalDirsInfo = fmt.Sprintf("Additional working directories: %s\n", strings.Join(additionalDirs, ", "))
	}

	cutoff := GetKnowledgeCutoff(modelID)
	var knowledgeCutoffMessage string
	if cutoff != "" {
		knowledgeCutoffMessage = fmt.Sprintf("\n\nAssistant knowledge cutoff is %s.", cutoff)
	}

	isGitStr := "No"
	if isGit {
		isGitStr = "Yes"
	}

	return fmt.Sprintf(`Here is useful information about the environment you are running in:
<env>
Working directory: %s
Is directory a git repo: %s
%sPlatform: %s
%s
OS Version: %s
</env>
%s%s`, cwd, isGitStr, additionalDirsInfo, runtime.GOOS, GetShellInfoLine(), GetUnameSR(), modelDescription, knowledgeCutoffMessage)
}

// GetOutputStyleSection returns the output style instruction block for the
// system prompt, or "" when no custom style is active. The cwd is used to
// discover project-level .claude/output-styles/*.md files.
// Source: constants/outputStyles.ts — getOutputStyleConfig → prompt injection
func GetOutputStyleSection(cwd string, styleName string) string {
	if styleName == "" || styleName == "default" {
		return ""
	}
	styles := output_styles.GetOutputStyleDirStyles(cwd)
	for _, s := range styles {
		if s.Name == styleName {
			return "# Output Style: " + s.Name + "\n\n" + s.Prompt
		}
	}
	return ""
}

// EnhanceSystemPromptWithEnvDetails appends agent notes + env info to an existing prompt.
// Source: constants/prompts.ts — enhanceSystemPromptWithEnvDetails()
func EnhanceSystemPromptWithEnvDetails(existingPrompt []string, modelID string, cwd string, isGit bool, additionalDirs []string) []string {
	notes := `Notes:
- Agent threads always have their cwd reset between bash calls, as a result please only use absolute file paths.
- In your final response, share file paths (always absolute, never relative) that are relevant to the task. Include code snippets only when the exact text is load-bearing (e.g., a bug you found, a function signature the caller asked for) — do not recap code you merely read.
- For clear communication with the user the assistant MUST avoid using emojis.
- Do not use a colon before tool calls. Text like "Let me read the file:" followed by a read tool call should just be "Let me read the file." with a period.`

	envInfo := ComputeEnvInfo(cwd, modelID, isGit, additionalDirs)

	result := make([]string, 0, len(existingPrompt)+2)
	result = append(result, existingPrompt...)
	result = append(result, notes, envInfo)
	return result
}
