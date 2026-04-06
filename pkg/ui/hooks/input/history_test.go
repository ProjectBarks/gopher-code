package input

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInputHistory_Add(t *testing.T) {
	h := NewInputHistory()
	h.Add("cmd1")
	h.Add("cmd2")
	h.Add("cmd3")
	assert.Equal(t, 3, h.Len())
	assert.Equal(t, "cmd1", h.Items[0].Display)
	assert.Equal(t, "cmd3", h.Items[2].Display)
}

func TestInputHistory_AddEmpty(t *testing.T) {
	h := NewInputHistory()
	h.Add("")
	h.Add("   ")
	assert.Equal(t, 0, h.Len(), "empty/whitespace entries should be rejected")
}

func TestInputHistory_AddDeduplicate(t *testing.T) {
	h := NewInputHistory()
	h.Add("cmd1")
	h.Add("cmd1")
	h.Add("cmd2")
	h.Add("cmd2")
	assert.Equal(t, 2, h.Len(), "consecutive duplicates should be suppressed")
	assert.Equal(t, "cmd1", h.Items[0].Display)
	assert.Equal(t, "cmd2", h.Items[1].Display)
}

func TestInputHistory_AddNonConsecutiveDuplicateAllowed(t *testing.T) {
	h := NewInputHistory()
	h.Add("cmd1")
	h.Add("cmd2")
	h.Add("cmd1")
	assert.Equal(t, 3, h.Len(), "non-consecutive duplicates should be kept")
}

func TestInputHistory_MaxItems(t *testing.T) {
	h := NewInputHistory()
	h.MaxItems = 3
	for i := 0; i < 10; i++ {
		h.Add("cmd" + string(rune('0'+i)))
	}
	assert.Equal(t, 3, h.Len())
	// Should keep the 3 most recent.
	assert.Equal(t, "cmd7", h.Items[0].Display)
}

func TestInputHistory_NavigateUp_Empty(t *testing.T) {
	h := NewInputHistory()
	text, changed := h.NavigateUp("draft")
	assert.False(t, changed)
	assert.Equal(t, "draft", text)
}

func TestInputHistory_NavigateUpDown(t *testing.T) {
	h := NewInputHistory()
	h.Add("first")
	h.Add("second")
	h.Add("third")

	// Up from empty → most recent ("third").
	text, changed := h.NavigateUp("")
	require.True(t, changed)
	assert.Equal(t, "third", text)

	// Up again → "second".
	text, changed = h.NavigateUp(text)
	require.True(t, changed)
	assert.Equal(t, "second", text)

	// Up again → "first".
	text, changed = h.NavigateUp(text)
	require.True(t, changed)
	assert.Equal(t, "first", text)

	// Up again — already at oldest, no change.
	text, changed = h.NavigateUp(text)
	assert.False(t, changed)
	assert.Equal(t, "first", text)

	// Down → "second".
	text, changed = h.NavigateDown()
	require.True(t, changed)
	assert.Equal(t, "second", text)

	// Down → "third".
	text, changed = h.NavigateDown()
	require.True(t, changed)
	assert.Equal(t, "third", text)

	// Down past newest → restore draft (empty string).
	text, changed = h.NavigateDown()
	require.True(t, changed)
	assert.Equal(t, "", text, "should restore the original empty draft")
}

func TestInputHistory_DraftPreservation(t *testing.T) {
	h := NewInputHistory()
	h.Add("old command")

	// User is typing something.
	draft := "my partial input"

	// Press Up — should save draft and show history.
	text, changed := h.NavigateUp(draft)
	require.True(t, changed)
	assert.Equal(t, "old command", text)
	assert.Equal(t, draft, h.Draft(), "draft should be preserved")

	// Press Down past newest — should restore draft.
	text, changed = h.NavigateDown()
	require.True(t, changed)
	assert.Equal(t, draft, text)
	assert.Equal(t, 0, h.Cursor(), "cursor should be 0 after restoring draft")
}

func TestInputHistory_DraftPreservation_NonEmpty(t *testing.T) {
	h := NewInputHistory()
	h.Add("cmd1")
	h.Add("cmd2")

	// Start with non-empty input.
	text, _ := h.NavigateUp("work in progress")
	assert.Equal(t, "cmd2", text)

	text, _ = h.NavigateUp(text)
	assert.Equal(t, "cmd1", text)

	// Navigate all the way back.
	text, _ = h.NavigateDown()
	assert.Equal(t, "cmd2", text)
	text, _ = h.NavigateDown()
	assert.Equal(t, "work in progress", text, "original text should be restored")
}

func TestInputHistory_Reset(t *testing.T) {
	h := NewInputHistory()
	h.Add("cmd")
	h.NavigateUp("draft")
	assert.Equal(t, 1, h.Cursor())

	h.Reset()
	assert.Equal(t, 0, h.Cursor())
	assert.Equal(t, "", h.Draft())
}

func TestInputHistory_ModeFilter(t *testing.T) {
	h := NewInputHistory()
	h.ModeFilter = "!"
	h.Add("!bash command")
	h.Add("regular prompt")
	h.Add("!another bash")

	// Up should only show bash entries.
	text, changed := h.NavigateUp("")
	require.True(t, changed)
	assert.Equal(t, "!another bash", text)

	text, changed = h.NavigateUp(text)
	require.True(t, changed)
	assert.Equal(t, "!bash command", text)

	// No more bash entries.
	_, changed = h.NavigateUp(text)
	assert.False(t, changed)
}

func TestInputHistory_HandleKey_Up(t *testing.T) {
	h := NewInputHistory()
	h.Add("test")

	text, cmd := h.HandleKey(tea.KeyPressMsg{Code: tea.KeyUp}, "")
	assert.Equal(t, "test", text)
	assert.Nil(t, cmd)
}

func TestInputHistory_HandleKey_Down_NotNavigating(t *testing.T) {
	h := NewInputHistory()
	h.Add("test")

	text, cmd := h.HandleKey(tea.KeyPressMsg{Code: tea.KeyDown}, "current")
	assert.Equal(t, "", text)
	assert.Nil(t, cmd)
}

func TestInputHistory_HandleKey_Unrelated(t *testing.T) {
	h := NewInputHistory()
	h.Add("test")

	text, cmd := h.HandleKey(tea.KeyPressMsg{Code: 'a'}, "current")
	assert.Equal(t, "", text)
	assert.Nil(t, cmd)
}

func TestInputHistory_AddResetsNavigation(t *testing.T) {
	h := NewInputHistory()
	h.Add("cmd1")
	h.NavigateUp("")
	assert.Equal(t, 1, h.Cursor())

	h.Add("cmd2")
	assert.Equal(t, 0, h.Cursor(), "adding resets navigation")
}

func TestInputHistory_NavigateDown_WhenNotNavigating(t *testing.T) {
	h := NewInputHistory()
	_, changed := h.NavigateDown()
	assert.False(t, changed)
}

func TestInputHistory_ConcurrentAccess(t *testing.T) {
	h := NewInputHistory()
	done := make(chan struct{})

	go func() {
		for i := 0; i < 100; i++ {
			h.Add("cmd")
		}
		close(done)
	}()

	for i := 0; i < 100; i++ {
		h.Len()
		h.NavigateUp("")
		h.NavigateDown()
	}
	<-done
}
