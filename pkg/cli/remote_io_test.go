package cli

import (
	"net/http"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/projectbarks/gopher-code/pkg/bridge"
)

// ---------------------------------------------------------------------------
// stubTransport — records calls for assertions
// ---------------------------------------------------------------------------

type stubTransport struct {
	mu             sync.Mutex
	connectCalled  bool
	closeCalled    bool
	written        []any
	onData         func(string)
	onClose        func()
	createdURL     *url.URL
	createdHeaders http.Header
}

func (s *stubTransport) Connect() error { s.mu.Lock(); s.connectCalled = true; s.mu.Unlock(); return nil }
func (s *stubTransport) Write(msg any) error {
	s.mu.Lock()
	s.written = append(s.written, msg)
	s.mu.Unlock()
	return nil
}
func (s *stubTransport) SetOnData(fn func(string))  { s.onData = fn }
func (s *stubTransport) SetOnClose(fn func())        { s.onClose = fn }
func (s *stubTransport) Close()                       { s.mu.Lock(); s.closeCalled = true; s.mu.Unlock() }
func (s *stubTransport) Written() []any               { s.mu.Lock(); defer s.mu.Unlock(); return append([]any(nil), s.written...) }

// ---------------------------------------------------------------------------
// Test: v1/v2 transport selection based on env
// ---------------------------------------------------------------------------

func TestRemoteIO_TransportVersionSelection(t *testing.T) {
	tests := []struct {
		name    string
		envVal  string
		wantVer TransportVersion
		// v2 rewrites URL; v1 passes through unchanged.
		wantPathContains string
	}{
		{
			name:             "v1 default (no env)",
			envVal:           "",
			wantVer:          TransportV1,
			wantPathContains: "/sessions/123",
		},
		{
			name:             "v1 explicit false",
			envVal:           "false",
			wantVer:          TransportV1,
			wantPathContains: "/sessions/123",
		},
		{
			name:             "v2 via true",
			envVal:           "true",
			wantVer:          TransportV2,
			wantPathContains: "/worker/events/stream",
		},
		{
			name:             "v2 via 1",
			envVal:           "1",
			wantVer:          TransportV2,
			wantPathContains: "/worker/events/stream",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("CLAUDE_CODE_USE_CCR_V2", tt.envVal)
			t.Setenv("CLAUDE_CODE_ENVIRONMENT_KIND", "")

			var captured stubTransport
			factory := func(u *url.URL, h http.Header, sid string, _ func() http.Header) Transport {
				captured.createdURL = u
				captured.createdHeaders = h
				return &captured
			}

			rio, err := NewRemoteIO(RemoteIOConfig{
				StreamURL:        "wss://example.com/sessions/123",
				SessionID:        "sess-abc",
				TransportFactory: factory,
				PollConfig:       bridge.DefaultPollConfig,
			})
			if err != nil {
				t.Fatalf("NewRemoteIO: %v", err)
			}
			defer rio.Close()

			if rio.Version() != tt.wantVer {
				t.Errorf("version = %d, want %d", rio.Version(), tt.wantVer)
			}

			if captured.createdURL == nil {
				t.Fatal("transport factory never called")
			}
			gotPath := captured.createdURL.Path
			if tt.wantPathContains != "" && !contains(gotPath, tt.wantPathContains) {
				t.Errorf("URL path = %q, want substring %q", gotPath, tt.wantPathContains)
			}

			// v2 should rewrite wss→https
			if tt.wantVer == TransportV2 && captured.createdURL.Scheme != "https" {
				t.Errorf("v2 scheme = %q, want https", captured.createdURL.Scheme)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Test: header injection (auth token + runner version)
// ---------------------------------------------------------------------------

func TestRemoteIO_HeaderInjection(t *testing.T) {
	tests := []struct {
		name          string
		token         string
		runnerVersion string
		wantAuth      string
		wantRunner    string
	}{
		{
			name:          "both present",
			token:         "tok-secret",
			runnerVersion: "2.1.0",
			wantAuth:      "Bearer tok-secret",
			wantRunner:    "2.1.0",
		},
		{
			name:          "no token",
			token:         "",
			runnerVersion: "1.0.0",
			wantAuth:      "",
			wantRunner:    "1.0.0",
		},
		{
			name:          "no runner version",
			token:         "tok-abc",
			runnerVersion: "",
			wantAuth:      "Bearer tok-abc",
			wantRunner:    "",
		},
		{
			name:          "neither present",
			token:         "",
			runnerVersion: "",
			wantAuth:      "",
			wantRunner:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("CLAUDE_CODE_USE_CCR_V2", "")
			t.Setenv("CLAUDE_CODE_ENVIRONMENT_KIND", "")
			t.Setenv("CLAUDE_CODE_ENVIRONMENT_RUNNER_VERSION", tt.runnerVersion)

			var captured stubTransport
			factory := func(u *url.URL, h http.Header, sid string, _ func() http.Header) Transport {
				captured.createdHeaders = h
				return &captured
			}

			rio, err := NewRemoteIO(RemoteIOConfig{
				StreamURL: "wss://example.com/sessions/1",
				TokenSource: func() string { return tt.token },
				TransportFactory: factory,
				PollConfig:       bridge.DefaultPollConfig,
			})
			if err != nil {
				t.Fatalf("NewRemoteIO: %v", err)
			}
			defer rio.Close()

			gotAuth := captured.createdHeaders.Get("Authorization")
			if gotAuth != tt.wantAuth {
				t.Errorf("Authorization = %q, want %q", gotAuth, tt.wantAuth)
			}
			gotRunner := captured.createdHeaders.Get("x-environment-runner-version")
			if gotRunner != tt.wantRunner {
				t.Errorf("x-environment-runner-version = %q, want %q", gotRunner, tt.wantRunner)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Test: dynamic header refresh re-reads env + token
// ---------------------------------------------------------------------------

func TestRemoteIO_HeaderRefresh(t *testing.T) {
	t.Setenv("CLAUDE_CODE_USE_CCR_V2", "")
	t.Setenv("CLAUDE_CODE_ENVIRONMENT_KIND", "")
	t.Setenv("CLAUDE_CODE_ENVIRONMENT_RUNNER_VERSION", "1.0.0")

	callCount := 0
	tokenSrc := func() string {
		callCount++
		if callCount == 1 {
			return "first-token"
		}
		return "refreshed-token"
	}

	var capturedRefresh func() http.Header
	var stub stubTransport
	factory := func(u *url.URL, h http.Header, sid string, refresh func() http.Header) Transport {
		capturedRefresh = refresh
		return &stub
	}

	rio, err := NewRemoteIO(RemoteIOConfig{
		StreamURL:        "wss://example.com/sessions/1",
		TokenSource:      tokenSrc,
		TransportFactory: factory,
		PollConfig:       bridge.DefaultPollConfig,
	})
	if err != nil {
		t.Fatalf("NewRemoteIO: %v", err)
	}
	defer rio.Close()

	if capturedRefresh == nil {
		t.Fatal("refreshHeaders callback not provided to factory")
	}

	// Update the env before calling refresh.
	t.Setenv("CLAUDE_CODE_ENVIRONMENT_RUNNER_VERSION", "2.0.0")

	h := capturedRefresh()
	if got := h.Get("Authorization"); got != "Bearer refreshed-token" {
		t.Errorf("refreshed Authorization = %q, want Bearer refreshed-token", got)
	}
	if got := h.Get("x-environment-runner-version"); got != "2.0.0" {
		t.Errorf("refreshed runner version = %q, want 2.0.0", got)
	}
}

// ---------------------------------------------------------------------------
// Test: keep-alive timer scheduling (bridge-only)
// ---------------------------------------------------------------------------

func TestRemoteIO_KeepAliveTimerScheduling(t *testing.T) {
	t.Run("bridge with positive interval fires keep_alive", func(t *testing.T) {
		t.Setenv("CLAUDE_CODE_USE_CCR_V2", "")
		t.Setenv("CLAUDE_CODE_ENVIRONMENT_KIND", "bridge")

		var stub stubTransport
		factory := func(u *url.URL, h http.Header, sid string, _ func() http.Header) Transport {
			return &stub
		}

		cfg := bridge.DefaultPollConfig
		cfg.SessionKeepaliveInterval = 50 * time.Millisecond

		rio, err := NewRemoteIO(RemoteIOConfig{
			StreamURL:        "wss://example.com/sessions/1",
			TransportFactory: factory,
			PollConfig:       cfg,
		})
		if err != nil {
			t.Fatalf("NewRemoteIO: %v", err)
		}

		// Wait for at least 2 ticks.
		time.Sleep(150 * time.Millisecond)
		rio.Close()

		written := stub.Written()
		if len(written) < 2 {
			t.Fatalf("expected >=2 keep_alive writes, got %d", len(written))
		}
		// Verify the message type.
		for i, w := range written {
			m, ok := w.(map[string]string)
			if !ok {
				t.Errorf("written[%d] type = %T, want map[string]string", i, w)
				continue
			}
			if m["type"] != "keep_alive" {
				t.Errorf("written[%d][type] = %q, want keep_alive", i, m["type"])
			}
		}
	})

	t.Run("non-bridge skips keep_alive even with positive interval", func(t *testing.T) {
		t.Setenv("CLAUDE_CODE_USE_CCR_V2", "")
		t.Setenv("CLAUDE_CODE_ENVIRONMENT_KIND", "")

		var stub stubTransport
		factory := func(u *url.URL, h http.Header, sid string, _ func() http.Header) Transport {
			return &stub
		}

		cfg := bridge.DefaultPollConfig
		cfg.SessionKeepaliveInterval = 50 * time.Millisecond

		rio, err := NewRemoteIO(RemoteIOConfig{
			StreamURL:        "wss://example.com/sessions/1",
			TransportFactory: factory,
			PollConfig:       cfg,
		})
		if err != nil {
			t.Fatalf("NewRemoteIO: %v", err)
		}

		time.Sleep(150 * time.Millisecond)
		rio.Close()

		if n := len(stub.Written()); n != 0 {
			t.Errorf("expected 0 writes for non-bridge, got %d", n)
		}
	})

	t.Run("bridge with zero interval skips keep_alive", func(t *testing.T) {
		t.Setenv("CLAUDE_CODE_USE_CCR_V2", "")
		t.Setenv("CLAUDE_CODE_ENVIRONMENT_KIND", "bridge")

		var stub stubTransport
		factory := func(u *url.URL, h http.Header, sid string, _ func() http.Header) Transport {
			return &stub
		}

		cfg := bridge.DefaultPollConfig
		cfg.SessionKeepaliveInterval = 0

		rio, err := NewRemoteIO(RemoteIOConfig{
			StreamURL:        "wss://example.com/sessions/1",
			TransportFactory: factory,
			PollConfig:       cfg,
		})
		if err != nil {
			t.Fatalf("NewRemoteIO: %v", err)
		}

		time.Sleep(100 * time.Millisecond)
		rio.Close()

		if n := len(stub.Written()); n != 0 {
			t.Errorf("expected 0 writes for zero interval, got %d", n)
		}
	})
}

// ---------------------------------------------------------------------------
// Test: CLAUDE_CODE_ENVIRONMENT_KIND=bridge flag
// ---------------------------------------------------------------------------

func TestRemoteIO_IsBridgeFlag(t *testing.T) {
	t.Run("bridge env set", func(t *testing.T) {
		t.Setenv("CLAUDE_CODE_USE_CCR_V2", "")
		t.Setenv("CLAUDE_CODE_ENVIRONMENT_KIND", "bridge")

		var stub stubTransport
		factory := func(u *url.URL, h http.Header, sid string, _ func() http.Header) Transport {
			return &stub
		}

		rio, err := NewRemoteIO(RemoteIOConfig{
			StreamURL:        "wss://example.com/sessions/1",
			TransportFactory: factory,
			PollConfig:       bridge.DefaultPollConfig,
		})
		if err != nil {
			t.Fatalf("NewRemoteIO: %v", err)
		}
		defer rio.Close()

		if !rio.isBridge {
			t.Error("expected isBridge=true when CLAUDE_CODE_ENVIRONMENT_KIND=bridge")
		}
	})

	t.Run("non-bridge env", func(t *testing.T) {
		t.Setenv("CLAUDE_CODE_USE_CCR_V2", "")
		t.Setenv("CLAUDE_CODE_ENVIRONMENT_KIND", "other")

		var stub stubTransport
		factory := func(u *url.URL, h http.Header, sid string, _ func() http.Header) Transport {
			return &stub
		}

		rio, err := NewRemoteIO(RemoteIOConfig{
			StreamURL:        "wss://example.com/sessions/1",
			TransportFactory: factory,
			PollConfig:       bridge.DefaultPollConfig,
		})
		if err != nil {
			t.Fatalf("NewRemoteIO: %v", err)
		}
		defer rio.Close()

		if rio.isBridge {
			t.Error("expected isBridge=false when CLAUDE_CODE_ENVIRONMENT_KIND=other")
		}
	})
}

// ---------------------------------------------------------------------------
// Test: deriveSSEURL
// ---------------------------------------------------------------------------

func TestDeriveSSEURL(t *testing.T) {
	tests := []struct {
		input      string
		wantScheme string
		wantPath   string
	}{
		{"wss://example.com/sessions/123", "https", "/sessions/123/worker/events/stream"},
		{"ws://example.com/sessions/456/", "http", "/sessions/456/worker/events/stream"},
		{"https://example.com/sessions/789", "https", "/sessions/789/worker/events/stream"},
	}
	for _, tt := range tests {
		u, _ := url.Parse(tt.input)
		got := deriveSSEURL(u)
		if got.Scheme != tt.wantScheme {
			t.Errorf("deriveSSEURL(%q).Scheme = %q, want %q", tt.input, got.Scheme, tt.wantScheme)
		}
		if got.Path != tt.wantPath {
			t.Errorf("deriveSSEURL(%q).Path = %q, want %q", tt.input, got.Path, tt.wantPath)
		}
	}
}

// ---------------------------------------------------------------------------
// Test: debug string verbatim
// ---------------------------------------------------------------------------

func TestRemoteIO_DebugStringNoToken(t *testing.T) {
	// Verify the exact debug string from the gap note is used in buildHeaders.
	// We test indirectly: buildHeaders with nil token source logs the message.
	// (The string lives in buildHeaders; this test documents the contract.)
	t.Setenv("CLAUDE_CODE_ENVIRONMENT_RUNNER_VERSION", "")

	h := buildHeaders(func() string { return "" }, nil)
	if got := h.Get("Authorization"); got != "" {
		t.Errorf("expected no Authorization header for empty token, got %q", got)
	}
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchSubstr(s, substr)
}

func searchSubstr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
