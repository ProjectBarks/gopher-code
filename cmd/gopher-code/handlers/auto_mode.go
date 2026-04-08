// Package handlers implements CLI subcommand handlers for gopher-code.
// Source: cli/handlers/autoMode.ts
package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"github.com/projectbarks/gopher-code/pkg/provider"
)

// AutoModeRules is the shape of the settings.autoMode config: three classifier
// prompt sections a user can customize.
// Source: utils/permissions/yoloClassifier.ts:85-89
type AutoModeRules struct {
	Allow       []string `json:"allow"`
	SoftDeny    []string `json:"soft_deny"`
	Environment []string `json:"environment"`
}

// AutoModeConfig is the optional-field variant read from settings. Empty/nil
// slices indicate the section was not configured by the user.
type AutoModeConfig struct {
	Allow       []string `json:"allow,omitempty"`
	SoftDeny    []string `json:"soft_deny,omitempty"`
	Environment []string `json:"environment,omitempty"`
}

// SideQueryFunc is the signature for sending a side query to a model.
// Abstracted for testability.
type SideQueryFunc func(ctx context.Context, opts SideQueryOptions) (*SideQueryResponse, error)

// SideQueryOptions configures a side query request.
type SideQueryOptions struct {
	QuerySource           string
	Model                 string
	System                string
	SkipSystemPromptPrefix bool
	MaxTokens             int
	Messages              []SideQueryMessage
}

// SideQueryMessage is a single message in a side query.
type SideQueryMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ContentBlock represents a block in a model response.
type ContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// SideQueryResponse is the response from a side query.
type SideQueryResponse struct {
	Content []ContentBlock `json:"content"`
}

// --- External permissions template ---
// Source: yolo-classifier-prompts/permissions_external.txt
// The external permissions template wraps each section's defaults in
// <user_*_to_replace> tags. User settings REPLACE these defaults.

const externalPermissionsTemplate = `## Auto-approve rules
<user_allow_rules_to_replace>
- Read files in the project directory
- List directory contents
- Search for patterns in files
- Run safe shell commands (ls, cat, head, tail, wc, grep, find, echo, date, pwd)
- Create new files in the project directory
- Apply edits to existing files in the project directory
</user_allow_rules_to_replace>

## Soft-deny rules
<user_deny_rules_to_replace>
- Execute shell commands that modify system state (rm, mv outside project, chmod, chown)
- Execute shell commands that install packages or modify global state (npm i -g, pip install, brew install)
- Execute shell commands that access the network (curl, wget, ssh)
- Run commands with sudo or elevated privileges
- Modify files outside the project directory
- Access environment variables containing secrets or credentials
</user_deny_rules_to_replace>

## Environment context
<user_environment_to_replace>
- The user is working in a software development project
- The project may have a build system and test suite
- The user expects safe, non-destructive operations by default
</user_environment_to_replace>`

// basePrompt is the outer wrapper that includes the permissions template.
const basePrompt = `You are an auto-mode classifier for Claude Code. Your job is to decide
whether tool calls should be auto-approved or require user confirmation.

<permissions_template>` + externalPermissionsTemplate + `</permissions_template>

Use these rules to classify each tool call.`

// extractTaggedBullets parses bullet items from a <tagName>...</tagName> block
// in the external permissions template.
// Source: utils/permissions/yoloClassifier.ts:108-118
func extractTaggedBullets(tagName string) []string {
	re := regexp.MustCompile(`<` + tagName + `>([\s\S]*?)</` + tagName + `>`)
	match := re.FindStringSubmatch(externalPermissionsTemplate)
	if match == nil {
		return nil
	}
	var bullets []string
	for _, line := range strings.Split(match[1], "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "- ") {
			bullets = append(bullets, line[2:])
		}
	}
	return bullets
}

// GetDefaultExternalAutoModeRules returns the default auto-mode rules parsed
// from the external permissions template.
// Source: utils/permissions/yoloClassifier.ts:100-106
func GetDefaultExternalAutoModeRules() AutoModeRules {
	return AutoModeRules{
		Allow:       extractTaggedBullets("user_allow_rules_to_replace"),
		SoftDeny:    extractTaggedBullets("user_deny_rules_to_replace"),
		Environment: extractTaggedBullets("user_environment_to_replace"),
	}
}

// BuildDefaultExternalSystemPrompt returns the full external classifier system
// prompt with default rules (no user overrides). Tags are stripped and their
// contents kept as-is.
// Source: utils/permissions/yoloClassifier.ts:125-142
func BuildDefaultExternalSystemPrompt() string {
	s := basePrompt
	// Strip the wrapping tags, keeping their inner content.
	for _, tag := range []string{
		"user_allow_rules_to_replace",
		"user_deny_rules_to_replace",
		"user_environment_to_replace",
	} {
		re := regexp.MustCompile(`<` + tag + `>([\s\S]*?)</` + tag + `>`)
		s = re.ReplaceAllString(s, "$1")
	}
	return s
}

// MergeAutoModeConfig applies per-section REPLACE semantics: if the user
// section is non-empty it replaces the corresponding default section entirely;
// otherwise the default section is used.
// Source: cli/handlers/autoMode.ts:36-47
func MergeAutoModeConfig(cfg *AutoModeConfig, defaults AutoModeRules) AutoModeRules {
	merged := AutoModeRules{
		Allow:       defaults.Allow,
		SoftDeny:    defaults.SoftDeny,
		Environment: defaults.Environment,
	}
	if cfg != nil {
		if len(cfg.Allow) > 0 {
			merged.Allow = cfg.Allow
		}
		if len(cfg.SoftDeny) > 0 {
			merged.SoftDeny = cfg.SoftDeny
		}
		if len(cfg.Environment) > 0 {
			merged.Environment = cfg.Environment
		}
	}
	return merged
}

// writeRulesJSON writes rules as 2-space-indented JSON to w.
func writeRulesJSON(w io.Writer, rules AutoModeRules) error {
	data, err := json.MarshalIndent(rules, "", "  ")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "%s\n", data)
	return err
}

// --- Handlers ---

// AutoModeDefaultsHandler prints the default auto-mode rules as JSON.
// Source: cli/handlers/autoMode.ts:24-26
func AutoModeDefaultsHandler() {
	writeRulesJSON(os.Stdout, GetDefaultExternalAutoModeRules()) //nolint:errcheck
}

// AutoModeConfigHandler prints the effective auto-mode config: user settings
// where provided, external defaults otherwise. Per-section REPLACE semantics.
// Source: cli/handlers/autoMode.ts:35-47
func AutoModeConfigHandler(cfg *AutoModeConfig) {
	defaults := GetDefaultExternalAutoModeRules()
	writeRulesJSON(os.Stdout, MergeAutoModeConfig(cfg, defaults)) //nolint:errcheck
}

// critiqueSystemPrompt is the system prompt for the critique side query.
// Source: cli/handlers/autoMode.ts:49-71
const critiqueSystemPrompt = "You are an expert reviewer of auto mode classifier rules for Claude Code.\n" +
	"\n" +
	"Claude Code has an \"auto mode\" that uses an AI classifier to decide whether " +
	"tool calls should be auto-approved or require user confirmation. Users can " +
	"write custom rules in three categories:\n" +
	"\n" +
	"- **allow**: Actions the classifier should auto-approve\n" +
	"- **soft_deny**: Actions the classifier should block (require user confirmation)\n" +
	"- **environment**: Context about the user's setup that helps the classifier make decisions\n" +
	"\n" +
	"Your job is to critique the user's custom rules for clarity, completeness, " +
	"and potential issues. The classifier is an LLM that reads these rules as " +
	"part of its system prompt.\n" +
	"\n" +
	"For each rule, evaluate:\n" +
	"1. **Clarity**: Is the rule unambiguous? Could the classifier misinterpret it?\n" +
	"2. **Completeness**: Are there gaps or edge cases the rule doesn't cover?\n" +
	"3. **Conflicts**: Do any of the rules conflict with each other?\n" +
	"4. **Actionability**: Is the rule specific enough for the classifier to act on?\n" +
	"\n" +
	"Be concise and constructive. Only comment on rules that could be improved. " +
	"If all rules look good, say so."

// FormatRulesForCritique formats a single section's user rules alongside the
// defaults they replace. Returns "" if the user has no rules for this section.
// Source: cli/handlers/autoMode.ts:151-170
func FormatRulesForCritique(section string, userRules, defaultRules []string) string {
	if len(userRules) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("## " + section + " (custom rules replacing defaults)\n")
	b.WriteString("Custom:\n")
	for _, r := range userRules {
		b.WriteString("- " + r + "\n")
	}
	b.WriteString("\nDefaults being replaced:\n")
	for _, r := range defaultRules {
		b.WriteString("- " + r + "\n")
	}
	b.WriteString("\n")
	return b.String()
}

// AutoModeCritiqueHandler sends the user's custom rules to a model for critique.
// Source: cli/handlers/autoMode.ts:73-149
func AutoModeCritiqueHandler(ctx context.Context, cfg *AutoModeConfig, model string, sideQuery SideQueryFunc) error {
	hasCustomRules := cfg != nil &&
		(len(cfg.Allow) > 0 || len(cfg.SoftDeny) > 0 || len(cfg.Environment) > 0)

	if !hasCustomRules {
		fmt.Fprint(os.Stdout,
			"No custom auto mode rules found.\n\n"+
				"Add rules to your settings file under autoMode.{allow, soft_deny, environment}.\n"+
				"Run `claude auto-mode defaults` to see the default rules for reference.\n")
		return nil
	}

	defaults := GetDefaultExternalAutoModeRules()
	classifierPrompt := BuildDefaultExternalSystemPrompt()

	allow := cfg.Allow
	if allow == nil {
		allow = []string{}
	}
	softDeny := cfg.SoftDeny
	if softDeny == nil {
		softDeny = []string{}
	}
	env := cfg.Environment
	if env == nil {
		env = []string{}
	}

	userRulesSummary := FormatRulesForCritique("allow", allow, defaults.Allow) +
		FormatRulesForCritique("soft_deny", softDeny, defaults.SoftDeny) +
		FormatRulesForCritique("environment", env, defaults.Environment)

	fmt.Fprint(os.Stdout, "Analyzing your auto mode rules\u2026\n\n")

	resp, err := sideQuery(ctx, SideQueryOptions{
		QuerySource:            "auto_mode_critique",
		Model:                  model,
		System:                 critiqueSystemPrompt,
		SkipSystemPromptPrefix: true,
		MaxTokens:              4096,
		Messages: []SideQueryMessage{
			{
				Role: "user",
				Content: "Here is the full classifier system prompt that the auto mode classifier receives:\n\n" +
					"<classifier_system_prompt>\n" +
					classifierPrompt +
					"\n</classifier_system_prompt>\n\n" +
					"Here are the user's custom rules that REPLACE the corresponding default sections:\n\n" +
					userRulesSummary +
					"\nPlease critique these custom rules.",
			},
		},
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to analyze rules: %v\n", err)
		return fmt.Errorf("auto-mode critique failed: %w", err)
	}

	for _, block := range resp.Content {
		if block.Type == "text" {
			fmt.Fprintln(os.Stdout, block.Text)
			return nil
		}
	}

	fmt.Fprint(os.Stdout, "No critique was generated. Please try again.\n")
	return nil
}

// NewProviderSideQuery creates a SideQueryFunc that delegates to provider.QueryWithModel,
// bridging the handler-level side query types to the real provider infrastructure.
// Source: services/api/claude.ts:3300-3348
func NewProviderSideQuery(prov provider.ModelProvider) SideQueryFunc {
	return func(ctx context.Context, opts SideQueryOptions) (*SideQueryResponse, error) {
		// Build system prompt blocks
		var systemBlocks []string
		if opts.System != "" {
			systemBlocks = []string{opts.System}
		}

		// Build user prompt from messages (side queries typically have a single user message)
		var userPrompt string
		for _, m := range opts.Messages {
			if m.Role == "user" {
				userPrompt = m.Content
				break
			}
		}

		maxTokens := opts.MaxTokens
		if maxTokens <= 0 {
			maxTokens = provider.MaxNonStreamingTokens
		}

		result, err := provider.QueryWithModel(ctx, prov, provider.QueryWithModelRequest{
			SystemPrompt: systemBlocks,
			UserPrompt:   userPrompt,
			Options: provider.QueryOptions{
				Model:           opts.Model,
				QuerySource:     provider.QuerySource(opts.QuerySource),
				MaxOutputTokens: maxTokens,
			},
		})
		if err != nil {
			return nil, err
		}

		// Convert provider response to handler response
		resp := &SideQueryResponse{}
		for _, c := range result.Response.Content {
			resp.Content = append(resp.Content, ContentBlock{
				Type: c.Type,
				Text: c.Text,
			})
		}
		return resp, nil
	}
}
