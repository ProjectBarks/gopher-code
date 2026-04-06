// Package handlers implements CLI subcommand handlers for gopher-code.
// Source: src/cli/handlers/plugins.ts
package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// PluginScope represents the scope at which a plugin is installed.
type PluginScope string

const (
	ScopeUser    PluginScope = "user"
	ScopeProject PluginScope = "project"
	ScopeLocal   PluginScope = "local"
	ScopeSession PluginScope = "session"
)

// ValidInstallableScopes lists the scopes that support install/uninstall.
var ValidInstallableScopes = []PluginScope{ScopeUser, ScopeProject, ScopeLocal}

// ValidUpdateScopes lists the scopes that support update.
var ValidUpdateScopes = []PluginScope{ScopeUser, ScopeProject, ScopeLocal}

// SourceKind describes how a marketplace or plugin was sourced.
type SourceKind string

const (
	SourceGitHub    SourceKind = "github"
	SourceGit       SourceKind = "git"
	SourceURL       SourceKind = "url"
	SourceDirectory SourceKind = "directory"
	SourceFile      SourceKind = "file"
)

// PluginManifest is the minimal manifest shape for a plugin.
// Full schema validation is stubbed until the plugin system is built.
type PluginManifest struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Description string `json:"description,omitempty"`
}

// PluginInstallation records one installed plugin instance.
type PluginInstallation struct {
	Version     string      `json:"version"`
	Scope       PluginScope `json:"scope"`
	InstallPath string      `json:"installPath"`
	InstalledAt string      `json:"installedAt,omitempty"`
	LastUpdated string      `json:"lastUpdated,omitempty"`
	Enabled     bool        `json:"enabled"`
}

// InstalledPluginsData is the on-disk format for installed plugins bookkeeping.
type InstalledPluginsData struct {
	Plugins map[string][]PluginInstallation `json:"plugins"`
}

// PluginHandler holds state for the `claude plugin` subcommand family.
type PluginHandler struct {
	CWD    string
	Stdout io.Writer
	Stderr io.Writer
	// DataDir overrides the directory where plugin data is stored.
	// If empty, defaults to CWD/.claude/plugins.
	DataDir string
}

// NewPluginHandler creates a handler targeting the given working directory.
func NewPluginHandler(cwd string) *PluginHandler {
	return &PluginHandler{
		CWD:    cwd,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}
}

// pointer and cross are platform-independent glyph helpers matching figures.
const (
	glyphPointer = "❯"
	glyphCross   = "✗"
	glyphTick    = "✓"
)

// dataDir returns the directory for plugin data storage.
func (h *PluginHandler) dataDir() string {
	if h.DataDir != "" {
		return h.DataDir
	}
	return filepath.Join(h.CWD, ".claude", "plugins")
}

// installedPluginsPath returns the path to the installed plugins JSON file.
func (h *PluginHandler) installedPluginsPath() string {
	return filepath.Join(h.dataDir(), "installed.json")
}

// loadInstalled reads the installed plugins data from disk.
func (h *PluginHandler) loadInstalled() InstalledPluginsData {
	data := InstalledPluginsData{Plugins: make(map[string][]PluginInstallation)}
	raw, err := os.ReadFile(h.installedPluginsPath())
	if err != nil {
		return data
	}
	_ = json.Unmarshal(raw, &data)
	if data.Plugins == nil {
		data.Plugins = make(map[string][]PluginInstallation)
	}
	return data
}

// saveInstalled writes the installed plugins data to disk.
func (h *PluginHandler) saveInstalled(data InstalledPluginsData) error {
	dir := filepath.Dir(h.installedPluginsPath())
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating plugin data directory: %w", err)
	}
	raw, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling plugin data: %w", err)
	}
	return os.WriteFile(h.installedPluginsPath(), raw, 0644)
}

// List prints all installed plugins with their status.
// Source: src/cli/handlers/plugins.ts — pluginListHandler
func (h *PluginHandler) List() error {
	data := h.loadInstalled()

	ids := make([]string, 0, len(data.Plugins))
	for id := range data.Plugins {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	if len(ids) == 0 {
		fmt.Fprintln(h.Stdout, "No plugins installed. Use `claude plugin install` to install a plugin.")
		return nil
	}

	fmt.Fprintf(h.Stdout, "Installed plugins:\n\n")

	for _, pluginID := range ids {
		installations := data.Plugins[pluginID]
		for _, inst := range installations {
			status := glyphTick + " enabled"
			if !inst.Enabled {
				status = glyphCross + " disabled"
			}
			version := inst.Version
			if version == "" {
				version = "unknown"
			}

			fmt.Fprintf(h.Stdout, "  %s %s\n", glyphPointer, pluginID)
			fmt.Fprintf(h.Stdout, "    Version: %s\n", version)
			fmt.Fprintf(h.Stdout, "    Scope: %s\n", inst.Scope)
			fmt.Fprintf(h.Stdout, "    Status: %s\n", status)
			if inst.InstallPath != "" {
				fmt.Fprintf(h.Stdout, "    Path: %s\n", inst.InstallPath)
			}
			fmt.Fprintln(h.Stdout)
		}
	}

	return nil
}

// parseScope validates and returns a PluginScope from a string.
func parseScope(s string) (PluginScope, error) {
	switch PluginScope(s) {
	case ScopeUser, ScopeProject, ScopeLocal:
		return PluginScope(s), nil
	default:
		return "", fmt.Errorf("invalid scope: %s. Must be one of: %s",
			s, strings.Join(scopeStrings(ValidInstallableScopes), ", "))
	}
}

func scopeStrings(scopes []PluginScope) []string {
	out := make([]string, len(scopes))
	for i, s := range scopes {
		out[i] = string(s)
	}
	return out
}

// Install installs a plugin from the given source identifier.
// Source: src/cli/handlers/plugins.ts — pluginInstallHandler
//
// The source can be a local directory path, a git URL, or a marketplace
// plugin ID (marketplace/name). Full marketplace resolution is stubbed.
func (h *PluginHandler) Install(source, scopeStr string) error {
	if scopeStr == "" {
		scopeStr = "user"
	}
	scope, err := parseScope(scopeStr)
	if err != nil {
		return err
	}

	// Determine source kind and resolve manifest.
	kind, resolvedPath, err := h.resolveSource(source)
	if err != nil {
		return fmt.Errorf("resolving plugin source: %w", err)
	}

	var manifest PluginManifest

	switch kind {
	case SourceDirectory:
		manifest, err = h.loadManifestFromDir(resolvedPath)
		if err != nil {
			return fmt.Errorf("loading plugin manifest: %w", err)
		}
	case SourceFile:
		manifest, err = h.loadManifestFromFile(resolvedPath)
		if err != nil {
			return fmt.Errorf("loading plugin manifest: %w", err)
		}
	default:
		// GitHub, Git, URL sources are stubbed — they would clone/fetch the
		// plugin and then read the manifest from the result.
		return fmt.Errorf("%s plugin sources are not yet supported", kind)
	}

	if manifest.Name == "" {
		return fmt.Errorf("plugin manifest missing required field: name")
	}

	pluginID := manifest.Name

	data := h.loadInstalled()

	// Check if already installed at this scope.
	for _, inst := range data.Plugins[pluginID] {
		if inst.Scope == scope {
			return fmt.Errorf("plugin %s is already installed at scope %s", pluginID, scope)
		}
	}

	installation := PluginInstallation{
		Version:     manifest.Version,
		Scope:       scope,
		InstallPath: resolvedPath,
		InstalledAt: time.Now().UTC().Format(time.RFC3339),
		Enabled:     true,
	}

	data.Plugins[pluginID] = append(data.Plugins[pluginID], installation)
	if err := h.saveInstalled(data); err != nil {
		return err
	}

	fmt.Fprintf(h.Stdout, "%s Successfully installed plugin: %s (scope: %s)\n", glyphTick, pluginID, scope)
	return nil
}

// Uninstall removes a plugin by name and optional scope.
// Source: src/cli/handlers/plugins.ts — pluginUninstallHandler
func (h *PluginHandler) Uninstall(name, scopeStr string) error {
	if scopeStr == "" {
		scopeStr = "user"
	}
	scope, err := parseScope(scopeStr)
	if err != nil {
		return err
	}

	data := h.loadInstalled()

	installations, ok := data.Plugins[name]
	if !ok || len(installations) == 0 {
		return fmt.Errorf("plugin %s is not installed", name)
	}

	// Filter out the matching scope.
	var remaining []PluginInstallation
	found := false
	for _, inst := range installations {
		if inst.Scope == scope {
			found = true
			continue
		}
		remaining = append(remaining, inst)
	}

	if !found {
		return fmt.Errorf("plugin %s is not installed at scope %s", name, scope)
	}

	if len(remaining) == 0 {
		delete(data.Plugins, name)
	} else {
		data.Plugins[name] = remaining
	}

	if err := h.saveInstalled(data); err != nil {
		return err
	}

	fmt.Fprintf(h.Stdout, "%s Successfully uninstalled plugin: %s (scope: %s)\n", glyphTick, name, scope)
	return nil
}

// Enable enables an installed plugin.
// Source: src/cli/handlers/plugins.ts — pluginEnableHandler
func (h *PluginHandler) Enable(name, scopeStr string) error {
	return h.setEnabled(name, scopeStr, true)
}

// Disable disables an installed plugin.
// Source: src/cli/handlers/plugins.ts — pluginDisableHandler
func (h *PluginHandler) Disable(name, scopeStr string) error {
	return h.setEnabled(name, scopeStr, false)
}

// setEnabled toggles the enabled state of a plugin.
func (h *PluginHandler) setEnabled(name, scopeStr string, enabled bool) error {
	data := h.loadInstalled()

	installations, ok := data.Plugins[name]
	if !ok || len(installations) == 0 {
		return fmt.Errorf("plugin %s is not installed", name)
	}

	// If scope is specified, toggle only that scope. Otherwise toggle all.
	var scope PluginScope
	if scopeStr != "" {
		var err error
		scope, err = parseScope(scopeStr)
		if err != nil {
			return err
		}
	}

	found := false
	for i := range installations {
		if scopeStr == "" || installations[i].Scope == scope {
			installations[i].Enabled = enabled
			found = true
		}
	}

	if scopeStr != "" && !found {
		return fmt.Errorf("plugin %s is not installed at scope %s", name, scope)
	}

	data.Plugins[name] = installations
	if err := h.saveInstalled(data); err != nil {
		return err
	}

	action := "enabled"
	if !enabled {
		action = "disabled"
	}
	fmt.Fprintf(h.Stdout, "%s Plugin %s %s\n", glyphTick, name, action)
	return nil
}

// ValidationIssue represents a single validation error or warning.
type ValidationIssue struct {
	Path    string `json:"path"`
	Message string `json:"message"`
}

// ValidationResult holds the outcome of manifest validation.
type ValidationResult struct {
	FilePath string            `json:"filePath"`
	FileType string            `json:"fileType"`
	Success  bool              `json:"success"`
	Errors   []ValidationIssue `json:"errors,omitempty"`
	Warnings []ValidationIssue `json:"warnings,omitempty"`
}

// Validate validates a plugin manifest at the given path.
// Source: src/cli/handlers/plugins.ts — pluginValidateHandler
func (h *PluginHandler) Validate(manifestPath string) (*ValidationResult, error) {
	info, err := os.Stat(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("cannot access path: %w", err)
	}

	var filePath string
	if info.IsDir() {
		// Look for plugin.json in a .claude-plugin subdirectory or directly.
		candidates := []string{
			filepath.Join(manifestPath, ".claude-plugin", "plugin.json"),
			filepath.Join(manifestPath, "plugin.json"),
		}
		for _, c := range candidates {
			if _, err := os.Stat(c); err == nil {
				filePath = c
				break
			}
		}
		if filePath == "" {
			return nil, fmt.Errorf("no plugin.json found in %s", manifestPath)
		}
	} else {
		filePath = manifestPath
	}

	raw, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("reading manifest: %w", err)
	}

	result := &ValidationResult{
		FilePath: filePath,
		FileType: "plugin",
		Success:  true,
	}

	var manifest PluginManifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		result.Success = false
		result.Errors = append(result.Errors, ValidationIssue{
			Path:    filePath,
			Message: fmt.Sprintf("invalid JSON: %v", err),
		})
	}

	// Field-level checks (only if JSON parsed successfully).
	if result.Success {
		if manifest.Name == "" {
			result.Success = false
			result.Errors = append(result.Errors, ValidationIssue{
				Path:    "name",
				Message: "required field 'name' is missing",
			})
		}
		if manifest.Version == "" {
			result.Warnings = append(result.Warnings, ValidationIssue{
				Path:    "version",
				Message: "field 'version' is missing (recommended)",
			})
		}
	}

	fmt.Fprintf(h.Stdout, "Validating %s manifest: %s\n", result.FileType, result.FilePath)

	if len(result.Errors) > 0 {
		fmt.Fprintf(h.Stdout, "\n%s Found %d error(s):\n\n", glyphCross, len(result.Errors))
		for _, e := range result.Errors {
			fmt.Fprintf(h.Stdout, "  %s %s: %s\n", glyphPointer, e.Path, e.Message)
		}
		fmt.Fprintln(h.Stdout)
	}

	if len(result.Warnings) > 0 {
		fmt.Fprintf(h.Stdout, "\nFound %d warning(s):\n\n", len(result.Warnings))
		for _, w := range result.Warnings {
			fmt.Fprintf(h.Stdout, "  %s %s: %s\n", glyphPointer, w.Path, w.Message)
		}
		fmt.Fprintln(h.Stdout)
	}

	if result.Success {
		if len(result.Warnings) > 0 {
			fmt.Fprintf(h.Stdout, "%s Validation passed with warnings\n", glyphTick)
		} else {
			fmt.Fprintf(h.Stdout, "%s Validation passed\n", glyphTick)
		}
	} else {
		fmt.Fprintf(h.Stdout, "%s Validation failed\n", glyphCross)
	}

	return result, nil
}

// resolveSource determines the SourceKind and resolved path for a plugin source.
func (h *PluginHandler) resolveSource(source string) (SourceKind, string, error) {
	// Local directory or file.
	if strings.HasPrefix(source, "/") || strings.HasPrefix(source, "./") || strings.HasPrefix(source, "../") || strings.HasPrefix(source, "~") {
		abs, err := filepath.Abs(source)
		if err != nil {
			return "", "", err
		}
		info, err := os.Stat(abs)
		if err != nil {
			return "", "", fmt.Errorf("path does not exist: %s", abs)
		}
		if info.IsDir() {
			return SourceDirectory, abs, nil
		}
		return SourceFile, abs, nil
	}

	// Git URLs.
	if strings.HasPrefix(source, "https://") || strings.HasPrefix(source, "git@") || strings.HasPrefix(source, "ssh://") {
		if strings.Contains(source, "github.com") {
			return SourceGitHub, source, nil
		}
		return SourceGit, source, nil
	}

	// GitHub shorthand: owner/repo.
	if strings.Count(source, "/") == 1 && !strings.Contains(source, " ") {
		return SourceGitHub, source, nil
	}

	// Marketplace plugin ID: marketplace/name or bare name.
	// Stubbed — would look up the marketplace index.
	return "", "", fmt.Errorf("cannot resolve plugin source: %s (marketplace lookup not yet implemented)", source)
}

// loadManifestFromDir reads a plugin.json from a directory.
func (h *PluginHandler) loadManifestFromDir(dir string) (PluginManifest, error) {
	candidates := []string{
		filepath.Join(dir, ".claude-plugin", "plugin.json"),
		filepath.Join(dir, "plugin.json"),
	}
	for _, c := range candidates {
		m, err := h.loadManifestFromFile(c)
		if err == nil {
			return m, nil
		}
	}
	return PluginManifest{}, fmt.Errorf("no plugin.json found in %s", dir)
}

// loadManifestFromFile reads and parses a single plugin.json file.
func (h *PluginHandler) loadManifestFromFile(path string) (PluginManifest, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return PluginManifest{}, err
	}
	var m PluginManifest
	if err := json.Unmarshal(raw, &m); err != nil {
		return PluginManifest{}, fmt.Errorf("invalid plugin manifest at %s: %w", path, err)
	}
	return m, nil
}
