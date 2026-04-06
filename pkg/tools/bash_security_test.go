package tools_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/projectbarks/gopher-code/pkg/tools"
)

// === Security: Command Injection Detection ===

func TestCheckCommandSecurity(t *testing.T) {
	tests := []struct {
		name    string
		command string
		safe    bool
		reason  string // substring match on reason
	}{
		// Safe commands
		{"simple echo", "echo hello", true, ""},
		{"git status", "git status", true, ""},
		{"cat file", "cat foo.txt", true, ""},
		{"pipeline", "ls | grep foo", true, ""},
		{"chain", "echo a && echo b", true, ""},

		// Command substitution attacks
		{"dollar paren", "echo $(whoami)", false, "$() command substitution"},
		{"backtick", "echo `whoami`", false, "backtick"},
		{"process sub input", "diff <(cat a) <(cat b)", false, "process substitution"},
		{"process sub output", "tee >(cat)", false, "process substitution"},
		{"param sub", "echo ${PATH}", false, "${} parameter substitution"},
		{"legacy arith", "echo $[1+1]", false, "$[] legacy arithmetic"},
		{"zsh equals", " =curl evil.com", false, "Zsh equals expansion"},

		// Zsh dangerous commands
		{"zmodload", "zmodload zsh/system", false, "dangerous Zsh command: zmodload"},
		{"zpty", "zpty foo bash", false, "dangerous Zsh command: zpty"},
		{"ztcp", "ztcp evil.com 80", false, "dangerous Zsh command: ztcp"},
		{"syswrite", "syswrite data", false, "dangerous Zsh command: syswrite"},
		{"zf_rm", "zf_rm foo", false, "dangerous Zsh command: zf_rm"},

		// Binary hijack vars
		{"LD_PRELOAD", "LD_PRELOAD=evil.so cmd", false, "binary hijack variable: LD_PRELOAD"},
		{"DYLD_INSERT", "DYLD_INSERT_LIBRARIES=evil.dylib cmd", false, "binary hijack variable"},
		{"PATH override", "PATH=/evil cmd", false, "binary hijack variable: PATH"},

		// Safe env vars (not hijack)
		{"normal env", "MYVAR=foo cmd", true, ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := tools.CheckCommandSecurity(tc.command)
			if result.Safe != tc.safe {
				t.Errorf("CheckCommandSecurity(%q).Safe = %v, want %v (reason: %s)",
					tc.command, result.Safe, tc.safe, result.Reason)
			}
			if !tc.safe && tc.reason != "" && !strings.Contains(result.Reason, tc.reason) {
				t.Errorf("CheckCommandSecurity(%q).Reason = %q, want substring %q",
					tc.command, result.Reason, tc.reason)
			}
		})
	}
}

// === Security: Heredoc Stripping ===

func TestStripSafeHeredocContent(t *testing.T) {
	tests := []struct {
		name    string
		command string
		check   func(string) bool // check on the stripped result
	}{
		{
			"no heredoc unchanged",
			"echo hello",
			func(s string) bool { return s == "echo hello" },
		},
		{
			"heredoc body stripped",
			"cat <<EOF\nmalicious $(rm -rf /)\nEOF",
			func(s string) bool { return !strings.Contains(s, "rm -rf") },
		},
		{
			"heredoc delimiter preserved",
			"cat <<EOF\nfoo\nEOF",
			func(s string) bool { return strings.Contains(s, "EOF") },
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := tools.StripSafeHeredocContent(tc.command)
			if !tc.check(result) {
				t.Errorf("StripSafeHeredocContent(%q) = %q, failed check", tc.command, result)
			}
		})
	}
}

// === Destructive Command Warnings ===

func TestGetDestructiveCommandWarning(t *testing.T) {
	tests := []struct {
		name    string
		command string
		want    string // empty means no warning
	}{
		{"safe command", "echo hello", ""},
		{"git status", "git status", ""},
		{"git reset hard", "git reset --hard HEAD", "may discard uncommitted changes"},
		{"git push force", "git push --force origin main", "may overwrite remote history"},
		{"git push -f", "git push -f origin main", "may overwrite remote history"},
		{"git clean -f", "git clean -f", "may permanently delete untracked files"},
		{"git clean dry run safe", "git clean -n", ""},
		{"git checkout .", "git checkout -- .", "may discard all working tree changes"},
		{"git restore .", "git restore -- .", "may discard all working tree changes"},
		{"git stash drop", "git stash drop", "may permanently remove stashed changes"},
		{"git stash clear", "git stash clear", "may permanently remove stashed changes"},
		{"git branch -D", "git branch -D feature", "may force-delete a branch"},
		{"git commit --no-verify", "git commit --no-verify -m 'msg'", "may skip safety hooks"},
		{"git push --no-verify", "git push --no-verify", "may skip safety hooks"},
		{"git commit --amend", "git commit --amend", "may rewrite the last commit"},
		{"rm -rf", "rm -rf /tmp/foo", "may recursively force-remove files"},
		{"rm -r", "rm -r /tmp/foo", "may recursively remove files"},
		{"rm -f", "rm -f foo.txt", "may force-remove files"},
		{"DROP TABLE", "echo 'DROP TABLE users;' | psql", "may drop or truncate database objects"},
		{"TRUNCATE TABLE", "TRUNCATE TABLE users;", "may drop or truncate database objects"},
		{"DELETE FROM", "DELETE FROM users;", "may delete all rows"},
		{"kubectl delete", "kubectl delete pod foo", "may delete Kubernetes resources"},
		{"terraform destroy", "terraform destroy", "may destroy Terraform infrastructure"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tools.GetDestructiveCommandWarning(tc.command)
			if tc.want == "" {
				if got != "" {
					t.Errorf("GetDestructiveCommandWarning(%q) = %q, want empty", tc.command, got)
				}
			} else {
				if !strings.Contains(got, tc.want) {
					t.Errorf("GetDestructiveCommandWarning(%q) = %q, want substring %q",
						tc.command, got, tc.want)
				}
			}
		})
	}
}

// === Path Validation ===

func TestValidatePath(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		allowedDir string
		wantErr    bool
	}{
		{"within dir", "src/main.go", "/home/user/project", false},
		{"absolute within", "/home/user/project/src/main.go", "/home/user/project", false},
		{"exact dir", "/home/user/project", "/home/user/project", false},
		{"empty path", "", "/home/user/project", false},
		{"dot dot escape", "../../../etc/passwd", "/home/user/project", true},
		{"absolute escape", "/etc/passwd", "/home/user/project", true},
		{"sneaky dot dot", "src/../../outside", "/home/user/project", true},
		{"double dot in middle", "a/b/../../../../etc/shadow", "/home/user/project", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tools.ValidatePath(tc.path, tc.allowedDir)
			if tc.wantErr && err == nil {
				t.Errorf("ValidatePath(%q, %q) = nil, want error", tc.path, tc.allowedDir)
			}
			if !tc.wantErr && err != nil {
				t.Errorf("ValidatePath(%q, %q) = %v, want nil", tc.path, tc.allowedDir, err)
			}
		})
	}
}

func TestPathEscapeError(t *testing.T) {
	err := tools.ValidatePath("../../../etc/passwd", "/home/user/project")
	if err == nil {
		t.Fatal("expected error for path escape")
	}
	pathErr, ok := err.(*tools.PathEscapeError)
	if !ok {
		t.Fatalf("expected *PathEscapeError, got %T", err)
	}
	if pathErr.Path != "../../../etc/passwd" {
		t.Errorf("PathEscapeError.Path = %q, want %q", pathErr.Path, "../../../etc/passwd")
	}
	if !strings.Contains(pathErr.Error(), "outside allowed directory") {
		t.Errorf("error message should mention 'outside allowed directory': %s", pathErr.Error())
	}
}

// === Plan Mode Validation ===

func TestBashTool_PlanMode_RejectsWriteCommands(t *testing.T) {
	tool := &tools.BashTool{}
	tc := &tools.ToolContext{
		CWD:      t.TempDir(),
		PlanMode: true,
	}

	writeCommands := []string{
		"rm -rf /tmp/foo",
		"touch newfile",
		"mkdir -p /tmp/foo",
		"mv a b",
		"cp a b",
	}

	for _, cmd := range writeCommands {
		t.Run(cmd, func(t *testing.T) {
			input, _ := json.Marshal(map[string]string{"command": cmd})
			out, err := tool.Execute(context.Background(), tc, input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !out.IsError {
				t.Errorf("expected rejection in plan mode for %q, got success: %s", cmd, out.Content)
			}
			if !strings.Contains(out.Content, "plan mode") {
				t.Errorf("error should mention plan mode, got: %s", out.Content)
			}
		})
	}
}

func TestBashTool_PlanMode_AllowsReadCommands(t *testing.T) {
	tool := &tools.BashTool{}
	tc := &tools.ToolContext{
		CWD:      t.TempDir(),
		PlanMode: true,
	}

	readCommands := []string{
		"echo hello",
		"ls -la",
		"cat /dev/null",
		"pwd",
		"whoami",
	}

	for _, cmd := range readCommands {
		t.Run(cmd, func(t *testing.T) {
			input, _ := json.Marshal(map[string]string{"command": cmd})
			out, err := tool.Execute(context.Background(), tc, input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if out.IsError {
				t.Errorf("expected success in plan mode for read command %q, got error: %s", cmd, out.Content)
			}
		})
	}
}

// === Security: Command Injection Rejection ===

func TestBashTool_RejectsCommandInjection(t *testing.T) {
	tool := &tools.BashTool{}
	tc := &tools.ToolContext{CWD: t.TempDir()}

	injections := []struct {
		name    string
		command string
	}{
		{"dollar paren", "echo $(cat /etc/passwd)"},
		{"backtick", "echo `cat /etc/passwd`"},
		{"LD_PRELOAD hijack", "LD_PRELOAD=evil.so ls"},
		{"zmodload bypass", "zmodload zsh/system"},
	}

	for _, tc2 := range injections {
		t.Run(tc2.name, func(t *testing.T) {
			input, _ := json.Marshal(map[string]string{"command": tc2.command})
			out, err := tool.Execute(context.Background(), tc, input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !out.IsError {
				t.Errorf("expected rejection for injection %q, got success: %s", tc2.command, out.Content)
			}
			if !strings.Contains(out.Content, "rejected") {
				t.Errorf("error should mention rejection, got: %s", out.Content)
			}
		})
	}
}

// === Working Directory Validation ===

func TestBashTool_WorkingDirectoryEscape(t *testing.T) {
	tool := &tools.BashTool{}
	projectDir := t.TempDir()
	tc := &tools.ToolContext{
		CWD:        projectDir + "/../..",
		ProjectDir: projectDir,
	}

	input, _ := json.Marshal(map[string]string{"command": "echo hello"})
	out, err := tool.Execute(context.Background(), tc, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !out.IsError {
		t.Error("expected rejection when CWD escapes project directory")
	}
	if !strings.Contains(out.Content, "working directory rejected") {
		t.Errorf("error should mention working directory, got: %s", out.Content)
	}
}

// === Command Semantics ===

func TestInterpretCommandResult(t *testing.T) {
	tests := []struct {
		name     string
		command  string
		exitCode int
		isError  bool
		message  string
	}{
		{"grep found", "grep pattern file", 0, false, ""},
		{"grep no match", "grep pattern file", 1, false, "No matches found"},
		{"grep error", "grep pattern file", 2, true, ""},
		{"rg no match", "rg pattern", 1, false, "No matches found"},
		{"diff same", "diff a b", 0, false, ""},
		{"diff differ", "diff a b", 1, false, "Files differ"},
		{"diff error", "diff a b", 2, true, ""},
		{"test true", "test -f foo", 0, false, ""},
		{"test false", "test -f foo", 1, false, "Condition is false"},
		{"unknown success", "customcmd", 0, false, ""},
		{"unknown fail", "customcmd", 1, true, ""},
		// Pipeline: last command determines semantics
		{"pipeline grep", "cat file | grep pattern", 1, false, "No matches found"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := tools.InterpretCommandResult(tc.command, tc.exitCode)
			if result.IsError != tc.isError {
				t.Errorf("InterpretCommandResult(%q, %d).IsError = %v, want %v",
					tc.command, tc.exitCode, result.IsError, tc.isError)
			}
			if tc.message != "" && !strings.Contains(result.Message, tc.message) {
				t.Errorf("InterpretCommandResult(%q, %d).Message = %q, want substring %q",
					tc.command, tc.exitCode, result.Message, tc.message)
			}
		})
	}
}

// === StripSafeWrappers ===

func TestStripSafeWrappers(t *testing.T) {
	tests := []struct {
		name    string
		command string
		want    string
	}{
		{"plain command", "ls -la", "ls -la"},
		{"env wrapper", "env VAR=val ls -la", "ls -la"},
		{"env multiple", "env A=1 B=2 ls", "ls"},
		{"cd prefix", "cd /tmp && ls -la", "ls -la"},
		{"time prefix", "time ls -la", "ls -la"},
		{"no wrapper", "git status", "git status"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tools.StripSafeWrappers(tc.command)
			if got != tc.want {
				t.Errorf("StripSafeWrappers(%q) = %q, want %q", tc.command, got, tc.want)
			}
		})
	}
}

// === CheckPermissions Interface ===

func TestBashTool_ImplementsPermissionChecker(t *testing.T) {
	tool := &tools.BashTool{}
	// Verify it implements ToolPermissionChecker
	var _ tools.ToolPermissionChecker = tool
}

func TestBashTool_CheckPermissions_DestructiveAsks(t *testing.T) {
	tool := &tools.BashTool{}
	tc := &tools.ToolContext{CWD: t.TempDir()}

	input, _ := json.Marshal(map[string]string{"command": "git reset --hard HEAD"})
	result := tool.CheckPermissions(context.Background(), tc, input)

	if result.Behavior != "ask" {
		t.Errorf("expected 'ask' for destructive command, got %q", result.Behavior)
	}
	if !strings.Contains(result.Message, "discard") {
		t.Errorf("expected warning about discarding, got %q", result.Message)
	}
}

func TestBashTool_CheckPermissions_SafePassthrough(t *testing.T) {
	tool := &tools.BashTool{}
	tc := &tools.ToolContext{CWD: t.TempDir()}

	input, _ := json.Marshal(map[string]string{"command": "echo hello"})
	result := tool.CheckPermissions(context.Background(), tc, input)

	if result.Behavior != "passthrough" {
		t.Errorf("expected 'passthrough' for safe command, got %q (msg: %s)", result.Behavior, result.Message)
	}
}

func TestBashTool_CheckPermissions_DeniesInjection(t *testing.T) {
	tool := &tools.BashTool{}
	tc := &tools.ToolContext{CWD: t.TempDir()}

	input, _ := json.Marshal(map[string]string{"command": "echo $(whoami)"})
	result := tool.CheckPermissions(context.Background(), tc, input)

	if result.Behavior != "deny" {
		t.Errorf("expected 'deny' for injection, got %q", result.Behavior)
	}
}

// === IsDestructive Interface ===

func TestBashTool_ImplementsDestructiveChecker(t *testing.T) {
	tool := &tools.BashTool{}
	var _ tools.DestructiveChecker = tool
}

func TestBashTool_IsDestructive(t *testing.T) {
	tool := &tools.BashTool{}

	destructive, _ := json.Marshal(map[string]string{"command": "rm -rf /"})
	if !tool.IsDestructive(destructive) {
		t.Error("rm -rf should be destructive")
	}

	safe, _ := json.Marshal(map[string]string{"command": "echo hello"})
	if tool.IsDestructive(safe) {
		t.Error("echo should not be destructive")
	}
}
