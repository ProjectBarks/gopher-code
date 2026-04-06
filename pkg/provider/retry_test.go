package provider

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// Source: services/api/withRetry.ts

func TestGetDefaultMaxRetries_EnvOverride(t *testing.T) {
	// Source: withRetry.ts:789-794 — CLAUDE_CODE_MAX_RETRIES env var
	t.Run("default_is_10", func(t *testing.T) {
		os.Unsetenv("CLAUDE_CODE_MAX_RETRIES")
		if got := GetDefaultMaxRetries(); got != 10 {
			t.Errorf("GetDefaultMaxRetries() = %d, want 10", got)
		}
	})

	t.Run("env_override", func(t *testing.T) {
		t.Setenv("CLAUDE_CODE_MAX_RETRIES", "5")
		if got := GetDefaultMaxRetries(); got != 5 {
			t.Errorf("GetDefaultMaxRetries() = %d, want 5", got)
		}
	})

	t.Run("invalid_env_uses_default", func(t *testing.T) {
		t.Setenv("CLAUDE_CODE_MAX_RETRIES", "notanumber")
		if got := GetDefaultMaxRetries(); got != 10 {
			t.Errorf("GetDefaultMaxRetries() = %d, want 10 on invalid env", got)
		}
	})
}

func TestPersistentMaxBackoffMs(t *testing.T) {
	// Source: withRetry.ts:96 — PERSISTENT_MAX_BACKOFF_MS = 5 * 60 * 1000
	if PersistentMaxBackoffMs != 5*60*1000 {
		t.Errorf("PersistentMaxBackoffMs = %d, want %d", PersistentMaxBackoffMs, 5*60*1000)
	}
}

func TestCannotRetryError(t *testing.T) {
	// Source: withRetry.ts:144-158
	orig := fmt.Errorf("original problem")
	ctx := RetryContext{Model: "claude-sonnet-4-20250514"}
	err := NewCannotRetryError(orig, ctx)

	if err.Error() != "original problem" {
		t.Errorf("Error() = %q, want 'original problem'", err.Error())
	}
	if err.OriginalError != orig {
		t.Error("OriginalError not preserved")
	}
	if err.Context.Model != "claude-sonnet-4-20250514" {
		t.Error("RetryContext not preserved")
	}
}

func TestFallbackTriggeredError(t *testing.T) {
	// Source: withRetry.ts:160-168
	err := NewFallbackTriggeredError("claude-opus-4-20250514", "claude-sonnet-4-20250514")

	if !strings.Contains(err.Error(), "claude-opus-4-20250514") {
		t.Errorf("Error() should mention original model: %q", err.Error())
	}
	if !strings.Contains(err.Error(), "claude-sonnet-4-20250514") {
		t.Errorf("Error() should mention fallback model: %q", err.Error())
	}
	if err.OriginalModel != "claude-opus-4-20250514" {
		t.Error("OriginalModel not set")
	}
	if err.FallbackModel != "claude-sonnet-4-20250514" {
		t.Error("FallbackModel not set")
	}
}

func TestShouldRetryHeader(t *testing.T) {
	// Source: withRetry.ts:732 — x-should-retry header
	t.Run("true_means_retry", func(t *testing.T) {
		h := http.Header{}
		h.Set("x-should-retry", "true")
		if got := CheckShouldRetryHeader(h); got != RetryHeaderYes {
			t.Errorf("got %v, want RetryHeaderYes", got)
		}
	})

	t.Run("false_means_no_retry", func(t *testing.T) {
		h := http.Header{}
		h.Set("x-should-retry", "false")
		if got := CheckShouldRetryHeader(h); got != RetryHeaderNo {
			t.Errorf("got %v, want RetryHeaderNo", got)
		}
	})

	t.Run("absent_means_default", func(t *testing.T) {
		h := http.Header{}
		if got := CheckShouldRetryHeader(h); got != RetryHeaderAbsent {
			t.Errorf("got %v, want RetryHeaderAbsent", got)
		}
	})
}


func TestWithRetry_SuccessOnFirstAttempt(t *testing.T) {
	result, err := WithRetry(context.Background(), func(attempt int, ctx RetryContext) (*http.Response, error) {
		return &http.Response{StatusCode: 200}, nil
	}, RetryOptions{
		MaxRetries: 3,
		Model:      "claude-sonnet-4-20250514",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.StatusCode != 200 {
		t.Errorf("StatusCode = %d, want 200", result.StatusCode)
	}
}

func TestWithRetry_RetriesOn529(t *testing.T) {
	// Source: withRetry.ts — 529 overloaded should be retried
	var attempts int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&attempts, 1)
		if n <= 2 {
			w.WriteHeader(529)
			w.Write([]byte(`{"type":"overloaded_error","message":"Overloaded"}`))
			return
		}
		w.WriteHeader(200)
		w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	result, err := WithRetry(context.Background(), func(attempt int, ctx RetryContext) (*http.Response, error) {
		resp, err := http.Get(srv.URL)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode == 529 {
			return nil, &APIError{StatusCode: 529, Message: `{"type":"overloaded_error"}`, Type: ErrServerOverload, Retryable: true}
		}
		return resp, nil
	}, RetryOptions{
		MaxRetries:  5,
		Model:       "claude-sonnet-4-20250514",
		QuerySource: QuerySourceREPLMainThread,
		BaseDelay:   10 * time.Millisecond, // fast for tests
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.StatusCode != 200 {
		t.Errorf("StatusCode = %d, want 200", result.StatusCode)
	}
	if atomic.LoadInt32(&attempts) != 3 {
		t.Errorf("expected 3 attempts, got %d", atomic.LoadInt32(&attempts))
	}
}

func TestWithRetry_MaxRetriesExceeded(t *testing.T) {
	// After maxRetries+1 attempts, should get CannotRetryError
	var attempts int32

	_, err := WithRetry(context.Background(), func(attempt int, ctx RetryContext) (*http.Response, error) {
		atomic.AddInt32(&attempts, 1)
		return nil, &APIError{StatusCode: 500, Message: "server error", Type: ErrServerError, Retryable: true}
	}, RetryOptions{
		MaxRetries: 3,
		Model:      "claude-sonnet-4-20250514",
		BaseDelay:  1 * time.Millisecond,
	})

	if err == nil {
		t.Fatal("expected error after max retries")
	}

	cannotRetry, ok := err.(*CannotRetryError)
	if !ok {
		t.Fatalf("expected *CannotRetryError, got %T: %v", err, err)
	}
	if cannotRetry.Context.Model != "claude-sonnet-4-20250514" {
		t.Errorf("model = %q", cannotRetry.Context.Model)
	}

	// maxRetries=3 means 1 initial + 3 retries = 4 total attempts
	if got := atomic.LoadInt32(&attempts); got != 4 {
		t.Errorf("expected 4 attempts (1 + 3 retries), got %d", got)
	}
}

func TestWithRetry_RetryAfterHeaderRespected(t *testing.T) {
	// Source: withRetry.ts:530-548 — retry-after header in seconds
	var attempts int32
	start := time.Now()

	_, err := WithRetry(context.Background(), func(attempt int, ctx RetryContext) (*http.Response, error) {
		n := atomic.AddInt32(&attempts, 1)
		if n == 1 {
			return nil, &APIError{
				StatusCode: 429,
				Message:    "rate limited",
				Type:       ErrRateLimit,
				Retryable:  true,
				RetryAfter: "1", // 1 second
			}
		}
		return &http.Response{StatusCode: 200}, nil
	}, RetryOptions{
		MaxRetries: 3,
		Model:      "claude-sonnet-4-20250514",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	elapsed := time.Since(start)
	// Should have waited ~1 second for the retry-after header
	if elapsed < 900*time.Millisecond {
		t.Errorf("expected ~1s delay from retry-after, got %v", elapsed)
	}
}

func TestWithRetry_JitterBounds(t *testing.T) {
	// Source: withRetry.ts:542-547 — jitter = rand * 0.25 * baseDelay
	// For attempt=1: baseDelay=500ms, jitter in [0, 125ms]
	// So total in [500ms, 625ms]
	for i := 0; i < 100; i++ {
		d := GetRetryDelay(1, "", DefaultMaxDelayMs)
		if d < 500*time.Millisecond || d > 625*time.Millisecond {
			t.Fatalf("attempt=1 delay %v outside [500ms, 625ms]", d)
		}
	}

	// For attempt=3: baseDelay=2000ms, jitter in [0, 500ms]
	// So total in [2000ms, 2500ms]
	for i := 0; i < 100; i++ {
		d := GetRetryDelay(3, "", DefaultMaxDelayMs)
		if d < 2000*time.Millisecond || d > 2500*time.Millisecond {
			t.Fatalf("attempt=3 delay %v outside [2000ms, 2500ms]", d)
		}
	}
}

func TestWithRetry_NonRetryableErrorNotRetried(t *testing.T) {
	var attempts int32

	_, err := WithRetry(context.Background(), func(attempt int, ctx RetryContext) (*http.Response, error) {
		atomic.AddInt32(&attempts, 1)
		return nil, &APIError{StatusCode: 400, Message: "bad request", Type: ErrClientError, Retryable: false}
	}, RetryOptions{
		MaxRetries: 3,
		Model:      "claude-sonnet-4-20250514",
		BaseDelay:  1 * time.Millisecond,
	})

	if err == nil {
		t.Fatal("expected error")
	}
	if _, ok := err.(*CannotRetryError); !ok {
		t.Errorf("expected *CannotRetryError, got %T", err)
	}
	if got := atomic.LoadInt32(&attempts); got != 1 {
		t.Errorf("non-retryable should only attempt once, got %d", got)
	}
}

func TestWithRetry_CancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	_, err := WithRetry(ctx, func(attempt int, retryCtx RetryContext) (*http.Response, error) {
		return nil, &APIError{StatusCode: 500, Retryable: true}
	}, RetryOptions{
		MaxRetries: 3,
		Model:      "claude-sonnet-4-20250514",
		BaseDelay:  1 * time.Millisecond,
	})

	if err == nil {
		t.Fatal("expected error on cancelled context")
	}
	if !strings.Contains(err.Error(), "cancel") && !strings.Contains(err.Error(), "context") {
		// Could be a CannotRetryError wrapping context error, or context.Canceled directly
		t.Logf("error type: %T, value: %v", err, err)
	}
}

func TestWithRetry_FallbackTriggeredAfter529(t *testing.T) {
	// Source: withRetry.ts:327-348 — after MAX_529_RETRIES consecutive 529s, trigger fallback
	var attempts int32

	_, err := WithRetry(context.Background(), func(attempt int, ctx RetryContext) (*http.Response, error) {
		atomic.AddInt32(&attempts, 1)
		return nil, &APIError{StatusCode: 529, Message: `{"type":"overloaded_error"}`, Type: ErrServerOverload, Retryable: true}
	}, RetryOptions{
		MaxRetries:    10,
		Model:         "claude-opus-4-20250514",
		FallbackModel: "claude-sonnet-4-20250514",
		QuerySource:   QuerySourceREPLMainThread,
		BaseDelay:     1 * time.Millisecond,
	})

	if err == nil {
		t.Fatal("expected FallbackTriggeredError")
	}
	fbErr, ok := err.(*FallbackTriggeredError)
	if !ok {
		t.Fatalf("expected *FallbackTriggeredError, got %T: %v", err, err)
	}
	if fbErr.OriginalModel != "claude-opus-4-20250514" {
		t.Errorf("OriginalModel = %q", fbErr.OriginalModel)
	}
	if fbErr.FallbackModel != "claude-sonnet-4-20250514" {
		t.Errorf("FallbackModel = %q", fbErr.FallbackModel)
	}
	// Should have tried Max529Retries times before triggering fallback
	if got := atomic.LoadInt32(&attempts); got != int32(Max529Retries) {
		t.Errorf("expected %d attempts before fallback, got %d", Max529Retries, got)
	}
}

func TestWithRetry_Background529NotRetried(t *testing.T) {
	// Source: withRetry.ts:318-324 — non-foreground sources bail immediately on 529
	var attempts int32

	_, err := WithRetry(context.Background(), func(attempt int, ctx RetryContext) (*http.Response, error) {
		atomic.AddInt32(&attempts, 1)
		return nil, &APIError{StatusCode: 529, Message: `{"type":"overloaded_error"}`, Type: ErrServerOverload, Retryable: true}
	}, RetryOptions{
		MaxRetries:  5,
		Model:       "claude-sonnet-4-20250514",
		QuerySource: QuerySource("title_gen"), // background source
		BaseDelay:   1 * time.Millisecond,
	})

	if err == nil {
		t.Fatal("expected error")
	}
	if _, ok := err.(*CannotRetryError); !ok {
		t.Fatalf("expected *CannotRetryError, got %T", err)
	}
	if got := atomic.LoadInt32(&attempts); got != 1 {
		t.Errorf("background 529 should not retry, got %d attempts", got)
	}
}

func TestWithRetry_ShouldRetryHeaderFalseVetoes(t *testing.T) {
	// Source: withRetry.ts:732 — x-should-retry: false vetoes retry
	var attempts int32

	_, err := WithRetry(context.Background(), func(attempt int, ctx RetryContext) (*http.Response, error) {
		atomic.AddInt32(&attempts, 1)
		return nil, &APIError{
			StatusCode:       429,
			Message:          "rate limited",
			Type:             ErrRateLimit,
			Retryable:        true,
			ShouldRetryHeader: "false",
		}
	}, RetryOptions{
		MaxRetries: 5,
		Model:      "claude-sonnet-4-20250514",
		BaseDelay:  1 * time.Millisecond,
	})

	if err == nil {
		t.Fatal("expected error")
	}
	if _, ok := err.(*CannotRetryError); !ok {
		t.Fatalf("expected *CannotRetryError, got %T", err)
	}
	if got := atomic.LoadInt32(&attempts); got != 1 {
		t.Errorf("x-should-retry:false should veto retry, got %d attempts", got)
	}
}
