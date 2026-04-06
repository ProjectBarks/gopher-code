package session

import (
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// Source: utils/sessionStoragePortable.ts

// MaxSanitizedLength is the maximum length for a sanitized directory name
// before a hash suffix is appended. Leaves room for the hash + separator
// within the 255-byte filesystem component limit.
// Source: sessionStoragePortable.ts:293
const MaxSanitizedLength = 200

// nonAlphanumeric matches any character that is not a-z, A-Z, or 0-9.
var nonAlphanumeric = regexp.MustCompile(`[^a-zA-Z0-9]`)

// uuidPattern matches standard UUID v4 format (case-insensitive).
// Source: sessionStoragePortable.ts:23-24
var uuidPattern = regexp.MustCompile(
	`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`,
)

// configDirFn resolves the Claude config home directory.
// Override in tests via setConfigDirForTest.
var configDirFn = defaultConfigDir

func defaultConfigDir() string {
	if d := os.Getenv("CLAUDE_CONFIG_DIR"); d != "" {
		return d
	}
	home, _ := homeDirFn()
	return filepath.Join(home, ".claude")
}

// setConfigDirForTest overrides the config directory function for testing.
func setConfigDirForTest(dir string) func() {
	orig := configDirFn
	configDirFn = func() string { return dir }
	return func() { configDirFn = orig }
}

// djb2Hash implements the DJB2 hash function, matching the TS implementation.
// Source: utils/hash.ts:7-13
func djb2Hash(s string) int32 {
	var hash int32
	for _, c := range s {
		hash = ((hash << 5) - hash + int32(c))
	}
	return hash
}

// SanitizeDirName makes a string safe for use as a directory or file name.
// Replaces all non-alphanumeric characters with hyphens. For long strings
// that exceed MaxSanitizedLength, truncates and appends a hash suffix.
// Source: sessionStoragePortable.ts:311-319
func SanitizeDirName(name string) string {
	sanitized := nonAlphanumeric.ReplaceAllString(name, "-")
	if len(sanitized) <= MaxSanitizedLength {
		return sanitized
	}
	h := math.Abs(float64(djb2Hash(name)))
	hashStr := strconv.FormatUint(uint64(h), 36)
	return sanitized[:MaxSanitizedLength] + "-" + hashStr
}

// ValidateUUID checks whether a string is a valid UUID (v4 format).
// Returns the string if valid, or empty string if not.
// Source: sessionStoragePortable.ts:26-29
func ValidateUUID(s string) string {
	if uuidPattern.MatchString(strings.ToLower(s)) {
		return s
	}
	return ""
}

// GetProjectsDir returns the path to the projects directory (~/.claude/projects/).
// Source: sessionStoragePortable.ts:325-327
func GetProjectsDir() string {
	return filepath.Join(configDirFn(), "projects")
}

// GetProjectDir returns the project directory for a given CWD.
// The CWD is sanitized into a filesystem-safe directory name.
// Source: sessionStoragePortable.ts:329-331
func GetProjectDir(cwd string) string {
	return filepath.Join(GetProjectsDir(), SanitizeDirName(cwd))
}

// GetTranscriptPath returns the JSONL transcript path for a session in a project.
// Source: sessionStorage.ts:202-205
func GetTranscriptPath(projectDir, sessionID string) string {
	return filepath.Join(projectDir, sessionID+".jsonl")
}

// FindProjectDir finds the project directory for a given path, tolerating
// hash mismatches for long paths. Falls back to prefix-based scanning
// when the exact match doesn't exist.
// Source: sessionStoragePortable.ts:354-379
func FindProjectDir(projectPath string) (string, bool) {
	exact := GetProjectDir(projectPath)
	if entries, err := os.ReadDir(exact); err == nil && len(entries) > 0 {
		return exact, true
	}

	// For short paths, no fallback
	sanitized := SanitizeDirName(projectPath)
	if len(sanitized) <= MaxSanitizedLength {
		return "", false
	}

	// Long paths: try prefix-based scan
	prefix := nonAlphanumeric.ReplaceAllString(projectPath, "-")
	if len(prefix) > MaxSanitizedLength {
		prefix = prefix[:MaxSanitizedLength]
	}

	projectsDir := GetProjectsDir()
	dirents, err := os.ReadDir(projectsDir)
	if err != nil {
		return "", false
	}
	for _, d := range dirents {
		if d.IsDir() && strings.HasPrefix(d.Name(), prefix+"-") {
			return filepath.Join(projectsDir, d.Name()), true
		}
	}
	return "", false
}
