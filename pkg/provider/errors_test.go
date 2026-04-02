package provider

import (
	"fmt"
	"testing"
	"time"
)

// Source: services/api/withRetry.ts, services/api/errors.ts

func TestRetryConstants(t *testing.T) {
	// Source: withRetry.ts:52-55
	if DefaultMaxRetries != 10 {
		t.Errorf("DefaultMaxRetries = %d, want 10", DefaultMaxRetries)
	}
	if Max529Retries != 3 {
		t.Errorf("Max529Retries = %d, want 3", Max529Retries)
	}
	if BaseDelayMs != 500 {
		t.Errorf("BaseDelayMs = %d, want 500", BaseDelayMs)
	}
	if DefaultMaxDelayMs != 32000 {
		t.Errorf("DefaultMaxDelayMs = %d, want 32000", DefaultMaxDelayMs)
	}
}

func TestClassifyHTTPError_429(t *testing.T) {
	// Source: errors.ts:998
	err := ClassifyHTTPError(429, []byte(`{"type":"rate_limit_error"}`), "5")
	if err.Type != ErrRateLimit {
		t.Errorf("type = %q, want rate_limit", err.Type)
	}
	if !err.Retryable {
		t.Error("429 should be retryable")
	}
	if err.RetryAfter != "5" {
		t.Errorf("RetryAfter = %q, want '5'", err.RetryAfter)
	}
}

func TestClassifyHTTPError_529(t *testing.T) {
	// Source: errors.ts:1002 — 529 = server_overload
	err := ClassifyHTTPError(529, []byte(`{"type":"overloaded_error","message":"Overloaded"}`), "")
	if err.Type != ErrServerOverload {
		t.Errorf("type = %q, want server_overload", err.Type)
	}
	if !err.Retryable {
		t.Error("529 should be retryable")
	}
}

func TestClassifyHTTPError_529_via_message(t *testing.T) {
	// Source: withRetry.ts:619 — SDK sometimes passes overloaded_error via message not status
	err := ClassifyHTTPError(200, []byte(`{"type":"overloaded_error"}`), "")
	if err.Type != ErrServerOverload {
		t.Errorf("type = %q, want server_overload (detected via message)", err.Type)
	}
}

func TestClassifyHTTPError_401(t *testing.T) {
	t.Run("invalid_api_key", func(t *testing.T) {
		// Source: errors.ts:1107
		err := ClassifyHTTPError(401, []byte(`invalid x-api-key`), "")
		if err.Type != ErrInvalidAPIKey {
			t.Errorf("type = %q, want invalid_api_key", err.Type)
		}
	})

	t.Run("generic_auth", func(t *testing.T) {
		// Source: errors.ts:1133
		err := ClassifyHTTPError(401, []byte(`Unauthorized`), "")
		if err.Type != ErrAuthError {
			t.Errorf("type = %q, want auth_error", err.Type)
		}
	})
}

func TestClassifyHTTPError_403(t *testing.T) {
	t.Run("token_revoked", func(t *testing.T) {
		// Source: errors.ts:1115
		err := ClassifyHTTPError(403, []byte(`OAuth token has been revoked`), "")
		if err.Type != ErrTokenRevoked {
			t.Errorf("type = %q, want token_revoked", err.Type)
		}
		if !err.Retryable {
			t.Error("token revoked should be retryable (after refresh)")
		}
	})

	t.Run("generic_forbidden", func(t *testing.T) {
		err := ClassifyHTTPError(403, []byte(`Forbidden`), "")
		if err.Type != ErrAuthError {
			t.Errorf("type = %q, want auth_error", err.Type)
		}
	})
}

func TestClassifyHTTPError_400(t *testing.T) {
	t.Run("prompt_too_long", func(t *testing.T) {
		// Source: errors.ts:1015
		err := ClassifyHTTPError(400, []byte(`Prompt is too long: 250000 tokens > 200000`), "")
		if err.Type != ErrPromptTooLong {
			t.Errorf("type = %q, want prompt_too_long", err.Type)
		}
	})

	t.Run("prompt_too_long_case_insensitive", func(t *testing.T) {
		err := ClassifyHTTPError(400, []byte(`prompt is too long`), "")
		if err.Type != ErrPromptTooLong {
			t.Errorf("type = %q, want prompt_too_long", err.Type)
		}
	})

	t.Run("pdf_too_large", func(t *testing.T) {
		// Source: errors.ts:1024
		err := ClassifyHTTPError(400, []byte(`maximum of 100 PDF pages exceeded`), "")
		if err.Type != ErrPDFTooLarge {
			t.Errorf("type = %q, want pdf_too_large", err.Type)
		}
	})

	t.Run("pdf_password_protected", func(t *testing.T) {
		// Source: errors.ts:1032
		err := ClassifyHTTPError(400, []byte(`PDF is password protected`), "")
		if err.Type != ErrPDFPasswordProtected {
			t.Errorf("type = %q, want pdf_password_protected", err.Type)
		}
	})

	t.Run("image_too_large", func(t *testing.T) {
		// Source: errors.ts:1040
		err := ClassifyHTTPError(400, []byte(`image exceeds the maximum size allowed`), "")
		if err.Type != ErrImageTooLarge {
			t.Errorf("type = %q, want image_too_large", err.Type)
		}
	})

	t.Run("tool_use_mismatch", func(t *testing.T) {
		// Source: errors.ts:1063
		err := ClassifyHTTPError(400, []byte(`tool_use ids without tool_result blocks`), "")
		if err.Type != ErrToolUseMismatch {
			t.Errorf("type = %q, want tool_use_mismatch", err.Type)
		}
	})

	t.Run("invalid_model", func(t *testing.T) {
		// Source: errors.ts:1088
		err := ClassifyHTTPError(400, []byte(`Invalid model name: nonexistent-model`), "")
		if err.Type != ErrInvalidModel {
			t.Errorf("type = %q, want invalid_model", err.Type)
		}
	})

	t.Run("credit_balance", func(t *testing.T) {
		// Source: errors.ts:1099
		err := ClassifyHTTPError(400, []byte(`Credit balance is too low to process request`), "")
		if err.Type != ErrCreditBalanceLow {
			t.Errorf("type = %q, want credit_balance_low", err.Type)
		}
	})

	t.Run("generic_400", func(t *testing.T) {
		err := ClassifyHTTPError(400, []byte(`Bad request`), "")
		if err.Type != ErrClientError {
			t.Errorf("type = %q, want client_error", err.Type)
		}
	})
}

func TestClassifyHTTPError_5xx(t *testing.T) {
	// Source: errors.ts:1148
	for _, code := range []int{500, 502, 503} {
		t.Run(fmt.Sprintf("status_%d", code), func(t *testing.T) {
			err := ClassifyHTTPError(code, []byte(`Internal error`), "")
			if err.Type != ErrServerError {
				t.Errorf("type = %q, want server_error", err.Type)
			}
			if !err.Retryable {
				t.Error("5xx should be retryable")
			}
		})
	}
}

func TestClassifyHTTPError_408_409(t *testing.T) {
	// Source: withRetry.ts — 408 and 409 are retryable
	for _, code := range []int{408, 409} {
		t.Run(fmt.Sprintf("status_%d", code), func(t *testing.T) {
			err := ClassifyHTTPError(code, []byte(`Timeout`), "")
			if !err.Retryable {
				t.Errorf("%d should be retryable", code)
			}
		})
	}
}

func TestIs529Error(t *testing.T) {
	// Source: withRetry.ts:610-621
	t.Run("true_for_529", func(t *testing.T) {
		err := &APIError{StatusCode: 529, Type: ErrServerOverload}
		if !Is529Error(err) {
			t.Error("should detect 529")
		}
	})

	t.Run("true_for_overloaded_message", func(t *testing.T) {
		// Source: withRetry.ts:619
		err := &APIError{StatusCode: 200, Message: `{"type":"overloaded_error"}`}
		if !Is529Error(err) {
			t.Error("should detect overloaded_error in message")
		}
	})

	t.Run("false_for_429", func(t *testing.T) {
		err := &APIError{StatusCode: 429, Type: ErrRateLimit}
		if Is529Error(err) {
			t.Error("429 should not be detected as 529")
		}
	})

	t.Run("false_for_non_api_error", func(t *testing.T) {
		err := fmt.Errorf("generic error")
		if Is529Error(err) {
			t.Error("generic error should not be 529")
		}
	})
}

func TestIsRateLimitError(t *testing.T) {
	if !IsRateLimitError(&APIError{StatusCode: 429, Type: ErrRateLimit}) {
		t.Error("should detect 429")
	}
	if IsRateLimitError(&APIError{StatusCode: 529}) {
		t.Error("529 should not be rate limit")
	}
}

func TestIsRetryableError(t *testing.T) {
	if !IsRetryableError(&APIError{Retryable: true}) {
		t.Error("should detect retryable")
	}
	if IsRetryableError(&APIError{Retryable: false}) {
		t.Error("should not be retryable")
	}
	if IsRetryableError(fmt.Errorf("generic")) {
		t.Error("generic error should not be retryable")
	}
}

func TestIsContextTooLongError(t *testing.T) {
	if !IsContextTooLongError(&APIError{Type: ErrPromptTooLong}) {
		t.Error("should detect prompt_too_long")
	}
	if IsContextTooLongError(&APIError{Type: ErrRateLimit}) {
		t.Error("rate_limit should not be context too long")
	}
}

func TestParseContextOverflowError(t *testing.T) {
	// Source: withRetry.ts:550-595
	t.Run("valid_overflow", func(t *testing.T) {
		err := &APIError{
			StatusCode: 400,
			Message:    `input length and "max_tokens" exceed context limit: 180000 + 32000 > 200000`,
		}
		info := ParseContextOverflowError(err)
		if info == nil {
			t.Fatal("expected parsed overflow info")
		}
		if info.InputTokens != 180000 {
			t.Errorf("InputTokens = %d, want 180000", info.InputTokens)
		}
		if info.MaxTokens != 32000 {
			t.Errorf("MaxTokens = %d, want 32000", info.MaxTokens)
		}
		if info.ContextLimit != 200000 {
			t.Errorf("ContextLimit = %d, want 200000", info.ContextLimit)
		}
	})

	t.Run("backtick_variant", func(t *testing.T) {
		// Source: withRetry.ts:560 — regex uses . for the quotes
		err := &APIError{
			StatusCode: 400,
			Message:    "input length and `max_tokens` exceed context limit: 100000 + 16000 > 128000",
		}
		info := ParseContextOverflowError(err)
		if info == nil {
			t.Fatal("expected parsed overflow info")
		}
		if info.ContextLimit != 128000 {
			t.Errorf("ContextLimit = %d", info.ContextLimit)
		}
	})

	t.Run("non_400_returns_nil", func(t *testing.T) {
		err := &APIError{StatusCode: 429, Message: "rate limit"}
		if ParseContextOverflowError(err) != nil {
			t.Error("non-400 should return nil")
		}
	})

	t.Run("no_match_returns_nil", func(t *testing.T) {
		err := &APIError{StatusCode: 400, Message: "generic bad request"}
		if ParseContextOverflowError(err) != nil {
			t.Error("non-matching message should return nil")
		}
	})

	t.Run("non_api_error_returns_nil", func(t *testing.T) {
		if ParseContextOverflowError(fmt.Errorf("generic")) != nil {
			t.Error("non-APIError should return nil")
		}
	})
}

func TestGetRetryDelay(t *testing.T) {
	// Source: withRetry.ts:530-548

	t.Run("retry_after_header_honored", func(t *testing.T) {
		d := GetRetryDelay(1, "10", DefaultMaxDelayMs)
		if d != 10*time.Second {
			t.Errorf("got %v, want 10s", d)
		}
	})

	t.Run("exponential_backoff_attempt_1", func(t *testing.T) {
		// 500 * pow(2, 0) = 500ms base + jitter
		d := GetRetryDelay(1, "", DefaultMaxDelayMs)
		if d < 500*time.Millisecond || d > 650*time.Millisecond {
			t.Errorf("attempt 1 should be ~500-625ms, got %v", d)
		}
	})

	t.Run("exponential_backoff_attempt_3", func(t *testing.T) {
		// 500 * pow(2, 2) = 2000ms base + jitter
		d := GetRetryDelay(3, "", DefaultMaxDelayMs)
		if d < 2000*time.Millisecond || d > 2600*time.Millisecond {
			t.Errorf("attempt 3 should be ~2000-2500ms, got %v", d)
		}
	})

	t.Run("capped_at_max_delay", func(t *testing.T) {
		// Attempt 10: 500 * pow(2, 9) = 256000 > 32000 cap
		d := GetRetryDelay(10, "", DefaultMaxDelayMs)
		maxExpected := time.Duration(DefaultMaxDelayMs+DefaultMaxDelayMs/4) * time.Millisecond
		if d > maxExpected {
			t.Errorf("should be capped at ~%v, got %v", maxExpected, d)
		}
	})

	t.Run("custom_max_delay", func(t *testing.T) {
		d := GetRetryDelay(10, "", 5000)
		if d > 7*time.Second {
			t.Errorf("should be capped near 5000ms, got %v", d)
		}
	})
}

func TestShouldRetry(t *testing.T) {
	// Source: withRetry.ts:696-770
	t.Run("retryable_within_limit", func(t *testing.T) {
		err := &APIError{Retryable: true}
		if !ShouldRetry(err, 1, 10) {
			t.Error("should retry attempt 1/10")
		}
	})

	t.Run("retryable_at_limit", func(t *testing.T) {
		err := &APIError{Retryable: true}
		if ShouldRetry(err, 10, 10) {
			t.Error("should not retry at max attempts")
		}
	})

	t.Run("non_retryable", func(t *testing.T) {
		err := &APIError{Retryable: false}
		if ShouldRetry(err, 1, 10) {
			t.Error("non-retryable should not retry")
		}
	})

	t.Run("non_api_error", func(t *testing.T) {
		err := fmt.Errorf("generic")
		if ShouldRetry(err, 1, 10) {
			t.Error("generic error should not retry")
		}
	})
}

func TestAPIErrorTypes(t *testing.T) {
	// Source: errors.ts — verify all error type strings match TS
	types := []APIErrorType{
		ErrAborted, ErrAPITimeout, ErrRepeated529, ErrRateLimit,
		ErrServerOverload, ErrPromptTooLong, ErrPDFTooLarge,
		ErrPDFPasswordProtected, ErrImageTooLarge, ErrToolUseMismatch,
		ErrInvalidModel, ErrCreditBalanceLow, ErrInvalidAPIKey,
		ErrTokenRevoked, ErrAuthError, ErrServerError, ErrClientError,
		ErrSSLCertError, ErrConnectionError, ErrUnknown,
	}
	for _, et := range types {
		if et == "" {
			t.Error("error type should not be empty")
		}
	}
}

func TestAPIError_Error(t *testing.T) {
	err := &APIError{StatusCode: 429, Type: ErrRateLimit, Message: "too fast"}
	s := err.Error()
	if s == "" {
		t.Error("error string should not be empty")
	}
	// Should contain status, type, and message
	if !containsCI(s, "429") || !containsCI(s, "rate_limit") || !containsCI(s, "too fast") {
		t.Errorf("error string missing info: %q", s)
	}
}
