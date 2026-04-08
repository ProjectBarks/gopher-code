package memdir

import (
	"regexp"
	"strings"
)

// Source: services/extractMemories.ts

// MemoryExtraction represents a memory extracted from conversation.
type MemoryExtraction struct {
	Name        string
	Description string
	Type        string // user, feedback, project, reference
	Content     string
}

// memoryPattern matches memory save instructions in assistant text.
// Looks for patterns like "[saves user memory: ...]" or "[saves feedback memory: ...]"
var memoryPattern = regexp.MustCompile(`\[saves?\s+(user|feedback|project|reference)\s+memory:\s*(.+?)\]`)

// ExtractMemoriesFromText finds memory save instructions in assistant output.
// Source: services/extractMemories.ts
func ExtractMemoriesFromText(text string) []MemoryExtraction {
	matches := memoryPattern.FindAllStringSubmatch(text, -1)
	if len(matches) == 0 {
		return nil
	}

	var extractions []MemoryExtraction
	for _, m := range matches {
		if len(m) >= 3 {
			extractions = append(extractions, MemoryExtraction{
				Type:    strings.TrimSpace(m[1]),
				Content: strings.TrimSpace(m[2]),
			})
		}
	}
	return extractions
}
