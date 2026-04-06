package provider

import (
	"context"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"time"
)

// Source: services/api/withRetry.ts

// RetryHeaderResult represents the x-should-retry header check outcome.
type RetryHeaderResult int

const (
	RetryHeaderAbsent RetryHeaderResult = iota
	RetryHeaderYes
	RetryHeaderNo
)

// CheckShouldRetryHeader inspects the x-should-retry header.
// Source: withRetry.ts:732
func CheckShouldRetryHeader(h http.Header) RetryHeaderResult {
	v := h.Get("x-should-retry")
	switch v {
	case "true":
		return RetryHeaderYes
	case "false":
		return RetryHeaderNo
	default:
		return RetryHeaderAbsent
	}
}

// RetryContext carries mutable state through the retry loop.
// Source: withRetry.ts:120-125
type RetryContext struct {
	MaxTokensOverride int
	Model             string
}

// RetryOptions configures the retry loop.
// Source: withRetry.ts:127-143
type RetryOptions struct {
	MaxRetries                   int
	Model                        string
	FallbackModel                string
	QuerySource                  QuerySource
	BaseDelay                    time.Duration // override BaseDelayMs for testing
	InitialConsecutive529Errors  int
}

// CannotRetryError is returned when retries are exhausted or the error is non-retryable.
// Source: withRetry.ts:144-158
type CannotRetryError struct {
	OriginalError error
	Context       RetryContext
}

func (e *CannotRetryError) Error() string {
	if e.OriginalError != nil {
		return e.OriginalError.Error()
	}
	return "cannot retry"
}

func (e *CannotRetryError) Unwrap() error { return e.OriginalError }

// NewCannotRetryError creates a CannotRetryError.
func NewCannotRetryError(err error, ctx RetryContext) *CannotRetryError {
	return &CannotRetryError{OriginalError: err, Context: ctx}
}

// FallbackTriggeredError signals that the retry loop wants to switch models.
// Source: withRetry.ts:160-168
type FallbackTriggeredError struct {
	OriginalModel string
	FallbackModel string
}

func (e *FallbackTriggeredError) Error() string {
	return fmt.Sprintf("Model fallback triggered: %s -> %s", e.OriginalModel, e.FallbackModel)
}

// NewFallbackTriggeredError creates a FallbackTriggeredError.
func NewFallbackTriggeredError(original, fallback string) *FallbackTriggeredError {
	return &FallbackTriggeredError{OriginalModel: original, FallbackModel: fallback}
}

// GetDefaultMaxRetries returns the max retries from env or default 10.
// Source: withRetry.ts:789-794
func GetDefaultMaxRetries() int {
	if v := os.Getenv("CLAUDE_CODE_MAX_RETRIES"); v != "" {
		n, err := strconv.Atoi(v)
		if err == nil && n > 0 {
			return n
		}
	}
	return DefaultMaxRetries
}

// WithRetry executes an operation with exponential backoff retry logic.
// It handles 529 overloaded detection, retry-after headers, fallback model
// triggering, and the x-should-retry header.
//
// The operation function receives the attempt number (1-based) and a RetryContext.
// It should return (result, nil) on success or (nil, *APIError) on failure.
//
// Source: withRetry.ts:170-517
func WithRetry[T any](
	ctx context.Context,
	operation func(attempt int, retryCtx RetryContext) (T, error),
	opts RetryOptions,
) (T, error) {
	var zero T

	maxRetries := opts.MaxRetries
	if maxRetries <= 0 {
		maxRetries = GetDefaultMaxRetries()
	}

	baseDelay := opts.BaseDelay
	if baseDelay <= 0 {
		baseDelay = time.Duration(BaseDelayMs) * time.Millisecond
	}

	retryCtx := RetryContext{
		Model: opts.Model,
	}

	consecutive529 := opts.InitialConsecutive529Errors
	var lastErr error

	// maxRetries+1 total attempts: 1 initial + maxRetries retries
	for attempt := 1; attempt <= maxRetries+1; attempt++ {
		// Check context cancellation before each attempt.
		if err := ctx.Err(); err != nil {
			return zero, err
		}

		result, err := operation(attempt, retryCtx)
		if err == nil {
			return result, nil
		}
		lastErr = err

		apiErr, isAPIErr := err.(*APIError)

		// Non-foreground sources bail immediately on 529.
		// Source: withRetry.ts:318-324
		if isAPIErr && Is529Error(apiErr) && !ShouldRetry529(opts.QuerySource) {
			return zero, NewCannotRetryError(err, retryCtx)
		}

		// Track consecutive 529 errors for fallback trigger.
		// Source: withRetry.ts:327-365
		if isAPIErr && Is529Error(apiErr) {
			consecutive529++
			if consecutive529 >= Max529Retries {
				if opts.FallbackModel != "" {
					return zero, NewFallbackTriggeredError(opts.Model, opts.FallbackModel)
				}
			}
		}

		// Check x-should-retry header veto.
		// Source: withRetry.ts:732-749
		if isAPIErr && apiErr.ShouldRetryHeader == "false" {
			return zero, NewCannotRetryError(err, retryCtx)
		}

		// Non-retryable errors are terminal.
		if isAPIErr && !apiErr.Retryable {
			return zero, NewCannotRetryError(err, retryCtx)
		}

		// Non-APIError: wrap and stop.
		if !isAPIErr {
			return zero, NewCannotRetryError(err, retryCtx)
		}

		// If this was the last attempt, stop.
		if attempt > maxRetries {
			return zero, NewCannotRetryError(err, retryCtx)
		}

		// Compute delay: honor Retry-After, then exponential backoff with jitter.
		// Source: withRetry.ts:430-462
		var delay time.Duration
		if apiErr.RetryAfter != "" {
			delay = GetRetryDelay(attempt, apiErr.RetryAfter, DefaultMaxDelayMs)
		} else {
			// Use custom base delay for testing, but use standard formula.
			baseDur := float64(baseDelay/time.Millisecond) * pow2(attempt-1)
			maxDur := float64(DefaultMaxDelayMs)
			if baseDur > maxDur {
				baseDur = maxDur
			}
			jitter := jitterRand() * 0.25 * baseDur
			delay = time.Duration(baseDur+jitter) * time.Millisecond
		}

		// Sleep with context cancellation.
		select {
		case <-ctx.Done():
			return zero, ctx.Err()
		case <-time.After(delay):
		}
	}

	return zero, NewCannotRetryError(lastErr, retryCtx)
}

// pow2 returns 2^n as float64.
func pow2(n int) float64 {
	return float64(int(1) << uint(n))
}

// jitterRand returns a random float64 in [0, 1) for jitter calculation.
// Uses math/rand global source (auto-seeded in Go 1.20+).
func jitterRand() float64 {
	return rand.Float64()
}
