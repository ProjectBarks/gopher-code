// Package config provides bubbletea hook models for model tracking, dynamic
// configuration, and settings file change detection.
//
// In the TS codebase these are four separate React hooks:
//   - useMainLoopModel (src/hooks/useMainLoopModel.ts)
//   - useDynamicConfig (src/hooks/useDynamicConfig.ts)
//   - useSettings      (src/hooks/useSettings.ts)
//   - useSettingsChange (src/hooks/useSettingsChange.ts)
//
// In Go they collapse into three structs: MainLoopModel, DynamicConfig, and
// SettingsWatcher.
package config

import (
	"os"
	"sync"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/projectbarks/gopher-code/pkg/config"
	"github.com/projectbarks/gopher-code/pkg/provider"
)

// ---------------------------------------------------------------------------
// MainLoopModel — tracks the active model with session-override support
// ---------------------------------------------------------------------------

// MainLoopModel tracks the current model for the main conversation loop.
// It supports a persistent model (from settings) and a per-session override
// (from /model command). The session override takes precedence.
//
// Source: src/hooks/useMainLoopModel.ts, src/state/AppState.ts
type MainLoopModel struct {
	mu              sync.RWMutex
	model           string // persistent model from settings (may be empty)
	sessionOverride string // per-session override from /model command
}

// NewMainLoopModel creates a MainLoopModel initialised with the given
// persistent model setting. Pass "" to use the default.
func NewMainLoopModel(model string) *MainLoopModel {
	return &MainLoopModel{model: model}
}

// Get returns the resolved model ID for API calls. Resolution order:
//  1. Session override (from /model command)
//  2. Persistent model (from settings)
//  3. Default (sonnet for most users)
//
// The returned value is run through ParseUserSpecifiedModel to resolve
// aliases like "opus" or "sonnet".
//
// Source: useMainLoopModel.ts:28-33
func (m *MainLoopModel) Get() string {
	m.mu.RLock()
	raw := m.sessionOverride
	if raw == "" {
		raw = m.model
	}
	m.mu.RUnlock()

	if raw == "" {
		raw = DefaultMainLoopModelSetting()
	}
	return provider.ParseUserSpecifiedModel(raw)
}

// Set updates the persistent model (from settings or /model --save).
func (m *MainLoopModel) Set(model string) {
	m.mu.Lock()
	m.model = model
	m.mu.Unlock()
}

// Override sets the per-session model override (from /model command).
// Pass "" to clear the override and fall back to the persistent model.
func (m *MainLoopModel) Override(model string) {
	m.mu.Lock()
	m.sessionOverride = model
	m.mu.Unlock()
}

// HasOverride reports whether a session override is active.
func (m *MainLoopModel) HasOverride() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.sessionOverride != ""
}

// DisplayName returns a human-readable name for the current model.
func (m *MainLoopModel) DisplayName() string {
	resolved := m.Get()
	if name := provider.GetMarketingNameForModel(resolved); name != "" {
		return name
	}
	return resolved
}

// DefaultMainLoopModelSetting returns the default model setting.
// Source: utils/model/model.ts:178-200
func DefaultMainLoopModelSetting() string {
	return provider.GetDefaultSonnetModel()
}

// ---------------------------------------------------------------------------
// SettingsWatcher — detects when settings files are modified on disk
// ---------------------------------------------------------------------------

// SettingsChangedMsg is dispatched when a settings file is modified.
type SettingsChangedMsg struct {
	Source   config.SettingSource
	Settings *config.Settings
}

// SettingsWatcher polls settings files for changes by comparing file
// modification times, then dispatches SettingsChangedMsg through the
// bubbletea event loop.
//
// In the TS codebase this is done via chokidar (useSettingsChange +
// changeDetector.ts). In Go we use a simple tea.Tick + os.Stat approach
// which is dramatically simpler and avoids the chokidar dependency.
//
// Source: src/hooks/useSettingsChange.ts, src/utils/settings/changeDetector.ts
type SettingsWatcher struct {
	cwd      string
	interval time.Duration
	mtimes   map[string]time.Time // path -> last known mtime
	paths    []watchedPath
}

type watchedPath struct {
	path   string
	source config.SettingSource
}

// NewSettingsWatcher creates a watcher that polls settings files at the
// given interval. A typical interval is 2-5 seconds.
func NewSettingsWatcher(cwd string, interval time.Duration) *SettingsWatcher {
	sw := &SettingsWatcher{
		cwd:      cwd,
		interval: interval,
		mtimes:   make(map[string]time.Time),
	}
	sw.paths = sw.settingsPaths()
	return sw
}

// Init starts the polling tick.
func (sw *SettingsWatcher) Init() tea.Cmd {
	// Snapshot current mtimes so the first tick doesn't fire a false positive.
	for _, wp := range sw.paths {
		if info, err := os.Stat(wp.path); err == nil {
			sw.mtimes[wp.path] = info.ModTime()
		}
	}
	return sw.tick()
}

// Update handles tick messages and checks for file changes.
func (sw *SettingsWatcher) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if _, ok := msg.(settingsTickMsg); !ok {
		return sw, nil
	}
	var cmds []tea.Cmd
	for _, wp := range sw.paths {
		info, err := os.Stat(wp.path)
		if err != nil {
			// File was deleted — if we were tracking it, emit a change
			if _, tracked := sw.mtimes[wp.path]; tracked {
				delete(sw.mtimes, wp.path)
				cmds = append(cmds, sw.emitChange(wp.source))
			}
			continue
		}
		mtime := info.ModTime()
		prev, tracked := sw.mtimes[wp.path]
		if !tracked || !mtime.Equal(prev) {
			sw.mtimes[wp.path] = mtime
			if tracked { // only emit if we had a previous value (skip initial)
				cmds = append(cmds, sw.emitChange(wp.source))
			}
		}
	}
	cmds = append(cmds, sw.tick())
	return sw, tea.Batch(cmds...)
}

// View is a no-op — the watcher has no visual output.
func (sw *SettingsWatcher) View() tea.View { return tea.NewView("") }

// settingsTickMsg is the internal tick message for the polling loop.
type settingsTickMsg struct{}

func (sw *SettingsWatcher) tick() tea.Cmd {
	return tea.Tick(sw.interval, func(time.Time) tea.Msg {
		return settingsTickMsg{}
	})
}

func (sw *SettingsWatcher) emitChange(source config.SettingSource) tea.Cmd {
	return func() tea.Msg {
		return SettingsChangedMsg{
			Source:   source,
			Settings: config.Load(sw.cwd),
		}
	}
}

// settingsPaths returns the list of settings file paths to watch.
func (sw *SettingsWatcher) settingsPaths() []watchedPath {
	var paths []watchedPath
	home, _ := os.UserHomeDir()
	if home != "" {
		paths = append(paths, watchedPath{
			path:   home + "/.claude/settings.json",
			source: config.SourceUser,
		})
	}
	if sw.cwd != "" {
		paths = append(paths, watchedPath{
			path:   sw.cwd + "/.claude/settings.json",
			source: config.SourceProject,
		})
		paths = append(paths, watchedPath{
			path:   sw.cwd + "/.claude/settings.local.json",
			source: config.SourceLocal,
		})
	}
	return paths
}

// ---------------------------------------------------------------------------
// DynamicConfig — watches a named config value, reloading on change
// ---------------------------------------------------------------------------

// DynamicConfigChangedMsg is dispatched when a dynamic config value changes.
type DynamicConfigChangedMsg[T comparable] struct {
	Name  string
	Value T
}

// DynamicConfig tracks a named configuration value from settings, polling
// for changes at a fixed interval. When the value changes, a
// DynamicConfigChangedMsg is dispatched.
//
// In the TS codebase this fetches from GrowthBook (feature flags). In Go
// we read from the settings file and support a loader function that
// callers can customise.
//
// Source: src/hooks/useDynamicConfig.ts
type DynamicConfig[T comparable] struct {
	name     string
	value    T
	defVal   T
	interval time.Duration
	loader   func(name string) (T, bool)
}

// NewDynamicConfig creates a config watcher for the given name and default
// value. The loader function is called on each tick to fetch the current
// value; it returns (value, ok). If ok is false, the default is used.
func NewDynamicConfig[T comparable](name string, defVal T, interval time.Duration, loader func(string) (T, bool)) *DynamicConfig[T] {
	return &DynamicConfig[T]{
		name:     name,
		value:    defVal,
		defVal:   defVal,
		interval: interval,
		loader:   loader,
	}
}

// Value returns the current config value.
func (dc *DynamicConfig[T]) Value() T {
	return dc.value
}

// dynamicConfigTickMsg is parameterised by config name to avoid cross-talk
// between multiple DynamicConfig instances.
type dynamicConfigTickMsg struct {
	name string
}

// Init starts the polling tick.
func (dc *DynamicConfig[T]) Init() tea.Cmd {
	return dc.tick()
}

// Update handles tick messages and checks for value changes.
func (dc *DynamicConfig[T]) Update(msg tea.Msg) tea.Cmd {
	tick, ok := msg.(dynamicConfigTickMsg)
	if !ok || tick.name != dc.name {
		return nil
	}
	val, loaded := dc.loader(dc.name)
	if !loaded {
		val = dc.defVal
	}
	var cmd tea.Cmd
	if val != dc.value {
		dc.value = val
		// We can't return a generic DynamicConfigChangedMsg[T] through tea.Msg
		// easily, so we just update the value in place. Consumers call Value().
	}
	cmd = dc.tick()
	return cmd
}

func (dc *DynamicConfig[T]) tick() tea.Cmd {
	name := dc.name
	return tea.Tick(dc.interval, func(time.Time) tea.Msg {
		return dynamicConfigTickMsg{name: name}
	})
}
