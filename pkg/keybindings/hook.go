package keybindings

import tea "charm.land/bubbletea/v2"

// Source: keybindings/useKeybinding.ts
//
// In TS, useKeybinding/useKeybindings are React hooks that bridge Ink's
// useInput to the chord resolver and context stack. In Go/bubbletea,
// we expose a KeybindingHandler that a Model.Update() calls with each
// tea.KeyPressMsg. It resolves the key, invokes the matching handler,
// and reports back the result so the caller can decide whether to
// propagate the event further.

// HandleResult describes what happened when a key event was processed.
type HandleResult int

const (
	// HandleNone means no binding matched; the key should propagate.
	HandleNone HandleResult = iota
	// HandleMatched means an action was resolved and its handler was invoked.
	HandleMatched
	// HandleChordStarted means a multi-key chord is in progress; swallow the key.
	HandleChordStarted
	// HandleChordCancelled means a pending chord was broken; the key may propagate.
	HandleChordCancelled
	// HandleUnbound means the key is explicitly unbound; swallow the key.
	HandleUnbound
)

// KeybindingHandler ties a ChordResolver and ContextStack together so that
// bubbletea Model.Update() methods have a single call to dispatch keybindings.
type KeybindingHandler struct {
	resolver *ChordResolver
	stack    *ContextStack
}

// NewKeybindingHandler creates a handler from the given resolver and context stack.
func NewKeybindingHandler(resolver *ChordResolver, stack *ContextStack) *KeybindingHandler {
	return &KeybindingHandler{resolver: resolver, stack: stack}
}

// Handle processes a key press message through the resolver and, on a match,
// invokes the handler registered in the context stack. The caller's context
// is automatically included alongside all active contexts from the stack.
//
// Usage in a bubbletea Update():
//
//	func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
//	    switch msg := msg.(type) {
//	    case tea.KeyPressMsg:
//	        if m.keybindings.Handle(msg) == HandleMatched {
//	            return m, nil // consumed
//	        }
//	        // fall through to normal key handling
//	    }
//	}
func (h *KeybindingHandler) Handle(msg tea.KeyPressMsg) HandleResult {
	contexts := h.stack.ActiveContexts()
	result := h.resolver.Resolve(msg, contexts)

	switch result.Type {
	case ResolveMatch:
		h.stack.InvokeAction(Action(result.Action))
		return HandleMatched
	case ResolveChordStarted:
		return HandleChordStarted
	case ResolveChordCancelled:
		return HandleChordCancelled
	case ResolveUnbound:
		return HandleUnbound
	default:
		return HandleNone
	}
}

// PendingChord returns the current pending chord keystrokes, or nil.
func (h *KeybindingHandler) PendingChord() []ParsedKeystroke {
	return h.resolver.Pending()
}

// ClearPendingChord cancels any in-progress chord sequence.
func (h *KeybindingHandler) ClearPendingChord() {
	h.resolver.ClearPending()
}
