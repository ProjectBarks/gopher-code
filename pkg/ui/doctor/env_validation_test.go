package doctor

import (
	"strings"
	"testing"
)

func TestValidateEnvBounds_Empty(t *testing.T) {
	results := ValidateEnvBounds(nil)
	if len(results) != 0 {
		t.Error("nil bounds should return no results")
	}
}

func TestValidateEnvBounds_NoEnvSet(t *testing.T) {
	bounds := []EnvBound{{Name: "NONEXISTENT_TEST_VAR_12345", DefaultValue: 10, UpperLimit: 100}}
	results := ValidateEnvBounds(bounds)
	if len(results) != 0 {
		t.Error("unset env var should return no results")
	}
}

func TestValidateEnvBounds_ExceedsLimit(t *testing.T) {
	t.Setenv("TEST_DOCTOR_VAR", "999999999")
	bounds := []EnvBound{{Name: "TEST_DOCTOR_VAR", DefaultValue: 8000, UpperLimit: 1000000}}
	results := ValidateEnvBounds(bounds)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if !strings.Contains(results[0].Issue, "exceeds upper limit") {
		t.Errorf("expected exceeds upper limit, got %q", results[0].Issue)
	}
}

func TestValidateEnvBounds_Negative(t *testing.T) {
	t.Setenv("TEST_DOCTOR_VAR", "-5")
	bounds := []EnvBound{{Name: "TEST_DOCTOR_VAR", DefaultValue: 8000, UpperLimit: 1000000}}
	results := ValidateEnvBounds(bounds)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if !strings.Contains(results[0].Issue, "negative") {
		t.Errorf("expected negative error, got %q", results[0].Issue)
	}
}

func TestValidateEnvBounds_NotInteger(t *testing.T) {
	t.Setenv("TEST_DOCTOR_VAR", "abc")
	bounds := []EnvBound{{Name: "TEST_DOCTOR_VAR", DefaultValue: 8000, UpperLimit: 1000000}}
	results := ValidateEnvBounds(bounds)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if !strings.Contains(results[0].Issue, "not a valid integer") {
		t.Errorf("expected invalid integer, got %q", results[0].Issue)
	}
}

func TestValidateEnvBounds_InBounds(t *testing.T) {
	t.Setenv("TEST_DOCTOR_VAR", "5000")
	bounds := []EnvBound{{Name: "TEST_DOCTOR_VAR", DefaultValue: 8000, UpperLimit: 1000000}}
	results := ValidateEnvBounds(bounds)
	if len(results) != 0 {
		t.Error("in-bounds value should return no results")
	}
}

func TestRenderEnvValidation_Empty(t *testing.T) {
	out := RenderEnvValidation(nil)
	if out != "" {
		t.Error("nil results should produce empty output")
	}
}

func TestRenderEnvValidation_WithResults(t *testing.T) {
	results := []EnvValidationResult{
		{Name: "BASH_MAX_OUTPUT", Value: 2000000, Issue: "exceeds upper limit of 1000000"},
	}
	out := RenderEnvValidation(results)
	if !strings.Contains(out, "Environment Variable Warnings") {
		t.Error("expected header")
	}
	if !strings.Contains(out, "BASH_MAX_OUTPUT") {
		t.Error("expected env var name")
	}
	if !strings.Contains(out, "exceeds upper limit") {
		t.Error("expected issue text")
	}
}

func TestDefaultEnvBounds(t *testing.T) {
	bounds := DefaultEnvBounds()
	if len(bounds) < 2 {
		t.Errorf("expected at least 2 default bounds, got %d", len(bounds))
	}
	names := make(map[string]bool)
	for _, b := range bounds {
		names[b.Name] = true
	}
	if !names["BASH_MAX_OUTPUT"] {
		t.Error("expected BASH_MAX_OUTPUT in defaults")
	}
	if !names["TASK_MAX_OUTPUT"] {
		t.Error("expected TASK_MAX_OUTPUT in defaults")
	}
}
