package tools

import (
	"regexp"
	"strings"
)

// DestructivePattern pairs a regex with a human-readable warning.
// Source: destructiveCommandWarning.ts:7-9
type DestructivePattern struct {
	Pattern *regexp.Regexp
	Warning string
}

// destructivePatterns lists patterns that match dangerous/irreversible commands.
// Source: destructiveCommandWarning.ts:13-89 — DESTRUCTIVE_PATTERNS
var destructivePatterns = []DestructivePattern{
	// Git — data loss / hard to reverse
	{regexp.MustCompile(`\bgit\s+reset\s+--hard\b`), "Note: may discard uncommitted changes"},
	{regexp.MustCompile(`\bgit\s+push\b[^;&|\n]*[ \t](--force|--force-with-lease|-f)\b`), "Note: may overwrite remote history"},
	// git clean -f without dry-run: handled via custom check below (Go regexp lacks lookahead)
	{regexp.MustCompile(`\bgit\s+checkout\s+(--\s+)?\.[ \t]*($|[;&|\n])`), "Note: may discard all working tree changes"},
	{regexp.MustCompile(`\bgit\s+restore\s+(--\s+)?\.[ \t]*($|[;&|\n])`), "Note: may discard all working tree changes"},
	{regexp.MustCompile(`\bgit\s+stash[ \t]+(drop|clear)\b`), "Note: may permanently remove stashed changes"},
	{regexp.MustCompile(`\bgit\s+branch\s+(-D[ \t]|--delete\s+--force|--force\s+--delete)\b`), "Note: may force-delete a branch"},

	// Git — safety bypass
	{regexp.MustCompile(`\bgit\s+(commit|push|merge)\b[^;&|\n]*--no-verify\b`), "Note: may skip safety hooks"},
	{regexp.MustCompile(`\bgit\s+commit\b[^;&|\n]*--amend\b`), "Note: may rewrite the last commit"},

	// File deletion
	{regexp.MustCompile(`(^|[;&|\n]\s*)rm\s+-[a-zA-Z]*[rR][a-zA-Z]*f|(^|[;&|\n]\s*)rm\s+-[a-zA-Z]*f[a-zA-Z]*[rR]`), "Note: may recursively force-remove files"},
	{regexp.MustCompile(`(^|[;&|\n]\s*)rm\s+-[a-zA-Z]*[rR]`), "Note: may recursively remove files"},
	{regexp.MustCompile(`(^|[;&|\n]\s*)rm\s+-[a-zA-Z]*f`), "Note: may force-remove files"},

	// Database
	{regexp.MustCompile(`(?i)\b(DROP|TRUNCATE)\s+(TABLE|DATABASE|SCHEMA)\b`), "Note: may drop or truncate database objects"},
	{regexp.MustCompile(`(?i)\bDELETE\s+FROM\s+\w+[ \t]*(;|"|'|\n|$)`), "Note: may delete all rows from a database table"},

	// Infrastructure
	{regexp.MustCompile(`\bkubectl\s+delete\b`), "Note: may delete Kubernetes resources"},
	{regexp.MustCompile(`\bterraform\s+destroy\b`), "Note: may destroy Terraform infrastructure"},
}

// gitCleanForceRe matches `git clean` with `-f` flag.
var gitCleanForceRe = regexp.MustCompile(`\bgit\s+clean\b[^;&|\n]*-[a-zA-Z]*f`)

// gitCleanDryRunRe matches `git clean` with dry-run flag.
var gitCleanDryRunRe = regexp.MustCompile(`\bgit\s+clean\b[^;&|\n]*(-[a-zA-Z]*n|--dry-run)`)

// GetDestructiveCommandWarning checks if a command matches known destructive
// patterns and returns a human-readable warning, or empty string if safe.
// Source: destructiveCommandWarning.ts:95-102
func GetDestructiveCommandWarning(command string) string {
	// Custom check for git clean -f (needs negative lookahead equivalent)
	if gitCleanForceRe.MatchString(command) && !gitCleanDryRunRe.MatchString(command) {
		return "Note: may permanently delete untracked files"
	}

	for _, dp := range destructivePatterns {
		if dp.Pattern.MatchString(command) {
			return dp.Warning
		}
	}
	return ""
}

// CommandSemantic interprets command exit codes with command-specific knowledge.
// Source: commandSemantics.ts:10-17
type CommandSemantic struct {
	IsError bool
	Message string
}

// commandSemanticRules maps base commands to their exit code interpreters.
// Source: commandSemantics.ts:31-89
var commandSemanticRules = map[string]func(exitCode int) CommandSemantic{
	// grep: 0=matches found, 1=no matches, 2+=error
	"grep": func(exitCode int) CommandSemantic {
		if exitCode == 1 {
			return CommandSemantic{IsError: false, Message: "No matches found"}
		}
		return CommandSemantic{IsError: exitCode >= 2}
	},
	"rg": func(exitCode int) CommandSemantic {
		if exitCode == 1 {
			return CommandSemantic{IsError: false, Message: "No matches found"}
		}
		return CommandSemantic{IsError: exitCode >= 2}
	},
	// find: 0=success, 1=partial, 2+=error
	"find": func(exitCode int) CommandSemantic {
		if exitCode == 1 {
			return CommandSemantic{IsError: false, Message: "Some directories were inaccessible"}
		}
		return CommandSemantic{IsError: exitCode >= 2}
	},
	// diff: 0=no differences, 1=differences found, 2+=error
	"diff": func(exitCode int) CommandSemantic {
		if exitCode == 1 {
			return CommandSemantic{IsError: false, Message: "Files differ"}
		}
		return CommandSemantic{IsError: exitCode >= 2}
	},
	// test/[: 0=true, 1=false, 2+=error
	"test": func(exitCode int) CommandSemantic {
		if exitCode == 1 {
			return CommandSemantic{IsError: false, Message: "Condition is false"}
		}
		return CommandSemantic{IsError: exitCode >= 2}
	},
	"[": func(exitCode int) CommandSemantic {
		if exitCode == 1 {
			return CommandSemantic{IsError: false, Message: "Condition is false"}
		}
		return CommandSemantic{IsError: exitCode >= 2}
	},
}

// InterpretCommandResult applies command-specific exit code semantics.
// Source: commandSemantics.ts:124-140
func InterpretCommandResult(command string, exitCode int) CommandSemantic {
	base := extractLastBaseCommand(command)
	if fn, ok := commandSemanticRules[base]; ok {
		return fn(exitCode)
	}
	// Default: only 0 is success
	return CommandSemantic{
		IsError: exitCode != 0,
		Message: func() string {
			if exitCode != 0 {
				return "Command failed with exit code"
			}
			return ""
		}(),
	}
}

// extractLastBaseCommand extracts the base command from the last segment
// of a pipeline (since the last command determines exit code).
// Source: commandSemantics.ts:112-119
func extractLastBaseCommand(command string) string {
	// Split on pipes to get last command
	segments := splitOnPipes(command)
	last := segments[len(segments)-1]
	// Extract first word
	fields := strings.Fields(strings.TrimSpace(last))
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

// splitOnPipes splits a command on pipe characters, respecting quotes.
func splitOnPipes(command string) []string {
	var segments []string
	var current strings.Builder
	inSingle := false
	inDouble := false
	escaped := false

	for i := 0; i < len(command); i++ {
		ch := command[i]

		if escaped {
			escaped = false
			current.WriteByte(ch)
			continue
		}
		if ch == '\\' {
			escaped = true
			current.WriteByte(ch)
			continue
		}
		if ch == '\'' && !inDouble {
			inSingle = !inSingle
			current.WriteByte(ch)
			continue
		}
		if ch == '"' && !inSingle {
			inDouble = !inDouble
			current.WriteByte(ch)
			continue
		}
		if ch == '|' && !inSingle && !inDouble {
			segments = append(segments, current.String())
			current.Reset()
			continue
		}
		current.WriteByte(ch)
	}
	segments = append(segments, current.String())
	return segments
}

// ValidateBashCommand runs the full validation pipeline for a bash command.
// Returns nil if the command is allowed, or a ToolOutput error if rejected.
// Source: BashTool.tsx — pre-execution validation
func ValidateBashCommand(command string, cwd string, projectDir string, planMode bool) *ToolOutput {
	// 1. Plan mode: reject write commands
	if planMode {
		if !IsReadOnlyCommand(command) {
			return ErrorOutput("cannot execute write commands in plan mode — only read-only commands are allowed")
		}
	}

	// 2. Security checks (command injection, env hijack, etc.)
	secResult := CheckCommandSecurity(command)
	if !secResult.Safe {
		return ErrorOutput("command rejected: " + secResult.Reason)
	}

	// 3. Validate working directory doesn't escape project
	if cwd != "" && projectDir != "" {
		if err := ValidateWorkingDirectory(cwd, projectDir); err != nil {
			return ErrorOutput("working directory rejected: " + err.Error())
		}
	}

	return nil // All checks passed
}
