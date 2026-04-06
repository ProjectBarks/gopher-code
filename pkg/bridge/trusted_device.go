// Package bridge — trusted device token management.
// Source: src/bridge/trustedDevice.ts
package bridge

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/zalando/go-keyring"
)

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

const (
	// TrustedDeviceGate is the GrowthBook feature flag controlling trusted-device
	// token enrollment and header injection.
	TrustedDeviceGate = "tengu_sessions_elevated_auth_enforcement"

	// envTrustedDeviceToken is the env var that overrides keychain storage for
	// testing, canary, and enterprise-wrapper scenarios.
	envTrustedDeviceToken = "CLAUDE_TRUSTED_DEVICE_TOKEN"

	// keyringService is the keyring service name for trusted-device tokens.
	keyringService = "claude-code"

	// keyringUser is the keyring username key for the trusted-device token.
	keyringUser = "trusted-device-token"

	// enrollmentTimeout is the HTTP timeout for the enrollment POST.
	enrollmentTimeout = 10 * time.Second

	// errBodyTruncate is the max chars of error body to include in log messages.
	errBodyTruncate = 200
)

// ---------------------------------------------------------------------------
// Dependency injection for testability
// ---------------------------------------------------------------------------

// KeyringStore abstracts OS-native credential storage so tests can supply
// an in-memory stub instead of hitting the real keyring.
type KeyringStore interface {
	Get(service, user string) (string, error)
	Set(service, user, password string) error
	Delete(service, user string) error
}

// osKeyring is the default production keyring backed by go-keyring.
type osKeyring struct{}

func (osKeyring) Get(service, user string) (string, error)          { return keyring.Get(service, user) }
func (osKeyring) Set(service, user, password string) error          { return keyring.Set(service, user, password) }
func (osKeyring) Delete(service, user string) error                 { return keyring.Delete(service, user) }

// EssentialTrafficFunc returns true when only essential traffic should be sent
// (privacy/diagnostic mode). When true, enrollment is skipped.
type EssentialTrafficFunc func() bool

// TrustedDeviceDeps holds injectable dependencies for trusted-device management.
type TrustedDeviceDeps struct {
	// Keyring is the credential store. Defaults to osKeyring (go-keyring).
	Keyring KeyringStore

	// GetFeatureValueBool returns a cached (possibly stale) boolean feature value.
	GetFeatureValueBool func(key string, defaultVal bool) bool

	// CheckGateBlocking returns a fresh boolean gate value (may block).
	CheckGateBlocking func(key string) (bool, error)

	// GetAccessToken returns the current OAuth access token and ok=true,
	// or ("", false) when not logged in.
	GetAccessToken AccessTokenFunc

	// GetBaseAPIURL returns the base API URL for enrollment.
	GetBaseAPIURL BaseURLFunc

	// IsEssentialTrafficOnly returns true when enrollment should be skipped.
	IsEssentialTrafficOnly EssentialTrafficFunc

	// Hostname overrides os.Hostname for display_name (test seam).
	Hostname string

	// HTTPClient overrides the default HTTP client (test seam).
	HTTPClient *http.Client

	// Now returns the current time (test seam for expiry tests).
	Now func() time.Time
}

// ---------------------------------------------------------------------------
// TrustedDeviceManager
// ---------------------------------------------------------------------------

// TrustedDeviceManager manages trusted-device token enrollment, caching,
// retrieval, and header injection for bridge API calls.
type TrustedDeviceManager struct {
	deps TrustedDeviceDeps

	mu          sync.RWMutex
	cachedToken string
	cacheValid  bool
}

// NewTrustedDeviceManager creates a manager with the given dependencies.
func NewTrustedDeviceManager(deps TrustedDeviceDeps) *TrustedDeviceManager {
	if deps.Keyring == nil {
		deps.Keyring = osKeyring{}
	}
	if deps.HTTPClient == nil {
		deps.HTTPClient = &http.Client{Timeout: enrollmentTimeout}
	}
	if deps.Now == nil {
		deps.Now = time.Now
	}
	return &TrustedDeviceManager{deps: deps}
}

// isGateEnabled checks the cached (possibly stale) gate value.
func (m *TrustedDeviceManager) isGateEnabled() bool {
	if m.deps.GetFeatureValueBool == nil {
		return false
	}
	return m.deps.GetFeatureValueBool(TrustedDeviceGate, false)
}

// ---------------------------------------------------------------------------
// Token retrieval (env var → in-memory cache → keyring)
// ---------------------------------------------------------------------------

// GetToken returns the trusted-device token if the gate is enabled.
// Precedence: env var > in-memory cache > keyring.
// Returns "" when the gate is off or no token is available.
func (m *TrustedDeviceManager) GetToken() string {
	if !m.isGateEnabled() {
		return ""
	}
	return m.readStoredToken()
}

// readStoredToken implements the memoized token read.
// Env var wins; otherwise check in-memory cache; finally fall back to keyring.
func (m *TrustedDeviceManager) readStoredToken() string {
	// Env var always takes precedence.
	if envTok := os.Getenv(envTrustedDeviceToken); envTok != "" {
		return envTok
	}

	// Fast path: in-memory cache.
	m.mu.RLock()
	if m.cacheValid {
		tok := m.cachedToken
		m.mu.RUnlock()
		return tok
	}
	m.mu.RUnlock()

	// Slow path: read from keyring (~40ms on macOS).
	tok, err := m.deps.Keyring.Get(keyringService, keyringUser)
	if err != nil {
		// keyring.ErrNotFound or other OS error → no token stored.
		tok = ""
	}

	m.mu.Lock()
	m.cachedToken = tok
	m.cacheValid = true
	m.mu.Unlock()

	return tok
}

// Header returns the X-Trusted-Device-Token header key/value pair for
// injection into bridge API requests, or ("", "") when no token is available.
func (m *TrustedDeviceManager) Header() (key, value string) {
	tok := m.GetToken()
	if tok == "" {
		return "", ""
	}
	return "X-Trusted-Device-Token", tok
}

// ---------------------------------------------------------------------------
// Cache management
// ---------------------------------------------------------------------------

// ClearCache invalidates the in-memory memoized token. The next call to
// GetToken will re-read from keyring (or env var).
func (m *TrustedDeviceManager) ClearCache() {
	m.mu.Lock()
	m.cachedToken = ""
	m.cacheValid = false
	m.mu.Unlock()
}

// ClearToken removes the token from both keyring storage and the in-memory
// cache. Called before enrollment during /login so a stale token from a
// previous account isn't sent while enrollment is in-flight.
func (m *TrustedDeviceManager) ClearToken() {
	if !m.isGateEnabled() {
		return
	}
	_ = m.deps.Keyring.Delete(keyringService, keyringUser)
	m.ClearCache()
}

// ---------------------------------------------------------------------------
// Enrollment
// ---------------------------------------------------------------------------

// enrollmentRequest is the JSON body POSTed to /api/auth/trusted_devices.
type enrollmentRequest struct {
	DisplayName string `json:"display_name"`
}

// enrollmentResponse is the JSON body returned from enrollment.
type enrollmentResponse struct {
	DeviceToken string `json:"device_token,omitempty"`
	DeviceID    string `json:"device_id,omitempty"`
}

// Enroll enrolls this device via POST /api/auth/trusted_devices and persists
// the token to keyring. Best-effort: logs and returns on failure so callers
// (post-login hooks) don't block the login flow.
func (m *TrustedDeviceManager) Enroll(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			debugLog("[trusted-device] Enrollment error: %v", r)
		}
	}()

	// Blocking gate check (awaits GrowthBook refresh after login).
	if m.deps.CheckGateBlocking != nil {
		enabled, err := m.deps.CheckGateBlocking(TrustedDeviceGate)
		if err != nil || !enabled {
			debugLog("[trusted-device] Gate %s is off, skipping enrollment", TrustedDeviceGate)
			return
		}
	} else if !m.isGateEnabled() {
		debugLog("[trusted-device] Gate %s is off, skipping enrollment", TrustedDeviceGate)
		return
	}

	// Skip if env var is set — it takes precedence, so an enrolled token
	// would be shadowed and never used.
	if os.Getenv(envTrustedDeviceToken) != "" {
		debugLog("[trusted-device] CLAUDE_TRUSTED_DEVICE_TOKEN env var is set, skipping enrollment (env var takes precedence)")
		return
	}

	// Need an OAuth access token.
	accessToken := ""
	if m.deps.GetAccessToken != nil {
		var ok bool
		accessToken, ok = m.deps.GetAccessToken()
		if !ok || accessToken == "" {
			debugLog("[trusted-device] No OAuth token, skipping enrollment")
			return
		}
	} else {
		debugLog("[trusted-device] No OAuth token, skipping enrollment")
		return
	}

	// Essential-traffic-only mode check.
	if m.deps.IsEssentialTrafficOnly != nil && m.deps.IsEssentialTrafficOnly() {
		debugLog("[trusted-device] Essential traffic only, skipping enrollment")
		return
	}

	// Build display name: "Claude Code on <hostname> · <platform>"
	host := m.deps.Hostname
	if host == "" {
		h, err := os.Hostname()
		if err != nil {
			h = "unknown"
		}
		host = h
	}
	displayName := fmt.Sprintf("Claude Code on %s \u00b7 %s", host, runtime.GOOS)

	// Build base URL.
	baseURL := ""
	if m.deps.GetBaseAPIURL != nil {
		baseURL = m.deps.GetBaseAPIURL()
	}
	enrollURL := baseURL + "/api/auth/trusted_devices"

	// Marshal request body.
	body, err := json.Marshal(enrollmentRequest{DisplayName: displayName})
	if err != nil {
		debugLog("[trusted-device] Enrollment error: %v", err)
		return
	}

	// POST enrollment.
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, enrollURL, bytes.NewReader(body))
	if err != nil {
		debugLog("[trusted-device] Enrollment error: %v", err)
		return
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := m.deps.HTTPClient.Do(req)
	if err != nil {
		debugLog("[trusted-device] Enrollment request failed: %v", err)
		return
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		truncated := string(respBody)
		if len(truncated) > errBodyTruncate {
			truncated = truncated[:errBodyTruncate]
		}
		debugLog("[trusted-device] Enrollment failed %d: %s", resp.StatusCode, truncated)
		return
	}

	var enrollResp enrollmentResponse
	if err := json.Unmarshal(respBody, &enrollResp); err != nil {
		debugLog("[trusted-device] Enrollment error: %v", err)
		return
	}

	if enrollResp.DeviceToken == "" {
		debugLog("[trusted-device] Enrollment response missing device_token field")
		return
	}

	// Persist to keyring.
	if err := m.deps.Keyring.Set(keyringService, keyringUser, enrollResp.DeviceToken); err != nil {
		debugLog("[trusted-device] Storage write failed: %v", err)
		return
	}

	// Invalidate cache so next GetToken() picks up the new token.
	m.ClearCache()

	deviceID := enrollResp.DeviceID
	if deviceID == "" {
		deviceID = "unknown"
	}
	debugLog("[trusted-device] Enrolled device_id=%s", deviceID)
}

// debugLog logs a debug message. Uses log.Printf for now; can be wired to
// a structured logger later.
func debugLog(format string, args ...any) {
	log.Printf(format, args...)
}
