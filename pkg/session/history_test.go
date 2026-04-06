package session

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/projectbarks/gopher-code/pkg/message"
)

// T1: Verify HISTORY_PAGE_SIZE constant.
func TestHistoryPageSize(t *testing.T) {
	if HistoryPageSize != 100 {
		t.Errorf("HistoryPageSize = %d, want 100", HistoryPageSize)
	}
}

// T2: HistoryPage JSON round-trip.
func TestHistoryPage_JSONRoundTrip(t *testing.T) {
	firstID := "evt_001"
	page := HistoryPage{
		Events: []message.Message{
			{Role: message.RoleUser, Content: []message.ContentBlock{{Type: message.ContentText, Text: "hello"}}},
			{Role: message.RoleAssistant, Content: []message.ContentBlock{{Type: message.ContentText, Text: "hi"}}},
		},
		FirstID: &firstID,
		HasMore: true,
	}

	data, err := json.Marshal(page)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got HistoryPage
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(got.Events) != 2 {
		t.Fatalf("events len = %d, want 2", len(got.Events))
	}
	if got.Events[0].Role != message.RoleUser {
		t.Errorf("events[0].Role = %q, want %q", got.Events[0].Role, message.RoleUser)
	}
	if got.FirstID == nil || *got.FirstID != "evt_001" {
		t.Errorf("FirstID = %v, want %q", got.FirstID, "evt_001")
	}
	if !got.HasMore {
		t.Error("HasMore = false, want true")
	}
}

// T2: HistoryPage with nil FirstID.
func TestHistoryPage_NilFirstID(t *testing.T) {
	page := HistoryPage{
		Events:  []message.Message{},
		FirstID: nil,
		HasMore: false,
	}
	data, err := json.Marshal(page)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(data), `"first_id":null`) {
		t.Errorf("expected null first_id in JSON, got %s", data)
	}
}

// T3: HistoryAuthCtx type fields.
func TestHistoryAuthCtx_Fields(t *testing.T) {
	ctx := HistoryAuthCtx{
		BaseURL: "https://api.anthropic.com/v1/sessions/s123/events",
		Headers: map[string]string{
			"Authorization":  "Bearer tok",
			"anthropic-beta": "ccr-byoc-2025-07-29",
		},
	}
	if ctx.BaseURL == "" {
		t.Error("BaseURL should not be empty")
	}
	if ctx.Headers["anthropic-beta"] != "ccr-byoc-2025-07-29" {
		t.Errorf("anthropic-beta header = %q", ctx.Headers["anthropic-beta"])
	}
}

// T5: FetchLatestEvents sends correct query params and headers.
func TestFetchLatestEvents_RequestFormat(t *testing.T) {
	var gotPath, gotQuery string
	var gotHeaders http.Header

	firstID := "evt_first"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		gotHeaders = r.Header

		resp := map[string]any{
			"data": []map[string]any{
				{"role": "user", "content": []map[string]any{{"type": "text", "text": "hello"}}},
			},
			"has_more": true,
			"first_id": firstID,
			"last_id":  "evt_last",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	ctx := HistoryAuthCtx{
		BaseURL: srv.URL + "/v1/sessions/sess-abc/events",
		Headers: map[string]string{
			"Authorization":       "Bearer test-token",
			"anthropic-beta":      "ccr-byoc-2025-07-29",
			"x-organization-uuid": "org-123",
		},
	}

	page, err := FetchLatestEvents(ctx, 50)
	if err != nil {
		t.Fatalf("FetchLatestEvents error: %v", err)
	}

	// Verify request path
	if gotPath != "/v1/sessions/sess-abc/events" {
		t.Errorf("path = %q, want /v1/sessions/sess-abc/events", gotPath)
	}

	// Verify query params: anchor_to_latest=true & limit=50
	if !strings.Contains(gotQuery, "anchor_to_latest=true") {
		t.Errorf("query missing anchor_to_latest=true: %q", gotQuery)
	}
	if !strings.Contains(gotQuery, "limit=50") {
		t.Errorf("query missing limit=50: %q", gotQuery)
	}

	// Verify headers
	if got := gotHeaders.Get("Authorization"); got != "Bearer test-token" {
		t.Errorf("Authorization = %q", got)
	}
	if got := gotHeaders.Get("Anthropic-Beta"); got != "ccr-byoc-2025-07-29" {
		t.Errorf("anthropic-beta = %q", got)
	}
	if got := gotHeaders.Get("X-Organization-Uuid"); got != "org-123" {
		t.Errorf("x-organization-uuid = %q", got)
	}

	// Verify response parsing
	if page == nil {
		t.Fatal("page is nil")
	}
	if len(page.Events) != 1 {
		t.Fatalf("events len = %d, want 1", len(page.Events))
	}
	if page.Events[0].Role != message.RoleUser {
		t.Errorf("events[0].Role = %q", page.Events[0].Role)
	}
	if page.FirstID == nil || *page.FirstID != "evt_first" {
		t.Errorf("FirstID = %v, want evt_first", page.FirstID)
	}
	if !page.HasMore {
		t.Error("HasMore = false, want true")
	}
}

// T5: FetchLatestEvents defaults to HistoryPageSize when limit=0.
func TestFetchLatestEvents_DefaultLimit(t *testing.T) {
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data":[],"has_more":false,"first_id":null,"last_id":null}`))
	}))
	defer srv.Close()

	ctx := HistoryAuthCtx{BaseURL: srv.URL + "/v1/sessions/s1/events"}
	_, err := FetchLatestEvents(ctx, 0)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if !strings.Contains(gotQuery, "limit=100") {
		t.Errorf("expected default limit=100, got query: %q", gotQuery)
	}
}

// T5+T11: Non-200 returns nil page, no error.
func TestFetchLatestEvents_Non200ReturnsNil(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"internal"}`))
	}))
	defer srv.Close()

	ctx := HistoryAuthCtx{BaseURL: srv.URL + "/v1/sessions/s1/events"}
	page, err := FetchLatestEvents(ctx, HistoryPageSize)
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
	if page != nil {
		t.Errorf("expected nil page on 500, got: %+v", page)
	}
}

// T13: Non-array data field treated as empty events.
func TestFetchLatestEvents_NonArrayData(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// data is a string instead of an array
		w.Write([]byte(`{"data":"not-an-array","has_more":false,"first_id":null,"last_id":null}`))
	}))
	defer srv.Close()

	ctx := HistoryAuthCtx{BaseURL: srv.URL + "/v1/sessions/s1/events"}
	page, err := FetchLatestEvents(ctx, HistoryPageSize)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if page == nil {
		t.Fatal("page is nil")
	}
	if len(page.Events) != 0 {
		t.Errorf("expected empty events for non-array data, got %d", len(page.Events))
	}
}

// Network error returns nil page, nil error.
func TestFetchLatestEvents_NetworkError(t *testing.T) {
	ctx := HistoryAuthCtx{BaseURL: "http://127.0.0.1:1/unreachable"}
	page, err := FetchLatestEvents(ctx, HistoryPageSize)
	if err != nil {
		t.Fatalf("expected nil error on network failure, got: %v", err)
	}
	if page != nil {
		t.Error("expected nil page on network failure")
	}
}
