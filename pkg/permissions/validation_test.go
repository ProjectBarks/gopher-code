package permissions

import "testing"

// Source: utils/settings/permissionValidation.ts, utils/settings/validation.ts

func TestValidatePermissionRule(t *testing.T) {
	// Source: utils/settings/permissionValidation.ts:58-98

	t.Run("valid_tool_name", func(t *testing.T) {
		r := ValidatePermissionRule("Bash")
		if !r.Valid {
			t.Errorf("expected valid, got error: %s", r.Error)
		}
	})

	t.Run("valid_tool_with_content", func(t *testing.T) {
		r := ValidatePermissionRule("Bash(npm install)")
		if !r.Valid {
			t.Errorf("expected valid, got error: %s", r.Error)
		}
	})

	t.Run("valid_glob_pattern", func(t *testing.T) {
		r := ValidatePermissionRule("mcp__*")
		if !r.Valid {
			t.Errorf("expected valid, got error: %s", r.Error)
		}
	})

	t.Run("empty_rule_invalid", func(t *testing.T) {
		// Source: permissionValidation.ts:65-67
		r := ValidatePermissionRule("")
		if r.Valid {
			t.Error("empty rule should be invalid")
		}
		if r.Error != "Permission rule cannot be empty" {
			t.Errorf("unexpected error: %s", r.Error)
		}
	})

	t.Run("whitespace_only_invalid", func(t *testing.T) {
		r := ValidatePermissionRule("   ")
		if r.Valid {
			t.Error("whitespace-only rule should be invalid")
		}
	})

	t.Run("mismatched_parens_invalid", func(t *testing.T) {
		// Source: permissionValidation.ts:70-79
		r := ValidatePermissionRule("Bash(npm install")
		if r.Valid {
			t.Error("mismatched parens should be invalid")
		}
		if r.Error != "Mismatched parentheses" {
			t.Errorf("unexpected error: %s", r.Error)
		}
	})

	t.Run("empty_parens_invalid", func(t *testing.T) {
		// Source: permissionValidation.ts:82-97
		r := ValidatePermissionRule("Bash()")
		if r.Valid {
			t.Error("empty parens should be invalid")
		}
		if r.Error != "Empty parentheses" {
			t.Errorf("unexpected error: %s", r.Error)
		}
	})

	t.Run("escaped_parens_valid", func(t *testing.T) {
		r := ValidatePermissionRule(`Bash(print\(1\))`)
		if !r.Valid {
			t.Errorf("escaped parens should be valid, got error: %s", r.Error)
		}
	})
}

func TestFilterInvalidPermissionRules(t *testing.T) {
	// Source: utils/settings/validation.ts:224-265

	t.Run("filters_invalid_keeps_valid", func(t *testing.T) {
		rules := []string{
			"Bash(npm install)", // valid
			"",                  // invalid — empty
			"Read",              // valid
			"Bash(",             // invalid — mismatched parens
			"Edit(*.go)",        // valid
		}

		valid, warnings := FilterInvalidPermissionRules(rules, "allow", "settings.json")

		if len(valid) != 3 {
			t.Fatalf("expected 3 valid rules, got %d: %v", len(valid), valid)
		}
		if valid[0] != "Bash(npm install)" || valid[1] != "Read" || valid[2] != "Edit(*.go)" {
			t.Errorf("unexpected valid rules: %v", valid)
		}

		if len(warnings) != 2 {
			t.Fatalf("expected 2 warnings, got %d", len(warnings))
		}
		// Source: validation.ts:250-258
		for _, w := range warnings {
			if w.File != "settings.json" {
				t.Errorf("warning file = %q, want 'settings.json'", w.File)
			}
			if w.Path != "permissions.allow" {
				t.Errorf("warning path = %q, want 'permissions.allow'", w.Path)
			}
		}
	})

	t.Run("all_valid", func(t *testing.T) {
		rules := []string{"Bash", "Read", "Edit"}
		valid, warnings := FilterInvalidPermissionRules(rules, "deny", "test.json")
		if len(valid) != 3 {
			t.Errorf("expected 3 valid, got %d", len(valid))
		}
		if len(warnings) != 0 {
			t.Errorf("expected 0 warnings, got %d", len(warnings))
		}
	})

	t.Run("all_invalid", func(t *testing.T) {
		rules := []string{"", "  ", "Bash("}
		valid, warnings := FilterInvalidPermissionRules(rules, "ask", "bad.json")
		if len(valid) != 0 {
			t.Errorf("expected 0 valid, got %d", len(valid))
		}
		if len(warnings) != 3 {
			t.Errorf("expected 3 warnings, got %d", len(warnings))
		}
	})

	t.Run("nil_input", func(t *testing.T) {
		valid, warnings := FilterInvalidPermissionRules(nil, "allow", "")
		if len(valid) != 0 || len(warnings) != 0 {
			t.Error("nil input should return empty results")
		}
	})
}
