package util

import (
	"testing"
	"time"
)

func TestFormatFileSize(t *testing.T) {
	tests := []struct {
		bytes int
		want  string
	}{
		{0, "0 bytes"},
		{512, "512 bytes"},
		{1536, "1.5KB"},
		{1024, "1KB"},
		{1048576, "1MB"},
		{1572864, "1.5MB"},
		{1073741824, "1GB"},
	}
	for _, tt := range tests {
		got := FormatFileSize(tt.bytes)
		if got != tt.want {
			t.Errorf("FormatFileSize(%d) = %q, want %q", tt.bytes, got, tt.want)
		}
	}
}

func TestFormatSecondsShort(t *testing.T) {
	if got := FormatSecondsShort(1234); got != "1.2s" {
		t.Errorf("got %q, want 1.2s", got)
	}
	if got := FormatSecondsShort(500); got != "0.5s" {
		t.Errorf("got %q, want 0.5s", got)
	}
}

func TestFormatDurationCompact(t *testing.T) {
	tests := []struct {
		d    time.Duration
		want string
	}{
		{0, "0s"},
		{30 * time.Second, "30s"},
		{90 * time.Second, "1m30s"},
		{5 * time.Minute, "5m"},
		{65 * time.Minute, "1h5m"},
		{2 * time.Hour, "2h"},
		{25 * time.Hour, "1d1h"},
	}
	for _, tt := range tests {
		got := FormatDurationCompact(tt.d)
		if got != tt.want {
			t.Errorf("FormatDurationCompact(%v) = %q, want %q", tt.d, got, tt.want)
		}
	}
}

func TestFormatTokenCount(t *testing.T) {
	tests := []struct {
		tokens int
		want   string
	}{
		{500, "500"},
		{1500, "1.5k"},
		{10000, "10k"},
		{1500000, "1.5M"},
	}
	for _, tt := range tests {
		got := FormatTokenCount(tt.tokens)
		if got != tt.want {
			t.Errorf("FormatTokenCount(%d) = %q, want %q", tt.tokens, got, tt.want)
		}
	}
}
