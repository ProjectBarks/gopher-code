package permissions

import "testing"

// Source: utils/permissions/dangerousPatterns.ts, utils/permissions/permissionSetup.ts

func TestIsDangerousBashPermission(t *testing.T) {
	// Source: utils/permissions/permissionSetup.ts:94-147

	t.Run("non_bash_tool_safe", func(t *testing.T) {
		// Source: permissionSetup.ts:99-101
		if IsDangerousBashPermission("Read", "") {
			t.Error("non-Bash tools should never be dangerous")
		}
	})

	t.Run("tool_level_allow_dangerous", func(t *testing.T) {
		// Source: permissionSetup.ts:104-106
		if !IsDangerousBashPermission("Bash", "") {
			t.Error("Bash with no content should be dangerous")
		}
	})

	t.Run("wildcard_dangerous", func(t *testing.T) {
		// Source: permissionSetup.ts:111-113
		if !IsDangerousBashPermission("Bash", "*") {
			t.Error("Bash(*) should be dangerous")
		}
	})

	t.Run("interpreter_exact_match", func(t *testing.T) {
		// Source: dangerousPatterns.ts:20-21
		for _, cmd := range []string{"python", "python3", "node", "ruby", "perl"} {
			if !IsDangerousBashPermission("Bash", cmd) {
				t.Errorf("Bash(%s) should be dangerous", cmd)
			}
		}
	})

	t.Run("interpreter_prefix_syntax", func(t *testing.T) {
		// Source: permissionSetup.ts:126-128
		if !IsDangerousBashPermission("Bash", "python:*") {
			t.Error("Bash(python:*) should be dangerous")
		}
	})

	t.Run("interpreter_wildcard_suffix", func(t *testing.T) {
		// Source: permissionSetup.ts:130-133
		if !IsDangerousBashPermission("Bash", "python*") {
			t.Error("Bash(python*) should be dangerous")
		}
	})

	t.Run("interpreter_space_wildcard", func(t *testing.T) {
		// Source: permissionSetup.ts:135-138
		if !IsDangerousBashPermission("Bash", "python *") {
			t.Error("Bash(python *) should be dangerous")
		}
	})

	t.Run("interpreter_dash_wildcard", func(t *testing.T) {
		// Source: permissionSetup.ts:141-143
		if !IsDangerousBashPermission("Bash", "python -c *") {
			t.Error("Bash(python -c *) should be dangerous")
		}
	})

	t.Run("safe_commands_not_dangerous", func(t *testing.T) {
		safeCmds := []string{"npm install", "git status", "ls -la", "cat file.txt", "go test"}
		for _, cmd := range safeCmds {
			if IsDangerousBashPermission("Bash", cmd) {
				t.Errorf("Bash(%s) should be safe", cmd)
			}
		}
	})

	t.Run("case_insensitive", func(t *testing.T) {
		if !IsDangerousBashPermission("Bash", "Python") {
			t.Error("should be case insensitive")
		}
		if !IsDangerousBashPermission("Bash", "NODE:*") {
			t.Error("should be case insensitive")
		}
	})

	t.Run("dangerous_builtins", func(t *testing.T) {
		// Source: dangerousPatterns.ts:46-50
		for _, cmd := range []string{"eval", "exec", "env", "xargs", "sudo"} {
			if !IsDangerousBashPermission("Bash", cmd) {
				t.Errorf("Bash(%s) should be dangerous", cmd)
			}
		}
	})

	t.Run("shell_commands_dangerous", func(t *testing.T) {
		// Source: dangerousPatterns.ts:38-39, 46-47
		for _, cmd := range []string{"bash", "sh", "zsh", "fish"} {
			if !IsDangerousBashPermission("Bash", cmd) {
				t.Errorf("Bash(%s) should be dangerous", cmd)
			}
		}
	})

	t.Run("package_runners_dangerous", func(t *testing.T) {
		// Source: dangerousPatterns.ts:32-36
		for _, cmd := range []string{"npx", "bunx", "npm run", "yarn run", "pnpm run", "bun run"} {
			if !IsDangerousBashPermission("Bash", cmd) {
				t.Errorf("Bash(%s) should be dangerous", cmd)
			}
		}
	})
}

func TestStripDangerousPermissions(t *testing.T) {
	// Source: utils/permissions/permissionSetup.ts:510-553

	rules := []PermissionRuleValue{
		{ToolName: "Bash", RuleContent: "npm install"},  // safe
		{ToolName: "Bash", RuleContent: "python:*"},     // dangerous
		{ToolName: "Read"},                               // safe (not Bash)
		{ToolName: "Bash"},                               // dangerous (tool-level)
		{ToolName: "Bash", RuleContent: "git status"},   // safe
	}

	safe, stripped := StripDangerousPermissions(rules)

	if len(safe) != 3 {
		t.Fatalf("expected 3 safe rules, got %d", len(safe))
	}
	if len(stripped) != 2 {
		t.Fatalf("expected 2 stripped rules, got %d", len(stripped))
	}

	// Verify safe rules
	if safe[0].RuleContent != "npm install" || safe[1].ToolName != "Read" || safe[2].RuleContent != "git status" {
		t.Errorf("unexpected safe rules: %v", safe)
	}

	// Verify stripped rules
	if stripped[0].RuleContent != "python:*" || stripped[1].ToolName != "Bash" {
		t.Errorf("unexpected stripped rules: %v", stripped)
	}
}

func TestRestoreDangerousPermissions(t *testing.T) {
	// Source: utils/permissions/permissionSetup.ts:561-578

	current := []PermissionRuleValue{{ToolName: "Bash", RuleContent: "npm install"}}
	stashed := []PermissionRuleValue{{ToolName: "Bash", RuleContent: "python:*"}}

	restored := RestoreDangerousPermissions(current, stashed)
	if len(restored) != 2 {
		t.Fatalf("expected 2 rules after restore, got %d", len(restored))
	}

	// Empty stash is a no-op
	result := RestoreDangerousPermissions(current, nil)
	if len(result) != 1 {
		t.Errorf("nil stash should return current unchanged, got %d", len(result))
	}
}
