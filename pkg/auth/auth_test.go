package auth

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/zalando/go-keyring"
)

func TestMain(m *testing.M) {
	// Use in-memory mock keyring to avoid macOS Keychain access,
	// which blocks threads when no keychain is available.
	keyring.MockInit()
	os.Exit(m.Run())
}

func TestGetAPIKey_EnvVar(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "sk-ant-test-key-123")

	key, err := GetAPIKey()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if key != "sk-ant-test-key-123" {
		t.Errorf("expected sk-ant-test-key-123, got %s", key)
	}
}

func TestGetAPIKey_EnvVarOverridesFile(t *testing.T) {
	// Set up a temp auth file
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	claudeDir := filepath.Join(tmpDir, ".claude")
	os.MkdirAll(claudeDir, 0700)
	cfg := AuthConfig{APIKey: "file-key-456", Provider: "api-key"}
	data, _ := json.MarshalIndent(cfg, "", "  ")
	os.WriteFile(filepath.Join(claudeDir, "auth.json"), data, 0600)

	// Env var should take priority
	t.Setenv("ANTHROPIC_API_KEY", "env-key-789")

	key, err := GetAPIKey()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if key != "env-key-789" {
		t.Errorf("expected env-key-789, got %s", key)
	}
}

func TestGetAPIKey_FileAuth(t *testing.T) {
	keyring.MockInit() // reset mock keyring so previous tests' saved keys don't leak
	t.Setenv("ANTHROPIC_API_KEY", "")

	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	claudeDir := filepath.Join(tmpDir, ".claude")
	os.MkdirAll(claudeDir, 0700)
	cfg := AuthConfig{APIKey: "file-key-456", Provider: "api-key"}
	data, _ := json.MarshalIndent(cfg, "", "  ")
	os.WriteFile(filepath.Join(claudeDir, "auth.json"), data, 0600)

	key, err := GetAPIKey()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if key != "file-key-456" {
		t.Errorf("expected file-key-456, got %s", key)
	}
}

func TestGetAPIKey_NoKeyFound(t *testing.T) {
	keyring.MockInit() // reset mock keyring so previous tests' saved keys don't leak
	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("HOME", t.TempDir())

	_, err := GetAPIKey()
	if err == nil {
		t.Fatal("expected error when no key is configured")
	}
}

func TestSaveAPIKey(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	err := SaveAPIKey("sk-ant-saved-key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify the file was written
	data, err := os.ReadFile(filepath.Join(tmpDir, ".claude", "auth.json"))
	if err != nil {
		t.Fatalf("failed to read auth.json: %v", err)
	}

	var cfg AuthConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("failed to parse auth.json: %v", err)
	}
	if cfg.APIKey != "sk-ant-saved-key" {
		t.Errorf("expected sk-ant-saved-key, got %s", cfg.APIKey)
	}
	if cfg.Provider != "api-key" {
		t.Errorf("expected provider api-key, got %s", cfg.Provider)
	}
}

func TestStatus_Authenticated(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "sk-ant-1234567890abcdef")

	status := Status()
	if status != "Authenticated (key: sk-a...cdef)" {
		t.Errorf("unexpected status: %s", status)
	}
}

func TestStatus_NotAuthenticated(t *testing.T) {
	keyring.MockInit() // reset mock keyring so previous tests' saved keys don't leak
	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("HOME", t.TempDir())

	status := Status()
	if status != "Not authenticated" {
		t.Errorf("unexpected status: %s", status)
	}
}
