package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// FileEditTool performs string replacements in files.
type FileEditTool struct{}

type fileEditInput struct {
	FilePath  string `json:"file_path"`
	OldString string `json:"old_string"`
	NewString string `json:"new_string"`
}

func (f *FileEditTool) Name() string        { return "Edit" }
func (f *FileEditTool) Description() string { return "Perform exact string replacement in a file" }
func (f *FileEditTool) IsReadOnly() bool    { return false }

func (f *FileEditTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"file_path": {"type": "string", "description": "The path to the file to edit"},
			"old_string": {"type": "string", "description": "The text to find and replace"},
			"new_string": {"type": "string", "description": "The replacement text"}
		},
		"required": ["file_path", "old_string", "new_string"],
		"additionalProperties": false
	}`)
}

func (f *FileEditTool) Execute(_ context.Context, tc *ToolContext, input json.RawMessage) (*ToolOutput, error) {
	var in fileEditInput
	if err := json.Unmarshal(input, &in); err != nil {
		return ErrorOutput(fmt.Sprintf("invalid input: %s", err)), nil
	}
	if in.FilePath == "" {
		return ErrorOutput("file_path is required"), nil
	}
	if in.OldString == "" {
		return ErrorOutput("old_string is required"), nil
	}
	if in.OldString == in.NewString {
		return ErrorOutput("old_string and new_string must be different"), nil
	}

	path := in.FilePath
	if !filepath.IsAbs(path) {
		path = filepath.Join(tc.CWD, path)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return ErrorOutput(fmt.Sprintf("failed to read file: %s", err)), nil
	}

	content := string(data)
	count := strings.Count(content, in.OldString)

	if count == 0 {
		return ErrorOutput("old_string not found in file"), nil
	}
	if count > 1 {
		return ErrorOutput(fmt.Sprintf("old_string found %d times in file (must be unique)", count)), nil
	}

	newContent := strings.Replace(content, in.OldString, in.NewString, 1)

	info, err := os.Stat(path)
	if err != nil {
		return ErrorOutput(fmt.Sprintf("failed to stat file: %s", err)), nil
	}

	if err := os.WriteFile(path, []byte(newContent), info.Mode()); err != nil {
		return ErrorOutput(fmt.Sprintf("failed to write file: %s", err)), nil
	}

	return SuccessOutput(fmt.Sprintf("Successfully edited %s", path)), nil
}
