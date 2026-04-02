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

// MEMORY.md index constants.
// Source: memdir/memdir.ts:34-38
const (
	EntrypointName     = "MEMORY.md"
	MaxEntrypointLines = 200
	MaxEntrypointBytes = 25_000
)

// MemoryDir returns the path to the memory directory.
func MemoryDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".claude", "projects")
}

// MemoryIndexPath returns the path to MEMORY.md in the given memory dir.
func MemoryIndexPath(memDir string) string {
	return filepath.Join(memDir, EntrypointName)
}

// EntrypointTruncation describes the result of truncating MEMORY.md.
// Source: memdir/memdir.ts:41-47
type EntrypointTruncation struct {
	Content          string
	LineCount        int
	ByteCount        int
	WasLineTruncated bool
	WasByteTruncated bool
}

// TruncateEntrypointContent truncates MEMORY.md to line and byte caps.
// Source: memdir/memdir.ts:57-103
func TruncateEntrypointContent(raw string) EntrypointTruncation {
	trimmed := strings.TrimSpace(raw)
	lines := strings.Split(trimmed, "\n")
	lineCount := len(lines)
	byteCount := len(trimmed)

	wasLineTruncated := lineCount > MaxEntrypointLines
	wasByteTruncated := byteCount > MaxEntrypointBytes

	if !wasLineTruncated && !wasByteTruncated {
		return EntrypointTruncation{
			Content:          trimmed,
			LineCount:        lineCount,
			ByteCount:        byteCount,
			WasLineTruncated: false,
			WasByteTruncated: false,
		}
	}

	truncated := trimmed
	if wasLineTruncated {
		truncated = strings.Join(lines[:MaxEntrypointLines], "\n")
	}
	if len(truncated) > MaxEntrypointBytes {
		cutAt := strings.LastIndex(truncated[:MaxEntrypointBytes], "\n")
		if cutAt > 0 {
			truncated = truncated[:cutAt]
		} else {
			truncated = truncated[:MaxEntrypointBytes]
		}
	}

	var reason string
	switch {
	case wasByteTruncated && !wasLineTruncated:
		reason = fmt.Sprintf("%d bytes (limit: %d) — index entries are too long", byteCount, MaxEntrypointBytes)
	case wasLineTruncated && !wasByteTruncated:
		reason = fmt.Sprintf("%d lines (limit: %d)", lineCount, MaxEntrypointLines)
	default:
		reason = fmt.Sprintf("%d lines and %d bytes", lineCount, byteCount)
	}

	return EntrypointTruncation{
		Content:          truncated + "\n\n> WARNING: " + EntrypointName + " is " + reason + ". Only part of it was loaded. Keep index entries to one line under ~200 chars; move detail into topic files.",
		LineCount:        lineCount,
		ByteCount:        byteCount,
		WasLineTruncated: wasLineTruncated,
		WasByteTruncated: wasByteTruncated,
	}
}

// AppendToMemoryIndex adds a one-line entry to MEMORY.md.
func AppendToMemoryIndex(memDir, line string) error {
	indexPath := MemoryIndexPath(memDir)
	if err := os.MkdirAll(memDir, 0755); err != nil {
		return err
	}
	f, err := os.OpenFile(indexPath, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(line + "\n")
	return err
}

// ReadMemoryIndex reads MEMORY.md with truncation applied.
func ReadMemoryIndex(memDir string) (EntrypointTruncation, error) {
	data, err := os.ReadFile(MemoryIndexPath(memDir))
	if err != nil {
		if os.IsNotExist(err) {
			return EntrypointTruncation{}, nil
		}
		return EntrypointTruncation{}, err
	}
	return TruncateEntrypointContent(string(data)), nil
}
