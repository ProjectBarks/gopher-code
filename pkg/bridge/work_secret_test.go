package bridge

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// helper: encode a WorkSecret to base64url (no padding).
func encodeSecret(t *testing.T, ws WorkSecret) string {
	t.Helper()
	b, err := json.Marshal(ws)
	if err != nil {
		t.Fatal(err)
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

func validSecret() WorkSecret {
	return WorkSecret{
		Version:             1,
		SessionIngressToken: "tok_abc123",
		APIBaseURL:          "https://api.anthropic.com",
		Sources: []WorkSecretSource{
			{Type: "git", GitInfo: &GitInfo{Type: "github", Repo: "org/repo", Token: "ghp_xxx"}},
		},
		Auth: []WorkSecretAuth{
			{Type: "api_key", Token: "sk-ant-xxx"},
		},
	}
}

// ---------------------------------------------------------------------------
// DecodeWorkSecret
// ---------------------------------------------------------------------------

func TestDecodeWorkSecret_Valid(t *testing.T) {
	ws := validSecret()
	encoded := encodeSecret(t, ws)

	got, err := DecodeWorkSecret(encoded)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Version != 1 {
		t.Errorf("version = %d, want 1", got.Version)
	}
	if got.SessionIngressToken != "tok_abc123" {
		t.Errorf("token = %q, want %q", got.SessionIngressToken, "tok_abc123")
	}
	if got.APIBaseURL != "https://api.anthropic.com" {
		t.Errorf("api_base_url = %q", got.APIBaseURL)
	}
	if len(got.Sources) != 1 || got.Sources[0].GitInfo.Token != "ghp_xxx" {
		t.Error("git credential not extracted")
	}
	if len(got.Auth) != 1 || got.Auth[0].Token != "sk-ant-xxx" {
		t.Error("auth token not extracted")
	}
}

func TestDecodeWorkSecret_UnsupportedVersion(t *testing.T) {
	ws := validSecret()
	ws.Version = 99
	encoded := encodeSecret(t, ws)

	_, err := DecodeWorkSecret(encoded)
	if err == nil {
		t.Fatal("expected error for unsupported version")
	}
	want := "Unsupported work secret version: 99"
	if err.Error() != want {
		t.Errorf("error = %q, want %q", err.Error(), want)
	}
}

func TestDecodeWorkSecret_VersionZero(t *testing.T) {
	// Version defaults to 0 when omitted from JSON; that's also unsupported.
	raw := `{"session_ingress_token":"tok","api_base_url":"https://x.com"}`
	encoded := base64.RawURLEncoding.EncodeToString([]byte(raw))

	_, err := DecodeWorkSecret(encoded)
	if err == nil {
		t.Fatal("expected error for version 0")
	}
}

func TestDecodeWorkSecret_MissingToken(t *testing.T) {
	ws := validSecret()
	ws.SessionIngressToken = ""
	encoded := encodeSecret(t, ws)

	_, err := DecodeWorkSecret(encoded)
	if err == nil {
		t.Fatal("expected error for missing token")
	}
	want := "Invalid work secret: missing or empty session_ingress_token"
	if err.Error() != want {
		t.Errorf("error = %q, want %q", err.Error(), want)
	}
}

func TestDecodeWorkSecret_MissingAPIBaseURL(t *testing.T) {
	ws := validSecret()
	ws.APIBaseURL = ""
	encoded := encodeSecret(t, ws)

	_, err := DecodeWorkSecret(encoded)
	if err == nil {
		t.Fatal("expected error for missing api_base_url")
	}
	want := "Invalid work secret: missing api_base_url"
	if err.Error() != want {
		t.Errorf("error = %q, want %q", err.Error(), want)
	}
}

func TestDecodeWorkSecret_InvalidBase64(t *testing.T) {
	_, err := DecodeWorkSecret("!!!not-base64!!!")
	if err == nil {
		t.Fatal("expected error for invalid base64")
	}
}

func TestDecodeWorkSecret_InvalidJSON(t *testing.T) {
	encoded := base64.RawURLEncoding.EncodeToString([]byte("not json"))
	_, err := DecodeWorkSecret(encoded)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

// ---------------------------------------------------------------------------
// BuildSdkUrl
// ---------------------------------------------------------------------------

func TestBuildSdkUrl_Production(t *testing.T) {
	got := BuildSdkUrl("https://api.anthropic.com", "sess_123")
	want := "wss://api.anthropic.com/v1/session_ingress/ws/sess_123"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestBuildSdkUrl_ProductionTrailingSlash(t *testing.T) {
	got := BuildSdkUrl("https://api.anthropic.com/", "sess_123")
	want := "wss://api.anthropic.com/v1/session_ingress/ws/sess_123"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestBuildSdkUrl_Localhost(t *testing.T) {
	got := BuildSdkUrl("http://localhost:8080", "sess_abc")
	want := "ws://localhost:8080/v2/session_ingress/ws/sess_abc"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestBuildSdkUrl_127001(t *testing.T) {
	got := BuildSdkUrl("http://127.0.0.1:3000/", "sess_xyz")
	want := "ws://127.0.0.1:3000/v2/session_ingress/ws/sess_xyz"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// ---------------------------------------------------------------------------
// BuildCCRv2SdkUrl
// ---------------------------------------------------------------------------

func TestBuildCCRv2SdkUrl(t *testing.T) {
	got := BuildCCRv2SdkUrl("https://api.anthropic.com", "sess_123")
	want := "https://api.anthropic.com/v1/code/sessions/sess_123"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestBuildCCRv2SdkUrl_TrailingSlash(t *testing.T) {
	got := BuildCCRv2SdkUrl("https://api.anthropic.com/", "sess_123")
	want := "https://api.anthropic.com/v1/code/sessions/sess_123"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// ---------------------------------------------------------------------------
// SameSessionId
// ---------------------------------------------------------------------------

func TestSameSessionId_Identical(t *testing.T) {
	if !SameSessionId("cse_abc123", "cse_abc123") {
		t.Error("identical IDs should match")
	}
}

func TestSameSessionId_DifferentPrefix(t *testing.T) {
	if !SameSessionId("cse_abc123def4", "session_abc123def4") {
		t.Error("same body with different prefix should match")
	}
}

func TestSameSessionId_StagingPrefix(t *testing.T) {
	// {tag}_staging_{body} — body is after the last underscore.
	if !SameSessionId("cse_staging_abc123def4", "session_abc123def4") {
		t.Error("staging prefix should still match on body")
	}
}

func TestSameSessionId_Different(t *testing.T) {
	if SameSessionId("cse_abc123", "cse_xyz789") {
		t.Error("different bodies should not match")
	}
}

func TestSameSessionId_ShortBody(t *testing.T) {
	// Body < 4 chars should not match (guard against accidental matches).
	if SameSessionId("a_abc", "b_abc") {
		t.Error("3-char body should not match")
	}
}

func TestSameSessionId_FourCharBody(t *testing.T) {
	// Exactly 4 chars should match.
	if !SameSessionId("a_abcd", "b_abcd") {
		t.Error("4-char body should match")
	}
}

func TestSameSessionId_NoUnderscore(t *testing.T) {
	// No underscore: body == full string, already checked a == b.
	if SameSessionId("abc123", "abc123x") {
		t.Error("different strings with no underscore should not match")
	}
}

// ---------------------------------------------------------------------------
// RegisterWorker
// ---------------------------------------------------------------------------

func TestRegisterWorker_NumberEpoch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/worker/register" {
			t.Errorf("path = %s", r.URL.Path)
		}
		if got := r.Header.Get("anthropic-version"); got != "2023-06-01" {
			t.Errorf("anthropic-version = %q", got)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer tok123" {
			t.Errorf("auth = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"worker_epoch": 42}`))
	}))
	defer srv.Close()

	epoch, err := RegisterWorker(context.Background(), srv.URL, "tok123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if epoch != 42 {
		t.Errorf("epoch = %d, want 42", epoch)
	}
}

func TestRegisterWorker_StringEpoch(t *testing.T) {
	// protojson serializes int64 as string.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"worker_epoch": "99"}`))
	}))
	defer srv.Close()

	epoch, err := RegisterWorker(context.Background(), srv.URL, "tok")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if epoch != 99 {
		t.Errorf("epoch = %d, want 99", epoch)
	}
}

func TestRegisterWorker_InvalidEpoch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"worker_epoch": "not_a_number"}`))
	}))
	defer srv.Close()

	_, err := RegisterWorker(context.Background(), srv.URL, "tok")
	if err == nil {
		t.Fatal("expected error for non-numeric epoch")
	}
}

func TestRegisterWorker_MissingEpoch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	_, err := RegisterWorker(context.Background(), srv.URL, "tok")
	if err == nil {
		t.Fatal("expected error for missing epoch")
	}
}

func TestRegisterWorker_FloatEpoch(t *testing.T) {
	// 1.5 is not a valid int64.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"worker_epoch": 1.5}`))
	}))
	defer srv.Close()

	_, err := RegisterWorker(context.Background(), srv.URL, "tok")
	if err == nil {
		t.Fatal("expected error for float epoch")
	}
}
