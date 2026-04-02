package tools

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// DefaultGrepHeadLimit is the default cap on results when head_limit is unspecified.
// Source: GrepTool.ts:104-108
const DefaultGrepHeadLimit = 250

// GrepTool searches for patterns in files.
type GrepTool struct{}

type grepInput struct {
	Pattern       string `json:"pattern"`
	Path          string `json:"path"`
	Glob          string `json:"glob"`
	Type          string `json:"type"`
	OutputMode    string `json:"output_mode"`
	ContextBefore *int   `json:"-B"`
	ContextAfter  *int   `json:"-A"`
	ContextC      *int   `json:"-C"`
	Context       *int   `json:"context"`
	ShowLineNums  *bool  `json:"-n"`
	CaseInsensitive *bool `json:"-i"`
	HeadLimit     *int   `json:"head_limit"`
	Offset        int    `json:"offset"`
	Multiline     bool   `json:"multiline"`
}

func (g *GrepTool) Name() string        { return "Grep" }
func (g *GrepTool) Description() string { return "A powerful search tool built on ripgrep" }
func (g *GrepTool) IsReadOnly() bool    { return true }

// Source: GrepTool.ts:30-92
func (g *GrepTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"pattern": {"type": "string", "description": "The regular expression pattern to search for in file contents"},
			"path": {"type": "string", "description": "File or directory to search in (rg PATH). Defaults to current working directory."},
			"glob": {"type": "string", "description": "Glob pattern to filter files (e.g. \"*.js\", \"*.{ts,tsx}\") - maps to rg --glob"},
			"type": {"type": "string", "description": "File type to search (rg --type). Common types: js, py, rust, go, java, etc."},
			"output_mode": {"type": "string", "enum": ["content", "files_with_matches", "count"], "description": "Output mode: \"content\" shows matching lines, \"files_with_matches\" shows file paths (default), \"count\" shows match counts."},
			"-B": {"type": "number", "description": "Number of lines to show before each match (rg -B). Requires output_mode: \"content\"."},
			"-A": {"type": "number", "description": "Number of lines to show after each match (rg -A). Requires output_mode: \"content\"."},
			"-C": {"type": "number", "description": "Alias for context."},
			"context": {"type": "number", "description": "Number of lines to show before and after each match (rg -C). Requires output_mode: \"content\"."},
			"-n": {"type": "boolean", "description": "Show line numbers in output (rg -n). Requires output_mode: \"content\". Defaults to true."},
			"-i": {"type": "boolean", "description": "Case insensitive search (rg -i)"},
			"head_limit": {"type": "number", "description": "Limit output to first N lines/entries. Defaults to 250. Pass 0 for unlimited."},
			"offset": {"type": "number", "description": "Skip first N lines/entries before applying head_limit. Defaults to 0."},
			"multiline": {"type": "boolean", "description": "Enable multiline mode (rg -U --multiline-dotall). Default: false."}
		},
		"required": ["pattern"],
		"additionalProperties": false
	}`)
}

// Source: GrepTool.ts:310-530
func (g *GrepTool) Execute(ctx context.Context, tc *ToolContext, input json.RawMessage) (*ToolOutput, error) {
	var in grepInput
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

	outputMode := in.OutputMode
	if outputMode == "" {
		outputMode = "files_with_matches"
	}

	// Try ripgrep first
	if rgPath, err := exec.LookPath("rg"); err == nil {
		return g.executeWithRg(ctx, rgPath, searchPath, &in, outputMode)
	}

	// Fall back to Go-native implementation (files_with_matches only)
	return g.executeNative(in.Pattern, searchPath, in.Glob, outputMode, in.HeadLimit, in.Offset)
}

// Source: GrepTool.ts:329-440
func (g *GrepTool) executeWithRg(ctx context.Context, rgPath, searchPath string, in *grepInput, outputMode string) (*ToolOutput, error) {
	args := []string{"--hidden", "--max-columns", "500"}

	// VCS exclusions
	// Source: GrepTool.ts:333-335
	for _, dir := range []string{".git", ".hg", ".svn", ".jj", ".sl"} {
		args = append(args, "--glob", "!"+dir)
	}

	// Multiline
	// Source: GrepTool.ts:341-343
	if in.Multiline {
		args = append(args, "-U", "--multiline-dotall")
	}

	// Case insensitive
	if in.CaseInsensitive != nil && *in.CaseInsensitive {
		args = append(args, "-i")
	}

	// Output mode flags
	// Source: GrepTool.ts:351-355
	switch outputMode {
	case "files_with_matches":
		args = append(args, "-l")
	case "count":
		args = append(args, "-c")
	}

	// Line numbers for content mode
	// Source: GrepTool.ts:358-360
	showLineNums := true
	if in.ShowLineNums != nil {
		showLineNums = *in.ShowLineNums
	}
	if showLineNums && outputMode == "content" {
		args = append(args, "-n")
	}

	// Context flags (content mode only)
	// Source: GrepTool.ts:363-376
	if outputMode == "content" {
		if in.Context != nil {
			args = append(args, "-C", fmt.Sprintf("%d", *in.Context))
		} else if in.ContextC != nil {
			args = append(args, "-C", fmt.Sprintf("%d", *in.ContextC))
		} else {
			if in.ContextBefore != nil {
				args = append(args, "-B", fmt.Sprintf("%d", *in.ContextBefore))
			}
			if in.ContextAfter != nil {
				args = append(args, "-A", fmt.Sprintf("%d", *in.ContextAfter))
			}
		}
	}

	// Pattern (handle leading dash)
	// Source: GrepTool.ts:380-384
	if strings.HasPrefix(in.Pattern, "-") {
		args = append(args, "-e", in.Pattern)
	} else {
		args = append(args, in.Pattern)
	}

	// Type filter
	if in.Type != "" {
		args = append(args, "--type", in.Type)
	}

	// Glob filter
	if in.Glob != "" {
		args = append(args, "--glob", in.Glob)
	}

	// Add search path
	args = append(args, searchPath)

	cmd := exec.CommandContext(ctx, rgPath, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return SuccessOutput("No matches found"), nil
		}
		return ErrorOutput(fmt.Sprintf("rg failed: %s\n%s", err, stderr.String())), nil
	}

	output := stdout.String()
	if output == "" {
		return SuccessOutput("No matches found"), nil
	}

	// Apply head_limit and offset
	// Source: GrepTool.ts:450-454, 481-485
	lines := strings.Split(strings.TrimRight(output, "\n"), "\n")
	lines, limitInfo := applyGrepHeadLimit(lines, in.HeadLimit, in.Offset)

	result := strings.Join(lines, "\n")
	if limitInfo != "" {
		result += "\n" + limitInfo
	}

	return SuccessOutput(result), nil
}

func (g *GrepTool) executeNative(pattern, searchPath, globFilter, outputMode string, headLimit *int, offset int) (*ToolOutput, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return ErrorOutput(fmt.Sprintf("invalid regex pattern: %s", err)), nil
	}

	var results []string

	err = filepath.WalkDir(searchPath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if skipDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if globFilter != "" {
			matched, _ := filepath.Match(globFilter, d.Name())
			if !matched {
				return nil
			}
		}

		file, err := os.Open(path)
		if err != nil {
			return nil
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		lineNum := 0
		matchCount := 0

		for scanner.Scan() {
			lineNum++
			line := scanner.Text()
			if re.MatchString(line) {
				matchCount++
				relPath, relErr := filepath.Rel(searchPath, path)
				if relErr != nil {
					relPath = path
				}
				switch outputMode {
				case "content":
					results = append(results, fmt.Sprintf("%s:%d:%s", relPath, lineNum, line))
				case "count":
					// Handled after file scan
				default: // files_with_matches
					results = append(results, relPath)
					return nil // One match is enough for this file
				}
			}
		}

		if outputMode == "count" && matchCount > 0 {
			relPath, relErr := filepath.Rel(searchPath, path)
			if relErr != nil {
				relPath = path
			}
			results = append(results, fmt.Sprintf("%s:%d", relPath, matchCount))
		}

		return nil
	})

	if err != nil {
		return ErrorOutput(fmt.Sprintf("error walking directory: %s", err)), nil
	}

	if len(results) == 0 {
		return SuccessOutput("No matches found"), nil
	}

	// Apply head_limit and offset
	results, limitInfo := applyGrepHeadLimit(results, headLimit, offset)

	result := strings.Join(results, "\n")
	if limitInfo != "" {
		result += "\n" + limitInfo
	}

	return SuccessOutput(result), nil
}

// applyGrepHeadLimit applies head_limit and offset to results.
// Source: GrepTool.ts:110-128 (applyHeadLimit)
func applyGrepHeadLimit(items []string, limit *int, offset int) ([]string, string) {
	// Explicit 0 = unlimited
	if limit != nil && *limit == 0 {
		if offset > 0 && offset < len(items) {
			return items[offset:], ""
		}
		return items, ""
	}

	effectiveLimit := DefaultGrepHeadLimit
	if limit != nil {
		effectiveLimit = *limit
	}

	// Apply offset
	start := offset
	if start > len(items) {
		start = len(items)
	}
	end := start + effectiveLimit
	if end > len(items) {
		end = len(items)
	}

	sliced := items[start:end]
	wasTruncated := len(items)-start > effectiveLimit

	// Format limit info
	// Source: GrepTool.ts:134-142
	var info string
	if wasTruncated || offset > 0 {
		var parts []string
		if wasTruncated {
			parts = append(parts, fmt.Sprintf("limit: %d", effectiveLimit))
		}
		if offset > 0 {
			parts = append(parts, fmt.Sprintf("offset: %d", offset))
		}
		info = "(" + strings.Join(parts, ", ") + ")"
	}

	return sliced, info
}
