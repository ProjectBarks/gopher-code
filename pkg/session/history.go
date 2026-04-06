package session

// Source: src/assistant/sessionHistory.ts — HTTP client for paginated
// conversation-event history from the Anthropic session-events API.

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/projectbarks/gopher-code/pkg/auth"
	"github.com/projectbarks/gopher-code/pkg/message"
)

// T1: HISTORY_PAGE_SIZE is the default number of events per page.
const HistoryPageSize = 100

// T2: HistoryPage holds a page of session events returned by the API.
type HistoryPage struct {
	// Events in chronological order within the page.
	Events []message.Message `json:"events"`
	// FirstID is the oldest event ID in this page (cursor for older pages).
	FirstID *string `json:"first_id"`
	// HasMore indicates older events exist beyond this page.
	HasMore bool `json:"has_more"`
}

// T3: HistoryAuthCtx holds the pre-computed base URL and headers for
// session-history API requests. Reused across pages.
type HistoryAuthCtx struct {
	BaseURL string
	Headers map[string]string
}

// sessionEventsResponse is the raw API response shape.
type sessionEventsResponse struct {
	Data    json.RawMessage `json:"data"`
	HasMore bool            `json:"has_more"`
	FirstID *string         `json:"first_id"`
	LastID  *string         `json:"last_id"`
}

// historyClient is the shared HTTP client with 15s timeout (T10).
var historyClient = &http.Client{Timeout: 15 * time.Second}

// T4: CreateHistoryAuthCtx prepares auth context for session history requests.
// It resolves the OAuth config and API key once; the caller reuses the context
// across multiple page fetches.
func CreateHistoryAuthCtx(sessionID string) (*HistoryAuthCtx, error) {
	oauthCfg, err := auth.GetOAuthConfig()
	if err != nil {
		return nil, fmt.Errorf("history auth: oauth config: %w", err)
	}

	apiKey, err := auth.EnsureValidAuth()
	if err != nil {
		return nil, fmt.Errorf("history auth: %w", err)
	}

	baseURL := fmt.Sprintf("%s/v1/sessions/%s/events", oauthCfg.BaseAPIURL, sessionID)

	headers := map[string]string{
		"Authorization":        "Bearer " + apiKey,
		"anthropic-beta":       "ccr-byoc-2025-07-29",
		"x-organization-uuid":  "", // populated by caller when available
		"Content-Type":         "application/json",
	}

	return &HistoryAuthCtx{
		BaseURL: baseURL,
		Headers: headers,
	}, nil
}

// T5: FetchLatestEvents retrieves the newest page of events
// (anchor_to_latest=true). Returns nil, nil on HTTP failure (matches TS
// null-returning contract).
func FetchLatestEvents(ctx HistoryAuthCtx, limit int) (*HistoryPage, error) {
	if limit <= 0 {
		limit = HistoryPageSize
	}
	params := url.Values{
		"limit":            {strconv.Itoa(limit)},
		"anchor_to_latest": {"true"},
	}
	return fetchPage(ctx, params, "fetchLatestEvents")
}

// fetchPage is the internal helper that performs the actual HTTP GET.
func fetchPage(ctx HistoryAuthCtx, params url.Values, label string) (*HistoryPage, error) {
	reqURL := ctx.BaseURL + "?" + params.Encode()
	req, err := http.NewRequest(http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("[%s] build request: %w", label, err)
	}

	for k, v := range ctx.Headers {
		if v != "" {
			req.Header.Set(k, v)
		}
	}

	resp, err := historyClient.Do(req)
	if err != nil {
		// Network error -> nil (matches TS .catch(() => null))
		slog.Debug(fmt.Sprintf("[%s] HTTP error", label))
		return nil, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// T11: don't throw on non-2xx; T12: debug log
		slog.Debug(fmt.Sprintf("[%s] HTTP %d", label, resp.StatusCode))
		return nil, nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		slog.Debug(fmt.Sprintf("[%s] read body: %v", label, err))
		return nil, nil
	}

	var raw sessionEventsResponse
	if err := json.Unmarshal(body, &raw); err != nil {
		slog.Debug(fmt.Sprintf("[%s] decode response: %v", label, err))
		return nil, nil
	}

	// T13: Array.isArray guard on response data
	var events []message.Message
	if len(raw.Data) > 0 && raw.Data[0] == '[' {
		if err := json.Unmarshal(raw.Data, &events); err != nil {
			slog.Debug(fmt.Sprintf("[%s] decode events: %v", label, err))
			events = nil
		}
	}
	if events == nil {
		events = []message.Message{}
	}

	return &HistoryPage{
		Events:  events,
		FirstID: raw.FirstID,
		HasMore: raw.HasMore,
	}, nil
}
