package hooks

import "sync"

// RegisteredHookCallback represents a single hook callback within a matcher.
// This is the Go equivalent of HookCallback in TS (types/hooks.ts).
// The actual callback is stored as an interface{} so callers can use their
// own function signatures; the registry only tracks registration and lookup.
type RegisteredHookCallback struct {
	Matcher    string      `json:"matcher,omitempty"`
	PluginName string      `json:"plugin_name,omitempty"`
	PluginRoot string      `json:"plugin_root,omitempty"` // non-empty for plugin hooks
	Callback   interface{} `json:"-"`                     // opaque callback
	Timeout    int         `json:"timeout,omitempty"`     // seconds
	Internal   bool        `json:"internal,omitempty"`    // exclude from metrics
}

// HookRegistry tracks registered hook callbacks (SDK callbacks and plugin
// native hooks). Thread-safe for concurrent register/read/clear.
// Source: bootstrap/state.ts — registeredHooks, registerHookCallbacks,
// clearRegisteredHooks, clearRegisteredPluginHooks
type HookRegistry struct {
	mu    sync.RWMutex
	hooks map[HookEvent][]RegisteredHookCallback
}

// NewHookRegistry creates an empty HookRegistry.
func NewHookRegistry() *HookRegistry {
	return &HookRegistry{}
}

// Register merges hook callbacks into the registry. May be called multiple
// times; new matchers are appended (not overwritten) for each event.
// Source: bootstrap/state.ts — registerHookCallbacks
func (r *HookRegistry) Register(hooks map[HookEvent][]RegisteredHookCallback) {
	if len(hooks) == 0 {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.hooks == nil {
		r.hooks = make(map[HookEvent][]RegisteredHookCallback)
	}
	for event, matchers := range hooks {
		r.hooks[event] = append(r.hooks[event], matchers...)
	}
}

// Get returns the registered callbacks for the given event (may be nil).
// The returned slice must not be mutated by callers.
func (r *HookRegistry) Get(event HookEvent) []RegisteredHookCallback {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.hooks[event]
}

// GetAll returns all registered hooks (may be nil).
// Source: bootstrap/state.ts — getRegisteredHooks
func (r *HookRegistry) GetAll() map[HookEvent][]RegisteredHookCallback {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.hooks
}

// Clear removes all registered hooks.
// Source: bootstrap/state.ts — clearRegisteredHooks
func (r *HookRegistry) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.hooks = nil
}

// ClearPluginHooks removes hooks that have a non-empty PluginRoot, keeping
// only SDK callback hooks. If no hooks remain, the internal map is set to nil.
// Source: bootstrap/state.ts — clearRegisteredPluginHooks
func (r *HookRegistry) ClearPluginHooks() {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.hooks == nil {
		return
	}

	filtered := make(map[HookEvent][]RegisteredHookCallback)
	for event, matchers := range r.hooks {
		var kept []RegisteredHookCallback
		for _, m := range matchers {
			if m.PluginRoot == "" {
				kept = append(kept, m)
			}
		}
		if len(kept) > 0 {
			filtered[event] = kept
		}
	}

	if len(filtered) == 0 {
		r.hooks = nil
	} else {
		r.hooks = filtered
	}
}

// IsEmpty returns true when no hooks are registered.
func (r *HookRegistry) IsEmpty() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.hooks) == 0
}
