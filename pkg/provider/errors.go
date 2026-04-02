package provider

import (
	"fmt"
	"math"
	"math/rand"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Source: services/api/withRetry.ts, services/api/errors.ts

// Retry constants matching TS exactly.
// Source: withRetry.ts:52-55
const (
	DefaultMaxRetries = 10   // Source: withRetry.ts:52
	Max529Retries     = 3    // Source: withRetry.ts:54
	BaseDelayMs       = 500  // Source: withRetry.ts:55
	DefaultMaxDelayMs = 32000 // Source: withRetry.ts:533
)

// APIErrorType classifies the API error for display and retry logic.
// Source: services/api/errors.ts:965-1163
type APIErrorType string

const (
	ErrAborted             APIErrorType = "aborted"
	ErrAPITimeout          APIErrorType = "api_timeout"
	ErrRepeated529         APIErrorType = "repeated_529"
	ErrRateLimit           APIErrorType = "rate_limit"
	ErrServerOverload      APIErrorType = "server_overload"
	ErrPromptTooLong       APIErrorType = "prompt_too_long"
	ErrPDFTooLarge         APIErrorType = "pdf_too_large"
	ErrPDFPasswordProtected APIErrorType = "pdf_password_protected"
	ErrImageTooLarge       APIErrorType = "image_too_large"
	ErrToolUseMismatch     APIErrorType = "tool_use_mismatch"
	ErrInvalidModel        APIErrorType = "invalid_model"
	ErrCreditBalanceLow    APIErrorType = "credit_balance_low"
	ErrInvalidAPIKey       APIErrorType = "invalid_api_key"
	ErrTokenRevoked        APIErrorType = "token_revoked"
	ErrAuthError           APIErrorType = "auth_error"
	ErrServerError         APIErrorType = "server_error"
	ErrClientError         APIErrorType = "client_error"
	ErrSSLCertError        APIErrorType = "ssl_cert_error"
	ErrConnectionError     APIErrorType = "connection_error"
	ErrUnknown             APIErrorType = "unknown"
)

// APIError represents a classified error from the Anthropic API.
// Source: services/api/errors.ts
type APIError struct {
	StatusCode int
	Message    string
	Type       APIErrorType
	RetryAfter string // Retry-After header value (seconds)
	Retryable  bool
}

func (e *APIError) Error() string {
	return fmt.Sprintf("API error (%d, %s): %s", e.StatusCode, e.Type, e.Message)
}

// ClassifyHTTPError creates a typed APIError from an HTTP response.
// Source: services/api/errors.ts:965-1163
func ClassifyHTTPError(statusCode int, body []byte, retryAfter string) *APIError {
	msg := string(body)

	err := &APIError{
		StatusCode: statusCode,
		Message:    msg,
		RetryAfter: retryAfter,
	}

	// Classify — Source: errors.ts:965-1163
	switch {
	case statusCode == 429:
		err.Type = ErrRateLimit
		err.Retryable = true

	case statusCode == 529 || strings.Contains(msg, `"type":"overloaded_error"`):
		// Source: withRetry.ts:610-621
		err.Type = ErrServerOverload
		err.Retryable = true

	case statusCode == 401:
		if strings.Contains(msg, "x-api-key") {
			err.Type = ErrInvalidAPIKey
		} else {
			err.Type = ErrAuthError
		}

	case statusCode == 403:
		if strings.Contains(msg, "OAuth token has been revoked") {
			err.Type = ErrTokenRevoked
			err.Retryable = true // retry after refresh
		} else {
			err.Type = ErrAuthError
		}

	case statusCode == 400:
		switch {
		case containsCI(msg, "prompt is too long"):
			err.Type = ErrPromptTooLong
		case regexp.MustCompile(`maximum of \d+ PDF pages`).MatchString(msg):
			err.Type = ErrPDFTooLarge
		case strings.Contains(msg, "password protected"):
			err.Type = ErrPDFPasswordProtected
		case strings.Contains(msg, "image exceeds") && strings.Contains(msg, "maximum"):
			err.Type = ErrImageTooLarge
		case strings.Contains(msg, "tool_use ids without tool_result"):
			err.Type = ErrToolUseMismatch
		case containsCI(msg, "invalid model"):
			err.Type = ErrInvalidModel
		case strings.Contains(msg, "Credit balance is too low"):
			err.Type = ErrCreditBalanceLow
		default:
			err.Type = ErrClientError
		}

	case statusCode == 408 || statusCode == 409:
		err.Type = ErrServerError
		err.Retryable = true

	case statusCode >= 500:
		err.Type = ErrServerError
		err.Retryable = true

	default:
		err.Type = ErrClientError
	}

	return err
}

// Is529Error checks if an error is a 529 overloaded error.
// Source: withRetry.ts:610-621
func Is529Error(err error) bool {
	if apiErr, ok := err.(*APIError); ok {
		return apiErr.StatusCode == 529 ||
			strings.Contains(apiErr.Message, `"type":"overloaded_error"`)
	}
	return false
}

// IsRateLimitError checks if an error is a 429 rate limit error.
func IsRateLimitError(err error) bool {
	if apiErr, ok := err.(*APIError); ok {
		return apiErr.StatusCode == 429
	}
	return false
}

// IsRetryableError checks if an error should be retried.
func IsRetryableError(err error) bool {
	if apiErr, ok := err.(*APIError); ok {
		return apiErr.Retryable
	}
	return false
}

// IsContextTooLongError checks if the error is a context/prompt too long error.
func IsContextTooLongError(err error) bool {
	if apiErr, ok := err.(*APIError); ok {
		return apiErr.Type == ErrPromptTooLong
	}
	return false
}

// ContextOverflowInfo holds parsed context overflow error details.
// Source: withRetry.ts:550-595
type ContextOverflowInfo struct {
	InputTokens  int
	MaxTokens    int
	ContextLimit int
}

// contextOverflowRe matches the context overflow error message.
// Source: withRetry.ts:560
var contextOverflowRe = regexp.MustCompile(
	`input length and .max_tokens. exceed context limit: (\d+) \+ (\d+) > (\d+)`,
)

// ParseContextOverflowError extracts token counts from a context overflow error.
// Source: withRetry.ts:550-595
func ParseContextOverflowError(err error) *ContextOverflowInfo {
	apiErr, ok := err.(*APIError)
	if !ok || apiErr.StatusCode != 400 {
		return nil
	}

	matches := contextOverflowRe.FindStringSubmatch(apiErr.Message)
	if matches == nil || len(matches) < 4 {
		return nil
	}

	input, _ := strconv.Atoi(matches[1])
	maxTok, _ := strconv.Atoi(matches[2])
	ctxLimit, _ := strconv.Atoi(matches[3])

	if input == 0 || ctxLimit == 0 {
		return nil
	}

	return &ContextOverflowInfo{
		InputTokens:  input,
		MaxTokens:    maxTok,
		ContextLimit: ctxLimit,
	}
}

// GetRetryDelay calculates the delay for a retry attempt with exponential backoff
// and optional Retry-After header.
// Source: withRetry.ts:530-548
func GetRetryDelay(attempt int, retryAfterHeader string, maxDelayMs int) time.Duration {
	if maxDelayMs <= 0 {
		maxDelayMs = DefaultMaxDelayMs
	}

	// Honor Retry-After header
	if retryAfterHeader != "" {
		seconds, err := strconv.Atoi(retryAfterHeader)
		if err == nil && seconds > 0 {
			return time.Duration(seconds) * time.Second
		}
	}

	// Exponential backoff with 25% jitter
	// Source: withRetry.ts:542-547
	baseDelay := math.Min(
		float64(BaseDelayMs)*math.Pow(2, float64(attempt-1)),
		float64(maxDelayMs),
	)
	jitter := rand.Float64() * 0.25 * baseDelay
	return time.Duration(baseDelay+jitter) * time.Millisecond
}

// ShouldRetry determines if an API error should be retried for a given attempt.
// Source: withRetry.ts:696-770
func ShouldRetry(err error, attempt, maxRetries int) bool {
	if attempt >= maxRetries {
		return false
	}

	apiErr, ok := err.(*APIError)
	if !ok {
		return false
	}

	return apiErr.Retryable
}

func containsCI(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}
