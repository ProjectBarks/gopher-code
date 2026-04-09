package util

import (
	"fmt"
	"strings"
	"time"
)

// Source: utils/format.ts — pure display formatters

// FormatFileSize formats a byte count to human-readable (KB, MB, GB).
// Source: format.ts:9-23
func FormatFileSize(bytes int) string {
	if bytes < 1024 {
		return fmt.Sprintf("%d bytes", bytes)
	}
	kb := float64(bytes) / 1024
	if kb < 1024 {
		return trimTrailingZero(fmt.Sprintf("%.1f", kb)) + "KB"
	}
	mb := kb / 1024
	if mb < 1024 {
		return trimTrailingZero(fmt.Sprintf("%.1f", mb)) + "MB"
	}
	gb := mb / 1024
	return trimTrailingZero(fmt.Sprintf("%.1f", gb)) + "GB"
}

func trimTrailingZero(s string) string {
	return strings.TrimSuffix(s, ".0")
}

// FormatSecondsShort formats milliseconds as "N.Ns" (e.g., 1234 → "1.2s").
// Source: format.ts:30-32
func FormatSecondsShort(ms int) string {
	return fmt.Sprintf("%.1fs", float64(ms)/1000)
}

// FormatDurationCompact formats a duration as a compact string.
// Source: format.ts:34-100
func FormatDurationCompact(d time.Duration) string {
	ms := d.Milliseconds()
	if ms == 0 {
		return "0s"
	}
	if ms < 60000 {
		return fmt.Sprintf("%ds", ms/1000)
	}

	days := ms / 86400000
	hours := (ms % 86400000) / 3600000
	minutes := (ms % 3600000) / 60000
	seconds := (ms % 60000) / 1000

	if days > 0 {
		if hours > 0 {
			return fmt.Sprintf("%dd%dh", days, hours)
		}
		return fmt.Sprintf("%dd", days)
	}
	if hours > 0 {
		if minutes > 0 {
			return fmt.Sprintf("%dh%dm", hours, minutes)
		}
		return fmt.Sprintf("%dh", hours)
	}
	if seconds > 0 {
		return fmt.Sprintf("%dm%ds", minutes, seconds)
	}
	return fmt.Sprintf("%dm", minutes)
}

// FormatTokenCount formats a token count with K/M suffixes.
func FormatTokenCount(tokens int) string {
	if tokens < 1000 {
		return fmt.Sprintf("%d", tokens)
	}
	if tokens < 1_000_000 {
		return trimTrailingZero(fmt.Sprintf("%.1f", float64(tokens)/1000)) + "k"
	}
	return trimTrailingZero(fmt.Sprintf("%.1f", float64(tokens)/1_000_000)) + "M"
}

// FormatCostUSD formats a dollar amount with appropriate precision.
func FormatCostUSD(cost float64) string {
	if cost < 0.01 {
		return fmt.Sprintf("$%.4f", cost)
	}
	if cost < 1 {
		return fmt.Sprintf("$%.2f", cost)
	}
	return fmt.Sprintf("$%.2f", cost)
}
