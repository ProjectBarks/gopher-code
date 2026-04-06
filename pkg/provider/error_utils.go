package provider

import (
	"errors"
	"fmt"
	"net"
	"os"
	"regexp"
	"strings"
)

// Source: services/api/errorUtils.ts

// sslErrorCodes is the set of OpenSSL/TLS error codes that indicate an SSL problem.
// Source: errorUtils.ts:5-29
var sslErrorCodes = map[string]bool{
	// Certificate verification errors
	"UNABLE_TO_VERIFY_LEAF_SIGNATURE":              true,
	"UNABLE_TO_GET_ISSUER_CERT":                    true,
	"UNABLE_TO_GET_ISSUER_CERT_LOCALLY":            true,
	"CERT_SIGNATURE_FAILURE":                       true,
	"CERT_NOT_YET_VALID":                           true,
	"CERT_HAS_EXPIRED":                             true,
	"CERT_REVOKED":                                 true,
	"CERT_REJECTED":                                true,
	"CERT_UNTRUSTED":                               true,
	"DEPTH_ZERO_SELF_SIGNED_CERT":                  true,
	"SELF_SIGNED_CERT_IN_CHAIN":                    true,
	"CERT_CHAIN_TOO_LONG":                          true,
	"PATH_LENGTH_EXCEEDED":                         true,
	"ERR_TLS_CERT_ALTNAME_INVALID":                 true,
	"HOSTNAME_MISMATCH":                            true,
	"ERR_TLS_HANDSHAKE_TIMEOUT":                    true,
	"ERR_SSL_WRONG_VERSION_NUMBER":                 true,
	"ERR_SSL_DECRYPTION_FAILED_OR_BAD_RECORD_MAC":  true,
}

// ConnectionErrorDetails holds extracted details from a low-level connection error.
// Source: errorUtils.ts:31-35
type ConnectionErrorDetails struct {
	Code       string
	Message    string
	IsSSLError bool
}

// ExtractConnectionErrorDetails walks the error chain (up to 5 levels) to find
// a network-level error with a code. Returns nil if no connection error found.
// Source: errorUtils.ts:42-83
func ExtractConnectionErrorDetails(err error) *ConnectionErrorDetails {
	if err == nil {
		return nil
	}

	// Check for TLS/certificate errors via string matching on the error message,
	// since Go's tls package doesn't expose typed error codes like Node.js.
	msg := err.Error()
	for code := range sslErrorCodes {
		if strings.Contains(strings.ToUpper(msg), code) {
			return &ConnectionErrorDetails{
				Code:       code,
				Message:    msg,
				IsSSLError: true,
			}
		}
	}

	// Check for DNS / dial / syscall errors via Go's net + os error types.
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		code := ""
		if opErr.Timeout() {
			code = "ETIMEDOUT"
		} else if sysErr := new(os.SyscallError); errors.As(opErr.Err, &sysErr) {
			code = sysErr.Syscall
		} else {
			code = "ECONNREFUSED"
		}
		return &ConnectionErrorDetails{
			Code:    code,
			Message: opErr.Error(),
		}
	}

	// Check for generic timeout
	type timeouter interface{ Timeout() bool }
	if te, ok := err.(timeouter); ok && te.Timeout() {
		return &ConnectionErrorDetails{
			Code:    "ETIMEDOUT",
			Message: err.Error(),
		}
	}

	return nil
}

// GetSSLErrorHint returns an actionable hint for SSL/TLS errors.
// Returns empty string if the error is not SSL-related.
// Source: errorUtils.ts:94-100
func GetSSLErrorHint(err error) string {
	details := ExtractConnectionErrorDetails(err)
	if details == nil || !details.IsSSLError {
		return ""
	}
	return fmt.Sprintf(
		"SSL certificate error (%s). If you are behind a corporate proxy or TLS-intercepting firewall, set NODE_EXTRA_CA_CERTS to your CA bundle path, or ask IT to allowlist *.anthropic.com. Run /doctor for details.",
		details.Code,
	)
}

// htmlTagRe detects HTML content (CloudFlare error pages, etc.).
var htmlTagRe = regexp.MustCompile(`(?i)<!DOCTYPE html|<html`)

// htmlTitleRe extracts the <title> from an HTML page.
var htmlTitleRe = regexp.MustCompile(`<title>([^<]+)</title>`)

// SanitizeMessageHTML strips HTML from error messages, returning the <title>
// text if found, or empty string if HTML detected but no title.
// Returns the original message if no HTML found.
// Source: errorUtils.ts:107-116
func SanitizeMessageHTML(message string) string {
	if !htmlTagRe.MatchString(message) {
		return message
	}
	m := htmlTitleRe.FindStringSubmatch(message)
	if m != nil && len(m) > 1 {
		return strings.TrimSpace(m[1])
	}
	return ""
}

// SanitizeAPIErrorMessage strips HTML from an API error message.
// Returns empty string if the message is empty.
// Source: errorUtils.ts:122-130
func SanitizeAPIErrorMessage(message string) string {
	if message == "" {
		return ""
	}
	return SanitizeMessageHTML(message)
}

// FormatAPIErrorForUser formats an API error into a human-readable string.
// Handles connection errors (timeout, SSL), HTML sanitization, and nested
// error shapes from deserialized JSONL.
// Source: errorUtils.ts:200-260
func FormatAPIErrorForUser(err *APIError) string {
	if err == nil {
		return "unknown API error"
	}

	// Check for connection errors in the underlying cause
	connDetails := ExtractConnectionErrorDetails(err)

	if connDetails != nil {
		// Handle timeout
		if connDetails.Code == "ETIMEDOUT" {
			return "Request timed out. Check your internet connection and proxy settings"
		}

		// Handle SSL/TLS errors with specific messages
		// Source: errorUtils.ts:214-234
		if connDetails.IsSSLError {
			switch connDetails.Code {
			case "UNABLE_TO_VERIFY_LEAF_SIGNATURE",
				"UNABLE_TO_GET_ISSUER_CERT",
				"UNABLE_TO_GET_ISSUER_CERT_LOCALLY":
				return "Unable to connect to API: SSL certificate verification failed. Check your proxy or corporate SSL certificates"
			case "CERT_HAS_EXPIRED":
				return "Unable to connect to API: SSL certificate has expired"
			case "CERT_REVOKED":
				return "Unable to connect to API: SSL certificate has been revoked"
			case "DEPTH_ZERO_SELF_SIGNED_CERT", "SELF_SIGNED_CERT_IN_CHAIN":
				return "Unable to connect to API: Self-signed certificate detected. Check your proxy or corporate SSL certificates"
			case "ERR_TLS_CERT_ALTNAME_INVALID", "HOSTNAME_MISMATCH":
				return "Unable to connect to API: SSL certificate hostname mismatch"
			case "CERT_NOT_YET_VALID":
				return "Unable to connect to API: SSL certificate is not yet valid"
			default:
				return fmt.Sprintf("Unable to connect to API: SSL error (%s)", connDetails.Code)
			}
		}
	}

	// Connection error without SSL
	if err.Message == "Connection error." {
		if connDetails != nil && connDetails.Code != "" {
			return fmt.Sprintf("Unable to connect to API (%s)", connDetails.Code)
		}
		return "Unable to connect to API. Check your internet connection"
	}

	// No message — fallback
	if err.Message == "" {
		return fmt.Sprintf("API error (status %d)", err.StatusCode)
	}

	// Sanitize HTML (CloudFlare pages, etc.)
	sanitized := SanitizeAPIErrorMessage(err.Message)
	if sanitized != err.Message && sanitized != "" {
		return sanitized
	}

	return err.Message
}
