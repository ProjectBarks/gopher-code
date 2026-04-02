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

// GrepTool searches for patterns in files.
type GrepTool struct{}

type grepInput struct {
	Pattern string `json:"pattern"`
	Path    string `json:"path"`
	Glob    string `json:"glob"`
}

func (g *GrepTool) Name() string        { return "Grep" }
func (g *GrepTool) Description() string { return "Search for patterns in files" }
func (g *GrepTool) IsReadOnly() bool    { return true }

func (g *GrepTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"pattern": {"type": "string", "description": "The regex pattern to search for"},
			"path": {"type": "string", "description": "File or directory to search in"},
			"glob": {"type": "string", "description": "File glob filter (e.g. \"*.go\")"}
		},
		"required": ["pattern"],
		"additionalProperties": false
	}`)
}

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

	// Try ripgrep first
	if rgPath, err := exec.LookPath("rg"); err == nil {
		return g.executeWithRg(ctx, rgPath, in.Pattern, searchPath, in.Glob)
	}

	// Fall back to Go-native implementation
	return g.executeNative(in.Pattern, searchPath, in.Glob)
}

func (g *GrepTool) executeWithRg(ctx context.Context, rgPath, pattern, searchPath, globFilter string) (*ToolOutput, error) {
	args := []string{"--no-heading", "-n", pattern, searchPath}
	if globFilter != "" {
		args = []string{"--no-heading", "-n", "--glob", globFilter, pattern, searchPath}
	}

	cmd := exec.CommandContext(ctx, rgPath, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		// rg returns exit code 1 when no matches found
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return SuccessOutput("No matches found"), nil
		}
		return ErrorOutput(fmt.Sprintf("rg failed: %s\n%s", err, stderr.String())), nil
	}

	output := stdout.String()
	if output == "" {
		return SuccessOutput("No matches found"), nil
	}

	return SuccessOutput(output), nil
}

func (g *GrepTool) executeNative(pattern, searchPath, globFilter string) (*ToolOutput, error) {
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

		// Apply glob filter if provided
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

		for scanner.Scan() {
			lineNum++
			line := scanner.Text()
			if re.MatchString(line) {
				relPath, relErr := filepath.Rel(searchPath, path)
				if relErr != nil {
					relPath = path
				}
				results = append(results, fmt.Sprintf("%s:%d:%s", relPath, lineNum, line))
			}
		}

		return nil
	})

	if err != nil {
		return ErrorOutput(fmt.Sprintf("error walking directory: %s", err)), nil
	}

	if len(results) == 0 {
		return SuccessOutput("No matches found"), nil
	}

	return SuccessOutput(strings.Join(results, "\n") + "\n"), nil
}
