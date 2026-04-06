package provider

import (
	"fmt"
	"strings"
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
		ErrPDFPasswordProtected, ErrPDFInvalid, ErrImageTooLarge,
		ErrRequestTooLarge, ErrToolUseMismatch,
		ErrInvalidModel, ErrCreditBalanceLow, ErrInvalidAPIKey,
		ErrTokenRevoked, ErrOrgDisabled, ErrOAuthOrgNotAllowed,
		ErrAuthError, ErrServerError, ErrClientError,
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

// === T478: API error classification and user-facing messages ===

func TestErrorMessageConstants(t *testing.T) {
	// Source: errors.ts:54-169 — verbatim string constants
	tests := []struct {
		name string
		got  string
		want string
	}{
		{"APIErrorMessagePrefix", APIErrorMessagePrefix, "API Error"},
		{"PromptTooLongErrorMessage", PromptTooLongErrorMessage, "Prompt is too long"},
		{"CreditBalanceTooLowErrorMessage", CreditBalanceTooLowErrorMessage, "Credit balance is too low"},
		{"InvalidAPIKeyErrorMessage", InvalidAPIKeyErrorMessage, "Not logged in · Please run /login"},
		{"InvalidAPIKeyErrorMessageExternal", InvalidAPIKeyErrorMessageExternal, "Invalid API key · Fix external API key"},
		{"OrgDisabledErrorMessageEnvKeyWithOAuth", OrgDisabledErrorMessageEnvKeyWithOAuth,
			"Your ANTHROPIC_API_KEY belongs to a disabled organization · Unset the environment variable to use your subscription instead"},
		{"OrgDisabledErrorMessageEnvKey", OrgDisabledErrorMessageEnvKey,
			"Your ANTHROPIC_API_KEY belongs to a disabled organization · Update or unset the environment variable"},
		{"TokenRevokedErrorMessage", TokenRevokedErrorMessage, "OAuth token revoked · Please run /login"},
		{"CCRAuthErrorMessage", CCRAuthErrorMessage,
			"Authentication error · This may be a temporary network issue, please try again"},
		{"Repeated529ErrorMessage", Repeated529ErrorMessage, "Repeated 529 Overloaded errors"},
		{"CustomOffSwitchMessage", CustomOffSwitchMessage,
			"Opus is experiencing high load, please use /model to switch to Sonnet"},
		{"APITimeoutErrorMessage", APITimeoutErrorMessage, "Request timed out"},
		{"OAuthOrgNotAllowedErrorMessage", OAuthOrgNotAllowedErrorMessage,
			"Your account does not have access to Claude Code. Please run /login."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("got %q, want %q", tt.got, tt.want)
			}
		})
	}
}

func TestNewErrorTypes(t *testing.T) {
	// Source: errors.ts — new error type strings added in T478
	if ErrPDFInvalid != "pdf_invalid" {
		t.Errorf("ErrPDFInvalid = %q", ErrPDFInvalid)
	}
	if ErrRequestTooLarge != "request_too_large" {
		t.Errorf("ErrRequestTooLarge = %q", ErrRequestTooLarge)
	}
	if ErrOrgDisabled != "org_disabled" {
		t.Errorf("ErrOrgDisabled = %q", ErrOrgDisabled)
	}
	if ErrOAuthOrgNotAllowed != "oauth_org_not_allowed" {
		t.Errorf("ErrOAuthOrgNotAllowed = %q", ErrOAuthOrgNotAllowed)
	}
}

func TestIsOverloadedError(t *testing.T) {
	// Source: errors.ts — 529 or overloaded_error type
	t.Run("529_is_overloaded", func(t *testing.T) {
		err := &APIError{StatusCode: 529, Type: ErrServerOverload}
		if !IsOverloadedError(err) {
			t.Error("529 should be overloaded")
		}
	})
	t.Run("overloaded_error_in_message", func(t *testing.T) {
		err := &APIError{StatusCode: 200, Type: ErrServerOverload}
		if !IsOverloadedError(err) {
			t.Error("server_overload type should be overloaded")
		}
	})
	t.Run("429_is_not_overloaded", func(t *testing.T) {
		err := &APIError{StatusCode: 429, Type: ErrRateLimit}
		if IsOverloadedError(err) {
			t.Error("429 should not be overloaded")
		}
	})
	t.Run("non_api_error", func(t *testing.T) {
		if IsOverloadedError(fmt.Errorf("generic")) {
			t.Error("generic error should not be overloaded")
		}
	})
}

func TestIsBillingError(t *testing.T) {
	// Source: errors.ts — credit_balance_low classification
	t.Run("credit_balance_low", func(t *testing.T) {
		err := &APIError{Type: ErrCreditBalanceLow}
		if !IsBillingError(err) {
			t.Error("credit_balance_low should be billing error")
		}
	})
	t.Run("rate_limit_not_billing", func(t *testing.T) {
		err := &APIError{Type: ErrRateLimit}
		if IsBillingError(err) {
			t.Error("rate_limit should not be billing error")
		}
	})
	t.Run("non_api_error", func(t *testing.T) {
		if IsBillingError(fmt.Errorf("generic")) {
			t.Error("generic error should not be billing")
		}
	})
}

func TestIsInvalidRequestError(t *testing.T) {
	// Source: errors.ts — client_error type is invalid request
	t.Run("client_error", func(t *testing.T) {
		err := &APIError{Type: ErrClientError}
		if !IsInvalidRequestError(err) {
			t.Error("client_error should be invalid request")
		}
	})
	t.Run("invalid_model", func(t *testing.T) {
		err := &APIError{Type: ErrInvalidModel}
		if !IsInvalidRequestError(err) {
			t.Error("invalid_model should be invalid request")
		}
	})
	t.Run("tool_use_mismatch", func(t *testing.T) {
		err := &APIError{Type: ErrToolUseMismatch}
		if !IsInvalidRequestError(err) {
			t.Error("tool_use_mismatch should be invalid request")
		}
	})
	t.Run("rate_limit_not_invalid_request", func(t *testing.T) {
		err := &APIError{Type: ErrRateLimit}
		if IsInvalidRequestError(err) {
			t.Error("rate_limit should not be invalid request")
		}
	})
	t.Run("non_api_error", func(t *testing.T) {
		if IsInvalidRequestError(fmt.Errorf("generic")) {
			t.Error("generic error should not be invalid request")
		}
	})
}

func TestIsContextWindowError(t *testing.T) {
	// Source: errors.ts — prompt_too_long is context window error
	t.Run("prompt_too_long", func(t *testing.T) {
		err := &APIError{Type: ErrPromptTooLong}
		if !IsContextWindowError(err) {
			t.Error("prompt_too_long should be context window")
		}
	})
	t.Run("rate_limit_not_context_window", func(t *testing.T) {
		err := &APIError{Type: ErrRateLimit}
		if IsContextWindowError(err) {
			t.Error("rate_limit should not be context window")
		}
	})
	t.Run("non_api_error", func(t *testing.T) {
		if IsContextWindowError(fmt.Errorf("generic")) {
			t.Error("generic error should not be context window")
		}
	})
}

func TestStartsWithAPIErrorPrefix(t *testing.T) {
	// Source: errors.ts:56-61
	t.Run("starts_with_prefix", func(t *testing.T) {
		if !StartsWithAPIErrorPrefix("API Error: something broke") {
			t.Error("should match API Error prefix")
		}
	})
	t.Run("starts_with_login_prefix", func(t *testing.T) {
		if !StartsWithAPIErrorPrefix("Please run /login · API Error: 401") {
			t.Error("should match login+API Error prefix")
		}
	})
	t.Run("no_match", func(t *testing.T) {
		if StartsWithAPIErrorPrefix("Something else happened") {
			t.Error("should not match unrelated text")
		}
	})
}

func TestParsePromptTooLongTokenCounts(t *testing.T) {
	// Source: errors.ts:85-96
	t.Run("standard_format", func(t *testing.T) {
		actual, limit := ParsePromptTooLongTokenCounts(
			"prompt is too long: 137500 tokens > 135000 maximum",
		)
		if actual != 137500 {
			t.Errorf("actual = %d, want 137500", actual)
		}
		if limit != 135000 {
			t.Errorf("limit = %d, want 135000", limit)
		}
	})
	t.Run("case_insensitive", func(t *testing.T) {
		actual, limit := ParsePromptTooLongTokenCounts(
			"Prompt Is Too Long: 250000 tokens > 200000 maximum",
		)
		if actual != 250000 || limit != 200000 {
			t.Errorf("got %d/%d, want 250000/200000", actual, limit)
		}
	})
	t.Run("no_match_returns_zero", func(t *testing.T) {
		actual, limit := ParsePromptTooLongTokenCounts("generic error")
		if actual != 0 || limit != 0 {
			t.Errorf("expected 0/0 for non-matching, got %d/%d", actual, limit)
		}
	})
	t.Run("wrapped_in_sdk_prefix", func(t *testing.T) {
		actual, limit := ParsePromptTooLongTokenCounts(
			`400 {"error":{"type":"invalid_request_error","message":"prompt is too long: 137500 tokens > 135000 maximum"}}`,
		)
		if actual != 137500 || limit != 135000 {
			t.Errorf("got %d/%d, want 137500/135000", actual, limit)
		}
	})
}

func TestGetPromptTooLongTokenGap(t *testing.T) {
	// Source: errors.ts:104-118
	t.Run("positive_gap", func(t *testing.T) {
		gap := GetPromptTooLongTokenGap("prompt is too long: 137500 tokens > 135000 maximum")
		if gap != 2500 {
			t.Errorf("gap = %d, want 2500", gap)
		}
	})
	t.Run("no_gap_when_under_limit", func(t *testing.T) {
		gap := GetPromptTooLongTokenGap("prompt is too long: 100 tokens > 200 maximum")
		if gap != 0 {
			t.Errorf("gap = %d, want 0 (under limit)", gap)
		}
	})
	t.Run("no_gap_on_non_matching", func(t *testing.T) {
		gap := GetPromptTooLongTokenGap("generic error")
		if gap != 0 {
			t.Errorf("gap = %d, want 0", gap)
		}
	})
}

func TestIsMediaSizeError(t *testing.T) {
	// Source: errors.ts:133-139
	t.Run("image_exceeds_maximum", func(t *testing.T) {
		if !IsMediaSizeError("image exceeds the maximum allowed size") {
			t.Error("should detect image exceeds + maximum")
		}
	})
	t.Run("pdf_pages", func(t *testing.T) {
		if !IsMediaSizeError("maximum of 100 PDF pages exceeded") {
			t.Error("should detect PDF pages pattern")
		}
	})
	t.Run("image_dimensions_many_image", func(t *testing.T) {
		if !IsMediaSizeError("image dimensions exceed the limit in many-image request") {
			t.Error("should detect image dimensions exceed + many-image")
		}
	})
	t.Run("unrelated_message", func(t *testing.T) {
		if IsMediaSizeError("generic error") {
			t.Error("should not match generic error")
		}
	})
}

func TestClassifyHTTPError_403_OrgDisabled(t *testing.T) {
	// Source: errors.ts — 403 with "organization has been disabled"
	err := ClassifyHTTPError(403, []byte(`Your organization has been disabled`), "")
	if err.Type != ErrOrgDisabled {
		t.Errorf("type = %q, want org_disabled", err.Type)
	}
}

func TestClassifyHTTPError_403_OAuthOrgNotAllowed(t *testing.T) {
	// Source: errors.ts — 403 with "does not have access to Claude Code"
	err := ClassifyHTTPError(403, []byte(`Your account does not have access to Claude Code`), "")
	if err.Type != ErrOAuthOrgNotAllowed {
		t.Errorf("type = %q, want oauth_org_not_allowed", err.Type)
	}
}

func TestUserFacingMessage(t *testing.T) {
	// Source: errors.ts — UserFacingMessage maps error types to display strings
	tests := []struct {
		name    string
		err     *APIError
		want    string
	}{
		{
			"rate_limit",
			&APIError{Type: ErrRateLimit, StatusCode: 429},
			APIErrorMessagePrefix,
		},
		{
			"overload_529",
			&APIError{Type: ErrServerOverload, StatusCode: 529},
			APIErrorMessagePrefix,
		},
		{
			"prompt_too_long",
			&APIError{Type: ErrPromptTooLong, StatusCode: 400, Message: "prompt is too long: 137500 tokens > 135000 maximum"},
			PromptTooLongErrorMessage,
		},
		{
			"credit_balance_low",
			&APIError{Type: ErrCreditBalanceLow, StatusCode: 400},
			CreditBalanceTooLowErrorMessage,
		},
		{
			"invalid_api_key",
			&APIError{Type: ErrInvalidAPIKey, StatusCode: 401},
			InvalidAPIKeyErrorMessage,
		},
		{
			"token_revoked",
			&APIError{Type: ErrTokenRevoked, StatusCode: 403},
			TokenRevokedErrorMessage,
		},
		{
			"repeated_529",
			&APIError{Type: ErrRepeated529},
			Repeated529ErrorMessage,
		},
		{
			"api_timeout",
			&APIError{Type: ErrAPITimeout},
			APITimeoutErrorMessage,
		},
		{
			"unknown_uses_prefix",
			&APIError{Type: ErrUnknown, StatusCode: 418, Message: "teapot"},
			APIErrorMessagePrefix,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.err.UserFacingMessage()
			if !strings.HasPrefix(got, tt.want) {
				t.Errorf("UserFacingMessage() = %q, want prefix %q", got, tt.want)
			}
		})
	}
}

func TestGetPdfTooLargeErrorMessage(t *testing.T) {
	// Source: errors.ts:170-175
	interactive := GetPDFTooLargeErrorMessage(false)
	if !strings.Contains(interactive, "PDF too large") {
		t.Errorf("interactive message missing 'PDF too large': %q", interactive)
	}
	if !strings.Contains(interactive, "esc") {
		t.Errorf("interactive message should mention esc: %q", interactive)
	}
	nonInteractive := GetPDFTooLargeErrorMessage(true)
	if !strings.Contains(nonInteractive, "PDF too large") {
		t.Errorf("non-interactive message missing 'PDF too large': %q", nonInteractive)
	}
	if !strings.Contains(nonInteractive, "pdftotext") {
		t.Errorf("non-interactive message should mention pdftotext: %q", nonInteractive)
	}
}

func TestGetPdfPasswordProtectedErrorMessage(t *testing.T) {
	// Source: errors.ts:176-179
	interactive := GetPDFPasswordProtectedErrorMessage(false)
	if !strings.Contains(interactive, "password protected") {
		t.Errorf("missing 'password protected': %q", interactive)
	}
	nonInteractive := GetPDFPasswordProtectedErrorMessage(true)
	if !strings.Contains(nonInteractive, "password protected") {
		t.Errorf("missing 'password protected': %q", nonInteractive)
	}
}

func TestGetPdfInvalidErrorMessage(t *testing.T) {
	// Source: errors.ts:181-184
	interactive := GetPDFInvalidErrorMessage(false)
	if !strings.Contains(interactive, "not valid") {
		t.Errorf("missing 'not valid': %q", interactive)
	}
	nonInteractive := GetPDFInvalidErrorMessage(true)
	if !strings.Contains(nonInteractive, "not valid") {
		t.Errorf("missing 'not valid': %q", nonInteractive)
	}
}

func TestGetImageTooLargeErrorMessage(t *testing.T) {
	// Source: errors.ts:186-189
	interactive := GetImageTooLargeErrorMessage(false)
	if !strings.Contains(interactive, "too large") {
		t.Errorf("missing 'too large': %q", interactive)
	}
	nonInteractive := GetImageTooLargeErrorMessage(true)
	if !strings.Contains(nonInteractive, "too large") {
		t.Errorf("missing 'too large': %q", nonInteractive)
	}
}

func TestGetRequestTooLargeErrorMessage(t *testing.T) {
	// Source: errors.ts:191-195
	interactive := GetRequestTooLargeErrorMessage(false)
	if !strings.Contains(interactive, "Request too large") {
		t.Errorf("missing 'Request too large': %q", interactive)
	}
	nonInteractive := GetRequestTooLargeErrorMessage(true)
	if !strings.Contains(nonInteractive, "Request too large") {
		t.Errorf("missing 'Request too large': %q", nonInteractive)
	}
}
