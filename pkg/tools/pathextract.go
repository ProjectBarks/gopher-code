package tools

import (
	"os"
	"path/filepath"
	"strings"
)

// Source: tools/BashTool/pathValidation.ts

// FilterOutFlags removes flag arguments from a command's args, keeping only
// positional arguments (paths). Handles -- end-of-options delimiter.
// Source: tools/BashTool/pathValidation.ts:126-139
func FilterOutFlags(args []string) []string {
	var result []string
	afterDoubleDash := false
	for _, arg := range args {
		if afterDoubleDash {
			result = append(result, arg)
		} else if arg == "--" {
			afterDoubleDash = true
		} else if !strings.HasPrefix(arg, "-") {
			result = append(result, arg)
		}
	}
	return result
}

// ExtractPaths extracts file paths from a parsed shell command's arguments.
// Different commands have different argument patterns for where paths appear.
// Source: tools/BashTool/pathValidation.ts:190-310
func ExtractPaths(command string, args []string) []string {
	switch command {
	case "cd":
		// Source: pathValidation.ts:195
		if len(args) == 0 {
			home, _ := os.UserHomeDir()
			return []string{home}
		}
		return []string{strings.Join(args, " ")}

	case "ls":
		// Source: pathValidation.ts:198-201
		paths := FilterOutFlags(args)
		if len(paths) > 0 {
			return paths
		}
		return []string{"."}

	case "find":
		// Source: pathValidation.ts:211-269 — collect paths before first non-global flag
		return extractFindPaths(args)

	case "mkdir", "touch", "rm", "rmdir", "mv", "cp",
		"cat", "head", "tail", "sort", "uniq", "wc", "cut", "paste",
		"column", "file", "stat", "diff", "awk", "strings", "hexdump",
		"od", "base64", "nl", "sha256sum", "sha1sum", "md5sum":
		// Source: pathValidation.ts:272-298
		return FilterOutFlags(args)

	default:
		return FilterOutFlags(args)
	}
}

// extractFindPaths handles find's complex argument pattern.
// Source: tools/BashTool/pathValidation.ts:211-269
func extractFindPaths(args []string) []string {
	var paths []string
	pathFlags := map[string]bool{
		"-newer": true, "-anewer": true, "-cnewer": true, "-mnewer": true,
		"-samefile": true, "-path": true, "-wholename": true,
		"-ilname": true, "-lname": true, "-ipath": true, "-iwholename": true,
	}
	foundNonGlobalFlag := false
	afterDoubleDash := false

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "" {
			continue
		}

		if afterDoubleDash {
			paths = append(paths, arg)
			continue
		}

		if arg == "--" {
			afterDoubleDash = true
			continue
		}

		if strings.HasPrefix(arg, "-") {
			// Global options don't stop collection
			// Source: pathValidation.ts:247
			if arg == "-H" || arg == "-L" || arg == "-P" {
				continue
			}
			foundNonGlobalFlag = true

			// Path-taking flags
			// Source: pathValidation.ts:253-259
			if pathFlags[arg] && i+1 < len(args) {
				paths = append(paths, args[i+1])
				i++
			}
			continue
		}

		// Only collect non-flag arguments before first non-global flag
		// Source: pathValidation.ts:263-266
		if !foundNonGlobalFlag {
			paths = append(paths, arg)
		}
	}

	if len(paths) == 0 {
		return []string{"."}
	}
	return paths
}

// ResolvePath resolves a path relative to a working directory.
func ResolvePath(path, cwd string) string {
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	return filepath.Clean(filepath.Join(cwd, path))
}

// IsPathInDirectory checks if a resolved path is within the given directory.
func IsPathInDirectory(path, dir string) bool {
	absPath := filepath.Clean(path)
	absDir := filepath.Clean(dir)
	return absPath == absDir || strings.HasPrefix(absPath, absDir+string(filepath.Separator))
}
