package auth

import (
	"os"
	"strings"
)

// Source: utils/auth.ts — auth utility functions
//
// Subscription detection, API key source, token source, provider detection.

// SubscriptionType identifies the user's Claude subscription tier.
// Source: services/oauth/types.ts
type SubscriptionType string

const (
	SubFree             SubscriptionType = "free"
	SubPro              SubscriptionType = "pro"
	SubMax5             SubscriptionType = "max_5"
	SubMax20            SubscriptionType = "max_20"
	SubTeam             SubscriptionType = "team"
	SubTeamPremium      SubscriptionType = "team_premium"
	SubEnterprise       SubscriptionType = "enterprise"
	SubInternalEmployee SubscriptionType = "internal_employee"
)

// AuthTokenSource identifies where the auth token came from.
// Source: utils/auth.ts — getAuthTokenSource
type AuthTokenSource string

const (
	SourceEnvVar         AuthTokenSource = "env_var"
	SourceKeyring        AuthTokenSource = "keyring"
	SourceFile           AuthTokenSource = "file"
	SourceOAuth          AuthTokenSource = "oauth"
	SourceFileDescriptor AuthTokenSource = "file_descriptor"
	SourceAPIKeyHelper   AuthTokenSource = "api_key_helper"
	SourceNone           AuthTokenSource = "none"
)

// APIKeyWithSource returns the API key and where it was resolved from.
// Source: utils/auth.ts — getAnthropicApiKeyWithSource
func APIKeyWithSource() (key string, source AuthTokenSource) {
	if k := os.Getenv("ANTHROPIC_API_KEY"); k != "" {
		return k, SourceEnvVar
	}
	if k := os.Getenv("ANTHROPIC_AUTH_TOKEN"); k != "" {
		return k, SourceEnvVar
	}
	// Try file descriptor tokens (CCR/Desktop)
	if k := GetAPIKeyFromFD(); k != "" {
		return k, SourceFileDescriptor
	}
	// OAuth token
	if k := GetOAuthTokenFromFD(); k != "" {
		return k, SourceOAuth
	}
	// Keyring / file
	if k, err := GetAPIKey(); err == nil {
		// Determine if it came from keyring or file
		return k, SourceKeyring // simplified — GetAPIKey tries keyring first
	}
	return "", SourceNone
}

// GetOAuthToken returns the stored OAuth token, if any.
func GetOAuthToken() string {
	return GetOAuthTokenFromFD()
}

// IsClaudeAISubscriber returns true if the user has a Claude.ai subscription.
// Source: utils/auth.ts — isClaudeAISubscriber
func IsClaudeAISubscriber() bool {
	sub := GetSubscriptionType()
	return sub != "" && sub != SubFree
}

// Is1PApiCustomer returns true if the user has a 1P API key (not OAuth).
// Source: utils/auth.ts — is1PApiCustomer
func Is1PApiCustomer() bool {
	_, source := APIKeyWithSource()
	return source == SourceEnvVar || source == SourceKeyring || source == SourceFile
}

// IsMaxSubscriber returns true if the user has a Max subscription.
// Source: utils/auth.ts — isMaxSubscriber
func IsMaxSubscriber() bool {
	sub := GetSubscriptionType()
	return sub == SubMax5 || sub == SubMax20
}

// IsTeamSubscriber returns true for team-tier subscriptions.
func IsTeamSubscriber() bool {
	sub := GetSubscriptionType()
	return sub == SubTeam || sub == SubTeamPremium
}

// IsEnterpriseSubscriber returns true for enterprise subscriptions.
func IsEnterpriseSubscriber() bool {
	return GetSubscriptionType() == SubEnterprise
}

// IsProSubscriber returns true for pro subscriptions.
func IsProSubscriber() bool {
	return GetSubscriptionType() == SubPro
}

// GetSubscriptionType returns the current subscription type from the
// cached OAuth profile or environment override.
// Source: utils/auth.ts — getSubscriptionType
func GetSubscriptionType() SubscriptionType {
	if mock := os.Getenv("MOCK_SUBSCRIPTION_TYPE"); mock != "" {
		return SubscriptionType(mock)
	}
	// In a real implementation, this reads from the OAuth profile cache.
	// For now, return empty (no subscription detected).
	return ""
}

// GetSubscriptionName returns a display name for the subscription type.
// Source: utils/auth.ts — getSubscriptionName
func GetSubscriptionName() string {
	switch GetSubscriptionType() {
	case SubFree:
		return "Free"
	case SubPro:
		return "Pro"
	case SubMax5:
		return "Max (5x)"
	case SubMax20:
		return "Max (20x)"
	case SubTeam:
		return "Team"
	case SubTeamPremium:
		return "Team Premium"
	case SubEnterprise:
		return "Enterprise"
	case SubInternalEmployee:
		return "Internal"
	default:
		return "Unknown"
	}
}

// IsUsing3PServices returns true if the user is using third-party providers
// (Bedrock, Vertex, custom base URL).
// Source: utils/auth.ts — isUsing3PServices
func IsUsing3PServices() bool {
	return os.Getenv("ANTHROPIC_BASE_URL") != "" ||
		os.Getenv("CLAUDE_CODE_USE_BEDROCK") != "" ||
		os.Getenv("CLAUDE_CODE_USE_VERTEX") != ""
}

// IsAnthropicAuthEnabled returns true if 1P Anthropic auth is available.
// Source: utils/auth.ts — isAnthropicAuthEnabled
func IsAnthropicAuthEnabled() bool {
	if IsUsing3PServices() {
		return false
	}
	key, _ := APIKeyWithSource()
	return key != "" || shouldUseClaudeAIAuth()
}

func shouldUseClaudeAIAuth() bool {
	return strings.EqualFold(os.Getenv("CLAUDE_CODE_AUTH_TYPE"), "oauth")
}
