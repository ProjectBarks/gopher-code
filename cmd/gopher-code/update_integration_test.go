package main

import (
	"os/exec"
	"strings"
	"testing"
)

// TestUpdateSubcommand_Integration verifies that `gopher-code update`
// dispatches to the update handler and produces the expected version output.
func TestUpdateSubcommand_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	bin := buildBinary(t)
	cmd := exec.Command(bin, "update")
	cmd.Dir = t.TempDir()
	out, _ := cmd.CombinedOutput()

	got := string(out)
	// The update handler always prints the current version first,
	// regardless of whether the network fetch succeeds or fails.
	if !strings.Contains(got, "Current version:") {
		t.Errorf("expected 'Current version:' in output, got:\n%s", got)
	}
	if !strings.Contains(got, "Checking for updates") {
		t.Errorf("expected 'Checking for updates' in output, got:\n%s", got)
	}
}
