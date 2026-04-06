// Package util provides shared helper functions.
package util

import (
	"os"
	"sync"
	"time"
)

// GetLocalISODate returns the local date as "YYYY-MM-DD".
// If CLAUDE_CODE_OVERRIDE_DATE is set, its value is returned as-is.
func GetLocalISODate() string {
	if v := os.Getenv("CLAUDE_CODE_OVERRIDE_DATE"); v != "" {
		return v
	}
	return time.Now().Format("2006-01-02")
}

// sessionStartDate is memoized via sync.OnceValue for prompt-cache stability.
var sessionStartDate = sync.OnceValue(GetLocalISODate)

// GetSessionStartDate returns the date captured on the first call, memoized
// for the lifetime of the process (prompt-cache stability).
func GetSessionStartDate() string { return sessionStartDate() }

// resetSessionStartDate replaces the OnceValue so tests can re-trigger it.
func resetSessionStartDate() { sessionStartDate = sync.OnceValue(GetLocalISODate) }

// GetLocalMonthYear returns "Month YYYY" (e.g. "February 2026") in en-US long
// format. If CLAUDE_CODE_OVERRIDE_DATE is set, that date is parsed first.
func GetLocalMonthYear() string {
	var t time.Time
	if v := os.Getenv("CLAUDE_CODE_OVERRIDE_DATE"); v != "" {
		parsed, err := time.Parse("2006-01-02", v)
		if err == nil {
			t = parsed
		} else {
			t = time.Now()
		}
	} else {
		t = time.Now()
	}
	return t.Format("January 2006")
}
