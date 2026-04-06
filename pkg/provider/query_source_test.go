package provider

import "testing"

func TestIsAgenticQuerySource(t *testing.T) {
	// Source: claude.ts:1066-1070
	tests := []struct {
		source QuerySource
		want   bool
	}{
		{QuerySourceREPLMainThread, true},
		{QuerySourceSDK, true},
		{QuerySourceHookAgent, true},
		{QuerySourceVerificationAgent, true},
		{"agent:subagent_123", true},
		{"agent:", true}, // edge: prefix only — still counts
		{QuerySourceCompact, false},
		{QuerySourceMicroCompact, false},
		{QuerySourceTitle, false},
		{QuerySourceMemory, false},
		{"random_source", false},
		{"", false},
	}

	for _, tc := range tests {
		t.Run(string(tc.source), func(t *testing.T) {
			got := IsAgenticQuerySource(tc.source)
			if got != tc.want {
				t.Errorf("IsAgenticQuerySource(%q) = %v, want %v", tc.source, got, tc.want)
			}
		})
	}
}

func TestShouldRetry529(t *testing.T) {
	// Foreground sources should retry 529s; background sources should not.
	// Source: withRetry.ts:316-324
	tests := []struct {
		source QuerySource
		want   bool
	}{
		{QuerySourceREPLMainThread, true},
		{QuerySourceSDK, true},
		{QuerySourceCompact, false},
		{QuerySourceTitle, false},
		{"agent:tools", true},
	}

	for _, tc := range tests {
		t.Run(string(tc.source), func(t *testing.T) {
			got := ShouldRetry529(tc.source)
			if got != tc.want {
				t.Errorf("ShouldRetry529(%q) = %v, want %v", tc.source, got, tc.want)
			}
		})
	}
}

func TestQuerySourceConstants(t *testing.T) {
	// Verify the string values match what the TS codebase uses.
	if QuerySourceREPLMainThread != "repl_main_thread" {
		t.Errorf("QuerySourceREPLMainThread = %q, want repl_main_thread", QuerySourceREPLMainThread)
	}
	if QuerySourceSDK != "sdk" {
		t.Errorf("QuerySourceSDK = %q, want sdk", QuerySourceSDK)
	}
	if QuerySourceHookAgent != "hook_agent" {
		t.Errorf("QuerySourceHookAgent = %q, want hook_agent", QuerySourceHookAgent)
	}
	if QuerySourceVerificationAgent != "verification_agent" {
		t.Errorf("QuerySourceVerificationAgent = %q, want verification_agent", QuerySourceVerificationAgent)
	}
}
