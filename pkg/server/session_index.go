// Package server — session index persistence for ~/.claude/server-sessions.json.
// Source: src/server/types.ts (SessionIndex persistence)
package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

const sessionIndexFile = "server-sessions.json"

// SessionIndexStore manages loading and saving the SessionIndex to disk.
// It uses atomic writes (write temp + rename) and a mutex for concurrent access.
type SessionIndexStore struct {
	mu   sync.Mutex
	dir  string // directory containing server-sessions.json
	path string // full path to server-sessions.json
}

// NewSessionIndexStore creates a store rooted at the given directory.
// The directory is created if it does not exist.
func NewSessionIndexStore(dir string) *SessionIndexStore {
	return &SessionIndexStore{
		dir:  dir,
		path: filepath.Join(dir, sessionIndexFile),
	}
}

// DefaultSessionIndexStore returns a store using ~/.claude as the directory.
func DefaultSessionIndexStore() (*SessionIndexStore, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("session index: %w", err)
	}
	return NewSessionIndexStore(filepath.Join(home, ".claude")), nil
}

// Load reads the session index from disk.
// Returns an empty index if the file does not exist.
func (s *SessionIndexStore) Load() (SessionIndex, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.loadLocked()
}

func (s *SessionIndexStore) loadLocked() (SessionIndex, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return make(SessionIndex), nil
		}
		return nil, fmt.Errorf("load session index: %w", err)
	}

	var idx SessionIndex
	if err := json.Unmarshal(data, &idx); err != nil {
		return nil, fmt.Errorf("parse session index: %w", err)
	}
	if idx == nil {
		idx = make(SessionIndex)
	}
	return idx, nil
}

// Save atomically writes the session index to disk.
// Creates the directory if needed. Uses write-to-temp + rename for atomicity.
func (s *SessionIndexStore) Save(idx SessionIndex) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.saveLocked(idx)
}

func (s *SessionIndexStore) saveLocked(idx SessionIndex) error {
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return fmt.Errorf("create session index dir: %w", err)
	}

	data, err := json.MarshalIndent(idx, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal session index: %w", err)
	}
	data = append(data, '\n')

	// Atomic write: temp file in same directory, then rename.
	tmp, err := os.CreateTemp(s.dir, ".server-sessions-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("close temp file: %w", err)
	}
	if err := os.Rename(tmpPath, s.path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("rename temp file: %w", err)
	}
	return nil
}

// Put adds or updates an entry and persists the index.
func (s *SessionIndexStore) Put(key string, entry SessionIndexEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	idx, err := s.loadLocked()
	if err != nil {
		return err
	}
	idx[key] = entry
	return s.saveLocked(idx)
}

// Delete removes an entry by key and persists the index.
// Returns true if the key existed.
func (s *SessionIndexStore) Delete(key string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	idx, err := s.loadLocked()
	if err != nil {
		return false, err
	}
	_, existed := idx[key]
	if !existed {
		return false, nil
	}
	delete(idx, key)
	return true, s.saveLocked(idx)
}

// Path returns the full path to the session index file.
func (s *SessionIndexStore) Path() string {
	return s.path
}
