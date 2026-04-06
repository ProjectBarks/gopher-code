package input

import (
	"strings"
	"sync"
	"time"

	tea "charm.land/bubbletea/v2"
)

// PasteThreshold is the character count above which a single input event is
// treated as a paste rather than typing. Matches the TS PASTE_THRESHOLD.
const PasteThreshold = 20

// PasteCompletionTimeout is how long to wait after the last chunk before
// finalizing a paste. Terminals may split long pastes across multiple reads.
const PasteCompletionTimeout = 100 * time.Millisecond

// PasteCompleteMsg is sent when a paste has been fully assembled.
type PasteCompleteMsg struct {
	Text     string
	IsImage  bool   // True if the pasted text looks like an image file path.
	FilePath string // Non-empty when IsImage is true.
}

// PasteHandler detects multi-line pastes and image file drops. It collects
// chunks that arrive in rapid succession and emits a single PasteCompleteMsg
// once the stream settles.
//
// Port of the TS usePasteHandler hook.
type PasteHandler struct {
	mu sync.Mutex

	chunks  []string
	pending bool // true while collecting paste chunks

	// OnPaste is called with the assembled text for regular pastes.
	// If nil, pasted text is returned via PasteCompleteMsg.
	OnPaste func(text string)

	// imageExtensions are checked to detect image file drops.
	imageExtensions map[string]bool
}

// NewPasteHandler returns a PasteHandler with default image extensions.
func NewPasteHandler() *PasteHandler {
	return &PasteHandler{
		imageExtensions: map[string]bool{
			".png": true, ".jpg": true, ".jpeg": true,
			".gif": true, ".bmp": true, ".webp": true,
			".svg": true, ".tiff": true, ".tif": true,
		},
	}
}

// pasteTimeoutMsg is an internal message sent after the timeout.
type pasteTimeoutMsg struct {
	id int // disambiguate stale timeouts
}

// timeoutID increments each time we reset the timer so stale timeouts are
// ignored.
var (
	timeoutMu sync.Mutex
	timeoutID int
)

func nextTimeoutID() int {
	timeoutMu.Lock()
	defer timeoutMu.Unlock()
	timeoutID++
	return timeoutID
}

// IsPasting returns true while the handler is accumulating paste chunks.
func (p *PasteHandler) IsPasting() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.pending
}

// HandleInput inspects an input string and returns (handled bool, cmd tea.Cmd).
// If handled is true, the caller should NOT process the input as normal
// keystrokes -- it is being buffered as part of a paste.
func (p *PasteHandler) HandleInput(text string) (bool, tea.Cmd) {
	if len(text) < PasteThreshold && !p.IsPasting() && !p.looksLikeImagePath(text) {
		return false, nil
	}

	p.mu.Lock()
	p.chunks = append(p.chunks, text)
	p.pending = true
	id := nextTimeoutID()
	p.mu.Unlock()

	// Return a tick command that will finalize the paste after the timeout.
	return true, tea.Tick(PasteCompletionTimeout, func(time.Time) tea.Msg {
		return pasteTimeoutMsg{id: id}
	})
}

// HandlePasteTimeout should be called when a pasteTimeoutMsg is received.
// It assembles the chunks and returns a PasteCompleteMsg (or nil if stale).
func (p *PasteHandler) HandlePasteTimeout(msg pasteTimeoutMsg) tea.Cmd {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.pending {
		return nil
	}

	// Check that no newer timeout supersedes this one.
	timeoutMu.Lock()
	current := timeoutID
	timeoutMu.Unlock()
	if msg.id != current {
		return nil
	}

	text := strings.Join(p.chunks, "")
	// Clean up orphaned focus sequences that can appear during paste.
	text = strings.TrimSuffix(text, "[I")
	text = strings.TrimSuffix(text, "[O")

	p.chunks = p.chunks[:0]
	p.pending = false

	isImage, filePath := p.detectImagePath(text)

	return func() tea.Msg {
		return PasteCompleteMsg{
			Text:     text,
			IsImage:  isImage,
			FilePath: filePath,
		}
	}
}

// Update processes messages relevant to paste handling. Returns (tea.Cmd, bool)
// where bool indicates whether the message was consumed.
func (p *PasteHandler) Update(msg tea.Msg) (tea.Cmd, bool) {
	if tm, ok := msg.(pasteTimeoutMsg); ok {
		cmd := p.HandlePasteTimeout(tm)
		return cmd, true
	}
	return nil, false
}

// looksLikeImagePath checks if the text looks like a dragged image file path.
func (p *PasteHandler) looksLikeImagePath(text string) bool {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return false
	}
	lower := strings.ToLower(trimmed)
	for ext := range p.imageExtensions {
		if strings.HasSuffix(lower, ext) {
			return true
		}
	}
	return false
}

// detectImagePath checks whether the pasted text is an image file path.
func (p *PasteHandler) detectImagePath(text string) (bool, string) {
	lines := strings.Split(strings.TrimSpace(text), "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if p.looksLikeImagePath(trimmed) {
			return true, trimmed
		}
	}
	return false, ""
}
