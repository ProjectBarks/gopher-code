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
	"sort"
	"strconv"
	"strings"
)

// DefaultGrepHeadLimit is the default cap on results when head_limit is unspecified.
// Source: GrepTool.ts:104-108
const DefaultGrepHeadLimit = 250

// GrepTool searches for patterns in files.
type GrepTool struct{}

// grepInput holds the parsed input for a grep invocation.
// Fields with dash-prefixed JSON keys (-B, -A, -C, -n, -i) require
// custom unmarshaling because Go's encoding/json treats "-" specially.
type grepInput struct {
	Pattern         string `json:"pattern"`
	Path            string `json:"path"`
	Glob            string `json:"glob"`
	Type            string `json:"type"`
	OutputMode      string `json:"output_mode"`
	ContextBefore   *int
	ContextAfter    *int
	ContextC        *int
	Context         *int
	ShowLineNums    *bool
	CaseInsensitive *bool
	HeadLimit       *int
	Offset          int
	Multiline       bool
}

// UnmarshalJSON handles the dash-prefixed field names (-B, -A, -C, -n, -i)
// that Go's standard json tags cannot express.
func (g *grepInput) UnmarshalJSON(data []byte) error {
	// Use a raw map to capture all fields including dash-prefixed ones.
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if v, ok := raw["pattern"]; ok {
		json.Unmarshal(v, &g.Pattern)
	}
	if v, ok := raw["path"]; ok {
		json.Unmarshal(v, &g.Path)
	}
	if v, ok := raw["glob"]; ok {
		json.Unmarshal(v, &g.Glob)
	}
	if v, ok := raw["type"]; ok {
		json.Unmarshal(v, &g.Type)
	}
	if v, ok := raw["output_mode"]; ok {
		json.Unmarshal(v, &g.OutputMode)
	}
	if v, ok := raw["-B"]; ok {
		var n int
		if json.Unmarshal(v, &n) == nil {
			g.ContextBefore = &n
		}
	}
	if v, ok := raw["-A"]; ok {
		var n int
		if json.Unmarshal(v, &n) == nil {
			g.ContextAfter = &n
		}
	}
	if v, ok := raw["-C"]; ok {
		var n int
		if json.Unmarshal(v, &n) == nil {
			g.ContextC = &n
		}
	}
	if v, ok := raw["context"]; ok {
		var n int
		if json.Unmarshal(v, &n) == nil {
			g.Context = &n
		}
	}
	if v, ok := raw["-n"]; ok {
		var b bool
		if json.Unmarshal(v, &b) == nil {
			g.ShowLineNums = &b
		}
	}
	if v, ok := raw["-i"]; ok {
		var b bool
		if json.Unmarshal(v, &b) == nil {
			g.CaseInsensitive = &b
		}
	}
	if v, ok := raw["head_limit"]; ok {
		var n int
		if json.Unmarshal(v, &n) == nil {
			g.HeadLimit = &n
		}
	}
	if v, ok := raw["offset"]; ok {
		json.Unmarshal(v, &g.Offset)
	}
	if v, ok := raw["multiline"]; ok {
		json.Unmarshal(v, &g.Multiline)
	}
	return nil
}

func (g *GrepTool) Name() string { return "Grep" }

// Description returns the full prompt text matching the TS source.
// Source: prompt.ts:6-18
func (g *GrepTool) Description() string {
	return `A powerful search tool built on ripgrep

  Usage:
  - ALWAYS use Grep for search tasks. NEVER invoke ` + "`grep`" + ` or ` + "`rg`" + ` as a Bash command. The Grep tool has been optimized for correct permissions and access.
  - Supports full regex syntax (e.g., "log.*Error", "function\s+\w+")
  - Filter files with glob parameter (e.g., "*.js", "**/*.tsx") or type parameter (e.g., "js", "py", "rust")
  - Output modes: "content" shows matching lines, "files_with_matches" shows only file paths (default), "count" shows match counts
  - Use Agent tool for open-ended searches requiring multiple rounds
  - Pattern syntax: Uses ripgrep (not grep) - literal braces need escaping (use ` + "`interface\\{\\}`" + ` to find ` + "`interface{}`" + ` in Go code)
  - Multiline matching: By default patterns match within single lines only. For cross-line patterns like ` + "`struct \\{[\\s\\S]*?field`" + `, use ` + "`multiline: true`" + `
`
}

func (g *GrepTool) IsReadOnly() bool { return true }

// SearchHint implements SearchHinter for tool discovery.
// Source: GrepTool.ts:162
func (g *GrepTool) SearchHint() string {
	return "search file contents with regex (ripgrep)"
}

// IsConcurrencySafe implements ConcurrencySafeChecker.
// Source: GrepTool.ts:184-186
func (g *GrepTool) IsConcurrencySafe(_ json.RawMessage) bool { return true }

// Prompt implements ToolPrompter — returns same text as Description.
// Source: GrepTool.ts:241-243
func (g *GrepTool) Prompt() string { return g.Description() }

// Source: GrepTool.ts:30-92
func (g *GrepTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"pattern": {"type": "string", "description": "The regular expression pattern to search for in file contents"},
			"path": {"type": "string", "description": "File or directory to search in (rg PATH). Defaults to current working directory."},
			"glob": {"type": "string", "description": "Glob pattern to filter files (e.g. \"*.js\", \"*.{ts,tsx}\") - maps to rg --glob"},
			"type": {"type": "string", "description": "File type to search (rg --type). Common types: js, py, rust, go, java, etc. More efficient than include for standard file types."},
			"output_mode": {"type": "string", "enum": ["content", "files_with_matches", "count"], "description": "Output mode: \"content\" shows matching lines (supports -A/-B/-C context, -n line numbers, head_limit), \"files_with_matches\" shows file paths (supports head_limit), \"count\" shows match counts (supports head_limit). Defaults to \"files_with_matches\"."},
			"-B": {"type": "number", "description": "Number of lines to show before each match (rg -B). Requires output_mode: \"content\", ignored otherwise."},
			"-A": {"type": "number", "description": "Number of lines to show after each match (rg -A). Requires output_mode: \"content\", ignored otherwise."},
			"-C": {"type": "number", "description": "Alias for context."},
			"context": {"type": "number", "description": "Number of lines to show before and after each match (rg -C). Requires output_mode: \"content\", ignored otherwise."},
			"-n": {"type": "boolean", "description": "Show line numbers in output (rg -n). Requires output_mode: \"content\", ignored otherwise. Defaults to true."},
			"-i": {"type": "boolean", "description": "Case insensitive search (rg -i)"},
			"head_limit": {"type": "number", "description": "Limit output to first N lines/entries, equivalent to \"| head -N\". Works across all output modes: content (limits output lines), files_with_matches (limits file paths), count (limits count entries). Defaults to 250 when unspecified. Pass 0 for unlimited (use sparingly — large result sets waste context)."},
			"offset": {"type": "number", "description": "Skip first N lines/entries before applying head_limit, equivalent to \"| tail -n +N | head -N\". Works across all output modes. Defaults to 0."},
			"multiline": {"type": "boolean", "description": "Enable multiline mode where . matches newlines and patterns can span lines (rg -U --multiline-dotall). Default: false."}
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

	// Validate path exists.
	// Source: GrepTool.ts:201-229
	if in.Path != "" {
		if _, err := os.Stat(searchPath); os.IsNotExist(err) {
			return ErrorOutput(fmt.Sprintf("Path does not exist: %s. Make sure the path is correct and accessible from the current working directory: %s.", in.Path, tc.CWD)), nil
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

	// Fall back to Go-native implementation
	return g.executeNative(searchPath, &in, outputMode)
}

// Source: GrepTool.ts:329-440
func (g *GrepTool) executeWithRg(ctx context.Context, rgPath, searchPath string, in *grepInput, outputMode string) (*ToolOutput, error) {
	args := []string{"--hidden", "--max-columns", "500"}

	// VCS exclusions
	// Source: GrepTool.ts:333-335
	for _, dir := range []string{".git", ".svn", ".hg", ".bzr", ".jj", ".sl"} {
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

	// Glob filter — brace-aware splitting
	// Source: GrepTool.ts:392-409
	if in.Glob != "" {
		for _, gp := range splitGlobPatterns(in.Glob) {
			args = append(args, "--glob", gp)
		}
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
			// No matches
			return g.formatNoMatches(outputMode), nil
		}
		return ErrorOutput(fmt.Sprintf("rg failed: %s\n%s", err, stderr.String())), nil
	}

	output := stdout.String()
	if output == "" {
		return g.formatNoMatches(outputMode), nil
	}

	lines := strings.Split(strings.TrimRight(output, "\n"), "\n")

	// Convert absolute paths to relative paths to save tokens
	// Source: GrepTool.ts:456-465, 488-497
	for i, line := range lines {
		lines[i] = relativizeLine(line, searchPath, outputMode)
	}

	return g.formatOutput(lines, outputMode, in.HeadLimit, in.Offset, searchPath)
}

func (g *GrepTool) executeNative(searchPath string, in *grepInput, outputMode string) (*ToolOutput, error) {
	pattern := in.Pattern
	// Case-insensitive: compile with (?i) prefix
	if in.CaseInsensitive != nil && *in.CaseInsensitive {
		pattern = "(?i)" + pattern
	}
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
		if in.Glob != "" {
			matched, _ := filepath.Match(in.Glob, d.Name())
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

		// Determine line number display
		showLineNums := true
		if in.ShowLineNums != nil {
			showLineNums = *in.ShowLineNums
		}

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
					if showLineNums {
						results = append(results, fmt.Sprintf("%s:%d:%s", relPath, lineNum, line))
					} else {
						results = append(results, fmt.Sprintf("%s:%s", relPath, line))
					}
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
		return g.formatNoMatches(outputMode), nil
	}

	return g.formatOutput(results, outputMode, in.HeadLimit, in.Offset, searchPath)
}

// formatNoMatches returns the appropriate no-match message per output mode.
// Source: GrepTool.ts:269, 295-300
func (g *GrepTool) formatNoMatches(outputMode string) *ToolOutput {
	if outputMode == "files_with_matches" {
		return SuccessOutput("No files found")
	}
	return SuccessOutput("No matches found")
}

// formatOutput applies head_limit/offset and formats the result string
// according to the output mode, matching the TS mapToolResultToToolResultBlockParam.
// Source: GrepTool.ts:256-308
func (g *GrepTool) formatOutput(lines []string, outputMode string, headLimit *int, offset int, _ string) (*ToolOutput, error) {
	switch outputMode {
	case "content":
		limited, limitInfo := applyGrepHeadLimit(lines, headLimit, offset)
		content := strings.Join(limited, "\n")
		if content == "" {
			content = "No matches found"
		}
		if limitInfo != "" {
			content += "\n\n[Showing results with pagination = " + limitInfo + "]"
		}
		return SuccessOutput(content), nil

	case "count":
		limited, limitInfo := applyGrepHeadLimit(lines, headLimit, offset)
		content := strings.Join(limited, "\n")
		if content == "" {
			content = "No matches found"
		}
		// Parse counts for summary
		totalMatches := 0
		fileCount := 0
		for _, line := range limited {
			idx := strings.LastIndex(line, ":")
			if idx > 0 {
				if n, err := strconv.Atoi(line[idx+1:]); err == nil {
					totalMatches += n
					fileCount++
				}
			}
		}
		summary := fmt.Sprintf("\n\nFound %d total %s across %d %s.",
			totalMatches, plural(totalMatches, "occurrence"),
			fileCount, plural(fileCount, "file"))
		if limitInfo != "" {
			summary = summary[:len(summary)-1] + " with pagination = " + limitInfo + "."
		}
		return SuccessOutput(content + summary), nil

	default: // files_with_matches
		// Sort by mtime (newest first), filename tiebreaker.
		// In test mode, sort by filename only for determinism.
		// Source: GrepTool.ts:529-554
		sortFilesByMtime(lines)

		limited, limitInfo := applyGrepHeadLimit(lines, headLimit, offset)
		if len(limited) == 0 {
			return SuccessOutput("No files found"), nil
		}
		header := fmt.Sprintf("Found %d %s", len(limited), plural(len(limited), "file"))
		if limitInfo != "" {
			header += " " + limitInfo
		}
		result := header + "\n" + strings.Join(limited, "\n")
		return SuccessOutput(result), nil
	}
}

// sortFilesByMtime sorts file paths by modification time (newest first),
// with filename as tiebreaker. If GOPHER_TEST_DETERMINISTIC is set or
// stat fails, sorts by filename only.
// Source: GrepTool.ts:529-553
func sortFilesByMtime(files []string) {
	deterministic := os.Getenv("GOPHER_TEST_DETERMINISTIC") != ""

	type entry struct {
		path  string
		mtime int64
	}
	entries := make([]entry, len(files))
	for i, f := range files {
		entries[i].path = f
		if !deterministic {
			if info, err := os.Stat(f); err == nil {
				entries[i].mtime = info.ModTime().UnixNano()
			}
		}
	}

	sort.SliceStable(entries, func(i, j int) bool {
		if deterministic {
			return entries[i].path < entries[j].path
		}
		if entries[i].mtime != entries[j].mtime {
			return entries[i].mtime > entries[j].mtime // newest first
		}
		return entries[i].path < entries[j].path
	})

	for i, e := range entries {
		files[i] = e.path
	}
}

// relativizeLine converts absolute paths in rg output lines to relative paths.
// Source: GrepTool.ts:456-465 (content), 488-497 (count)
func relativizeLine(line, searchPath, outputMode string) string {
	switch outputMode {
	case "content":
		// Format: /absolute/path:linenum:content or /absolute/path-content (context)
		idx := strings.Index(line, ":")
		if idx > 0 {
			filePath := line[:idx]
			rest := line[idx:]
			if rel, err := filepath.Rel(searchPath, filePath); err == nil {
				return rel + rest
			}
		}
	case "count":
		// Format: /absolute/path:count
		idx := strings.LastIndex(line, ":")
		if idx > 0 {
			filePath := line[:idx]
			rest := line[idx:]
			if rel, err := filepath.Rel(searchPath, filePath); err == nil {
				return rel + rest
			}
		}
	default: // files_with_matches
		if rel, err := filepath.Rel(searchPath, line); err == nil {
			return rel
		}
	}
	return line
}

// splitGlobPatterns splits a glob string on whitespace, preserving brace patterns.
// Source: GrepTool.ts:392-409
func splitGlobPatterns(glob string) []string {
	rawPatterns := strings.Fields(glob)
	var result []string
	for _, rp := range rawPatterns {
		if strings.Contains(rp, "{") && strings.Contains(rp, "}") {
			// Brace pattern — keep as-is
			result = append(result, rp)
		} else {
			// Split on commas for patterns without braces
			for _, p := range strings.Split(rp, ",") {
				if p != "" {
					result = append(result, p)
				}
			}
		}
	}
	return result
}

// plural returns word or word+"s" based on count.
// Source: GrepTool.ts plural helper
func plural(n int, word string) string {
	if n == 1 {
		return word
	}
	return word + "s"
}

// applyGrepHeadLimit applies head_limit and offset to results.
// Source: GrepTool.ts:110-128 (applyHeadLimit)
func applyGrepHeadLimit(items []string, limit *int, offset int) ([]string, string) {
	// Explicit 0 = unlimited
	if limit != nil && *limit == 0 {
		if offset > 0 && offset < len(items) {
			return items[offset:], formatLimitInfo(nil, offset)
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

	// Only report appliedLimit when truncation actually occurred
	// Source: GrepTool.ts:123-127
	var appliedLimit *int
	if wasTruncated {
		appliedLimit = &effectiveLimit
	}

	info := formatLimitInfo(appliedLimit, offset)
	return sliced, info
}

// formatLimitInfo builds the (limit: N, offset: N) string.
// Source: GrepTool.ts:134-142
func formatLimitInfo(appliedLimit *int, offset int) string {
	var parts []string
	if appliedLimit != nil {
		parts = append(parts, fmt.Sprintf("limit: %d", *appliedLimit))
	}
	if offset > 0 {
		parts = append(parts, fmt.Sprintf("offset: %d", offset))
	}
	if len(parts) == 0 {
		return ""
	}
	return "(" + strings.Join(parts, ", ") + ")"
}
