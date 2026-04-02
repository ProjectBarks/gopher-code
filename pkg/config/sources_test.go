package config

import (
	"errors"
	"testing"
)

// Source: utils/settings/constants.ts

func TestSettingSources(t *testing.T) {
	// Source: utils/settings/constants.ts:7-22
	t.Run("order_matches_ts", func(t *testing.T) {
		expected := []SettingSource{SourceUser, SourceProject, SourceLocal, SourceFlag, SourcePolicy}
		if len(SettingSources) != len(expected) {
			t.Fatalf("expected %d sources, got %d", len(expected), len(SettingSources))
		}
		for i, s := range expected {
			if SettingSources[i] != s {
				t.Errorf("source[%d] = %q, want %q", i, SettingSources[i], s)
			}
		}
	})

	t.Run("string_values_match_ts", func(t *testing.T) {
		if string(SourceUser) != "userSettings" {
			t.Error("SourceUser")
		}
		if string(SourceProject) != "projectSettings" {
			t.Error("SourceProject")
		}
		if string(SourceLocal) != "localSettings" {
			t.Error("SourceLocal")
		}
		if string(SourceFlag) != "flagSettings" {
			t.Error("SourceFlag")
		}
		if string(SourcePolicy) != "policySettings" {
			t.Error("SourcePolicy")
		}
	})
}

func TestSourceDisplayName(t *testing.T) {
	// Source: utils/settings/constants.ts:26-39
	tests := []struct {
		source SettingSource
		name   string
	}{
		{SourceUser, "user"},
		{SourceProject, "project"},
		{SourceLocal, "project, gitignored"},
		{SourceFlag, "cli flag"},
		{SourcePolicy, "managed"},
	}
	for _, tt := range tests {
		got := SourceDisplayName(tt.source)
		if got != tt.name {
			t.Errorf("SourceDisplayName(%q) = %q, want %q", tt.source, got, tt.name)
		}
	}
}

func TestParseSettingSourcesFlag(t *testing.T) {
	// Source: utils/settings/constants.ts:128-153

	t.Run("valid_sources", func(t *testing.T) {
		result, err := ParseSettingSourcesFlag("user,project,local")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(result) != 3 {
			t.Fatalf("expected 3 sources, got %d", len(result))
		}
		if result[0] != SourceUser || result[1] != SourceProject || result[2] != SourceLocal {
			t.Errorf("unexpected sources: %v", result)
		}
	})

	t.Run("empty_string", func(t *testing.T) {
		result, err := ParseSettingSourcesFlag("")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != nil {
			t.Errorf("expected nil, got %v", result)
		}
	})

	t.Run("invalid_source_errors", func(t *testing.T) {
		// Source: utils/settings/constants.ts:146-149
		_, err := ParseSettingSourcesFlag("user,invalid")
		if err == nil {
			t.Fatal("expected error for invalid source")
		}
		var ise *InvalidSourceError
		if !errors.As(err, &ise) {
			t.Errorf("expected InvalidSourceError, got %T", err)
		}
		if ise.Name != "invalid" {
			t.Errorf("error name = %q, want 'invalid'", ise.Name)
		}
	})

	t.Run("trims_whitespace", func(t *testing.T) {
		result, err := ParseSettingSourcesFlag(" user , project ")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(result) != 2 {
			t.Errorf("expected 2 sources, got %d", len(result))
		}
	})
}

func TestGetEnabledSources(t *testing.T) {
	// Source: utils/settings/constants.ts:159-167

	t.Run("always_includes_policy_and_flag", func(t *testing.T) {
		result := GetEnabledSources([]SettingSource{SourceUser})
		hasPolicy := false
		hasFlag := false
		for _, s := range result {
			if s == SourcePolicy {
				hasPolicy = true
			}
			if s == SourceFlag {
				hasFlag = true
			}
		}
		if !hasPolicy {
			t.Error("should always include policySettings")
		}
		if !hasFlag {
			t.Error("should always include flagSettings")
		}
	})

	t.Run("no_duplicates", func(t *testing.T) {
		result := GetEnabledSources([]SettingSource{SourcePolicy, SourceFlag, SourceUser})
		seen := make(map[SettingSource]bool)
		for _, s := range result {
			if seen[s] {
				t.Errorf("duplicate source: %s", s)
			}
			seen[s] = true
		}
	})

	t.Run("preserves_order", func(t *testing.T) {
		result := GetEnabledSources([]SettingSource{SourceUser, SourceProject})
		if result[0] != SourceUser {
			t.Errorf("first should be user, got %s", result[0])
		}
		if result[1] != SourceProject {
			t.Errorf("second should be project, got %s", result[1])
		}
	})
}
