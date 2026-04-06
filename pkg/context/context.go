// Package context provides context-window analysis utilities.
// Source: utils/context.ts, utils/contextAnalysis.ts, utils/contentArray.ts,
//         utils/analyzeContext.ts, utils/contextSuggestions.ts
package context

import (
	"encoding/json"
	"math"
	"strings"

	"github.com/projectbarks/gopher-code/pkg/message"
)

// ---------------------------------------------------------------------------
// Constants — Source: utils/context.ts
// ---------------------------------------------------------------------------

const (
	// ModelContextWindowDefault is the default context window (200k tokens).
	ModelContextWindowDefault = 200_000

	// CompactMaxOutputTokens is the max output tokens for compact operations.
	CompactMaxOutputTokens = 20_000

	// CappedDefaultMaxTokens is the slot-reservation optimization cap.
	CappedDefaultMaxTokens = 8_000

	// EscalatedMaxTokens is used when the model signals complex output.
	EscalatedMaxTokens = 64_000
)

// ---------------------------------------------------------------------------
// TokenUsage + calculateContextPercentages — Source: utils/context.ts:118-144
// ---------------------------------------------------------------------------

// TokenUsage holds token counts from the API response.
type TokenUsage struct {
	InputTokens              int
	CacheCreationInputTokens int
	CacheReadInputTokens     int
}

// ContextPercentages holds the used/remaining percentage of the context window.
type ContextPercentages struct {
	Used      *int
	Remaining *int
}

// CalculateContextPercentages returns the used and remaining percentage of the
// context window, or nil values if usage is nil.
func CalculateContextPercentages(usage *TokenUsage, contextWindowSize int) ContextPercentages {
	if usage == nil {
		return ContextPercentages{}
	}

	total := usage.InputTokens + usage.CacheCreationInputTokens + usage.CacheReadInputTokens

	var usedPct int
	if contextWindowSize > 0 {
		usedPct = int(math.Round(float64(total) / float64(contextWindowSize) * 100))
	} else {
		usedPct = 100 // clamp
	}
	usedPct = clamp(usedPct, 0, 100)
	remaining := 100 - usedPct

	return ContextPercentages{Used: &usedPct, Remaining: &remaining}
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// ---------------------------------------------------------------------------
// roughTokenCount — delegates to the same heuristic as compact/microcompact
// ---------------------------------------------------------------------------

func roughTokenCount(text string) int {
	return int(math.Ceil(float64(len(text)) / 4.0))
}

// ---------------------------------------------------------------------------
// analyzeContext — Source: utils/contextAnalysis.ts
// ---------------------------------------------------------------------------

// DuplicateRead tracks a file that was read more than once.
type DuplicateRead struct {
	Count  int
	Tokens int
}

// TokenStats is the per-category token breakdown of a conversation.
type TokenStats struct {
	ToolRequests        map[string]int
	ToolResults         map[string]int
	HumanMessages       int
	AssistantMessages   int
	LocalCommandOutputs int
	Other               int
	DuplicateFileReads  map[string]DuplicateRead
	Total               int
}

// AnalyzeContext computes per-category token statistics for the given messages.
// Source: utils/contextAnalysis.ts:27-97
func AnalyzeContext(messages []message.Message) *TokenStats {
	stats := &TokenStats{
		ToolRequests:       make(map[string]int),
		ToolResults:        make(map[string]int),
		DuplicateFileReads: make(map[string]DuplicateRead),
	}
	if len(messages) == 0 {
		return stats
	}

	// Maps for correlating tool_use -> tool_result
	toolIDToName := make(map[string]string)
	readToolIDToPath := make(map[string]string)
	fileReadStats := make(map[string]struct{ count, totalTokens int })

	for _, msg := range messages {
		for _, block := range msg.Content {
			tokens := blockTokens(block)
			stats.Total += tokens

			switch block.Type {
			case message.ContentText:
				if msg.Role == message.RoleUser && strings.Contains(block.Text, "local-command-stdout") {
					stats.LocalCommandOutputs += tokens
				} else if msg.Role == message.RoleUser {
					stats.HumanMessages += tokens
				} else {
					stats.AssistantMessages += tokens
				}

			case message.ContentToolUse:
				name := block.Name
				if name == "" {
					name = "unknown"
				}
				toolIDToName[block.ID] = name
				stats.ToolRequests[name] += tokens

				// Track Read tool file paths
				if name == "Read" && len(block.Input) > 0 {
					var inp struct {
						FilePath string `json:"file_path"`
					}
					if json.Unmarshal(block.Input, &inp) == nil && inp.FilePath != "" {
						readToolIDToPath[block.ID] = inp.FilePath
					}
				}

			case message.ContentToolResult:
				name := toolIDToName[block.ToolUseID]
				if name == "" {
					name = "unknown"
				}
				stats.ToolResults[name] += tokens

				// Track file read stats for duplicate detection
				if name == "Read" {
					if path, ok := readToolIDToPath[block.ToolUseID]; ok {
						entry := fileReadStats[path]
						entry.count++
						entry.totalTokens += tokens
						fileReadStats[path] = entry
					}
				}

			default:
				// thinking, redacted_thinking, image, etc.
				stats.Other += tokens
			}
		}
	}

	// Calculate duplicate file reads
	for path, data := range fileReadStats {
		if data.count > 1 {
			avg := data.totalTokens / data.count
			stats.DuplicateFileReads[path] = DuplicateRead{
				Count:  data.count,
				Tokens: avg * (data.count - 1),
			}
		}
	}

	return stats
}

// blockTokens estimates token count for a single content block.
func blockTokens(b message.ContentBlock) int {
	switch b.Type {
	case message.ContentText:
		return roughTokenCount(b.Text)
	case message.ContentToolResult:
		return roughTokenCount(b.Content)
	case message.ContentToolUse:
		data, _ := json.Marshal(b)
		return roughTokenCount(string(data))
	case message.ContentThinking:
		return roughTokenCount(b.Thinking)
	default:
		data, _ := json.Marshal(b)
		return roughTokenCount(string(data))
	}
}

// ---------------------------------------------------------------------------
// tokenStatsToMetrics — Source: utils/contextAnalysis.ts:195-272
// ---------------------------------------------------------------------------

// TokenStatsToMetrics converts TokenStats to a flat metrics map suitable for
// analytics/logging.
func TokenStatsToMetrics(stats *TokenStats) map[string]int {
	m := map[string]int{
		"total_tokens":                stats.Total,
		"human_message_tokens":        stats.HumanMessages,
		"assistant_message_tokens":    stats.AssistantMessages,
		"local_command_output_tokens": stats.LocalCommandOutputs,
		"other_tokens":               stats.Other,
	}

	for tool, tokens := range stats.ToolRequests {
		m["tool_request_"+tool+"_tokens"] = tokens
	}
	for tool, tokens := range stats.ToolResults {
		m["tool_result_"+tool+"_tokens"] = tokens
	}

	var dupTotal int
	for _, d := range stats.DuplicateFileReads {
		dupTotal += d.Tokens
	}
	m["duplicate_read_tokens"] = dupTotal
	m["duplicate_read_file_count"] = len(stats.DuplicateFileReads)

	if stats.Total > 0 {
		m["human_message_percent"] = int(math.Round(float64(stats.HumanMessages) / float64(stats.Total) * 100))
		m["assistant_message_percent"] = int(math.Round(float64(stats.AssistantMessages) / float64(stats.Total) * 100))
		m["local_command_output_percent"] = int(math.Round(float64(stats.LocalCommandOutputs) / float64(stats.Total) * 100))
		m["duplicate_read_percent"] = int(math.Round(float64(dupTotal) / float64(stats.Total) * 100))

		var reqTotal, resTotal int
		for _, v := range stats.ToolRequests {
			reqTotal += v
		}
		for _, v := range stats.ToolResults {
			resTotal += v
		}
		m["tool_request_percent"] = int(math.Round(float64(reqTotal) / float64(stats.Total) * 100))
		m["tool_result_percent"] = int(math.Round(float64(resTotal) / float64(stats.Total) * 100))

		for tool, tokens := range stats.ToolRequests {
			m["tool_request_"+tool+"_percent"] = int(math.Round(float64(tokens) / float64(stats.Total) * 100))
		}
		for tool, tokens := range stats.ToolResults {
			m["tool_result_"+tool+"_percent"] = int(math.Round(float64(tokens) / float64(stats.Total) * 100))
		}
	}

	return m
}

// ---------------------------------------------------------------------------
// insertBlockAfterToolResults — Source: utils/contentArray.ts
// ---------------------------------------------------------------------------

// InsertBlockAfterToolResults inserts a block after the last tool_result in a
// content slice. If no tool_result exists, it inserts before the last block.
// A continuation text block is appended when the inserted block would be last.
// Returns a new slice (does not mutate the input).
func InsertBlockAfterToolResults(content []message.ContentBlock, block message.ContentBlock) []message.ContentBlock {
	result := make([]message.ContentBlock, len(content))
	copy(result, content)

	lastToolResultIdx := -1
	for i, b := range result {
		if b.Type == message.ContentToolResult {
			lastToolResultIdx = i
		}
	}

	if lastToolResultIdx >= 0 {
		insertPos := lastToolResultIdx + 1
		result = sliceInsert(result, insertPos, block)
		if insertPos == len(result)-1 {
			result = append(result, message.TextBlock("."))
		}
	} else {
		insertIdx := len(result) - 1
		if insertIdx < 0 {
			insertIdx = 0
		}
		result = sliceInsert(result, insertIdx, block)
	}

	return result
}

func sliceInsert(s []message.ContentBlock, idx int, v message.ContentBlock) []message.ContentBlock {
	s = append(s, message.ContentBlock{})
	copy(s[idx+1:], s[idx:])
	s[idx] = v
	return s
}
