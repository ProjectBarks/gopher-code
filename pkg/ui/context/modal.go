package context

// Source: context/modalContext.tsx, context/overlayContext.tsx
//
// In TS, ModalContext is a React context providing the modal slot's available
// size so components inside it (Select pagination, Tabs scroll, etc.) can
// cap their height to the modal's inner area instead of the full terminal.
//
// OverlayContext tracks active overlay IDs so escape key handling can
// distinguish "dismiss overlay" from "cancel running request".
//
// In Go, both become struct fields on the parent model — no context providers
// needed. Components receive a ModalState or check OverlayTracker directly.

// ModalState describes the available space when rendering inside a modal slot.
// nil/zero value = not inside a modal (use terminal size instead).
type ModalState struct {
	Active  bool // true when rendering inside the modal slot
	Rows    int  // available content rows
	Columns int  // available content columns
}

// AvailableSize returns the modal dimensions if active, otherwise the fallback.
// This is the Go equivalent of useModalOrTerminalSize().
func (m ModalState) AvailableSize(fallbackRows, fallbackCols int) (rows, cols int) {
	if m.Active {
		return m.Rows, m.Columns
	}
	return fallbackRows, fallbackCols
}

// OverlayTracker tracks active overlay IDs for escape key coordination.
// Components that capture Escape (Select, dialogs, etc.) register themselves
// so CancelRequestHandler knows when Escape should dismiss an overlay rather
// than cancel a running request.
//
// Non-modal overlays (like autocomplete) don't disable text input focus.
type OverlayTracker struct {
	active map[string]bool
}

// NonModalOverlays are overlay IDs that don't capture all input.
var NonModalOverlays = map[string]bool{
	"autocomplete": true,
}

// NewOverlayTracker creates an empty overlay tracker.
func NewOverlayTracker() *OverlayTracker {
	return &OverlayTracker{active: make(map[string]bool)}
}

// Register adds an overlay as active.
func (t *OverlayTracker) Register(id string) {
	t.active[id] = true
}

// Unregister removes an overlay.
func (t *OverlayTracker) Unregister(id string) {
	delete(t.active, id)
}

// IsAnyActive returns true if any overlay is registered.
func (t *OverlayTracker) IsAnyActive() bool {
	return len(t.active) > 0
}

// IsModalActive returns true if any modal (non-autocomplete) overlay is active.
// Modal overlays capture all input; non-modal ones (autocomplete) don't.
func (t *OverlayTracker) IsModalActive() bool {
	for id := range t.active {
		if !NonModalOverlays[id] {
			return true
		}
	}
	return false
}

// ActiveIDs returns all currently active overlay IDs.
func (t *OverlayTracker) ActiveIDs() []string {
	ids := make([]string, 0, len(t.active))
	for id := range t.active {
		ids = append(ids, id)
	}
	return ids
}

// Count returns the number of active overlays.
func (t *OverlayTracker) Count() int {
	return len(t.active)
}
