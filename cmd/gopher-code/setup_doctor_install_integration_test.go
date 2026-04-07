package main

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

// TestSetupTokenSubcommand_Integration verifies that `gopher-code setup-token`
// runs successfully and prints the expected starting message.
func TestSetupTokenSubcommand_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	bin := buildBinary(t)
	cmd := exec.Command(bin, "setup-token")
	cmd.Dir = t.TempDir()
	// Clear env vars that would trigger auth-conflict warnings, so we get
	// a clean starting message.
	cmd.Env = append(os.Environ(),
		"ANTHROPIC_API_KEY=",
		"ANTHROPIC_AUTH_TOKEN=",
		"CLAUDE_CODE_USE_BEDROCK=",
		"CLAUDE_CODE_USE_VERTEX=",
		"CLAUDE_CODE_USE_FOUNDRY=",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("gopher-code setup-token failed: %v\n%s", err, out)
	}

	got := string(out)
	if !strings.Contains(got, "long-lived") {
		t.Errorf("expected starting message containing 'long-lived', got:\n%s", got)
	}
}

// TestSetupTokenSubcommand_AuthWarning verifies that `gopher-code setup-token`
// prints an auth-conflict warning when ANTHROPIC_API_KEY is set.
func TestSetupTokenSubcommand_AuthWarning(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	bin := buildBinary(t)
	cmd := exec.Command(bin, "setup-token")
	cmd.Dir = t.TempDir()
	cmd.Env = append(os.Environ(), "ANTHROPIC_API_KEY=sk-test-key")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("gopher-code setup-token failed: %v\n%s", err, out)
	}

	got := string(out)
	if !strings.Contains(got, "Warning") {
		t.Errorf("expected auth-conflict warning, got:\n%s", got)
	}
}

// TestDoctorSubcommand_Integration verifies that `gopher-code doctor`
// runs successfully and prints the diagnostics message.
func TestDoctorSubcommand_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	bin := buildBinary(t)
	cmd := exec.Command(bin, "doctor")
	cmd.Dir = t.TempDir()
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("gopher-code doctor failed: %v\n%s", err, out)
	}

	got := string(out)
	if !strings.Contains(got, "Running diagnostics") {
		t.Errorf("expected 'Running diagnostics' in output, got:\n%s", got)
	}
}

// TestInstallSubcommand_Integration verifies that `gopher-code install`
// runs successfully with default (no target) and with a target argument.
func TestInstallSubcommand_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	bin := buildBinary(t)

	// No target
	cmd := exec.Command(bin, "install")
	cmd.Dir = t.TempDir()
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("gopher-code install failed: %v\n%s", err, out)
	}

	got := string(out)
	if !strings.Contains(got, "Install complete") {
		t.Errorf("expected 'Install complete' in output, got:\n%s", got)
	}
}

// TestInstallSubcommand_WithTarget verifies install with a specific target.
func TestInstallSubcommand_WithTarget(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	bin := buildBinary(t)
	cmd := exec.Command(bin, "install", "beta")
	cmd.Dir = t.TempDir()
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("gopher-code install beta failed: %v\n%s", err, out)
	}

	got := string(out)
	if !strings.Contains(got, "beta") {
		t.Errorf("expected 'beta' in output, got:\n%s", got)
	}
}

// TestInstallSubcommand_Force verifies install with --force flag.
func TestInstallSubcommand_Force(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	bin := buildBinary(t)
	cmd := exec.Command(bin, "install", "--force")
	cmd.Dir = t.TempDir()
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("gopher-code install --force failed: %v\n%s", err, out)
	}

	got := string(out)
	if !strings.Contains(got, "forced") {
		t.Errorf("expected 'forced' in output, got:\n%s", got)
	}
}
