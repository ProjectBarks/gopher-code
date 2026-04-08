package keybindings

import "sync"

// Source: keybindings/KeybindingContext.tsx
//
// In TS, this is a React context with a Set<KeybindingContextName> tracking
// which contexts are active (e.g., "Global", "Input", "ThemePicker").
// Components register/unregister contexts on mount/unmount.
//
// In Go, we use a thread-safe set with Push/Pop semantics matching the
// component lifecycle (NewAppModel pushes "Global", theme picker pushes
// "ThemePicker" on open, pops on close).

// HandlerFunc is a callback invoked when a keybinding action fires.
type HandlerFunc func()

// HandlerRegistration ties an action+context to a callback.
type HandlerRegistration struct {
	Action  Action
	Context Context
	Handler HandlerFunc
}

// ContextStack manages active keybinding contexts and action handlers.
// It replaces the TS KeybindingContext React context provider.
type ContextStack struct {
	mu       sync.RWMutex
	active   map[Context]int // context → reference count (multiple components can register same context)
	handlers map[Action][]HandlerRegistration
}

// NewContextStack creates an empty context stack.
func NewContextStack() *ContextStack {
	return &ContextStack{
		active:   make(map[Context]int),
		handlers: make(map[Action][]HandlerRegistration),
	}
}

// Push registers a context as active. Multiple pushes of the same context
// increment a reference count — the context stays active until all pushers pop.
func (s *ContextStack) Push(ctx Context) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.active[ctx]++
}

// Pop unregisters a context. Decrements the reference count; removes
// the context only when the count reaches zero.
func (s *ContextStack) Pop(ctx Context) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.active[ctx] > 1 {
		s.active[ctx]--
	} else {
		delete(s.active, ctx)
	}
}

// IsActive returns true if the context has any active registrations.
func (s *ContextStack) IsActive(ctx Context) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.active[ctx] > 0
}

// ActiveContexts returns all currently active contexts.
func (s *ContextStack) ActiveContexts() []Context {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]Context, 0, len(s.active))
	for ctx := range s.active {
		result = append(result, ctx)
	}
	return result
}

// RegisterHandler binds an action+context to a handler. Returns an unregister
// function (matches the TS pattern where registerHandler returns a cleanup fn).
func (s *ContextStack) RegisterHandler(reg HandlerRegistration) func() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.handlers[reg.Action] = append(s.handlers[reg.Action], reg)

	return func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		regs := s.handlers[reg.Action]
		for i, r := range regs {
			if r.Context == reg.Context && r.Action == reg.Action {
				s.handlers[reg.Action] = append(regs[:i], regs[i+1:]...)
				break
			}
		}
		if len(s.handlers[reg.Action]) == 0 {
			delete(s.handlers, reg.Action)
		}
	}
}

// InvokeAction calls all handlers registered for the given action whose
// context is currently active. Returns true if any handler was invoked.
// Source: KeybindingContext.tsx — invokeAction
func (s *ContextStack) InvokeAction(action Action) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	regs, ok := s.handlers[action]
	if !ok {
		return false
	}

	invoked := false
	for _, reg := range regs {
		if s.active[reg.Context] > 0 {
			// Release lock before calling handler to avoid deadlock.
			s.mu.RUnlock()
			reg.Handler()
			s.mu.RLock()
			invoked = true
		}
	}
	return invoked
}
