package services

import (
	"sync"
	"time"
)

// Source: services/settingsSync.ts

// SettingsSync periodically checks for remote settings changes.
type SettingsSync struct {
	mu          sync.Mutex
	lastSync    time.Time
	interval    time.Duration
	enabled     bool
	onUpdate    func() // called when settings change
}

// NewSettingsSync creates a settings sync manager.
func NewSettingsSync(interval time.Duration) *SettingsSync {
	return &SettingsSync{
		interval: interval,
		enabled:  true,
	}
}

// SetEnabled enables or disables sync.
func (s *SettingsSync) SetEnabled(enabled bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.enabled = enabled
}

// OnUpdate registers a callback for settings changes.
func (s *SettingsSync) OnUpdate(fn func()) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onUpdate = fn
}

// ShouldSync returns true if enough time has passed since the last sync.
func (s *SettingsSync) ShouldSync() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.enabled {
		return false
	}
	return time.Since(s.lastSync) >= s.interval
}

// MarkSynced records that a sync just completed.
func (s *SettingsSync) MarkSynced() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastSync = time.Now()
}
