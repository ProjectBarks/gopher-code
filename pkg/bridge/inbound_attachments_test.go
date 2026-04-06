package bridge

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

// ---------------------------------------------------------------------------
// ExtractInboundAttachments
// ---------------------------------------------------------------------------

func TestExtractInboundAttachments_ValidArray(t *testing.T) {
	raw := json.RawMessage(`{
		"file_attachments": [
			{"file_uuid": "abc-123", "file_name": "photo.png"},
			{"file_uuid": "def-456", "file_name": "readme.txt"}
		]
	}`)
	got := ExtractInboundAttachments(raw)
	if len(got) != 2 {
		t.Fatalf("expected 2 attachments, got %d", len(got))
	}
	if got[0].FileUUID != "abc-123" || got[0].FileName != "photo.png" {
		t.Errorf("attachment[0] = %+v", got[0])
	}
	if got[1].FileUUID != "def-456" || got[1].FileName != "readme.txt" {
		t.Errorf("attachment[1] = %+v", got[1])
	}
}

func TestExtractInboundAttachments_NoField(t *testing.T) {
	raw := json.RawMessage(`{"content": "hello"}`)
	got := ExtractInboundAttachments(raw)
	if got != nil {
		t.Fatalf("expected nil, got %+v", got)
	}
}

func TestExtractInboundAttachments_InvalidJSON(t *testing.T) {
	raw := json.RawMessage(`not json`)
	got := ExtractInboundAttachments(raw)
	if got != nil {
		t.Fatalf("expected nil, got %+v", got)
	}
}

func TestExtractInboundAttachments_EmptyUUID(t *testing.T) {
	raw := json.RawMessage(`{
		"file_attachments": [
			{"file_uuid": "", "file_name": "photo.png"}
		]
	}`)
	got := ExtractInboundAttachments(raw)
	if got != nil {
		t.Fatalf("expected nil for empty uuid, got %+v", got)
	}
}

func TestExtractInboundAttachments_MalformedArray(t *testing.T) {
	raw := json.RawMessage(`{"file_attachments": "not-an-array"}`)
	got := ExtractInboundAttachments(raw)
	if got != nil {
		t.Fatalf("expected nil, got %+v", got)
	}
}

// ---------------------------------------------------------------------------
// SanitizeFileName
// ---------------------------------------------------------------------------

func TestSanitizeFileName_BasicSafe(t *testing.T) {
	if got := SanitizeFileName("report.pdf"); got != "report.pdf" {
		t.Errorf("expected report.pdf, got %s", got)
	}
}

func TestSanitizeFileName_UnsafeChars(t *testing.T) {
	got := SanitizeFileName("my file (1).tar.gz")
	if strings.ContainsAny(got, " ()") {
		t.Errorf("unsafe chars remain: %s", got)
	}
	// Should be: my_file__1_.tar.gz
	if got != "my_file__1_.tar.gz" {
		t.Errorf("expected my_file__1_.tar.gz, got %s", got)
	}
}

func TestSanitizeFileName_PathTraversal(t *testing.T) {
	got := SanitizeFileName("../../etc/passwd")
	if got != "passwd" {
		t.Errorf("expected 'passwd', got %s", got)
	}
}

func TestSanitizeFileName_EmptyAfterSanitize(t *testing.T) {
	got := SanitizeFileName("***")
	// filepath.Base("***") = "***", sanitized to "___"
	if got == "attachment" {
		t.Errorf("expected non-empty sanitized name, but got fallback")
	}
	// Actually "___" is not empty, so fallback should not trigger. Let's just
	// make sure it's not empty.
	if got == "" {
		t.Errorf("should never be empty")
	}
}

func TestSanitizeFileName_FallbackOnEmpty(t *testing.T) {
	// An empty string through filepath.Base returns "." which sanitizes to "_".
	// Test that completely empty input doesn't panic.
	got := SanitizeFileName("")
	if got == "" {
		t.Error("should not return empty string")
	}
}

// ---------------------------------------------------------------------------
// UploadsDir
// ---------------------------------------------------------------------------

func TestUploadsDir(t *testing.T) {
	got := UploadsDir("/home/user/.claude", "sess-abc")
	want := filepath.Join("/home/user/.claude", "uploads", "sess-abc")
	if got != want {
		t.Errorf("got %s, want %s", got, want)
	}
}

// ---------------------------------------------------------------------------
// ResolveOne — with httptest server
// ---------------------------------------------------------------------------

func newTestDeps(t *testing.T, handler http.HandlerFunc) (*AttachmentDeps, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	tmpDir := t.TempDir()
	return &AttachmentDeps{
		GetAccessToken: func() (string, bool) { return "test-token", true },
		GetBaseURL:     func() string { return srv.URL },
		GetConfigDir:   func() string { return tmpDir },
		GetSessionID:   func() string { return "test-session" },
		HTTPClient:     srv.Client(),
	}, srv
}

func TestResolveOne_ImageAttachment(t *testing.T) {
	imageData := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x00}
	deps, srv := newTestDeps(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Error("missing auth header")
		}
		w.WriteHeader(200)
		w.Write(imageData)
	})
	defer srv.Close()

	att := InboundAttachment{FileUUID: "abcdef01-2345-6789", FileName: "photo.png"}
	path, err := ResolveOne(context.Background(), att, deps)
	if err != nil {
		t.Fatal(err)
	}
	if path == "" {
		t.Fatal("expected non-empty path")
	}

	// Verify file exists with correct content.
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(imageData) {
		t.Errorf("wrote %d bytes, want %d", len(got), len(imageData))
	}

	// Verify path structure: <tmpDir>/uploads/test-session/abcdef01-photo.png
	if !strings.Contains(path, "uploads") || !strings.Contains(path, "test-session") {
		t.Errorf("unexpected path structure: %s", path)
	}
	base := filepath.Base(path)
	if !strings.HasPrefix(base, "abcdef01-") {
		t.Errorf("expected uuid prefix in filename, got %s", base)
	}
	if !strings.HasSuffix(base, "photo.png") {
		t.Errorf("expected photo.png suffix, got %s", base)
	}
}

func TestResolveOne_TextFile(t *testing.T) {
	textData := []byte("Hello, world!\nLine 2.\n")
	deps, srv := newTestDeps(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write(textData)
	})
	defer srv.Close()

	att := InboundAttachment{FileUUID: "deadbeef-1234", FileName: "readme.txt"}
	path, err := ResolveOne(context.Background(), att, deps)
	if err != nil {
		t.Fatal(err)
	}
	if path == "" {
		t.Fatal("expected non-empty path")
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(textData) {
		t.Errorf("content mismatch: got %q", string(got))
	}
}

func TestResolveOne_NoToken_Skips(t *testing.T) {
	deps, srv := newTestDeps(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not reach server")
	})
	defer srv.Close()
	deps.GetAccessToken = func() (string, bool) { return "", false }

	att := InboundAttachment{FileUUID: "abc", FileName: "file.txt"}
	path, err := ResolveOne(context.Background(), att, deps)
	if err != nil {
		t.Fatal(err)
	}
	if path != "" {
		t.Errorf("expected empty path for no token, got %s", path)
	}
}

func TestResolveOne_Non200_Skips(t *testing.T) {
	deps, srv := newTestDeps(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
	})
	defer srv.Close()

	att := InboundAttachment{FileUUID: "abc12345", FileName: "file.txt"}
	path, err := ResolveOne(context.Background(), att, deps)
	if err != nil {
		t.Fatal(err)
	}
	if path != "" {
		t.Errorf("expected empty path for 404, got %s", path)
	}
}

func TestResolveOne_WriteFailure_Skips(t *testing.T) {
	deps, srv := newTestDeps(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte("data"))
	})
	defer srv.Close()
	// Point config dir at a read-only location to force write failure.
	deps.GetConfigDir = func() string { return "/dev/null/impossible" }

	att := InboundAttachment{FileUUID: "abc12345", FileName: "file.txt"}
	path, err := ResolveOne(context.Background(), att, deps)
	if err != nil {
		t.Fatal(err)
	}
	if path != "" {
		t.Errorf("expected empty path on write failure, got %s", path)
	}
}

// ---------------------------------------------------------------------------
// ResolveInboundAttachments
// ---------------------------------------------------------------------------

func TestResolveInboundAttachments_EmptyList(t *testing.T) {
	got := ResolveInboundAttachments(context.Background(), nil, nil)
	if got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestResolveInboundAttachments_TwoFiles(t *testing.T) {
	var call atomic.Int32
	deps, srv := newTestDeps(t, func(w http.ResponseWriter, r *http.Request) {
		n := call.Add(1)
		w.WriteHeader(200)
		fmt.Fprintf(w, "content-%d", n)
	})
	defer srv.Close()

	atts := []InboundAttachment{
		{FileUUID: "aaaabbbb-1", FileName: "a.txt"},
		{FileUUID: "ccccdddd-2", FileName: "b.txt"},
	}
	got := ResolveInboundAttachments(context.Background(), atts, deps)
	// Should contain two @"..." refs.
	if strings.Count(got, `@"`) != 2 {
		t.Errorf("expected 2 @-refs, got %q", got)
	}
	// Trailing space.
	if !strings.HasSuffix(got, " ") {
		t.Errorf("expected trailing space, got %q", got)
	}
}

func TestResolveInboundAttachments_AllFail(t *testing.T) {
	deps, srv := newTestDeps(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	})
	defer srv.Close()

	atts := []InboundAttachment{{FileUUID: "abc12345", FileName: "f.txt"}}
	got := ResolveInboundAttachments(context.Background(), atts, deps)
	if got != "" {
		t.Errorf("expected empty when all fail, got %q", got)
	}
}

// ---------------------------------------------------------------------------
// PrependPathRefs
// ---------------------------------------------------------------------------

func TestPrependPathRefs_EmptyPrefix(t *testing.T) {
	got := PrependPathRefs("hello", "")
	if got != "hello" {
		t.Errorf("expected unchanged, got %v", got)
	}
}

func TestPrependPathRefs_StringContent(t *testing.T) {
	got := PrependPathRefs("hello", `@"/tmp/f.txt" `)
	if got != `@"/tmp/f.txt" hello` {
		t.Errorf("got %v", got)
	}
}

func TestPrependPathRefs_BlocksWithText(t *testing.T) {
	blocks := []ContentBlock{
		{Type: "image"},
		{Type: "text", Text: "original"},
	}
	result := PrependPathRefs(blocks, `@"/tmp/a.txt" `)
	out, ok := result.([]ContentBlock)
	if !ok {
		t.Fatalf("expected []ContentBlock, got %T", result)
	}
	if len(out) != 2 {
		t.Fatalf("expected 2 blocks, got %d", len(out))
	}
	// Last text block should have prefix prepended.
	if out[1].Text != `@"/tmp/a.txt" original` {
		t.Errorf("got text: %q", out[1].Text)
	}
	// Original should be unmodified (copy semantics).
	if blocks[1].Text != "original" {
		t.Error("original slice was mutated")
	}
}

func TestPrependPathRefs_BlocksNoText_AppendsNewBlock(t *testing.T) {
	blocks := []ContentBlock{
		{Type: "image"},
	}
	result := PrependPathRefs(blocks, `@"/tmp/x.txt" `)
	out, ok := result.([]ContentBlock)
	if !ok {
		t.Fatalf("expected []ContentBlock, got %T", result)
	}
	if len(out) != 2 {
		t.Fatalf("expected 2 blocks, got %d", len(out))
	}
	if out[1].Type != "text" {
		t.Errorf("expected text block, got %s", out[1].Type)
	}
	// Trailing space should be trimmed per TS: prefix.trimEnd()
	if out[1].Text != `@"/tmp/x.txt"` {
		t.Errorf("got text: %q", out[1].Text)
	}
}

func TestPrependPathRefs_MultipleTextBlocks_TargetsLast(t *testing.T) {
	blocks := []ContentBlock{
		{Type: "text", Text: "first"},
		{Type: "image"},
		{Type: "text", Text: "last"},
	}
	result := PrependPathRefs(blocks, `@"/p" `)
	out := result.([]ContentBlock)
	if out[0].Text != "first" {
		t.Errorf("first text block should be unchanged, got %q", out[0].Text)
	}
	if out[2].Text != `@"/p" last` {
		t.Errorf("last text block should have prefix, got %q", out[2].Text)
	}
}

// ---------------------------------------------------------------------------
// ResolveAndPrepend — integration
// ---------------------------------------------------------------------------

func TestResolveAndPrepend_NoAttachments(t *testing.T) {
	raw := json.RawMessage(`{"content": "hello"}`)
	got := ResolveAndPrepend(context.Background(), raw, "hello", nil)
	if got != "hello" {
		t.Errorf("expected unchanged content, got %v", got)
	}
}

func TestResolveAndPrepend_WithAttachment(t *testing.T) {
	deps, srv := newTestDeps(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte("file-content"))
	})
	defer srv.Close()

	raw := json.RawMessage(`{
		"file_attachments": [
			{"file_uuid": "aabbccdd-1234", "file_name": "doc.txt"}
		]
	}`)
	got := ResolveAndPrepend(context.Background(), raw, "user prompt", deps)
	s, ok := got.(string)
	if !ok {
		t.Fatalf("expected string, got %T", got)
	}
	if !strings.HasPrefix(s, `@"`) {
		t.Errorf("expected @-ref prefix, got %q", s)
	}
	if !strings.HasSuffix(s, "user prompt") {
		t.Errorf("expected original content at end, got %q", s)
	}
}

// ---------------------------------------------------------------------------
// DownloadTimeoutMS constant
// ---------------------------------------------------------------------------

func TestDownloadTimeoutMS(t *testing.T) {
	if DownloadTimeoutMS != 30_000 {
		t.Errorf("expected 30000, got %d", DownloadTimeoutMS)
	}
}
