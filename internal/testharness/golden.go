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
