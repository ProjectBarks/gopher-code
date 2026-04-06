package doctor

import (
	"runtime"
	"strings"
	"testing"
)

func TestDetectSandboxStatus(t *testing.T) {
	status := DetectSandboxStatus()
	switch runtime.GOOS {
	case "darwin":
		if !status.Available {
			t.Error("expected sandbox available on macOS")
		}
		if status.Type != "macos-sandbox" {
			t.Errorf("expected macos-sandbox type, got %q", status.Type)
		}
	case "linux":
		if !status.Available {
			t.Error("expected sandbox available on linux")
		}
		if status.Type != "docker" {
			t.Errorf("expected docker type, got %q", status.Type)
		}
	}
	if status.Message == "" {
		t.Error("expected non-empty message")
	}
}

func TestRenderSandbox_Available(t *testing.T) {
	status := SandboxStatus{
		Available: true,
		Type:      "macos-sandbox",
		Message:   "macOS sandbox available (sandbox-exec)",
	}
	out := RenderSandbox(status)
	if !strings.Contains(out, "Sandbox") {
		t.Error("expected header")
	}
	if !strings.Contains(out, "available") {
		t.Error("expected available status")
	}
	if !strings.Contains(out, "macos-sandbox") {
		t.Error("expected type")
	}
}

func TestRenderSandbox_Unavailable(t *testing.T) {
	status := SandboxStatus{
		Available: false,
		Type:      "none",
		Message:   "No sandbox support on windows",
	}
	out := RenderSandbox(status)
	if !strings.Contains(out, "Sandbox") {
		t.Error("expected header")
	}
	if !strings.Contains(out, "unavailable") {
		t.Error("expected unavailable status")
	}
}
