package context

import (
	"fmt"
	"path/filepath"
	"sort"
)

// Source: utils/contextSuggestions.ts

// SuggestionSeverity indicates the urgency of a context suggestion.
type SuggestionSeverity string

const (
	SeverityInfo    SuggestionSeverity = "info"
	SeverityWarning SuggestionSeverity = "warning"
)

// ContextSuggestion is an actionable recommendation about context usage.
type ContextSuggestion struct {
	Severity      SuggestionSeverity
	Title         string
	Detail        string
	SavingsTokens int // estimated tokens that could be saved
}

// Thresholds — Source: utils/contextSuggestions.ts:22-27
const (
	largeToolResultPercent = 15
	largeToolResultTokens  = 10_000
	nearCapacityPercent    = 80
	memoryHighPercent      = 5
	memoryHighTokens       = 5_000
)

// MemoryFile describes a memory file loaded into context.
type MemoryFile struct {
	Path   string
	Type   string
	Tokens int
}

// ToolCallBreakdown is per-tool token usage.
type ToolCallBreakdown struct {
	Name         string
	CallTokens   int
	ResultTokens int
}

// MessageBreakdown is the per-category message token breakdown.
type MessageBreakdown struct {
	ToolCallsByType []ToolCallBreakdown
}

// ContextData holds all the data needed to generate context suggestions.
// Source: utils/analyzeContext.ts:190-232
type ContextData struct {
	Percentage           int
	RawMaxTokens         int
	IsAutoCompactEnabled bool
	MemoryFiles          []MemoryFile
	MessageBreakdown     *MessageBreakdown
}

// GenerateContextSuggestions produces actionable suggestions about context usage.
// Source: utils/contextSuggestions.ts:32-51
func GenerateContextSuggestions(data ContextData) []ContextSuggestion {
	var suggestions []ContextSuggestion

	checkNearCapacity(data, &suggestions)
	checkLargeToolResults(data, &suggestions)
	checkMemoryBloat(data, &suggestions)
	checkAutoCompactDisabled(data, &suggestions)

	// Sort: warnings first, then by savings descending
	sort.Slice(suggestions, func(i, j int) bool {
		a, b := suggestions[i], suggestions[j]
		if a.Severity != b.Severity {
			return a.Severity == SeverityWarning
		}
		return a.SavingsTokens > b.SavingsTokens
	})

	return suggestions
}

func checkNearCapacity(data ContextData, out *[]ContextSuggestion) {
	if data.Percentage < nearCapacityPercent {
		return
	}
	detail := "Autocompact will trigger soon, which discards older messages. Use /compact now to control what gets kept."
	if !data.IsAutoCompactEnabled {
		detail = "Autocompact is disabled. Use /compact to free space, or enable autocompact in /config."
	}
	*out = append(*out, ContextSuggestion{
		Severity: SeverityWarning,
		Title:    fmt.Sprintf("Context is %d%% full", data.Percentage),
		Detail:   detail,
	})
}

func checkLargeToolResults(data ContextData, out *[]ContextSuggestion) {
	if data.MessageBreakdown == nil {
		return
	}
	for _, tool := range data.MessageBreakdown.ToolCallsByType {
		total := tool.CallTokens + tool.ResultTokens
		pct := float64(total) / float64(data.RawMaxTokens) * 100
		if pct < largeToolResultPercent || total < largeToolResultTokens {
			continue
		}
		if s := largeToolSuggestion(tool.Name, total, pct); s != nil {
			*out = append(*out, *s)
		}
	}
}

func largeToolSuggestion(name string, tokens int, pct float64) *ContextSuggestion {
	tokenStr := formatTokens(tokens)
	pctStr := fmt.Sprintf("%.0f", pct)

	switch name {
	case "Bash":
		return &ContextSuggestion{
			Severity:      SeverityWarning,
			Title:         fmt.Sprintf("Bash results using %s tokens (%s%%)", tokenStr, pctStr),
			Detail:        "Pipe output through head, tail, or grep to reduce result size. Avoid cat on large files — use Read with offset/limit instead.",
			SavingsTokens: tokens / 2,
		}
	case "Read":
		return &ContextSuggestion{
			Severity:      SeverityInfo,
			Title:         fmt.Sprintf("Read results using %s tokens (%s%%)", tokenStr, pctStr),
			Detail:        "Use offset and limit parameters to read only the sections you need. Avoid re-reading entire files when you only need a few lines.",
			SavingsTokens: tokens * 3 / 10,
		}
	case "Grep":
		return &ContextSuggestion{
			Severity:      SeverityInfo,
			Title:         fmt.Sprintf("Grep results using %s tokens (%s%%)", tokenStr, pctStr),
			Detail:        "Add more specific patterns or use the glob or type parameter to narrow file types. Consider Glob for file discovery instead of Grep.",
			SavingsTokens: tokens * 3 / 10,
		}
	case "WebFetch":
		return &ContextSuggestion{
			Severity:      SeverityInfo,
			Title:         fmt.Sprintf("WebFetch results using %s tokens (%s%%)", tokenStr, pctStr),
			Detail:        "Web page content can be very large. Consider extracting only the specific information needed.",
			SavingsTokens: tokens * 4 / 10,
		}
	default:
		if pct >= 20 {
			return &ContextSuggestion{
				Severity:      SeverityInfo,
				Title:         fmt.Sprintf("%s using %s tokens (%s%%)", name, tokenStr, pctStr),
				Detail:        "This tool is consuming a significant portion of context.",
				SavingsTokens: tokens / 5,
			}
		}
		return nil
	}
}

func checkMemoryBloat(data ContextData, out *[]ContextSuggestion) {
	var totalTokens int
	for _, f := range data.MemoryFiles {
		totalTokens += f.Tokens
	}
	pct := float64(totalTokens) / float64(data.RawMaxTokens) * 100
	if pct < memoryHighPercent || totalTokens < memoryHighTokens {
		return
	}

	// Show up to 3 largest files
	sorted := make([]MemoryFile, len(data.MemoryFiles))
	copy(sorted, data.MemoryFiles)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Tokens > sorted[j].Tokens })
	if len(sorted) > 3 {
		sorted = sorted[:3]
	}
	var parts []string
	for _, f := range sorted {
		parts = append(parts, fmt.Sprintf("%s (%s)", filepath.Base(f.Path), formatTokens(f.Tokens)))
	}
	largest := ""
	for i, p := range parts {
		if i > 0 {
			largest += ", "
		}
		largest += p
	}

	*out = append(*out, ContextSuggestion{
		Severity:      SeverityInfo,
		Title:         fmt.Sprintf("Memory files using %s tokens (%.0f%%)", formatTokens(totalTokens), pct),
		Detail:        fmt.Sprintf("Largest: %s. Use /memory to review and prune stale entries.", largest),
		SavingsTokens: totalTokens * 3 / 10,
	})
}

func checkAutoCompactDisabled(data ContextData, out *[]ContextSuggestion) {
	if data.IsAutoCompactEnabled || data.Percentage < 50 || data.Percentage >= nearCapacityPercent {
		return
	}
	*out = append(*out, ContextSuggestion{
		Severity: SeverityInfo,
		Title:    "Autocompact is disabled",
		Detail:   "Without autocompact, you will hit context limits and lose the conversation. Enable it in /config or use /compact manually.",
	})
}

// formatTokens renders a token count in human-readable form (e.g. "12.5k").
func formatTokens(n int) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}
	k := float64(n) / 1000
	if k == float64(int(k)) {
		return fmt.Sprintf("%dk", int(k))
	}
	return fmt.Sprintf("%.1fk", k)
}
