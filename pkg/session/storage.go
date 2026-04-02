package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const sessionDir = ".claude/sessions"

// homeDirFn is the function used to resolve the home directory.
// Override in tests via SetHomeDirForTest.
var homeDirFn = os.UserHomeDir

// SessionMetadata is the summary stored alongside each session.
type SessionMetadata struct {
	ID        string    `json:"id"`
	Model     string    `json:"model"`
	CWD       string    `json:"cwd"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	TurnCount int       `json:"turn_count"`
	Name      string    `json:"name,omitempty"`
}

// SetHomeDirForTest overrides the home directory function for testing.
func SetHomeDirForTest(dir string) {
	homeDirFn = func() (string, error) { return dir, nil }
}

// Save persists the session state to ~/.claude/sessions/{id}.json
func (s *SessionState) Save() error {
	dir := filepath.Join(homeDir(), sessionDir)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create session dir: %w", err)
	}

	// Save full session
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal session: %w", err)
	}
	path := filepath.Join(dir, s.ID+".json")
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("write session: %w", err)
	}

	// Save metadata index
	meta := SessionMetadata{
		ID:        s.ID,
		Model:     s.Config.Model,
		CWD:       s.CWD,
		CreatedAt: s.CreatedAt,
		UpdatedAt: time.Now(),
		TurnCount: s.TurnCount,
	}
	metaData, _ := json.MarshalIndent(meta, "", "  ")
	metaPath := filepath.Join(dir, s.ID+".meta.json")
	os.WriteFile(metaPath, metaData, 0600)

	return nil
}

// Load reads a session from ~/.claude/sessions/{id}.json
func Load(id string) (*SessionState, error) {
	path := filepath.Join(homeDir(), sessionDir, id+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read session %s: %w", id, err)
	}
	var s SessionState
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("unmarshal session: %w", err)
	}
	return &s, nil
}

// LoadLatest loads the most recently updated session in the given CWD.
// If cwd is empty, it returns the most recent session regardless of CWD.
func LoadLatest(cwd string) (*SessionState, error) {
	dir := filepath.Join(homeDir(), sessionDir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("list sessions: %w", err)
	}

	// Find .meta.json files, filter by CWD, sort by UpdatedAt
	type metaEntry struct {
		meta SessionMetadata
	}
	var candidates []metaEntry

	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".meta.json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		var m SessionMetadata
		if json.Unmarshal(data, &m) != nil {
			continue
		}
		if cwd != "" && m.CWD != cwd {
			continue
		}
		candidates = append(candidates, metaEntry{meta: m})
	}

	if len(candidates) == 0 {
		return nil, fmt.Errorf("no sessions found")
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].meta.UpdatedAt.After(candidates[j].meta.UpdatedAt)
	})

	return Load(candidates[0].meta.ID)
}

// ListSessions returns metadata for all saved sessions, most recently updated first.
func ListSessions() ([]SessionMetadata, error) {
	dir := filepath.Join(homeDir(), sessionDir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var metas []SessionMetadata
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".meta.json") {
			continue
		}
		data, _ := os.ReadFile(filepath.Join(dir, e.Name()))
		var m SessionMetadata
		if json.Unmarshal(data, &m) == nil {
			metas = append(metas, m)
		}
	}
	sort.Slice(metas, func(i, j int) bool {
		return metas[i].UpdatedAt.After(metas[j].UpdatedAt)
	})
	return metas, nil
}

func homeDir() string {
	home, _ := homeDirFn()
	return home
}
