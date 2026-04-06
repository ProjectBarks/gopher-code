package input

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPasteHandler_ShortInputNotHandled(t *testing.T) {
	p := NewPasteHandler()
	handled, _ := p.HandleInput("hello")
	assert.False(t, handled, "short input should not be treated as paste")
}

func TestPasteHandler_LongInputHandled(t *testing.T) {
	p := NewPasteHandler()
	longText := strings.Repeat("a", PasteThreshold+1)
	handled, cmd := p.HandleInput(longText)
	assert.True(t, handled, "long input should be treated as paste")
	assert.NotNil(t, cmd)
	assert.True(t, p.IsPasting())
}

func TestPasteHandler_ImagePathDetected(t *testing.T) {
	p := NewPasteHandler()
	handled, _ := p.HandleInput("/Users/test/photo.png")
	assert.True(t, handled, "image path should be treated as paste")
}

func TestPasteHandler_ImageExtensions(t *testing.T) {
	p := NewPasteHandler()
	extensions := []string{".png", ".jpg", ".jpeg", ".gif", ".bmp", ".webp", ".svg", ".tiff", ".tif"}
	for _, ext := range extensions {
		handled, _ := p.HandleInput("/path/to/file" + ext)
		assert.True(t, handled, "image extension %s should be detected", ext)
	}
}

func TestPasteHandler_NonImagePathNotHandled(t *testing.T) {
	p := NewPasteHandler()
	handled, _ := p.HandleInput("/path/to/file.txt")
	assert.False(t, handled, "non-image path should not be treated as paste")
}

func TestPasteHandler_MultipleChunks(t *testing.T) {
	p := NewPasteHandler()

	// First chunk.
	longText := strings.Repeat("x", PasteThreshold+1)
	handled, _ := p.HandleInput(longText)
	require.True(t, handled)

	// Second chunk while still pasting.
	handled, _ = p.HandleInput("more text")
	assert.True(t, handled, "subsequent chunks should also be captured")
}

func TestPasteHandler_IsPasting(t *testing.T) {
	p := NewPasteHandler()
	assert.False(t, p.IsPasting())

	longText := strings.Repeat("a", PasteThreshold+1)
	p.HandleInput(longText)
	assert.True(t, p.IsPasting())
}

func TestPasteHandler_CompleteMsg(t *testing.T) {
	p := NewPasteHandler()

	// Simulate a paste with an image path.
	p.mu.Lock()
	p.chunks = []string{"/Users/test/photo.png"}
	p.pending = true
	p.mu.Unlock()

	id := nextTimeoutID()
	cmd := p.HandlePasteTimeout(pasteTimeoutMsg{id: id})
	require.NotNil(t, cmd)

	msg := cmd()
	pm, ok := msg.(PasteCompleteMsg)
	require.True(t, ok)
	assert.Equal(t, "/Users/test/photo.png", pm.Text)
	assert.True(t, pm.IsImage)
	assert.Equal(t, "/Users/test/photo.png", pm.FilePath)
	assert.False(t, p.IsPasting(), "should no longer be pasting after completion")
}

func TestPasteHandler_CompleteMsgRegularText(t *testing.T) {
	p := NewPasteHandler()

	p.mu.Lock()
	p.chunks = []string{"hello\nworld\nfoo bar"}
	p.pending = true
	p.mu.Unlock()

	id := nextTimeoutID()
	cmd := p.HandlePasteTimeout(pasteTimeoutMsg{id: id})
	require.NotNil(t, cmd)

	msg := cmd()
	pm, ok := msg.(PasteCompleteMsg)
	require.True(t, ok)
	assert.Equal(t, "hello\nworld\nfoo bar", pm.Text)
	assert.False(t, pm.IsImage)
}

func TestPasteHandler_StaleTimeoutIgnored(t *testing.T) {
	p := NewPasteHandler()
	p.mu.Lock()
	p.chunks = []string{"text"}
	p.pending = true
	p.mu.Unlock()

	// Use an old ID.
	cmd := p.HandlePasteTimeout(pasteTimeoutMsg{id: -999})
	assert.Nil(t, cmd, "stale timeout should be ignored")
}

func TestPasteHandler_CleansFocusSequences(t *testing.T) {
	p := NewPasteHandler()
	p.mu.Lock()
	p.chunks = []string{"pasted text[I"}
	p.pending = true
	p.mu.Unlock()

	id := nextTimeoutID()
	cmd := p.HandlePasteTimeout(pasteTimeoutMsg{id: id})
	require.NotNil(t, cmd)

	msg := cmd()
	pm := msg.(PasteCompleteMsg)
	assert.Equal(t, "pasted text", pm.Text, "focus sequences should be stripped")
}

func TestPasteHandler_Update(t *testing.T) {
	p := NewPasteHandler()
	p.mu.Lock()
	p.chunks = []string{"data"}
	p.pending = true
	p.mu.Unlock()

	id := nextTimeoutID()
	cmd, handled := p.Update(pasteTimeoutMsg{id: id})
	assert.True(t, handled)
	assert.NotNil(t, cmd)
}

func TestPasteHandler_Update_UnrelatedMsg(t *testing.T) {
	p := NewPasteHandler()
	cmd, handled := p.Update("some other message")
	assert.False(t, handled)
	assert.Nil(t, cmd)
}
