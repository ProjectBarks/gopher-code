package session

import (
	"errors"
	"testing"
)

// Source: memdir/teamMemPaths.ts:22-64

func TestSanitizePathKey(t *testing.T) {

	t.Run("valid_path", func(t *testing.T) {
		result, err := SanitizePathKey("user_preferences.md")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != "user_preferences.md" {
			t.Errorf("expected unchanged path, got %q", result)
		}
	})

	t.Run("valid_nested_path", func(t *testing.T) {
		result, err := SanitizePathKey("feedback/code_style.md")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != "feedback/code_style.md" {
			t.Errorf("expected unchanged path, got %q", result)
		}
	})

	t.Run("null_byte_rejected", func(t *testing.T) {
		// Source: teamMemPaths.ts:24-26
		_, err := SanitizePathKey("malicious\x00path")
		if err == nil {
			t.Fatal("expected error for null byte")
		}
		var pte *PathTraversalError
		if !errors.As(err, &pte) {
			t.Errorf("expected PathTraversalError, got %T", err)
		}
	})

	t.Run("url_encoded_traversal_rejected", func(t *testing.T) {
		// Source: teamMemPaths.ts:28-38
		_, err := SanitizePathKey("%2e%2e%2fetc%2fpasswd")
		if err == nil {
			t.Fatal("expected error for URL-encoded traversal")
		}
		var pte *PathTraversalError
		if !errors.As(err, &pte) {
			t.Errorf("expected PathTraversalError, got %T", err)
		}
	})

	t.Run("unicode_normalization_traversal_rejected", func(t *testing.T) {
		// Source: teamMemPaths.ts:43-54
		// Fullwidth period U+FF0E normalizes to '.' under NFKC
		_, err := SanitizePathKey("\uFF0E\uFF0E\uFF0Fetc")
		if err == nil {
			t.Fatal("expected error for Unicode-normalized traversal")
		}
		var pte *PathTraversalError
		if !errors.As(err, &pte) {
			t.Errorf("expected PathTraversalError, got %T", err)
		}
	})

	t.Run("backslash_rejected", func(t *testing.T) {
		// Source: teamMemPaths.ts:56-58
		_, err := SanitizePathKey("path\\to\\file")
		if err == nil {
			t.Fatal("expected error for backslash")
		}
	})

	t.Run("absolute_path_rejected", func(t *testing.T) {
		// Source: teamMemPaths.ts:60-62
		_, err := SanitizePathKey("/etc/passwd")
		if err == nil {
			t.Fatal("expected error for absolute path")
		}
	})

	t.Run("dot_dot_traversal_rejected", func(t *testing.T) {
		_, err := SanitizePathKey("../../../etc/passwd")
		if err == nil {
			t.Fatal("expected error for .. traversal")
		}
	})

	t.Run("malformed_percent_encoding_safe", func(t *testing.T) {
		// Source: teamMemPaths.ts:31-34 — malformed encoding = no traversal possible
		result, err := SanitizePathKey("file%ZZname.md")
		if err != nil {
			t.Fatalf("malformed percent encoding should be safe, got: %v", err)
		}
		if result != "file%ZZname.md" {
			t.Errorf("expected unchanged, got %q", result)
		}
	})

	t.Run("url_encoded_safe_chars_ok", func(t *testing.T) {
		// %20 = space, decoded doesn't contain .. or /
		result, err := SanitizePathKey("file%20name.md")
		if err != nil {
			t.Fatalf("URL-encoded space should be safe, got: %v", err)
		}
		if result != "file%20name.md" {
			t.Errorf("expected unchanged, got %q", result)
		}
	})

	t.Run("control_character_rejected", func(t *testing.T) {
		_, err := SanitizePathKey("file\x07name.md")
		if err == nil {
			t.Fatal("expected error for control character")
		}
	})
}
