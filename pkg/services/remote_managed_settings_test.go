package services

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRemoteManagedSettings_Fetch(t *testing.T) {
	settings := map[string]any{"model": "claude-opus-4-6", "theme": "dark"}
	checksum := computeSettingsChecksum(settings)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			w.WriteHeader(401)
			return
		}
		// Check If-None-Match for caching
		if r.Header.Get("If-None-Match") == checksum {
			w.WriteHeader(304)
			return
		}
		resp := RemoteManagedSettingsResponse{
			UUID:     "uuid-123",
			Checksum: checksum,
			Settings: settings,
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	svc := NewRemoteManagedSettingsService(server.URL, func() string { return "test-token" })

	// First fetch — should get settings
	result := svc.Fetch(context.Background())
	if !result.Success {
		t.Fatalf("first fetch failed: %s", result.Error)
	}
	if result.Settings == nil {
		t.Fatal("first fetch should return settings")
	}
	if result.Settings["model"] != "claude-opus-4-6" {
		t.Errorf("model = %v", result.Settings["model"])
	}

	// Second fetch — should get 304 (cached)
	result = svc.Fetch(context.Background())
	if !result.Success {
		t.Fatalf("second fetch failed: %s", result.Error)
	}
	if result.Settings != nil {
		t.Error("second fetch should return nil settings (304)")
	}
}

func TestRemoteManagedSettings_NoToken(t *testing.T) {
	svc := NewRemoteManagedSettingsService("http://localhost", func() string { return "" })
	result := svc.Fetch(context.Background())
	if result.Success {
		t.Error("should fail without token")
	}
	if !result.SkipRetry {
		t.Error("should skip retry for auth errors")
	}
}

func TestRemoteManagedSettings_Cache(t *testing.T) {
	svc := NewRemoteManagedSettingsService("http://localhost", func() string { return "" })

	// No cache initially
	if svc.GetCachedSettings() != nil {
		t.Error("should have no cache initially")
	}
}

func TestRemoteManagedSettings_OnChange(t *testing.T) {
	settings := map[string]any{"key": "value"}
	checksum := computeSettingsChecksum(settings)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := RemoteManagedSettingsResponse{UUID: "u", Checksum: checksum, Settings: settings}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	svc := NewRemoteManagedSettingsService(server.URL, func() string { return "tok" })

	changed := false
	svc.OnChange(func(s map[string]any) { changed = true })

	svc.Fetch(context.Background())
	if !changed {
		t.Error("OnChange should fire on first fetch")
	}
}

func TestComputeSettingsChecksum(t *testing.T) {
	settings := map[string]any{"a": 1, "b": "two"}
	c1 := computeSettingsChecksum(settings)
	c2 := computeSettingsChecksum(settings)
	if c1 != c2 {
		t.Error("same settings should produce same checksum")
	}
	if c1 == "" {
		t.Error("checksum should not be empty")
	}
}
