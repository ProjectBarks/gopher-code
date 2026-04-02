package session

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Source: memdir/memoryTypes.ts

// MemoryType represents the taxonomy of memory types.
// Source: memdir/memoryTypes.ts:14-19
type MemoryType string

const (
	MemoryTypeUser      MemoryType = "user"
	MemoryTypeFeedback  MemoryType = "feedback"
	MemoryTypeProject   MemoryType = "project"
	MemoryTypeReference MemoryType = "reference"
)

// ValidMemoryTypes is the list of valid memory types.
// Source: memdir/memoryTypes.ts:14-19
var ValidMemoryTypes = []MemoryType{
	MemoryTypeUser,
	MemoryTypeFeedback,
	MemoryTypeProject,
	MemoryTypeReference,
}

// ParseMemoryType parses a raw string into a MemoryType.
// Returns empty string for invalid or missing values.
// Source: memdir/memoryTypes.ts:28-31
func ParseMemoryType(raw string) MemoryType {
	for _, t := range ValidMemoryTypes {
		if string(t) == raw {
			return t
		}
	}
	return ""
}

// MemoryEntry represents a memory stored as Markdown with YAML frontmatter.
// Source: memdir/memoryTypes.ts:261-271
type MemoryEntry struct {
	Name        string     `yaml:"name"`
	Description string     `yaml:"description"`
	Type        MemoryType `yaml:"type"`
	Body        string     `yaml:"-"` // Content below the frontmatter
	FilePath    string     `yaml:"-"` // Path on disk
}

// ParseMemoryFile parses a Markdown file with YAML frontmatter into a MemoryEntry.
// Source: memdir/memoryTypes.ts:261-271
func ParseMemoryFile(content string) (*MemoryEntry, error) {
	entry := &MemoryEntry{}

	// Check for frontmatter (---\n...\n---\n)
	if !strings.HasPrefix(content, "---\n") {
		entry.Body = content
		return entry, nil
	}

	// Find closing ---
	rest := content[4:] // skip opening ---\n
	endIdx := strings.Index(rest, "\n---\n")
	if endIdx == -1 {
		// Try trailing --- without newline after
		endIdx = strings.Index(rest, "\n---")
		if endIdx == -1 {
			entry.Body = content
			return entry, nil
		}
	}

	frontmatter := rest[:endIdx]
	entry.Body = strings.TrimLeft(rest[endIdx+4:], "\n") // skip \n---\n

	// Simple YAML parsing (key: value per line)
	for _, line := range strings.Split(frontmatter, "\n") {
		colonIdx := strings.Index(line, ":")
		if colonIdx == -1 {
			continue
		}
		key := strings.TrimSpace(line[:colonIdx])
		value := strings.TrimSpace(line[colonIdx+1:])

		switch key {
		case "name":
			entry.Name = value
		case "description":
			entry.Description = value
		case "type":
			entry.Type = ParseMemoryType(value)
		}
	}

	return entry, nil
}

// FormatMemoryFile formats a MemoryEntry as Markdown with YAML frontmatter.
// Source: memdir/memoryTypes.ts:261-271
func FormatMemoryFile(entry *MemoryEntry) string {
	var sb strings.Builder
	sb.WriteString("---\n")
	sb.WriteString(fmt.Sprintf("name: %s\n", entry.Name))
	sb.WriteString(fmt.Sprintf("description: %s\n", entry.Description))
	sb.WriteString(fmt.Sprintf("type: %s\n", entry.Type))
	sb.WriteString("---\n\n")
	sb.WriteString(entry.Body)
	return sb.String()
}

// MemoryDir returns the path to the memory directory.
func MemoryDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".claude", "projects")
}

// MemoryIndexPath returns the path to MEMORY.md in the given memory dir.
func MemoryIndexPath(memDir string) string {
	return filepath.Join(memDir, "MEMORY.md")
}
