package session

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Source: utils/sessionStoragePortable.ts

func TestSanitizeDirName(t *testing.T) {
	// Source: sessionStoragePortable.ts:311-319

	t.Run("replaces_non_alphanumeric_with_hyphens", func(t *testing.T) {
		got := SanitizeDirName("/Users/foo/my-project")
		want := "-Users-foo-my-project"
		if got != want {
			t.Errorf("SanitizeDirName(%q) = %q, want %q", "/Users/foo/my-project", got, want)
		}
	})

	t.Run("preserves_alphanumeric", func(t *testing.T) {
		got := SanitizeDirName("simpleProject123")
		want := "simpleProject123"
		if got != want {
			t.Errorf("SanitizeDirName(%q) = %q, want %q", "simpleProject123", got, want)
		}
	})

	t.Run("colons_replaced", func(t *testing.T) {
		// Windows-unsafe characters like colons must be replaced
		got := SanitizeDirName("plugin:name:server")
		want := "plugin-name-server"
		if got != want {
			t.Errorf("SanitizeDirName(%q) = %q, want %q", "plugin:name:server", got, want)
		}
	})

	t.Run("spaces_replaced", func(t *testing.T) {
		got := SanitizeDirName("my project dir")
		want := "my-project-dir"
		if got != want {
			t.Errorf("SanitizeDirName(%q) = %q, want %q", "my project dir", got, want)
		}
	})

	t.Run("short_path_no_hash", func(t *testing.T) {
		input := "/tmp/short"
		got := SanitizeDirName(input)
		// Should not contain a hash suffix since len <= MaxSanitizedLength
		if len(got) > MaxSanitizedLength {
			t.Errorf("short path should not exceed MaxSanitizedLength, got len %d", len(got))
		}
	})

	t.Run("long_path_gets_hash_suffix", func(t *testing.T) {
		// Build a path longer than MaxSanitizedLength
		input := "/" + strings.Repeat("a/very/deep/nested/path/", 20)
		got := SanitizeDirName(input)
		if len(got) <= MaxSanitizedLength {
			t.Errorf("long path should exceed MaxSanitizedLength after hash, got len %d", len(got))
		}
		// Should start with the truncated sanitized prefix
		prefix := nonAlphanumeric.ReplaceAllString(input, "-")[:MaxSanitizedLength]
		if !strings.HasPrefix(got, prefix) {
			t.Errorf("long path should start with truncated prefix")
		}
		// Should have a hyphen separator before the hash
		rest := got[MaxSanitizedLength:]
		if !strings.HasPrefix(rest, "-") {
			t.Errorf("hash suffix should be separated by hyphen, got rest %q", rest)
		}
	})

	t.Run("deterministic_hash", func(t *testing.T) {
		input := "/" + strings.Repeat("x/", 200)
		a := SanitizeDirName(input)
		b := SanitizeDirName(input)
		if a != b {
			t.Errorf("SanitizeDirName should be deterministic: %q != %q", a, b)
		}
	})

	t.Run("different_long_paths_different_hashes", func(t *testing.T) {
		input1 := "/" + strings.Repeat("a/", 200)
		input2 := "/" + strings.Repeat("b/", 200)
		got1 := SanitizeDirName(input1)
		got2 := SanitizeDirName(input2)
		if got1 == got2 {
			t.Errorf("different long paths should produce different sanitized names")
		}
	})

	t.Run("empty_string", func(t *testing.T) {
		got := SanitizeDirName("")
		if got != "" {
			t.Errorf("SanitizeDirName(%q) = %q, want empty", "", got)
		}
	})
}

func TestValidateUUID(t *testing.T) {
	// Source: sessionStoragePortable.ts:26-29

	t.Run("valid_uuid", func(t *testing.T) {
		input := "550e8400-e29b-41d4-a716-446655440000"
		got := ValidateUUID(input)
		if got != input {
			t.Errorf("ValidateUUID(%q) = %q, want %q", input, got, input)
		}
	})

	t.Run("valid_uuid_uppercase", func(t *testing.T) {
		input := "550E8400-E29B-41D4-A716-446655440000"
		got := ValidateUUID(input)
		if got != input {
			t.Errorf("ValidateUUID(%q) = %q, want %q", input, got, input)
		}
	})

	t.Run("invalid_uuid_too_short", func(t *testing.T) {
		got := ValidateUUID("not-a-uuid")
		if got != "" {
			t.Errorf("ValidateUUID(%q) = %q, want empty", "not-a-uuid", got)
		}
	})

	t.Run("invalid_uuid_bad_chars", func(t *testing.T) {
		got := ValidateUUID("550e8400-e29b-41d4-a716-44665544000g")
		if got != "" {
			t.Errorf("ValidateUUID should reject non-hex chars, got %q", got)
		}
	})

	t.Run("empty_string", func(t *testing.T) {
		got := ValidateUUID("")
		if got != "" {
			t.Errorf("ValidateUUID(%q) = %q, want empty", "", got)
		}
	})
}

func TestGetProjectsDir(t *testing.T) {
	// Source: sessionStoragePortable.ts:325-327
	dir := t.TempDir()
	cleanup := setConfigDirForTest(dir)
	defer cleanup()

	got := GetProjectsDir()
	want := filepath.Join(dir, "projects")
	if got != want {
		t.Errorf("GetProjectsDir() = %q, want %q", got, want)
	}
}

func TestGetProjectDir(t *testing.T) {
	// Source: sessionStoragePortable.ts:329-331
	dir := t.TempDir()
	cleanup := setConfigDirForTest(dir)
	defer cleanup()

	got := GetProjectDir("/Users/dev/myapp")
	want := filepath.Join(dir, "projects", "-Users-dev-myapp")
	if got != want {
		t.Errorf("GetProjectDir(%q) = %q, want %q", "/Users/dev/myapp", got, want)
	}
}

func TestGetTranscriptPath(t *testing.T) {
	// Source: sessionStorage.ts:202-205
	projectDir := "/home/user/.claude/projects/-Users-dev-myapp"
	sessionID := "550e8400-e29b-41d4-a716-446655440000"

	got := GetTranscriptPath(projectDir, sessionID)
	want := filepath.Join(projectDir, sessionID+".jsonl")
	if got != want {
		t.Errorf("GetTranscriptPath() = %q, want %q", got, want)
	}
}

func TestFindProjectDir(t *testing.T) {
	// Source: sessionStoragePortable.ts:354-379

	t.Run("exact_match", func(t *testing.T) {
		dir := t.TempDir()
		cleanup := setConfigDirForTest(dir)
		defer cleanup()

		// Create the expected project directory with a file in it
		projDir := GetProjectDir("/tmp/myproject")
		os.MkdirAll(projDir, 0700)
		os.WriteFile(filepath.Join(projDir, "test.jsonl"), []byte("{}"), 0600)

		got, found := FindProjectDir("/tmp/myproject")
		if !found {
			t.Fatal("FindProjectDir should find exact match")
		}
		if got != projDir {
			t.Errorf("FindProjectDir() = %q, want %q", got, projDir)
		}
	})

	t.Run("not_found_short_path", func(t *testing.T) {
		dir := t.TempDir()
		cleanup := setConfigDirForTest(dir)
		defer cleanup()

		// Create projects dir but no matching project
		os.MkdirAll(GetProjectsDir(), 0700)

		_, found := FindProjectDir("/tmp/nonexistent")
		if found {
			t.Error("FindProjectDir should not find nonexistent short path")
		}
	})

	t.Run("prefix_fallback_for_long_paths", func(t *testing.T) {
		dir := t.TempDir()
		cleanup := setConfigDirForTest(dir)
		defer cleanup()

		// Build a long path
		longPath := "/" + strings.Repeat("very/deep/nested/project/", 20)
		sanitized := nonAlphanumeric.ReplaceAllString(longPath, "-")
		prefix := sanitized[:MaxSanitizedLength]

		// Create a directory with a different hash suffix (simulating Bun vs Node mismatch)
		projectsDir := GetProjectsDir()
		os.MkdirAll(projectsDir, 0700)
		fakeDir := filepath.Join(projectsDir, prefix+"-differenthash")
		os.MkdirAll(fakeDir, 0700)

		got, found := FindProjectDir(longPath)
		if !found {
			t.Fatal("FindProjectDir should fallback to prefix match for long paths")
		}
		if got != fakeDir {
			t.Errorf("FindProjectDir() = %q, want %q", got, fakeDir)
		}
	})
}

func TestDjb2Hash(t *testing.T) {
	// Source: utils/hash.ts:7-13
	// The TS implementation uses 32-bit int overflow semantics via |0

	t.Run("empty_string", func(t *testing.T) {
		got := djb2Hash("")
		if got != 0 {
			t.Errorf("djb2Hash(%q) = %d, want 0", "", got)
		}
	})

	t.Run("deterministic", func(t *testing.T) {
		a := djb2Hash("hello world")
		b := djb2Hash("hello world")
		if a != b {
			t.Errorf("djb2Hash should be deterministic: %d != %d", a, b)
		}
	})

	t.Run("different_inputs_different_outputs", func(t *testing.T) {
		a := djb2Hash("hello")
		b := djb2Hash("world")
		if a == b {
			t.Errorf("djb2Hash(hello) == djb2Hash(world) = %d, expected different", a)
		}
	})
}
