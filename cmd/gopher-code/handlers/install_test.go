package handlers

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

func TestInstall_ForceFlag(t *testing.T) {
	var calledForce bool
	installer := func(target string, force bool) (string, error) {
		calledForce = force
		return "Install complete (forced)", nil
	}

	var buf bytes.Buffer
	code := Install(InstallOpts{
		Target:    "",
		Force:     true,
		Output:    &buf,
		Installer: installer,
	})
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !calledForce {
		t.Error("--force flag was not passed to installer")
	}
}

func TestInstall_TargetPassed(t *testing.T) {
	var capturedTarget string
	installer := func(target string, force bool) (string, error) {
		capturedTarget = target
		return "Install complete", nil
	}

	var buf bytes.Buffer
	Install(InstallOpts{
		Target:    "alpha",
		Force:     false,
		Output:    &buf,
		Installer: installer,
	})
	if capturedTarget != "alpha" {
		t.Errorf("expected target %q, got %q", "alpha", capturedTarget)
	}
}

func TestInstall_FailureDetection(t *testing.T) {
	t.Run("result containing failed returns exit 1", func(t *testing.T) {
		installer := func(target string, force bool) (string, error) {
			return "shell integration failed to install", nil
		}
		var buf bytes.Buffer
		code := Install(InstallOpts{
			Output:    &buf,
			Installer: installer,
		})
		if code != 1 {
			t.Errorf("expected exit 1 for failed result, got %d", code)
		}
	})

	t.Run("error returns exit 1", func(t *testing.T) {
		installer := func(target string, force bool) (string, error) {
			return "", fmt.Errorf("native installer not found")
		}
		var buf bytes.Buffer
		code := Install(InstallOpts{
			Output:    &buf,
			Installer: installer,
		})
		if code != 1 {
			t.Errorf("expected exit 1 for error, got %d", code)
		}
		if !strings.Contains(buf.String(), "Install failed") {
			t.Errorf("expected failure message in output, got: %s", buf.String())
		}
	})

	t.Run("success returns exit 0", func(t *testing.T) {
		installer := func(target string, force bool) (string, error) {
			return "Install complete", nil
		}
		var buf bytes.Buffer
		code := Install(InstallOpts{
			Output:    &buf,
			Installer: installer,
		})
		if code != 0 {
			t.Errorf("expected exit 0, got %d", code)
		}
	})
}

func TestInstall_DefaultInstaller(t *testing.T) {
	var buf bytes.Buffer
	code := Install(InstallOpts{
		Target: "beta",
		Force:  true,
		Output: &buf,
	})
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	out := buf.String()
	if !strings.Contains(out, "beta") {
		t.Errorf("expected target in output, got: %s", out)
	}
	if !strings.Contains(out, "forced") {
		t.Errorf("expected forced indicator in output, got: %s", out)
	}
}
