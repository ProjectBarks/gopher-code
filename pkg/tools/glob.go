package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// DefaultGlobMaxResults is the default max results for glob operations.
// Source: GlobTool.ts:157
const DefaultGlobMaxResults = 100

// GlobTruncationMessage is appended when results are truncated.
// Source: GlobTool.ts:191-193
const GlobTruncationMessage = "(Results are truncated. Consider using a more specific path or pattern.)"

// GlobTool finds files matching a glob pattern.
type GlobTool struct{}

type globInput struct {
	Pattern string `json:"pattern"`
	Path    string `json:"path"`
}

// skipDirs lists directories to skip during traversal.
var skipDirs = map[string]bool{
	".git":        true,
	"node_modules": true,
	"vendor":      true,
	"__pycache__": true,
	".idea":       true,
	".vscode":     true,
}

func (g *GlobTool) Name() string        { return "Glob" }
func (g *GlobTool) Description() string { return "Find files matching a glob pattern" }
func (g *GlobTool) IsReadOnly() bool    { return true }

func (g *GlobTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"pattern": {"type": "string", "description": "The glob pattern to match files against"},
			"path": {"type": "string", "description": "Directory to search in (defaults to CWD)"}
		},
		"required": ["pattern"],
		"additionalProperties": false
	}`)
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

	// Check that the search path exists
	if _, err := os.Stat(searchPath); err != nil {
		return ErrorOutput(fmt.Sprintf("path does not exist: %s", err)), nil
	}

	var matches []string

	err := filepath.WalkDir(searchPath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // skip files/dirs we can't access
		}

		if d.IsDir() {
			if skipDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}

		// Match against the file name or relative path depending on the pattern
		name := d.Name()
		relPath, relErr := filepath.Rel(searchPath, path)
		if relErr != nil {
			relPath = path
		}

		// Try matching against both the filename and the relative path
		matched, _ := filepath.Match(in.Pattern, name)
		if !matched {
			matched, _ = filepath.Match(in.Pattern, relPath)
		}
		// Also support ** prefix by matching just the base
		if !matched && strings.Contains(in.Pattern, "**") {
			// Extract the base pattern after **/ or **/
			basePattern := in.Pattern
			if idx := strings.LastIndex(basePattern, "/"); idx >= 0 {
				basePattern = basePattern[idx+1:]
			}
			matched, _ = filepath.Match(basePattern, name)
		}

		if matched {
			matches = append(matches, relPath)
		}

		return nil
	})

	if err != nil {
		return ErrorOutput(fmt.Sprintf("error walking directory: %s", err)), nil
	}

	sort.Strings(matches)

	if len(matches) == 0 {
		// Source: GlobTool.ts:178-183
		return SuccessOutput("No files found"), nil
	}

	// Enforce max results limit with truncation message
	// Source: GlobTool.ts:157, 190-193
	truncated := len(matches) > DefaultGlobMaxResults
	if truncated {
		matches = matches[:DefaultGlobMaxResults]
	}

	result := strings.Join(matches, "\n")
	if truncated {
		result += "\n" + GlobTruncationMessage
	}

	return SuccessOutput(result), nil
}
