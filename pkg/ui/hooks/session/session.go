// Package session provides bubbletea hook models for session-level concerns:
// incremental message logging to JSONL transcript, session backgrounding
// (Ctrl+B to spawn/resume background tasks), and file-history snapshots for
// undo/rewind support.
//
// Source: src/hooks/useLogMessages.ts, src/hooks/useSessionBackgrounding.ts,
//
//	src/hooks/useFileHistorySnapshotInit.ts, src/utils/fileHistory.ts
package session

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/projectbarks/gopher-code/pkg/message"
	sess "github.com/projectbarks/gopher-code/pkg/session"
)

// ---------------------------------------------------------------------------
// MessageLogger — incremental JSONL transcript writer
// Source: src/hooks/useLogMessages.ts (119 LOC)
// ---------------------------------------------------------------------------

// MessageLogger tracks the last-written index into the message slice and
// appends only new messages to the JSONL transcript on each turn. This avoids
// re-scanning the full message array on every render (~20x/turn).
//
// Compaction detection: if the first message UUID changes (e.g. after /clear
// or compaction), the logger resets and re-writes the full array.
type MessageLogger struct {
	writer    *sess.TranscriptWriter
	sessionID string

	mu              sync.Mutex
	lastWrittenIdx  int    // resume marker (messages[0:lastWrittenIdx] already on disk)
	firstMsgContent string // fingerprint of messages[0] — detects compaction
	ignore          bool
}

// NewMessageLogger creates a logger for the given session transcript file.
func NewMessageLogger(transcriptPath, sessionID string) *MessageLogger {
	return &MessageLogger{
		writer:    sess.NewTranscriptWriter(transcriptPath),
		sessionID: sessionID,
	}
}

// SetIgnore toggles recording suppression (e.g. for headless/pipe mode).
func (l *MessageLogger) SetIgnore(ignore bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.ignore = ignore
}

// messageFingerprint returns a lightweight fingerprint for compaction detection.
func messageFingerprint(msg message.Message) string {
	if len(msg.Content) == 0 {
		return ""
	}
	return fmt.Sprintf("%s:%s:%s", msg.Role, msg.Content[0].Type, msg.Content[0].Text)
}

// LogMessagesMsg is dispatched after messages are written to the transcript.
type LogMessagesMsg struct {
	Count int   // number of messages written
	Err   error // nil on success
}

// LogMessages writes any new messages since the last call to the JSONL transcript.
// Returns a tea.Cmd that performs the I/O and reports the result.
func (l *MessageLogger) LogMessages(messages []message.Message) tea.Cmd {
	l.mu.Lock()
	if l.ignore || len(messages) == 0 {
		l.mu.Unlock()
		return nil
	}

	// Detect compaction: first message changed → reset and re-write all.
	currentFP := messageFingerprint(messages[0])
	isCompaction := l.firstMsgContent != "" && currentFP != l.firstMsgContent
	isShrink := l.lastWrittenIdx > len(messages)

	startIdx := l.lastWrittenIdx
	if isCompaction || isShrink {
		startIdx = 0
	}
	if startIdx >= len(messages) {
		l.mu.Unlock()
		return nil
	}

	// Snapshot the slice to write.
	toWrite := make([]message.Message, len(messages)-startIdx)
	copy(toWrite, messages[startIdx:])

	// Update bookkeeping eagerly (fire-and-forget pattern from TS).
	l.lastWrittenIdx = len(messages)
	l.firstMsgContent = currentFP
	writer := l.writer
	sid := l.sessionID
	l.mu.Unlock()

	return func() tea.Msg {
		var writeErr error
		for _, msg := range toWrite {
			if err := writer.AppendMessage(sid, msg); err != nil {
				writeErr = err
			}
		}
		return LogMessagesMsg{Count: len(toWrite), Err: writeErr}
	}
}

// Reset clears the tracking state (e.g. after /clear).
func (l *MessageLogger) Reset() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.lastWrittenIdx = 0
	l.firstMsgContent = ""
}

// LastWrittenIndex returns how many messages have been written (for testing).
func (l *MessageLogger) LastWrittenIndex() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.lastWrittenIdx
}

// ---------------------------------------------------------------------------
// SessionBackgrounder — Ctrl+B background/foreground task management
// Source: src/hooks/useSessionBackgrounding.ts (158 LOC)
// ---------------------------------------------------------------------------

// TaskStatus represents the state of a background task.
type TaskStatus int

const (
	TaskRunning   TaskStatus = iota
	TaskCompleted
	TaskAborted
)

// BackgroundTask represents a task that has been backgrounded.
type BackgroundTask struct {
	ID             string
	Messages       []message.Message
	Status         TaskStatus
	IsBackgrounded bool
	CreatedAt      time.Time
}

// BackgroundTaskMsg is dispatched when a background task changes state.
type BackgroundTaskMsg struct {
	TaskID string
	Status TaskStatus
}

// SessionBackgrounder manages backgrounding the current session query and
// foregrounding background tasks. Corresponds to Ctrl+B in the TUI.
type SessionBackgrounder struct {
	mu               sync.Mutex
	tasks            map[string]*BackgroundTask
	foregroundedID   string // currently-foregrounded task ID (empty = none)
	lastSyncedLength int    // tracks message-sync to avoid redundant updates

	// OnBackground is called when the user backgrounds the current query.
	// The caller should spawn the background task and return its ID.
	OnBackground func() string
}

// NewSessionBackgrounder creates a backgrounder.
func NewSessionBackgrounder(onBackground func() string) *SessionBackgrounder {
	return &SessionBackgrounder{
		tasks:        make(map[string]*BackgroundTask),
		OnBackground: onBackground,
	}
}

// HandleBackground processes a Ctrl+B press.
// If a task is foregrounded, re-backgrounds it. Otherwise spawns a new background task.
func (b *SessionBackgrounder) HandleBackground() {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.foregroundedID != "" {
		// Re-background the currently foregrounded task.
		if task, ok := b.tasks[b.foregroundedID]; ok {
			task.IsBackgrounded = true
		}
		b.foregroundedID = ""
		b.lastSyncedLength = 0
		return
	}

	// Background current query.
	if b.OnBackground != nil {
		taskID := b.OnBackground()
		if taskID != "" {
			b.tasks[taskID] = &BackgroundTask{
				ID:             taskID,
				Status:         TaskRunning,
				IsBackgrounded: true,
				CreatedAt:      time.Now(),
			}
		}
	}
}

// Foreground brings a background task to the foreground for viewing.
func (b *SessionBackgrounder) Foreground(taskID string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	task, ok := b.tasks[taskID]
	if !ok {
		return false
	}
	task.IsBackgrounded = false
	b.foregroundedID = taskID
	b.lastSyncedLength = 0
	return true
}

// ForegroundedTaskID returns the currently-foregrounded task ID (empty if none).
func (b *SessionBackgrounder) ForegroundedTaskID() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.foregroundedID
}

// UpdateTask updates a background task's messages and/or status.
func (b *SessionBackgrounder) UpdateTask(taskID string, messages []message.Message, status TaskStatus) {
	b.mu.Lock()
	defer b.mu.Unlock()

	task, ok := b.tasks[taskID]
	if !ok {
		return
	}
	task.Messages = messages
	task.Status = status

	// Auto-restore to background on completion.
	if status == TaskCompleted || status == TaskAborted {
		if b.foregroundedID == taskID {
			task.IsBackgrounded = true
			b.foregroundedID = ""
			b.lastSyncedLength = 0
		}
	}
}

// Tasks returns a snapshot of all background tasks.
func (b *SessionBackgrounder) Tasks() map[string]*BackgroundTask {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make(map[string]*BackgroundTask, len(b.tasks))
	for k, v := range b.tasks {
		cp := *v
		out[k] = &cp
	}
	return out
}

// IsTmuxOrScreen returns true if the current process is running inside
// tmux or GNU screen (detected via environment variables).
func IsTmuxOrScreen() bool {
	if os.Getenv("TMUX") != "" {
		return true
	}
	if term := os.Getenv("TERM"); term == "screen" || term == "screen-256color" {
		return true
	}
	if os.Getenv("STY") != "" {
		return true // GNU Screen session
	}
	return false
}

// ---------------------------------------------------------------------------
// FileHistorySnapshot — captures file states at turn boundaries for undo
// Source: src/utils/fileHistory.ts, src/hooks/useFileHistorySnapshotInit.ts
// ---------------------------------------------------------------------------

// MaxSnapshots is the maximum number of snapshots retained before eviction.
// Source: src/utils/fileHistory.ts MAX_SNAPSHOTS
const MaxSnapshots = 100

// FileBackup records a single file's backup within a snapshot.
type FileBackup struct {
	BackupFileName string    `json:"backup_file_name"` // empty string means file did not exist
	Version        int       `json:"version"`
	BackupTime     time.Time `json:"backup_time"`
}

// FileSnapshot captures all tracked file backups at a single turn boundary.
type FileSnapshot struct {
	MessageID          string                `json:"message_id"`
	TrackedFileBackups map[string]FileBackup `json:"tracked_file_backups"`
	Timestamp          time.Time             `json:"timestamp"`
}

// FileHistoryState holds the mutable file-history tracking state.
type FileHistoryState struct {
	Snapshots        []FileSnapshot `json:"snapshots"`
	TrackedFiles     map[string]bool
	SnapshotSequence int // monotonically increasing, even after eviction
}

// NewFileHistoryState creates an empty file-history state.
func NewFileHistoryState() *FileHistoryState {
	return &FileHistoryState{
		TrackedFiles: make(map[string]bool),
	}
}

// FileHistoryManager manages file-history snapshots for undo/rewind.
type FileHistoryManager struct {
	mu       sync.Mutex
	state    *FileHistoryState
	backupDir string // directory where backup copies are stored
	enabled  bool
}

// NewFileHistoryManager creates a manager. backupDir is where file copies are stored.
func NewFileHistoryManager(backupDir string, enabled bool) *FileHistoryManager {
	return &FileHistoryManager{
		state:     NewFileHistoryState(),
		backupDir: backupDir,
		enabled:   enabled,
	}
}

// Enabled returns whether file history is active.
func (m *FileHistoryManager) Enabled() bool {
	return m.enabled
}

// State returns a copy of the current state (for serialization / inspection).
func (m *FileHistoryManager) State() FileHistoryState {
	m.mu.Lock()
	defer m.mu.Unlock()
	return *m.state
}

// TrackFile registers a file path for snapshot tracking.
func (m *FileHistoryManager) TrackFile(path string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.state.TrackedFiles[path] = true
}

// backupFileName returns a deterministic backup name: sha256(path)@vN
func backupFileName(filePath string, version int) string {
	h := sha256.Sum256([]byte(filePath))
	return fmt.Sprintf("%s@v%d", hex.EncodeToString(h[:16]), version)
}

// MakeSnapshot captures the current state of all tracked files as a new snapshot.
// This should be called at turn boundaries (after assistant responses).
func (m *FileHistoryManager) MakeSnapshot(messageID string) (*FileSnapshot, error) {
	if !m.enabled {
		return nil, nil
	}

	m.mu.Lock()
	trackedFiles := make([]string, 0, len(m.state.TrackedFiles))
	for f := range m.state.TrackedFiles {
		trackedFiles = append(trackedFiles, f)
	}

	// Find latest version for each file from the most recent snapshot.
	var prevSnap *FileSnapshot
	if len(m.state.Snapshots) > 0 {
		prevSnap = &m.state.Snapshots[len(m.state.Snapshots)-1]
	}
	m.mu.Unlock()

	// Phase 2: async-safe file I/O outside the lock.
	backups := make(map[string]FileBackup, len(trackedFiles))
	if err := os.MkdirAll(m.backupDir, 0700); err != nil {
		return nil, fmt.Errorf("create backup dir: %w", err)
	}

	for _, fp := range trackedFiles {
		version := 1
		if prevSnap != nil {
			if prev, ok := prevSnap.TrackedFileBackups[fp]; ok {
				version = prev.Version + 1
			}
		}

		bkName, err := m.createBackup(fp, version)
		if err != nil {
			// File may have been deleted — record empty backup name.
			backups[fp] = FileBackup{
				BackupFileName: "",
				Version:        version,
				BackupTime:     time.Now(),
			}
			continue
		}
		backups[fp] = FileBackup{
			BackupFileName: bkName,
			Version:        version,
			BackupTime:     time.Now(),
		}
	}

	snap := &FileSnapshot{
		MessageID:          messageID,
		TrackedFileBackups: backups,
		Timestamp:          time.Now(),
	}

	// Phase 3: commit under lock.
	m.mu.Lock()
	m.state.Snapshots = append(m.state.Snapshots, *snap)
	m.state.SnapshotSequence++
	// Evict old snapshots.
	if len(m.state.Snapshots) > MaxSnapshots {
		m.state.Snapshots = m.state.Snapshots[len(m.state.Snapshots)-MaxSnapshots:]
	}
	m.mu.Unlock()

	return snap, nil
}

// RestoreSnapshot restores tracked files to the state captured in the given snapshot.
func (m *FileHistoryManager) RestoreSnapshot(snap *FileSnapshot) error {
	if snap == nil {
		return fmt.Errorf("nil snapshot")
	}

	for filePath, backup := range snap.TrackedFileBackups {
		if backup.BackupFileName == "" {
			// File did not exist at snapshot time — remove it.
			_ = os.Remove(filePath)
			continue
		}
		src := filepath.Join(m.backupDir, backup.BackupFileName)
		if err := copyFileContents(src, filePath); err != nil {
			return fmt.Errorf("restore %s: %w", filePath, err)
		}
	}
	return nil
}

// RestoreFromLog re-initializes file history state from persisted snapshots
// (called on session restore). Source: useFileHistorySnapshotInit.ts.
func (m *FileHistoryManager) RestoreFromLog(snapshots []FileSnapshot) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.state.Snapshots = snapshots
	m.state.SnapshotSequence = len(snapshots)

	// Rebuild tracked-files set from all snapshots.
	m.state.TrackedFiles = make(map[string]bool)
	for _, snap := range snapshots {
		for fp := range snap.TrackedFileBackups {
			m.state.TrackedFiles[fp] = true
		}
	}
}

// Snapshots returns a copy of all current snapshots.
func (m *FileHistoryManager) Snapshots() []FileSnapshot {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]FileSnapshot, len(m.state.Snapshots))
	copy(out, m.state.Snapshots)
	return out
}

// createBackup copies a file to the backup directory. Returns the backup file name.
// If the source file does not exist, returns empty string and an error.
func (m *FileHistoryManager) createBackup(filePath string, version int) (string, error) {
	bkName := backupFileName(filePath, version)
	dst := filepath.Join(m.backupDir, bkName)
	if err := copyFileContents(filePath, dst); err != nil {
		return "", err
	}
	return bkName, nil
}

// copyFileContents copies src to dst, creating parent directories as needed.
func copyFileContents(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0700); err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0600)
}
