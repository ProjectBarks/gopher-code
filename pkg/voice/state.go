// Package voice manages voice input state for the TUI.
//
// Source: context/voice.tsx, hooks/useVoice.ts
//
// In TS, VoiceState lives in a React context store with selectors.
// In Go, it's a plain struct on the app model — no providers needed.
package voice

// Phase describes the current voice input lifecycle phase.
type Phase string

const (
	// PhaseIdle means no voice activity.
	PhaseIdle Phase = "idle"
	// PhaseRecording means the microphone is active and capturing audio.
	PhaseRecording Phase = "recording"
	// PhaseProcessing means audio is being transcribed.
	PhaseProcessing Phase = "processing"
)

// State holds the current voice input state.
// Zero value is idle with no error.
type State struct {
	// Phase is the current voice lifecycle phase.
	Phase Phase
	// Error is a user-visible error message, or empty.
	Error string
	// InterimTranscript is the partial transcription shown during recording.
	InterimTranscript string
	// AudioLevels is recent audio amplitude samples for the VU meter.
	AudioLevels []float64
	// WarmingUp is true while the voice engine is initializing.
	WarmingUp bool
}

// DefaultState returns an idle state with no error.
func DefaultState() State {
	return State{Phase: PhaseIdle}
}

// IsIdle returns true if voice is not active.
func (s State) IsIdle() bool { return s.Phase == PhaseIdle }

// IsRecording returns true if the microphone is active.
func (s State) IsRecording() bool { return s.Phase == PhaseRecording }

// IsProcessing returns true if audio is being transcribed.
func (s State) IsProcessing() bool { return s.Phase == PhaseProcessing }

// IsActive returns true if voice is recording or processing.
func (s State) IsActive() bool { return s.Phase != PhaseIdle }

// HasError returns true if there is a voice error.
func (s State) HasError() bool { return s.Error != "" }

// StartRecording transitions to the recording phase.
func (s *State) StartRecording() {
	s.Phase = PhaseRecording
	s.Error = ""
	s.InterimTranscript = ""
	s.AudioLevels = nil
	s.WarmingUp = false
}

// StartProcessing transitions from recording to processing.
func (s *State) StartProcessing() {
	s.Phase = PhaseProcessing
	s.AudioLevels = nil
}

// SetInterimTranscript updates the partial transcription during recording.
func (s *State) SetInterimTranscript(text string) {
	s.InterimTranscript = text
}

// UpdateAudioLevels replaces the current audio level samples.
func (s *State) UpdateAudioLevels(levels []float64) {
	s.AudioLevels = levels
}

// SetWarmingUp marks the engine as warming up.
func (s *State) SetWarmingUp(warming bool) {
	s.WarmingUp = warming
}

// SetError sets an error and returns to idle.
func (s *State) SetError(msg string) {
	s.Phase = PhaseIdle
	s.Error = msg
	s.AudioLevels = nil
	s.WarmingUp = false
}

// Reset returns to idle, clearing all state.
func (s *State) Reset() {
	*s = DefaultState()
}

// Complete finishes voice input and returns to idle.
// The final transcript is returned by the voice engine, not stored here.
func (s *State) Complete() {
	s.Phase = PhaseIdle
	s.InterimTranscript = ""
	s.AudioLevels = nil
	s.WarmingUp = false
}
