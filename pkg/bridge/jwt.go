// JWT decoding helpers for bridge session tokens.
// Source: src/bridge/jwtUtils.ts
//
// These routines decode JWTs without verifying signatures — the API server
// is responsible for signature verification. We only need to read claims
// from trusted tokens (e.g. to schedule proactive refresh).
package bridge

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

// TokenRefreshBuffer is how long before expiry to request a new token.
const TokenRefreshBuffer = 5 * time.Minute // 300 000 ms

// FallbackRefreshInterval is the follow-up refresh when expiry is unknown.
const FallbackRefreshInterval = 30 * time.Minute // 1 800 000 ms

// MaxRefreshFailures caps consecutive refresh failures per session.
const MaxRefreshFailures = 3

// RefreshRetryDelay is the wait between retries when getAccessToken is nil.
const RefreshRetryDelay = 60 * time.Second // 60 000 ms

// sessionIngressPrefix is stripped from session-ingress tokens before decoding.
const sessionIngressPrefix = "sk-ant-si-"

// clockSkewTolerance is the tolerance applied when checking token expiry.
// Tokens within this window of their exp claim are still considered valid.
const clockSkewTolerance = 30 * time.Second

// ---------------------------------------------------------------------------
// JWTClaims
// ---------------------------------------------------------------------------

// JWTClaims holds the decoded claims we care about from a bridge JWT.
// Fields are pointers or zero-valued where the claim may be absent.
type JWTClaims struct {
	Exp       *int64   `json:"exp,omitempty"`
	Sub       string   `json:"sub,omitempty"`
	OrgUUID   string   `json:"org_uuid,omitempty"`
	Scopes    []string `json:"scopes,omitempty"`
	SessionID string   `json:"session_id,omitempty"`
}

// ---------------------------------------------------------------------------
// Decode helpers
// ---------------------------------------------------------------------------

// DecodeJWTPayload decodes a JWT's payload segment without verifying the
// signature. It strips the "sk-ant-si-" session-ingress prefix if present.
// Returns nil if the token is malformed or the payload is not valid JSON.
func DecodeJWTPayload(token string) map[string]any {
	jwt := strings.TrimPrefix(token, sessionIngressPrefix)
	parts := strings.Split(jwt, ".")
	if len(parts) != 3 || parts[1] == "" {
		return nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil
	}
	return m
}

// DecodeJWTClaims decodes the payload into a typed JWTClaims struct.
// Returns nil if the token is not a decodable JWT.
func DecodeJWTClaims(token string) *JWTClaims {
	jwt := strings.TrimPrefix(token, sessionIngressPrefix)
	parts := strings.Split(jwt, ".")
	if len(parts) != 3 || parts[1] == "" {
		return nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil
	}
	var c JWTClaims
	if err := json.Unmarshal(raw, &c); err != nil {
		return nil
	}
	return &c
}

// DecodeJWTExpiry returns the exp claim in Unix seconds, or 0 and false
// if the token is unparseable or has no numeric exp claim.
func DecodeJWTExpiry(token string) (int64, bool) {
	payload := DecodeJWTPayload(token)
	if payload == nil {
		return 0, false
	}
	exp, ok := payload["exp"]
	if !ok {
		return 0, false
	}
	// JSON numbers decode as float64.
	switch v := exp.(type) {
	case float64:
		return int64(v), true
	case json.Number:
		n, err := v.Int64()
		if err != nil {
			return 0, false
		}
		return n, true
	default:
		return 0, false
	}
}

// IsJWTExpired reports whether the token's exp claim is in the past,
// accounting for clockSkewTolerance. Returns true (expired) if the token
// cannot be decoded.
func IsJWTExpired(token string) bool {
	return IsJWTExpiredAt(token, time.Now())
}

// IsJWTExpiredAt reports whether the token's exp claim is before t,
// accounting for clockSkewTolerance. Returns true (expired) if the token
// cannot be decoded.
func IsJWTExpiredAt(token string, t time.Time) bool {
	exp, ok := DecodeJWTExpiry(token)
	if !ok {
		return true
	}
	expTime := time.Unix(exp, 0)
	// Token is expired if current time is past exp + tolerance.
	return t.After(expTime.Add(clockSkewTolerance))
}

// TokenPrefix returns the first 15 characters of a token for logging,
// appending "…" as an ellipsis. Matches the TS `.slice(0, 15)` convention.
func TokenPrefix(token string) string {
	if len(token) <= 15 {
		return token + "…"
	}
	return token[:15] + "…"
}

// FormatDuration formats a duration as a human-readable string matching
// the TS formatDuration output: "5s", "2m 30s", "3m".
func FormatDuration(d time.Duration) string {
	totalSec := int(d.Seconds())
	if totalSec < 0 {
		totalSec = 0
	}
	if totalSec < 60 {
		return fmt.Sprintf("%ds", totalSec)
	}
	m := totalSec / 60
	s := totalSec % 60
	if s > 0 {
		return fmt.Sprintf("%dm %ds", m, s)
	}
	return fmt.Sprintf("%dm", m)
}
