package permissions

import "testing"

// Source: utils/permissions/PermissionMode.ts

func TestPermissionModeTitle(t *testing.T) {
	// Source: PermissionMode.ts:42-91 — verbatim strings
	tests := []struct {
		mode  PermissionMode
		title string
	}{
		{ModeDefault, "Default"},
		{ModePlan, "Plan Mode"},
		{ModeAcceptEdits, "Accept edits"},
		{ModeBypassPermissions, "Bypass Permissions"},
		{ModeDontAsk, "Don't Ask"},
		{ModeAuto, "Auto mode"},
	}
	for _, tt := range tests {
		got := PermissionModeTitle(tt.mode)
		if got != tt.title {
			t.Errorf("PermissionModeTitle(%q) = %q, want %q", tt.mode, got, tt.title)
		}
	}
}

func TestPermissionModeShortTitle(t *testing.T) {
	// Source: PermissionMode.ts:42-91
	tests := []struct {
		mode  PermissionMode
		short string
	}{
		{ModeDefault, "Default"},
		{ModePlan, "Plan"},
		{ModeAcceptEdits, "Accept"},
		{ModeBypassPermissions, "Bypass"},
		{ModeDontAsk, "DontAsk"},
		{ModeAuto, "Auto"},
	}
	for _, tt := range tests {
		got := PermissionModeShortTitle(tt.mode)
		if got != tt.short {
			t.Errorf("PermissionModeShortTitle(%q) = %q, want %q", tt.mode, got, tt.short)
		}
	}
}

func TestPermissionModeSymbol(t *testing.T) {
	// Source: PermissionMode.ts:42-91
	if PermissionModeSymbol(ModeDefault) != "" {
		t.Error("default should have no symbol")
	}
	if PermissionModeSymbol(ModePlan) != PauseIcon {
		t.Errorf("plan symbol = %q, want %q", PermissionModeSymbol(ModePlan), PauseIcon)
	}
	if PermissionModeSymbol(ModeBypassPermissions) != "⏵⏵" {
		t.Error("bypass should use ⏵⏵")
	}
	if PermissionModeSymbol(ModeAuto) != "⏵⏵" {
		t.Error("auto should use ⏵⏵")
	}
}

func TestGetModeColor(t *testing.T) {
	// Source: PermissionMode.ts:42-91
	tests := []struct {
		mode  PermissionMode
		color ModeColorKey
	}{
		{ModeDefault, ColorText},
		{ModePlan, ColorPlanMode},
		{ModeAcceptEdits, ColorAutoAccept},
		{ModeBypassPermissions, ColorError},
		{ModeDontAsk, ColorError},
		{ModeAuto, ColorWarning},
	}
	for _, tt := range tests {
		got := GetModeColor(tt.mode)
		if got != tt.color {
			t.Errorf("GetModeColor(%q) = %q, want %q", tt.mode, got, tt.color)
		}
	}
}

func TestPermissionModeFromString(t *testing.T) {
	// Source: PermissionMode.ts:117-121

	t.Run("valid_mode", func(t *testing.T) {
		if PermissionModeFromString("bypassPermissions") != ModeBypassPermissions {
			t.Error("should parse bypassPermissions")
		}
	})

	t.Run("auto_mode", func(t *testing.T) {
		if PermissionModeFromString("auto") != ModeAuto {
			t.Error("should parse auto")
		}
	})

	t.Run("invalid_defaults", func(t *testing.T) {
		if PermissionModeFromString("garbage") != ModeDefault {
			t.Error("invalid should default to ModeDefault")
		}
	})

	t.Run("empty_defaults", func(t *testing.T) {
		if PermissionModeFromString("") != ModeDefault {
			t.Error("empty should default to ModeDefault")
		}
	})
}

func TestIsExternalPermissionMode(t *testing.T) {
	// Source: PermissionMode.ts:97-105
	if !IsExternalPermissionMode(ModeDefault) {
		t.Error("default is external")
	}
	if !IsExternalPermissionMode(ModeBypassPermissions) {
		t.Error("bypass is external")
	}
	if IsExternalPermissionMode(ModeAuto) {
		t.Error("auto is NOT external")
	}
}

func TestToExternalPermissionMode(t *testing.T) {
	// Source: PermissionMode.ts:111-115
	if ToExternalPermissionMode(ModeDefault) != ModeDefault {
		t.Error("default → default")
	}
	if ToExternalPermissionMode(ModeAuto) != ModeDefault {
		t.Error("auto → default (internal only)")
	}
	if ToExternalPermissionMode(ModeBypassPermissions) != ModeBypassPermissions {
		t.Error("bypass → bypass")
	}
}

func TestIsDefaultMode(t *testing.T) {
	// Source: PermissionMode.ts:127-129
	if !IsDefaultMode(ModeDefault) {
		t.Error("ModeDefault should be default")
	}
	if !IsDefaultMode("") {
		t.Error("empty should be default")
	}
	if IsDefaultMode(ModeBypassPermissions) {
		t.Error("bypass is not default")
	}
}

func TestModeConfigFallback(t *testing.T) {
	// Unknown mode should fall back to default config
	got := PermissionModeTitle("nonexistent")
	if got != "Default" {
		t.Errorf("unknown mode title = %q, want Default", got)
	}
}
