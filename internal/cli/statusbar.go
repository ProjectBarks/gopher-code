package cli

import (
	"fmt"
	"strings"
	"sync"
)

// Source: components — background task pills show in status bar

// TaskPill represents a background task shown in the status bar.
type TaskPill struct {
	ID     string
	Label  string
	Status string // "running", "completed", "failed"
}

// StatusBar manages background task pills for the terminal status line.
type StatusBar struct {
	mu    sync.RWMutex
	pills map[string]*TaskPill
}

// NewStatusBar creates a new status bar.
func NewStatusBar() *StatusBar {
	return &StatusBar{pills: make(map[string]*TaskPill)}
}

// AddPill adds a background task pill to the status bar.
func (s *StatusBar) AddPill(id, label string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pills[id] = &TaskPill{ID: id, Label: label, Status: "running"}
}

// UpdatePill updates a pill's status.
func (s *StatusBar) UpdatePill(id, status string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if pill, ok := s.pills[id]; ok {
		pill.Status = status
	}
}

// RemovePill removes a pill from the status bar.
func (s *StatusBar) RemovePill(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.pills, id)
}

// Render returns the status bar as a formatted string.
func (s *StatusBar) Render() string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(s.pills) == 0 {
		return ""
	}

	var parts []string
	for _, pill := range s.pills {
		var icon string
		switch pill.Status {
		case "running":
			icon = "⟳"
		case "completed":
			icon = "✓"
		case "failed":
			icon = "✗"
		default:
			icon = "•"
		}
		parts = append(parts, fmt.Sprintf("\033[90m[%s %s]\033[0m", icon, pill.Label))
	}
	return strings.Join(parts, " ")
}

// Count returns the number of active pills.
func (s *StatusBar) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.pills)
}
