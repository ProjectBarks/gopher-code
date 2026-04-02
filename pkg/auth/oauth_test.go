package auth

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"
)

// Source: services/oauth/crypto.ts, services/oauth/auth-code-listener.ts

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
}
