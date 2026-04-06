package bridge

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// CreateCodeSession — success (returns cse_* id)
// ---------------------------------------------------------------------------

func TestCreateCodeSession_Success(t *testing.T) {
	var capturedBody map[string]any
	var capturedHeaders http.Header

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedHeaders = r.Header.Clone()
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/v1/code/sessions") {
			t.Errorf("path = %q", r.URL.Path)
		}
		json.NewDecoder(r.Body).Decode(&capturedBody)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		json.NewEncoder(w).Encode(map[string]any{
			"session": map[string]string{"id": "cse_abc123"},
		})
	}))
	defer srv.Close()

	client := NewCodeSessionClient(CodeSessionClientConfig{})

	id, err := client.CreateCodeSession(
		context.Background(), srv.URL, "tok-abc", "My Session", 5000, nil,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "cse_abc123" {
		t.Errorf("session ID = %q, want cse_abc123", id)
	}

	// Verify headers.
	if got := capturedHeaders.Get("Authorization"); got != "Bearer tok-abc" {
		t.Errorf("authorization = %q", got)
	}
	if got := capturedHeaders.Get("Anthropic-Version"); got != "2023-06-01" {
		t.Errorf("anthropic-version = %q", got)
	}
	if got := capturedHeaders.Get("Content-Type"); got != "application/json" {
		t.Errorf("content-type = %q", got)
	}

	// Verify body: title, bridge:{}, no tags.
	if capturedBody["title"] != "My Session" {
		t.Errorf("title = %v", capturedBody["title"])
	}
	if _, ok := capturedBody["bridge"]; !ok {
		t.Error("bridge key missing from request body")
	}
	if _, ok := capturedBody["tags"]; ok {
		t.Error("tags should be omitted when nil")
	}
}

// ---------------------------------------------------------------------------
// CreateCodeSession — with tags
// ---------------------------------------------------------------------------

func TestCreateCodeSession_WithTags(t *testing.T) {
	var capturedBody map[string]any

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&capturedBody)
		w.WriteHeader(201)
		json.NewEncoder(w).Encode(map[string]any{
			"session": map[string]string{"id": "cse_tagged"},
		})
	}))
	defer srv.Close()

	client := NewCodeSessionClient(CodeSessionClientConfig{})

	id, err := client.CreateCodeSession(
		context.Background(), srv.URL, "tok", "T", 5000, []string{"alpha", "beta"},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "cse_tagged" {
		t.Errorf("id = %q", id)
	}

	tags, ok := capturedBody["tags"].([]any)
	if !ok || len(tags) != 2 {
		t.Fatalf("tags = %v", capturedBody["tags"])
	}
	if tags[0] != "alpha" || tags[1] != "beta" {
		t.Errorf("tags = %v", tags)
	}
}

// ---------------------------------------------------------------------------
// CreateCodeSession — rejects non-cse_ id
// ---------------------------------------------------------------------------

func TestCreateCodeSession_RejectsNonCseID(t *testing.T) {
	var debugMsgs []string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		json.NewEncoder(w).Encode(map[string]any{
			"session": map[string]string{"id": "sess_not_cse"},
		})
	}))
	defer srv.Close()

	client := NewCodeSessionClient(CodeSessionClientConfig{
		OnDebug: func(msg string) { debugMsgs = append(debugMsgs, msg) },
	})

	id, err := client.CreateCodeSession(
		context.Background(), srv.URL, "tok", "T", 5000, nil,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "" {
		t.Errorf("expected empty ID for non-cse_ prefix, got %q", id)
	}
	if len(debugMsgs) == 0 || !strings.Contains(debugMsgs[len(debugMsgs)-1], "No session.id (cse_*)") {
		t.Errorf("debug = %v", debugMsgs)
	}
}

// ---------------------------------------------------------------------------
// CreateCodeSession — server returns 400
// ---------------------------------------------------------------------------

func TestCreateCodeSession_ServerRejects(t *testing.T) {
	var debugMsgs []string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]string{"message": "bad request"})
	}))
	defer srv.Close()

	client := NewCodeSessionClient(CodeSessionClientConfig{
		OnDebug: func(msg string) { debugMsgs = append(debugMsgs, msg) },
	})

	id, err := client.CreateCodeSession(
		context.Background(), srv.URL, "tok", "T", 5000, nil,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "" {
		t.Errorf("expected empty ID, got %q", id)
	}
	if len(debugMsgs) == 0 {
		t.Fatal("expected debug message")
	}
	last := debugMsgs[len(debugMsgs)-1]
	if !strings.Contains(last, "Session create failed 400") {
		t.Errorf("debug = %q", last)
	}
	if !strings.Contains(last, "bad request") {
		t.Errorf("debug should contain detail, got %q", last)
	}
}

// ---------------------------------------------------------------------------
// CreateCodeSession — missing session field in response
// ---------------------------------------------------------------------------

func TestCreateCodeSession_MissingSessionField(t *testing.T) {
	var debugMsgs []string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		json.NewEncoder(w).Encode(map[string]string{"other": "field"})
	}))
	defer srv.Close()

	client := NewCodeSessionClient(CodeSessionClientConfig{
		OnDebug: func(msg string) { debugMsgs = append(debugMsgs, msg) },
	})

	id, _ := client.CreateCodeSession(
		context.Background(), srv.URL, "tok", "T", 5000, nil,
	)
	if id != "" {
		t.Errorf("expected empty ID, got %q", id)
	}
	if len(debugMsgs) == 0 || !strings.Contains(debugMsgs[len(debugMsgs)-1], "No session.id (cse_*)") {
		t.Errorf("debug = %v", debugMsgs)
	}
}

// ---------------------------------------------------------------------------
// FetchRemoteCredentials — success
// ---------------------------------------------------------------------------

func TestFetchRemoteCredentials_Success(t *testing.T) {
	var capturedHeaders http.Header
	var capturedPath string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedHeaders = r.Header.Clone()
		capturedPath = r.URL.Path
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}
		w.WriteHeader(200)
		json.NewEncoder(w).Encode(map[string]any{
			"worker_jwt":   "jwt-opaque",
			"api_base_url": "https://api.example.com",
			"expires_in":   3600,
			"worker_epoch": 42,
		})
	}))
	defer srv.Close()

	client := NewCodeSessionClient(CodeSessionClientConfig{})

	creds, err := client.FetchRemoteCredentials(
		context.Background(), "cse_abc", srv.URL, "tok-xyz", 5000, "",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if creds == nil {
		t.Fatal("expected non-nil credentials")
	}

	if creds.WorkerJWT != "jwt-opaque" {
		t.Errorf("worker_jwt = %q", creds.WorkerJWT)
	}
	if creds.APIBaseURL != "https://api.example.com" {
		t.Errorf("api_base_url = %q", creds.APIBaseURL)
	}
	if creds.ExpiresIn != 3600 {
		t.Errorf("expires_in = %d", creds.ExpiresIn)
	}
	if creds.WorkerEpoch != 42 {
		t.Errorf("worker_epoch = %d", creds.WorkerEpoch)
	}

	// Verify path includes session ID.
	if !strings.Contains(capturedPath, "/v1/code/sessions/cse_abc/bridge") {
		t.Errorf("path = %q", capturedPath)
	}

	// Verify anthropic-version header.
	if got := capturedHeaders.Get("Anthropic-Version"); got != "2023-06-01" {
		t.Errorf("anthropic-version = %q", got)
	}

	// No trusted device token header when empty.
	if got := capturedHeaders.Get("X-Trusted-Device-Token"); got != "" {
		t.Errorf("X-Trusted-Device-Token should be empty, got %q", got)
	}
}

// ---------------------------------------------------------------------------
// FetchRemoteCredentials — trusted device token header
// ---------------------------------------------------------------------------

func TestFetchRemoteCredentials_TrustedDeviceToken(t *testing.T) {
	var capturedHeaders http.Header

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedHeaders = r.Header.Clone()
		w.WriteHeader(200)
		json.NewEncoder(w).Encode(map[string]any{
			"worker_jwt":   "jwt",
			"api_base_url": "https://api.test",
			"expires_in":   100,
			"worker_epoch": 1,
		})
	}))
	defer srv.Close()

	client := NewCodeSessionClient(CodeSessionClientConfig{})

	creds, err := client.FetchRemoteCredentials(
		context.Background(), "cse_x", srv.URL, "tok", 5000, "device-token-abc",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if creds == nil {
		t.Fatal("expected non-nil credentials")
	}
	if got := capturedHeaders.Get("X-Trusted-Device-Token"); got != "device-token-abc" {
		t.Errorf("X-Trusted-Device-Token = %q, want device-token-abc", got)
	}
}

// ---------------------------------------------------------------------------
// FetchRemoteCredentials — worker_epoch as string (protojson int64-as-string)
// ---------------------------------------------------------------------------

func TestFetchRemoteCredentials_WorkerEpochAsString(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		// Manually write JSON with worker_epoch as a string.
		w.Write([]byte(`{"worker_jwt":"jwt","api_base_url":"https://api.test","expires_in":100,"worker_epoch":"99"}`))
	}))
	defer srv.Close()

	client := NewCodeSessionClient(CodeSessionClientConfig{})

	creds, err := client.FetchRemoteCredentials(
		context.Background(), "cse_x", srv.URL, "tok", 5000, "",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if creds == nil {
		t.Fatal("expected non-nil credentials")
	}
	if creds.WorkerEpoch != 99 {
		t.Errorf("worker_epoch = %d, want 99", creds.WorkerEpoch)
	}
}

// ---------------------------------------------------------------------------
// FetchRemoteCredentials — worker_epoch exceeds safe integer
// ---------------------------------------------------------------------------

func TestFetchRemoteCredentials_WorkerEpochUnsafe(t *testing.T) {
	var debugMsgs []string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		// 2^53 is NOT safe.
		w.Write([]byte(fmt.Sprintf(`{"worker_jwt":"jwt","api_base_url":"https://api.test","expires_in":100,"worker_epoch":%d}`, 1<<53)))
	}))
	defer srv.Close()

	client := NewCodeSessionClient(CodeSessionClientConfig{
		OnDebug: func(msg string) { debugMsgs = append(debugMsgs, msg) },
	})

	creds, err := client.FetchRemoteCredentials(
		context.Background(), "cse_x", srv.URL, "tok", 5000, "",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if creds != nil {
		t.Errorf("expected nil for unsafe epoch, got %+v", creds)
	}
	if len(debugMsgs) == 0 || !strings.Contains(debugMsgs[len(debugMsgs)-1], "worker_epoch invalid") {
		t.Errorf("debug = %v", debugMsgs)
	}
}

// ---------------------------------------------------------------------------
// FetchRemoteCredentials — malformed response (missing fields)
// ---------------------------------------------------------------------------

func TestFetchRemoteCredentials_MalformedResponse(t *testing.T) {
	var debugMsgs []string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		// Missing worker_jwt.
		json.NewEncoder(w).Encode(map[string]any{
			"api_base_url": "https://api.test",
			"expires_in":   100,
			"worker_epoch": 1,
		})
	}))
	defer srv.Close()

	client := NewCodeSessionClient(CodeSessionClientConfig{
		OnDebug: func(msg string) { debugMsgs = append(debugMsgs, msg) },
	})

	creds, err := client.FetchRemoteCredentials(
		context.Background(), "cse_x", srv.URL, "tok", 5000, "",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if creds != nil {
		t.Errorf("expected nil for malformed response, got %+v", creds)
	}
	if len(debugMsgs) == 0 || !strings.Contains(debugMsgs[len(debugMsgs)-1], "malformed") {
		t.Errorf("debug = %v", debugMsgs)
	}
}

// ---------------------------------------------------------------------------
// FetchRemoteCredentials — non-200 status
// ---------------------------------------------------------------------------

func TestFetchRemoteCredentials_NonOKStatus(t *testing.T) {
	var debugMsgs []string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(403)
		json.NewEncoder(w).Encode(map[string]string{"message": "forbidden"})
	}))
	defer srv.Close()

	client := NewCodeSessionClient(CodeSessionClientConfig{
		OnDebug: func(msg string) { debugMsgs = append(debugMsgs, msg) },
	})

	creds, err := client.FetchRemoteCredentials(
		context.Background(), "cse_x", srv.URL, "tok", 5000, "",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if creds != nil {
		t.Error("expected nil for 403")
	}
	if len(debugMsgs) == 0 {
		t.Fatal("expected debug message")
	}
	last := debugMsgs[len(debugMsgs)-1]
	if !strings.Contains(last, "/bridge failed 403") {
		t.Errorf("debug = %q", last)
	}
	if !strings.Contains(last, "forbidden") {
		t.Errorf("debug should contain detail, got %q", last)
	}
}

// ---------------------------------------------------------------------------
// FetchRemoteCredentials — worker_epoch as invalid string
// ---------------------------------------------------------------------------

func TestFetchRemoteCredentials_WorkerEpochInvalidString(t *testing.T) {
	var debugMsgs []string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte(`{"worker_jwt":"jwt","api_base_url":"https://api.test","expires_in":100,"worker_epoch":"not_a_number"}`))
	}))
	defer srv.Close()

	client := NewCodeSessionClient(CodeSessionClientConfig{
		OnDebug: func(msg string) { debugMsgs = append(debugMsgs, msg) },
	})

	creds, _ := client.FetchRemoteCredentials(
		context.Background(), "cse_x", srv.URL, "tok", 5000, "",
	)
	if creds != nil {
		t.Errorf("expected nil for invalid epoch string, got %+v", creds)
	}
	if len(debugMsgs) == 0 || !strings.Contains(debugMsgs[len(debugMsgs)-1], "worker_epoch invalid") {
		t.Errorf("debug = %v", debugMsgs)
	}
}

// ---------------------------------------------------------------------------
// truncatePayload — log truncation
// ---------------------------------------------------------------------------

func TestTruncatePayload(t *testing.T) {
	short := "hello"
	if got := truncatePayload(short); got != short {
		t.Errorf("truncatePayload(%q) = %q", short, got)
	}

	long := strings.Repeat("x", 300)
	got := truncatePayload(long)
	if len(got) != 200 {
		t.Errorf("len(truncatePayload(300)) = %d, want 200", len(got))
	}
}

// ---------------------------------------------------------------------------
// extractCodeSessionErrorDetail
// ---------------------------------------------------------------------------

func TestExtractCodeSessionErrorDetail(t *testing.T) {
	cases := []struct {
		name string
		data string
		want string
	}{
		{"message field", `{"message":"oops"}`, "oops"},
		{"nested error.message", `{"error":{"message":"deep"}}`, "deep"},
		{"message takes priority", `{"message":"top","error":{"message":"deep"}}`, "top"},
		{"empty", `{}`, ""},
		{"nil", ``, ""},
		{"invalid json", `not json`, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := extractCodeSessionErrorDetail([]byte(c.data))
			if got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}
