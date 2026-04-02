package tools

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/projectbarks/gopher-code/pkg/permissions"
)

// HookRunner is the interface for pre/post tool execution hooks.
// This is defined here (rather than importing pkg/hooks) to avoid import cycles.
type HookRunner interface {
	RunForOrchestrator(ctx context.Context, hookType string, toolName string, toolInput json.RawMessage) (blocked bool, message string, err error)
}

// ReadFileEntry tracks when a file was last read by the agent.
// Used by FileEdit's staleness guard to detect concurrent modifications.
// Source: FileEditTool.ts:275-311
type ReadFileEntry struct {
	Timestamp     time.Time
	Content       string // content at time of read (for full reads)
	IsPartialView bool   // true if read was offset/limit
}

// ReadFileState is a thread-safe map tracking file read timestamps.
// Source: FileEditTool.ts:275 — toolUseContext.readFileState
type ReadFileState struct {
	mu      sync.RWMutex
	entries map[string]*ReadFileEntry
}

// NewReadFileState creates a new empty read file state tracker.
func NewReadFileState() *ReadFileState {
	return &ReadFileState{entries: make(map[string]*ReadFileEntry)}
}

// Record stores a read event for a file path.
func (s *ReadFileState) Record(path string, content string, isPartial bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries[path] = &ReadFileEntry{
		Timestamp:     time.Now(),
		Content:       content,
		IsPartialView: isPartial,
	}
}

// Get returns the read entry for a file path, or nil if never read.
func (s *ReadFileState) Get(path string) *ReadFileEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.entries[path]
}

// ToolContext provides context for tool execution.
type ToolContext struct {
	CWD           string
	Permissions   permissions.PermissionPolicy
	SessionID     string
	Hooks         HookRunner     // optional hook runner for pre/post tool hooks
	ReadFileState *ReadFileState // tracks file read timestamps for staleness guard
}
