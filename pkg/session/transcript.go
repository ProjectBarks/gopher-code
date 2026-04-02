package session

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/projectbarks/gopher-code/pkg/message"
)

// Source: utils/sessionStorage.ts

// TranscriptEntry is a single JSONL line in a session transcript.
// The Type field determines which other fields are populated.
// Source: utils/sessionStorage.ts:768-834
type TranscriptEntry struct {
	Type      string    `json:"type"`
	Timestamp time.Time `json:"timestamp,omitempty"`
	SessionID string    `json:"sessionId,omitempty"`

	// type: "message"
	Message *message.Message `json:"message,omitempty"`
	Role    string           `json:"role,omitempty"`

	// type: "usage"
	InputTokens  int `json:"inputTokens,omitempty"`
	OutputTokens int `json:"outputTokens,omitempty"`
	TurnCount    int `json:"turnCount,omitempty"`

	// type: "custom-title"
	CustomTitle string `json:"customTitle,omitempty"`

	// type: "tag"
	Tag string `json:"tag,omitempty"`

	// type: "cwd"
	CWD string `json:"cwd,omitempty"`

	// type: "last-prompt"
	LastPrompt string `json:"lastPrompt,omitempty"`

	// type: "model"
	Model string `json:"model,omitempty"`
}

// TranscriptWriter appends JSONL entries to a session file.
// Source: utils/sessionStorage.ts:2568-2584
type TranscriptWriter struct {
	path string
}

// NewTranscriptWriter creates a writer for the given session transcript file.
func NewTranscriptWriter(path string) *TranscriptWriter {
	return &TranscriptWriter{path: path}
}

// TranscriptPathForSession returns the JSONL path for a session ID.
// Source: utils/sessionStorage.ts:204
func TranscriptPathForSession(projectDir, sessionID string) string {
	return filepath.Join(projectDir, sessionID+".jsonl")
}

// Append writes a transcript entry as a single JSONL line.
// Creates the parent directory if missing.
// Source: utils/sessionStorage.ts:2572-2584
func (w *TranscriptWriter) Append(entry TranscriptEntry) error {
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now()
	}

	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("marshal transcript entry: %w", err)
	}
	line := string(data) + "\n"

	// Try append first; create dir on failure
	f, err := os.OpenFile(w.path, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0600)
	if err != nil {
		if err := os.MkdirAll(filepath.Dir(w.path), 0700); err != nil {
			return fmt.Errorf("create transcript dir: %w", err)
		}
		f, err = os.OpenFile(w.path, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0600)
		if err != nil {
			return fmt.Errorf("open transcript: %w", err)
		}
	}
	defer f.Close()

	_, err = f.WriteString(line)
	return err
}

// AppendMessage writes a message event to the transcript.
func (w *TranscriptWriter) AppendMessage(sessionID string, msg message.Message) error {
	return w.Append(TranscriptEntry{
		Type:      "message",
		SessionID: sessionID,
		Message:   &msg,
		Role:      string(msg.Role),
	})
}

// AppendUsage writes a usage event to the transcript.
func (w *TranscriptWriter) AppendUsage(sessionID string, inputTokens, outputTokens, turnCount int) error {
	return w.Append(TranscriptEntry{
		Type:         "usage",
		SessionID:    sessionID,
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		TurnCount:    turnCount,
	})
}

// AppendTitle writes a custom-title event to the transcript.
// Source: utils/sessionStorage.ts:777-781
func (w *TranscriptWriter) AppendTitle(sessionID, title string) error {
	return w.Append(TranscriptEntry{
		Type:        "custom-title",
		SessionID:   sessionID,
		CustomTitle: title,
	})
}

// ReadTranscript reads all entries from a JSONL transcript file.
func ReadTranscript(path string) ([]TranscriptEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var entries []TranscriptEntry
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var entry TranscriptEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue // Skip malformed lines
		}
		entries = append(entries, entry)
	}

	return entries, scanner.Err()
}

// RestoreMessages extracts messages from transcript entries.
func RestoreMessages(entries []TranscriptEntry) []message.Message {
	var msgs []message.Message
	for _, e := range entries {
		if e.Type == "message" && e.Message != nil {
			msgs = append(msgs, *e.Message)
		}
	}
	return msgs
}
