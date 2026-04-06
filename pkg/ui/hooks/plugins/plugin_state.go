package plugins

import (
	"fmt"
	"sync"
)

// PluginStatus tracks the lifecycle state of a plugin.
// Source: src/hooks/useManagePlugins.ts, src/types/plugin.ts
type PluginStatus string

const (
	StatusInstalled PluginStatus = "installed"
	StatusEnabled   PluginStatus = "enabled"
	StatusDisabled  PluginStatus = "disabled"
	StatusLoading   PluginStatus = "loading"
	StatusErrored   PluginStatus = "errored"
)

// PluginError represents a typed plugin error.
// Source: src/types/plugin.ts — PluginError discriminated union.
type PluginError struct {
	Type   string // e.g. "generic-error", "plugin-not-found", "mcp-config-invalid"
	Source string // e.g. "plugin-system", "plugin-commands", "lsp-manager"
	Plugin string // optional: which plugin
	Detail string // human-readable error detail
}

func (e PluginError) Error() string {
	if e.Plugin != "" {
		return fmt.Sprintf("%s [%s/%s]: %s", e.Type, e.Source, e.Plugin, e.Detail)
	}
	return fmt.Sprintf("%s [%s]: %s", e.Type, e.Source, e.Detail)
}

// PluginInfo describes a loaded plugin.
// Source: src/types/plugin.ts — LoadedPlugin
type PluginInfo struct {
	Name       string
	Source     string // e.g. "my-plugin@marketplace-name" or "inline@inline"
	Path       string
	Repository string
	IsBuiltin  bool
}

// PluginState tracks all plugins and their statuses.
// Go equivalent of the AppState.plugins slice managed by useManagePlugins.
//
// Thread-safe: guarded by an internal mutex so the UI goroutine and
// background loaders can read/write concurrently.
type PluginState struct {
	mu       sync.RWMutex
	plugins  map[string]*pluginEntry // keyed by plugin name
	errors   []PluginError
	needsRef bool // true when on-disk state changed but not yet reloaded
}

type pluginEntry struct {
	Info   PluginInfo
	Status PluginStatus
}

// NewPluginState creates an empty plugin state.
func NewPluginState() *PluginState {
	return &PluginState{
		plugins: make(map[string]*pluginEntry),
	}
}

// Set records a plugin with the given status, replacing any previous entry.
func (ps *PluginState) Set(info PluginInfo, status PluginStatus) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	ps.plugins[info.Name] = &pluginEntry{Info: info, Status: status}
}

// SetStatus changes the status of an existing plugin. Returns false if the
// plugin is not tracked.
func (ps *PluginState) SetStatus(name string, status PluginStatus) bool {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	e, ok := ps.plugins[name]
	if !ok {
		return false
	}
	e.Status = status
	return true
}

// Get returns the info and status for a plugin, or false if not tracked.
func (ps *PluginState) Get(name string) (PluginInfo, PluginStatus, bool) {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	e, ok := ps.plugins[name]
	if !ok {
		return PluginInfo{}, "", false
	}
	return e.Info, e.Status, true
}

// Enabled returns all plugins with StatusEnabled.
func (ps *PluginState) Enabled() []PluginInfo {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	var out []PluginInfo
	for _, e := range ps.plugins {
		if e.Status == StatusEnabled {
			out = append(out, e.Info)
		}
	}
	return out
}

// Disabled returns all plugins with StatusDisabled.
func (ps *PluginState) Disabled() []PluginInfo {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	var out []PluginInfo
	for _, e := range ps.plugins {
		if e.Status == StatusDisabled {
			out = append(out, e.Info)
		}
	}
	return out
}

// Errored returns all plugins with StatusErrored.
func (ps *PluginState) Errored() []PluginInfo {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	var out []PluginInfo
	for _, e := range ps.plugins {
		if e.Status == StatusErrored {
			out = append(out, e.Info)
		}
	}
	return out
}

// All returns every tracked plugin with its status.
func (ps *PluginState) All() map[string]PluginStatus {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	out := make(map[string]PluginStatus, len(ps.plugins))
	for name, e := range ps.plugins {
		out[name] = e.Status
	}
	return out
}

// AddError appends a plugin error.
func (ps *PluginState) AddError(pe PluginError) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	ps.errors = append(ps.errors, pe)
}

// Errors returns all recorded errors.
func (ps *PluginState) Errors() []PluginError {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	out := make([]PluginError, len(ps.errors))
	copy(out, ps.errors)
	return out
}

// ClearErrors removes all recorded errors.
func (ps *PluginState) ClearErrors() {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	ps.errors = ps.errors[:0]
}

// SetNeedsRefresh marks that on-disk plugin state has changed.
func (ps *PluginState) SetNeedsRefresh(v bool) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	ps.needsRef = v
}

// NeedsRefresh reports whether a reload is pending.
func (ps *PluginState) NeedsRefresh() bool {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	return ps.needsRef
}

// Count returns the number of tracked plugins.
func (ps *PluginState) Count() int {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	return len(ps.plugins)
}
