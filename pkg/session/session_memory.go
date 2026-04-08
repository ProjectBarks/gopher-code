package session

// Source: services/sessionMemory.ts

// SessionMemoryEntry represents a piece of session-level memory
// (not persisted across sessions, used for in-context recall).
type SessionMemoryEntry struct {
	Key       string
	Value     string
	TurnIndex int
}

// SessionMemory tracks in-session memories for context recall.
type SessionMemory struct {
	entries []SessionMemoryEntry
}

// NewSessionMemory creates a new session memory store.
func NewSessionMemory() *SessionMemory {
	return &SessionMemory{}
}

// Add stores a session memory entry.
func (m *SessionMemory) Add(key, value string, turnIndex int) {
	m.entries = append(m.entries, SessionMemoryEntry{
		Key: key, Value: value, TurnIndex: turnIndex,
	})
}

// Get returns the latest value for a key, or empty string.
func (m *SessionMemory) Get(key string) string {
	for i := len(m.entries) - 1; i >= 0; i-- {
		if m.entries[i].Key == key {
			return m.entries[i].Value
		}
	}
	return ""
}

// All returns all session memory entries.
func (m *SessionMemory) All() []SessionMemoryEntry {
	return m.entries
}

// Clear removes all entries.
func (m *SessionMemory) Clear() {
	m.entries = nil
}
