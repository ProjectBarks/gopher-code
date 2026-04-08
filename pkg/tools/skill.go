package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/projectbarks/gopher-code/pkg/message"
	"github.com/projectbarks/gopher-code/pkg/skills"
)

// Budget constants for skill listing in the system prompt.
// Source: tools/SkillTool/prompt.ts:21-29
const (
	SkillBudgetContextPercent = 0.01
	CharsPerToken             = 4
	DefaultCharBudget         = 8_000 // 1% of 200k × 4
	MaxListingDescChars       = 250
	MinDescLength             = 20
)

// commandNameTag references the canonical XML tag constant for already-loaded skills.
// Source: constants/xml.ts — COMMAND_NAME_TAG
var commandNameTag = message.CommandNameTag

// SkillTool executes a skill (prompt-based command).
type SkillTool struct {
	skills []skills.Skill
}

// NewSkillTool creates a SkillTool with the given loaded skills.
func NewSkillTool(s []skills.Skill) *SkillTool {
	return &SkillTool{skills: s}
}

func (t *SkillTool) Name() string        { return "Skill" }
func (t *SkillTool) Description() string { return "Execute a skill within the main conversation" }
func (t *SkillTool) IsReadOnly() bool    { return true }

func (t *SkillTool) SearchHint() string { return "invoke a slash-command skill" }

func (t *SkillTool) MaxResultSizeChars() int { return 100_000 }

func (t *SkillTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
	"type": "object",
	"properties": {
		"skill": {"type": "string", "description": "The skill name. E.g., \"commit\", \"review-pr\", or \"pdf\""},
		"args": {"type": "string", "description": "Optional arguments for the skill"}
	},
	"required": ["skill"],
	"additionalProperties": false
}`)
}

// Prompt implements ToolPrompter and returns the verbatim system-prompt section
// guiding the model on when/how to use the Skill tool.
// Source: tools/SkillTool/prompt.ts:173-196
func (t *SkillTool) Prompt() string {
	return `Execute a skill within the main conversation

When users ask you to perform tasks, check if any of the available skills match. Skills provide specialized capabilities and domain knowledge.

When users reference a "slash command" or "/<something>" (e.g., "/commit", "/review-pr"), they are referring to a skill. Use this tool to invoke it.

How to invoke:
- Use this tool with the skill name and optional arguments
- Examples:
  - ` + "`" + `skill: "pdf"` + "`" + ` - invoke the pdf skill
  - ` + "`" + `skill: "commit", args: "-m 'Fix bug'"` + "`" + ` - invoke with arguments
  - ` + "`" + `skill: "review-pr", args: "123"` + "`" + ` - invoke with arguments
  - ` + "`" + `skill: "ms-office-suite:pdf"` + "`" + ` - invoke using fully qualified name

Important:
- Available skills are listed in system-reminder messages in the conversation
- When a skill matches the user's request, this is a BLOCKING REQUIREMENT: invoke the relevant Skill tool BEFORE generating any other response about the task
- NEVER mention a skill without actually calling this tool
- Do not invoke a skill that is already running
- Do not use this tool for built-in CLI commands (like /help, /clear, etc.)
- If you see a <` + commandNameTag + `> tag in the current conversation turn, the skill has ALREADY been loaded - follow the instructions directly instead of calling this tool again
`
}

// GetCharBudget returns the character budget for skill listings.
// Source: tools/SkillTool/prompt.ts:31-41
func GetCharBudget(contextWindowTokens int) int {
	if contextWindowTokens > 0 {
		return int(math.Floor(float64(contextWindowTokens) * float64(CharsPerToken) * SkillBudgetContextPercent))
	}
	return DefaultCharBudget
}

// GetSkillDescription returns the display description for a skill, with
// optional whenToUse appended, capped at MaxListingDescChars.
// Source: tools/SkillTool/prompt.ts:43-50
func GetSkillDescription(s skills.Skill) string {
	desc := s.Description
	if s.WhenToUse != "" {
		desc = desc + " - " + s.WhenToUse
	}
	runes := []rune(desc)
	if len(runes) > MaxListingDescChars {
		return string(runes[:MaxListingDescChars-1]) + "\u2026"
	}
	return desc
}

// FormatSkillListing formats a single skill entry.
// Source: tools/SkillTool/prompt.ts:52-66 — "- {name}: {description}"
func FormatSkillListing(s skills.Skill) string {
	return "- " + s.Name + ": " + GetSkillDescription(s)
}

// FormatSkillsWithinBudget formats all skills within the character budget.
// Bundled skills always keep full descriptions; non-bundled are truncated to fit.
// Source: tools/SkillTool/prompt.ts:70-171
func FormatSkillsWithinBudget(ss []skills.Skill, contextWindowTokens int) string {
	if len(ss) == 0 {
		return ""
	}

	budget := GetCharBudget(contextWindowTokens)

	// Try full descriptions first
	fullEntries := make([]string, len(ss))
	fullTotal := 0
	for i, s := range ss {
		fullEntries[i] = FormatSkillListing(s)
		fullTotal += len(fullEntries[i])
	}
	// join('\n') produces N-1 newlines
	fullTotal += len(ss) - 1

	if fullTotal <= budget {
		return strings.Join(fullEntries, "\n")
	}

	// Partition into bundled (never truncated) and rest
	bundledSet := make(map[int]bool)
	var restIndices []int
	for i, s := range ss {
		if s.Source == "bundled" {
			bundledSet[i] = true
		} else {
			restIndices = append(restIndices, i)
		}
	}

	// Bundled chars (full descriptions + newlines)
	bundledChars := 0
	for i := range ss {
		if bundledSet[i] {
			bundledChars += len(fullEntries[i]) + 1
		}
	}
	remainingBudget := budget - bundledChars

	if len(restIndices) == 0 {
		return strings.Join(fullEntries, "\n")
	}

	// Calculate max description length for non-bundled commands
	restNameOverhead := 0
	for _, i := range restIndices {
		restNameOverhead += len(ss[i].Name) + 4 // "- " + ": "
	}
	restNameOverhead += len(restIndices) - 1 // newlines
	availableForDescs := remainingBudget - restNameOverhead
	maxDescLen := availableForDescs / len(restIndices)

	if maxDescLen < MinDescLength {
		// Extreme: non-bundled go names-only, bundled keep descriptions
		parts := make([]string, len(ss))
		for i, s := range ss {
			if bundledSet[i] {
				parts[i] = fullEntries[i]
			} else {
				parts[i] = "- " + s.Name
			}
		}
		return strings.Join(parts, "\n")
	}

	// Truncate non-bundled descriptions to fit
	parts := make([]string, len(ss))
	for i, s := range ss {
		if bundledSet[i] {
			parts[i] = fullEntries[i]
		} else {
			desc := GetSkillDescription(s)
			descRunes := []rune(desc)
			if len(descRunes) > maxDescLen {
				desc = string(descRunes[:maxDescLen-1]) + "\u2026"
			}
			parts[i] = "- " + s.Name + ": " + desc
		}
	}
	return strings.Join(parts, "\n")
}

// normalizeSkillName strips a leading slash if present.
// Source: SkillTool.ts:370-372, 440, 598
func normalizeSkillName(name string) string {
	trimmed := strings.TrimSpace(name)
	if strings.HasPrefix(trimmed, "/") {
		return trimmed[1:]
	}
	return trimmed
}

// findSkill looks up a skill by normalized name, supporting both exact match
// and fully-qualified "package:skill" names.
func (t *SkillTool) findSkill(name string) *skills.Skill {
	normalized := normalizeSkillName(name)
	for i, s := range t.skills {
		if s.Name == normalized {
			return &t.skills[i]
		}
	}
	return nil
}

func (t *SkillTool) Execute(_ context.Context, _ *ToolContext, input json.RawMessage) (*ToolOutput, error) {
	var params struct {
		Skill string `json:"skill"`
		Args  string `json:"args"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return ErrorOutput(fmt.Sprintf("invalid input: %s", err)), nil
	}
	if strings.TrimSpace(params.Skill) == "" {
		return ErrorOutput("skill name is required"), nil
	}

	normalized := normalizeSkillName(params.Skill)

	s := t.findSkill(normalized)
	if s == nil {
		return ErrorOutput(fmt.Sprintf("Unknown skill: %s", normalized)), nil
	}

	// Check disableModelInvocation — Source: SkillTool.ts:412-418
	if s.DisableModelInvocation {
		return ErrorOutput(fmt.Sprintf("Skill %s cannot be used with Skill tool due to disable-model-invocation", normalized)), nil
	}

	// Substitute $ARGUMENTS placeholders — Source: utils/argumentSubstitution.ts
	prompt := SubstituteArguments(s.Prompt, params.Args, true, s.ArgumentNames)

	return SuccessOutput(prompt), nil
}

// SubstituteArguments replaces $ARGUMENTS placeholders in content with actual values.
// Source: utils/argumentSubstitution.ts:94-145
//
// Supports:
//   - $ARGUMENTS — replaced with the full arguments string
//   - $ARGUMENTS[0], $ARGUMENTS[1] — individual indexed arguments
//   - $0, $1 — shorthand for $ARGUMENTS[0], $ARGUMENTS[1]
//   - Named arguments ($foo, $bar) — when argument names are defined in frontmatter
//
// If no placeholders are found and appendIfNoPlaceholder is true, appends
// "ARGUMENTS: {args}" to the content.
func SubstituteArguments(content, args string, appendIfNoPlaceholder bool, argumentNames []string) string {
	// Empty args means no substitution needed
	if args == "" {
		return content
	}

	parsedArgs := parseShellArguments(args)
	original := content

	// Replace named arguments ($foo, $bar) with their positional values
	// Source: argumentSubstitution.ts:111-121
	for i, name := range argumentNames {
		if name == "" {
			continue
		}
		val := ""
		if i < len(parsedArgs) {
			val = parsedArgs[i]
		}
		content = replaceNamedArg(content, name, val)
	}

	// Replace $ARGUMENTS[N] — Source: argumentSubstitution.ts:124-127
	content = replaceIndexedArgs(content, parsedArgs)

	// Replace $N shorthand — Source: argumentSubstitution.ts:130-133
	content = replaceShorthandArgs(content, parsedArgs)

	// Replace $ARGUMENTS with full args string — Source: argumentSubstitution.ts:136
	content = strings.ReplaceAll(content, "$ARGUMENTS", args)

	// If no placeholders were found, append — Source: argumentSubstitution.ts:140-142
	if content == original && appendIfNoPlaceholder && args != "" {
		content = content + "\n\nARGUMENTS: " + args
	}

	return content
}

// isWordChar returns true if c is a word character [a-zA-Z0-9_].
func isWordChar(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_'
}

// replaceNamedArg replaces $name in content, but only when not followed by
// a word char or '['. Go regexp lacks negative lookahead so we do it manually.
func replaceNamedArg(content, name, val string) string {
	needle := "$" + name
	var b strings.Builder
	i := 0
	for i < len(content) {
		idx := strings.Index(content[i:], needle)
		if idx < 0 {
			b.WriteString(content[i:])
			break
		}
		pos := i + idx
		after := pos + len(needle)
		// Check that the char after the match is not a word char or '['
		if after < len(content) && (isWordChar(content[after]) || content[after] == '[') {
			b.WriteString(content[i : pos+len(needle)])
			i = pos + len(needle)
			continue
		}
		b.WriteString(content[i:pos])
		b.WriteString(val)
		i = after
	}
	return b.String()
}

// indexedArgRe matches $ARGUMENTS[N].
var indexedArgRe = regexp.MustCompile(`\$ARGUMENTS\[(\d+)\]`)

// replaceIndexedArgs replaces $ARGUMENTS[N] with the Nth parsed argument.
func replaceIndexedArgs(content string, parsedArgs []string) string {
	return indexedArgRe.ReplaceAllStringFunc(content, func(match string) string {
		sub := indexedArgRe.FindStringSubmatch(match)
		if len(sub) < 2 {
			return match
		}
		idx, err := strconv.Atoi(sub[1])
		if err != nil || idx >= len(parsedArgs) {
			return ""
		}
		return parsedArgs[idx]
	})
}

// shorthandArgRe matches $N (digits only, captured).
var shorthandArgRe = regexp.MustCompile(`\$(\d+)`)

// replaceShorthandArgs replaces $N with the Nth parsed argument, but only
// when not followed by a word character (since Go lacks negative lookahead).
func replaceShorthandArgs(content string, parsedArgs []string) string {
	// Use FindAllStringSubmatchIndex for position-aware replacement
	matches := shorthandArgRe.FindAllStringSubmatchIndex(content, -1)
	if len(matches) == 0 {
		return content
	}
	var b strings.Builder
	prev := 0
	for _, loc := range matches {
		// loc[0]:loc[1] is the full match, loc[2]:loc[3] is the digit group
		end := loc[1]
		// Check char after match
		if end < len(content) && isWordChar(content[end]) {
			continue // skip — followed by word char
		}
		b.WriteString(content[prev:loc[0]])
		digitStr := content[loc[2]:loc[3]]
		idx, err := strconv.Atoi(digitStr)
		if err != nil || idx >= len(parsedArgs) {
			// out of bounds → empty string
		} else {
			b.WriteString(parsedArgs[idx])
		}
		prev = end
	}
	b.WriteString(content[prev:])
	return b.String()
}

// parseShellArguments splits an argument string respecting quoted substrings.
// Source: argumentSubstitution.ts:24-39 — uses shell-quote; we approximate
// with a simple quoted-string-aware tokenizer.
func parseShellArguments(args string) []string {
	args = strings.TrimSpace(args)
	if args == "" {
		return nil
	}

	var result []string
	var current strings.Builder
	inSingle := false
	inDouble := false

	for i := 0; i < len(args); i++ {
		ch := args[i]
		switch {
		case ch == '\'' && !inDouble:
			inSingle = !inSingle
		case ch == '"' && !inSingle:
			inDouble = !inDouble
		case (ch == ' ' || ch == '\t') && !inSingle && !inDouble:
			if current.Len() > 0 {
				result = append(result, current.String())
				current.Reset()
			}
		default:
			current.WriteByte(ch)
		}
	}
	if current.Len() > 0 {
		result = append(result, current.String())
	}
	return result
}
