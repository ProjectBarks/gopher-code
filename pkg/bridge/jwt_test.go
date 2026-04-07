package bridge

import (
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Helpers — craft JWT tokens for tests
// ---------------------------------------------------------------------------

// makeJWT builds a JWT (header.payload.signature) from a payload map.
// The signature segment is a dummy — we never verify it.
func makeJWT(t *testing.T, payload map[string]any) string {
	t.Helper()
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	p, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("json.Marshal payload: %v", err)
	}
	body := base64.RawURLEncoding.EncodeToString(p)
	return header + "." + body + ".fakesig"
}

// ---------------------------------------------------------------------------
// DecodeJWTPayload
// ---------------------------------------------------------------------------

func TestDecodeJWTPayload_ValidToken(t *testing.T) {
	tok := makeJWT(t, map[string]any{
		"exp":        1700000000,
		"sub":        "user-42",
		"org_uuid":   "org-abc-123",
		"scopes":     []string{"read", "write"},
		"session_id": "sess-xyz",
	})
	m := DecodeJWTPayload(tok)
	if m == nil {
		t.Fatal("expected non-nil payload")
	}
	if m["sub"] != "user-42" {
		t.Errorf("sub = %v, want user-42", m["sub"])
	}
	if m["org_uuid"] != "org-abc-123" {
		t.Errorf("org_uuid = %v, want org-abc-123", m["org_uuid"])
	}
	if m["session_id"] != "sess-xyz" {
		t.Errorf("session_id = %v, want sess-xyz", m["session_id"])
	}
}

func TestDecodeJWTPayload_StripsSkAntSiPrefix(t *testing.T) {
	tok := "sk-ant-si-" + makeJWT(t, map[string]any{"sub": "user-99"})
	m := DecodeJWTPayload(tok)
	if m == nil {
		t.Fatal("expected non-nil payload after prefix strip")
	}
	if m["sub"] != "user-99" {
		t.Errorf("sub = %v, want user-99", m["sub"])
	}
}

func TestDecodeJWTPayload_InvalidTokens(t *testing.T) {
	cases := []struct {
		name  string
		token string
	}{
		{"empty", ""},
		{"no dots", "abcdef"},
		{"one dot", "abc.def"},
		{"four dots", "a.b.c.d"},
		{"empty payload", "header..sig"},
		{"bad base64 payload", "header.!!!invalid!!!.sig"},
		{"non-json payload", "header." + base64.RawURLEncoding.EncodeToString([]byte("not json")) + ".sig"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if m := DecodeJWTPayload(tc.token); m != nil {
				t.Errorf("expected nil, got %v", m)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// DecodeJWTClaims
// ---------------------------------------------------------------------------

func TestDecodeJWTClaims_AllFields(t *testing.T) {
	exp := int64(1700000000)
	tok := makeJWT(t, map[string]any{
		"exp":        exp,
		"sub":        "user-42",
		"org_uuid":   "org-abc-123",
		"scopes":     []string{"read", "write"},
		"session_id": "sess-xyz",
	})
	c := DecodeJWTClaims(tok)
	if c == nil {
		t.Fatal("expected non-nil claims")
	}
	if c.Exp == nil || *c.Exp != exp {
		t.Errorf("Exp = %v, want %d", c.Exp, exp)
	}
	if c.Sub != "user-42" {
		t.Errorf("Sub = %q, want user-42", c.Sub)
	}
	if c.OrgUUID != "org-abc-123" {
		t.Errorf("OrgUUID = %q, want org-abc-123", c.OrgUUID)
	}
	if len(c.Scopes) != 2 || c.Scopes[0] != "read" || c.Scopes[1] != "write" {
		t.Errorf("Scopes = %v, want [read write]", c.Scopes)
	}
	if c.SessionID != "sess-xyz" {
		t.Errorf("SessionID = %q, want sess-xyz", c.SessionID)
	}
}

func TestDecodeJWTClaims_MissingFields(t *testing.T) {
	tok := makeJWT(t, map[string]any{"sub": "only-sub"})
	c := DecodeJWTClaims(tok)
	if c == nil {
		t.Fatal("expected non-nil claims even with partial payload")
	}
	if c.Exp != nil {
		t.Errorf("Exp should be nil, got %d", *c.Exp)
	}
	if c.Sub != "only-sub" {
		t.Errorf("Sub = %q, want only-sub", c.Sub)
	}
	if c.OrgUUID != "" {
		t.Errorf("OrgUUID should be empty, got %q", c.OrgUUID)
	}
}

func TestDecodeJWTClaims_Malformed(t *testing.T) {
	if c := DecodeJWTClaims("garbage"); c != nil {
		t.Errorf("expected nil for malformed token, got %+v", c)
	}
}

// ---------------------------------------------------------------------------
// DecodeJWTExpiry
// ---------------------------------------------------------------------------

func TestDecodeJWTExpiry_ValidExp(t *testing.T) {
	tok := makeJWT(t, map[string]any{"exp": 1700000000})
	exp, ok := DecodeJWTExpiry(tok)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if exp != 1700000000 {
		t.Errorf("exp = %d, want 1700000000", exp)
	}
}

func TestDecodeJWTExpiry_NoExp(t *testing.T) {
	tok := makeJWT(t, map[string]any{"sub": "user"})
	_, ok := DecodeJWTExpiry(tok)
	if ok {
		t.Error("expected ok=false for token without exp")
	}
}

func TestDecodeJWTExpiry_NonNumericExp(t *testing.T) {
	tok := makeJWT(t, map[string]any{"exp": "not-a-number"})
	_, ok := DecodeJWTExpiry(tok)
	if ok {
		t.Error("expected ok=false for non-numeric exp")
	}
}

func TestDecodeJWTExpiry_WithPrefix(t *testing.T) {
	tok := "sk-ant-si-" + makeJWT(t, map[string]any{"exp": 1234567890})
	exp, ok := DecodeJWTExpiry(tok)
	if !ok {
		t.Fatal("expected ok=true with prefix")
	}
	if exp != 1234567890 {
		t.Errorf("exp = %d, want 1234567890", exp)
	}
}

// ---------------------------------------------------------------------------
// IsJWTExpired / IsJWTExpiredAt
// ---------------------------------------------------------------------------

func TestIsJWTExpiredAt_FutureToken(t *testing.T) {
	// Token expires 1 hour from now — should NOT be expired.
	futureExp := time.Now().Add(time.Hour).Unix()
	tok := makeJWT(t, map[string]any{"exp": futureExp})
	if IsJWTExpired(tok) {
		t.Error("token with future exp should not be expired")
	}
}

func TestIsJWTExpiredAt_PastToken(t *testing.T) {
	// Token expired 1 hour ago — should be expired.
	pastExp := time.Now().Add(-time.Hour).Unix()
	tok := makeJWT(t, map[string]any{"exp": pastExp})
	if !IsJWTExpired(tok) {
		t.Error("token with past exp should be expired")
	}
}

func TestIsJWTExpiredAt_WithinClockSkew(t *testing.T) {
	// Token expired 10 seconds ago — within the 30s tolerance, NOT expired.
	recentExp := time.Now().Add(-10 * time.Second).Unix()
	tok := makeJWT(t, map[string]any{"exp": recentExp})
	if IsJWTExpired(tok) {
		t.Error("token expired within clock skew tolerance should not be expired")
	}
}

func TestIsJWTExpiredAt_JustPastSkew(t *testing.T) {
	// Token expired 31 seconds ago — past tolerance, IS expired.
	ref := time.Now()
	pastExp := ref.Add(-31 * time.Second).Unix()
	tok := makeJWT(t, map[string]any{"exp": pastExp})
	if !IsJWTExpiredAt(tok, ref) {
		t.Error("token expired past clock skew tolerance should be expired")
	}
}

func TestIsJWTExpiredAt_Undecodable(t *testing.T) {
	if !IsJWTExpired("garbage-token") {
		t.Error("undecodable token should be treated as expired")
	}
}

func TestIsJWTExpiredAt_DeterministicClock(t *testing.T) {
	// Deterministic: exp=1000, clock=1025 (within 30s skew) → not expired.
	tok := makeJWT(t, map[string]any{"exp": 1000})
	at := time.Unix(1025, 0)
	if IsJWTExpiredAt(tok, at) {
		t.Error("at=1025, exp=1000 is within 30s skew, should not be expired")
	}
	// clock=1031 → past skew → expired.
	at2 := time.Unix(1031, 0)
	if !IsJWTExpiredAt(tok, at2) {
		t.Error("at=1031, exp=1000 is past 30s skew, should be expired")
	}
}

// ---------------------------------------------------------------------------
// TokenPrefix
// ---------------------------------------------------------------------------

func TestTokenPrefix_Long(t *testing.T) {
	got := TokenPrefix("abcdefghijklmnopqrstuvwxyz")
	want := "abcdefghijklmno…"
	if got != want {
		t.Errorf("TokenPrefix = %q, want %q", got, want)
	}
}

func TestTokenPrefix_Short(t *testing.T) {
	got := TokenPrefix("abc")
	want := "abc…"
	if got != want {
		t.Errorf("TokenPrefix = %q, want %q", got, want)
	}
}

func TestTokenPrefix_Exactly15(t *testing.T) {
	got := TokenPrefix("123456789012345")
	want := "123456789012345…"
	if got != want {
		t.Errorf("TokenPrefix = %q, want %q", got, want)
	}
}

// ---------------------------------------------------------------------------
// FormatDuration
// ---------------------------------------------------------------------------

func TestFormatDuration(t *testing.T) {
	cases := []struct {
		d    time.Duration
		want string
	}{
		{0, "0s"},
		{5 * time.Second, "5s"},
		{59 * time.Second, "59s"},
		{60 * time.Second, "1m"},
		{90 * time.Second, "1m 30s"},
		{150 * time.Second, "2m 30s"},
		{3 * time.Minute, "3m"},
		{5*time.Minute + 1*time.Second, "5m 1s"},
		{30 * time.Minute, "30m"},
	}
	for _, tc := range cases {
		t.Run(tc.want, func(t *testing.T) {
			got := FormatDuration(tc.d)
			if got != tc.want {
				t.Errorf("FormatDuration(%v) = %q, want %q", tc.d, got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Integration: end-to-end JWT parse → validate → check fields
// ---------------------------------------------------------------------------

func TestJWTIntegration_ParseValidateFields(t *testing.T) {
	// Build a realistic bridge JWT with all standard claims.
	exp := time.Now().Add(time.Hour).Unix()
	tok := makeJWT(t, map[string]any{
		"exp":        exp,
		"sub":        "user-integration-1",
		"org_uuid":   "org-int-456",
		"scopes":     []string{"bridge:read", "bridge:write"},
		"session_id": "sess-int-789",
	})

	// Step 1: raw payload decode succeeds.
	payload := DecodeJWTPayload(tok)
	if payload == nil {
		t.Fatal("DecodeJWTPayload returned nil for valid token")
	}

	// Step 2: typed claims decode succeeds with correct values.
	claims := DecodeJWTClaims(tok)
	if claims == nil {
		t.Fatal("DecodeJWTClaims returned nil for valid token")
	}
	if claims.Sub != "user-integration-1" {
		t.Errorf("Sub = %q, want user-integration-1", claims.Sub)
	}
	if claims.OrgUUID != "org-int-456" {
		t.Errorf("OrgUUID = %q, want org-int-456", claims.OrgUUID)
	}
	if claims.SessionID != "sess-int-789" {
		t.Errorf("SessionID = %q, want sess-int-789", claims.SessionID)
	}
	if len(claims.Scopes) != 2 {
		t.Errorf("Scopes len = %d, want 2", len(claims.Scopes))
	}
	if claims.Exp == nil || *claims.Exp != exp {
		t.Errorf("Exp = %v, want %d", claims.Exp, exp)
	}

	// Step 3: expiry check — token is valid (not expired).
	if IsJWTExpired(tok) {
		t.Error("fresh token should not be expired")
	}

	// Step 4: expiry with a fixed clock far in the future → expired.
	futureTime := time.Unix(exp+3600, 0) // 1 hour after expiry
	if !IsJWTExpiredAt(tok, futureTime) {
		t.Error("token should be expired when clock is past exp")
	}

	// Step 5: session-ingress prefix variant.
	prefixed := "sk-ant-si-" + tok
	prefixedClaims := DecodeJWTClaims(prefixed)
	if prefixedClaims == nil {
		t.Fatal("DecodeJWTClaims should handle sk-ant-si- prefix")
	}
	if prefixedClaims.Sub != claims.Sub {
		t.Errorf("prefixed Sub = %q, want %q", prefixedClaims.Sub, claims.Sub)
	}

	// Step 6: TokenPrefix for logging.
	prefix := TokenPrefix(tok)
	if len(prefix) < 16 {
		t.Errorf("TokenPrefix too short: %q", prefix)
	}
}

func TestJWTIntegration_ExpiredTokenRejected(t *testing.T) {
	// Token that expired 5 minutes ago — should be treated as expired.
	exp := time.Now().Add(-5 * time.Minute).Unix()
	tok := makeJWT(t, map[string]any{
		"exp":        exp,
		"sub":        "user-expired",
		"session_id": "sess-old",
	})

	claims := DecodeJWTClaims(tok)
	if claims == nil {
		t.Fatal("should still decode expired token claims")
	}
	if !IsJWTExpired(tok) {
		t.Error("token expired 5m ago should be expired (past 30s tolerance)")
	}
}

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

func TestJWTConstants(t *testing.T) {
	if TokenRefreshBuffer != 5*time.Minute {
		t.Errorf("TokenRefreshBuffer = %v, want 5m", TokenRefreshBuffer)
	}
	if FallbackRefreshInterval != 30*time.Minute {
		t.Errorf("FallbackRefreshInterval = %v, want 30m", FallbackRefreshInterval)
	}
	if MaxRefreshFailures != 3 {
		t.Errorf("MaxRefreshFailures = %d, want 3", MaxRefreshFailures)
	}
	if RefreshRetryDelay != 60*time.Second {
		t.Errorf("RefreshRetryDelay = %v, want 60s", RefreshRetryDelay)
	}
}
