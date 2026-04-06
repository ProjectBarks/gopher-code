package tools

import (
	"path/filepath"
	"regexp"
	"strings"

	"mvdan.cc/sh/v3/syntax"
)

// Security constants matching TS source.
// Source: bashPermissions.ts:1-3
const (
	MaxSubcommandsForSecurityCheck = 50
	MaxSuggestedRulesForCompound   = 5
)

// binaryHijackVarsRe matches environment variable names that could hijack
// binary loading (LD_PRELOAD, DYLD_INSERT_LIBRARIES, PATH, etc.).
// Source: bashPermissions.ts — BINARY_HIJACK_VARS = /^(LD_|DYLD_|PATH$)/
var binaryHijackVarsRe = regexp.MustCompile(`^(LD_|DYLD_|PATH$)`)

// commandSubstitutionPatterns detects shell constructs that can execute
// arbitrary code inside what appears to be a simple command.
// Source: bashSecurity.ts:16-41 — COMMAND_SUBSTITUTION_PATTERNS
var commandSubstitutionPatterns = []struct {
	pattern *regexp.Regexp
	message string
}{
	{regexp.MustCompile(`<\(`), "process substitution <()"},
	{regexp.MustCompile(`>\(`), "process substitution >()"},
	{regexp.MustCompile(`=\(`), "Zsh process substitution =()"},
	{regexp.MustCompile(`(?:^|[\s;&|])=[a-zA-Z_]`), "Zsh equals expansion (=cmd)"},
	{regexp.MustCompile(`\$\(`), "$() command substitution"},
	{regexp.MustCompile(`\$\{`), "${} parameter substitution"},
	{regexp.MustCompile(`\$\[`), "$[] legacy arithmetic expansion"},
	{regexp.MustCompile(`~\[`), "Zsh-style parameter expansion"},
	{regexp.MustCompile(`\(e:`), "Zsh-style glob qualifiers"},
	{regexp.MustCompile(`\(\+`), "Zsh glob qualifier with command execution"},
	{regexp.MustCompile(`\}\s*always\s*\{`), "Zsh always block (try/always construct)"},
	{regexp.MustCompile(`<#`), "PowerShell comment syntax"},
}

// zshDangerousCommands are Zsh-specific commands that bypass security.
// Source: bashSecurity.ts:45-74 — ZSH_DANGEROUS_COMMANDS
var zshDangerousCommands = map[string]bool{
	"zmodload": true, "emulate": true,
	"sysopen": true, "sysread": true, "syswrite": true, "sysseek": true,
	"zpty": true, "ztcp": true, "zsocket": true, "mapfile": true,
	"zf_rm": true, "zf_mv": true, "zf_ln": true, "zf_chmod": true,
	"zf_chown": true, "zf_mkdir": true, "zf_rmdir": true, "zf_chgrp": true,
}

// SecurityCheckResult describes the outcome of a security analysis.
type SecurityCheckResult struct {
	Safe    bool   // true if the command passed all checks
	Reason  string // human-readable reason for rejection
	CheckID int    // numeric identifier for the failing check (0 = passed)
}

// CheckCommandSecurity runs fail-closed security checks on a bash command.
// Uses AST-based analysis via mvdan.cc/sh/v3 where possible, falling back
// to pattern matching for constructs the parser doesn't surface.
// Source: bashSecurity.ts — stripSafeHeredocSubstitutions + validateDangerousPatterns
func CheckCommandSecurity(command string) SecurityCheckResult {
	// 1. Strip heredoc content before analysis (safe inner content)
	stripped := StripSafeHeredocContent(command)

	// 2. Check for command substitution patterns in unquoted content
	if result := checkCommandSubstitutions(stripped); !result.Safe {
		return result
	}

	// 3. Check for dangerous Zsh commands via AST
	if result := checkZshDangerousCommands(command); !result.Safe {
		return result
	}

	// 4. Check for binary hijack env vars via AST
	if result := checkBinaryHijackVars(command); !result.Safe {
		return result
	}

	return SecurityCheckResult{Safe: true}
}

// StripSafeHeredocContent replaces the body of heredoc blocks with empty
// content, keeping the delimiters. This prevents heredoc payloads from
// triggering false positives in pattern-based security checks.
// Source: bashSecurity.ts — stripSafeHeredocSubstitutions
//
// Example: `cat <<EOF\nmalicious $(rm -rf /)\nEOF` → `cat <<EOF\nEOF`
func StripSafeHeredocContent(command string) string {
	reader := strings.NewReader(command)
	parser := syntax.NewParser(syntax.KeepComments(false))

	file, err := parser.Parse(reader, "")
	if err != nil {
		return command // Parse error → return original (fail-closed elsewhere)
	}

	// Collect heredoc bodies to strip (process in reverse order to preserve offsets)
	type region struct{ start, end int }
	var regions []region

	syntax.Walk(file, func(node syntax.Node) bool {
		if redirect, ok := node.(*syntax.Redirect); ok {
			if redirect.Hdoc != nil {
				// The Hdoc word contains the heredoc body content
				hdoc := redirect.Hdoc
				if hdoc.Pos().IsValid() && hdoc.End().IsValid() {
					start := int(hdoc.Pos().Offset())
					end := int(hdoc.End().Offset())
					if start < end && start < len(command) {
						regions = append(regions, region{start, end})
					}
				}
			}
		}
		return true
	})

	if len(regions) == 0 {
		return command
	}

	// Strip from end to start to preserve offsets
	result := command
	for i := len(regions) - 1; i >= 0; i-- {
		r := regions[i]
		end := r.end
		if end > len(result) {
			end = len(result)
		}
		result = result[:r.start] + result[end:]
	}

	return result
}

// checkCommandSubstitutions scans for shell command substitution patterns
// that could execute arbitrary code.
// Source: bashSecurity.ts:16-41
func checkCommandSubstitutions(command string) SecurityCheckResult {
	// Extract unquoted content (strip single-quoted regions)
	unquoted := extractUnquotedContent(command)

	for _, cs := range commandSubstitutionPatterns {
		if cs.pattern.MatchString(unquoted) {
			return SecurityCheckResult{
				Safe:    false,
				Reason:  "dangerous pattern: " + cs.message,
				CheckID: 8, // DANGEROUS_PATTERNS_COMMAND_SUBSTITUTION
			}
		}
	}

	// Check for unescaped backticks in unquoted content
	if hasUnescapedBacktick(unquoted) {
		return SecurityCheckResult{
			Safe:    false,
			Reason:  "dangerous pattern: backtick command substitution",
			CheckID: 8,
		}
	}

	return SecurityCheckResult{Safe: true}
}

// checkZshDangerousCommands uses AST parsing to detect dangerous Zsh builtins.
// Source: bashSecurity.ts:45-74
func checkZshDangerousCommands(command string) SecurityCheckResult {
	result := ParseShellCommand(command)
	if result.Kind == "parse-error" {
		return SecurityCheckResult{Safe: true} // Let other checks handle parse errors
	}

	for _, cmd := range result.Commands {
		base := strings.Fields(cmd)
		if len(base) > 0 && zshDangerousCommands[base[0]] {
			return SecurityCheckResult{
				Safe:    false,
				Reason:  "dangerous Zsh command: " + base[0],
				CheckID: 20, // ZSH_DANGEROUS_COMMANDS
			}
		}
	}

	return SecurityCheckResult{Safe: true}
}

// checkBinaryHijackVars detects env var assignments that could hijack
// binary loading (LD_PRELOAD=evil.so, DYLD_INSERT_LIBRARIES=..., PATH=...).
// Source: bashPermissions.ts — BINARY_HIJACK_VARS
func checkBinaryHijackVars(command string) SecurityCheckResult {
	reader := strings.NewReader(command)
	parser := syntax.NewParser(syntax.KeepComments(false))

	file, err := parser.Parse(reader, "")
	if err != nil {
		return SecurityCheckResult{Safe: true}
	}

	var found string
	syntax.Walk(file, func(node syntax.Node) bool {
		if found != "" {
			return false
		}
		// Check for env var assignments in call expressions
		if call, ok := node.(*syntax.CallExpr); ok {
			for _, assign := range call.Assigns {
				if assign.Name != nil && binaryHijackVarsRe.MatchString(assign.Name.Value) {
					found = assign.Name.Value
					return false
				}
			}
		}
		return true
	})

	if found != "" {
		return SecurityCheckResult{
			Safe:    false,
			Reason:  "binary hijack variable: " + found,
			CheckID: 6, // DANGEROUS_VARIABLES
		}
	}

	return SecurityCheckResult{Safe: true}
}

// extractUnquotedContent removes single-quoted content from a command string,
// keeping only content that is unquoted or in double quotes (which allow expansions).
// Source: bashSecurity.ts:128-174 — extractQuotedContent (withDoubleQuotes variant)
func extractUnquotedContent(command string) string {
	var sb strings.Builder
	inSingleQuote := false
	inDoubleQuote := false
	escaped := false

	for i := 0; i < len(command); i++ {
		ch := command[i]

		if escaped {
			escaped = false
			if !inSingleQuote {
				sb.WriteByte(ch)
			}
			continue
		}

		if ch == '\\' && !inSingleQuote {
			escaped = true
			if !inSingleQuote {
				sb.WriteByte(ch)
			}
			continue
		}

		if ch == '\'' && !inDoubleQuote {
			inSingleQuote = !inSingleQuote
			continue
		}

		if ch == '"' && !inSingleQuote {
			inDoubleQuote = !inDoubleQuote
			continue
		}

		if !inSingleQuote {
			sb.WriteByte(ch)
		}
	}

	return sb.String()
}

// hasUnescapedBacktick checks for unescaped backtick characters.
// Source: bashSecurity.ts:209-231 — hasUnescapedChar
func hasUnescapedBacktick(content string) bool {
	for i := 0; i < len(content); i++ {
		if content[i] == '\\' && i+1 < len(content) {
			i++ // Skip escaped character
			continue
		}
		if content[i] == '`' {
			return true
		}
	}
	return false
}

// ValidatePath checks that a path is within the allowed working directory.
// Rejects path traversal attempts (../ escapes).
// Source: pathValidation.ts — checkPathConstraints
func ValidatePath(path string, allowedDir string) error {
	if path == "" {
		return nil
	}

	// Resolve to absolute
	absPath := path
	if !filepath.IsAbs(path) {
		absPath = filepath.Join(allowedDir, path)
	}

	// Clean to resolve .. components
	cleaned := filepath.Clean(absPath)

	// Check that the cleaned path is within allowedDir
	allowedCleaned := filepath.Clean(allowedDir)

	if !strings.HasPrefix(cleaned, allowedCleaned+string(filepath.Separator)) && cleaned != allowedCleaned {
		return &PathEscapeError{
			Path:       path,
			Resolved:   cleaned,
			AllowedDir: allowedCleaned,
		}
	}

	return nil
}

// PathEscapeError indicates a path escapes the allowed directory.
type PathEscapeError struct {
	Path       string
	Resolved   string
	AllowedDir string
}

func (e *PathEscapeError) Error() string {
	return "path " + e.Path + " resolves to " + e.Resolved + " which is outside allowed directory " + e.AllowedDir
}

// ValidateWorkingDirectory checks that the specified CWD is valid and within
// the project directory. Empty string means use project default.
// Source: pathValidation.ts — working directory enforcement
func ValidateWorkingDirectory(cwd string, projectDir string) error {
	if cwd == "" {
		return nil
	}
	return ValidatePath(cwd, projectDir)
}

// StripSafeWrappers removes safe command wrappers (env, cd prefix, time, etc.)
// to expose the actual command for permission checking.
// Source: bashPermissions.ts — stripSafeWrappers
func StripSafeWrappers(command string) string {
	trimmed := strings.TrimSpace(command)

	// Strip leading `env` with optional var assignments
	if strings.HasPrefix(trimmed, "env ") {
		rest := strings.TrimPrefix(trimmed, "env ")
		// Skip VAR=val pairs
		parts := strings.Fields(rest)
		for i, p := range parts {
			if !strings.Contains(p, "=") {
				return strings.Join(parts[i:], " ")
			}
		}
	}

	// Strip `cd <dir> &&` prefix
	if strings.HasPrefix(trimmed, "cd ") {
		if idx := strings.Index(trimmed, "&&"); idx > 0 {
			return strings.TrimSpace(trimmed[idx+2:])
		}
	}

	// Strip `time` prefix
	if strings.HasPrefix(trimmed, "time ") {
		return strings.TrimPrefix(trimmed, "time ")
	}

	return trimmed
}
