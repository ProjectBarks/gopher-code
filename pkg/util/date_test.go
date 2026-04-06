package util

import (
	"os"
	"testing"
	"time"
)

func TestGetLocalISODate(t *testing.T) {
	t.Run("returns local YYYY-MM-DD by default", func(t *testing.T) {
		t.Setenv("CLAUDE_CODE_OVERRIDE_DATE", "")
		got := GetLocalISODate()
		now := time.Now()
		want := now.Format("2006-01-02")
		if got != want {
			t.Errorf("GetLocalISODate() = %q, want %q", got, want)
		}
	})

	t.Run("returns override date when env set", func(t *testing.T) {
		t.Setenv("CLAUDE_CODE_OVERRIDE_DATE", "2099-12-25")
		got := GetLocalISODate()
		if got != "2099-12-25" {
			t.Errorf("GetLocalISODate() = %q, want %q", got, "2099-12-25")
		}
	})
}

func TestGetSessionStartDate(t *testing.T) {
	t.Run("memoizes the date from first call", func(t *testing.T) {
		// Reset so we get a fresh OnceValue.
		resetSessionStartDate()

		os.Setenv("CLAUDE_CODE_OVERRIDE_DATE", "2025-01-15")
		first := GetSessionStartDate()
		if first != "2025-01-15" {
			t.Fatalf("first call = %q, want %q", first, "2025-01-15")
		}

		os.Setenv("CLAUDE_CODE_OVERRIDE_DATE", "2025-06-30")
		second := GetSessionStartDate()
		if second != first {
			t.Errorf("memoized call = %q, want cached %q", second, first)
		}

		// Clean up.
		os.Unsetenv("CLAUDE_CODE_OVERRIDE_DATE")
	})
}

func TestGetLocalMonthYear(t *testing.T) {
	t.Run("returns en-US long month and year by default", func(t *testing.T) {
		t.Setenv("CLAUDE_CODE_OVERRIDE_DATE", "")
		got := GetLocalMonthYear()
		now := time.Now()
		want := now.Format("January 2006")
		if got != want {
			t.Errorf("GetLocalMonthYear() = %q, want %q", got, want)
		}
	})

	t.Run("parses override date for month-year", func(t *testing.T) {
		t.Setenv("CLAUDE_CODE_OVERRIDE_DATE", "2026-02-15")
		got := GetLocalMonthYear()
		if got != "February 2026" {
			t.Errorf("GetLocalMonthYear() = %q, want %q", got, "February 2026")
		}
	})

	t.Run("handles override at year boundary", func(t *testing.T) {
		t.Setenv("CLAUDE_CODE_OVERRIDE_DATE", "2030-12-01")
		got := GetLocalMonthYear()
		if got != "December 2030" {
			t.Errorf("GetLocalMonthYear() = %q, want %q", got, "December 2030")
		}
	})
}
