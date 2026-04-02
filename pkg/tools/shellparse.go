package tools

import (
	"strings"

	"mvdan.cc/sh/v3/syntax"
)

// Source: tools/BashTool/readOnlyValidation.ts:1432-1503
// Source: utils/shell/readOnlyCommandValidation.ts:1539-1543

// readOnlyCommands is the set of commands that are safe to auto-approve.
// Matches the TS READONLY_COMMANDS + EXTERNAL_READONLY_COMMANDS lists.
var readOnlyCommands = map[string]bool{
	// Cross-platform (from EXTERNAL_READONLY_COMMANDS)
	"docker ps":     true,
	"docker images": true,

	// Time and date
	"cal":    true,
	"uptime": true,

	// File content viewing
	"cat":     true,
	"head":    true,
	"tail":    true,
	"wc":      true,
	"stat":    true,
	"strings": true,
	"hexdump": true,
	"od":      true,
	"nl":      true,

	// System info
	"id":     true,
	"uname":  true,
	"free":   true,
	"df":     true,
	"du":     true,
	"locale": true,
	"groups": true,
	"nproc":  true,

	// Path information
	"basename": true,
	"dirname":  true,
	"realpath": true,
	"readlink": true,

	// Text processing
	"cut":      true,
	"paste":    true,
	"tr":       true,
	"column":   true,
	"tac":      true,
	"rev":      true,
	"fold":     true,
	"expand":   true,
	"unexpand": true,
	"fmt":      true,
	"comm":     true,
	"cmp":      true,
	"numfmt":   true,

	// File comparison
	"diff": true,

	// true/false
	"true":  true,
	"false": true,

	// Misc safe commands
	"sleep":   true,
	"which":   true,
	"type":    true,
	"expr":    true,
	"test":    true,
	"getconf": true,
	"seq":     true,
	"tsort":   true,
	"pr":      true,

	// Common read-only commands (ls, find, grep, etc.)
	"ls":    true,
	"pwd":   true,
	"echo":  true,
	"printf": true,
	"env":   true,
	"whoami": true,
	"date":  true, // without -s/--set
	"file":  true,
	"find":  true,
	"grep":  true,
	"egrep": true,
	"fgrep": true,
	"rg":    true,
	"ag":    true,
	"fd":    true,
	"fdfind": true,
	"tree":  true,
	"less":  true,
	"more":  true,
	"man":   true,
	"help":  true,
	"jq":    true,
	"yq":    true,
	"xargs": true,
}

// gitReadOnlySubcommands are git subcommands that are read-only.
// Source: utils/shell/readOnlyCommandValidation.ts:107-983
var gitReadOnlySubcommands = map[string]bool{
	"status":     true,
	"log":        true,
	"diff":       true,
	"show":       true,
	"branch":     true, // without -d/-D
	"remote":     true, // without add/remove/set-url
	"tag":        true, // without -d
	"describe":   true,
	"rev-parse":  true,
	"rev-list":   true,
	"ls-files":   true,
	"ls-tree":    true,
	"ls-remote":  true,
	"cat-file":   true,
	"shortlog":   true,
	"blame":      true,
	"name-rev":   true,
	"for-each-ref": true,
	"show-ref":   true,
	"reflog":     true,
	"count-objects": true,
	"fsck":       true,
	"verify-pack": true,
	"stash list": true,
}

// ParseResult describes the result of parsing a shell command for security analysis.
// Source: utils/bash/ast.ts:42-45
type ParseResult struct {
	Kind     string   // "simple", "too-complex", "parse-error"
	Commands []string // extracted command names (argv[0])
	Reason   string   // reason for too-complex
}

// ParseShellCommand parses a shell command string using mvdan.cc/sh and extracts
// the command names (argv[0]) from each simple command.
// Source: utils/bash/ast.ts:1-19 — fail-closed design
func ParseShellCommand(command string) ParseResult {
	reader := strings.NewReader(command)
	parser := syntax.NewParser(syntax.KeepComments(false))

	file, err := parser.Parse(reader, "")
	if err != nil {
		return ParseResult{Kind: "parse-error", Reason: err.Error()}
	}

	var commands []string
	tooComplex := false
	var complexReason string

	syntax.Walk(file, func(node syntax.Node) bool {
		if tooComplex {
			return false
		}
		switch n := node.(type) {
		case *syntax.CallExpr:
			if len(n.Args) > 0 {
				// Extract the command name from the first word
				cmdName := wordToString(n.Args[0])
				if cmdName != "" {
					commands = append(commands, cmdName)
				}
			}
		case *syntax.BinaryCmd:
			// Pipes, &&, ||, etc. — walk children
			return true
		case *syntax.Subshell:
			// $() or backticks — too complex for simple analysis
			tooComplex = true
			complexReason = "subshell"
			return false
		case *syntax.FuncDecl:
			tooComplex = true
			complexReason = "function declaration"
			return false
		case *syntax.IfClause, *syntax.WhileClause, *syntax.ForClause, *syntax.CaseClause:
			tooComplex = true
			complexReason = "control flow"
			return false
		}
		return true
	})

	if tooComplex {
		return ParseResult{Kind: "too-complex", Reason: complexReason}
	}

	return ParseResult{Kind: "simple", Commands: commands}
}

// wordToString extracts a simple string from a syntax.Word.
// Returns empty string for complex words (expansions, etc.).
func wordToString(word *syntax.Word) string {
	if len(word.Parts) == 0 {
		return ""
	}
	var sb strings.Builder
	for _, part := range word.Parts {
		switch p := part.(type) {
		case *syntax.Lit:
			sb.WriteString(p.Value)
		case *syntax.SglQuoted:
			sb.WriteString(p.Value)
		case *syntax.DblQuoted:
			// Simple double-quoted strings with only literals
			for _, qp := range p.Parts {
				if lit, ok := qp.(*syntax.Lit); ok {
					sb.WriteString(lit.Value)
				} else {
					return "" // Contains expansion
				}
			}
		default:
			return "" // Complex expression
		}
	}
	return sb.String()
}

// IsReadOnlyCommand checks if a shell command string contains only read-only commands.
// Returns true if ALL commands in the pipeline/list are read-only.
// Source: tools/BashTool/readOnlyValidation.ts:1432-1503
func IsReadOnlyCommand(command string) bool {
	result := ParseShellCommand(command)

	if result.Kind != "simple" {
		return false // Can't determine safety
	}

	if len(result.Commands) == 0 {
		return false // Empty command
	}

	for _, cmd := range result.Commands {
		if !isCommandReadOnly(cmd) {
			return false
		}
	}
	return true
}

// isCommandReadOnly checks if a single command name is in the read-only set.
func isCommandReadOnly(cmd string) bool {
	// Direct match
	if readOnlyCommands[cmd] {
		return true
	}

	// Git subcommand check
	parts := strings.Fields(cmd)
	if len(parts) >= 2 && parts[0] == "git" {
		if gitReadOnlySubcommands[parts[1]] {
			return true
		}
	}

	return false
}
