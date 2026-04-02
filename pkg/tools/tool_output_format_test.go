package tools_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func outputFormatPath() string {
	_, filename, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(filename), "..", "..", "testdata", "tool_output_format.json")
}

type ToolOutputFormat struct {
	FileRead struct {
		MaxLinesDefault               int    `json:"max_lines_default"`
		LineNumberFormat              string `json:"line_number_format"`
		CompactLineFormat             string `json:"compact_line_format"`
		StandardLineFormat            string `json:"standard_line_format"`
		LineNumberSeparatorCompact    string `json:"line_number_separator_compact"`
		LineNumberSeparatorStandard   string `json:"line_number_separator_standard"`
		LineNumbers1Indexed           bool   `json:"line_numbers_1_indexed"`
		EmptyContentReturnsEmpty      bool   `json:"empty_content_returns_empty_string"`
	} `json:"file_read"`
	Glob struct {
		NoResultsText      string `json:"no_results_text"`
		ResultFormat        string `json:"result_format"`
		TruncationAppended  bool   `json:"truncation_appended"`
		TruncationText      string `json:"truncation_text"`
	} `json:"glob"`
	Grep struct {
		ContentMode struct {
			NoResultsText    string `json:"no_results_text"`
			PaginationSuffix string `json:"pagination_suffix"`
		} `json:"content_mode"`
		CountMode struct {
			NoResultsText string `json:"no_results_text"`
			SummaryFormat string `json:"summary_format"`
		} `json:"count_mode"`
		FilesWithMatchesMode struct {
			NoResultsText string `json:"no_results_text"`
		} `json:"files_with_matches_mode"`
	} `json:"grep"`
	Bash struct {
		MaxResultSizeChars        int  `json:"max_result_size_chars"`
		OutputPersistedWhenTooLarge bool `json:"output_persisted_when_too_large"`
		PersistedPathPattern       string `json:"persisted_path_pattern"`
	} `json:"bash"`
	FileEdit struct {
		ErrorMessages map[string]string `json:"error_messages"`
	} `json:"file_edit"`
	Common struct {
		ToolResultBlockType           string `json:"tool_result_block_type"`
		ToolResultHasToolUseID        bool   `json:"tool_result_has_tool_use_id"`
		ToolResultContentIsStringOrArray bool `json:"tool_result_content_is_string_or_array"`
		ErrorResultsHaveIsErrorTrue   bool   `json:"error_results_have_is_error_true"`
	} `json:"common"`
}

func loadOutputFormat(t *testing.T) *ToolOutputFormat {
	t.Helper()
	data, err := os.ReadFile(outputFormatPath())
	if err != nil {
		t.Fatalf("failed to load: %v", err)
	}
	var f ToolOutputFormat
	if err := json.Unmarshal(data, &f); err != nil {
		t.Fatalf("failed to parse: %v", err)
	}
	return &f
}

// TestFileReadOutputFormat validates Read tool output formatting from TS source.
// Source: file.ts:288-320, FileReadTool/prompt.ts:10
func TestFileReadOutputFormat(t *testing.T) {
	f := loadOutputFormat(t)
	fr := f.FileRead

	t.Run("max_lines_2000", func(t *testing.T) {
		// Source: FileReadTool/prompt.ts:10 — MAX_LINES_TO_READ = 2000
		if fr.MaxLinesDefault != 2000 {
			t.Errorf("expected 2000, got %d", fr.MaxLinesDefault)
		}
	})
	t.Run("cat_n_format", func(t *testing.T) {
		if fr.LineNumberFormat != "cat -n style" {
			t.Errorf("expected 'cat -n style', got %s", fr.LineNumberFormat)
		}
	})
	t.Run("compact_separator_tab", func(t *testing.T) {
		// Source: file.ts:308 — `${index + startLine}\t${line}`
		if fr.LineNumberSeparatorCompact != "\\t" {
			t.Errorf("expected tab, got %s", fr.LineNumberSeparatorCompact)
		}
	})
	t.Run("standard_separator_arrow", func(t *testing.T) {
		// Source: file.ts:315 — `${numStr.padStart(6, ' ')}→${line}`
		if fr.LineNumberSeparatorStandard != "→" {
			t.Errorf("expected →, got %s", fr.LineNumberSeparatorStandard)
		}
	})
	t.Run("1_indexed_line_numbers", func(t *testing.T) {
		if !fr.LineNumbers1Indexed {
			t.Error("line numbers must be 1-indexed")
		}
	})
	t.Run("empty_returns_empty", func(t *testing.T) {
		if !fr.EmptyContentReturnsEmpty {
			t.Error("empty content should return empty string")
		}
	})
}

// TestGlobOutputFormat validates Glob tool output formatting.
// Source: GlobTool.ts:175-200
func TestGlobOutputFormat(t *testing.T) {
	f := loadOutputFormat(t)
	g := f.Glob

	t.Run("no_results_text", func(t *testing.T) {
		if g.NoResultsText != "No files found" {
			t.Errorf("expected 'No files found', got %q", g.NoResultsText)
		}
	})
	t.Run("result_format_newline_join", func(t *testing.T) {
		if g.ResultFormat != "filenames joined by newline" {
			t.Errorf("expected 'filenames joined by newline', got %q", g.ResultFormat)
		}
	})
	t.Run("truncation_appended", func(t *testing.T) {
		if !g.TruncationAppended {
			t.Error("truncation message should be appended to results")
		}
	})
	t.Run("truncation_text_exact", func(t *testing.T) {
		expected := "(Results are truncated. Consider using a more specific path or pattern.)"
		if g.TruncationText != expected {
			t.Errorf("got %q", g.TruncationText)
		}
	})
}

// TestGrepOutputFormat validates Grep tool output formatting per mode.
// Source: GrepTool.ts:254-310
func TestGrepOutputFormat(t *testing.T) {
	f := loadOutputFormat(t)
	g := f.Grep

	t.Run("content_mode", func(t *testing.T) {
		t.Run("no_results_text", func(t *testing.T) {
			if g.ContentMode.NoResultsText != "No matches found" {
				t.Errorf("expected 'No matches found', got %q", g.ContentMode.NoResultsText)
			}
		})
		t.Run("has_pagination_suffix", func(t *testing.T) {
			if g.ContentMode.PaginationSuffix == "" {
				t.Error("should have pagination suffix format")
			}
		})
	})
	t.Run("count_mode", func(t *testing.T) {
		t.Run("no_results_text", func(t *testing.T) {
			if g.CountMode.NoResultsText != "No matches found" {
				t.Errorf("expected 'No matches found', got %q", g.CountMode.NoResultsText)
			}
		})
		t.Run("summary_format", func(t *testing.T) {
			// Source: GrepTool.ts:286 — "Found {N} total occurrence(s) across {M} file(s)."
			expected := "Found {N} total occurrence(s) across {M} file(s)."
			if g.CountMode.SummaryFormat != expected {
				t.Errorf("got %q", g.CountMode.SummaryFormat)
			}
		})
	})
	t.Run("files_with_matches_mode", func(t *testing.T) {
		t.Run("no_results_text", func(t *testing.T) {
			if g.FilesWithMatchesMode.NoResultsText != "No files found" {
				t.Errorf("expected 'No files found', got %q", g.FilesWithMatchesMode.NoResultsText)
			}
		})
	})
}

// TestBashOutputFormat validates Bash tool output constraints.
// Source: BashTool.tsx:424
func TestBashOutputFormat(t *testing.T) {
	f := loadOutputFormat(t)
	b := f.Bash

	t.Run("max_result_30k", func(t *testing.T) {
		if b.MaxResultSizeChars != 30000 {
			t.Errorf("expected 30000, got %d", b.MaxResultSizeChars)
		}
	})
	t.Run("persisted_when_too_large", func(t *testing.T) {
		if !b.OutputPersistedWhenTooLarge {
			t.Error("large outputs must be persisted to disk")
		}
	})
	t.Run("persisted_path_in_claude_dir", func(t *testing.T) {
		if b.PersistedPathPattern != "~/.claude/tool-results/" {
			t.Errorf("expected ~/.claude/tool-results/, got %s", b.PersistedPathPattern)
		}
	})
}

// TestFileEditErrorMessages validates exact error messages from TS source.
// Source: FileEditTool.ts:240-343
func TestFileEditErrorMessages(t *testing.T) {
	f := loadOutputFormat(t)
	msgs := f.FileEdit.ErrorMessages

	expectedKeys := []string{"code_3", "code_4_pattern", "code_6", "code_7", "code_8_pattern", "code_9_pattern"}
	for _, key := range expectedKeys {
		key := key
		t.Run(fmt.Sprintf("has_%s", key), func(t *testing.T) {
			val, ok := msgs[key]
			if !ok {
				t.Errorf("missing error message for %s", key)
			}
			if val == "" {
				t.Errorf("empty error message for %s", key)
			}
		})
	}

	t.Run("code_3_exact", func(t *testing.T) {
		// Source: FileEditTool.ts:253-257
		expected := "Cannot create new file - file already exists."
		if msgs["code_3"] != expected {
			t.Errorf("got %q", msgs["code_3"])
		}
	})
	t.Run("code_6_exact", func(t *testing.T) {
		// Source: FileEditTool.ts:281
		expected := "File has not been read yet. Read it first before writing to it."
		if msgs["code_6"] != expected {
			t.Errorf("got %q", msgs["code_6"])
		}
	})
	t.Run("code_7_exact", func(t *testing.T) {
		// Source: FileEditTool.ts:306
		expected := "File has been modified since read, either by the user or by a linter. Read it again before attempting to write it."
		if msgs["code_7"] != expected {
			t.Errorf("got %q", msgs["code_7"])
		}
	})
}

// TestCommonOutputRules validates output rules shared across all tools.
// Source: Tool.ts mapToolResultToToolResultBlockParam
func TestCommonOutputRules(t *testing.T) {
	f := loadOutputFormat(t)
	c := f.Common

	t.Run("block_type_tool_result", func(t *testing.T) {
		if c.ToolResultBlockType != "tool_result" {
			t.Errorf("expected tool_result, got %s", c.ToolResultBlockType)
		}
	})
	t.Run("has_tool_use_id", func(t *testing.T) {
		if !c.ToolResultHasToolUseID {
			t.Error("tool_result must have tool_use_id")
		}
	})
	t.Run("content_string_or_array", func(t *testing.T) {
		if !c.ToolResultContentIsStringOrArray {
			t.Error("content can be string or array of content blocks")
		}
	})
	t.Run("errors_have_is_error", func(t *testing.T) {
		if !c.ErrorResultsHaveIsErrorTrue {
			t.Error("error results must set is_error: true")
		}
	})
}
