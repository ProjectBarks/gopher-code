package permissions

import "sync"

// ResolveOnce is an atomic resolve-once guard. It ensures that among
// multiple concurrent resolvers (user, hooks, classifier, bridge, channel),
// only the first caller to claim() wins. This closes the TOCTOU window
// between checking IsResolved() and calling Resolve().
//
// Source: src/hooks/toolPermission/PermissionContext.ts — createResolveOnce
type ResolveOnce[T any] struct {
	mu        sync.Mutex
	claimed   bool
	delivered bool
	resolve   func(T)
}

// NewResolveOnce wraps a resolve callback in an atomic guard.
func NewResolveOnce[T any](resolve func(T)) *ResolveOnce[T] {
	return &ResolveOnce[T]{resolve: resolve}
}

// Claim atomically checks-and-marks as resolved. Returns true if this
// caller won the race (nobody else has claimed yet), false otherwise.
// Use this in async callbacks BEFORE doing work, to close the window
// between the IsResolved() check and the actual Resolve() call.
func (r *ResolveOnce[T]) Claim() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.claimed {
		return false
	}
	r.claimed = true
	return true
}

// Resolve delivers the value to the wrapped callback. If already
// delivered, this is a no-op.
func (r *ResolveOnce[T]) Resolve(value T) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.delivered {
		return
	}
	r.delivered = true
	r.claimed = true
	r.resolve(value)
}

// IsResolved returns true if Claim() or Resolve() has been called.
func (r *ResolveOnce[T]) IsResolved() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.claimed
}
