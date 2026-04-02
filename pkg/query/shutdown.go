package query

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

// Source: utils/cleanupRegistry.ts, utils/gracefulShutdown.ts

// CleanupRegistry holds functions to run during graceful shutdown.
// Source: utils/cleanupRegistry.ts:7
type CleanupRegistry struct {
	mu    sync.Mutex
	funcs []func()
}

// NewCleanupRegistry creates an empty cleanup registry.
func NewCleanupRegistry() *CleanupRegistry {
	return &CleanupRegistry{}
}

// Register adds a cleanup function. Returns an unregister function.
// Source: utils/cleanupRegistry.ts:14-17
func (r *CleanupRegistry) Register(fn func()) func() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.funcs = append(r.funcs, fn)
	idx := len(r.funcs) - 1
	return func() {
		r.mu.Lock()
		defer r.mu.Unlock()
		if idx < len(r.funcs) {
			r.funcs[idx] = nil
		}
	}
}

// RunAll executes all registered cleanup functions.
// Source: utils/cleanupRegistry.ts:23-25
func (r *CleanupRegistry) RunAll() {
	r.mu.Lock()
	funcs := make([]func(), len(r.funcs))
	copy(funcs, r.funcs)
	r.mu.Unlock()

	var wg sync.WaitGroup
	for _, fn := range funcs {
		if fn == nil {
			continue
		}
		wg.Add(1)
		go func(f func()) {
			defer wg.Done()
			f()
		}(fn)
	}
	wg.Wait()
}

// Len returns the number of registered cleanup functions.
func (r *CleanupRegistry) Len() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	count := 0
	for _, fn := range r.funcs {
		if fn != nil {
			count++
		}
	}
	return count
}

// GracefulShutdown sets up signal handling for SIGINT/SIGTERM and runs
// cleanup functions before exiting.
// Returns a context that is cancelled on shutdown signal.
// Source: utils/gracefulShutdown.ts
func GracefulShutdown(registry *CleanupRegistry, timeout time.Duration) context.Context {
	ctx, cancel := context.WithCancel(context.Background())

	sigCh := make(chan os.Signal, 2)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		cancel()

		// Run cleanup with timeout
		done := make(chan struct{})
		go func() {
			registry.RunAll()
			close(done)
		}()

		select {
		case <-done:
			// Cleanup completed
		case <-time.After(timeout):
			// Timeout — force exit
		}

		// Second signal = force exit
		<-sigCh
		os.Exit(1)
	}()

	return ctx
}
