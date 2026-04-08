package aws

import (
	"encoding/json"
	"testing"
)

func TestIsValidSTSOutput_Valid(t *testing.T) {
	out := STSOutput{
		Credentials: Credentials{
			AccessKeyID:     "AKIAIOSFODNN7EXAMPLE",
			SecretAccessKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
			SessionToken:    "FwoGZXIvYXdzEBYaDBeRsm...",
		},
	}
	data, _ := json.Marshal(out)
	if !IsValidSTSOutput(data) {
		t.Error("should be valid")
	}
}

func TestIsValidSTSOutput_MissingFields(t *testing.T) {
	out := STSOutput{
		Credentials: Credentials{
			AccessKeyID: "AKIAIOSFODNN7EXAMPLE",
			// missing SecretAccessKey and SessionToken
		},
	}
	data, _ := json.Marshal(out)
	if IsValidSTSOutput(data) {
		t.Error("should be invalid — missing fields")
	}
}

func TestIsValidSTSOutput_BadJSON(t *testing.T) {
	if IsValidSTSOutput([]byte("not json")) {
		t.Error("should be invalid for bad JSON")
	}
}

func TestAuthStatusManager_Lifecycle(t *testing.T) {
	m := &AuthStatusManager{}

	// Initial state
	s := m.GetStatus()
	if s.IsAuthenticating {
		t.Error("should not be authenticating initially")
	}

	// Start
	m.StartAuthentication()
	s = m.GetStatus()
	if !s.IsAuthenticating {
		t.Error("should be authenticating after Start")
	}

	// Add output
	m.AddOutput("Refreshing credentials...")
	s = m.GetStatus()
	if len(s.Output) != 1 || s.Output[0] != "Refreshing credentials..." {
		t.Errorf("output = %v, want [Refreshing credentials...]", s.Output)
	}

	// Set error
	m.SetError("token expired")
	s = m.GetStatus()
	if s.Error != "token expired" {
		t.Errorf("error = %q, want 'token expired'", s.Error)
	}

	// End with failure — keeps error state
	m.EndAuthentication(false)
	s = m.GetStatus()
	if s.IsAuthenticating {
		t.Error("should not be authenticating after End")
	}
	if s.Error != "token expired" {
		t.Error("error should persist on failure")
	}

	// Start again and succeed — clears everything
	m.StartAuthentication()
	m.EndAuthentication(true)
	s = m.GetStatus()
	if s.IsAuthenticating || s.Error != "" || len(s.Output) != 0 {
		t.Error("success should clear all state")
	}
}

func TestAuthStatusManager_OnChange(t *testing.T) {
	m := &AuthStatusManager{}
	callCount := 0
	m.OnChange(func(s AuthStatus) { callCount++ })

	m.StartAuthentication()
	m.AddOutput("line1")
	m.EndAuthentication(true)

	if callCount != 3 {
		t.Errorf("OnChange called %d times, want 3", callCount)
	}
}

func TestIsBedrockEnabled(t *testing.T) {
	t.Setenv("CLAUDE_CODE_USE_BEDROCK", "")
	if IsBedrockEnabled() {
		t.Error("should be false when empty")
	}
	t.Setenv("CLAUDE_CODE_USE_BEDROCK", "1")
	if !IsBedrockEnabled() {
		t.Error("should be true for '1'")
	}
	t.Setenv("CLAUDE_CODE_USE_BEDROCK", "true")
	if !IsBedrockEnabled() {
		t.Error("should be true for 'true'")
	}
}
