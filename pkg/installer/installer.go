// Package installer provides native installer utilities.
// Source: utils/installer/ (platform-specific installation)
package installer

import (
	"os"
	"path/filepath"
	"runtime"
)

// InstallDir returns the default installation directory for the platform.
func InstallDir() string {
	switch runtime.GOOS {
	case "darwin":
		home, _ := os.UserHomeDir()
		return filepath.Join(home, ".claude", "bin")
	case "linux":
		home, _ := os.UserHomeDir()
		return filepath.Join(home, ".local", "bin")
	case "windows":
		appData := os.Getenv("LOCALAPPDATA")
		if appData == "" {
			appData, _ = os.UserHomeDir()
		}
		return filepath.Join(appData, "Claude", "bin")
	default:
		home, _ := os.UserHomeDir()
		return filepath.Join(home, ".claude", "bin")
	}
}

// BinaryName returns the binary name for the current platform.
func BinaryName() string {
	if runtime.GOOS == "windows" {
		return "claude.exe"
	}
	return "claude"
}

// IsInstalled checks if the CLI is installed in the default location.
func IsInstalled() bool {
	path := filepath.Join(InstallDir(), BinaryName())
	_, err := os.Stat(path)
	return err == nil
}
