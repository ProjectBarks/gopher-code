// Package handlers — BrowserOAuthFlow implements the OAuthFlowStarter interface
// using the OAuth primitives from pkg/auth (PKCE, AuthCodeListener, token exchange).
//
// Source: services/oauth/client.ts — oauthLogin flow
package handlers

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"time"

	"github.com/projectbarks/gopher-code/pkg/auth"
)

// BrowserOAuthFlow implements OAuthFlowStarter by opening the system browser
// for the OAuth authorization flow and listening for the callback locally.
//
// Source: services/oauth/client.ts — oauthLogin
type BrowserOAuthFlow struct {
	// OpenBrowser is the function used to open a URL in the browser.
	// Defaults to the system browser opener. Override for testing.
	OpenBrowser func(url string) error

	// CallbackTimeout is how long to wait for the OAuth callback.
	// Defaults to 5 minutes.
	CallbackTimeout time.Duration
}

// NewBrowserOAuthFlow creates a BrowserOAuthFlow with default settings.
func NewBrowserOAuthFlow() *BrowserOAuthFlow {
	return &BrowserOAuthFlow{
		OpenBrowser:     openSystemBrowser,
		CallbackTimeout: 5 * time.Minute,
	}
}

// StartOAuthFlow implements OAuthFlowStarter. It:
// 1. Resolves the OAuth environment config
// 2. Generates PKCE parameters
// 3. Starts a local callback listener
// 4. Opens the browser to the authorization URL
// 5. Waits for the authorization code
// 6. Exchanges the code for tokens
// 7. Fetches the user profile and stores account info
//
// Source: services/oauth/client.ts — oauthLogin
func (f *BrowserOAuthFlow) StartOAuthFlow(email string, scopes []string, useClaudeAI bool) (*auth.OAuthTokens, error) {
	// 1. Resolve OAuth config for current environment
	envCfg, err := auth.GetOAuthConfig()
	if err != nil {
		return nil, fmt.Errorf("resolve OAuth config: %w", err)
	}

	// 2. Generate PKCE parameters
	pkce := auth.NewPKCEParams()

	// 3. Start local callback listener
	listener := auth.NewAuthCodeListener()
	port, err := listener.Start()
	if err != nil {
		return nil, fmt.Errorf("start callback listener: %w", err)
	}
	defer listener.Stop()

	redirectURI := fmt.Sprintf("http://127.0.0.1:%d/callback", port)

	// 4. Build auth URL and choose the right authorize endpoint
	authorizeURL := envCfg.ConsoleAuthorizeURL
	if useClaudeAI {
		authorizeURL = envCfg.ClaudeAIAuthorizeURL
	}

	oauthCfg := auth.OAuthConfig{
		AuthURL:     authorizeURL,
		TokenURL:    envCfg.TokenURL,
		ClientID:    envCfg.ClientID,
		RedirectURI: redirectURI,
		Scopes:      scopes,
	}

	authURL := auth.BuildAuthURL(oauthCfg, pkce)

	// Open browser
	openFn := f.OpenBrowser
	if openFn == nil {
		openFn = openSystemBrowser
	}
	if err := openFn(authURL); err != nil {
		return nil, fmt.Errorf("open browser: %w", err)
	}

	// 5. Wait for authorization code with state validation
	timeout := f.CallbackTimeout
	if timeout == 0 {
		timeout = 5 * time.Minute
	}
	code, err := listener.WaitForCodeWithState(context.Background(), timeout, pkce.State)
	if err != nil {
		return nil, fmt.Errorf("authorization: %w", err)
	}

	// 6. Exchange code for tokens
	tokens, err := auth.ExchangeCodeForTokens(auth.ExchangeCodeParams{
		TokenURL:     envCfg.TokenURL,
		ClientID:     envCfg.ClientID,
		Code:         code,
		CodeVerifier: pkce.CodeVerifier,
		RedirectURI:  redirectURI,
		State:        pkce.State,
	})
	if err != nil {
		return nil, fmt.Errorf("token exchange: %w", err)
	}

	// 7. Fetch profile and store account info
	profile, err := auth.FetchOAuthProfileFromOAuthToken(envCfg.BaseAPIURL, tokens.AccessToken)
	if err != nil {
		// Profile fetch is best-effort — don't fail the login
		return tokens, nil
	}

	subType := auth.ResolveSubscriptionType(profile.Organization.OrganizationType)
	StoreAccountInfo(
		profile.Account.Email,
		profile.Account.UUID,
		profile.Organization.UUID,
		"", // org name not in profile response
		subType,
	)

	return tokens, nil
}

// openSystemBrowser opens a URL in the default system browser.
func openSystemBrowser(url string) error {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", url).Start()
	case "linux":
		return exec.Command("xdg-open", url).Start()
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	default:
		return fmt.Errorf("unsupported platform %s", runtime.GOOS)
	}
}
