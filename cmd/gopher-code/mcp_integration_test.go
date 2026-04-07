package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestMCPListSubcommand_Integration builds the binary and verifies that
// `gopher-code mcp list` runs successfully with no servers configured.
func TestMCPListSubcommand_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Build the binary into a temp dir.
	tmp := t.TempDir()
	bin := filepath.Join(tmp, "gopher-code")
	build := exec.Command("go", "build", "-o", bin, ".")
	build.Dir = filepath.Dir(selfPathMCP(t))
	build.Env = append(os.Environ(), "CGO_ENABLED=0")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build failed: %v\n%s", err, out)
	}

	// Run `mcp list` in an empty temp dir — no servers configured.
	cwd := t.TempDir()
	cmd := exec.Command(bin, "mcp", "list")
	cmd.Dir = cwd
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("gopher-code mcp list failed: %v\n%s", err, out)
	}

	got := string(out)
	if !strings.Contains(got, "No MCP servers configured") {
		t.Errorf("expected 'No MCP servers configured' in output, got:\n%s", got)
	}
}

// TestMCPUnknownSubcommand_Integration verifies that an unknown mcp subcommand
// produces an error.
func TestMCPUnknownSubcommand_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tmp := t.TempDir()
	bin := filepath.Join(tmp, "gopher-code")
	build := exec.Command("go", "build", "-o", bin, ".")
	build.Dir = filepath.Dir(selfPathMCP(t))
	build.Env = append(os.Environ(), "CGO_ENABLED=0")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build failed: %v\n%s", err, out)
	}

	cmd := exec.Command(bin, "mcp", "bogus")
	cmd.Dir = t.TempDir()
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("expected non-zero exit for unknown mcp subcommand")
	}

	got := string(out)
	if !strings.Contains(got, "Unknown mcp subcommand") {
		t.Errorf("expected 'Unknown mcp subcommand' in output, got:\n%s", got)
	}
}

// TestMCPResetChoicesSubcommand_Integration verifies that `mcp reset-choices`
// runs successfully.
func TestMCPResetChoicesSubcommand_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tmp := t.TempDir()
	bin := filepath.Join(tmp, "gopher-code")
	build := exec.Command("go", "build", "-o", bin, ".")
	build.Dir = filepath.Dir(selfPathMCP(t))
	build.Env = append(os.Environ(), "CGO_ENABLED=0")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build failed: %v\n%s", err, out)
	}

	cmd := exec.Command(bin, "mcp", "reset-choices")
	cmd.Dir = t.TempDir()
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("gopher-code mcp reset-choices failed: %v\n%s", err, out)
	}

	got := string(out)
	if !strings.Contains(got, "approvals and rejections have been reset") {
		t.Errorf("expected reset confirmation in output, got:\n%s", got)
	}
}

func selfPathMCP(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Join(wd, "mcp_integration_test.go")
}
