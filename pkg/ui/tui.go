package ui

import (
	"fmt"
	"os"
	"runtime"
	"strings"

	"golang.org/x/term"
)

// Source: utils/terminal.ts, utils/platform.ts
//
// Terminal detection, platform identification, and text rendering utilities
// for the TUI layer.

// Platform identifies the operating system variant.
// Source: utils/platform.ts
type Platform string

const (
	PlatformMacOS   Platform = "macos"
	PlatformLinux   Platform = "linux"
	PlatformWindows Platform = "windows"
	PlatformWSL     Platform = "wsl"
	PlatformUnknown Platform = "unknown"
)

// GetPlatform detects the current platform, including WSL detection.
// Source: utils/platform.ts — getPlatform
func GetPlatform() Platform {
	switch runtime.GOOS {
	case "darwin":
		return PlatformMacOS
	case "windows":
		return PlatformWindows
	case "linux":
		if isWSL() {
			return PlatformWSL
		}
		return PlatformLinux
	default:
		return PlatformUnknown
	}
}

func isWSL() bool {
	data, err := os.ReadFile("/proc/version")
	if err != nil {
		return false
	}
	lower := strings.ToLower(string(data))
	return strings.Contains(lower, "microsoft") || strings.Contains(lower, "wsl")
}

// TerminalWidth returns the current terminal width, or fallback if unknown.
func TerminalWidth(fallback int) int {
	w, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || w <= 0 {
		return fallback
	}
	return w
}

// TerminalHeight returns the current terminal height, or fallback if unknown.
func TerminalHeight(fallback int) int {
	_, h, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || h <= 0 {
		return fallback
	}
	return h
}

// MaxLinesToShow is the default number of visible lines before truncation.
// Source: utils/terminal.ts:7
const MaxLinesToShow = 3

// RenderTruncatedContent truncates content to MaxLinesToShow visible lines.
// If truncated, appends "... +N lines" indicator.
// Source: utils/terminal.ts — renderTruncatedContent
func RenderTruncatedContent(content string, terminalWidth int) string {
	trimmed := strings.TrimRight(content, "\n\r\t ")
	if trimmed == "" {
		return ""
	}

	lines := strings.Split(trimmed, "\n")

	// Wrap long lines
	var wrapped []string
	for _, line := range lines {
		if visibleLen(line) <= terminalWidth {
			wrapped = append(wrapped, strings.TrimRight(line, " "))
		} else {
			// Break into chunks
			for i := 0; i < len(line); i += terminalWidth {
				end := i + terminalWidth
				if end > len(line) {
					end = len(line)
				}
				wrapped = append(wrapped, strings.TrimRight(line[i:end], " "))
			}
		}
	}

	remaining := len(wrapped) - MaxLinesToShow

	// If only 1 extra line, show it rather than "... +1 line"
	if remaining == 1 {
		return strings.Join(wrapped[:MaxLinesToShow+1], "\n")
	}

	if remaining > 0 {
		aboveFold := strings.Join(wrapped[:MaxLinesToShow], "\n")
		return aboveFold + fmt.Sprintf("\n... +%d lines", remaining)
	}

	return strings.Join(wrapped, "\n")
}

// visibleLen returns the visible character count (strips ANSI escapes).
func visibleLen(s string) int {
	// Simple ANSI stripping — count chars outside escape sequences
	n := 0
	inEsc := false
	for _, r := range s {
		if r == '\033' {
			inEsc = true
			continue
		}
		if inEsc {
			if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') {
				inEsc = false
			}
			continue
		}
		n++
	}
	return n
}

// IsColorTerminal returns true if the terminal supports color output.
func IsColorTerminal() bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	if os.Getenv("FORCE_COLOR") != "" {
		return true
	}
	return term.IsTerminal(int(os.Stdout.Fd()))
}

// Is256ColorTerminal returns true if the terminal supports 256 colors.
func Is256ColorTerminal() bool {
	t := os.Getenv("TERM")
	return strings.Contains(t, "256color") || strings.Contains(t, "kitty") ||
		os.Getenv("COLORTERM") == "truecolor"
}
