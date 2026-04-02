package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Source: services/oauth/crypto.ts, services/oauth/index.ts, services/oauth/auth-code-listener.ts

// OAuthConfig holds the OAuth 2.0 configuration.
type OAuthConfig struct {
	AuthURL     string // Authorization endpoint
	TokenURL    string // Token exchange endpoint
	ClientID    string
	RedirectURI string // Set dynamically based on listener port
	Scopes      []string
}

// OAuthTokens holds the token response.
// Source: services/oauth/types.ts
type OAuthTokens struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token,omitempty"`
	ExpiresIn    int    `json:"expires_in,omitempty"`
	TokenType    string `json:"token_type"`
	Scope        string `json:"scope,omitempty"`
}

// PKCEParams holds the PKCE challenge parameters.
type PKCEParams struct {
	CodeVerifier  string
	CodeChallenge string
	State         string
}

// GenerateCodeVerifier creates a random PKCE code verifier.
// Source: services/oauth/crypto.ts:11-13
func GenerateCodeVerifier() string {
	b := make([]byte, 32)
	rand.Read(b)
	return base64URLEncode(b)
}

// GenerateCodeChallenge creates a PKCE code challenge from a verifier.
// Source: services/oauth/crypto.ts:15-19
func GenerateCodeChallenge(verifier string) string {
	h := sha256.Sum256([]byte(verifier))
	return base64URLEncode(h[:])
}

// GenerateState creates a random state parameter.
// Source: services/oauth/crypto.ts:21-23
func GenerateState() string {
	b := make([]byte, 32)
	rand.Read(b)
	return base64URLEncode(b)
}

// base64URLEncode encodes bytes as URL-safe base64 without padding.
// Source: services/oauth/crypto.ts:3-9
func base64URLEncode(b []byte) string {
	s := base64.StdEncoding.EncodeToString(b)
	s = strings.ReplaceAll(s, "+", "-")
	s = strings.ReplaceAll(s, "/", "_")
	s = strings.TrimRight(s, "=")
	return s
}

// NewPKCEParams generates a fresh set of PKCE parameters.
func NewPKCEParams() PKCEParams {
	verifier := GenerateCodeVerifier()
	return PKCEParams{
		CodeVerifier:  verifier,
		CodeChallenge: GenerateCodeChallenge(verifier),
		State:         GenerateState(),
	}
}

// BuildAuthURL constructs the OAuth authorization URL.
// Source: services/oauth/client.ts — buildAuthUrl
func BuildAuthURL(cfg OAuthConfig, pkce PKCEParams) string {
	params := url.Values{
		"response_type":         {"code"},
		"client_id":             {cfg.ClientID},
		"redirect_uri":          {cfg.RedirectURI},
		"state":                 {pkce.State},
		"code_challenge":        {pkce.CodeChallenge},
		"code_challenge_method": {"S256"},
	}
	if len(cfg.Scopes) > 0 {
		params.Set("scope", strings.Join(cfg.Scopes, " "))
	}
	return cfg.AuthURL + "?" + params.Encode()
}

// AuthCodeListener listens for the OAuth callback on localhost.
// Source: services/oauth/auth-code-listener.ts
type AuthCodeListener struct {
	server   *http.Server
	listener net.Listener
	codeCh   chan string
	errCh    chan error
}

// NewAuthCodeListener creates a new callback listener.
func NewAuthCodeListener() *AuthCodeListener {
	return &AuthCodeListener{
		codeCh: make(chan string, 1),
		errCh:  make(chan error, 1),
	}
}

// Start begins listening on a random port. Returns the port.
// Source: services/oauth/auth-code-listener.ts — start()
func (l *AuthCodeListener) Start() (int, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, fmt.Errorf("listen: %w", err)
	}
	l.listener = listener
	port := listener.Addr().(*net.TCPAddr).Port

	mux := http.NewServeMux()
	mux.HandleFunc("/callback", l.handleCallback)

	l.server = &http.Server{Handler: mux}
	go l.server.Serve(listener)

	return port, nil
}

// handleCallback processes the OAuth redirect.
func (l *AuthCodeListener) handleCallback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	if code == "" {
		errMsg := r.URL.Query().Get("error")
		if errMsg == "" {
			errMsg = "no authorization code received"
		}
		l.errCh <- fmt.Errorf("OAuth error: %s", errMsg)
		http.Error(w, "Authorization failed", http.StatusBadRequest)
		return
	}

	l.codeCh <- code
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprint(w, `<html><body><h1>Authorization successful!</h1><p>You can close this window and return to the terminal.</p></body></html>`)
}

// WaitForCode waits for the authorization code with a timeout.
func (l *AuthCodeListener) WaitForCode(ctx context.Context, timeout time.Duration) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	select {
	case code := <-l.codeCh:
		return code, nil
	case err := <-l.errCh:
		return "", err
	case <-ctx.Done():
		return "", fmt.Errorf("OAuth authorization timed out")
	}
}

// Stop shuts down the listener.
func (l *AuthCodeListener) Stop() {
	if l.server != nil {
		l.server.Close()
	}
}

// Port returns the port the listener is running on.
func (l *AuthCodeListener) Port() int {
	if l.listener == nil {
		return 0
	}
	return l.listener.Addr().(*net.TCPAddr).Port
}
