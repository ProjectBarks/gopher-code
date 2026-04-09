package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// Source: services/remoteManagedSettings/
//
// Remote managed settings for enterprise customers. Fetches policy-managed
// settings from the API, caches with checksum-based invalidation, polls hourly.

const (
	settingsTimeoutMs     = 10_000
	defaultMaxRetries     = 5
	pollingIntervalMs     = 60 * 60 * 1000 // 1 hour
)

// RemoteManagedSettingsResponse is the API response.
// Source: services/remoteManagedSettings/types.ts
type RemoteManagedSettingsResponse struct {
	UUID     string         `json:"uuid"`
	Checksum string         `json:"checksum"`
	Settings map[string]any `json:"settings"`
}

// FetchResult is the outcome of fetching remote settings.
type FetchResult struct {
	Success   bool
	Settings  map[string]any // nil = 304 Not Modified
	Checksum  string
	Error     string
	SkipRetry bool // e.g. auth errors
}

// RemoteManagedSettingsService manages fetching, caching, and polling.
type RemoteManagedSettingsService struct {
	mu           sync.RWMutex
	baseURL      string
	getToken     func() string
	cache        *RemoteManagedSettingsResponse
	lastFetch    time.Time
	pollInterval time.Duration
	stopPoll     chan struct{}
	onChange     func(map[string]any)
}

// NewRemoteManagedSettingsService creates the service.
func NewRemoteManagedSettingsService(baseURL string, tokenFn func() string) *RemoteManagedSettingsService {
	return &RemoteManagedSettingsService{
		baseURL:      baseURL,
		getToken:     tokenFn,
		pollInterval: time.Duration(pollingIntervalMs) * time.Millisecond,
	}
}

// OnChange registers a callback for when settings change.
func (s *RemoteManagedSettingsService) OnChange(fn func(map[string]any)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onChange = fn
}

// Fetch retrieves remote settings from the API.
// Uses If-None-Match with cached checksum to minimize traffic.
// Source: services/remoteManagedSettings/index.ts:100-180
func (s *RemoteManagedSettingsService) Fetch(ctx context.Context) FetchResult {
	token := s.getToken()
	if token == "" {
		return FetchResult{Success: false, Error: "no auth token", SkipRetry: true}
	}

	url := s.baseURL + "/v1/code/settings"
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return FetchResult{Success: false, Error: err.Error()}
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("anthropic-version", "2023-06-01")

	s.mu.RLock()
	if s.cache != nil && s.cache.Checksum != "" {
		req.Header.Set("If-None-Match", s.cache.Checksum)
	}
	s.mu.RUnlock()

	client := &http.Client{Timeout: time.Duration(settingsTimeoutMs) * time.Millisecond}
	resp, err := client.Do(req)
	if err != nil {
		return FetchResult{Success: false, Error: err.Error()}
	}
	defer resp.Body.Close()

	// 304 Not Modified — cache is valid
	if resp.StatusCode == http.StatusNotModified {
		s.mu.Lock()
		s.lastFetch = time.Now()
		s.mu.Unlock()
		return FetchResult{Success: true, Settings: nil, Checksum: s.cache.Checksum}
	}

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return FetchResult{Success: false, Error: fmt.Sprintf("HTTP %d", resp.StatusCode), SkipRetry: true}
	}

	if resp.StatusCode != http.StatusOK {
		return FetchResult{Success: false, Error: fmt.Sprintf("HTTP %d", resp.StatusCode)}
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return FetchResult{Success: false, Error: err.Error()}
	}

	var parsed RemoteManagedSettingsResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return FetchResult{Success: false, Error: "invalid response: " + err.Error()}
	}

	// Verify checksum
	if parsed.Checksum != "" {
		computed := computeSettingsChecksum(parsed.Settings)
		if computed != parsed.Checksum {
			return FetchResult{Success: false, Error: "checksum mismatch"}
		}
	}

	s.mu.Lock()
	changed := s.cache == nil || s.cache.Checksum != parsed.Checksum
	s.cache = &parsed
	s.lastFetch = time.Now()
	cb := s.onChange
	s.mu.Unlock()

	if changed && cb != nil {
		cb(parsed.Settings)
	}

	return FetchResult{Success: true, Settings: parsed.Settings, Checksum: parsed.Checksum}
}

// GetCachedSettings returns the last successfully fetched settings.
func (s *RemoteManagedSettingsService) GetCachedSettings() map[string]any {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.cache == nil {
		return nil
	}
	return s.cache.Settings
}

// StartPolling begins hourly background polling.
func (s *RemoteManagedSettingsService) StartPolling(ctx context.Context) {
	s.mu.Lock()
	if s.stopPoll != nil {
		s.mu.Unlock()
		return
	}
	s.stopPoll = make(chan struct{})
	s.mu.Unlock()

	go func() {
		ticker := time.NewTicker(s.pollInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				s.Fetch(ctx)
			case <-s.stopPoll:
				return
			case <-ctx.Done():
				return
			}
		}
	}()
}

// StopPolling stops background polling.
func (s *RemoteManagedSettingsService) StopPolling() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stopPoll != nil {
		close(s.stopPoll)
		s.stopPoll = nil
	}
}

func computeSettingsChecksum(settings map[string]any) string {
	data, _ := json.Marshal(settings)
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}
