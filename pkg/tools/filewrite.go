package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// FileWriteMaxResultSizeChars is the per-tool max result size.
// Source: FileWriteTool.ts:97 — maxResultSizeChars: 100_000
const FileWriteMaxResultSizeChars = 100_000

// FileWriteTool writes content to a file.
type FileWriteTool struct{}

type fileWriteInput struct {
	FilePath string `json:"file_path"`
	Content  string `json:"content"`
}

func (f *FileWriteTool) Name() string { return "Write" }

// Description returns the tool's short description.
// Source: FileWriteTool.ts:100 — verbatim
func (f *FileWriteTool) Description() string { return "Write a file to the local filesystem." }

func (f *FileWriteTool) IsReadOnly() bool { return false }

// SearchHint returns the search hint for tool discovery.
// Source: FileWriteTool.ts:96
func (f *FileWriteTool) SearchHint() string { return "create or overwrite files" }

// MaxResultSizeChars implements MaxResultSizeCharsProvider.
// Source: FileWriteTool.ts:97
func (f *FileWriteTool) MaxResultSizeChars() int { return FileWriteMaxResultSizeChars }

// Prompt returns the system prompt for the Write tool.
// Source: prompt.ts:10-17 — verbatim
func (f *FileWriteTool) Prompt() string {
	return `Writes a file to the local filesystem.

Usage:
- This tool will overwrite the existing file if there is one at the provided path.
- If this is an existing file, you MUST use the Read tool first to read the file's contents. This tool will fail if you did not read the file first.
- Prefer the Edit tool for modifying existing files — it only sends the diff. Only use this tool to create new files or for complete rewrites.
- NEVER create documentation files (*.md) or README files unless explicitly requested by the User.
- Only use emojis if the user explicitly requests it. Avoid writing emojis to files unless asked.`
}

func (f *FileWriteTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"file_path": {"type": "string", "description": "The absolute path to the file to write (must be absolute, not relative)"},
			"content": {"type": "string", "description": "The content to write to the file"}
		},
		"required": ["file_path", "content"],
		"additionalProperties": false
	}`)
}

// Source: FileWriteTool.ts:186-434
func (f *FileWriteTool) Execute(_ context.Context, tc *ToolContext, input json.RawMessage) (*ToolOutput, error) {
	var in fileWriteInput
	if err := json.Unmarshal(input, &in); err != nil {
		return ErrorOutput(fmt.Sprintf("invalid input: %s", err)), nil
	}
	if in.FilePath == "" {
		return ErrorOutput("file_path is required"), nil
	}

	path := in.FilePath
	if !filepath.IsAbs(path) {
		path = filepath.Join(tc.CWD, path)
	}

	// Determine if file exists (create vs update).
	// Source: FileWriteTool.ts:186-221 — validateInput
	fileExists := false
	var oldContent string
	if info, statErr := os.Stat(path); statErr == nil {
		fileExists = true
		_ = info // used for mtime check below

		// Check ReadFileState — file must have been fully read before overwriting.
		// Source: FileWriteTool.ts:198-205
		if tc.ReadFileState != nil {
			entry := tc.ReadFileState.Get(path)
			if entry == nil || entry.IsPartialView {
				return ErrorOutput("File has not been read yet. Read it first before writing to it."), nil
			}
			// Source: FileWriteTool.ts:211-218 — staleness mtime check
			if info.ModTime().After(entry.Timestamp) {
				return ErrorOutput("File has been modified since read, either by the user or by a linter. Read it again before attempting to write it."), nil
			}
		}

		// Capture old content for diff + checkpoint.
		if existing, readErr := os.ReadFile(path); readErr == nil {
			oldContent = string(existing)
		}
	}
	// File doesn't exist — new file creation is always allowed.
	// Source: FileWriteTool.ts:192-193

	// Create parent directories before the write.
	// Source: FileWriteTool.ts:254 — must stay OUTSIDE the atomic critical section.
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return ErrorOutput(fmt.Sprintf("failed to create directories: %s", err)), nil
	}

	// File history checkpoint — backup pre-edit content for undo.
	// Source: FileWriteTool.ts:255-264 — fileHistoryTrackEdit
	if fileExists && tc.FileHistory != nil {
		if err := tc.FileHistory.TrackEdit(path, oldContent); err != nil {
			// Non-fatal: log but continue. A failed backup should not block the write.
			_ = err
		}
	}

	// Normalize content to LF line endings.
	// Source: FileWriteTool.ts:305 — writeTextContent(fullFilePath, content, enc, 'LF')
	// The model sent explicit line endings in content — always write with LF.
	// Previously preserving CRLF or sampling repo line endings corrupted scripts.
	normalizedContent := strings.ReplaceAll(in.Content, "\r\n", "\n")

	if err := os.WriteFile(path, []byte(normalizedContent), 0644); err != nil {
		return ErrorOutput(fmt.Sprintf("failed to write file: %s", err)), nil
	}

	// Update ReadFileState after successful write.
	// Source: FileWriteTool.ts:332-337
	if tc.ReadFileState != nil {
		tc.ReadFileState.Record(path, normalizedContent, false)
	}

	// Produce create vs update result with verbatim TS messages.
	// Source: FileWriteTool.ts:418-432 — mapToolResultToToolResultBlockParam
	if fileExists {
		out := SuccessOutput(fmt.Sprintf("The file %s has been updated successfully.", path))
		if hunks := ComputeDiffHunks(oldContent, normalizedContent); len(hunks) > 0 {
			out.Display = DiffDisplay{FilePath: path, Hunks: hunks}
		}
		return out, nil
	}

	out := SuccessOutput(fmt.Sprintf("File created successfully at: %s", path))
	if hunks := ComputeDiffHunks("", normalizedContent); len(hunks) > 0 {
		out.Display = DiffDisplay{FilePath: path, Hunks: hunks}
	}
	return out, nil
}
