package session

import (
	"fmt"
	"net/url"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// Source: memdir/teamMemPaths.ts:10-64

// PathTraversalError is returned when a path key contains traversal or injection patterns.
// Source: memdir/teamMemPaths.ts:10-15
type PathTraversalError struct {
	Message string
}

func (e *PathTraversalError) Error() string {
	return e.Message
}

// SanitizePathKey validates a file path key by rejecting dangerous patterns.
// Returns the sanitized string or a PathTraversalError.
// Source: memdir/teamMemPaths.ts:22-64
func SanitizePathKey(key string) (string, error) {
	// Null bytes can truncate paths in C-based syscalls
	// Source: teamMemPaths.ts:24-26
	if strings.ContainsRune(key, 0) {
		return "", &PathTraversalError{
			Message: fmt.Sprintf("Null byte in path key: %q", key),
		}
	}

	// URL-encoded traversals (e.g. %2e%2e%2f = ../)
	// Source: teamMemPaths.ts:28-38
	decoded, err := url.PathUnescape(key)
	if err != nil {
		// Malformed percent-encoding — not valid URL-encoding,
		// so no URL-encoded traversal is possible
		decoded = key
	}
	if decoded != key && (strings.Contains(decoded, "..") || strings.Contains(decoded, "/")) {
		return "", &PathTraversalError{
			Message: fmt.Sprintf("URL-encoded traversal in path key: %q", key),
		}
	}

	// Unicode normalization attacks: fullwidth ．．／ (U+FF0E U+FF0F) normalize
	// to ASCII ../ under NFKC.
	// Source: teamMemPaths.ts:43-54
	normalized := norm.NFKC.String(key)
	if normalized != key &&
		(strings.Contains(normalized, "..") ||
			strings.Contains(normalized, "/") ||
			strings.Contains(normalized, "\\") ||
			strings.ContainsRune(normalized, 0)) {
		return "", &PathTraversalError{
			Message: fmt.Sprintf("Unicode-normalized traversal in path key: %q", key),
		}
	}

	// Reject backslashes (Windows path separator used as traversal vector)
	// Source: teamMemPaths.ts:56-58
	if strings.Contains(key, "\\") {
		return "", &PathTraversalError{
			Message: fmt.Sprintf("Backslash in path key: %q", key),
		}
	}

	// Reject absolute paths
	// Source: teamMemPaths.ts:60-62
	if strings.HasPrefix(key, "/") {
		return "", &PathTraversalError{
			Message: fmt.Sprintf("Absolute path key: %q", key),
		}
	}

	// Reject plain traversal
	if strings.Contains(key, "..") {
		return "", &PathTraversalError{
			Message: fmt.Sprintf("Path traversal in path key: %q", key),
		}
	}

	// Reject control characters
	for _, r := range key {
		if unicode.IsControl(r) && r != '\t' {
			return "", &PathTraversalError{
				Message: fmt.Sprintf("Control character in path key: %q", key),
			}
		}
	}

	return key, nil
}
