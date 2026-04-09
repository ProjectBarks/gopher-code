package output_styles

import (
	"strings"
	"testing"
)

func TestBuiltInStyles(t *testing.T) {
	if len(BuiltInStyles) < 3 {
		t.Errorf("expected at least 3 built-in styles, got %d", len(BuiltInStyles))
	}

	// Default should be nil
	if BuiltInStyles[DefaultOutputStyleName] != nil {
		t.Error("default style should be nil")
	}

	// Explanatory should exist
	expl := BuiltInStyles["Explanatory"]
	if expl == nil {
		t.Fatal("Explanatory style should exist")
	}
	if expl.Name != "Explanatory" {
		t.Errorf("name = %q", expl.Name)
	}
	if !strings.Contains(expl.Prompt, "educational") {
		t.Error("Explanatory prompt should mention educational")
	}
	if expl.KeepCodingInstructions == nil || !*expl.KeepCodingInstructions {
		t.Error("Explanatory should keep coding instructions")
	}

	// Learning should exist
	learn := BuiltInStyles["Learning"]
	if learn == nil {
		t.Fatal("Learning style should exist")
	}
	if !strings.Contains(learn.Prompt, "hands-on") {
		t.Error("Learning prompt should mention hands-on")
	}
}

func TestGetAllOutputStyles(t *testing.T) {
	dir := t.TempDir()
	all := GetAllOutputStyles(dir)

	if len(all) < 3 {
		t.Errorf("expected at least 3 styles, got %d", len(all))
	}
	if _, ok := all[DefaultOutputStyleName]; !ok {
		t.Error("should include default style")
	}
	if _, ok := all["Explanatory"]; !ok {
		t.Error("should include Explanatory")
	}
}

func TestGetOutputStyleNames(t *testing.T) {
	dir := t.TempDir()
	names := GetOutputStyleNames(dir)
	if len(names) < 3 {
		t.Errorf("expected at least 3 names, got %d", len(names))
	}
}

func TestGetOutputStyleConfig_Default(t *testing.T) {
	dir := t.TempDir()
	cfg := GetOutputStyleConfig(dir)
	// With no settings, should return nil (default style)
	if cfg != nil {
		t.Error("default config should be nil")
	}
}
