package auth

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/zalando/go-keyring"
)

// Source: utils/secureStorage/

// Keyring constants.
// Source: utils/secureStorage/macOsKeychainStorage.ts
const (
	KeyringService = "claude-code"
	KeyringAPIKey  = "api-key"
)

// AuthConfig holds authentication state.
type AuthConfig struct {
	APIKey   string `json:"api_key,omitempty"`
	Provider string `json:"provider,omitempty"` // "api-key", "oauth"
}

// GetAPIKey resolves the API key from multiple sources (in priority order):
// 1. ANTHROPIC_API_KEY environment variable
// 2. OS keyring via go-keyring (secure storage)
// 3. ~/.claude/auth.json file (plaintext fallback)
// Source: utils/secureStorage/index.ts:9-17
func GetAPIKey() (string, error) {
	// 1. Environment variable (highest priority)
	if key := os.Getenv("ANTHROPIC_API_KEY"); key != "" {
		return key, nil
	}

	// 2. OS keyring (macOS Keychain, Linux secret-service, Windows credential manager)
	if key, err := keyring.Get(KeyringService, KeyringAPIKey); err == nil && key != "" {
		return key, nil
	}

	// 3. Plaintext fallback: ~/.claude/auth.json
	// Source: utils/secureStorage/plainTextStorage.ts
	if home, err := os.UserHomeDir(); err == nil {
		path := filepath.Join(home, ".claude", "auth.json")
		data, err := os.ReadFile(path)
		if err == nil {
			var cfg AuthConfig
			if json.Unmarshal(data, &cfg) == nil && cfg.APIKey != "" {
				return cfg.APIKey, nil
			}
		}
	}

	return "", fmt.Errorf("no API key found. Set ANTHROPIC_API_KEY or run 'gopher-code login'")
}

// SaveAPIKey saves the key to both OS keyring and ~/.claude/auth.json.
// Keyring is tried first; plaintext fallback ensures it always works.
func SaveAPIKey(key string) error {
	// Try keyring first (secure)
	_ = keyring.Set(KeyringService, KeyringAPIKey, key)

	// Always save to plaintext as fallback
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	dir := filepath.Join(home, ".claude")
	os.MkdirAll(dir, 0700)

	cfg := AuthConfig{APIKey: key, Provider: "api-key"}
	data, _ := json.MarshalIndent(cfg, "", "  ")
	return os.WriteFile(filepath.Join(dir, "auth.json"), data, 0600)
}

// DeleteAPIKey removes the key from both keyring and auth.json.
func DeleteAPIKey() {
	_ = keyring.Delete(KeyringService, KeyringAPIKey)
	if home, err := os.UserHomeDir(); err == nil {
		os.Remove(filepath.Join(home, ".claude", "auth.json"))
	}
}

// EnsureValidAuth checks that a valid API key or token is available before
// making an API call. For API key auth, this is a simple lookup.
// For OAuth (future), this would refresh expired tokens.
// Source: services/api/claude.ts — getClient() calls refreshIfExpired before each request
func EnsureValidAuth() (string, error) {
	key, err := GetAPIKey()
	if err != nil {
		return "", err
	}

	// Future: OAuth token refresh would go here
	// if isOAuthToken(key) && isExpired(key) {
	//     key, err = refreshToken(key)
	// }

	return key, nil
}

// Status returns the current auth status.
func Status() string {
	key, err := GetAPIKey()
	if err != nil {
		return "Not authenticated"
	}
	// Mask the key
	if len(key) > 8 {
		return fmt.Sprintf("Authenticated (key: %s...%s)", key[:4], key[len(key)-4:])
	}
	return "Authenticated"
}
