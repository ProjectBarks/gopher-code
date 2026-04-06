package coordinator

import (
	"os"
	"testing"

	"github.com/projectbarks/gopher-code/pkg/analytics"
)

// captureSink captures analytics events for assertions.
type captureSink struct {
	events []capturedEvent
}

type capturedEvent struct {
	name     string
	metadata analytics.EventMetadata
}

func (s *captureSink) LogEvent(name string, md analytics.EventMetadata)      { s.events = append(s.events, capturedEvent{name, md}) }
func (s *captureSink) LogEventAsync(name string, md analytics.EventMetadata) { s.events = append(s.events, capturedEvent{name, md}) }
func (s *captureSink) Shutdown()                                              {}

func resetEnv(t *testing.T) {
	t.Helper()
	os.Unsetenv(coordinatorModeEnv)
	FeatureGateChecker = nil
	analytics.ResetForTesting()
}

// --- T14: isCoordinatorMode flag ---

func TestIsCoordinatorMode_FalseWhenGateDisabled(t *testing.T) {
	resetEnv(t)
	os.Setenv(coordinatorModeEnv, "1")
	defer os.Unsetenv(coordinatorModeEnv)

	// No gate checker → gate disabled → false
	if IsCoordinatorMode() {
		t.Fatal("expected false when feature gate is disabled")
	}
}

func TestIsCoordinatorMode_FalseWhenEnvUnset(t *testing.T) {
	resetEnv(t)
	FeatureGateChecker = func(gate string) bool { return gate == coordinatorModeGate }

	if IsCoordinatorMode() {
		t.Fatal("expected false when env is unset")
	}
}

func TestIsCoordinatorMode_TrueWhenBothEnabled(t *testing.T) {
	resetEnv(t)
	FeatureGateChecker = func(gate string) bool { return gate == coordinatorModeGate }
	os.Setenv(coordinatorModeEnv, "1")
	defer os.Unsetenv(coordinatorModeEnv)

	if !IsCoordinatorMode() {
		t.Fatal("expected true when gate enabled and env truthy")
	}
}

func TestIsCoordinatorMode_AcceptsTruthyVariants(t *testing.T) {
	resetEnv(t)
	FeatureGateChecker = func(gate string) bool { return true }

	for _, val := range []string{"1", "true", "TRUE", "yes", "Yes"} {
		os.Setenv(coordinatorModeEnv, val)
		if !IsCoordinatorMode() {
			t.Fatalf("expected true for env=%q", val)
		}
	}

	for _, val := range []string{"0", "false", "no", ""} {
		os.Setenv(coordinatorModeEnv, val)
		if IsCoordinatorMode() {
			t.Fatalf("expected false for env=%q", val)
		}
	}
}

// --- T15: COORDINATOR_MODE feature gate ---

func TestFeatureGateConstant(t *testing.T) {
	if coordinatorModeGate != "COORDINATOR_MODE" {
		t.Fatalf("gate name must be COORDINATOR_MODE, got %q", coordinatorModeGate)
	}
}

func TestCoordinatorModeEnvName(t *testing.T) {
	if coordinatorModeEnv != "CLAUDE_CODE_COORDINATOR_MODE" {
		t.Fatalf("env name must be CLAUDE_CODE_COORDINATOR_MODE, got %q", coordinatorModeEnv)
	}
}

// --- T16: matchSessionMode resume reconciliation ---

func TestMatchSessionMode_NoStoredMode(t *testing.T) {
	resetEnv(t)
	msg := MatchSessionMode("")
	if msg != "" {
		t.Fatalf("expected empty string for no stored mode, got %q", msg)
	}
}

func TestMatchSessionMode_AlreadyMatched(t *testing.T) {
	resetEnv(t)
	// Both normal → no switch needed
	msg := MatchSessionMode(SessionModeNormal)
	if msg != "" {
		t.Fatalf("expected empty when modes match, got %q", msg)
	}
}

func TestMatchSessionMode_SwitchToCoordinator(t *testing.T) {
	resetEnv(t)
	sink := &captureSink{}
	analytics.AttachSink(sink)

	// Gate enabled but env not set → currently normal mode
	FeatureGateChecker = func(gate string) bool { return gate == coordinatorModeGate }

	msg := MatchSessionMode(SessionModeCoordinator)

	// T17: verbatim enter message
	expected := "Entered coordinator mode to match resumed session."
	if msg != expected {
		t.Fatalf("expected %q, got %q", expected, msg)
	}

	// Env should now be set
	if os.Getenv(coordinatorModeEnv) != "1" {
		t.Fatal("env should be set to 1 after switching to coordinator")
	}

	// After flip, IsCoordinatorMode should return true
	if !IsCoordinatorMode() {
		t.Fatal("IsCoordinatorMode should be true after switching to coordinator")
	}

	// T18: analytics event
	if len(sink.events) != 1 {
		t.Fatalf("expected 1 analytics event, got %d", len(sink.events))
	}
	if sink.events[0].name != "tengu_coordinator_mode_switched" {
		t.Fatalf("expected tengu_coordinator_mode_switched, got %q", sink.events[0].name)
	}
	if sink.events[0].metadata["to"] != "coordinator" {
		t.Fatalf("expected to=coordinator, got %v", sink.events[0].metadata["to"])
	}
}

func TestMatchSessionMode_SwitchToNormal(t *testing.T) {
	resetEnv(t)
	sink := &captureSink{}
	analytics.AttachSink(sink)

	// Set up coordinator mode
	FeatureGateChecker = func(gate string) bool { return gate == coordinatorModeGate }
	os.Setenv(coordinatorModeEnv, "1")

	msg := MatchSessionMode(SessionModeNormal)

	// T17: verbatim exit message
	expected := "Exited coordinator mode to match resumed session."
	if msg != expected {
		t.Fatalf("expected %q, got %q", expected, msg)
	}

	// Env should be cleared
	if os.Getenv(coordinatorModeEnv) != "" {
		t.Fatalf("env should be cleared after switching to normal, got %q", os.Getenv(coordinatorModeEnv))
	}

	// After flip, IsCoordinatorMode should return false
	if IsCoordinatorMode() {
		t.Fatal("IsCoordinatorMode should be false after switching to normal")
	}

	// T18: analytics event
	if len(sink.events) != 1 {
		t.Fatalf("expected 1 analytics event, got %d", len(sink.events))
	}
	if sink.events[0].name != "tengu_coordinator_mode_switched" {
		t.Fatalf("expected tengu_coordinator_mode_switched, got %q", sink.events[0].name)
	}
	if sink.events[0].metadata["to"] != "normal" {
		t.Fatalf("expected to=normal, got %v", sink.events[0].metadata["to"])
	}
}

// --- T17: verbatim messages ---

func TestMatchSessionMode_VerbatimMessages(t *testing.T) {
	// Test exact string matching for both messages
	resetEnv(t)
	analytics.ResetForTesting()
	sink := &captureSink{}
	analytics.AttachSink(sink)
	FeatureGateChecker = func(gate string) bool { return gate == coordinatorModeGate }

	// Switch to coordinator
	msg := MatchSessionMode(SessionModeCoordinator)
	if msg != "Entered coordinator mode to match resumed session." {
		t.Fatalf("enter message mismatch: %q", msg)
	}

	// Reset sink for next event
	analytics.ResetForTesting()
	sink2 := &captureSink{}
	analytics.AttachSink(sink2)

	// Switch back to normal
	msg = MatchSessionMode(SessionModeNormal)
	if msg != "Exited coordinator mode to match resumed session." {
		t.Fatalf("exit message mismatch: %q", msg)
	}
}

// --- T18: analytics event name ---

func TestAnalyticsEventName(t *testing.T) {
	resetEnv(t)
	sink := &captureSink{}
	analytics.AttachSink(sink)
	FeatureGateChecker = func(gate string) bool { return gate == coordinatorModeGate }

	MatchSessionMode(SessionModeCoordinator)

	if len(sink.events) == 0 {
		t.Fatal("expected analytics event")
	}
	if sink.events[0].name != "tengu_coordinator_mode_switched" {
		t.Fatalf("event name must be tengu_coordinator_mode_switched, got %q", sink.events[0].name)
	}
}
