// Package services contains standalone service components.
package services

import (
	"regexp"
	"strings"
	"sync"
)

// Source: services/MagicDocs/magicDocs.ts

// MagicDocHeaderPattern matches "# MAGIC DOC: [title]" at the start of a file.
var MagicDocHeaderPattern = regexp.MustCompile(`(?im)^#\s*MAGIC\s+DOC:\s*(.+)$`)

// ItalicsPattern matches italics on the line immediately after the header.
var ItalicsPattern = regexp.MustCompile(`(?m)^[_*](.+?)[_*]\s*$`)

// MagicDocHeader holds the parsed header info.
type MagicDocHeader struct {
	Title        string
	Instructions string // optional italics line below header
}

// DetectMagicDocHeader checks if content contains a Magic Doc header.
// Returns nil if the content is not a magic doc.
// Source: magicDocs.ts:52-79
func DetectMagicDocHeader(content string) *MagicDocHeader {
	match := MagicDocHeaderPattern.FindStringSubmatch(content)
	if match == nil || len(match) < 2 {
		return nil
	}

	title := strings.TrimSpace(match[1])
	header := &MagicDocHeader{Title: title}

	// Check for italics instructions on the next line after the header.
	headerEnd := strings.Index(content, match[0]) + len(match[0])
	rest := content[headerEnd:]
	rest = strings.TrimLeft(rest, "\r\n")
	if italicsMatch := ItalicsPattern.FindStringSubmatch(rest); italicsMatch != nil {
		header.Instructions = strings.TrimSpace(italicsMatch[1])
	}

	return header
}

// MagicDocInfo tracks a detected magic doc file.
type MagicDocInfo struct {
	Path  string
	Title string
}

// MagicDocTracker tracks magic doc files discovered during the session.
// Source: magicDocs.ts:38-46
type MagicDocTracker struct {
	mu   sync.RWMutex
	docs map[string]MagicDocInfo // keyed by path
}

// NewMagicDocTracker creates a new tracker.
func NewMagicDocTracker() *MagicDocTracker {
	return &MagicDocTracker{docs: make(map[string]MagicDocInfo)}
}

// Track adds a magic doc to the tracker.
func (t *MagicDocTracker) Track(path, title string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.docs[path] = MagicDocInfo{Path: path, Title: title}
}

// Clear removes all tracked magic docs.
func (t *MagicDocTracker) Clear() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.docs = make(map[string]MagicDocInfo)
}

// All returns all tracked magic docs.
func (t *MagicDocTracker) All() []MagicDocInfo {
	t.mu.RLock()
	defer t.mu.RUnlock()
	result := make([]MagicDocInfo, 0, len(t.docs))
	for _, d := range t.docs {
		result = append(result, d)
	}
	return result
}

// Count returns the number of tracked magic docs.
func (t *MagicDocTracker) Count() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return len(t.docs)
}
