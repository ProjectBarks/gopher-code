package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestPluginSubcommand_ListEmpty builds the binary and verifies that
// `gopher-code plugin list` runs successfully with no plugins installed.
func TestPluginSubcommand_ListEmpty(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	bin := buildPluginBinary(t)
	cwd := t.TempDir()

	cmd := exec.Command(bin, "plugin", "list")
	cmd.Dir = cwd
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("gopher-code plugin list failed: %v\n%s", err, out)
	}

	got := string(out)
	if !strings.Contains(got, "No plugins installed") {
		t.Errorf("expected 'No plugins installed' in output, got:\n%s", got)
	}
}

// TestPluginSubcommand_ListAlias verifies that `plugin ls` is an alias for `plugin list`.
func TestPluginSubcommand_ListAlias(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	bin := buildPluginBinary(t)
	cwd := t.TempDir()

	cmd := exec.Command(bin, "plugin", "ls")
	cmd.Dir = cwd
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("gopher-code plugin ls failed: %v\n%s", err, out)
	}

	got := string(out)
	if !strings.Contains(got, "No plugins installed") {
		t.Errorf("expected 'No plugins installed' in output, got:\n%s", got)
	}
}

// TestPluginSubcommand_InstallAndList installs a plugin from a local directory
// and verifies it appears in the list output.
func TestPluginSubcommand_InstallAndList(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	bin := buildPluginBinary(t)
	cwd := t.TempDir()

	// Create a local plugin directory with a manifest.
	pluginDir := filepath.Join(cwd, "my-plugin")
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		t.Fatal(err)
	}
	manifest := map[string]string{
		"name":        "test-plugin",
		"version":     "1.0.0",
		"description": "A test plugin",
	}
	raw, _ := json.Marshal(manifest)
	if err := os.WriteFile(filepath.Join(pluginDir, "plugin.json"), raw, 0644); err != nil {
		t.Fatal(err)
	}

	// Install the plugin.
	installCmd := exec.Command(bin, "plugin", "install", pluginDir)
	installCmd.Dir = cwd
	out, err := installCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("plugin install failed: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "Successfully installed") {
		t.Errorf("expected success message, got:\n%s", out)
	}

	// List should now show it.
	listCmd := exec.Command(bin, "plugin", "list")
	listCmd.Dir = cwd
	out, err = listCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("plugin list failed: %v\n%s", err, out)
	}
	got := string(out)
	if !strings.Contains(got, "test-plugin") {
		t.Errorf("expected 'test-plugin' in list output, got:\n%s", got)
	}
}

// TestPluginSubcommand_UninstallPlugin installs and then uninstalls a plugin.
func TestPluginSubcommand_UninstallPlugin(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	bin := buildPluginBinary(t)
	cwd := t.TempDir()

	// Create and install a plugin.
	pluginDir := filepath.Join(cwd, "my-plugin")
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(map[string]string{"name": "rm-plugin", "version": "1.0.0"})
	if err := os.WriteFile(filepath.Join(pluginDir, "plugin.json"), raw, 0644); err != nil {
		t.Fatal(err)
	}

	installCmd := exec.Command(bin, "plugin", "install", pluginDir)
	installCmd.Dir = cwd
	if out, err := installCmd.CombinedOutput(); err != nil {
		t.Fatalf("plugin install failed: %v\n%s", err, out)
	}

	// Uninstall.
	uninstallCmd := exec.Command(bin, "plugin", "uninstall", "rm-plugin")
	uninstallCmd.Dir = cwd
	out, err := uninstallCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("plugin uninstall failed: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "Successfully uninstalled") {
		t.Errorf("expected uninstall success, got:\n%s", out)
	}
}

// TestPluginSubcommand_Validate verifies manifest validation works.
func TestPluginSubcommand_Validate(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	bin := buildPluginBinary(t)
	cwd := t.TempDir()

	// Create a valid manifest.
	raw, _ := json.Marshal(map[string]string{"name": "valid-plugin", "version": "2.0.0"})
	manifestPath := filepath.Join(cwd, "plugin.json")
	if err := os.WriteFile(manifestPath, raw, 0644); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(bin, "plugin", "validate", manifestPath)
	cmd.Dir = cwd
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("plugin validate failed: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "Validation passed") {
		t.Errorf("expected 'Validation passed', got:\n%s", out)
	}
}

// TestPluginSubcommand_UnknownSubcommand verifies an unknown subcommand exits with error.
func TestPluginSubcommand_UnknownSubcommand(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	bin := buildPluginBinary(t)
	cwd := t.TempDir()

	cmd := exec.Command(bin, "plugin", "bogus")
	cmd.Dir = cwd
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("expected non-zero exit for unknown subcommand")
	}
	if !strings.Contains(string(out), "Unknown plugin subcommand") {
		t.Errorf("expected error message about unknown subcommand, got:\n%s", out)
	}
}

// buildPluginBinary compiles the gopher-code binary into a temp directory and
// returns the path. It fails the test if compilation fails.
func buildPluginBinary(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	bin := filepath.Join(tmp, "gopher-code")
	build := exec.Command("go", "build", "-o", bin, ".")
	build.Dir = filepath.Dir(selfPath(t))
	build.Env = append(os.Environ(), "CGO_ENABLED=0")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build failed: %v\n%s", err, out)
	}
	return bin
}
