package permissions

import (
	"testing"
)

// Source: utils/permissions/filesystem.ts

func TestDangerousFilesAndDirectories(t *testing.T) {
	// Source: filesystem.ts:57-79

	t.Run("dangerous_files_populated", func(t *testing.T) {
		if len(DangerousFiles) == 0 {
			t.Fatal("DangerousFiles should not be empty")
		}
		// Verify key entries
		found := map[string]bool{}
		for _, f := range DangerousFiles {
			found[f] = true
		}
		for _, expected := range []string{".gitconfig", ".bashrc", ".zshrc", ".profile", ".mcp.json"} {
			if !found[expected] {
				t.Errorf("DangerousFiles missing %q", expected)
			}
		}
	})

	t.Run("dangerous_directories_populated", func(t *testing.T) {
		if len(DangerousDirectories) == 0 {
			t.Fatal("DangerousDirectories should not be empty")
		}
		found := map[string]bool{}
		for _, d := range DangerousDirectories {
			found[d] = true
		}
		for _, expected := range []string{".git", ".vscode", ".idea", ".claude"} {
			if !found[expected] {
				t.Errorf("DangerousDirectories missing %q", expected)
			}
		}
	})
}

func TestIsDangerousFilePath(t *testing.T) {
	// Source: filesystem.ts:435-488

	t.Run("dangerous_directory_git", func(t *testing.T) {
		if !IsDangerousFilePath("/repo/.git/config") {
			t.Error(".git/config should be dangerous")
		}
	})

	t.Run("dangerous_directory_vscode", func(t *testing.T) {
		if !IsDangerousFilePath("/repo/.vscode/settings.json") {
			t.Error(".vscode/settings.json should be dangerous")
		}
	})

	t.Run("dangerous_directory_idea", func(t *testing.T) {
		if !IsDangerousFilePath("/repo/.idea/workspace.xml") {
			t.Error(".idea/workspace.xml should be dangerous")
		}
	})

	t.Run("dangerous_directory_claude", func(t *testing.T) {
		if !IsDangerousFilePath("/repo/.claude/settings.json") {
			t.Error(".claude/settings.json should be dangerous")
		}
	})

	t.Run("claude_worktrees_exception", func(t *testing.T) {
		// Source: filesystem.ts:458-467
		if IsDangerousFilePath("/repo/.claude/worktrees/branch/file.go") {
			t.Error(".claude/worktrees/ should be allowed (structural path)")
		}
	})

	t.Run("claude_worktrees_nested_claude_still_blocked", func(t *testing.T) {
		// A nested .claude inside a worktree IS still dangerous
		if !IsDangerousFilePath("/repo/.claude/worktrees/branch/.claude/settings.json") {
			t.Error("nested .claude in worktree should still be dangerous")
		}
	})

	t.Run("dangerous_file_bashrc", func(t *testing.T) {
		if !IsDangerousFilePath("/home/user/.bashrc") {
			t.Error(".bashrc should be dangerous")
		}
	})

	t.Run("dangerous_file_gitconfig", func(t *testing.T) {
		if !IsDangerousFilePath("/home/user/.gitconfig") {
			t.Error(".gitconfig should be dangerous")
		}
	})

	t.Run("dangerous_file_mcp_json", func(t *testing.T) {
		if !IsDangerousFilePath("/project/.mcp.json") {
			t.Error(".mcp.json should be dangerous")
		}
	})

	t.Run("safe_regular_file", func(t *testing.T) {
		if IsDangerousFilePath("/repo/src/main.go") {
			t.Error("src/main.go should be safe")
		}
	})

	t.Run("safe_readme", func(t *testing.T) {
		if IsDangerousFilePath("/repo/README.md") {
			t.Error("README.md should be safe")
		}
	})

	t.Run("case_insensitive_directory", func(t *testing.T) {
		// Source: filesystem.ts:449-450
		if !IsDangerousFilePath("/repo/.GIT/config") {
			t.Error("case-insensitive: .GIT should be dangerous")
		}
	})

	t.Run("case_insensitive_file", func(t *testing.T) {
		if !IsDangerousFilePath("/home/user/.BASHRC") {
			t.Error("case-insensitive: .BASHRC should be dangerous")
		}
	})

	t.Run("unc_path_backslash", func(t *testing.T) {
		if !IsDangerousFilePath(`\\server\share\file.txt`) {
			t.Error("UNC path (backslash) should be dangerous")
		}
	})

	t.Run("unc_path_forward_slash", func(t *testing.T) {
		if !IsDangerousFilePath("//server/share/file.txt") {
			t.Error("UNC path (forward slash) should be dangerous")
		}
	})
}

func TestIsPathInWorkingDir(t *testing.T) {
	// Source: filesystem.ts:709-744

	t.Run("same_path", func(t *testing.T) {
		if !IsPathInWorkingDir("/project", "/project") {
			t.Error("same path should be in working dir")
		}
	})

	t.Run("child_path", func(t *testing.T) {
		if !IsPathInWorkingDir("/project/src/main.go", "/project") {
			t.Error("child path should be in working dir")
		}
	})

	t.Run("deeply_nested_child", func(t *testing.T) {
		if !IsPathInWorkingDir("/project/a/b/c/d.go", "/project") {
			t.Error("deeply nested path should be in working dir")
		}
	})

	t.Run("outside_path", func(t *testing.T) {
		if IsPathInWorkingDir("/other/project/file.go", "/project") {
			t.Error("path outside working dir should not match")
		}
	})

	t.Run("parent_path", func(t *testing.T) {
		if IsPathInWorkingDir("/project", "/project/src") {
			t.Error("parent path should not be in child working dir")
		}
	})

	t.Run("path_traversal", func(t *testing.T) {
		if IsPathInWorkingDir("/project/../etc/passwd", "/project") {
			t.Error("path traversal should be rejected")
		}
	})

	t.Run("case_insensitive", func(t *testing.T) {
		// Source: filesystem.ts:725-726
		if !IsPathInWorkingDir("/Project/src/file.go", "/project") {
			t.Error("should be case-insensitive")
		}
	})

	t.Run("macos_private_tmp_normalization", func(t *testing.T) {
		// Source: filesystem.ts:716-721
		if !IsPathInWorkingDir("/private/tmp/project/file.go", "/tmp/project") {
			t.Error("/private/tmp should normalize to /tmp")
		}
	})

	t.Run("macos_private_var_normalization", func(t *testing.T) {
		if !IsPathInWorkingDir("/private/var/data/file.go", "/var/data") {
			t.Error("/private/var should normalize to /var")
		}
	})

	t.Run("sibling_directory", func(t *testing.T) {
		if IsPathInWorkingDir("/project-b/file.go", "/project") {
			t.Error("sibling directory with shared prefix should not match")
		}
	})
}

func TestIsPathInAnyWorkingDir(t *testing.T) {
	// Source: filesystem.ts:683-707

	t.Run("in_primary_cwd", func(t *testing.T) {
		if !IsPathInAnyWorkingDir("/project/file.go", "/project", nil) {
			t.Error("file in cwd should match")
		}
	})

	t.Run("in_additional_dir", func(t *testing.T) {
		additional := map[string]string{"/shared/lib": "projectSettings"}
		if !IsPathInAnyWorkingDir("/shared/lib/util.go", "/project", additional) {
			t.Error("file in additional dir should match")
		}
	})

	t.Run("not_in_any", func(t *testing.T) {
		additional := map[string]string{"/shared/lib": "projectSettings"}
		if IsPathInAnyWorkingDir("/etc/passwd", "/project", additional) {
			t.Error("file outside all dirs should not match")
		}
	})
}

func TestCheckPathSafetyForAutoEdit(t *testing.T) {
	// Source: filesystem.ts:620-665

	t.Run("safe_regular_file", func(t *testing.T) {
		result := CheckPathSafetyForAutoEdit("/project/src/main.go")
		if !result.Safe {
			t.Error("regular file should be safe")
		}
	})

	t.Run("unsafe_dangerous_file", func(t *testing.T) {
		result := CheckPathSafetyForAutoEdit("/home/user/.bashrc")
		if result.Safe {
			t.Error(".bashrc should be unsafe")
		}
		if result.Message == "" {
			t.Error("should have a message")
		}
		if !result.ClassifierApprovable {
			t.Error("dangerous files should be classifier-approvable")
		}
	})

	t.Run("unsafe_git_directory", func(t *testing.T) {
		result := CheckPathSafetyForAutoEdit("/repo/.git/hooks/pre-commit")
		if result.Safe {
			t.Error(".git/hooks should be unsafe")
		}
	})
}

func TestIsClaudeSettingsPath(t *testing.T) {
	// Source: filesystem.ts:200-222

	t.Run("claude_settings", func(t *testing.T) {
		if !IsClaudeSettingsPath("/project/.claude/settings.json") {
			t.Error("should detect .claude/settings.json")
		}
	})

	t.Run("claude_local_settings", func(t *testing.T) {
		if !IsClaudeSettingsPath("/project/.claude/settings.local.json") {
			t.Error("should detect .claude/settings.local.json")
		}
	})

	t.Run("regular_json_file", func(t *testing.T) {
		if IsClaudeSettingsPath("/project/package.json") {
			t.Error("package.json is not a settings path")
		}
	})
}

func TestNormalizeCaseForComparison(t *testing.T) {
	if NormalizeCaseForComparison("FooBar.TXT") != "foobar.txt" {
		t.Error("should lowercase")
	}
	if NormalizeCaseForComparison("already-lower") != "already-lower" {
		t.Error("already lowercase should pass through")
	}
}

func TestCheckReadPermissionForTool(t *testing.T) {
	cwd := "/project"
	noDirs := map[string]string{}
	noRules := []PermissionRule(nil)

	t.Run("in_working_dir_allowed", func(t *testing.T) {
		d := CheckReadPermissionForTool("Read", "/project/src/file.go", cwd, noDirs, noRules, noRules)
		if _, ok := d.(AllowDecision); !ok {
			t.Errorf("read in working dir should allow, got %T", d)
		}
	})

	t.Run("outside_working_dir_asks", func(t *testing.T) {
		d := CheckReadPermissionForTool("Read", "/etc/passwd", cwd, noDirs, noRules, noRules)
		if _, ok := d.(AskDecision); !ok {
			t.Errorf("read outside working dir should ask, got %T", d)
		}
	})

	t.Run("deny_rule_denies", func(t *testing.T) {
		deny := []PermissionRule{{
			Source:       "session",
			RuleBehavior: "deny",
			RuleValue:    PermissionRuleValue{ToolName: "Read", RuleContent: "/etc"},
		}}
		d := CheckReadPermissionForTool("Read", "/etc/passwd", cwd, noDirs, deny, noRules)
		if _, ok := d.(DenyDecision); !ok {
			t.Errorf("deny rule should deny, got %T", d)
		}
	})

	t.Run("allow_rule_overrides_outside_cwd", func(t *testing.T) {
		allow := []PermissionRule{{
			Source:       "session",
			RuleBehavior: "allow",
			RuleValue:    PermissionRuleValue{ToolName: "Read", RuleContent: "/opt/shared"},
		}}
		d := CheckReadPermissionForTool("Read", "/opt/shared/lib.go", cwd, noDirs, noRules, allow)
		if _, ok := d.(AllowDecision); !ok {
			t.Errorf("allow rule should allow outside cwd, got %T", d)
		}
	})
}

func TestCheckWritePermissionForTool(t *testing.T) {
	cwd := "/project"
	noDirs := map[string]string{}
	noRules := []PermissionRule(nil)

	t.Run("in_working_dir_safe_file_allowed", func(t *testing.T) {
		d := CheckWritePermissionForTool("Write", "/project/src/main.go", cwd, noDirs, noRules, noRules)
		if _, ok := d.(AllowDecision); !ok {
			t.Errorf("write in working dir should allow, got %T", d)
		}
	})

	t.Run("dangerous_file_asks", func(t *testing.T) {
		d := CheckWritePermissionForTool("Edit", "/project/.bashrc", cwd, noDirs, noRules, noRules)
		if _, ok := d.(AskDecision); !ok {
			t.Errorf("write to dangerous file should ask, got %T", d)
		}
	})

	t.Run("outside_working_dir_asks", func(t *testing.T) {
		d := CheckWritePermissionForTool("Write", "/tmp/file.txt", cwd, noDirs, noRules, noRules)
		if _, ok := d.(AskDecision); !ok {
			t.Errorf("write outside working dir should ask, got %T", d)
		}
	})

	t.Run("deny_rule_denies", func(t *testing.T) {
		deny := []PermissionRule{{
			Source:       "session",
			RuleBehavior: "deny",
			RuleValue:    PermissionRuleValue{ToolName: "Write"},
		}}
		d := CheckWritePermissionForTool("Write", "/project/file.go", cwd, noDirs, deny, noRules)
		if _, ok := d.(DenyDecision); !ok {
			t.Errorf("deny rule should deny, got %T", d)
		}
	})
}
