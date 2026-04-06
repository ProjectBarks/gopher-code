package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/bmatcuk/doublestar/v4"
	ignore "github.com/sabhiram/go-gitignore"
)

// DefaultGlobMaxResults is the default max results for glob operations.
// Source: GlobTool.ts:157
const DefaultGlobMaxResults = 100

// GlobTruncationMessage is appended when results are truncated.
// Source: GlobTool.ts:191-193
const GlobTruncationMessage = "(Results are truncated. Consider using a more specific path or pattern.)"

// skipDirs lists directories to always skip during filesystem traversal.
// Used by both GlobTool and GrepTool.
var skipDirs = map[string]bool{
	".git":         true,
	"node_modules": true,
	"vendor":       true,
	"__pycache__":  true,
	".idea":        true,
	".vscode":      true,
}

// GlobTool finds files matching a glob pattern.
type GlobTool struct{}

type globInput struct {
	Pattern string `json:"pattern"`
	Path    string `json:"path"`
}

func (g *GlobTool) Name() string { return "Glob" }

// Description returns the 5-bullet description matching the TS prompt.ts.
// Source: prompt.ts DESCRIPTION
func (g *GlobTool) Description() string {
	return "- Fast file pattern matching tool that works with any codebase size\n" +
		"- Supports glob patterns like \"**/*.js\" or \"src/**/*.ts\"\n" +
		"- Returns matching file paths sorted by modification time\n" +
		"- Use this tool when you need to find files by name patterns\n" +
		"- When you are doing an open ended search that may require multiple rounds of globbing and grepping, use the Agent tool instead"
}

func (g *GlobTool) IsReadOnly() bool { return true }

func (g *GlobTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"pattern": {"type": "string", "description": "The glob pattern to match files against"},
			"path": {"type": "string", "description": "The directory to search in. If not specified, the current working directory will be used. IMPORTANT: Omit this field to use the default directory. DO NOT enter \"undefined\" or \"null\" - simply omit it for the default behavior. Must be a valid directory path if provided."}
		},
		"required": ["pattern"],
		"additionalProperties": false
	}`)
}

// globFileInfo holds a matched file path and its modification time for sorting.
type globFileInfo struct {
	relPath string
	modTime time.Time
}

func (g *GlobTool) Execute(_ context.Context, tc *ToolContext, input json.RawMessage) (*ToolOutput, error) {
	var in globInput
	if err := json.Unmarshal(input, &in); err != nil {
		return ErrorOutput(fmt.Sprintf("invalid input: %s", err)), nil
	}
	if in.Pattern == "" {
		return ErrorOutput("pattern is required"), nil
	}

	searchPath := tc.CWD
	if in.Path != "" {
		if filepath.IsAbs(in.Path) {
			searchPath = in.Path
		} else {
			searchPath = filepath.Join(tc.CWD, in.Path)
		}
	}

	// Validate search path exists and is a directory.
	// Source: GlobTool.ts:94-131 — validateInput
	info, err := os.Stat(searchPath)
	if err != nil {
		if os.IsNotExist(err) {
			return ErrorOutput(fmt.Sprintf(
				"Directory does not exist: %s. To find the right path, use Glob with a broader pattern or check the current working directory with Bash. Current working directory: %s.",
				searchPath, tc.CWD,
			)), nil
		}
		return ErrorOutput(fmt.Sprintf("path does not exist: %s", err)), nil
	}
	if !info.IsDir() {
		return ErrorOutput(fmt.Sprintf("Path is not a directory: %s", searchPath)), nil
	}

	// Load gitignore and claudeignore rules for filtering.
	ig := loadIgnoreRules(searchPath)

	var matches []globFileInfo

	pattern := filepath.ToSlash(in.Pattern)

	walkErr := filepath.WalkDir(searchPath, func(path string, d os.DirEntry, wErr error) error {
		if wErr != nil {
			return nil // skip files/dirs we can't access
		}

		relPath, relErr := filepath.Rel(searchPath, path)
		if relErr != nil {
			return nil
		}

		// Always skip .git directory.
		if d.IsDir() && d.Name() == ".git" {
			return filepath.SkipDir
		}

		// Check gitignore/claudeignore rules for directories (skip early).
		if d.IsDir() {
			if ig != nil && ig.MatchesPath(relPath+"/") {
				return filepath.SkipDir
			}
			return nil
		}

		// Check gitignore/claudeignore rules for files.
		if ig != nil && ig.MatchesPath(relPath) {
			return nil
		}

		// Use doublestar for proper ** glob matching.
		matchPath := filepath.ToSlash(relPath)

		matched, matchErr := doublestar.PathMatch(pattern, matchPath)
		if matchErr != nil {
			return nil
		}

		if matched {
			fi, fiErr := d.Info()
			if fiErr != nil {
				return nil
			}
			matches = append(matches, globFileInfo{
				relPath: relPath,
				modTime: fi.ModTime(),
			})
		}

		return nil
	})

	if walkErr != nil {
		return ErrorOutput(fmt.Sprintf("error walking directory: %s", walkErr)), nil
	}

	// Sort by modification time, newest first.
	// Source: prompt.ts — "sorted by modification time"
	// Source: porting notes — "latest-changed files surface first"
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].modTime.After(matches[j].modTime)
	})

	if len(matches) == 0 {
		// Source: GlobTool.ts:178-183
		return SuccessOutput("No files found"), nil
	}

	// Enforce max results limit with truncation message.
	// Source: GlobTool.ts:157, 190-193
	truncated := len(matches) > DefaultGlobMaxResults
	if truncated {
		matches = matches[:DefaultGlobMaxResults]
	}

	lines := make([]string, len(matches))
	for i, m := range matches {
		lines[i] = m.relPath
	}

	result := strings.Join(lines, "\n")
	if truncated {
		result += "\n" + GlobTruncationMessage
	}

	return SuccessOutput(result), nil
}

// loadIgnoreRules loads .gitignore and .claudeignore patterns from the search
// directory (and its parents up to the filesystem root). Returns nil if no
// ignore files are found.
func loadIgnoreRules(searchPath string) *ignore.GitIgnore {
	var patterns []string

	// Walk up from searchPath collecting ignore patterns.
	dir := searchPath
	for {
		for _, name := range []string{".gitignore", ".claudeignore"} {
			p := filepath.Join(dir, name)
			data, err := os.ReadFile(p)
			if err != nil {
				continue
			}
			for _, line := range strings.Split(string(data), "\n") {
				line = strings.TrimSpace(line)
				if line == "" || strings.HasPrefix(line, "#") {
					continue
				}
				patterns = append(patterns, line)
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	if len(patterns) == 0 {
		return nil
	}

	return ignore.CompileIgnoreLines(patterns...)
}
