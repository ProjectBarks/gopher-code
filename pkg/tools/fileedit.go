package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

// FileEdit error codes matching TS FileEditTool.ts.
// Source: FileEditTool.ts:146-341
const (
	EditErrSameStrings    = 1  // old_string == new_string
	EditErrDeniedByPerms  = 2  // path denied by permission settings
	EditErrFileExists     = 3  // file exists, old_string empty (create attempt on non-empty)
	EditErrFileNotFound   = 4  // file does not exist
	EditErrNotebookFile   = 5  // .ipynb — use NotebookEdit
	EditErrNotReadYet     = 6  // file not read yet
	EditErrModifiedSince  = 7  // file modified since last read
	EditErrStringNotFound = 8  // old_string not found in file
	EditErrMultipleMatch  = 9  // multiple matches, replace_all=false
	EditErrFileTooLarge   = 10 // file too large
)

// MaxEditFileSize is the maximum file size for editing (1 GiB).
// Source: FileEditTool.ts:79-84 — V8/Bun string limit ~2^30 chars
const MaxEditFileSize = 1024 * 1024 * 1024

// FILE_NOT_FOUND_CWD_NOTE is the standard note appended to file-not-found errors.
// Source: utils/file.ts — FILE_NOT_FOUND_CWD_NOTE
const fileNotFoundCWDNote = "Please verify the path and try again. Current working directory:"

// FileEditTool performs string replacements in files.
type FileEditTool struct{}

type fileEditInput struct {
	FilePath   string `json:"file_path"`
	OldString  string `json:"old_string"`
	NewString  string `json:"new_string"`
	ReplaceAll bool   `json:"replace_all,omitempty"`
}

func (f *FileEditTool) Name() string { return "Edit" }

// Description returns the short description used for tool listing.
// Source: FileEditTool.ts:92 — 'A tool for editing files'
func (f *FileEditTool) Description() string { return "A tool for editing files" }

func (f *FileEditTool) IsReadOnly() bool { return false }

// SearchHint returns the search hint for tool discovery.
// Source: FileEditTool.ts:88
func (f *FileEditTool) SearchHint() string { return "modify file contents in place" }

// MaxResultSizeChars returns the max result size (100_000).
// Source: FileEditTool.ts:89
func (f *FileEditTool) MaxResultSizeChars() int { return 100_000 }

// Prompt returns the full system prompt for the edit tool.
// Source: FileEditTool/prompt.ts — getEditToolDescription()
func (f *FileEditTool) Prompt() string {
	return getEditToolDescription()
}

// getEditToolDescription builds the full edit tool prompt.
// Source: FileEditTool/prompt.ts:8-28
func getEditToolDescription() string {
	// Default to compact line prefix format.
	// Source: prompt.ts:13-14
	prefixFormat := "line number + tab"

	return fmt.Sprintf(`Performs exact string replacements in files.

Usage:
- You must use your `+"`Read`"+` tool at least once in the conversation before editing. This tool will error if you attempt an edit without reading the file.
- When editing text from Read tool output, ensure you preserve the exact indentation (tabs/spaces) as it appears AFTER the line number prefix. The line number prefix format is: %s. Everything after that is the actual file content to match. Never include any part of the line number prefix in the old_string or new_string.
- ALWAYS prefer editing existing files in the codebase. NEVER write new files unless explicitly required.
- Only use emojis if the user explicitly requests it. Avoid adding emojis to files unless asked.
- The edit will FAIL if `+"`old_string`"+` is not unique in the file. Either provide a larger string with more surrounding context to make it unique or use `+"`replace_all`"+` to change every instance of `+"`old_string`"+`.
- Use `+"`replace_all`"+` for replacing and renaming strings across the file. This parameter is useful if you want to rename a variable for instance.`, prefixFormat)
}

// Source: FileEditTool.ts inputSchema
func (f *FileEditTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"file_path": {"type": "string", "description": "The absolute path to the file to modify"},
			"old_string": {"type": "string", "description": "The text to replace"},
			"new_string": {"type": "string", "description": "The text to replace it with (must be different from old_string)"},
			"replace_all": {"type": "boolean", "description": "Replace all occurrences of old_string (default false)", "default": false}
		},
		"required": ["file_path", "old_string", "new_string"],
		"additionalProperties": false
	}`)
}

// Source: FileEditTool.ts:137-355
func (f *FileEditTool) Execute(_ context.Context, tc *ToolContext, input json.RawMessage) (*ToolOutput, error) {
	var in fileEditInput
	if err := json.Unmarshal(input, &in); err != nil {
		return ErrorOutput(fmt.Sprintf("invalid input: %s", err)), nil
	}
	if in.FilePath == "" {
		return ErrorOutput("file_path is required"), nil
	}

	// Error code 1: old_string == new_string
	// Source: FileEditTool.ts:148-155
	if in.OldString == in.NewString {
		return ErrorOutput("No changes to make: old_string and new_string are exactly the same."), nil
	}

	path := in.FilePath
	if !filepath.IsAbs(path) {
		path = filepath.Join(tc.CWD, path)
	}

	// Error code 10: File too large
	// Source: FileEditTool.ts:186-195
	if info, err := os.Stat(path); err == nil {
		if info.Size() > MaxEditFileSize {
			return ErrorOutput(fmt.Sprintf("File is too large to edit (%d bytes). Maximum editable file size is %d bytes.", info.Size(), MaxEditFileSize)), nil
		}
	}

	// Read file with encoding detection
	// Source: FileEditTool.ts:202-221
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// Error code 4: File does not exist
			// Source: FileEditTool.ts:224-245
			if in.OldString == "" {
				// Empty old_string on nonexistent file = new file creation
				// Source: FileEditTool.ts:226-227
			} else {
				return ErrorOutput(fmt.Sprintf("File does not exist. %s %s.", fileNotFoundCWDNote, tc.CWD)), nil
			}
		} else {
			return ErrorOutput(fmt.Sprintf("failed to read file: %s", err)), nil
		}
	}

	// Detect UTF-16LE BOM and decode; normalize \r\n to \n
	// Source: FileEditTool.ts:208-214
	content := decodeFileContent(data)

	// File exists but old_string is empty — creating new content
	// Source: FileEditTool.ts:249-263
	if in.OldString == "" {
		if os.IsNotExist(err) {
			// Create new file
			dir := filepath.Dir(path)
			if err := os.MkdirAll(dir, 0755); err != nil {
				return ErrorOutput(fmt.Sprintf("failed to create directory: %s", err)), nil
			}
			if err := os.WriteFile(path, []byte(in.NewString), 0644); err != nil {
				return ErrorOutput(fmt.Sprintf("failed to write file: %s", err)), nil
			}
			// Update read file state after successful write
			if tc.ReadFileState != nil {
				tc.ReadFileState.Record(path, in.NewString, false)
			}
			out := SuccessOutput(fmt.Sprintf("Created %s", path))
			out.Display = DiffDisplay{FilePath: path, Hunks: ComputeDiffHunks("", in.NewString)}
			return out, nil
		}
		// Error code 3: file exists and has content
		if strings.TrimSpace(content) != "" {
			return ErrorOutput("Cannot create new file - file already exists."), nil
		}
		// Empty file — replace with new content
		if err := os.WriteFile(path, []byte(in.NewString), 0644); err != nil {
			return ErrorOutput(fmt.Sprintf("failed to write file: %s", err)), nil
		}
		// Update read file state after successful write
		if tc.ReadFileState != nil {
			tc.ReadFileState.Record(path, in.NewString, false)
		}
		out := SuccessOutput(fmt.Sprintf("Edited %s", path))
		out.Display = DiffDisplay{FilePath: path, Hunks: ComputeDiffHunks(content, in.NewString)}
		return out, nil
	}

	// Error code 5: .ipynb file
	// Source: FileEditTool.ts:266-273
	if strings.HasSuffix(path, ".ipynb") {
		return ErrorOutput("File is a Jupyter Notebook. Use the NotebookEdit tool to edit this file."), nil
	}

	// Error code 6: File not read yet (staleness guard)
	// Source: FileEditTool.ts:275-287
	if tc.ReadFileState != nil {
		entry := tc.ReadFileState.Get(path)
		if entry == nil || entry.IsPartialView {
			return ErrorOutput("File has not been read yet. Read it first before writing to it."), nil
		}

		// Error code 7: File modified since last read
		// Source: FileEditTool.ts:290-311
		if info, statErr := os.Stat(path); statErr == nil {
			if info.ModTime().After(entry.Timestamp) {
				// Content comparison fallback for full reads
				// Source: FileEditTool.ts:298-300
				if content != entry.Content {
					return ErrorOutput("File has been modified since read, either by the user or by a linter. Read it again before attempting to write it."), nil
				}
			}
		}
	}

	// Try exact match first; fall back to quote-normalized match.
	// Source: FileEditTool/utils.ts:73-93 — findActualString()
	oldString := in.OldString
	count := strings.Count(content, oldString)
	if count == 0 {
		// Try with normalized quotes (curly → straight)
		actualOld := FindActualString(content, oldString)
		if actualOld != "" {
			oldString = actualOld
			count = strings.Count(content, oldString)
		}
	}
	// Error code 8: old_string not found
	// Source: FileEditTool.ts:316-327
	if count == 0 {
		return ErrorOutput(fmt.Sprintf("String to replace not found in file.\nString: %s", in.OldString)), nil
	}

	// Error code 9: multiple matches without replace_all
	// Source: FileEditTool.ts:332-343
	if count > 1 && !in.ReplaceAll {
		return ErrorOutput(fmt.Sprintf(
			"Found %d matches of the string to replace, but replace_all is false. To replace all occurrences, set replace_all to true. To replace only one occurrence, please provide more context to uniquely identify the instance.\nString: %s",
			count, in.OldString,
		)), nil
	}

	// Preserve curly quotes in new_string when the file uses them
	// Source: FileEditTool.ts:475-479
	actualNewString := PreserveQuoteStyle(in.OldString, oldString, in.NewString)

	// Perform replacement using applyEditToFile logic.
	// Source: FileEditTool/utils.ts:206-228 — special newline handling on deletion
	newContent := ApplyEditToFile(content, oldString, actualNewString, in.ReplaceAll)

	info, statErr := os.Stat(path)
	mode := os.FileMode(0644)
	if statErr == nil {
		mode = info.Mode()
	}

	if err := os.WriteFile(path, []byte(newContent), mode); err != nil {
		return ErrorOutput(fmt.Sprintf("failed to write file: %s", err)), nil
	}

	// Update read file state after successful write
	if tc.ReadFileState != nil {
		tc.ReadFileState.Record(path, newContent, false)
	}

	// Source: FileEditTool.ts:575-593 — mapToolResultToToolResultBlockParam
	var msg string
	if in.ReplaceAll {
		msg = fmt.Sprintf("The file %s has been updated. All occurrences were successfully replaced.", path)
	} else {
		msg = fmt.Sprintf("The file %s has been updated successfully.", path)
	}

	out := SuccessOutput(msg)
	out.Display = DiffDisplay{FilePath: path, Hunks: ComputeDiffHunks(content, newContent)}
	return out, nil
}

// decodeFileContent handles UTF-16LE BOM detection and \r\n normalization.
// Source: FileEditTool.ts:208-214
func decodeFileContent(data []byte) string {
	if len(data) >= 2 && data[0] == 0xFF && data[1] == 0xFE {
		// UTF-16LE BOM — decode pairs of bytes
		runes := make([]rune, 0, len(data)/2)
		for i := 2; i+1 < len(data); i += 2 {
			runes = append(runes, rune(data[i])|rune(data[i+1])<<8)
		}
		return strings.ReplaceAll(string(runes), "\r\n", "\n")
	}
	return strings.ReplaceAll(string(data), "\r\n", "\n")
}

// normalizeQuotes converts curly quotes to straight quotes.
// Source: FileEditTool/utils.ts:31-37
func NormalizeQuotes(s string) string {
	s = strings.ReplaceAll(s, "\u2018", "'")  // LEFT SINGLE CURLY QUOTE
	s = strings.ReplaceAll(s, "\u2019", "'")  // RIGHT SINGLE CURLY QUOTE
	s = strings.ReplaceAll(s, "\u201C", "\"") // LEFT DOUBLE CURLY QUOTE
	s = strings.ReplaceAll(s, "\u201D", "\"") // RIGHT DOUBLE CURLY QUOTE
	return s
}

// Curly quote constants exported for use in tests and other tools.
// Source: FileEditTool/utils.ts:21-24
const (
	LeftSingleCurlyQuote  = "\u2018"
	RightSingleCurlyQuote = "\u2019"
	LeftDoubleCurlyQuote  = "\u201C"
	RightDoubleCurlyQuote = "\u201D"
)

// StripTrailingWhitespace removes trailing whitespace from each line
// while preserving line endings.
// Source: FileEditTool/utils.ts:44-64
func StripTrailingWhitespace(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	lineStart := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' || s[i] == '\r' {
			// Found a line ending — trim trailing whitespace from the line
			line := s[lineStart:i]
			b.WriteString(strings.TrimRightFunc(line, unicode.IsSpace))
			// Write the line ending character(s)
			if s[i] == '\r' && i+1 < len(s) && s[i+1] == '\n' {
				b.WriteString("\r\n")
				i++ // skip the \n in \r\n
			} else {
				b.WriteByte(s[i])
			}
			lineStart = i + 1
		}
	}
	// Last line (no trailing newline)
	if lineStart < len(s) {
		line := s[lineStart:]
		b.WriteString(strings.TrimRightFunc(line, unicode.IsSpace))
	}
	return b.String()
}

// findActualString finds the actual string in file content that matches via
// quote normalization. Returns the original file substring if found.
// Source: FileEditTool/utils.ts:73-93
func FindActualString(fileContent, searchString string) string {
	// First try exact match
	if strings.Contains(fileContent, searchString) {
		return searchString
	}
	// Try with normalized quotes. normalizeQuotes replaces each curly quote
	// rune with a straight quote rune (1-to-1 rune mapping), so rune positions
	// are preserved between original and normalized. Use rune indexing to map
	// back from normalized to original.
	normalizedSearch := NormalizeQuotes(searchString)
	normalizedFile := NormalizeQuotes(fileContent)
	byteIdx := strings.Index(normalizedFile, normalizedSearch)
	if byteIdx == -1 {
		return ""
	}
	// Convert byte offset in normalizedFile to rune offset
	runeStart := len([]rune(normalizedFile[:byteIdx]))
	runeLen := len([]rune(normalizedSearch))
	// Extract from original file content by rune offset
	fileRunes := []rune(fileContent)
	if runeStart+runeLen > len(fileRunes) {
		return ""
	}
	return string(fileRunes[runeStart : runeStart+runeLen])
}

// preserveQuoteStyle re-applies curly quote style from the file to new_string
// when the match was found via quote normalization.
// Source: FileEditTool/utils.ts:104-136
func PreserveQuoteStyle(oldString, actualOldString, newString string) string {
	// If they're the same, no normalization happened
	if oldString == actualOldString {
		return newString
	}

	// Detect which curly quote types were in the file
	hasDouble := strings.ContainsAny(actualOldString, LeftDoubleCurlyQuote+RightDoubleCurlyQuote)
	hasSingle := strings.ContainsAny(actualOldString, LeftSingleCurlyQuote+RightSingleCurlyQuote)

	if !hasDouble && !hasSingle {
		return newString
	}

	result := newString
	if hasDouble {
		result = applyCurlyDoubleQuotes(result)
	}
	if hasSingle {
		result = applyCurlySingleQuotes(result)
	}
	return result
}

// isOpeningContext checks if a quote at the given position in a rune slice
// is in an opening context (after whitespace, start of string, or opening punct).
// Source: FileEditTool/utils.ts:138-154
func isOpeningContext(chars []rune, index int) bool {
	if index == 0 {
		return true
	}
	prev := chars[index-1]
	return prev == ' ' || prev == '\t' || prev == '\n' || prev == '\r' ||
		prev == '(' || prev == '[' || prev == '{' ||
		prev == '\u2014' || // em dash
		prev == '\u2013' // en dash
}

// applyCurlyDoubleQuotes replaces straight double quotes with curly variants.
// Source: FileEditTool/utils.ts:156-171
func applyCurlyDoubleQuotes(s string) string {
	chars := []rune(s)
	result := make([]rune, 0, len(chars))
	for i, ch := range chars {
		if ch == '"' {
			if isOpeningContext(chars, i) {
				result = append(result, '\u201C') // LEFT DOUBLE
			} else {
				result = append(result, '\u201D') // RIGHT DOUBLE
			}
		} else {
			result = append(result, ch)
		}
	}
	return string(result)
}

// applyCurlySingleQuotes replaces straight single quotes with curly variants.
// Preserves apostrophes in contractions (letter-quote-letter).
// Source: FileEditTool/utils.ts:173-199
func applyCurlySingleQuotes(s string) string {
	chars := []rune(s)
	result := make([]rune, 0, len(chars))
	for i, ch := range chars {
		if ch == '\'' {
			// Don't convert apostrophes in contractions
			prevIsLetter := i > 0 && unicode.IsLetter(chars[i-1])
			nextIsLetter := i < len(chars)-1 && unicode.IsLetter(chars[i+1])
			if prevIsLetter && nextIsLetter {
				result = append(result, '\u2019') // RIGHT SINGLE (apostrophe)
			} else if isOpeningContext(chars, i) {
				result = append(result, '\u2018') // LEFT SINGLE
			} else {
				result = append(result, '\u2019') // RIGHT SINGLE
			}
		} else {
			result = append(result, ch)
		}
	}
	return string(result)
}

// applyEditToFile applies a string replacement with special deletion handling.
// When new_string is empty (deletion), also strips the trailing newline after
// old_string to avoid leaving blank lines.
// Source: FileEditTool/utils.ts:206-228
func ApplyEditToFile(content, oldString, newString string, replaceAll bool) string {
	if newString != "" {
		if replaceAll {
			return strings.ReplaceAll(content, oldString, newString)
		}
		return strings.Replace(content, oldString, newString, 1)
	}
	// Deletion: strip trailing newline if old_string doesn't end with \n
	// but appears followed by \n in the file
	stripTrailingNewline := !strings.HasSuffix(oldString, "\n") &&
		strings.Contains(content, oldString+"\n")
	target := oldString
	if stripTrailingNewline {
		target = oldString + "\n"
	}
	if replaceAll {
		return strings.ReplaceAll(content, target, newString)
	}
	return strings.Replace(content, target, newString, 1)
}
