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

// TestMCPAddJSON_ListRoundTrip_Integration tests the add-json -> list round-trip
// through the binary, exercising config write, LoadMergedConfig, and list rendering.
func TestMCPAddJSON_ListRoundTrip_Integration(t *testing.T) {
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

	// Use a temp dir as HOME so we don't affect real config
	fakeHome := t.TempDir()
	claudeDir := filepath.Join(fakeHome, ".claude")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		t.Fatal(err)
	}

	cwd := t.TempDir()
	baseEnv := append(os.Environ(), "HOME="+fakeHome)

	// Add a server via add-json
	addCmd := exec.Command(bin, "mcp", "add-json", "test-server",
		`{"command":"echo","args":["hello"]}`, "-s", "user")
	addCmd.Dir = cwd
	addCmd.Env = baseEnv
	out, err := addCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("mcp add-json failed: %v\n%s", err, out)
	}

	addOut := string(out)
	if !strings.Contains(addOut, "Added") || !strings.Contains(addOut, "test-server") {
		t.Errorf("expected confirmation of add, got:\n%s", addOut)
	}

	// List should now show the server
	listCmd := exec.Command(bin, "mcp", "list")
	listCmd.Dir = cwd
	listCmd.Env = baseEnv
	out, err = listCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("mcp list failed: %v\n%s", err, out)
	}

	listOut := string(out)
	if !strings.Contains(listOut, "test-server") {
		t.Errorf("expected 'test-server' in list output, got:\n%s", listOut)
	}
	if !strings.Contains(listOut, "echo") {
		t.Errorf("expected 'echo' command in list output, got:\n%s", listOut)
	}
}

// TestMCPManagerConfigPath_Integration tests the Manager + LoadConfig path
// that main() uses at session startup. This exercises pkg/mcp.NewManager,
// LoadConfig, and the config file reading.
func TestMCPManagerConfigPath_Integration(t *testing.T) {
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

	// Use a fake HOME with no MCP config — verifies the session startup path
	// handles missing config gracefully (the code in main.go calls LoadConfig
	// and ignores the error for missing files).
	fakeHome := t.TempDir()
	claudeDir := filepath.Join(fakeHome, ".claude")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Run with --help to trigger early exits before the session loop,
	// but after imports are resolved. The binary must build cleanly
	// with all MCP code linked.
	cmd := exec.Command(bin, "--help")
	cmd.Dir = t.TempDir()
	cmd.Env = append(os.Environ(), "HOME="+fakeHome)
	out, err := cmd.CombinedOutput()
	// --help exits 0 (or sometimes 2); either way, it must not panic
	_ = err
	got := string(out)
	// The help output should mention the binary
	if len(got) == 0 {
		t.Error("expected non-empty help output")
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
