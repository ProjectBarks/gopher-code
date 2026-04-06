package bridge

import (
	"testing"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// setEnvs sets env vars and returns a cleanup function that restores originals.
func setEnvs(t *testing.T, envs map[string]string) {
	t.Helper()
	for k, v := range envs {
		t.Setenv(k, v)
	}
}

func stubDeps(token string, tokenOK bool, baseURL string) ConfigDeps {
	return ConfigDeps{
		GetAccessToken: func() (string, bool) { return token, tokenOK },
		GetBaseAPIURL:  func() string { return baseURL },
	}
}

// ---------------------------------------------------------------------------
// BridgeTokenOverride
// ---------------------------------------------------------------------------

func TestBridgeTokenOverride_AntWithToken(t *testing.T) {
	setEnvs(t, map[string]string{
		envUserType:         userTypeAnt,
		envBridgeOAuthToken: "dev-token-123",
	})

	tok, ok := BridgeTokenOverride()
	if !ok {
		t.Fatal("expected ok=true")
	}
	if tok != "dev-token-123" {
		t.Fatalf("got %q, want %q", tok, "dev-token-123")
	}
}

func TestBridgeTokenOverride_AntWithoutToken(t *testing.T) {
	setEnvs(t, map[string]string{
		envUserType: userTypeAnt,
	})

	_, ok := BridgeTokenOverride()
	if ok {
		t.Fatal("expected ok=false when CLAUDE_BRIDGE_OAUTH_TOKEN is unset")
	}
}

func TestBridgeTokenOverride_NonAntIgnoresToken(t *testing.T) {
	setEnvs(t, map[string]string{
		envUserType:         "external",
		envBridgeOAuthToken: "should-be-ignored",
	})

	_, ok := BridgeTokenOverride()
	if ok {
		t.Fatal("expected ok=false for non-ant user")
	}
}

func TestBridgeTokenOverride_NoUserType(t *testing.T) {
	// USER_TYPE not set at all — default env is empty string
	setEnvs(t, map[string]string{
		envBridgeOAuthToken: "should-be-ignored",
	})

	_, ok := BridgeTokenOverride()
	if ok {
		t.Fatal("expected ok=false when USER_TYPE is unset")
	}
}

// ---------------------------------------------------------------------------
// BridgeBaseURLOverride
// ---------------------------------------------------------------------------

func TestBridgeBaseURLOverride_AntWithURL(t *testing.T) {
	setEnvs(t, map[string]string{
		envUserType:     userTypeAnt,
		envBridgeBaseURL: "https://dev.example.com",
	})

	u, ok := BridgeBaseURLOverride()
	if !ok {
		t.Fatal("expected ok=true")
	}
	if u != "https://dev.example.com" {
		t.Fatalf("got %q, want %q", u, "https://dev.example.com")
	}
}

func TestBridgeBaseURLOverride_AntWithoutURL(t *testing.T) {
	setEnvs(t, map[string]string{
		envUserType: userTypeAnt,
	})

	_, ok := BridgeBaseURLOverride()
	if ok {
		t.Fatal("expected ok=false when CLAUDE_BRIDGE_BASE_URL is unset")
	}
}

func TestBridgeBaseURLOverride_NonAntIgnoresURL(t *testing.T) {
	setEnvs(t, map[string]string{
		envUserType:     "external",
		envBridgeBaseURL: "https://dev.example.com",
	})

	_, ok := BridgeBaseURLOverride()
	if ok {
		t.Fatal("expected ok=false for non-ant user")
	}
}

// ---------------------------------------------------------------------------
// BridgeAccessToken — override > keychain fallback
// ---------------------------------------------------------------------------

func TestBridgeAccessToken_OverrideWins(t *testing.T) {
	setEnvs(t, map[string]string{
		envUserType:         userTypeAnt,
		envBridgeOAuthToken: "override-tok",
	})
	deps := stubDeps("keychain-tok", true, "")

	tok, ok := BridgeAccessToken(deps)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if tok != "override-tok" {
		t.Fatalf("got %q, want %q (override should win)", tok, "override-tok")
	}
}

func TestBridgeAccessToken_FallsBackToKeychain(t *testing.T) {
	// No override set — should fall through to keychain.
	deps := stubDeps("keychain-tok", true, "")

	tok, ok := BridgeAccessToken(deps)
	if !ok {
		t.Fatal("expected ok=true from keychain")
	}
	if tok != "keychain-tok" {
		t.Fatalf("got %q, want %q", tok, "keychain-tok")
	}
}

func TestBridgeAccessToken_NoTokenAnywhere(t *testing.T) {
	deps := stubDeps("", false, "")

	_, ok := BridgeAccessToken(deps)
	if ok {
		t.Fatal("expected ok=false when no token is available")
	}
}

func TestBridgeAccessToken_NilDeps(t *testing.T) {
	_, ok := BridgeAccessToken(ConfigDeps{})
	if ok {
		t.Fatal("expected ok=false with nil deps")
	}
}

// ---------------------------------------------------------------------------
// BridgeBaseURL — override > production fallback
// ---------------------------------------------------------------------------

func TestBridgeBaseURL_OverrideWins(t *testing.T) {
	setEnvs(t, map[string]string{
		envUserType:     userTypeAnt,
		envBridgeBaseURL: "https://dev-api.example.com",
	})
	deps := stubDeps("", false, "https://prod.example.com")

	u := BridgeBaseURL(deps)
	if u != "https://dev-api.example.com" {
		t.Fatalf("got %q, want override URL", u)
	}
}

func TestBridgeBaseURL_FallsBackToConfig(t *testing.T) {
	deps := stubDeps("", false, "https://prod.example.com")

	u := BridgeBaseURL(deps)
	if u != "https://prod.example.com" {
		t.Fatalf("got %q, want %q", u, "https://prod.example.com")
	}
}

func TestBridgeBaseURL_FallsBackToHardcodedDefault(t *testing.T) {
	u := BridgeBaseURL(ConfigDeps{})
	if u != "https://api.anthropic.com" {
		t.Fatalf("got %q, want production default", u)
	}
}

// ---------------------------------------------------------------------------
// Env-var constant values
// ---------------------------------------------------------------------------

func TestEnvVarConstants(t *testing.T) {
	// Ensure the env var names match the TS source verbatim.
	if envUserType != "USER_TYPE" {
		t.Errorf("envUserType = %q", envUserType)
	}
	if envBridgeOAuthToken != "CLAUDE_BRIDGE_OAUTH_TOKEN" {
		t.Errorf("envBridgeOAuthToken = %q", envBridgeOAuthToken)
	}
	if envBridgeBaseURL != "CLAUDE_BRIDGE_BASE_URL" {
		t.Errorf("envBridgeBaseURL = %q", envBridgeBaseURL)
	}
	if userTypeAnt != "ant" {
		t.Errorf("userTypeAnt = %q", userTypeAnt)
	}
}
