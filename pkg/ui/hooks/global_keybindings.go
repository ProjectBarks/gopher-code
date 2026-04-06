package hooks

// GlobalKeybindings dispatches app-level keybinding actions not handled by
// specific models: ctrl+t (cycle expanded view), ctrl+o (toggle transcript),
// ctrl+e (show-all in transcript), ctrl+shift+b (brief toggle),
// ctrl+c/escape (exit transcript).
// Source: hooks/useGlobalKeybindings.tsx

// ExpandedView enumerates the possible expanded-panel states.
type ExpandedView string

const (
	ExpandedNone      ExpandedView = "none"
	ExpandedTasks     ExpandedView = "tasks"
	ExpandedTeammates ExpandedView = "teammates"
)

// Screen identifies the current top-level screen.
type Screen string

const (
	ScreenPrompt     Screen = "prompt"
	ScreenTranscript Screen = "transcript"
)

// GlobalKeybindings manages state for the global keybinding handlers.
type GlobalKeybindings struct {
	Screen               Screen
	ExpandedView         ExpandedView
	ShowAllInTranscript  bool
	IsBriefOnly          bool
	HasRunningTeammates  bool
	BriefFeatureEnabled  bool
	VirtualScrollActive  bool
	SearchBarOpen        bool
	MessageCount         int

	// Callbacks invoked on screen transitions.
	OnEnterTranscript func()
	OnExitTranscript  func()

	// LogEvent is called with event name and metadata.
	LogEvent func(event string, meta map[string]any)
}

// NewGlobalKeybindings returns a GlobalKeybindings with sensible defaults.
func NewGlobalKeybindings() *GlobalKeybindings {
	return &GlobalKeybindings{
		Screen:       ScreenPrompt,
		ExpandedView: ExpandedNone,
		LogEvent:     func(string, map[string]any) {},
	}
}

// HandleToggleTodos processes ctrl+t: cycles expanded view.
// With running teammates: none -> tasks -> teammates -> none.
// Without teammates: none <-> tasks.
func (g *GlobalKeybindings) HandleToggleTodos() {
	g.LogEvent("tengu_toggle_todos", map[string]any{
		"is_expanded": g.ExpandedView == ExpandedTasks,
	})

	if g.HasRunningTeammates {
		switch g.ExpandedView {
		case ExpandedNone:
			g.ExpandedView = ExpandedTasks
		case ExpandedTasks:
			g.ExpandedView = ExpandedTeammates
		case ExpandedTeammates:
			g.ExpandedView = ExpandedNone
		}
		return
	}

	// Tasks only: binary toggle.
	if g.ExpandedView == ExpandedTasks {
		g.ExpandedView = ExpandedNone
	} else {
		g.ExpandedView = ExpandedTasks
	}
}

// HandleToggleTranscript processes ctrl+o: toggles between prompt and transcript.
// Includes escape-hatch logic for stuck brief-only state when KAIROS kill-switch fires.
func (g *GlobalKeybindings) HandleToggleTranscript() {
	// Escape hatch: if brief is stuck on but feature disabled, clear it first.
	if g.IsBriefOnly && !g.BriefFeatureEnabled && g.Screen != ScreenTranscript {
		g.IsBriefOnly = false
		return
	}

	isEntering := g.Screen != ScreenTranscript

	g.LogEvent("tengu_toggle_transcript", map[string]any{
		"is_entering":   isEntering,
		"show_all":      g.ShowAllInTranscript,
		"message_count": g.MessageCount,
	})

	if isEntering {
		g.Screen = ScreenTranscript
		if g.OnEnterTranscript != nil {
			g.OnEnterTranscript()
		}
	} else {
		g.Screen = ScreenPrompt
		if g.OnExitTranscript != nil {
			g.OnExitTranscript()
		}
	}
}

// HandleToggleShowAll processes ctrl+e: toggles showing all messages in transcript.
func (g *GlobalKeybindings) HandleToggleShowAll() {
	g.ShowAllInTranscript = !g.ShowAllInTranscript
	g.LogEvent("tengu_transcript_toggle_show_all", map[string]any{
		"is_expanding":  g.ShowAllInTranscript,
		"message_count": g.MessageCount,
	})
}

// HandleExitTranscript processes ctrl+c/escape when in transcript mode.
// Returns true if the event was consumed (was in transcript mode).
func (g *GlobalKeybindings) HandleExitTranscript() bool {
	if g.Screen != ScreenTranscript {
		return false
	}
	if g.VirtualScrollActive || g.SearchBarOpen {
		return false
	}

	g.LogEvent("tengu_transcript_exit", map[string]any{
		"show_all":      g.ShowAllInTranscript,
		"message_count": g.MessageCount,
	})
	g.Screen = ScreenPrompt
	if g.OnExitTranscript != nil {
		g.OnExitTranscript()
	}
	return true
}

// HandleToggleBrief processes ctrl+shift+b: toggles brief-only mode.
// Asymmetric gate: turning OFF is always allowed (escape hatch for stuck state).
// Turning ON requires the brief feature to be enabled.
func (g *GlobalKeybindings) HandleToggleBrief() {
	if g.IsBriefOnly {
		// OFF always allowed.
		g.IsBriefOnly = false
		return
	}
	if g.BriefFeatureEnabled {
		g.IsBriefOnly = true
	}
}
