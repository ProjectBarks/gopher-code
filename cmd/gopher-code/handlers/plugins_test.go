package handlers_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/projectbarks/gopher-code/cmd/gopher-code/handlers"
)

// newTestPluginHandler creates a PluginHandler rooted at a temp dir with captured output.
func newTestPluginHandler(t *testing.T) (*handlers.PluginHandler, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	dir := t.TempDir()
	dataDir := filepath.Join(dir, ".claude", "plugins")
	os.MkdirAll(dataDir, 0755)

	var stdout, stderr bytes.Buffer
	h := &handlers.PluginHandler{
		CWD:     dir,
		Stdout:  &stdout,
		Stderr:  &stderr,
		DataDir: dataDir,
	}
	return h, &stdout, &stderr
}

// writeManifest creates a plugin.json in the given directory.
func writeManifest(t *testing.T, dir string, manifest handlers.PluginManifest) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "plugin.json")
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestList_NoPlugins(t *testing.T) {
	t.Parallel()
	h, stdout, _ := newTestPluginHandler(t)

	err := h.List()
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}

	got := stdout.String()
	want := "No plugins installed. Use `claude plugin install` to install a plugin."
	if !strings.Contains(got, want) {
		t.Errorf("List() output = %q, want it to contain %q", got, want)
	}
}

func TestInstall_LocalDir(t *testing.T) {
	t.Parallel()
	h, stdout, _ := newTestPluginHandler(t)

	// Create a plugin directory with a manifest.
	pluginDir := filepath.Join(h.CWD, "my-plugin")
	writeManifest(t, pluginDir, handlers.PluginManifest{
		Name:    "my-plugin",
		Version: "1.0.0",
	})

	err := h.Install(pluginDir, "user")
	if err != nil {
		t.Fatalf("Install() error: %v", err)
	}

	got := stdout.String()
	if !strings.Contains(got, "Successfully installed plugin: my-plugin") {
		t.Errorf("Install() output = %q, want install confirmation", got)
	}
	if !strings.Contains(got, "scope: user") {
		t.Errorf("Install() output = %q, want scope in output", got)
	}

	// Verify it shows up in list.
	stdout.Reset()
	err = h.List()
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}

	got = stdout.String()
	if !strings.Contains(got, "Installed plugins:") {
		t.Errorf("List() output = %q, want 'Installed plugins:' header", got)
	}
	if !strings.Contains(got, "my-plugin") {
		t.Errorf("List() output = %q, want plugin name", got)
	}
	if !strings.Contains(got, "Version: 1.0.0") {
		t.Errorf("List() output = %q, want version", got)
	}
	if !strings.Contains(got, "enabled") {
		t.Errorf("List() output = %q, want enabled status", got)
	}
}

func TestInstall_DefaultScope(t *testing.T) {
	t.Parallel()
	h, stdout, _ := newTestPluginHandler(t)

	pluginDir := filepath.Join(h.CWD, "default-scope-plugin")
	writeManifest(t, pluginDir, handlers.PluginManifest{
		Name:    "default-scope-plugin",
		Version: "0.1.0",
	})

	// Empty scope should default to "user".
	err := h.Install(pluginDir, "")
	if err != nil {
		t.Fatalf("Install() error: %v", err)
	}

	got := stdout.String()
	if !strings.Contains(got, "scope: user") {
		t.Errorf("Install() output = %q, want default scope 'user'", got)
	}
}

func TestInstall_InvalidScope(t *testing.T) {
	t.Parallel()
	h, _, _ := newTestPluginHandler(t)

	pluginDir := filepath.Join(h.CWD, "bad-scope")
	writeManifest(t, pluginDir, handlers.PluginManifest{Name: "bad-scope"})

	err := h.Install(pluginDir, "bogus")
	if err == nil {
		t.Fatal("Install() expected error for invalid scope")
	}
	if !strings.Contains(err.Error(), "invalid scope") {
		t.Errorf("error = %q, want 'invalid scope'", err.Error())
	}
}

func TestInstall_DuplicateScope(t *testing.T) {
	t.Parallel()
	h, _, _ := newTestPluginHandler(t)

	pluginDir := filepath.Join(h.CWD, "dup-plugin")
	writeManifest(t, pluginDir, handlers.PluginManifest{
		Name:    "dup-plugin",
		Version: "1.0.0",
	})

	err := h.Install(pluginDir, "user")
	if err != nil {
		t.Fatalf("first Install() error: %v", err)
	}

	err = h.Install(pluginDir, "user")
	if err == nil {
		t.Fatal("second Install() expected error for duplicate")
	}
	if !strings.Contains(err.Error(), "already installed") {
		t.Errorf("error = %q, want 'already installed'", err.Error())
	}
}

func TestInstall_MissingManifest(t *testing.T) {
	t.Parallel()
	h, _, _ := newTestPluginHandler(t)

	emptyDir := filepath.Join(h.CWD, "empty-plugin")
	os.MkdirAll(emptyDir, 0755)

	err := h.Install(emptyDir, "user")
	if err == nil {
		t.Fatal("Install() expected error for missing manifest")
	}
	if !strings.Contains(err.Error(), "no plugin.json") {
		t.Errorf("error = %q, want 'no plugin.json'", err.Error())
	}
}

func TestUninstall(t *testing.T) {
	t.Parallel()
	h, stdout, _ := newTestPluginHandler(t)

	// Install first.
	pluginDir := filepath.Join(h.CWD, "rm-plugin")
	writeManifest(t, pluginDir, handlers.PluginManifest{
		Name:    "rm-plugin",
		Version: "1.0.0",
	})

	err := h.Install(pluginDir, "user")
	if err != nil {
		t.Fatalf("Install() error: %v", err)
	}
	stdout.Reset()

	// Uninstall.
	err = h.Uninstall("rm-plugin", "user")
	if err != nil {
		t.Fatalf("Uninstall() error: %v", err)
	}

	got := stdout.String()
	if !strings.Contains(got, "Successfully uninstalled plugin: rm-plugin") {
		t.Errorf("Uninstall() output = %q, want uninstall confirmation", got)
	}

	// Verify it's gone from list.
	stdout.Reset()
	err = h.List()
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}
	got = stdout.String()
	if !strings.Contains(got, "No plugins installed") {
		t.Errorf("List() after uninstall = %q, want 'No plugins installed'", got)
	}
}

func TestUninstall_NotInstalled(t *testing.T) {
	t.Parallel()
	h, _, _ := newTestPluginHandler(t)

	err := h.Uninstall("ghost", "user")
	if err == nil {
		t.Fatal("Uninstall() expected error for nonexistent plugin")
	}
	if !strings.Contains(err.Error(), "not installed") {
		t.Errorf("error = %q, want 'not installed'", err.Error())
	}
}

func TestUninstall_WrongScope(t *testing.T) {
	t.Parallel()
	h, _, _ := newTestPluginHandler(t)

	pluginDir := filepath.Join(h.CWD, "scope-plugin")
	writeManifest(t, pluginDir, handlers.PluginManifest{
		Name:    "scope-plugin",
		Version: "1.0.0",
	})

	err := h.Install(pluginDir, "user")
	if err != nil {
		t.Fatalf("Install() error: %v", err)
	}

	err = h.Uninstall("scope-plugin", "project")
	if err == nil {
		t.Fatal("Uninstall() expected error for wrong scope")
	}
	if !strings.Contains(err.Error(), "not installed at scope project") {
		t.Errorf("error = %q, want 'not installed at scope project'", err.Error())
	}
}

func TestEnableDisable(t *testing.T) {
	t.Parallel()
	h, stdout, _ := newTestPluginHandler(t)

	pluginDir := filepath.Join(h.CWD, "toggle-plugin")
	writeManifest(t, pluginDir, handlers.PluginManifest{
		Name:    "toggle-plugin",
		Version: "1.0.0",
	})

	err := h.Install(pluginDir, "user")
	if err != nil {
		t.Fatalf("Install() error: %v", err)
	}
	stdout.Reset()

	// Disable.
	err = h.Disable("toggle-plugin", "")
	if err != nil {
		t.Fatalf("Disable() error: %v", err)
	}
	got := stdout.String()
	if !strings.Contains(got, "disabled") {
		t.Errorf("Disable() output = %q, want 'disabled'", got)
	}
	stdout.Reset()

	// Verify list shows disabled.
	err = h.List()
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}
	got = stdout.String()
	if !strings.Contains(got, "disabled") {
		t.Errorf("List() output = %q, want 'disabled' status", got)
	}
	stdout.Reset()

	// Re-enable.
	err = h.Enable("toggle-plugin", "")
	if err != nil {
		t.Fatalf("Enable() error: %v", err)
	}
	got = stdout.String()
	if !strings.Contains(got, "enabled") {
		t.Errorf("Enable() output = %q, want 'enabled'", got)
	}
}

func TestEnableDisable_NotInstalled(t *testing.T) {
	t.Parallel()
	h, _, _ := newTestPluginHandler(t)

	err := h.Enable("ghost", "")
	if err == nil {
		t.Fatal("Enable() expected error for nonexistent plugin")
	}
	if !strings.Contains(err.Error(), "not installed") {
		t.Errorf("error = %q, want 'not installed'", err.Error())
	}

	err = h.Disable("ghost", "")
	if err == nil {
		t.Fatal("Disable() expected error for nonexistent plugin")
	}
}

func TestValidate_ValidManifest(t *testing.T) {
	t.Parallel()
	h, stdout, _ := newTestPluginHandler(t)

	pluginDir := filepath.Join(h.CWD, "valid-plugin")
	writeManifest(t, pluginDir, handlers.PluginManifest{
		Name:    "valid-plugin",
		Version: "1.0.0",
	})

	result, err := h.Validate(pluginDir)
	if err != nil {
		t.Fatalf("Validate() error: %v", err)
	}

	if !result.Success {
		t.Errorf("Validate() success = false, want true")
	}
	if len(result.Errors) > 0 {
		t.Errorf("Validate() errors = %v, want none", result.Errors)
	}

	got := stdout.String()
	if !strings.Contains(got, "Validating plugin manifest:") {
		t.Errorf("Validate() output = %q, want validating message", got)
	}
	if !strings.Contains(got, "Validation passed") {
		t.Errorf("Validate() output = %q, want 'Validation passed'", got)
	}
}

func TestValidate_MissingName(t *testing.T) {
	t.Parallel()
	h, stdout, _ := newTestPluginHandler(t)

	pluginDir := filepath.Join(h.CWD, "no-name")
	writeManifest(t, pluginDir, handlers.PluginManifest{
		Version: "1.0.0",
	})

	result, err := h.Validate(pluginDir)
	if err != nil {
		t.Fatalf("Validate() error: %v", err)
	}

	if result.Success {
		t.Errorf("Validate() success = true, want false for missing name")
	}
	if len(result.Errors) != 1 {
		t.Errorf("Validate() errors = %d, want 1", len(result.Errors))
	}

	got := stdout.String()
	if !strings.Contains(got, "Validation failed") {
		t.Errorf("Validate() output = %q, want 'Validation failed'", got)
	}
}

func TestValidate_InvalidJSON(t *testing.T) {
	t.Parallel()
	h, stdout, _ := newTestPluginHandler(t)

	pluginDir := filepath.Join(h.CWD, "bad-json")
	os.MkdirAll(pluginDir, 0755)
	os.WriteFile(filepath.Join(pluginDir, "plugin.json"), []byte("not json{"), 0644)

	result, err := h.Validate(pluginDir)
	if err != nil {
		t.Fatalf("Validate() error: %v", err)
	}

	if result.Success {
		t.Errorf("Validate() success = true, want false for invalid JSON")
	}

	got := stdout.String()
	if !strings.Contains(got, "invalid JSON") {
		t.Errorf("Validate() output = %q, want 'invalid JSON' error", got)
	}
}

func TestValidate_NonexistentPath(t *testing.T) {
	t.Parallel()
	h, _, _ := newTestPluginHandler(t)

	_, err := h.Validate("/nonexistent/path")
	if err == nil {
		t.Fatal("Validate() expected error for nonexistent path")
	}
	if !strings.Contains(err.Error(), "cannot access path") {
		t.Errorf("error = %q, want 'cannot access path'", err.Error())
	}
}

func TestValidate_MissingVersion(t *testing.T) {
	t.Parallel()
	h, stdout, _ := newTestPluginHandler(t)

	pluginDir := filepath.Join(h.CWD, "no-version")
	writeManifest(t, pluginDir, handlers.PluginManifest{
		Name: "no-version",
	})

	result, err := h.Validate(pluginDir)
	if err != nil {
		t.Fatalf("Validate() error: %v", err)
	}

	if !result.Success {
		t.Errorf("Validate() success = false, want true (version is only a warning)")
	}
	if len(result.Warnings) != 1 {
		t.Errorf("Validate() warnings = %d, want 1", len(result.Warnings))
	}

	got := stdout.String()
	if !strings.Contains(got, "Validation passed with warnings") {
		t.Errorf("Validate() output = %q, want 'Validation passed with warnings'", got)
	}
}

func TestValidate_DirectFile(t *testing.T) {
	t.Parallel()
	h, stdout, _ := newTestPluginHandler(t)

	pluginDir := filepath.Join(h.CWD, "file-plugin")
	manifestPath := writeManifest(t, pluginDir, handlers.PluginManifest{
		Name:    "file-plugin",
		Version: "2.0.0",
	})

	result, err := h.Validate(manifestPath)
	if err != nil {
		t.Fatalf("Validate() error: %v", err)
	}

	if !result.Success {
		t.Errorf("Validate() success = false, want true")
	}

	got := stdout.String()
	if !strings.Contains(got, "Validation passed") {
		t.Errorf("Validate() output = %q, want 'Validation passed'", got)
	}
}

func TestList_MultiplePlugins(t *testing.T) {
	t.Parallel()
	h, stdout, _ := newTestPluginHandler(t)

	// Install two plugins.
	for _, name := range []string{"alpha-plugin", "beta-plugin"} {
		dir := filepath.Join(h.CWD, name)
		writeManifest(t, dir, handlers.PluginManifest{
			Name:    name,
			Version: "1.0.0",
		})
		if err := h.Install(dir, "user"); err != nil {
			t.Fatalf("Install(%s) error: %v", name, err)
		}
	}
	stdout.Reset()

	err := h.List()
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}

	got := stdout.String()
	// Verify sorted order.
	alphaIdx := strings.Index(got, "alpha-plugin")
	betaIdx := strings.Index(got, "beta-plugin")
	if alphaIdx < 0 || betaIdx < 0 || alphaIdx >= betaIdx {
		t.Errorf("expected alpha-plugin before beta-plugin in output:\n%s", got)
	}
}

func TestInstall_ClaudePluginSubdir(t *testing.T) {
	t.Parallel()
	h, stdout, _ := newTestPluginHandler(t)

	// Create a plugin with manifest in .claude-plugin subdirectory.
	pluginDir := filepath.Join(h.CWD, "subdir-plugin")
	subDir := filepath.Join(pluginDir, ".claude-plugin")
	writeManifest(t, subDir, handlers.PluginManifest{
		Name:    "subdir-plugin",
		Version: "1.0.0",
	})

	err := h.Install(pluginDir, "user")
	if err != nil {
		t.Fatalf("Install() error: %v", err)
	}

	got := stdout.String()
	if !strings.Contains(got, "Successfully installed plugin: subdir-plugin") {
		t.Errorf("Install() output = %q, want install confirmation", got)
	}
}
