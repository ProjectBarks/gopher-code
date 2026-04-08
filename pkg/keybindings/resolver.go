package keybindings

import tea "charm.land/bubbletea/v2"

// Source: keybindings/resolver.ts
//
// Resolves key events to actions, with multi-keystroke chord support.
// In TS this is a pure function + external pending state. In Go the
// ChordResolver struct owns the pending state.

// ResolveResultType classifies a key resolution outcome.
type ResolveResultType int

const (
	ResolveNone           ResolveResultType = iota // no binding matched
	ResolveMatch                                   // action found
	ResolveUnbound                                 // explicitly unbound (null action)
	ResolveChordStarted                            // first key of a multi-key chord
	ResolveChordCancelled                          // chord sequence broken
)

// ResolveResult is the outcome of resolving a key event.
type ResolveResult struct {
	Type    ResolveResultType
	Action  string             // set when Type == ResolveMatch
	Pending []ParsedKeystroke  // set when Type == ResolveChordStarted
}

// keystrokesEqual compares two keystrokes, collapsing alt/meta.
// Source: resolver.ts:107-118
func keystrokesEqual(a, b ParsedKeystroke) bool {
	return a.Key == b.Key &&
		a.Ctrl == b.Ctrl &&
		a.Shift == b.Shift &&
		(a.Alt || a.Meta) == (b.Alt || b.Meta) &&
		a.Super == b.Super
}

// chordPrefixMatches checks if prefix matches the start of binding's chord.
func chordPrefixMatches(prefix []ParsedKeystroke, binding ParsedBinding) bool {
	if len(prefix) >= len(binding.Chord) {
		return false
	}
	for i := range prefix {
		if !keystrokesEqual(prefix[i], binding.Chord[i]) {
			return false
		}
	}
	return true
}

// chordExactlyMatches checks if chord matches binding's chord exactly.
func chordExactlyMatches(chord []ParsedKeystroke, binding ParsedBinding) bool {
	if len(chord) != len(binding.Chord) {
		return false
	}
	for i := range chord {
		if !keystrokesEqual(chord[i], binding.Chord[i]) {
			return false
		}
	}
	return true
}

// buildKeystroke converts a bubbletea key event to a ParsedKeystroke.
func buildKeystroke(msg tea.KeyPressMsg) *ParsedKeystroke {
	keyName := GetKeyName(msg)
	if keyName == "" {
		return nil
	}
	ctrl, shift, alt, super := extractModifiers(msg)
	// QUIRK: ignore alt on escape (see match.go)
	if msg.Code == tea.KeyEscape {
		alt = false
	}
	return &ParsedKeystroke{
		Key:   keyName,
		Ctrl:  ctrl,
		Shift: shift,
		Alt:   alt,
		Meta:  alt, // terminals conflate alt/meta
		Super: super,
	}
}

// ChordResolver tracks chord state and resolves key events to actions.
type ChordResolver struct {
	bindings []ParsedBinding
	pending  []ParsedKeystroke // nil when not in a chord
}

// NewChordResolver creates a resolver with the given parsed bindings.
func NewChordResolver(bindings []ParsedBinding) *ChordResolver {
	return &ChordResolver{bindings: bindings}
}

// Pending returns the current chord prefix, or nil if not in a chord.
func (r *ChordResolver) Pending() []ParsedKeystroke { return r.pending }

// ClearPending cancels any in-progress chord.
func (r *ChordResolver) ClearPending() { r.pending = nil }

// Resolve processes a key event against the active contexts and returns
// the resolution result. Manages chord state internally.
// Source: resolver.ts:166-244 (resolveKeyWithChordState)
func (r *ChordResolver) Resolve(msg tea.KeyPressMsg, activeContexts []Context) ResolveResult {
	// Cancel chord on escape
	if msg.Code == tea.KeyEscape && r.pending != nil {
		r.pending = nil
		return ResolveResult{Type: ResolveChordCancelled}
	}

	ks := buildKeystroke(msg)
	if ks == nil {
		if r.pending != nil {
			r.pending = nil
			return ResolveResult{Type: ResolveChordCancelled}
		}
		return ResolveResult{Type: ResolveNone}
	}

	// Build test chord: pending + current keystroke
	var testChord []ParsedKeystroke
	if r.pending != nil {
		testChord = append(append([]ParsedKeystroke{}, r.pending...), *ks)
	} else {
		testChord = []ParsedKeystroke{*ks}
	}

	// Filter bindings by active contexts
	ctxSet := make(map[Context]bool, len(activeContexts))
	for _, c := range activeContexts {
		ctxSet[c] = true
	}

	// Check for longer chord prefixes (chord_started)
	hasLonger := false
	for _, b := range r.bindings {
		if !ctxSet[b.Context] {
			continue
		}
		if len(b.Chord) > len(testChord) && chordPrefixMatches(testChord, b) && b.Action != "" {
			hasLonger = true
			break
		}
	}

	if hasLonger {
		r.pending = testChord
		return ResolveResult{Type: ResolveChordStarted, Pending: testChord}
	}

	// Check for exact match (last one wins for overrides)
	var exactMatch *ParsedBinding
	for i := range r.bindings {
		b := &r.bindings[i]
		if !ctxSet[b.Context] {
			continue
		}
		if chordExactlyMatches(testChord, *b) {
			exactMatch = b
		}
	}

	r.pending = nil // chord resolved or cancelled

	if exactMatch != nil {
		if exactMatch.Action == "" {
			return ResolveResult{Type: ResolveUnbound}
		}
		return ResolveResult{Type: ResolveMatch, Action: exactMatch.Action}
	}

	// No match — if we were in a chord, it's cancelled
	if len(testChord) > 1 {
		return ResolveResult{Type: ResolveChordCancelled}
	}
	return ResolveResult{Type: ResolveNone}
}
