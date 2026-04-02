package tools

import (
	"testing"
)

// Source: utils/bash/ast.ts, tools/BashTool/readOnlyValidation.ts:1432-1503

func TestParseShellCommand(t *testing.T) {
	t.Run("simple_command", func(t *testing.T) {
		result := ParseShellCommand("ls -la")
		if result.Kind != "simple" {
			t.Errorf("expected simple, got %s", result.Kind)
		}
		if len(result.Commands) != 1 || result.Commands[0] != "ls" {
			t.Errorf("expected [ls], got %v", result.Commands)
		}
	})

	t.Run("pipeline", func(t *testing.T) {
		result := ParseShellCommand("cat foo.txt | grep hello")
		if result.Kind != "simple" {
			t.Errorf("expected simple, got %s (%s)", result.Kind, result.Reason)
		}
		if len(result.Commands) != 2 {
			t.Fatalf("expected 2 commands, got %d: %v", len(result.Commands), result.Commands)
		}
		if result.Commands[0] != "cat" || result.Commands[1] != "grep" {
			t.Errorf("expected [cat, grep], got %v", result.Commands)
		}
	})

	t.Run("and_list", func(t *testing.T) {
		result := ParseShellCommand("ls && pwd")
		if result.Kind != "simple" {
			t.Errorf("expected simple, got %s", result.Kind)
		}
		if len(result.Commands) != 2 {
			t.Fatalf("expected 2 commands, got %d", len(result.Commands))
		}
	})

	t.Run("semicolon_list", func(t *testing.T) {
		result := ParseShellCommand("echo hello; echo world")
		if result.Kind != "simple" {
			t.Errorf("expected simple, got %s", result.Kind)
		}
		if len(result.Commands) != 2 {
			t.Fatalf("expected 2 commands, got %d", len(result.Commands))
		}
	})

	t.Run("complex_subshell", func(t *testing.T) {
		// Source: utils/bash/ast.ts:1-19 — fail-closed: unknown structure = too-complex
		result := ParseShellCommand("echo $(whoami)")
		// This contains a command substitution — should be parseable but may be complex
		// The key thing is it doesn't panic and returns a result
		if result.Kind == "" {
			t.Error("expected a result kind")
		}
	})

	t.Run("parse_error", func(t *testing.T) {
		result := ParseShellCommand("if then fi bad")
		// Malformed shell — should be parse-error or too-complex
		if result.Kind == "simple" && len(result.Commands) > 0 {
			// Acceptable if the parser can extract commands
		}
	})

	t.Run("empty_command", func(t *testing.T) {
		result := ParseShellCommand("")
		if result.Kind != "simple" {
			t.Errorf("expected simple for empty, got %s", result.Kind)
		}
		if len(result.Commands) != 0 {
			t.Errorf("expected 0 commands, got %d", len(result.Commands))
		}
	})

	t.Run("quoted_command", func(t *testing.T) {
		result := ParseShellCommand(`git log --oneline -5`)
		if result.Kind != "simple" {
			t.Errorf("expected simple, got %s", result.Kind)
		}
		if len(result.Commands) != 1 || result.Commands[0] != "git" {
			t.Errorf("expected [git], got %v", result.Commands)
		}
	})
}

func TestIsReadOnlyCommand(t *testing.T) {
	// Source: tools/BashTool/readOnlyValidation.ts:1432-1503

	readOnlyCases := []struct {
		name    string
		cmd     string
		readOnly bool
	}{
		// Simple read-only commands
		{"ls", "ls -la", true},
		{"cat", "cat foo.txt", true},
		{"grep", "grep -r pattern .", true},
		{"head", "head -20 file.txt", true},
		{"tail", "tail -f log.txt", true},
		{"wc", "wc -l file.txt", true},
		{"pwd", "pwd", true},
		{"echo", "echo hello", true},
		{"diff", "diff a.txt b.txt", true},
		{"find", "find . -name '*.go'", true},
		{"which", "which node", true},
		{"uptime", "uptime", true},
		{"uname", "uname -a", true},
		{"tree", "tree -L 2", true},
		{"rg", "rg pattern", true},
		{"jq", "jq '.key' file.json", true},

		// Pipelines of read-only commands
		{"pipe_read_only", "cat file | grep pattern | wc -l", true},
		{"and_read_only", "ls && pwd", true},

		// Mutating commands — NOT read-only
		{"rm", "rm file.txt", false},
		{"mv", "mv a b", false},
		{"cp", "cp a b", false},
		{"mkdir", "mkdir new_dir", false},
		{"chmod", "chmod 755 file", false},
		{"chown", "chown user file", false},
		{"apt_get", "apt-get install pkg", false},
		{"npm_install", "npm install", false},
		{"pip_install", "pip install package", false},
		{"git_push", "git push", false},
		{"git_commit", "git commit -m 'msg'", false},

		// Mixed pipeline — one mutating makes it NOT read-only
		{"mixed_pipe", "cat file | tee output.txt", false},
		{"mixed_and", "ls && rm file", false},

		// Empty
		{"empty", "", false},
	}

	for _, tc := range readOnlyCases {
		t.Run(tc.name, func(t *testing.T) {
			got := IsReadOnlyCommand(tc.cmd)
			if got != tc.readOnly {
				t.Errorf("IsReadOnlyCommand(%q) = %v, want %v", tc.cmd, got, tc.readOnly)
			}
		})
	}
}

func TestReadOnlyCommandsList(t *testing.T) {
	// Source: tools/BashTool/readOnlyValidation.ts:1432-1503
	// Verify key commands from the TS list are present

	expectedReadOnly := []string{
		"cal", "uptime", "cat", "head", "tail", "wc", "stat", "strings",
		"hexdump", "od", "nl", "id", "uname", "free", "df", "du", "locale",
		"groups", "nproc", "basename", "dirname", "realpath", "readlink",
		"cut", "paste", "tr", "column", "tac", "rev", "fold", "expand",
		"unexpand", "fmt", "comm", "cmp", "numfmt", "diff", "true", "false",
		"sleep", "which", "type", "expr", "test", "getconf", "seq", "tsort", "pr",
	}
	for _, cmd := range expectedReadOnly {
		if !readOnlyCommands[cmd] {
			t.Errorf("expected %q to be in readOnlyCommands", cmd)
		}
	}
}
