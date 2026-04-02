package tools

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// Source: utils/sandbox/sandbox-adapter.ts, @anthropic-ai/sandbox-runtime

// SandboxType identifies the sandboxing mechanism available.
type SandboxType string

const (
	SandboxNone      SandboxType = "none"
	SandboxSeatbelt  SandboxType = "seatbelt"  // macOS sandbox-exec
	SandboxBubblewrap SandboxType = "bubblewrap" // Linux bwrap
)

// SandboxConfig controls what the sandbox allows.
type SandboxConfig struct {
	AllowNetwork    bool
	AllowedPaths    []string // Read-write access
	ReadOnlyPaths   []string // Read-only access
	WorkingDir      string
}

// DetectSandbox returns the available sandbox type for the current platform.
// Source: sandbox-adapter.ts — platform detection
func DetectSandbox() SandboxType {
	switch runtime.GOOS {
	case "darwin":
		if _, err := exec.LookPath("sandbox-exec"); err == nil {
			return SandboxSeatbelt
		}
	case "linux":
		if _, err := exec.LookPath("bwrap"); err == nil {
			return SandboxBubblewrap
		}
	}
	return SandboxNone
}

// IsSandboxAvailable returns true if any sandbox mechanism is available.
func IsSandboxAvailable() bool {
	return DetectSandbox() != SandboxNone
}

// WrapCommand wraps a shell command with sandbox restrictions.
// Returns the modified command and args, or the original if no sandbox available.
func WrapCommand(command string, cfg SandboxConfig) (string, []string) {
	sandbox := DetectSandbox()
	switch sandbox {
	case SandboxSeatbelt:
		return wrapSeatbelt(command, cfg)
	case SandboxBubblewrap:
		return wrapBubblewrap(command, cfg)
	default:
		return "sh", []string{"-c", command}
	}
}

// wrapSeatbelt wraps a command with macOS sandbox-exec.
// Source: @anthropic-ai/sandbox-runtime — seatbelt profile generation
func wrapSeatbelt(command string, cfg SandboxConfig) (string, []string) {
	profile := generateSeatbeltProfile(cfg)

	// Write profile to temp file
	tmpDir := os.TempDir()
	profilePath := filepath.Join(tmpDir, ".claude-sandbox.sb")
	os.WriteFile(profilePath, []byte(profile), 0600)

	return "sandbox-exec", []string{"-f", profilePath, "sh", "-c", command}
}

// generateSeatbeltProfile creates a macOS seatbelt SBPL profile.
// Source: @anthropic-ai/sandbox-runtime — profile generation
func generateSeatbeltProfile(cfg SandboxConfig) string {
	var sb strings.Builder
	sb.WriteString("(version 1)\n")
	sb.WriteString("(deny default)\n")
	sb.WriteString("(allow process-exec)\n")
	sb.WriteString("(allow process-fork)\n")
	sb.WriteString("(allow sysctl-read)\n")
	sb.WriteString("(allow mach-lookup)\n")
	sb.WriteString("(allow signal)\n")
	sb.WriteString("(allow ipc-posix-shm-read*)\n")

	// Allow reading system paths
	for _, path := range []string{"/usr", "/bin", "/sbin", "/dev", "/etc", "/var/tmp", "/tmp", "/private/tmp"} {
		sb.WriteString(fmt.Sprintf("(allow file-read* (subpath \"%s\"))\n", path))
	}

	// Read-only paths
	for _, path := range cfg.ReadOnlyPaths {
		sb.WriteString(fmt.Sprintf("(allow file-read* (subpath \"%s\"))\n", path))
	}

	// Read-write paths
	for _, path := range cfg.AllowedPaths {
		sb.WriteString(fmt.Sprintf("(allow file-read* file-write* (subpath \"%s\"))\n", path))
	}

	// Working directory always writable
	if cfg.WorkingDir != "" {
		sb.WriteString(fmt.Sprintf("(allow file-read* file-write* (subpath \"%s\"))\n", cfg.WorkingDir))
	}

	// Network
	if cfg.AllowNetwork {
		sb.WriteString("(allow network*)\n")
	}

	return sb.String()
}

// wrapBubblewrap wraps a command with Linux bwrap.
// Source: @anthropic-ai/sandbox-runtime — bwrap wrapper
func wrapBubblewrap(command string, cfg SandboxConfig) (string, []string) {
	args := []string{
		"--die-with-parent",
		"--ro-bind", "/usr", "/usr",
		"--ro-bind", "/bin", "/bin",
		"--ro-bind", "/lib", "/lib",
		"--ro-bind", "/etc", "/etc",
		"--dev", "/dev",
		"--proc", "/proc",
		"--tmpfs", "/tmp",
	}

	// Bind /lib64 if it exists (needed for x86_64)
	if _, err := os.Stat("/lib64"); err == nil {
		args = append(args, "--ro-bind", "/lib64", "/lib64")
	}

	// Read-only paths
	for _, path := range cfg.ReadOnlyPaths {
		args = append(args, "--ro-bind", path, path)
	}

	// Read-write paths
	for _, path := range cfg.AllowedPaths {
		args = append(args, "--bind", path, path)
	}

	// Working directory
	if cfg.WorkingDir != "" {
		args = append(args, "--bind", cfg.WorkingDir, cfg.WorkingDir)
		args = append(args, "--chdir", cfg.WorkingDir)
	}

	// Network (bwrap shares network by default, --unshare-net to disable)
	if !cfg.AllowNetwork {
		args = append(args, "--unshare-net")
	}

	args = append(args, "sh", "-c", command)
	return "bwrap", args
}
