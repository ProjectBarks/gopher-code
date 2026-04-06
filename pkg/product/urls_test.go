package product

import "testing"

func TestConstants(t *testing.T) {
	tests := []struct {
		name string
		got  string
		want string
	}{
		{"ProductURL", ProductURL, "https://claude.com/claude-code"},
		{"ClaudeAIBaseURL", ClaudeAIBaseURL, "https://claude.ai"},
		{"ClaudeAIStagingBaseURL", ClaudeAIStagingBaseURL, "https://claude-ai.staging.ant.dev"},
		{"ClaudeAILocalBaseURL", ClaudeAILocalBaseURL, "http://localhost:4000"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("got %q, want %q", tt.got, tt.want)
			}
		})
	}
}

func TestIsRemoteSessionStaging(t *testing.T) {
	tests := []struct {
		name       string
		sessionID  string
		ingressURL string
		want       bool
	}{
		{"empty", "", "", false},
		{"staging in session ID", "session_staging_abc", "", true},
		{"staging in ingress URL", "", "https://staging.example.com", true},
		{"both staging", "session_staging_abc", "https://staging.example.com", true},
		{"neither staging", "session_prod_abc", "https://prod.example.com", false},
		{"local session ID", "session_local_abc", "", false},
		{"prod session ID", "cse_abc123", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsRemoteSessionStaging(tt.sessionID, tt.ingressURL); got != tt.want {
				t.Errorf("IsRemoteSessionStaging(%q, %q) = %v, want %v",
					tt.sessionID, tt.ingressURL, got, tt.want)
			}
		})
	}
}

func TestIsRemoteSessionLocal(t *testing.T) {
	tests := []struct {
		name       string
		sessionID  string
		ingressURL string
		want       bool
	}{
		{"empty", "", "", false},
		{"local in session ID", "session_local_abc", "", true},
		{"localhost in ingress URL", "", "http://localhost:4000", true},
		{"both local", "session_local_abc", "http://localhost:4000", true},
		{"neither local", "session_prod_abc", "https://prod.example.com", false},
		{"staging session ID", "session_staging_abc", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsRemoteSessionLocal(tt.sessionID, tt.ingressURL); got != tt.want {
				t.Errorf("IsRemoteSessionLocal(%q, %q) = %v, want %v",
					tt.sessionID, tt.ingressURL, got, tt.want)
			}
		})
	}
}

func TestGetClaudeAIBaseURL(t *testing.T) {
	tests := []struct {
		name       string
		sessionID  string
		ingressURL string
		want       string
	}{
		{"prod default", "", "", ClaudeAIBaseURL},
		{"prod session", "session_abc123", "", ClaudeAIBaseURL},
		{"local by session ID", "session_local_abc", "", ClaudeAILocalBaseURL},
		{"local by ingress URL", "", "http://localhost:4000", ClaudeAILocalBaseURL},
		{"staging by session ID", "session_staging_abc", "", ClaudeAIStagingBaseURL},
		{"staging by ingress URL", "", "https://staging.example.com", ClaudeAIStagingBaseURL},
		// local takes precedence over staging (checked first in TS)
		{"local precedence over staging", "session_local_staging_abc", "", ClaudeAILocalBaseURL},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetClaudeAIBaseURL(tt.sessionID, tt.ingressURL); got != tt.want {
				t.Errorf("GetClaudeAIBaseURL(%q, %q) = %q, want %q",
					tt.sessionID, tt.ingressURL, got, tt.want)
			}
		})
	}
}

func TestToCompatSessionID(t *testing.T) {
	tests := []struct {
		name string
		id   string
		want string
	}{
		{"cse prefix replaced", "cse_abc123", "session_abc123"},
		{"session prefix unchanged", "session_abc123", "session_abc123"},
		{"other prefix unchanged", "other_abc123", "other_abc123"},
		{"empty string", "", ""},
		{"cse with staging", "cse_staging_abc", "session_staging_abc"},
		{"cse with local", "cse_local_abc", "session_local_abc"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ToCompatSessionID(tt.id); got != tt.want {
				t.Errorf("ToCompatSessionID(%q) = %q, want %q", tt.id, got, tt.want)
			}
		})
	}
}

func TestToInfraSessionID(t *testing.T) {
	tests := []struct {
		name string
		id   string
		want string
	}{
		{"session prefix replaced", "session_abc123", "cse_abc123"},
		{"cse prefix unchanged", "cse_abc123", "cse_abc123"},
		{"other prefix unchanged", "other_abc123", "other_abc123"},
		{"empty string", "", ""},
		{"session with staging", "session_staging_abc", "cse_staging_abc"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ToInfraSessionID(tt.id); got != tt.want {
				t.Errorf("ToInfraSessionID(%q) = %q, want %q", tt.id, got, tt.want)
			}
		})
	}
}

func TestGetRemoteSessionURL(t *testing.T) {
	tests := []struct {
		name       string
		sessionID  string
		ingressURL string
		want       string
	}{
		{
			"prod cse session",
			"cse_abc123",
			"",
			"https://claude.ai/code/session_abc123",
		},
		{
			"prod session already compat",
			"session_abc123",
			"",
			"https://claude.ai/code/session_abc123",
		},
		{
			"staging cse session",
			"cse_staging_abc",
			"",
			"https://claude-ai.staging.ant.dev/code/session_staging_abc",
		},
		{
			"local cse session",
			"cse_local_abc",
			"",
			"http://localhost:4000/code/session_local_abc",
		},
		{
			"local by ingress URL",
			"cse_abc123",
			"http://localhost:4000",
			"http://localhost:4000/code/session_abc123",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetRemoteSessionURL(tt.sessionID, tt.ingressURL); got != tt.want {
				t.Errorf("GetRemoteSessionURL(%q, %q) = %q, want %q",
					tt.sessionID, tt.ingressURL, got, tt.want)
			}
		})
	}
}
