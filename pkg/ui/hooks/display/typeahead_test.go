package display

import (
	"sync"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func key(r rune) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: r}
}

func TestTypeahead_UnblockedPassesThrough(t *testing.T) {
	ta := NewTypeahead()
	ok := ta.Push(key('a'))
	assert.True(t, ok, "unblocked push should return ok=true")
	assert.Equal(t, 0, ta.Len(), "nothing should be buffered when unblocked")
}

func TestTypeahead_BlockedBuffersKeystrokes(t *testing.T) {
	ta := NewTypeahead()
	ta.Block()

	ok := ta.Push(key('a'))
	assert.False(t, ok, "blocked push should return ok=false")
	assert.Equal(t, 1, ta.Len())

	ta.Push(key('b'))
	ta.Push(key('c'))
	assert.Equal(t, 3, ta.Len())
}

func TestTypeahead_UnblockReturnsBufferedInOrder(t *testing.T) {
	ta := NewTypeahead()
	ta.Block()

	ta.Push(key('x'))
	ta.Push(key('y'))
	ta.Push(key('z'))

	got := ta.Unblock()
	require.Len(t, got, 3)
	assert.Equal(t, rune('x'), got[0].Code)
	assert.Equal(t, rune('y'), got[1].Code)
	assert.Equal(t, rune('z'), got[2].Code)

	assert.False(t, ta.IsBlocked())
	assert.Equal(t, 0, ta.Len(), "buffer should be empty after unblock")
}

func TestTypeahead_UnblockWhenEmptyReturnsNil(t *testing.T) {
	ta := NewTypeahead()
	ta.Block()
	got := ta.Unblock()
	assert.Nil(t, got)
}

func TestTypeahead_ClearDiscardsBuffer(t *testing.T) {
	ta := NewTypeahead()
	ta.Block()
	ta.Push(key('a'))
	ta.Push(key('b'))
	ta.Clear()
	assert.Equal(t, 0, ta.Len())
	// Still blocked after clear.
	assert.True(t, ta.IsBlocked())
}

func TestTypeahead_BlockUnblockCycle(t *testing.T) {
	ta := NewTypeahead()

	// First cycle.
	ta.Block()
	ta.Push(key('1'))
	got := ta.Unblock()
	require.Len(t, got, 1)

	// Keys pass through after unblock.
	ok := ta.Push(key('2'))
	assert.True(t, ok)

	// Second cycle.
	ta.Block()
	ta.Push(key('3'))
	got2 := ta.Unblock()
	require.Len(t, got2, 1)
	assert.Equal(t, rune('3'), got2[0].Code)
}

func TestTypeahead_ConcurrentAccess(t *testing.T) {
	ta := NewTypeahead()
	ta.Block()

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ta.Push(key('x'))
		}()
	}
	wg.Wait()

	got := ta.Unblock()
	assert.Len(t, got, 100)
}
