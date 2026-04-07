package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestAgentsSubcommand_Integration builds the binary and verifies that
// `gopher-code agents` runs successfully and produces expected output.
func TestAgentsSubcommand_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Build the binary into a temp dir.
	tmp := t.TempDir()
	bin := filepath.Join(tmp, "gopher-code")
	build := exec.Command("go", "build", "-o", bin, ".")
	build.Dir = filepath.Dir(selfPath(t))
	build.Env = append(os.Environ(), "CGO_ENABLED=0")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build failed: %v\n%s", err, out)
	}

	// Run the binary with "agents" subcommand in an empty temp dir so there
	// are no agents to discover.
	cwd := t.TempDir()
	cmd := exec.Command(bin, "agents")
	cmd.Dir = cwd
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("gopher-code agents failed: %v\n%s", err, out)
	}

	got := string(out)
	if !strings.Contains(got, "No agents found.") {
		t.Errorf("expected 'No agents found.' in output, got:\n%s", got)
	}
}

// TestAgentsSubcommand_WithAgents builds the binary and verifies that it
// discovers and lists agents from a project directory.
func TestAgentsSubcommand_WithAgents(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Build the binary.
	tmp := t.TempDir()
	bin := filepath.Join(tmp, "gopher-code")
	build := exec.Command("go", "build", "-o", bin, ".")
	build.Dir = filepath.Dir(selfPath(t))
	build.Env = append(os.Environ(), "CGO_ENABLED=0")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build failed: %v\n%s", err, out)
	}

	// Set up a project dir with an agent.
	cwd := t.TempDir()
	agentsDir := filepath.Join(cwd, ".claude", "agents")
	if err := os.MkdirAll(agentsDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(agentsDir, "helper.md"), []byte("# Helper\n"), 0644); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(bin, "agents")
	cmd.Dir = cwd
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("gopher-code agents failed: %v\n%s", err, out)
	}

	got := string(out)
	if !strings.Contains(got, "helper") {
		t.Errorf("expected 'helper' in output, got:\n%s", got)
	}
	if !strings.Contains(got, "1 active agents") {
		t.Errorf("expected '1 active agents' header, got:\n%s", got)
	}
}

// selfPath returns the directory of this test file.
func selfPath(t *testing.T) string {
	t.Helper()
	// Use the known package path relative to the module root.
	// runtime.Caller would work but is fragile in some test runners.
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Join(wd, "agents_integration_test.go")
}
