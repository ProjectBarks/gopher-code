package bridge

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// In-memory keyring stub
// ---------------------------------------------------------------------------

type memKeyring struct {
	mu   sync.Mutex
	data map[string]string
}

func newMemKeyring() *memKeyring {
	return &memKeyring{data: make(map[string]string)}
}

func (k *memKeyring) key(service, user string) string { return service + "/" + user }

func (k *memKeyring) Get(service, user string) (string, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	v, ok := k.data[k.key(service, user)]
	if !ok {
		return "", errKeyringNotFound
	}
	return v, nil
}

func (k *memKeyring) Set(service, user, password string) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.data[k.key(service, user)] = password
	return nil
}

func (k *memKeyring) Delete(service, user string) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	delete(k.data, k.key(service, user))
	return nil
}

// sentinel for "not found"
var errKeyringNotFound = fmt.Errorf("secret not found in keyring")

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func gateOn(_ string, _ bool) bool  { return true }
func gateOff(_ string, _ bool) bool { return false }

func blockingGateOn(_ string) (bool, error)  { return true, nil }
func blockingGateOff(_ string) (bool, error) { return false, nil }

func baseDeps(kr *memKeyring) TrustedDeviceDeps {
	return TrustedDeviceDeps{
		Keyring:             kr,
		GetFeatureValueBool: gateOn,
		CheckGateBlocking:   blockingGateOn,
		GetAccessToken:      func() (string, bool) { return "test-oauth-token", true },
		GetBaseAPIURL:       func() string { return "https://api.example.com" },
		IsEssentialTrafficOnly: func() bool { return false },
		Hostname:            "test-host",
	}
}

// ---------------------------------------------------------------------------
// Test: token cache hit — second GetToken does NOT hit keyring again
// ---------------------------------------------------------------------------

func TestGetToken_CacheHit(t *testing.T) {
	kr := newMemKeyring()
	_ = kr.Set(keyringService, keyringUser, "cached-token-abc")

	callCount := 0
	countingKr := &countingKeyring{inner: kr, getCalls: &callCount}

	deps := baseDeps(kr)
	deps.Keyring = countingKr
	m := NewTrustedDeviceManager(deps)

	// First call — should read from keyring.
	tok1 := m.GetToken()
	if tok1 != "cached-token-abc" {
		t.Fatalf("first GetToken: got %q, want %q", tok1, "cached-token-abc")
	}
	if callCount != 1 {
		t.Fatalf("expected 1 keyring Get call, got %d", callCount)
	}

	// Second call — should hit in-memory cache, NOT keyring.
	tok2 := m.GetToken()
	if tok2 != "cached-token-abc" {
		t.Fatalf("second GetToken: got %q, want %q", tok2, "cached-token-abc")
	}
	if callCount != 1 {
		t.Fatalf("expected still 1 keyring Get call after cache hit, got %d", callCount)
	}
}

func TestGetToken_CacheClearForcesKeyringRead(t *testing.T) {
	kr := newMemKeyring()
	_ = kr.Set(keyringService, keyringUser, "token-v1")

	callCount := 0
	countingKr := &countingKeyring{inner: kr, getCalls: &callCount}

	deps := baseDeps(kr)
	deps.Keyring = countingKr
	m := NewTrustedDeviceManager(deps)

	_ = m.GetToken()
	if callCount != 1 {
		t.Fatalf("expected 1 keyring call, got %d", callCount)
	}

	// Update keyring behind the scenes and clear cache.
	_ = kr.Set(keyringService, keyringUser, "token-v2")
	m.ClearCache()

	tok := m.GetToken()
	if tok != "token-v2" {
		t.Fatalf("after ClearCache: got %q, want %q", tok, "token-v2")
	}
	if callCount != 2 {
		t.Fatalf("expected 2 keyring calls after cache clear, got %d", callCount)
	}
}

func TestGetToken_EnvVarTakesPrecedence(t *testing.T) {
	kr := newMemKeyring()
	_ = kr.Set(keyringService, keyringUser, "keyring-token")

	deps := baseDeps(kr)
	m := NewTrustedDeviceManager(deps)

	t.Setenv(envTrustedDeviceToken, "env-override-token")

	tok := m.GetToken()
	if tok != "env-override-token" {
		t.Fatalf("got %q, want %q", tok, "env-override-token")
	}
}

func TestGetToken_GateOff_ReturnsEmpty(t *testing.T) {
	kr := newMemKeyring()
	_ = kr.Set(keyringService, keyringUser, "some-token")

	deps := baseDeps(kr)
	deps.GetFeatureValueBool = gateOff
	m := NewTrustedDeviceManager(deps)

	tok := m.GetToken()
	if tok != "" {
		t.Fatalf("expected empty token when gate is off, got %q", tok)
	}
}

// ---------------------------------------------------------------------------
// Test: enrollment request format
// ---------------------------------------------------------------------------

func TestEnroll_RequestFormat(t *testing.T) {
	kr := newMemKeyring()

	var capturedReq enrollmentRequest
	var capturedAuth string
	var capturedContentType string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedAuth = r.Header.Get("Authorization")
		capturedContentType = r.Header.Get("Content-Type")
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &capturedReq)
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(enrollmentResponse{
			DeviceToken: "new-device-token",
			DeviceID:    "device-123",
		})
	}))
	defer srv.Close()

	deps := baseDeps(kr)
	deps.GetBaseAPIURL = func() string { return srv.URL }
	deps.HTTPClient = srv.Client()
	m := NewTrustedDeviceManager(deps)

	m.Enroll(context.Background())

	// Check Authorization header.
	if capturedAuth != "Bearer test-oauth-token" {
		t.Fatalf("Authorization header: got %q, want %q", capturedAuth, "Bearer test-oauth-token")
	}

	// Check Content-Type.
	if capturedContentType != "application/json" {
		t.Fatalf("Content-Type: got %q, want %q", capturedContentType, "application/json")
	}

	// Check display_name format: "Claude Code on <host> · <platform>"
	expectedPrefix := "Claude Code on test-host \u00b7 "
	if len(capturedReq.DisplayName) < len(expectedPrefix) ||
		capturedReq.DisplayName[:len(expectedPrefix)] != expectedPrefix {
		t.Fatalf("display_name: got %q, want prefix %q", capturedReq.DisplayName, expectedPrefix)
	}

	// Check token was persisted to keyring.
	stored, err := kr.Get(keyringService, keyringUser)
	if err != nil {
		t.Fatalf("keyring Get after enrollment: %v", err)
	}
	if stored != "new-device-token" {
		t.Fatalf("persisted token: got %q, want %q", stored, "new-device-token")
	}
}

func TestEnroll_PostsToCorrectPath(t *testing.T) {
	kr := newMemKeyring()

	var capturedPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(enrollmentResponse{DeviceToken: "tok", DeviceID: "id"})
	}))
	defer srv.Close()

	deps := baseDeps(kr)
	deps.GetBaseAPIURL = func() string { return srv.URL }
	deps.HTTPClient = srv.Client()
	m := NewTrustedDeviceManager(deps)

	m.Enroll(context.Background())

	if capturedPath != "/api/auth/trusted_devices" {
		t.Fatalf("POST path: got %q, want %q", capturedPath, "/api/auth/trusted_devices")
	}
}

func TestEnroll_MissingDeviceToken_DoesNotPersist(t *testing.T) {
	kr := newMemKeyring()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		// Response with empty device_token.
		_ = json.NewEncoder(w).Encode(map[string]string{"device_id": "id-only"})
	}))
	defer srv.Close()

	deps := baseDeps(kr)
	deps.GetBaseAPIURL = func() string { return srv.URL }
	deps.HTTPClient = srv.Client()
	m := NewTrustedDeviceManager(deps)

	m.Enroll(context.Background())

	// Token should NOT be in keyring.
	_, err := kr.Get(keyringService, keyringUser)
	if err == nil {
		t.Fatal("expected no keyring entry when device_token is missing in response")
	}
}

func TestEnroll_GateOff_Skips(t *testing.T) {
	kr := newMemKeyring()
	serverCalled := false

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serverCalled = true
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	deps := baseDeps(kr)
	deps.CheckGateBlocking = blockingGateOff
	deps.GetBaseAPIURL = func() string { return srv.URL }
	deps.HTTPClient = srv.Client()
	m := NewTrustedDeviceManager(deps)

	m.Enroll(context.Background())

	if serverCalled {
		t.Fatal("enrollment should not call server when gate is off")
	}
}

func TestEnroll_NoOAuthToken_Skips(t *testing.T) {
	kr := newMemKeyring()
	serverCalled := false

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serverCalled = true
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	deps := baseDeps(kr)
	deps.GetAccessToken = func() (string, bool) { return "", false }
	deps.GetBaseAPIURL = func() string { return srv.URL }
	deps.HTTPClient = srv.Client()
	m := NewTrustedDeviceManager(deps)

	m.Enroll(context.Background())

	if serverCalled {
		t.Fatal("enrollment should not call server when no OAuth token")
	}
}

func TestEnroll_EssentialTrafficOnly_Skips(t *testing.T) {
	kr := newMemKeyring()
	serverCalled := false

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serverCalled = true
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	deps := baseDeps(kr)
	deps.IsEssentialTrafficOnly = func() bool { return true }
	deps.GetBaseAPIURL = func() string { return srv.URL }
	deps.HTTPClient = srv.Client()
	m := NewTrustedDeviceManager(deps)

	m.Enroll(context.Background())

	if serverCalled {
		t.Fatal("enrollment should not call server in essential-traffic-only mode")
	}
}

func TestEnroll_EnvVarSet_SkipsEnrollment(t *testing.T) {
	kr := newMemKeyring()
	serverCalled := false

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serverCalled = true
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	t.Setenv(envTrustedDeviceToken, "env-token")

	deps := baseDeps(kr)
	deps.GetBaseAPIURL = func() string { return srv.URL }
	deps.HTTPClient = srv.Client()
	m := NewTrustedDeviceManager(deps)

	m.Enroll(context.Background())

	if serverCalled {
		t.Fatal("enrollment should be skipped when env var is set")
	}
}

// ---------------------------------------------------------------------------
// Test: expiry-triggered refresh — mock HTTP returns new token after re-enroll
// ---------------------------------------------------------------------------

func TestEnroll_RefreshesToken_AfterReenrollment(t *testing.T) {
	kr := newMemKeyring()

	// Simulate an existing (possibly expired) token in keyring.
	_ = kr.Set(keyringService, keyringUser, "old-token-expired")

	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(enrollmentResponse{
			DeviceToken: "fresh-token-after-refresh",
			DeviceID:    "device-456",
		})
	}))
	defer srv.Close()

	deps := baseDeps(kr)
	deps.GetBaseAPIURL = func() string { return srv.URL }
	deps.HTTPClient = srv.Client()
	m := NewTrustedDeviceManager(deps)

	// Verify old token is readable.
	tok := m.GetToken()
	if tok != "old-token-expired" {
		t.Fatalf("pre-enroll token: got %q, want %q", tok, "old-token-expired")
	}

	// Re-enroll (simulates /login re-enrollment on every login).
	m.Enroll(context.Background())

	if callCount != 1 {
		t.Fatalf("expected 1 enrollment call, got %d", callCount)
	}

	// After enrollment, cache was cleared — GetToken should return the new token.
	tok = m.GetToken()
	if tok != "fresh-token-after-refresh" {
		t.Fatalf("post-enroll token: got %q, want %q", tok, "fresh-token-after-refresh")
	}

	// Verify keyring was updated.
	stored, _ := kr.Get(keyringService, keyringUser)
	if stored != "fresh-token-after-refresh" {
		t.Fatalf("keyring token: got %q, want %q", stored, "fresh-token-after-refresh")
	}
}

func TestEnroll_ServerError_DoesNotOverwriteExistingToken(t *testing.T) {
	kr := newMemKeyring()
	_ = kr.Set(keyringService, keyringUser, "existing-good-token")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"internal server error"}`))
	}))
	defer srv.Close()

	deps := baseDeps(kr)
	deps.GetBaseAPIURL = func() string { return srv.URL }
	deps.HTTPClient = srv.Client()
	m := NewTrustedDeviceManager(deps)

	m.Enroll(context.Background())

	// Existing token should remain untouched.
	stored, _ := kr.Get(keyringService, keyringUser)
	if stored != "existing-good-token" {
		t.Fatalf("keyring token: got %q, want %q", stored, "existing-good-token")
	}
}

// ---------------------------------------------------------------------------
// Test: Header injection
// ---------------------------------------------------------------------------

func TestHeader_ReturnsTokenWhenAvailable(t *testing.T) {
	kr := newMemKeyring()
	_ = kr.Set(keyringService, keyringUser, "header-token")

	deps := baseDeps(kr)
	m := NewTrustedDeviceManager(deps)

	key, val := m.Header()
	if key != "X-Trusted-Device-Token" {
		t.Fatalf("header key: got %q, want %q", key, "X-Trusted-Device-Token")
	}
	if val != "header-token" {
		t.Fatalf("header value: got %q, want %q", val, "header-token")
	}
}

func TestHeader_EmptyWhenGateOff(t *testing.T) {
	kr := newMemKeyring()
	_ = kr.Set(keyringService, keyringUser, "header-token")

	deps := baseDeps(kr)
	deps.GetFeatureValueBool = gateOff
	m := NewTrustedDeviceManager(deps)

	key, val := m.Header()
	if key != "" || val != "" {
		t.Fatalf("expected empty header when gate off, got (%q, %q)", key, val)
	}
}

// ---------------------------------------------------------------------------
// Test: ClearToken removes from keyring and cache
// ---------------------------------------------------------------------------

func TestClearToken_RemovesFromKeyringAndCache(t *testing.T) {
	kr := newMemKeyring()
	_ = kr.Set(keyringService, keyringUser, "to-be-cleared")

	deps := baseDeps(kr)
	m := NewTrustedDeviceManager(deps)

	// Warm cache.
	_ = m.GetToken()

	m.ClearToken()

	// Keyring should be empty.
	_, err := kr.Get(keyringService, keyringUser)
	if err == nil {
		t.Fatal("expected keyring entry to be deleted")
	}

	// GetToken should return empty (cache cleared + keyring empty).
	tok := m.GetToken()
	if tok != "" {
		t.Fatalf("expected empty token after ClearToken, got %q", tok)
	}
}

// ---------------------------------------------------------------------------
// Test: error body truncation
// ---------------------------------------------------------------------------

func TestEnroll_ErrorBodyTruncation(t *testing.T) {
	kr := newMemKeyring()

	// Build a body longer than 200 chars.
	longBody := make([]byte, 300)
	for i := range longBody {
		longBody[i] = 'x'
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write(longBody)
	}))
	defer srv.Close()

	deps := baseDeps(kr)
	deps.GetBaseAPIURL = func() string { return srv.URL }
	deps.HTTPClient = srv.Client()
	m := NewTrustedDeviceManager(deps)

	// This should not panic and should log a truncated body.
	// We're verifying the code path executes without error.
	m.Enroll(context.Background())
}

// ---------------------------------------------------------------------------
// Counting keyring wrapper (for cache-hit assertions)
// ---------------------------------------------------------------------------

type countingKeyring struct {
	inner    *memKeyring
	getCalls *int
}

func (c *countingKeyring) Get(service, user string) (string, error) {
	*c.getCalls++
	return c.inner.Get(service, user)
}

func (c *countingKeyring) Set(service, user, password string) error {
	return c.inner.Set(service, user, password)
}

func (c *countingKeyring) Delete(service, user string) error {
	return c.inner.Delete(service, user)
}

// Suppress unused import warnings.
var _ = time.Now
