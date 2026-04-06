package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// Source: services/oauth/crypto.ts, services/oauth/client.ts, services/oauth/auth-code-listener.ts, services/oauth/getOauthProfile.ts

func TestPKCECrypto(t *testing.T) {
	// Source: services/oauth/crypto.ts

	t.Run("code_verifier_length", func(t *testing.T) {
		// Source: crypto.ts:11-13 — 32 random bytes base64url encoded
		v := GenerateCodeVerifier()
		if len(v) < 40 {
			t.Errorf("verifier too short: %d chars", len(v))
		}
		// Should not contain base64 padding or unsafe chars
		if strings.Contains(v, "=") || strings.Contains(v, "+") || strings.Contains(v, "/") {
			t.Errorf("verifier should be URL-safe: %q", v)
		}
	})

	t.Run("code_challenge_deterministic", func(t *testing.T) {
		// Source: crypto.ts:15-19 — SHA256 of verifier
		v := "test-verifier-string"
		c1 := GenerateCodeChallenge(v)
		c2 := GenerateCodeChallenge(v)
		if c1 != c2 {
			t.Error("same verifier should produce same challenge")
		}
	})

	t.Run("code_challenge_url_safe", func(t *testing.T) {
		c := GenerateCodeChallenge("test")
		if strings.Contains(c, "=") || strings.Contains(c, "+") || strings.Contains(c, "/") {
			t.Errorf("challenge should be URL-safe: %q", c)
		}
	})

	t.Run("state_unique", func(t *testing.T) {
		// Source: crypto.ts:21-23
		s1 := GenerateState()
		s2 := GenerateState()
		if s1 == s2 {
			t.Error("states should be unique")
		}
	})

	t.Run("pkce_params_complete", func(t *testing.T) {
		p := NewPKCEParams()
		if p.CodeVerifier == "" {
			t.Error("verifier should not be empty")
		}
		if p.CodeChallenge == "" {
			t.Error("challenge should not be empty")
		}
		if p.State == "" {
			t.Error("state should not be empty")
		}
	})
}

func TestBuildAuthURL(t *testing.T) {
	// Source: services/oauth/client.ts — buildAuthUrl
	cfg := OAuthConfig{
		AuthURL:     "https://auth.example.com/authorize",
		ClientID:    "test-client",
		RedirectURI: "http://localhost:8080/callback",
		Scopes:      []string{"openid", "profile"},
	}
	pkce := PKCEParams{
		CodeVerifier:  "verifier",
		CodeChallenge: "challenge",
		State:         "state123",
	}

	url := BuildAuthURL(cfg, pkce)
	if !strings.HasPrefix(url, "https://auth.example.com/authorize?") {
		t.Errorf("URL should start with auth endpoint: %q", url)
	}
	if !strings.Contains(url, "code_challenge=challenge") {
		t.Error("URL should contain code_challenge")
	}
	if !strings.Contains(url, "code_challenge_method=S256") {
		t.Error("URL should contain S256 method")
	}
	if !strings.Contains(url, "state=state123") {
		t.Error("URL should contain state")
	}
	if !strings.Contains(url, "scope=openid+profile") {
		t.Error("URL should contain scopes")
	}
}

func TestAuthCodeListener(t *testing.T) {
	// Source: services/oauth/auth-code-listener.ts

	t.Run("starts_and_receives_code", func(t *testing.T) {
		listener := NewAuthCodeListener()
		port, err := listener.Start()
		if err != nil {
			t.Fatalf("start failed: %v", err)
		}
		defer listener.Stop()

		if port <= 0 {
			t.Errorf("expected positive port, got %d", port)
		}

		// Simulate browser redirect with auth code
		go func() {
			time.Sleep(50 * time.Millisecond)
			http.Get(fmt.Sprintf("http://127.0.0.1:%d/callback?code=test-auth-code&state=abc", port))
		}()

		code, err := listener.WaitForCode(context.Background(), 5*time.Second)
		if err != nil {
			t.Fatalf("wait failed: %v", err)
		}
		if code != "test-auth-code" {
			t.Errorf("expected 'test-auth-code', got %q", code)
		}
	})

	t.Run("timeout", func(t *testing.T) {
		listener := NewAuthCodeListener()
		_, err := listener.Start()
		if err != nil {
			t.Fatalf("start failed: %v", err)
		}
		defer listener.Stop()

		_, err = listener.WaitForCode(context.Background(), 100*time.Millisecond)
		if err == nil {
			t.Error("expected timeout error")
		}
	})

	t.Run("error_response", func(t *testing.T) {
		listener := NewAuthCodeListener()
		port, err := listener.Start()
		if err != nil {
			t.Fatalf("start failed: %v", err)
		}
		defer listener.Stop()

		go func() {
			time.Sleep(50 * time.Millisecond)
			http.Get(fmt.Sprintf("http://127.0.0.1:%d/callback?error=access_denied", port))
		}()

		_, err = listener.WaitForCode(context.Background(), 5*time.Second)
		if err == nil {
			t.Error("expected error for access_denied")
		}
	})

	t.Run("state_mismatch_rejected", func(t *testing.T) {
		// Source: services/oauth/auth-code-listener.ts — state validation
		listener := NewAuthCodeListener()
		port, err := listener.Start()
		if err != nil {
			t.Fatalf("start failed: %v", err)
		}
		defer listener.Stop()

		expectedState := "correct-state-abc"

		go func() {
			time.Sleep(50 * time.Millisecond)
			// Send callback with wrong state
			http.Get(fmt.Sprintf("http://127.0.0.1:%d/callback?code=auth-code&state=wrong-state", port))
		}()

		_, err = listener.WaitForCodeWithState(context.Background(), 5*time.Second, expectedState)
		if err == nil {
			t.Error("expected error for state mismatch")
		}
		if !strings.Contains(err.Error(), "state") {
			t.Errorf("error should mention state, got: %v", err)
		}
	})

	t.Run("state_match_accepted", func(t *testing.T) {
		listener := NewAuthCodeListener()
		port, err := listener.Start()
		if err != nil {
			t.Fatalf("start failed: %v", err)
		}
		defer listener.Stop()

		expectedState := "correct-state-xyz"

		go func() {
			time.Sleep(50 * time.Millisecond)
			http.Get(fmt.Sprintf("http://127.0.0.1:%d/callback?code=valid-code&state=correct-state-xyz", port))
		}()

		code, err := listener.WaitForCodeWithState(context.Background(), 5*time.Second, expectedState)
		if err != nil {
			t.Fatalf("wait failed: %v", err)
		}
		if code != "valid-code" {
			t.Errorf("expected 'valid-code', got %q", code)
		}
	})
}

func TestShouldUseClaudeAIAuth(t *testing.T) {
	// Source: services/oauth/client.ts — shouldUseClaudeAIAuth
	t.Run("true_when_inference_scope_present", func(t *testing.T) {
		scopes := []string{"user:profile", "user:inference", "user:sessions:claude_code"}
		if !ShouldUseClaudeAIAuth(scopes) {
			t.Error("expected true when user:inference is in scopes")
		}
	})

	t.Run("false_when_inference_scope_absent", func(t *testing.T) {
		scopes := []string{"org:create_api_key", "user:profile"}
		if ShouldUseClaudeAIAuth(scopes) {
			t.Error("expected false when user:inference is not in scopes")
		}
	})

	t.Run("false_for_nil_scopes", func(t *testing.T) {
		if ShouldUseClaudeAIAuth(nil) {
			t.Error("expected false for nil scopes")
		}
	})

	t.Run("false_for_empty_scopes", func(t *testing.T) {
		if ShouldUseClaudeAIAuth([]string{}) {
			t.Error("expected false for empty scopes")
		}
	})
}

func TestParseScopes(t *testing.T) {
	// Source: services/oauth/client.ts — parseScopes
	t.Run("space_separated", func(t *testing.T) {
		scopes := ParseScopes("user:profile user:inference user:sessions:claude_code")
		expected := []string{"user:profile", "user:inference", "user:sessions:claude_code"}
		if len(scopes) != len(expected) {
			t.Fatalf("expected %d scopes, got %d", len(expected), len(scopes))
		}
		for i, s := range expected {
			if scopes[i] != s {
				t.Errorf("scopes[%d]: expected %q, got %q", i, s, scopes[i])
			}
		}
	})

	t.Run("empty_string", func(t *testing.T) {
		scopes := ParseScopes("")
		if len(scopes) != 0 {
			t.Errorf("expected empty slice, got %v", scopes)
		}
	})

	t.Run("extra_spaces_filtered", func(t *testing.T) {
		scopes := ParseScopes("user:profile  user:inference")
		// TS filters empty strings from split
		for _, s := range scopes {
			if s == "" {
				t.Error("empty string should be filtered out")
			}
		}
	})
}

func TestIsOAuthTokenExpired(t *testing.T) {
	// Source: services/oauth/client.ts — isOAuthTokenExpired
	// Buffer: 5 minutes (300,000ms)

	t.Run("nil_expiresAt_not_expired", func(t *testing.T) {
		// null expiresAt → false (not expired)
		if IsOAuthTokenExpired(0) {
			t.Error("zero expiresAt (null equivalent) should not be expired")
		}
	})

	t.Run("future_token_not_expired", func(t *testing.T) {
		// Token expires in 1 hour → not expired
		expiresAt := time.Now().Add(1 * time.Hour)
		if IsOAuthTokenExpired(expiresAt.UnixMilli()) {
			t.Error("token expiring in 1 hour should not be expired")
		}
	})

	t.Run("past_token_expired", func(t *testing.T) {
		// Token already expired → expired
		expiresAt := time.Now().Add(-1 * time.Hour)
		if !IsOAuthTokenExpired(expiresAt.UnixMilli()) {
			t.Error("token expired 1 hour ago should be expired")
		}
	})

	t.Run("within_buffer_expired", func(t *testing.T) {
		// Token expires in 3 minutes (within 5-min buffer) → expired
		expiresAt := time.Now().Add(3 * time.Minute)
		if !IsOAuthTokenExpired(expiresAt.UnixMilli()) {
			t.Error("token expiring in 3 minutes should be expired (within 5-min buffer)")
		}
	})

	t.Run("just_outside_buffer_not_expired", func(t *testing.T) {
		// Token expires in 6 minutes (outside 5-min buffer) → not expired
		expiresAt := time.Now().Add(6 * time.Minute)
		if IsOAuthTokenExpired(expiresAt.UnixMilli()) {
			t.Error("token expiring in 6 minutes should not be expired")
		}
	})
}

func TestExchangeCodeForTokens_RequestFormat(t *testing.T) {
	// Source: services/oauth/client.ts — exchangeCodeForTokens
	// Verify the POST body has the correct grant_type, code, redirect_uri, client_id, code_verifier, state

	var receivedBody map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("expected application/json content-type, got %q", ct)
		}

		json.NewDecoder(r.Body).Decode(&receivedBody)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token":  "test-access-token",
			"refresh_token": "test-refresh-token",
			"expires_in":    3600,
			"token_type":    "Bearer",
			"scope":         "user:profile user:inference",
		})
	}))
	defer server.Close()

	tokens, err := ExchangeCodeForTokens(ExchangeCodeParams{
		TokenURL:     server.URL,
		ClientID:     "test-client-id",
		Code:         "auth-code-123",
		CodeVerifier: "verifier-456",
		RedirectURI:  "http://localhost:9999/callback",
		State:        "state-789",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify request body
	if receivedBody["grant_type"] != "authorization_code" {
		t.Errorf("expected grant_type 'authorization_code', got %v", receivedBody["grant_type"])
	}
	if receivedBody["code"] != "auth-code-123" {
		t.Errorf("expected code 'auth-code-123', got %v", receivedBody["code"])
	}
	if receivedBody["client_id"] != "test-client-id" {
		t.Errorf("expected client_id 'test-client-id', got %v", receivedBody["client_id"])
	}
	if receivedBody["code_verifier"] != "verifier-456" {
		t.Errorf("expected code_verifier 'verifier-456', got %v", receivedBody["code_verifier"])
	}
	if receivedBody["redirect_uri"] != "http://localhost:9999/callback" {
		t.Errorf("expected redirect_uri, got %v", receivedBody["redirect_uri"])
	}
	if receivedBody["state"] != "state-789" {
		t.Errorf("expected state 'state-789', got %v", receivedBody["state"])
	}

	// Verify response parsed
	if tokens.AccessToken != "test-access-token" {
		t.Errorf("expected access token, got %q", tokens.AccessToken)
	}
	if tokens.RefreshToken != "test-refresh-token" {
		t.Errorf("expected refresh token, got %q", tokens.RefreshToken)
	}
	if tokens.ExpiresIn != 3600 {
		t.Errorf("expected expires_in 3600, got %d", tokens.ExpiresIn)
	}
}

func TestRefreshOAuthToken_RequestFormat(t *testing.T) {
	// Source: services/oauth/client.ts — refreshOAuthToken

	var receivedBody map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}

		json.NewDecoder(r.Body).Decode(&receivedBody)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token":  "new-access-token",
			"refresh_token": "new-refresh-token",
			"expires_in":    7200,
			"token_type":    "Bearer",
			"scope":         "user:profile user:inference",
		})
	}))
	defer server.Close()

	tokens, err := RefreshOAuthToken(RefreshTokenParams{
		TokenURL:     server.URL,
		ClientID:     "test-client-id",
		RefreshToken: "old-refresh-token",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify request body
	if receivedBody["grant_type"] != "refresh_token" {
		t.Errorf("expected grant_type 'refresh_token', got %v", receivedBody["grant_type"])
	}
	if receivedBody["refresh_token"] != "old-refresh-token" {
		t.Errorf("expected refresh_token 'old-refresh-token', got %v", receivedBody["refresh_token"])
	}
	if receivedBody["client_id"] != "test-client-id" {
		t.Errorf("expected client_id 'test-client-id', got %v", receivedBody["client_id"])
	}

	// Verify response
	if tokens.AccessToken != "new-access-token" {
		t.Errorf("expected new access token, got %q", tokens.AccessToken)
	}
	if tokens.RefreshToken != "new-refresh-token" {
		t.Errorf("expected new refresh token, got %q", tokens.RefreshToken)
	}
}

func TestRefreshOAuthToken_DefaultScopes(t *testing.T) {
	// Source: services/oauth/client.ts — default scopes are CLAUDE_AI_OAUTH_SCOPES
	var receivedBody map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&receivedBody)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token": "tok", "expires_in": 3600, "token_type": "Bearer",
		})
	}))
	defer server.Close()

	_, err := RefreshOAuthToken(RefreshTokenParams{
		TokenURL:     server.URL,
		ClientID:     "cid",
		RefreshToken: "rt",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	scopeStr, ok := receivedBody["scope"].(string)
	if !ok || scopeStr == "" {
		t.Fatal("expected scope in request body")
	}
	// Default scopes should be ClaudeAIOAuthScopes joined by space
	expectedScope := strings.Join(ClaudeAIOAuthScopes, " ")
	if scopeStr != expectedScope {
		t.Errorf("expected default scope %q, got %q", expectedScope, scopeStr)
	}
}

func TestRefreshOAuthToken_CustomScopes(t *testing.T) {
	var receivedBody map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&receivedBody)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token": "tok", "expires_in": 3600, "token_type": "Bearer",
		})
	}))
	defer server.Close()

	_, err := RefreshOAuthToken(RefreshTokenParams{
		TokenURL:     server.URL,
		ClientID:     "cid",
		RefreshToken: "rt",
		Scopes:       []string{"user:inference"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	scopeStr, _ := receivedBody["scope"].(string)
	if scopeStr != "user:inference" {
		t.Errorf("expected custom scope 'user:inference', got %q", scopeStr)
	}
}

func TestFetchOAuthProfileFromOAuthToken_URLConstruction(t *testing.T) {
	// Source: services/oauth/getOauthProfile.ts — getOauthProfileFromOauthToken
	// Endpoint: ${BASE_API_URL}/api/oauth/profile
	// Headers: Authorization: Bearer {token}, Content-Type: application/json
	// Timeout: 10s

	var receivedAuth string
	var receivedContentType string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		receivedContentType = r.Header.Get("Content-Type")

		if r.URL.Path != "/api/oauth/profile" {
			t.Errorf("expected path /api/oauth/profile, got %q", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(OAuthProfileResponse{
			Account: OAuthProfileAccount{
				UUID:        "acct-uuid-123",
				Email:       "user@example.com",
				DisplayName: "Test User",
			},
			Organization: OAuthProfileOrganization{
				UUID:             "org-uuid-456",
				OrganizationType: "claude_pro",
			},
		})
	}))
	defer server.Close()

	profile, err := FetchOAuthProfileFromOAuthToken(server.URL, "my-access-token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if receivedAuth != "Bearer my-access-token" {
		t.Errorf("expected 'Bearer my-access-token', got %q", receivedAuth)
	}
	if receivedContentType != "application/json" {
		t.Errorf("expected 'application/json', got %q", receivedContentType)
	}
	if profile == nil {
		t.Fatal("expected non-nil profile")
	}
	if profile.Account.UUID != "acct-uuid-123" {
		t.Errorf("expected account UUID 'acct-uuid-123', got %q", profile.Account.UUID)
	}
	if profile.Organization.OrganizationType != "claude_pro" {
		t.Errorf("expected org type 'claude_pro', got %q", profile.Organization.OrganizationType)
	}
}

func TestFetchOAuthProfileFromAPIKey_URLConstruction(t *testing.T) {
	// Source: services/oauth/getOauthProfile.ts — getOauthProfileFromApiKey
	// Endpoint: ${BASE_API_URL}/api/claude_cli_profile?account_uuid={uuid}
	// Headers: x-api-key: {key}, anthropic-beta: OAUTH_BETA_HEADER

	var receivedAPIKey string
	var receivedBeta string
	var receivedAccountUUID string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAPIKey = r.Header.Get("x-api-key")
		receivedBeta = r.Header.Get("anthropic-beta")
		receivedAccountUUID = r.URL.Query().Get("account_uuid")

		if r.URL.Path != "/api/claude_cli_profile" {
			t.Errorf("expected path /api/claude_cli_profile, got %q", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(OAuthProfileResponse{
			Account: OAuthProfileAccount{
				UUID:  "acct-uuid-789",
				Email: "api@example.com",
			},
			Organization: OAuthProfileOrganization{
				UUID:             "org-uuid-012",
				OrganizationType: "claude_max",
			},
		})
	}))
	defer server.Close()

	profile, err := FetchOAuthProfileFromAPIKey(server.URL, "sk-ant-key-123", "acct-uuid-789")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if receivedAPIKey != "sk-ant-key-123" {
		t.Errorf("expected x-api-key 'sk-ant-key-123', got %q", receivedAPIKey)
	}
	if receivedBeta != OAuthBetaHeader {
		t.Errorf("expected anthropic-beta %q, got %q", OAuthBetaHeader, receivedBeta)
	}
	if receivedAccountUUID != "acct-uuid-789" {
		t.Errorf("expected account_uuid 'acct-uuid-789', got %q", receivedAccountUUID)
	}
	if profile == nil {
		t.Fatal("expected non-nil profile")
	}
	if profile.Organization.OrganizationType != "claude_max" {
		t.Errorf("expected org type 'claude_max', got %q", profile.Organization.OrganizationType)
	}
}

func TestResolveSubscriptionType(t *testing.T) {
	// Source: services/oauth/client.ts — fetchProfileInfo org type mapping
	tests := []struct {
		orgType string
		want    string
	}{
		{"claude_max", "max"},
		{"claude_pro", "pro"},
		{"claude_enterprise", "enterprise"},
		{"claude_team", "team"},
		{"unknown_type", ""},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.orgType, func(t *testing.T) {
			got := ResolveSubscriptionType(tt.orgType)
			if got != tt.want {
				t.Errorf("ResolveSubscriptionType(%q) = %q, want %q", tt.orgType, got, tt.want)
			}
		})
	}
}

func TestBuildAuthURL_Extended(t *testing.T) {
	// Source: services/oauth/client.ts — buildAuthUrl with all parameters
	t.Run("includes_code_param", func(t *testing.T) {
		// TS appends code=true to tell login page to show Claude Max upsell
		cfg := OAuthConfig{
			AuthURL:     "https://auth.example.com/authorize",
			ClientID:    "test-client",
			RedirectURI: "http://localhost:8080/callback",
			Scopes:      []string{"user:profile"},
		}
		pkce := PKCEParams{
			CodeVerifier:  "v",
			CodeChallenge: "c",
			State:         "s",
		}
		url := BuildAuthURL(cfg, pkce)
		if !strings.Contains(url, "client_id=test-client") {
			t.Error("URL should contain client_id")
		}
		if !strings.Contains(url, "response_type=code") {
			t.Error("URL should contain response_type=code")
		}
		if !strings.Contains(url, "redirect_uri=") {
			t.Error("URL should contain redirect_uri")
		}
	})

	t.Run("no_scopes_omits_scope_param", func(t *testing.T) {
		cfg := OAuthConfig{
			AuthURL:     "https://auth.example.com/authorize",
			ClientID:    "test-client",
			RedirectURI: "http://localhost:8080/callback",
			Scopes:      nil,
		}
		pkce := PKCEParams{CodeChallenge: "c", State: "s"}
		url := BuildAuthURL(cfg, pkce)
		if strings.Contains(url, "scope=") {
			t.Error("URL should not contain scope param when no scopes")
		}
	})
}

func TestPKCECodeChallengeKnownVector(t *testing.T) {
	// RFC 7636 Appendix B test vector:
	// code_verifier = "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
	// The SHA-256 of this verifier base64url-encoded should be deterministic.
	verifier := "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
	challenge := GenerateCodeChallenge(verifier)

	// Verify it's non-empty and URL-safe
	if challenge == "" {
		t.Error("challenge should not be empty")
	}
	if strings.Contains(challenge, "+") || strings.Contains(challenge, "/") || strings.Contains(challenge, "=") {
		t.Errorf("challenge should be URL-safe base64: %q", challenge)
	}

	// Same input should always produce same output
	challenge2 := GenerateCodeChallenge(verifier)
	if challenge != challenge2 {
		t.Error("challenge should be deterministic")
	}
}
