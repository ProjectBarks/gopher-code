package auth

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// AuthConfig holds authentication state.
type AuthConfig struct {
	APIKey   string `json:"api_key,omitempty"`
	Provider string `json:"provider,omitempty"` // "api-key", "oauth"
}

// GetAPIKey resolves the API key from multiple sources (in priority order):
// 1. ANTHROPIC_API_KEY environment variable
// 2. ~/.claude/auth.json file
// 3. Keychain (future)
func GetAPIKey() (string, error) {
	// 1. Environment variable (highest priority)
	if key := os.Getenv("ANTHROPIC_API_KEY"); key != "" {
		return key, nil
	}

	// 2. Auth config file
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

// SaveAPIKey saves the key to ~/.claude/auth.json
func SaveAPIKey(key string) error {
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
