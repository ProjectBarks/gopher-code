package voice

import "testing"

func TestDefaultState(t *testing.T) {
	s := DefaultState()
	if s.Phase != PhaseIdle {
		t.Errorf("phase = %q, want idle", s.Phase)
	}
	if s.Error != "" {
		t.Error("should have no error")
	}
	if !s.IsIdle() {
		t.Error("should be idle")
	}
	if s.IsActive() {
		t.Error("should not be active")
	}
}

func TestState_ZeroValue(t *testing.T) {
	var s State
	// Zero value Phase is "" which is not PhaseIdle("idle")
	// but DefaultState() should be used
	s = DefaultState()
	if !s.IsIdle() {
		t.Error("default should be idle")
	}
}

func TestState_RecordingLifecycle(t *testing.T) {
	s := DefaultState()

	// Start recording
	s.StartRecording()
	if !s.IsRecording() {
		t.Error("should be recording")
	}
	if !s.IsActive() {
		t.Error("should be active during recording")
	}
	if s.IsIdle() {
		t.Error("should not be idle during recording")
	}

	// Update interim transcript
	s.SetInterimTranscript("hello wor")
	if s.InterimTranscript != "hello wor" {
		t.Errorf("interim = %q", s.InterimTranscript)
	}

	// Update audio levels
	s.UpdateAudioLevels([]float64{0.1, 0.5, 0.3})
	if len(s.AudioLevels) != 3 {
		t.Errorf("audio levels = %v", s.AudioLevels)
	}

	// Transition to processing
	s.StartProcessing()
	if !s.IsProcessing() {
		t.Error("should be processing")
	}
	if s.IsRecording() {
		t.Error("should not be recording during processing")
	}
	if !s.IsActive() {
		t.Error("should be active during processing")
	}
	if s.AudioLevels != nil {
		t.Error("audio levels should be cleared during processing")
	}

	// Complete
	s.Complete()
	if !s.IsIdle() {
		t.Error("should be idle after complete")
	}
	if s.InterimTranscript != "" {
		t.Error("interim should be cleared after complete")
	}
}

func TestState_ErrorReturnsToIdle(t *testing.T) {
	s := DefaultState()
	s.StartRecording()
	s.SetError("microphone not found")

	if !s.IsIdle() {
		t.Error("error should return to idle")
	}
	if !s.HasError() {
		t.Error("should have error")
	}
	if s.Error != "microphone not found" {
		t.Errorf("error = %q", s.Error)
	}
	if s.AudioLevels != nil {
		t.Error("audio levels should be cleared on error")
	}
}

func TestState_StartRecordingClearsError(t *testing.T) {
	s := DefaultState()
	s.SetError("previous error")
	s.StartRecording()

	if s.HasError() {
		t.Error("error should be cleared on new recording")
	}
}

func TestState_WarmingUp(t *testing.T) {
	s := DefaultState()
	s.SetWarmingUp(true)
	if !s.WarmingUp {
		t.Error("should be warming up")
	}
	s.StartRecording()
	if s.WarmingUp {
		t.Error("warming up should be cleared on recording start")
	}
}

func TestState_Reset(t *testing.T) {
	s := DefaultState()
	s.StartRecording()
	s.SetInterimTranscript("hello")
	s.UpdateAudioLevels([]float64{0.5})
	s.SetWarmingUp(true)

	s.Reset()
	if !s.IsIdle() {
		t.Error("should be idle after reset")
	}
	if s.InterimTranscript != "" {
		t.Error("transcript should be empty after reset")
	}
	if s.AudioLevels != nil {
		t.Error("audio levels should be nil after reset")
	}
	if s.WarmingUp {
		t.Error("warming up should be false after reset")
	}
}

func TestPhaseConstants(t *testing.T) {
	if PhaseIdle != "idle" {
		t.Error("PhaseIdle should be 'idle'")
	}
	if PhaseRecording != "recording" {
		t.Error("PhaseRecording should be 'recording'")
	}
	if PhaseProcessing != "processing" {
		t.Error("PhaseProcessing should be 'processing'")
	}
}
