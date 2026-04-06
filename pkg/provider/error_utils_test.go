package provider

import (
	"errors"
	"fmt"
	"net"
	"strings"
	"testing"
)

// Source: services/api/errorUtils.ts

func TestExtractConnectionErrorDetails_NilError(t *testing.T) {
	if d := ExtractConnectionErrorDetails(nil); d != nil {
		t.Errorf("expected nil for nil error, got %+v", d)
	}
}

func TestExtractConnectionErrorDetails_PlainError(t *testing.T) {
	if d := ExtractConnectionErrorDetails(errors.New("something went wrong")); d != nil {
		t.Errorf("expected nil for plain error, got %+v", d)
	}
}

func TestExtractConnectionErrorDetails_SSLErrorInMessage(t *testing.T) {
	err := errors.New("tls: failed to verify certificate - CERT_HAS_EXPIRED")
	d := ExtractConnectionErrorDetails(err)
	if d == nil {
		t.Fatal("expected connection error details for SSL error")
	}
	if !d.IsSSLError {
		t.Error("expected IsSSLError=true")
	}
	if d.Code != "CERT_HAS_EXPIRED" {
		t.Errorf("Code = %q, want CERT_HAS_EXPIRED", d.Code)
	}
}

func TestExtractConnectionErrorDetails_NetOpError(t *testing.T) {
	opErr := &net.OpError{
		Op:  "dial",
		Net: "tcp",
		Err: fmt.Errorf("connection refused"),
	}
	d := ExtractConnectionErrorDetails(opErr)
	if d == nil {
		t.Fatal("expected connection error details for net.OpError")
	}
	if d.IsSSLError {
		t.Error("expected IsSSLError=false for dial error")
	}
}

func TestGetSSLErrorHint_SSLError(t *testing.T) {
	err := errors.New("UNABLE_TO_VERIFY_LEAF_SIGNATURE: certificate verify failed")
	hint := GetSSLErrorHint(err)
	if hint == "" {
		t.Fatal("expected non-empty hint for SSL error")
	}
	if !strings.Contains(hint, "SSL certificate error") {
		t.Errorf("hint = %q, should contain 'SSL certificate error'", hint)
	}
	if !strings.Contains(hint, "UNABLE_TO_VERIFY_LEAF_SIGNATURE") {
		t.Errorf("hint = %q, should contain error code", hint)
	}
	if !strings.Contains(hint, "/doctor") {
		t.Errorf("hint = %q, should mention /doctor", hint)
	}
}

func TestGetSSLErrorHint_NonSSLError(t *testing.T) {
	hint := GetSSLErrorHint(errors.New("file not found"))
	if hint != "" {
		t.Errorf("expected empty hint for non-SSL error, got %q", hint)
	}
}

func TestSanitizeMessageHTML_NoHTML(t *testing.T) {
	msg := "rate limit exceeded"
	if got := SanitizeMessageHTML(msg); got != msg {
		t.Errorf("got %q, want %q", got, msg)
	}
}

func TestSanitizeMessageHTML_HTMLWithTitle(t *testing.T) {
	// Source: errorUtils.ts:107-116 — CloudFlare error page with title
	msg := `<!DOCTYPE html><html><head><title>503 Service Unavailable</title></head><body>...</body></html>`
	got := SanitizeMessageHTML(msg)
	if got != "503 Service Unavailable" {
		t.Errorf("got %q, want '503 Service Unavailable'", got)
	}
}

func TestSanitizeMessageHTML_HTMLNoTitle(t *testing.T) {
	msg := `<html><body>error</body></html>`
	got := SanitizeMessageHTML(msg)
	if got != "" {
		t.Errorf("got %q, want empty string for HTML without title", got)
	}
}

func TestSanitizeAPIErrorMessage_Empty(t *testing.T) {
	if got := SanitizeAPIErrorMessage(""); got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

func TestFormatAPIErrorForUser_NilError(t *testing.T) {
	got := FormatAPIErrorForUser(nil)
	if got != "unknown API error" {
		t.Errorf("got %q, want 'unknown API error'", got)
	}
}

func TestFormatAPIErrorForUser_ConnectionError(t *testing.T) {
	// Source: errorUtils.ts:237-243
	err := &APIError{StatusCode: 0, Message: "Connection error."}
	got := FormatAPIErrorForUser(err)
	if got != "Unable to connect to API. Check your internet connection" {
		t.Errorf("got %q", got)
	}
}

func TestFormatAPIErrorForUser_EmptyMessage(t *testing.T) {
	// Source: errorUtils.ts:248-252 — fallback for missing message
	err := &APIError{StatusCode: 500, Message: ""}
	got := FormatAPIErrorForUser(err)
	if got != "API error (status 500)" {
		t.Errorf("got %q, want 'API error (status 500)'", got)
	}
}

func TestFormatAPIErrorForUser_PlainMessage(t *testing.T) {
	err := &APIError{StatusCode: 429, Message: "rate limit exceeded"}
	got := FormatAPIErrorForUser(err)
	if got != "rate limit exceeded" {
		t.Errorf("got %q, want 'rate limit exceeded'", got)
	}
}

func TestFormatAPIErrorForUser_HTMLMessage(t *testing.T) {
	err := &APIError{
		StatusCode: 503,
		Message:    `<!DOCTYPE html><html><head><title>Service Temporarily Unavailable</title></head></html>`,
	}
	got := FormatAPIErrorForUser(err)
	if got != "Service Temporarily Unavailable" {
		t.Errorf("got %q, want 'Service Temporarily Unavailable'", got)
	}
}

func TestFormatAPIErrorForUser_SSLVerifyFailed(t *testing.T) {
	// Source: errorUtils.ts:216-218
	tests := []struct {
		code string
		want string
	}{
		{"CERT_HAS_EXPIRED", "Unable to connect to API: SSL certificate has expired"},
		{"CERT_REVOKED", "Unable to connect to API: SSL certificate has been revoked"},
		{"DEPTH_ZERO_SELF_SIGNED_CERT", "Unable to connect to API: Self-signed certificate detected. Check your proxy or corporate SSL certificates"},
		{"CERT_NOT_YET_VALID", "Unable to connect to API: SSL certificate is not yet valid"},
		{"ERR_TLS_CERT_ALTNAME_INVALID", "Unable to connect to API: SSL certificate hostname mismatch"},
	}

	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			// Create an APIError whose message contains the SSL code so
			// ExtractConnectionErrorDetails picks it up.
			err := &APIError{
				StatusCode: 0,
				Message:    fmt.Sprintf("tls: %s", tt.code),
			}
			got := FormatAPIErrorForUser(err)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSSLErrorCodes_KnownSet(t *testing.T) {
	// Source: errorUtils.ts:5-29 — ensure all TS codes are present
	expected := []string{
		"UNABLE_TO_VERIFY_LEAF_SIGNATURE",
		"UNABLE_TO_GET_ISSUER_CERT",
		"UNABLE_TO_GET_ISSUER_CERT_LOCALLY",
		"CERT_SIGNATURE_FAILURE",
		"CERT_NOT_YET_VALID",
		"CERT_HAS_EXPIRED",
		"CERT_REVOKED",
		"CERT_REJECTED",
		"CERT_UNTRUSTED",
		"DEPTH_ZERO_SELF_SIGNED_CERT",
		"SELF_SIGNED_CERT_IN_CHAIN",
		"CERT_CHAIN_TOO_LONG",
		"PATH_LENGTH_EXCEEDED",
		"ERR_TLS_CERT_ALTNAME_INVALID",
		"HOSTNAME_MISMATCH",
		"ERR_TLS_HANDSHAKE_TIMEOUT",
		"ERR_SSL_WRONG_VERSION_NUMBER",
		"ERR_SSL_DECRYPTION_FAILED_OR_BAD_RECORD_MAC",
	}
	for _, code := range expected {
		if !sslErrorCodes[code] {
			t.Errorf("missing SSL error code: %s", code)
		}
	}
	if len(sslErrorCodes) != len(expected) {
		t.Errorf("sslErrorCodes has %d entries, expected %d", len(sslErrorCodes), len(expected))
	}
}
