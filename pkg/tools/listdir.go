package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ListDirectoryTool lists directory contents.
type ListDirectoryTool struct{}

type listDirInput struct {
	Path string `json:"path"`
	All  bool   `json:"all"`
	Long bool   `json:"long"`
}

func (t *ListDirectoryTool) Name() string        { return "LS" }
func (t *ListDirectoryTool) Description() string { return "List directory contents" }
func (t *ListDirectoryTool) IsReadOnly() bool    { return true }

func (t *ListDirectoryTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"path": {"type": "string", "description": "Directory path (default: CWD)"},
			"all": {"type": "boolean", "description": "Include hidden files"},
			"long": {"type": "boolean", "description": "Long format with sizes and dates"}
		},
		"required": [],
		"additionalProperties": false
	}`)
}

func (t *ListDirectoryTool) Execute(_ context.Context, tc *ToolContext, input json.RawMessage) (*ToolOutput, error) {
	var in listDirInput
	if err := json.Unmarshal(input, &in); err != nil {
		return ErrorOutput(fmt.Sprintf("invalid input: %s", err)), nil
	}

	dir := tc.CWD
	if in.Path != "" {
		if filepath.IsAbs(in.Path) {
			dir = in.Path
		} else {
			dir = filepath.Join(tc.CWD, in.Path)
		}
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return ErrorOutput(fmt.Sprintf("failed to read directory: %s", err)), nil
	}

	var lines []string
	for _, entry := range entries {
		name := entry.Name()

		// Skip hidden files unless "all" is true
		if !in.All && strings.HasPrefix(name, ".") {
			continue
		}

		if entry.IsDir() {
			name += "/"
		}

		if in.Long {
			info, err := entry.Info()
			if err != nil {
				// If we can't stat, still show the name
				lines = append(lines, name)
				continue
			}
			mode := info.Mode().String()
			size := info.Size()
			modTime := info.ModTime().Format("2006-01-02 15:04")
			lines = append(lines, fmt.Sprintf("%s %8d %s %s", mode, size, modTime, name))
		} else {
			lines = append(lines, name)
		}
	}

	if len(lines) == 0 {
		return SuccessOutput("(empty directory)"), nil
	}

	return SuccessOutput(strings.Join(lines, "\n") + "\n"), nil
}
