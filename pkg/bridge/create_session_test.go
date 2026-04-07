package bridge

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// ParseGitRemote
// ---------------------------------------------------------------------------

func TestParseGitRemote_SSH(t *testing.T) {
	p := ParseGitRemote("git@github.com:owner/repo.git")
	if p == nil {
		t.Fatal("expected non-nil")
	}
	if p.Host != "github.com" || p.Owner != "owner" || p.Name != "repo" {
		t.Errorf("got %+v", p)
	}
}

func TestParseGitRemote_HTTPS(t *testing.T) {
	p := ParseGitRemote("https://github.com/owner/repo.git")
	if p == nil {
		t.Fatal("expected non-nil")
	}
	if p.Host != "github.com" || p.Owner != "owner" || p.Name != "repo" {
		t.Errorf("got %+v", p)
	}
}

func TestParseGitRemote_HTTPSNoGit(t *testing.T) {
	p := ParseGitRemote("https://github.com/owner/repo")
	if p == nil {
		t.Fatal("expected non-nil")
	}
	if p.Owner != "owner" || p.Name != "repo" {
		t.Errorf("got %+v", p)
	}
}

func TestParseGitRemote_Invalid(t *testing.T) {
	for _, u := range []string{"", "not-a-url", "ftp://host/path", "git@host", "https://host"} {
		if p := ParseGitRemote(u); p != nil {
			t.Errorf("ParseGitRemote(%q) = %+v, want nil", u, p)
		}
	}
}

func TestParseGitHubRepository(t *testing.T) {
	owner, name, ok := ParseGitHubRepository("octocat/hello-world")
	if !ok || owner != "octocat" || name != "hello-world" {
		t.Errorf("got owner=%q name=%q ok=%v", owner, name, ok)
	}

	// Reject URLs
	_, _, ok = ParseGitHubRepository("github.com/owner/repo")
	if ok {
		t.Error("should reject URL-like input")
	}

	// Reject empty parts
	_, _, ok = ParseGitHubRepository("/repo")
	if ok {
		t.Error("should reject empty owner")
	}
	_, _, ok = ParseGitHubRepository("owner/")
	if ok {
		t.Error("should reject empty name")
	}
}

// ---------------------------------------------------------------------------
// buildGitContext
// ---------------------------------------------------------------------------

func TestBuildGitContext_SSHRemote(t *testing.T) {
	src, out := buildGitContext("git@github.com:acme/project.git", "main")
	if src == nil || out == nil {
		t.Fatal("expected non-nil git context")
	}
	if src.Type != "git_repository" {
		t.Errorf("source type = %q", src.Type)
	}
	if src.URL != "https://github.com/acme/project" {
		t.Errorf("source url = %q", src.URL)
	}
	if src.Revision != "main" {
		t.Errorf("source revision = %q", src.Revision)
	}
	if out.GitInfo.Repo != "acme/project" {
		t.Errorf("outcome repo = %q", out.GitInfo.Repo)
	}
	if len(out.GitInfo.Branches) != 1 || out.GitInfo.Branches[0] != "claude/main" {
		t.Errorf("outcome branches = %v", out.GitInfo.Branches)
	}
}

func TestBuildGitContext_EmptyBranch(t *testing.T) {
	src, out := buildGitContext("git@github.com:acme/project.git", "")
	if src == nil || out == nil {
		t.Fatal("expected non-nil")
	}
	if src.Revision != "" {
		t.Errorf("revision should be empty, got %q", src.Revision)
	}
	if out.GitInfo.Branches[0] != "claude/task" {
		t.Errorf("branch should be claude/task, got %q", out.GitInfo.Branches[0])
	}
}

func TestBuildGitContext_OwnerRepo(t *testing.T) {
	src, out := buildGitContext("octocat/hello", "dev")
	if src == nil || out == nil {
		t.Fatal("expected non-nil")
	}
	if src.URL != "https://github.com/octocat/hello" {
		t.Errorf("url = %q", src.URL)
	}
	if out.GitInfo.Branches[0] != "claude/dev" {
		t.Errorf("branches = %v", out.GitInfo.Branches)
	}
}

func TestBuildGitContext_Empty(t *testing.T) {
	src, out := buildGitContext("", "main")
	if src != nil || out != nil {
		t.Error("expected nil for empty URL")
	}
}

// ---------------------------------------------------------------------------
// CreateBridgeSession — success
// ---------------------------------------------------------------------------

func TestCreateBridgeSession_Success(t *testing.T) {
	var capturedBody map[string]any
	var capturedHeaders http.Header

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedHeaders = r.Header.Clone()
		json.NewDecoder(r.Body).Decode(&capturedBody)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(201)
		json.NewEncoder(w).Encode(map[string]string{"id": "sess_new_123"})
	}))
	defer srv.Close()

	client := NewSessionClient(SessionClientConfig{
		BaseURL:        srv.URL,
		GetAccessToken: func() string { return "tok-abc" },
		GetOrgUUID:     func() string { return "org-uuid-1" },
		GetModel:       func() string { return "claude-sonnet-4-20250514" },
	})

	title := "My Session"
	id, err := client.CreateBridgeSession(context.Background(), CreateBridgeSessionOpts{
		EnvironmentID:  "env-1",
		Title:          &title,
		Events:         []SessionEvent{{Type: "event", Data: "hello"}},
		GitRepoURL:     "git@github.com:acme/proj.git",
		Branch:         "main",
		PermissionMode: "auto-accept",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "sess_new_123" {
		t.Errorf("session ID = %q, want sess_new_123", id)
	}

	// Check headers
	if got := capturedHeaders.Get("Anthropic-Beta"); got != SessionsBetaHeader {
		t.Errorf("anthropic-beta = %q, want %q", got, SessionsBetaHeader)
	}
	if got := capturedHeaders.Get("X-Organization-Uuid"); got != "org-uuid-1" {
		t.Errorf("x-organization-uuid = %q", got)
	}
	if got := capturedHeaders.Get("Authorization"); got != "Bearer tok-abc" {
		t.Errorf("authorization = %q", got)
	}

	// Check body fields
	if capturedBody["source"] != "remote-control" {
		t.Errorf("source = %v", capturedBody["source"])
	}
	if capturedBody["environment_id"] != "env-1" {
		t.Errorf("environment_id = %v", capturedBody["environment_id"])
	}
	if capturedBody["title"] != "My Session" {
		t.Errorf("title = %v", capturedBody["title"])
	}
	if capturedBody["permission_mode"] != "auto-accept" {
		t.Errorf("permission_mode = %v", capturedBody["permission_mode"])
	}
}

// ---------------------------------------------------------------------------
// CreateBridgeSession — no access token
// ---------------------------------------------------------------------------

func TestCreateBridgeSession_NoAccessToken(t *testing.T) {
	var debugMsgs []string
	client := NewSessionClient(SessionClientConfig{
		GetAccessToken: func() string { return "" },
		GetOrgUUID:     func() string { return "org" },
		OnDebug:        func(msg string) { debugMsgs = append(debugMsgs, msg) },
	})

	_, err := client.CreateBridgeSession(context.Background(), CreateBridgeSessionOpts{
		EnvironmentID: "env-1",
		Events:        []SessionEvent{},
	})
	if err == nil {
		t.Fatal("expected error for missing access token")
	}
	if !strings.Contains(err.Error(), "No access token") {
		t.Errorf("error = %q", err)
	}
	if len(debugMsgs) == 0 || !strings.Contains(debugMsgs[0], "No access token for session creation") {
		t.Errorf("debug = %v", debugMsgs)
	}
}

// ---------------------------------------------------------------------------
// CreateBridgeSession — no org UUID
// ---------------------------------------------------------------------------

func TestCreateBridgeSession_NoOrgUUID(t *testing.T) {
	var debugMsgs []string
	client := NewSessionClient(SessionClientConfig{
		GetAccessToken: func() string { return "tok" },
		GetOrgUUID:     func() string { return "" },
		OnDebug:        func(msg string) { debugMsgs = append(debugMsgs, msg) },
	})

	_, err := client.CreateBridgeSession(context.Background(), CreateBridgeSessionOpts{
		EnvironmentID: "env-1",
		Events:        []SessionEvent{},
	})
	if err == nil {
		t.Fatal("expected error for missing org UUID")
	}
	if !strings.Contains(err.Error(), "No org UUID") {
		t.Errorf("error = %q", err)
	}
}

// ---------------------------------------------------------------------------
// CreateBridgeSession — server error (non-success status)
// ---------------------------------------------------------------------------

func TestCreateBridgeSession_ServerRejects(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]string{"message": "bad request body"})
	}))
	defer srv.Close()

	client := NewSessionClient(SessionClientConfig{
		BaseURL:        srv.URL,
		GetAccessToken: func() string { return "tok" },
		GetOrgUUID:     func() string { return "org" },
	})

	_, err := client.CreateBridgeSession(context.Background(), CreateBridgeSessionOpts{
		EnvironmentID: "env-1",
		Events:        []SessionEvent{},
	})
	if err == nil {
		t.Fatal("expected error for 400 response")
	}
	if !strings.Contains(err.Error(), "status 400") {
		t.Errorf("error = %q, want 'status 400'", err)
	}
	if !strings.Contains(err.Error(), "bad request body") {
		t.Errorf("error = %q, want detail", err)
	}
}

// ---------------------------------------------------------------------------
// CreateBridgeSession — no session ID in response
// ---------------------------------------------------------------------------

func TestCreateBridgeSession_NoIDInResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		json.NewEncoder(w).Encode(map[string]string{"other": "field"})
	}))
	defer srv.Close()

	client := NewSessionClient(SessionClientConfig{
		BaseURL:        srv.URL,
		GetAccessToken: func() string { return "tok" },
		GetOrgUUID:     func() string { return "org" },
	})

	_, err := client.CreateBridgeSession(context.Background(), CreateBridgeSessionOpts{
		EnvironmentID: "env-1",
		Events:        []SessionEvent{},
	})
	if err == nil {
		t.Fatal("expected error for missing session ID")
	}
	if !strings.Contains(err.Error(), "No session ID") {
		t.Errorf("error = %q", err)
	}
}

// ---------------------------------------------------------------------------
// CreateBridgeSession — no title omitted from body
// ---------------------------------------------------------------------------

func TestCreateBridgeSession_NoTitle(t *testing.T) {
	var capturedBody map[string]any

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&capturedBody)
		w.WriteHeader(201)
		json.NewEncoder(w).Encode(map[string]string{"id": "s1"})
	}))
	defer srv.Close()

	client := NewSessionClient(SessionClientConfig{
		BaseURL:        srv.URL,
		GetAccessToken: func() string { return "tok" },
		GetOrgUUID:     func() string { return "org" },
	})

	_, err := client.CreateBridgeSession(context.Background(), CreateBridgeSessionOpts{
		EnvironmentID: "env-1",
		Events:        []SessionEvent{},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, exists := capturedBody["title"]; exists {
		t.Error("title should be omitted when nil")
	}
	if _, exists := capturedBody["permission_mode"]; exists {
		t.Error("permission_mode should be omitted when empty")
	}
}

// ---------------------------------------------------------------------------
// GetBridgeSession — success
// ---------------------------------------------------------------------------

func TestGetBridgeSession_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("method = %q, want GET", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/v1/sessions/sess-1") {
			t.Errorf("path = %q", r.URL.Path)
		}
		w.WriteHeader(200)
		json.NewEncoder(w).Encode(map[string]string{"environment_id": "env-1", "title": "hello"})
	}))
	defer srv.Close()

	client := NewSessionClient(SessionClientConfig{
		BaseURL:        srv.URL,
		GetAccessToken: func() string { return "tok" },
		GetOrgUUID:     func() string { return "org" },
	})

	info, err := client.GetBridgeSession(context.Background(), "sess-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.EnvironmentID != "env-1" {
		t.Errorf("environment_id = %q", info.EnvironmentID)
	}
	if info.Title != "hello" {
		t.Errorf("title = %q", info.Title)
	}
}

func TestGetBridgeSession_NoToken(t *testing.T) {
	client := NewSessionClient(SessionClientConfig{
		GetAccessToken: func() string { return "" },
		GetOrgUUID:     func() string { return "org" },
	})

	_, err := client.GetBridgeSession(context.Background(), "sess-1")
	if err == nil || !strings.Contains(err.Error(), "No access token for session fetch") {
		t.Errorf("err = %v", err)
	}
}

// ---------------------------------------------------------------------------
// ArchiveBridgeSession
// ---------------------------------------------------------------------------

func TestArchiveBridgeSession_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("method = %q", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/v1/sessions/sess-1/archive") {
			t.Errorf("path = %q", r.URL.Path)
		}
		w.WriteHeader(200)
		w.Write([]byte("{}"))
	}))
	defer srv.Close()

	var debugMsgs []string
	client := NewSessionClient(SessionClientConfig{
		BaseURL:        srv.URL,
		GetAccessToken: func() string { return "tok" },
		GetOrgUUID:     func() string { return "org" },
		OnDebug:        func(msg string) { debugMsgs = append(debugMsgs, msg) },
	})

	err := client.ArchiveBridgeSession(context.Background(), "sess-1", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Check that the success message was logged
	found := false
	for _, m := range debugMsgs {
		if strings.Contains(m, "archived successfully") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected 'archived successfully' in debug, got %v", debugMsgs)
	}
}

func TestArchiveBridgeSession_409Idempotent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(409)
		w.Write([]byte("{}"))
	}))
	defer srv.Close()

	client := NewSessionClient(SessionClientConfig{
		BaseURL:        srv.URL,
		GetAccessToken: func() string { return "tok" },
		GetOrgUUID:     func() string { return "org" },
	})

	err := client.ArchiveBridgeSession(context.Background(), "sess-1", nil)
	if err != nil {
		t.Fatalf("409 should not error: %v", err)
	}
}

func TestArchiveBridgeSession_NoToken(t *testing.T) {
	client := NewSessionClient(SessionClientConfig{
		GetAccessToken: func() string { return "" },
		GetOrgUUID:     func() string { return "org" },
	})

	// Missing token returns nil (non-fatal, matches TS)
	err := client.ArchiveBridgeSession(context.Background(), "s", nil)
	if err != nil {
		t.Errorf("expected nil for missing token, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// UpdateBridgeSessionTitle
// ---------------------------------------------------------------------------

func TestUpdateBridgeSessionTitle_Success(t *testing.T) {
	var capturedBody map[string]any
	var capturedPath string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		if r.Method != "PATCH" {
			t.Errorf("method = %q", r.Method)
		}
		json.NewDecoder(r.Body).Decode(&capturedBody)
		w.WriteHeader(200)
		w.Write([]byte("{}"))
	}))
	defer srv.Close()

	var debugMsgs []string
	client := NewSessionClient(SessionClientConfig{
		BaseURL:        srv.URL,
		GetAccessToken: func() string { return "tok" },
		GetOrgUUID:     func() string { return "org" },
		OnDebug:        func(msg string) { debugMsgs = append(debugMsgs, msg) },
	})

	err := client.UpdateBridgeSessionTitle(context.Background(), "cse_abc", "New Title")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Check that cse_ was re-tagged to session_
	if !strings.HasSuffix(capturedPath, "/v1/sessions/session_abc") {
		t.Errorf("path = %q, expected compat session ID", capturedPath)
	}
	if capturedBody["title"] != "New Title" {
		t.Errorf("title = %v", capturedBody["title"])
	}
	// Check debug messages contain the arrow
	found := false
	for _, m := range debugMsgs {
		if strings.Contains(m, "→") && strings.Contains(m, "New Title") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected title update debug with arrow, got %v", debugMsgs)
	}
}

func TestUpdateBridgeSessionTitle_NoCompatNeeded(t *testing.T) {
	var capturedPath string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		w.WriteHeader(200)
		w.Write([]byte("{}"))
	}))
	defer srv.Close()

	client := NewSessionClient(SessionClientConfig{
		BaseURL:        srv.URL,
		GetAccessToken: func() string { return "tok" },
		GetOrgUUID:     func() string { return "org" },
	})

	client.UpdateBridgeSessionTitle(context.Background(), "session_xyz", "T")
	if !strings.HasSuffix(capturedPath, "/v1/sessions/session_xyz") {
		t.Errorf("path = %q", capturedPath)
	}
}

// ---------------------------------------------------------------------------
// extractSessionErrorDetail
// ---------------------------------------------------------------------------

func TestExtractSessionErrorDetail(t *testing.T) {
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
			got := extractSessionErrorDetail([]byte(c.data))
			if got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Session context in request body — git source and outcome shape
// ---------------------------------------------------------------------------

func TestCreateBridgeSession_GitContextShape(t *testing.T) {
	var capturedBody json.RawMessage

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&capturedBody)
		w.WriteHeader(201)
		json.NewEncoder(w).Encode(map[string]string{"id": "s1"})
	}))
	defer srv.Close()

	client := NewSessionClient(SessionClientConfig{
		BaseURL:        srv.URL,
		GetAccessToken: func() string { return "tok" },
		GetOrgUUID:     func() string { return "org" },
		GetModel:       func() string { return "claude-sonnet-4-20250514" },
	})

	client.CreateBridgeSession(context.Background(), CreateBridgeSessionOpts{
		EnvironmentID: "env-1",
		Events:        []SessionEvent{},
		GitRepoURL:    "git@github.com:acme/proj.git",
		Branch:        "feat",
	})

	// Decode and check session_context shape
	var body struct {
		SessionContext struct {
			Sources []struct {
				Type     string `json:"type"`
				URL      string `json:"url"`
				Revision string `json:"revision"`
			} `json:"sources"`
			Outcomes []struct {
				Type    string `json:"type"`
				GitInfo struct {
					Type     string   `json:"type"`
					Repo     string   `json:"repo"`
					Branches []string `json:"branches"`
				} `json:"git_info"`
			} `json:"outcomes"`
			Model string `json:"model"`
		} `json:"session_context"`
	}
	json.Unmarshal(capturedBody, &body)

	if len(body.SessionContext.Sources) != 1 {
		t.Fatalf("sources len = %d", len(body.SessionContext.Sources))
	}
	src := body.SessionContext.Sources[0]
	if src.Type != "git_repository" {
		t.Errorf("source type = %q", src.Type)
	}
	if src.URL != "https://github.com/acme/proj" {
		t.Errorf("source url = %q", src.URL)
	}
	if src.Revision != "feat" {
		t.Errorf("source revision = %q", src.Revision)
	}

	if len(body.SessionContext.Outcomes) != 1 {
		t.Fatalf("outcomes len = %d", len(body.SessionContext.Outcomes))
	}
	out := body.SessionContext.Outcomes[0]
	if out.Type != "git_repository" {
		t.Errorf("outcome type = %q", out.Type)
	}
	if out.GitInfo.Type != "github" {
		t.Errorf("git_info type = %q", out.GitInfo.Type)
	}
	if out.GitInfo.Repo != "acme/proj" {
		t.Errorf("git_info repo = %q", out.GitInfo.Repo)
	}
	if len(out.GitInfo.Branches) != 1 || out.GitInfo.Branches[0] != "claude/feat" {
		t.Errorf("git_info branches = %v", out.GitInfo.Branches)
	}
	if body.SessionContext.Model != "claude-sonnet-4-20250514" {
		t.Errorf("model = %q", body.SessionContext.Model)
	}
}

// ---------------------------------------------------------------------------
// Integration: SessionClient constructed as in main.go remote-control handler
// ---------------------------------------------------------------------------

func TestSessionClient_IntegrationWithBridgeDeps(t *testing.T) {
	// Simulate the full create → get → archive lifecycle as wired in main.go.
	var createCalled, getCalled, archiveCalled bool

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/sessions":
			createCalled = true
			w.WriteHeader(201)
			json.NewEncoder(w).Encode(map[string]string{"id": "sess-integ-1"})
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/v1/sessions/sess-integ-1"):
			getCalled = true
			w.WriteHeader(200)
			json.NewEncoder(w).Encode(map[string]string{"environment_id": "env-1", "title": "Integ"})
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/archive"):
			archiveCalled = true
			w.WriteHeader(200)
			w.Write([]byte("{}"))
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			http.Error(w, "not found", 404)
		}
	}))
	defer srv.Close()

	// Construct SessionClient exactly as main.go does: bridgeURL from config,
	// token from bridgeDeps.GetAccessToken, org/model resolved lazily.
	bridgeURL := srv.URL
	getAccessToken := func() (string, bool) { return "tok-integ", true }

	sc := NewSessionClient(SessionClientConfig{
		BaseURL:        bridgeURL,
		GetAccessToken: func() string { tok, _ := getAccessToken(); return tok },
		GetOrgUUID:     func() string { return "org-integ" },
		GetModel:       func() string { return "claude-sonnet-4-20250514" },
		OnDebug:        func(msg string) { t.Logf("debug: %s", msg) },
	})

	ctx := context.Background()

	// 1. Create session
	id, err := sc.CreateBridgeSession(ctx, CreateBridgeSessionOpts{
		EnvironmentID: "env-1",
		Events:        []SessionEvent{{Type: "text", Data: "start"}},
		GitRepoURL:    "https://github.com/acme/proj",
		Branch:        "main",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if id != "sess-integ-1" {
		t.Fatalf("id = %q", id)
	}

	// 2. Get session
	info, err := sc.GetBridgeSession(ctx, id)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if info.Title != "Integ" {
		t.Errorf("title = %q", info.Title)
	}

	// 3. Archive session
	if err := sc.ArchiveBridgeSession(ctx, id, nil); err != nil {
		t.Fatalf("archive: %v", err)
	}

	if !createCalled || !getCalled || !archiveCalled {
		t.Errorf("lifecycle incomplete: create=%v get=%v archive=%v", createCalled, getCalled, archiveCalled)
	}
}

// ---------------------------------------------------------------------------
// CreateBridgeSession — empty git context produces empty arrays
// ---------------------------------------------------------------------------

func TestCreateBridgeSession_NoGitContext(t *testing.T) {
	var capturedBody json.RawMessage

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&capturedBody)
		w.WriteHeader(201)
		json.NewEncoder(w).Encode(map[string]string{"id": "s1"})
	}))
	defer srv.Close()

	client := NewSessionClient(SessionClientConfig{
		BaseURL:        srv.URL,
		GetAccessToken: func() string { return "tok" },
		GetOrgUUID:     func() string { return "org" },
	})

	client.CreateBridgeSession(context.Background(), CreateBridgeSessionOpts{
		EnvironmentID: "env-1",
		Events:        []SessionEvent{},
	})

	var body struct {
		SessionContext struct {
			Sources  []any `json:"sources"`
			Outcomes []any `json:"outcomes"`
		} `json:"session_context"`
	}
	json.Unmarshal(capturedBody, &body)

	if body.SessionContext.Sources == nil {
		t.Error("sources should be empty array, not nil")
	}
	if len(body.SessionContext.Sources) != 0 {
		t.Errorf("sources should be empty, got %d", len(body.SessionContext.Sources))
	}
	if len(body.SessionContext.Outcomes) != 0 {
		t.Errorf("outcomes should be empty, got %d", len(body.SessionContext.Outcomes))
	}
}
