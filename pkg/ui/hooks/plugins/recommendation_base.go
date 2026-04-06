package plugins

import (
	"sync"
)

// RecommendationBase is the Go equivalent of usePluginRecommendationBase.
// It provides a generic gate chain for plugin recommendations: at most one
// recommendation active, no concurrent resolution, no re-trigger while
// showing, clearable.
//
// Source: src/hooks/usePluginRecommendationBase.tsx
type RecommendationBase[T any] struct {
	mu         sync.Mutex
	value      *T
	checking   bool
	isRemote   bool // skip recommendations in remote mode
}

// NewRecommendationBase creates a new recommendation base.
// Set isRemote=true to suppress all recommendations (remote/headless mode).
func NewRecommendationBase[T any](isRemote bool) *RecommendationBase[T] {
	return &RecommendationBase[T]{isRemote: isRemote}
}

// TryResolve calls resolve if no recommendation is active and no resolution
// is in flight. If resolve returns a non-nil value it becomes the current
// recommendation. Thread-safe; resolve is called under the lock so callers
// should keep it fast (no I/O). For async resolution, resolve should
// capture state and the caller should call SetRecommendation separately.
func (rb *RecommendationBase[T]) TryResolve(resolve func() *T) {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	if rb.isRemote {
		return
	}
	if rb.value != nil {
		return
	}
	if rb.checking {
		return
	}
	rb.checking = true
	defer func() { rb.checking = false }()

	if v := resolve(); v != nil {
		rb.value = v
	}
}

// SetRecommendation sets the recommendation directly (for async flows).
func (rb *RecommendationBase[T]) SetRecommendation(v *T) {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	rb.value = v
}

// Recommendation returns the current recommendation, or nil.
func (rb *RecommendationBase[T]) Recommendation() *T {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	return rb.value
}

// Clear removes the current recommendation, allowing a new resolution.
func (rb *RecommendationBase[T]) Clear() {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	rb.value = nil
}

// IsPluginInstalled checks whether a plugin ID is present in the given
// installed set. This is the core of the "plugin recommendation base"
// gate: don't recommend what's already installed.
//
// Source: usePluginRecommendationBase.tsx — isPluginInstalled check
func IsPluginInstalled(pluginID string, installed map[string]struct{}) bool {
	_, ok := installed[pluginID]
	return ok
}
