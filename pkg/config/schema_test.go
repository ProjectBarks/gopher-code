package config

import "testing"

// Source: utils/settings/schemaOutput.ts, utils/settings/types.ts

func TestValidateSettingsJSON(t *testing.T) {

	t.Run("valid_settings", func(t *testing.T) {
		data := []byte(`{
			"model": "claude-sonnet-4-20250514",
			"max_turns": 100,
			"permission_mode": "default",
			"verbose": true
		}`)
		errs := ValidateSettingsJSON(data)
		if len(errs) > 0 {
			t.Errorf("expected no errors, got %v", errs)
		}
	})

	t.Run("invalid_json", func(t *testing.T) {
		data := []byte(`{invalid}`)
		errs := ValidateSettingsJSON(data)
		if len(errs) == 0 {
			t.Error("expected error for invalid JSON")
		}
	})

	t.Run("wrong_type_model", func(t *testing.T) {
		data := []byte(`{"model": 123}`)
		errs := ValidateSettingsJSON(data)
		if len(errs) != 1 {
			t.Fatalf("expected 1 error, got %d", len(errs))
		}
		if errs[0].Path != "model" {
			t.Errorf("path = %q, want 'model'", errs[0].Path)
		}
	})

	t.Run("invalid_permission_mode", func(t *testing.T) {
		// Source: types/permissions.ts:16-22 — valid modes
		data := []byte(`{"permission_mode": "invalid_mode"}`)
		errs := ValidateSettingsJSON(data)
		if len(errs) != 1 {
			t.Fatalf("expected 1 error, got %d", len(errs))
		}
		if errs[0].Path != "permission_mode" {
			t.Errorf("path = %q", errs[0].Path)
		}
	})

	t.Run("valid_permission_modes", func(t *testing.T) {
		for _, mode := range []string{"default", "acceptEdits", "bypassPermissions", "dontAsk", "plan", "auto"} {
			data := []byte(`{"permission_mode": "` + mode + `"}`)
			errs := ValidateSettingsJSON(data)
			if len(errs) > 0 {
				t.Errorf("mode %q should be valid, got errors: %v", mode, errs)
			}
		}
	})

	t.Run("max_turns_too_low", func(t *testing.T) {
		data := []byte(`{"max_turns": 0}`)
		errs := ValidateSettingsJSON(data)
		if len(errs) != 1 {
			t.Fatalf("expected 1 error, got %d", len(errs))
		}
	})

	t.Run("wrong_type_verbose", func(t *testing.T) {
		data := []byte(`{"verbose": "yes"}`)
		errs := ValidateSettingsJSON(data)
		if len(errs) != 1 {
			t.Fatalf("expected 1 error, got %d", len(errs))
		}
	})

	t.Run("non_string_in_allowed_tools", func(t *testing.T) {
		data := []byte(`{"allowed_tools": ["Bash", 123]}`)
		errs := ValidateSettingsJSON(data)
		if len(errs) != 1 {
			t.Fatalf("expected 1 error, got %d", len(errs))
		}
		if errs[0].Path != "allowed_tools[1]" {
			t.Errorf("path = %q", errs[0].Path)
		}
	})

	t.Run("additional_properties_allowed", func(t *testing.T) {
		// Source: schema has additionalProperties: true
		data := []byte(`{"unknown_field": "value", "model": "test"}`)
		errs := ValidateSettingsJSON(data)
		if len(errs) > 0 {
			t.Errorf("additional properties should be allowed, got %v", errs)
		}
	})

	t.Run("empty_object_valid", func(t *testing.T) {
		data := []byte(`{}`)
		errs := ValidateSettingsJSON(data)
		if len(errs) > 0 {
			t.Errorf("empty object should be valid, got %v", errs)
		}
	})
}

func TestIsValidPermissionMode(t *testing.T) {
	valid := []string{"default", "acceptEdits", "bypassPermissions", "dontAsk", "plan", "auto"}
	for _, m := range valid {
		if !IsValidPermissionMode(m) {
			t.Errorf("%q should be valid", m)
		}
	}

	invalid := []string{"", "deny", "interactive", "YOLO", "Default"}
	for _, m := range invalid {
		if IsValidPermissionMode(m) {
			t.Errorf("%q should be invalid", m)
		}
	}
}
