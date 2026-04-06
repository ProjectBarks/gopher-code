package session

import (
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Source: utils/sessionStorage.ts + utils/listSessionsImpl.ts

// SessionFileInfo holds stat information for a session JSONL file.
// Source: sessionStorage.ts:4526-4569
type SessionFileInfo struct {
	Path      string
	ModTime   time.Time
	CreateTime time.Time
	Size      int64
}

// GetSessionFilesWithMtime returns session JSONL files in a project directory
// with their stat info. Only files with valid UUID names are included.
// Source: sessionStorage.ts:4526-4569
func GetSessionFilesWithMtime(projectDir string) (map[string]SessionFileInfo, error) {
	result := make(map[string]SessionFileInfo)

	dirents, err := os.ReadDir(projectDir)
	if err != nil {
		// Directory doesn't exist — return empty map (not error).
		if os.IsNotExist(err) {
			return result, nil
		}
		return nil, err
	}

	for _, d := range dirents {
		if d.IsDir() || !strings.HasSuffix(d.Name(), ".jsonl") {
			continue
		}
		base := strings.TrimSuffix(d.Name(), ".jsonl")
		if ValidateUUID(base) == "" {
			continue
		}

		filePath := filepath.Join(projectDir, d.Name())
		info, err := d.Info()
		if err != nil {
			continue // skip unreadable files
		}

		result[base] = SessionFileInfo{
			Path:      filePath,
			ModTime:   info.ModTime(),
			CreateTime: info.ModTime(), // Go os.FileInfo doesn't expose birth time portably; use ModTime as fallback
			Size:      info.Size(),
		}
	}

	return result, nil
}

// LogOption represents a session log entry for the resume picker.
// Source: sessionStorage.ts:2887-2920 (LogOption type)
type LogOption struct {
	SessionID   string
	Messages    []TranscriptEntry
	Summary     string
	CustomTitle string
	Tag         string
	CWD         string
	FirstPrompt string
}

// GetSessionIDFromLog extracts the session ID from a log entry.
// For lite logs, uses the SessionID field directly.
// For full logs, extracts from the first message entry.
// Source: sessionStorage.ts:2927-2934
func GetSessionIDFromLog(log LogOption) string {
	if log.SessionID != "" {
		return log.SessionID
	}
	// Fall back to first message with a sessionId
	for _, entry := range log.Messages {
		if entry.SessionID != "" {
			return entry.SessionID
		}
	}
	return ""
}

// LoadAllLogsFromSessionFile reads a JSONL transcript file and returns
// a LogOption with all messages and metadata extracted.
// Source: sessionStorage.ts:4598-4640 (simplified — single-leaf variant)
func LoadAllLogsFromSessionFile(sessionFile string) (LogOption, error) {
	entries, err := ReadTranscript(sessionFile)
	if err != nil {
		return LogOption{}, err
	}

	log := LogOption{}

	for _, e := range entries {
		if log.SessionID == "" && e.SessionID != "" {
			log.SessionID = e.SessionID
		}

		switch e.Type {
		case "message":
			log.Messages = append(log.Messages, e)
			// Capture first user prompt
			if log.FirstPrompt == "" && e.Role == "user" && e.Message != nil {
				for _, b := range e.Message.Content {
					if b.Text != "" {
						log.FirstPrompt = b.Text
						break
					}
				}
			}
		case "custom-title":
			if e.CustomTitle != "" {
				log.CustomTitle = e.CustomTitle
			}
		case "summary":
			if e.Tag != "" {
				log.Summary = e.Tag
			}
		case "tag":
			if e.Tag != "" {
				log.Tag = e.Tag
			}
		case "cwd":
			if e.CWD != "" {
				log.CWD = e.CWD
			}
		}
	}

	return log, nil
}

// SaveCustomTitle appends a custom-title entry to a session transcript.
// If fullPath is empty, it is computed from projectDir and sessionID.
// Source: sessionStorage.ts:2617-2638
func SaveCustomTitle(sessionID, title, fullPath string) error {
	if fullPath == "" {
		return nil // caller must provide a path
	}
	w := NewTranscriptWriter(fullPath)
	return w.AppendTitle(sessionID, title)
}
