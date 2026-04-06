package auth

import (
	"os"
	"path/filepath"
	"testing"
)

// T125: sessionIngressToken / oauthTokenFromFd / apiKeyFromFd

func TestFDTokens_SetAndGet(t *testing.T) {
	ResetFDTokens()
	defer ResetFDTokens()

	// Initially empty
	if got := SessionIngressToken(); got != "" {
		t.Errorf("SessionIngressToken() = %q, want empty", got)
	}
	if got := OAuthTokenFromFD(); got != "" {
		t.Errorf("OAuthTokenFromFD() = %q, want empty", got)
	}
	if got := APIKeyFromFD(); got != "" {
		t.Errorf("APIKeyFromFD() = %q, want empty", got)
	}

	// Set tokens
	SetSessionIngressToken("ingress-token-123")
	SetOAuthTokenFromFD("oauth-fd-token-456")
	SetAPIKeyFromFD("sk-ant-fd-key-789")

	if got := SessionIngressToken(); got != "ingress-token-123" {
		t.Errorf("SessionIngressToken() = %q, want %q", got, "ingress-token-123")
	}
	if got := OAuthTokenFromFD(); got != "oauth-fd-token-456" {
		t.Errorf("OAuthTokenFromFD() = %q, want %q", got, "oauth-fd-token-456")
	}
	if got := APIKeyFromFD(); got != "sk-ant-fd-key-789" {
		t.Errorf("APIKeyFromFD() = %q, want %q", got, "sk-ant-fd-key-789")
	}

	// Reset clears all
	ResetFDTokens()
	if got := SessionIngressToken(); got != "" {
		t.Errorf("after reset, SessionIngressToken() = %q, want empty", got)
	}
	if got := OAuthTokenFromFD(); got != "" {
		t.Errorf("after reset, OAuthTokenFromFD() = %q, want empty", got)
	}
	if got := APIKeyFromFD(); got != "" {
		t.Errorf("after reset, APIKeyFromFD() = %q, want empty", got)
	}
}

func TestReadTokenFromWellKnownFile(t *testing.T) {
	dir := t.TempDir()

	// Non-existent file returns empty
	if got := readTokenFromWellKnownFile(filepath.Join(dir, "nope")); got != "" {
		t.Errorf("readTokenFromWellKnownFile(missing) = %q, want empty", got)
	}

	// File with content (with whitespace trimming)
	path := filepath.Join(dir, "token")
	os.WriteFile(path, []byte("  my-secret-token\n  "), 0600)
	if got := readTokenFromWellKnownFile(path); got != "my-secret-token" {
		t.Errorf("readTokenFromWellKnownFile() = %q, want %q", got, "my-secret-token")
	}

	// Empty file returns empty
	emptyPath := filepath.Join(dir, "empty")
	os.WriteFile(emptyPath, []byte("   \n"), 0600)
	if got := readTokenFromWellKnownFile(emptyPath); got != "" {
		t.Errorf("readTokenFromWellKnownFile(empty) = %q, want empty", got)
	}
}

func TestGetAPIKeyFromFD_WellKnownFile(t *testing.T) {
	ResetFDTokens()
	defer ResetFDTokens()

	// Create a well-known file in a temp dir and override the path via env.
	dir := t.TempDir()
	keyPath := filepath.Join(dir, ".api_key")
	os.WriteFile(keyPath, []byte("sk-ant-well-known-key"), 0600)

	// No env var set, no FD — readCredentialFromFD falls back to well-known file.
	// We test the internal function directly to avoid needing the real CCR path.
	t.Setenv("CLAUDE_CODE_API_KEY_FILE_DESCRIPTOR", "")

	token := readCredentialFromFD(
		"CLAUDE_CODE_API_KEY_FILE_DESCRIPTOR",
		keyPath,
		func() *string { return nil },
		func(s string) {},
	)
	if token != "sk-ant-well-known-key" {
		t.Errorf("readCredentialFromFD() = %q, want %q", token, "sk-ant-well-known-key")
	}
}

func TestGetOAuthTokenFromFD_WellKnownFile(t *testing.T) {
	ResetFDTokens()
	defer ResetFDTokens()

	dir := t.TempDir()
	tokenPath := filepath.Join(dir, ".oauth_token")
	os.WriteFile(tokenPath, []byte("oauth-well-known-token"), 0600)

	t.Setenv("CLAUDE_CODE_OAUTH_TOKEN_FILE_DESCRIPTOR", "")

	token := readCredentialFromFD(
		"CLAUDE_CODE_OAUTH_TOKEN_FILE_DESCRIPTOR",
		tokenPath,
		func() *string { return nil },
		func(s string) {},
	)
	if token != "oauth-well-known-token" {
		t.Errorf("readCredentialFromFD() = %q, want %q", token, "oauth-well-known-token")
	}
}

func TestGetCredentialFromFD_InvalidFD(t *testing.T) {
	ResetFDTokens()
	defer ResetFDTokens()

	t.Setenv("CLAUDE_CODE_API_KEY_FILE_DESCRIPTOR", "not-a-number")

	var cached string
	token := readCredentialFromFD(
		"CLAUDE_CODE_API_KEY_FILE_DESCRIPTOR",
		"/nonexistent/path",
		func() *string { return nil },
		func(s string) { cached = s },
	)

	if token != "" {
		t.Errorf("readCredentialFromFD(invalid FD) = %q, want empty", token)
	}
	if cached != "" {
		t.Errorf("cached should be empty for invalid FD, got %q", cached)
	}
}

func TestGetCredentialFromFD_CachedResult(t *testing.T) {
	ResetFDTokens()
	defer ResetFDTokens()

	cachedVal := "cached-token"
	token := readCredentialFromFD(
		"CLAUDE_CODE_API_KEY_FILE_DESCRIPTOR",
		"/nonexistent",
		func() *string { return &cachedVal },
		func(s string) {},
	)
	if token != "cached-token" {
		t.Errorf("readCredentialFromFD(cached) = %q, want %q", token, "cached-token")
	}
}

func TestMaybePersistTokenForSubprocesses(t *testing.T) {
	dir := t.TempDir()
	tokenPath := filepath.Join(dir, "token")

	// Without CLAUDE_CODE_REMOTE, should not write
	t.Setenv("CLAUDE_CODE_REMOTE", "")
	maybePersistTokenForSubprocesses(tokenPath, "secret")
	if _, err := os.Stat(tokenPath); err == nil {
		t.Error("should not write token when CLAUDE_CODE_REMOTE is not set")
	}

	// With CLAUDE_CODE_REMOTE=1, should write
	t.Setenv("CLAUDE_CODE_REMOTE", "1")
	maybePersistTokenForSubprocesses(tokenPath, "secret")
	data, err := os.ReadFile(tokenPath)
	if err != nil {
		t.Fatalf("expected token file to exist: %v", err)
	}
	if string(data) != "secret" {
		t.Errorf("token file content = %q, want %q", string(data), "secret")
	}
}

func TestIsEnvTruthy(t *testing.T) {
	tests := []struct {
		val  string
		want bool
	}{
		{"1", true},
		{"true", true},
		{"TRUE", true},
		{"yes", true},
		{"YES", true},
		{"  true  ", true},
		{"0", false},
		{"false", false},
		{"no", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := isEnvTruthy(tt.val); got != tt.want {
			t.Errorf("isEnvTruthy(%q) = %v, want %v", tt.val, got, tt.want)
		}
	}
}

func TestFDPath(t *testing.T) {
	got := fdPath(42)
	// Just ensure it contains the FD number
	if got != "/dev/fd/42" && got != "/proc/self/fd/42" {
		t.Errorf("fdPath(42) = %q, unexpected", got)
	}
}
