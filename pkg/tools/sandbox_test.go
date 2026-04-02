package tools

import (
	"runtime"
	"strings"
	"testing"
)

// Source: utils/sandbox/sandbox-adapter.ts, @anthropic-ai/sandbox-runtime

func TestDetectSandbox(t *testing.T) {
	result := DetectSandbox()
	switch runtime.GOOS {
	case "darwin":
		// macOS should have sandbox-exec
		if result != SandboxSeatbelt {
			t.Logf("sandbox-exec not found on this macOS (result: %s)", result)
		}
	case "linux":
		// Linux may or may not have bwrap
		t.Logf("detected sandbox: %s", result)
	default:
		if result != SandboxNone {
			t.Errorf("expected none on %s, got %s", runtime.GOOS, result)
		}
	}
}

func TestGenerateSeatbeltProfile(t *testing.T) {
	// Source: @anthropic-ai/sandbox-runtime — seatbelt profile generation

	t.Run("default_profile", func(t *testing.T) {
		profile := generateSeatbeltProfile(SandboxConfig{
			WorkingDir: "/home/user/project",
		})
		if !strings.Contains(profile, "(version 1)") {
			t.Error("should start with version 1")
		}
		if !strings.Contains(profile, "(deny default)") {
			t.Error("should deny by default")
		}
		if !strings.Contains(profile, "/home/user/project") {
			t.Error("should allow working directory")
		}
	})

	t.Run("with_network", func(t *testing.T) {
		profile := generateSeatbeltProfile(SandboxConfig{
			AllowNetwork: true,
		})
		if !strings.Contains(profile, "(allow network*)") {
			t.Error("should allow network when configured")
		}
	})

	t.Run("without_network", func(t *testing.T) {
		profile := generateSeatbeltProfile(SandboxConfig{
			AllowNetwork: false,
		})
		if strings.Contains(profile, "(allow network*)") {
			t.Error("should NOT allow network when disabled")
		}
	})

	t.Run("read_only_paths", func(t *testing.T) {
		profile := generateSeatbeltProfile(SandboxConfig{
			ReadOnlyPaths: []string{"/opt/data"},
		})
		if !strings.Contains(profile, "file-read*") || !strings.Contains(profile, "/opt/data") {
			t.Error("should include read-only paths")
		}
	})

	t.Run("read_write_paths", func(t *testing.T) {
		profile := generateSeatbeltProfile(SandboxConfig{
			AllowedPaths: []string{"/tmp/output"},
		})
		if !strings.Contains(profile, "file-write*") || !strings.Contains(profile, "/tmp/output") {
			t.Error("should include read-write paths")
		}
	})
}

func TestWrapBubblewrap(t *testing.T) {
	// Source: @anthropic-ai/sandbox-runtime — bwrap wrapper

	t.Run("basic_command", func(t *testing.T) {
		cmd, args := wrapBubblewrap("echo hello", SandboxConfig{
			WorkingDir: "/home/user/project",
		})
		if cmd != "bwrap" {
			t.Errorf("expected bwrap, got %s", cmd)
		}
		// Should contain die-with-parent
		found := false
		for _, a := range args {
			if a == "--die-with-parent" {
				found = true
			}
		}
		if !found {
			t.Error("should include --die-with-parent")
		}
		// Last args should be the command
		if args[len(args)-1] != "echo hello" {
			t.Errorf("last arg should be command, got %q", args[len(args)-1])
		}
	})

	t.Run("no_network", func(t *testing.T) {
		_, args := wrapBubblewrap("cmd", SandboxConfig{AllowNetwork: false})
		found := false
		for _, a := range args {
			if a == "--unshare-net" {
				found = true
			}
		}
		if !found {
			t.Error("should include --unshare-net when network disabled")
		}
	})

	t.Run("with_network", func(t *testing.T) {
		_, args := wrapBubblewrap("cmd", SandboxConfig{AllowNetwork: true})
		for _, a := range args {
			if a == "--unshare-net" {
				t.Error("should NOT include --unshare-net when network enabled")
			}
		}
	})
}
