package config

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"time"
)

// Source: utils/settings/mdm/constants.ts, utils/settings/mdm/rawRead.ts

// MDM constants matching TS source.
// Source: utils/settings/mdm/constants.ts
const (
	// MacOSPreferenceDomain is the preference domain for Claude Code MDM profiles.
	// Source: utils/settings/mdm/constants.ts:12
	MacOSPreferenceDomain = "com.anthropic.claudecode"

	// PlutilPath is the path to the macOS plutil binary.
	// Source: utils/settings/mdm/constants.ts:32
	PlutilPath = "/usr/bin/plutil"

	// MDMSubprocessTimeoutMs is the timeout for plutil/registry subprocesses.
	// Source: utils/settings/mdm/constants.ts:38
	MDMSubprocessTimeoutMs = 5000
)

// PlistPath is a macOS plist path with a label for logging.
type PlistPath struct {
	Path  string
	Label string
}

// GetMacOSPlistPaths returns the macOS plist paths in priority order (highest first).
// Source: utils/settings/mdm/constants.ts:45-81
func GetMacOSPlistPaths() []PlistPath {
	var paths []PlistPath

	u, err := user.Current()
	if err == nil && u.Username != "" {
		paths = append(paths, PlistPath{
			Path:  filepath.Join("/Library/Managed Preferences", u.Username, MacOSPreferenceDomain+".plist"),
			Label: "per-user managed preferences",
		})
	}

	paths = append(paths, PlistPath{
		Path:  filepath.Join("/Library/Managed Preferences", MacOSPreferenceDomain+".plist"),
		Label: "device-level managed preferences",
	})

	return paths
}

// LoadMDMSettings reads MDM-managed settings from the OS preference system.
// On macOS: reads plist via plutil. On Linux: reads managed-settings.json.
// Returns nil if no MDM settings are found.
// Source: utils/settings/mdm/rawRead.ts:51-80
func LoadMDMSettings() *Settings {
	switch runtime.GOOS {
	case "darwin":
		return loadMacOSMDM()
	case "linux":
		return loadLinuxMDM()
	default:
		return nil
	}
}

// loadMacOSMDM spawns plutil for each plist path, returns first success.
// Source: utils/settings/mdm/rawRead.ts:51-80
func loadMacOSMDM() *Settings {
	for _, p := range GetMacOSPlistPaths() {
		// Fast-path: skip if plist doesn't exist
		// Source: utils/settings/mdm/rawRead.ts:62-63
		if _, err := os.Stat(p.Path); os.IsNotExist(err) {
			continue
		}

		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(MDMSubprocessTimeoutMs)*time.Millisecond)
		defer cancel()

		// Source: utils/settings/mdm/constants.ts:34-35
		cmd := exec.CommandContext(ctx, PlutilPath, "-convert", "json", "-o", "-", "--", p.Path)
		output, err := cmd.Output()
		if err != nil {
			continue
		}

		var s Settings
		if err := json.Unmarshal(output, &s); err != nil {
			continue
		}

		return &s
	}
	return nil
}

// loadLinuxMDM reads managed-settings.json from the standard path.
// Source: utils/settings/settings.ts:394 — file-based managed settings
func loadLinuxMDM() *Settings {
	path := GetManagedSettingsFilePath()
	s, err := loadSettingsFile(path)
	if err != nil {
		return nil
	}
	return s
}
