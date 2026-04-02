package config

import (
	"runtime"
	"testing"
)

// Source: utils/settings/mdm/constants.ts

func TestMDMConstants(t *testing.T) {
	// Source: utils/settings/mdm/constants.ts:12, 32, 38
	if MacOSPreferenceDomain != "com.anthropic.claudecode" {
		t.Errorf("domain = %q, want 'com.anthropic.claudecode'", MacOSPreferenceDomain)
	}
	if PlutilPath != "/usr/bin/plutil" {
		t.Errorf("plutil = %q, want '/usr/bin/plutil'", PlutilPath)
	}
	if MDMSubprocessTimeoutMs != 5000 {
		t.Errorf("timeout = %d, want 5000", MDMSubprocessTimeoutMs)
	}
}

func TestGetMacOSPlistPaths(t *testing.T) {
	// Source: utils/settings/mdm/constants.ts:45-81
	paths := GetMacOSPlistPaths()

	// Should have at least the device-level path
	if len(paths) < 1 {
		t.Fatal("expected at least 1 plist path")
	}

	// Last path should be device-level
	last := paths[len(paths)-1]
	if last.Label != "device-level managed preferences" {
		t.Errorf("last label = %q, want 'device-level managed preferences'", last.Label)
	}
	if last.Path != "/Library/Managed Preferences/com.anthropic.claudecode.plist" {
		t.Errorf("last path = %q", last.Path)
	}

	// If we have a username, first should be per-user
	if len(paths) >= 2 {
		first := paths[0]
		if first.Label != "per-user managed preferences" {
			t.Errorf("first label = %q, want 'per-user managed preferences'", first.Label)
		}
	}
}

func TestLoadMDMSettings_NoMDM(t *testing.T) {
	// On most dev machines, no MDM profiles are installed
	result := LoadMDMSettings()
	if runtime.GOOS == "darwin" || runtime.GOOS == "linux" {
		// May or may not return nil depending on system
		// Just verify it doesn't panic
		_ = result
	} else {
		if result != nil {
			t.Error("expected nil on unsupported platform")
		}
	}
}
