package auth

// Source: utils/authFileDescriptor.ts
// T125: Auth tokens passed via file descriptor for bridge/CCR child processes.

import (
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
)

// Well-known token file locations in CCR.
// Source: utils/authFileDescriptor.ts:20-23
const (
	CCRTokenDir              = "/home/claude/.claude/remote"
	CCROAuthTokenPath        = CCRTokenDir + "/.oauth_token"
	CCRAPIKeyPath            = CCRTokenDir + "/.api_key"
	CCRSessionIngressTokenPath = CCRTokenDir + "/.session_ingress_token"
)

// fdTokens caches tokens read from file descriptors (singleton, like TS STATE).
var fdTokens struct {
	mu                  sync.RWMutex
	sessionIngressToken *string // nil = unread, non-nil = cached (possibly empty)
	oauthTokenFromFD    *string
	apiKeyFromFD        *string
}

// ── Getters / Setters (match TS state.ts getters) ────────────────────

// SessionIngressToken returns the cached session ingress token, or empty if unset.
func SessionIngressToken() string {
	fdTokens.mu.RLock()
	defer fdTokens.mu.RUnlock()
	if fdTokens.sessionIngressToken == nil {
		return ""
	}
	return *fdTokens.sessionIngressToken
}

// SetSessionIngressToken sets the cached session ingress token.
func SetSessionIngressToken(token string) {
	fdTokens.mu.Lock()
	defer fdTokens.mu.Unlock()
	fdTokens.sessionIngressToken = &token
}

// OAuthTokenFromFD returns the cached OAuth token read from a file descriptor.
func OAuthTokenFromFD() string {
	fdTokens.mu.RLock()
	defer fdTokens.mu.RUnlock()
	if fdTokens.oauthTokenFromFD == nil {
		return ""
	}
	return *fdTokens.oauthTokenFromFD
}

// SetOAuthTokenFromFD sets the cached OAuth token from file descriptor.
func SetOAuthTokenFromFD(token string) {
	fdTokens.mu.Lock()
	defer fdTokens.mu.Unlock()
	fdTokens.oauthTokenFromFD = &token
}

// APIKeyFromFD returns the cached API key read from a file descriptor.
func APIKeyFromFD() string {
	fdTokens.mu.RLock()
	defer fdTokens.mu.RUnlock()
	if fdTokens.apiKeyFromFD == nil {
		return ""
	}
	return *fdTokens.apiKeyFromFD
}

// SetAPIKeyFromFD sets the cached API key from file descriptor.
func SetAPIKeyFromFD(token string) {
	fdTokens.mu.Lock()
	defer fdTokens.mu.Unlock()
	fdTokens.apiKeyFromFD = &token
}

// ResetFDTokens clears all cached FD tokens (for testing).
func ResetFDTokens() {
	fdTokens.mu.Lock()
	defer fdTokens.mu.Unlock()
	fdTokens.sessionIngressToken = nil
	fdTokens.oauthTokenFromFD = nil
	fdTokens.apiKeyFromFD = nil
}

// ── FD reading (matches TS getCredentialFromFd) ──────────────────────

// fdPath returns the platform-appropriate path to read from a file descriptor.
// macOS/BSD: /dev/fd/<fd>, Linux: /proc/self/fd/<fd>
// Source: utils/authFileDescriptor.ts:137-139
func fdPath(fd int) string {
	if runtime.GOOS == "darwin" || runtime.GOOS == "freebsd" {
		return fmt.Sprintf("/dev/fd/%d", fd)
	}
	return fmt.Sprintf("/proc/self/fd/%d", fd)
}

// readTokenFromWellKnownFile reads a token from a well-known file path.
// Returns empty string if the file doesn't exist or is empty.
// Source: utils/authFileDescriptor.ts:57-82
func readTokenFromWellKnownFile(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// readCredentialFromFD reads a credential from a file descriptor env var,
// falling back to a well-known file path. The result is cached.
// Source: utils/authFileDescriptor.ts:97-166
func readCredentialFromFD(envVar, wellKnownPath string, getCached func() *string, setCached func(string)) string {
	cached := getCached()
	if cached != nil {
		return *cached
	}

	fdEnv := os.Getenv(envVar)
	if fdEnv == "" {
		// No FD env var — try well-known file (CCR subprocess fallback).
		token := readTokenFromWellKnownFile(wellKnownPath)
		setCached(token)
		return token
	}

	fd, err := strconv.Atoi(fdEnv)
	if err != nil {
		setCached("")
		return ""
	}

	data, err := os.ReadFile(fdPath(fd))
	if err != nil {
		// FD read failed — try well-known file fallback.
		token := readTokenFromWellKnownFile(wellKnownPath)
		setCached(token)
		return token
	}

	token := strings.TrimSpace(string(data))
	if token == "" {
		setCached("")
		return ""
	}

	setCached(token)
	// In CCR, persist to well-known file for subprocesses.
	maybePersistTokenForSubprocesses(wellKnownPath, token)
	return token
}

// maybePersistTokenForSubprocesses writes the token to disk for CCR subprocesses.
// Only writes when CLAUDE_CODE_REMOTE is truthy.
// Source: utils/authFileDescriptor.ts:30-50
func maybePersistTokenForSubprocesses(path, token string) {
	if !isEnvTruthy(os.Getenv("CLAUDE_CODE_REMOTE")) {
		return
	}
	_ = os.MkdirAll(CCRTokenDir, 0700)
	_ = os.WriteFile(path, []byte(token), 0600)
}

// GetOAuthTokenFromFD reads the OAuth token from file descriptor or well-known file.
// Env var: CLAUDE_CODE_OAUTH_TOKEN_FILE_DESCRIPTOR
// Well-known file: /home/claude/.claude/remote/.oauth_token
// Source: utils/authFileDescriptor.ts:173-181
func GetOAuthTokenFromFD() string {
	return readCredentialFromFD(
		"CLAUDE_CODE_OAUTH_TOKEN_FILE_DESCRIPTOR",
		CCROAuthTokenPath,
		func() *string {
			fdTokens.mu.RLock()
			defer fdTokens.mu.RUnlock()
			return fdTokens.oauthTokenFromFD
		},
		func(token string) {
			fdTokens.mu.Lock()
			defer fdTokens.mu.Unlock()
			fdTokens.oauthTokenFromFD = &token
		},
	)
}

// GetAPIKeyFromFD reads the API key from file descriptor or well-known file.
// Env var: CLAUDE_CODE_API_KEY_FILE_DESCRIPTOR
// Well-known file: /home/claude/.claude/remote/.api_key
// Source: utils/authFileDescriptor.ts:188-196
func GetAPIKeyFromFD() string {
	return readCredentialFromFD(
		"CLAUDE_CODE_API_KEY_FILE_DESCRIPTOR",
		CCRAPIKeyPath,
		func() *string {
			fdTokens.mu.RLock()
			defer fdTokens.mu.RUnlock()
			return fdTokens.apiKeyFromFD
		},
		func(token string) {
			fdTokens.mu.Lock()
			defer fdTokens.mu.Unlock()
			fdTokens.apiKeyFromFD = &token
		},
	)
}
