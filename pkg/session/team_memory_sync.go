package session

import (
	"sync"
	"time"
)

// Source: services/teamMemorySync.ts

// TeamMemorySync manages periodic syncing of team memories.
type TeamMemorySync struct {
	mu        sync.Mutex
	teamName  string
	lastSync  time.Time
	interval  time.Duration
	dirty     bool
}

// NewTeamMemorySync creates a sync manager for a team.
func NewTeamMemorySync(teamName string, interval time.Duration) *TeamMemorySync {
	return &TeamMemorySync{
		teamName: teamName,
		interval: interval,
	}
}

// MarkDirty signals that team memories have changed and need syncing.
func (s *TeamMemorySync) MarkDirty() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.dirty = true
}

// ShouldSync returns true if memories are dirty and enough time has passed.
func (s *TeamMemorySync) ShouldSync() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.dirty && time.Since(s.lastSync) >= s.interval
}

// MarkSynced records completion of a sync.
func (s *TeamMemorySync) MarkSynced() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastSync = time.Now()
	s.dirty = false
}
