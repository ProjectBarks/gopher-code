package testharness

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
)

// TestdataDir returns the absolute path to the testdata directory.
func TestdataDir() string {
	_, filename, _, _ := runtime.Caller(0)
	// internal/testharness/ -> project root -> testdata/
	return filepath.Join(filepath.Dir(filename), "..", "..", "testdata")
}

// ToolSchemaEntry represents a single tool's schema from the golden file.
type ToolSchemaEntry struct {
	Name               string                 `json:"name"`
	Description        string                 `json:"description"`
	IsReadOnly         bool                   `json:"is_read_only"`
	IsConcurrencySafe  bool                   `json:"is_concurrency_safe"`
	MaxResultSizeChars int                    `json:"max_result_size_chars"` // -1 means Infinity
	InputSchema        map[string]interface{} `json:"input_schema"`
}

// LoadToolSchemas loads tool schemas from testdata/tool_schemas.json.
func LoadToolSchemas() ([]ToolSchemaEntry, error) {
	path := filepath.Join(TestdataDir(), "tool_schemas.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var schemas []ToolSchemaEntry
	if err := json.Unmarshal(data, &schemas); err != nil {
		return nil, err
	}
	return schemas, nil
}

// QueryLoopConstants represents the operational constants from the golden file.
type QueryLoopConstants struct {
	MaxOutputTokensRecoveryLimit      int    `json:"max_output_tokens_recovery_limit"`
	MaxConsecutiveAutocompactFailures  int    `json:"max_consecutive_autocompact_failures"`
	AutocompactBufferTokens           int    `json:"autocompact_buffer_tokens"`
	WarningThresholdBufferTokens      int    `json:"warning_threshold_buffer_tokens"`
	ErrorThresholdBufferTokens        int    `json:"error_threshold_buffer_tokens"`
	ManualCompactBufferTokens         int    `json:"manual_compact_buffer_tokens"`
	MaxOutputTokensForSummary         int    `json:"max_output_tokens_for_summary"`
	EscalatedMaxTokens                int    `json:"escalated_max_tokens"`
	MaxToolUseConcurrency             int    `json:"max_tool_use_concurrency"`
	DefaultMaxOutputTokens            int    `json:"default_max_output_tokens"`
	SyntheticToolResultPlaceholder    string `json:"synthetic_tool_result_placeholder"`
}

// LoadQueryLoopConstants loads constants from testdata/query_loop_constants.json.
func LoadQueryLoopConstants() (*QueryLoopConstants, error) {
	path := filepath.Join(TestdataDir(), "query_loop_constants.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var constants QueryLoopConstants
	if err := json.Unmarshal(data, &constants); err != nil {
		return nil, err
	}
	return &constants, nil
}

// ToolBehavior represents per-tool behavioral constants from the golden file.
type ToolBehavior struct {
	Bash      BashBehavior      `json:"bash"`
	FileRead  FileReadBehavior  `json:"file_read"`
	FileWrite FileWriteBehavior `json:"file_write"`
	FileEdit  FileEditBehavior  `json:"file_edit"`
	Glob      GlobBehavior      `json:"glob"`
	Grep      GrepBehavior      `json:"grep"`
	QueryLoop QueryLoopBehavior `json:"query_loop"`
}

type BashBehavior struct {
	ToolName                string `json:"tool_name"`
	DefaultTimeoutMs        int    `json:"default_timeout_ms"`
	MaxTimeoutMs            int    `json:"max_timeout_ms"`
	MaxTimeoutAlwaysGteDefault bool `json:"max_timeout_always_gte_default"`
	MaxResultSizeChars      int    `json:"max_result_size_chars"`
}

type FileReadBehavior struct {
	ToolName                   string   `json:"tool_name"`
	DefaultMaxOutputTokens     int      `json:"default_max_output_tokens"`
	MaxOutputSizeBytes         int      `json:"max_output_size_bytes"`
	MaxResultSizeChars         int      `json:"max_result_size_chars"`
	PDFMaxPagesPerRead         int      `json:"pdf_max_pages_per_read"`
	PDFInlineThresholdPages    int      `json:"pdf_inline_threshold_pages"`
	PDFExtractSizeThresholdBytes int    `json:"pdf_extract_size_threshold_bytes"`
	BlockedPaths               []string `json:"blocked_paths"`
	IsReadOnly                 bool     `json:"is_read_only"`
	IsConcurrencySafe          bool     `json:"is_concurrency_safe"`
}

type FileWriteBehavior struct {
	ToolName           string `json:"tool_name"`
	MaxResultSizeChars int    `json:"max_result_size_chars"`
	IsReadOnly         bool   `json:"is_read_only"`
	IsConcurrencySafe  bool   `json:"is_concurrency_safe"`
}

type FileEditBehavior struct {
	ToolName               string              `json:"tool_name"`
	MaxFileSizeBytes       int64               `json:"max_file_size_bytes"`
	MaxResultSizeChars     int                 `json:"max_result_size_chars"`
	ErrorCodes             FileEditErrorCodes  `json:"error_codes"`
	RequiresFullReadBefore bool                `json:"requires_full_read_before_edit"`
	IsReadOnly             bool                `json:"is_read_only"`
	IsConcurrencySafe      bool                `json:"is_concurrency_safe"`
}

type FileEditErrorCodes struct {
	FileNotFoundWithOldString   int `json:"file_not_found_with_old_string"`
	FileNotReadYet              int `json:"file_not_read_yet"`
	FileModifiedSinceRead       int `json:"file_modified_since_read"`
	StringNotFound              int `json:"string_not_found"`
	MultipleMatchesNoReplaceAll int `json:"multiple_matches_no_replace_all"`
	FileAlreadyExistsEmptyOld   int `json:"file_already_exists_empty_old"`
	FileTooLarge                int `json:"file_too_large"`
}

type GlobBehavior struct {
	ToolName           string `json:"tool_name"`
	MaxResultsDefault  int    `json:"max_results_default"`
	TruncationMessage  string `json:"truncation_message"`
	MaxResultSizeChars int    `json:"max_result_size_chars"`
	IsReadOnly         bool   `json:"is_read_only"`
	IsConcurrencySafe  bool   `json:"is_concurrency_safe"`
}

type GrepBehavior struct {
	ToolName              string   `json:"tool_name"`
	DefaultHeadLimit      int      `json:"default_head_limit"`
	DefaultOutputMode     string   `json:"default_output_mode"`
	DefaultLineNumbers    bool     `json:"default_line_numbers"`
	MaxLineLengthChars    int      `json:"max_line_length_chars"`
	VCSDirectoriesExcluded []string `json:"vcs_directories_excluded"`
	MaxResultSizeChars    int      `json:"max_result_size_chars"`
	IsReadOnly            bool     `json:"is_read_only"`
	IsConcurrencySafe     bool     `json:"is_concurrency_safe"`
}

type QueryLoopBehavior struct {
	TurnCounterStartsAt                        int    `json:"turn_counter_starts_at"`
	MaxOutputTokensRecoveryMessage             string `json:"max_output_tokens_recovery_message"`
	ToolDetectionUsesContentBlocksNotStopReason bool   `json:"tool_detection_uses_content_blocks_not_stop_reason"`
	SystemPromptInSeparateFieldNotMessages     bool   `json:"system_prompt_in_separate_field_not_messages"`
	ToolResultsInUserRoleMessages              bool   `json:"tool_results_in_user_role_messages"`
	CappedDefaultMaxTokens                     int    `json:"capped_default_max_tokens"`
	CompactMaxOutputTokens                     int    `json:"compact_max_output_tokens"`
	FallbackDefaultMaxOutputTokens             int    `json:"fallback_default_max_output_tokens"`
}

// LoadToolBehavior loads per-tool behavioral constants from testdata/tool_behavior.json.
func LoadToolBehavior() (*ToolBehavior, error) {
	path := filepath.Join(TestdataDir(), "tool_behavior.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var behavior ToolBehavior
	if err := json.Unmarshal(data, &behavior); err != nil {
		return nil, err
	}
	return &behavior, nil
}
