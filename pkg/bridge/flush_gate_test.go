package bridge

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFlushGate_ZeroValueIsInactive(t *testing.T) {
	var g FlushGate[string]
	assert.False(t, g.Active())
	assert.Equal(t, 0, g.PendingCount())
}

func TestFlushGate_StartActivates(t *testing.T) {
	var g FlushGate[string]
	g.Start()
	assert.True(t, g.Active())
}

func TestFlushGate_EnqueueWhenInactive(t *testing.T) {
	var g FlushGate[string]
	ok := g.Enqueue("a", "b")
	assert.False(t, ok, "enqueue on inactive gate should return false")
	assert.Equal(t, 0, g.PendingCount())
}

func TestFlushGate_EnqueueWhenActive(t *testing.T) {
	var g FlushGate[int]
	g.Start()

	ok := g.Enqueue(1, 2, 3)
	assert.True(t, ok)
	assert.Equal(t, 3, g.PendingCount())

	ok = g.Enqueue(4)
	assert.True(t, ok)
	assert.Equal(t, 4, g.PendingCount())
}

func TestFlushGate_EndDrainsAndDeactivates(t *testing.T) {
	var g FlushGate[string]
	g.Start()
	g.Enqueue("a", "b", "c")

	items := g.End()
	require.Equal(t, []string{"a", "b", "c"}, items)
	assert.False(t, g.Active())
	assert.Equal(t, 0, g.PendingCount())
}

func TestFlushGate_EndWhenEmpty(t *testing.T) {
	var g FlushGate[int]
	g.Start()

	items := g.End()
	assert.Empty(t, items)
	assert.False(t, g.Active())
}

func TestFlushGate_DropDiscardsAndReturnsCount(t *testing.T) {
	var g FlushGate[string]
	g.Start()
	g.Enqueue("x", "y")

	count := g.Drop()
	assert.Equal(t, 2, count)
	assert.False(t, g.Active())
	assert.Equal(t, 0, g.PendingCount())
}

func TestFlushGate_DropWhenEmpty(t *testing.T) {
	var g FlushGate[int]
	g.Start()

	count := g.Drop()
	assert.Equal(t, 0, count)
	assert.False(t, g.Active())
}

func TestFlushGate_DeactivatePreservesItems(t *testing.T) {
	var g FlushGate[string]
	g.Start()
	g.Enqueue("a", "b")

	g.Deactivate()
	assert.False(t, g.Active())
	assert.Equal(t, 2, g.PendingCount(), "deactivate should preserve queued items")
}

func TestFlushGate_DeactivateThenEnqueueReturnsFalse(t *testing.T) {
	var g FlushGate[string]
	g.Start()
	g.Enqueue("a")
	g.Deactivate()

	ok := g.Enqueue("b")
	assert.False(t, ok, "enqueue after deactivate should return false")
	// "a" is still pending from before deactivate
	assert.Equal(t, 1, g.PendingCount())
}

func TestFlushGate_RestartAfterEnd(t *testing.T) {
	var g FlushGate[int]
	g.Start()
	g.Enqueue(1)
	g.End()

	// Restart a new flush cycle
	g.Start()
	assert.True(t, g.Active())
	g.Enqueue(2, 3)
	items := g.End()
	require.Equal(t, []int{2, 3}, items)
}

func TestFlushGate_FullLifecycle(t *testing.T) {
	var g FlushGate[string]

	// Inactive: enqueue fails
	assert.False(t, g.Enqueue("pre"))

	// Start flush
	g.Start()
	assert.True(t, g.Enqueue("a"))
	assert.True(t, g.Enqueue("b", "c"))
	assert.Equal(t, 3, g.PendingCount())

	// End flush: drain
	items := g.End()
	assert.Equal(t, []string{"a", "b", "c"}, items)
	assert.False(t, g.Active())
	assert.Equal(t, 0, g.PendingCount())

	// Second flush cycle with drop
	g.Start()
	g.Enqueue("d")
	count := g.Drop()
	assert.Equal(t, 1, count)
	assert.False(t, g.Active())
}
