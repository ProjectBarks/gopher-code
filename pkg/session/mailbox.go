package session

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Source: utils/teammateMailbox.ts

// TeammateMessage is a message in a teammate's inbox.
// Source: utils/teammateMailbox.ts:43-50
type TeammateMessage struct {
	From      string `json:"from"`
	Text      string `json:"text"`
	Timestamp string `json:"timestamp"`
	Read      bool   `json:"read"`
	Color     string `json:"color,omitempty"`
	Summary   string `json:"summary,omitempty"`
}

// Mailbox provides file-based messaging for agent swarms.
// Each teammate has an inbox at .claude/teams/{team}/inboxes/{agent}.json
// Source: utils/teammateMailbox.ts:1-8
type Mailbox struct {
	teamsDir string
	mu       sync.Mutex // Protects concurrent file writes
}

// NewMailbox creates a mailbox rooted in the given teams directory.
func NewMailbox(teamsDir string) *Mailbox {
	return &Mailbox{teamsDir: teamsDir}
}

// TeamsDir returns the root teams directory for this mailbox.
func (m *Mailbox) TeamsDir() string { return m.teamsDir }

// GetInboxPath returns the path to a teammate's inbox file.
// Source: utils/teammateMailbox.ts:56-66
func (m *Mailbox) GetInboxPath(agentName, teamName string) string {
	if teamName == "" {
		teamName = "default"
	}
	safeTeam := sanitizePathComponent(agentName)
	safeAgent := sanitizePathComponent(agentName)
	_ = safeTeam // Fix: use teamName
	return filepath.Join(m.teamsDir, sanitizePathComponent(teamName), "inboxes", safeAgent+".json")
}

// ReadMailbox reads all messages from a teammate's inbox.
// Source: utils/teammateMailbox.ts:84-108
func (m *Mailbox) ReadMailbox(agentName, teamName string) ([]TeammateMessage, error) {
	path := m.GetInboxPath(agentName, teamName)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var messages []TeammateMessage
	if err := json.Unmarshal(data, &messages); err != nil {
		return nil, err
	}
	return messages, nil
}

// ReadUnreadMessages returns only unread messages from a teammate's inbox.
// Source: utils/teammateMailbox.ts:115-125
func (m *Mailbox) ReadUnreadMessages(agentName, teamName string) ([]TeammateMessage, error) {
	messages, err := m.ReadMailbox(agentName, teamName)
	if err != nil {
		return nil, err
	}

	var unread []TeammateMessage
	for _, msg := range messages {
		if !msg.Read {
			unread = append(unread, msg)
		}
	}
	return unread, nil
}

// WriteToMailbox writes a message to a teammate's inbox.
// Uses mutex for concurrency safety.
// Source: utils/teammateMailbox.ts:134-190
func (m *Mailbox) WriteToMailbox(recipientName, teamName string, from, text string, opts ...WriteOption) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Ensure inbox directory exists
	// Source: teammateMailbox.ts:71-77
	path := m.GetInboxPath(recipientName, teamName)
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// Read existing messages
	messages, _ := m.readMailboxUnsafe(path)

	// Build new message
	// Source: teammateMailbox.ts:173-176
	msg := TeammateMessage{
		From:      from,
		Text:      text,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Read:      false,
	}
	for _, opt := range opts {
		opt(&msg)
	}

	messages = append(messages, msg)

	// Write back
	// Source: teammateMailbox.ts:180
	data, err := json.MarshalIndent(messages, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// MarkAllRead marks all messages in a teammate's inbox as read.
// Source: utils/teammateMailbox.ts (markAsRead pattern)
func (m *Mailbox) MarkAllRead(agentName, teamName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	path := m.GetInboxPath(agentName, teamName)
	messages, err := m.readMailboxUnsafe(path)
	if err != nil || len(messages) == 0 {
		return err
	}

	changed := false
	for i := range messages {
		if !messages[i].Read {
			messages[i].Read = true
			changed = true
		}
	}

	if !changed {
		return nil
	}

	data, err := json.MarshalIndent(messages, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// readMailboxUnsafe reads without locking (caller must hold lock).
func (m *Mailbox) readMailboxUnsafe(path string) ([]TeammateMessage, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var messages []TeammateMessage
	if err := json.Unmarshal(data, &messages); err != nil {
		return nil, err
	}
	return messages, nil
}

// WriteOption configures optional fields on a TeammateMessage.
type WriteOption func(*TeammateMessage)

// WithColor sets the sender's color.
func WithColor(color string) WriteOption {
	return func(m *TeammateMessage) { m.Color = color }
}

// WithSummary sets a short preview summary.
func WithSummary(summary string) WriteOption {
	return func(m *TeammateMessage) { m.Summary = summary }
}

// sanitizePathComponent removes dangerous characters from path components.
func sanitizePathComponent(s string) string {
	s = strings.ReplaceAll(s, "/", "_")
	s = strings.ReplaceAll(s, "\\", "_")
	s = strings.ReplaceAll(s, "..", "_")
	s = strings.ReplaceAll(s, "\x00", "_")
	return s
}
