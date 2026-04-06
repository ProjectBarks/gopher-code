// Package product defines Claude Code product URLs and remote-session helpers.
package product

import "strings"

// Product and Claude AI base URLs.
const (
	ProductURL            = "https://claude.com/claude-code"
	ClaudeAIBaseURL       = "https://claude.ai"
	ClaudeAIStagingBaseURL = "https://claude-ai.staging.ant.dev"
	ClaudeAILocalBaseURL  = "http://localhost:4000"
)

// IsRemoteSessionStaging reports whether the session targets a staging environment.
// It checks for "_staging_" in the session ID or "staging" in the ingress URL.
func IsRemoteSessionStaging(sessionID, ingressURL string) bool {
	return strings.Contains(sessionID, "_staging_") ||
		strings.Contains(ingressURL, "staging")
}

// IsRemoteSessionLocal reports whether the session targets a local-dev environment.
// It checks for "_local_" in the session ID or "localhost" in the ingress URL.
func IsRemoteSessionLocal(sessionID, ingressURL string) bool {
	return strings.Contains(sessionID, "_local_") ||
		strings.Contains(ingressURL, "localhost")
}

// GetClaudeAIBaseURL returns the Claude AI base URL for the given environment.
// Local is checked first, then staging, then production (default).
func GetClaudeAIBaseURL(sessionID, ingressURL string) string {
	if IsRemoteSessionLocal(sessionID, ingressURL) {
		return ClaudeAILocalBaseURL
	}
	if IsRemoteSessionStaging(sessionID, ingressURL) {
		return ClaudeAIStagingBaseURL
	}
	return ClaudeAIBaseURL
}

// ToCompatSessionID re-tags a "cse_*" session ID to "session_*" for use with
// the v1 compat API. No-op for IDs that don't start with "cse_".
func ToCompatSessionID(id string) string {
	if !strings.HasPrefix(id, "cse_") {
		return id
	}
	return "session_" + id[len("cse_"):]
}

// ToInfraSessionID re-tags a "session_*" session ID to "cse_*" for
// infrastructure-layer calls. Inverse of ToCompatSessionID.
// No-op for IDs that don't start with "session_".
func ToInfraSessionID(id string) string {
	if !strings.HasPrefix(id, "session_") {
		return id
	}
	return "cse_" + id[len("session_"):]
}

// GetRemoteSessionURL returns the full Claude AI URL for a remote session.
// It converts the session ID to compat form, resolves the base URL, and
// constructs "<base>/code/<compatId>".
func GetRemoteSessionURL(sessionID, ingressURL string) string {
	compatID := ToCompatSessionID(sessionID)
	baseURL := GetClaudeAIBaseURL(compatID, ingressURL)
	return baseURL + "/code/" + compatID
}
