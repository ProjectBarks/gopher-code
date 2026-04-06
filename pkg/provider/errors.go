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

// User-facing error message constants — verbatim from TS errors.ts.
// Source: services/api/errors.ts:54-169
const (
	APIErrorMessagePrefix                  = "API Error"
	PromptTooLongErrorMessage              = "Prompt is too long"
	CreditBalanceTooLowErrorMessage        = "Credit balance is too low"
	InvalidAPIKeyErrorMessage              = "Not logged in · Please run /login"
	InvalidAPIKeyErrorMessageExternal      = "Invalid API key · Fix external API key"
	OrgDisabledErrorMessageEnvKeyWithOAuth = "Your ANTHROPIC_API_KEY belongs to a disabled organization · Unset the environment variable to use your subscription instead"
	OrgDisabledErrorMessageEnvKey          = "Your ANTHROPIC_API_KEY belongs to a disabled organization · Update or unset the environment variable"
	TokenRevokedErrorMessage               = "OAuth token revoked · Please run /login"
	CCRAuthErrorMessage                    = "Authentication error · This may be a temporary network issue, please try again"
	Repeated529ErrorMessage                = "Repeated 529 Overloaded errors"
	CustomOffSwitchMessage                 = "Opus is experiencing high load, please use /model to switch to Sonnet"
	APITimeoutErrorMessage                 = "Request timed out"
	OAuthOrgNotAllowedErrorMessage         = "Your account does not have access to Claude Code. Please run /login."
)

// APIErrorType classifies the API error for display and retry logic.
// Source: services/api/errors.ts:965-1163
type APIErrorType string

const (
	ErrAborted              APIErrorType = "aborted"
	ErrAPITimeout           APIErrorType = "api_timeout"
	ErrRepeated529          APIErrorType = "repeated_529"
	ErrRateLimit            APIErrorType = "rate_limit"
	ErrServerOverload       APIErrorType = "server_overload"
	ErrPromptTooLong        APIErrorType = "prompt_too_long"
	ErrPDFTooLarge          APIErrorType = "pdf_too_large"
	ErrPDFPasswordProtected APIErrorType = "pdf_password_protected"
	ErrPDFInvalid           APIErrorType = "pdf_invalid"
	ErrImageTooLarge        APIErrorType = "image_too_large"
	ErrRequestTooLarge      APIErrorType = "request_too_large"
	ErrToolUseMismatch      APIErrorType = "tool_use_mismatch"
	ErrInvalidModel         APIErrorType = "invalid_model"
	ErrCreditBalanceLow     APIErrorType = "credit_balance_low"
	ErrInvalidAPIKey        APIErrorType = "invalid_api_key"
	ErrTokenRevoked         APIErrorType = "token_revoked"
	ErrOrgDisabled          APIErrorType = "org_disabled"
	ErrOAuthOrgNotAllowed   APIErrorType = "oauth_org_not_allowed"
	ErrAuthError            APIErrorType = "auth_error"
	ErrServerError          APIErrorType = "server_error"
	ErrClientError          APIErrorType = "client_error"
	ErrSSLCertError         APIErrorType = "ssl_cert_error"
	ErrConnectionError      APIErrorType = "connection_error"
	ErrUnknown              APIErrorType = "unknown"
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
		switch {
		case strings.Contains(msg, "OAuth token has been revoked"):
			err.Type = ErrTokenRevoked
			err.Retryable = true // retry after refresh
		case containsCI(msg, "organization has been disabled"):
			err.Type = ErrOrgDisabled
		case strings.Contains(msg, "does not have access to Claude Code"):
			err.Type = ErrOAuthOrgNotAllowed
		default:
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

// --- Classification helpers ---
// Source: services/api/errors.ts

// IsOverloadedError checks if the error is a 529/overloaded error.
func IsOverloadedError(err error) bool {
	if apiErr, ok := err.(*APIError); ok {
		return apiErr.Type == ErrServerOverload
	}
	return false
}

// IsBillingError checks if the error is a billing/credit-balance error.
func IsBillingError(err error) bool {
	if apiErr, ok := err.(*APIError); ok {
		return apiErr.Type == ErrCreditBalanceLow
	}
	return false
}

// IsInvalidRequestError checks if the error is a client-side invalid request.
func IsInvalidRequestError(err error) bool {
	if apiErr, ok := err.(*APIError); ok {
		switch apiErr.Type {
		case ErrClientError, ErrInvalidModel, ErrToolUseMismatch:
			return true
		}
	}
	return false
}

// IsContextWindowError checks if the error is a context/prompt window limit error.
func IsContextWindowError(err error) bool {
	if apiErr, ok := err.(*APIError); ok {
		return apiErr.Type == ErrPromptTooLong
	}
	return false
}

// --- Prefix / message predicates ---
// Source: services/api/errors.ts:56-61

// StartsWithAPIErrorPrefix checks if text starts with the API error prefix
// or the "Please run /login" variant.
func StartsWithAPIErrorPrefix(text string) bool {
	return strings.HasPrefix(text, APIErrorMessagePrefix) ||
		strings.HasPrefix(text, "Please run /login · "+APIErrorMessagePrefix)
}

// --- Prompt-too-long parsing ---
// Source: services/api/errors.ts:85-118

// promptTooLongRe parses "prompt is too long: 137500 tokens > 135000 maximum".
var promptTooLongRe = regexp.MustCompile(`(?i)prompt is too long[^0-9]*(\d+)\s*tokens?\s*>\s*(\d+)`)

// ParsePromptTooLongTokenCounts extracts (actual, limit) token counts from a
// raw prompt-too-long API error message. Returns (0, 0) if not parseable.
func ParsePromptTooLongTokenCounts(rawMessage string) (actual, limit int) {
	m := promptTooLongRe.FindStringSubmatch(rawMessage)
	if m == nil {
		return 0, 0
	}
	actual, _ = strconv.Atoi(m[1])
	limit, _ = strconv.Atoi(m[2])
	return actual, limit
}

// GetPromptTooLongTokenGap returns how many tokens over the limit a
// prompt-too-long error reports, or 0 if not parseable / not over.
// Source: errors.ts:104-118
func GetPromptTooLongTokenGap(rawMessage string) int {
	actual, limit := ParsePromptTooLongTokenCounts(rawMessage)
	if actual == 0 || limit == 0 {
		return 0
	}
	gap := actual - limit
	if gap > 0 {
		return gap
	}
	return 0
}

// --- Media size error detection ---
// Source: services/api/errors.ts:133-139

// mediaPDFPagesRe matches "maximum of N PDF pages".
var mediaPDFPagesRe = regexp.MustCompile(`maximum of \d+ PDF pages`)

// IsMediaSizeError checks if a raw API error string is a media-size rejection
// that can be fixed by stripping media from the request.
func IsMediaSizeError(raw string) bool {
	return (strings.Contains(raw, "image exceeds") && strings.Contains(raw, "maximum")) ||
		(strings.Contains(raw, "image dimensions exceed") && strings.Contains(raw, "many-image")) ||
		mediaPDFPagesRe.MatchString(raw)
}

// --- User-facing error messages ---
// Source: services/api/errors.ts:170-210

// UserFacingMessage returns a user-readable string for this API error.
// The returned string always starts with the appropriate constant message
// so callers can use StartsWithAPIErrorPrefix for detection.
func (e *APIError) UserFacingMessage() string {
	switch e.Type {
	case ErrPromptTooLong:
		return PromptTooLongErrorMessage
	case ErrCreditBalanceLow:
		return CreditBalanceTooLowErrorMessage
	case ErrInvalidAPIKey:
		return InvalidAPIKeyErrorMessage
	case ErrTokenRevoked:
		return TokenRevokedErrorMessage
	case ErrRepeated529:
		return Repeated529ErrorMessage
	case ErrAPITimeout:
		return APITimeoutErrorMessage
	case ErrOrgDisabled:
		return OrgDisabledErrorMessageEnvKey
	case ErrOAuthOrgNotAllowed:
		return OAuthOrgNotAllowedErrorMessage
	default:
		return fmt.Sprintf("%s: %s", APIErrorMessagePrefix, e.Message)
	}
}

// GetPDFTooLargeErrorMessage returns the user-facing message for a PDF that
// exceeds size limits. nonInteractive controls the recovery hint.
// Source: errors.ts:170-175
func GetPDFTooLargeErrorMessage(nonInteractive bool) string {
	if nonInteractive {
		return "PDF too large. Try reading the file a different way (e.g., extract text with pdftotext)."
	}
	return "PDF too large. Double press esc to go back and try again, or use pdftotext to convert to text first."
}

// GetPDFPasswordProtectedErrorMessage returns the user-facing message for a
// password-protected PDF. Source: errors.ts:176-179
func GetPDFPasswordProtectedErrorMessage(nonInteractive bool) string {
	if nonInteractive {
		return "PDF is password protected. Try using a CLI tool to extract or convert the PDF."
	}
	return "PDF is password protected. Please double press esc to edit your message and try again."
}

// GetPDFInvalidErrorMessage returns the user-facing message for an invalid PDF.
// Source: errors.ts:181-184
func GetPDFInvalidErrorMessage(nonInteractive bool) string {
	if nonInteractive {
		return "The PDF file was not valid. Try converting it to text first (e.g., pdftotext)."
	}
	return "The PDF file was not valid. Double press esc to go back and try again with a different file."
}

// GetImageTooLargeErrorMessage returns the user-facing message for an
// oversized image. Source: errors.ts:186-189
func GetImageTooLargeErrorMessage(nonInteractive bool) string {
	if nonInteractive {
		return "Image was too large. Try resizing the image or using a different approach."
	}
	return "Image was too large. Double press esc to go back and try again with a smaller image."
}

// GetRequestTooLargeErrorMessage returns the user-facing message for a request
// that exceeds the maximum payload size. Source: errors.ts:191-195
func GetRequestTooLargeErrorMessage(nonInteractive bool) string {
	if nonInteractive {
		return "Request too large. Try with a smaller file."
	}
	return "Request too large. Double press esc to go back and try with a smaller file."
}
