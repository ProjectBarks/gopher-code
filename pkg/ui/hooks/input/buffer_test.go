package input

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInputBuffer_SetValueAndGet(t *testing.T) {
	b := NewInputBuffer()
	b.SetValue("hello")
	assert.Equal(t, "hello", b.Value())
	assert.Equal(t, 5, b.Cursor())
}

func TestInputBuffer_Clear(t *testing.T) {
	b := NewInputBuffer()
	b.SetValue("hello")
	b.Clear()
	assert.Equal(t, "", b.Value())
	assert.Equal(t, 0, b.Cursor())
	assert.True(t, b.IsEmpty())
}

func TestInputBuffer_Insert(t *testing.T) {
	b := NewInputBuffer()
	b.SetValue("hllo")
	b.SetCursor(1)
	b.Insert("e")
	assert.Equal(t, "hello", b.Value())
	assert.Equal(t, 2, b.Cursor())
}

func TestInputBuffer_InsertAtEnd(t *testing.T) {
	b := NewInputBuffer()
	b.SetValue("hel")
	b.Insert("lo")
	assert.Equal(t, "hello", b.Value())
	assert.Equal(t, 5, b.Cursor())
}

func TestInputBuffer_InsertUnicode(t *testing.T) {
	b := NewInputBuffer()
	b.Insert("日本")
	assert.Equal(t, "日本", b.Value())
	assert.Equal(t, 2, b.Cursor()) // 2 runes
	assert.Equal(t, 2, b.Len())
}

func TestInputBuffer_DeleteBackward(t *testing.T) {
	b := NewInputBuffer()
	b.SetValue("abc")
	ok := b.DeleteBackward()
	assert.True(t, ok)
	assert.Equal(t, "ab", b.Value())
	assert.Equal(t, 2, b.Cursor())
}

func TestInputBuffer_DeleteBackwardEmpty(t *testing.T) {
	b := NewInputBuffer()
	ok := b.DeleteBackward()
	assert.False(t, ok)
}

func TestInputBuffer_DeleteForward(t *testing.T) {
	b := NewInputBuffer()
	b.SetValue("abc")
	b.SetCursor(1)
	ok := b.DeleteForward()
	assert.True(t, ok)
	assert.Equal(t, "ac", b.Value())
	assert.Equal(t, 1, b.Cursor())
}

func TestInputBuffer_DeleteForwardAtEnd(t *testing.T) {
	b := NewInputBuffer()
	b.SetValue("abc")
	ok := b.DeleteForward()
	assert.False(t, ok)
}

func TestInputBuffer_DeleteWordBackward(t *testing.T) {
	b := NewInputBuffer()
	b.SetValue("hello world")
	ok := b.DeleteWordBackward()
	assert.True(t, ok)
	assert.Equal(t, "hello ", b.Value())
}

func TestInputBuffer_DeleteWordBackwardMultipleSpaces(t *testing.T) {
	b := NewInputBuffer()
	b.SetValue("hello   world")
	b.SetCursor(8) // After "hello   "
	ok := b.DeleteWordBackward()
	assert.True(t, ok)
	assert.Equal(t, "world", b.Value())
}

func TestInputBuffer_DeleteWordBackwardAtStart(t *testing.T) {
	b := NewInputBuffer()
	b.SetValue("hello")
	b.SetCursor(0)
	ok := b.DeleteWordBackward()
	assert.False(t, ok)
}

func TestInputBuffer_KillToEnd(t *testing.T) {
	b := NewInputBuffer()
	b.SetValue("hello world")
	b.SetCursor(5)
	b.KillToEnd()
	assert.Equal(t, "hello", b.Value())
}

func TestInputBuffer_KillToStart(t *testing.T) {
	b := NewInputBuffer()
	b.SetValue("hello world")
	b.SetCursor(5)
	b.KillToStart()
	assert.Equal(t, " world", b.Value())
	assert.Equal(t, 0, b.Cursor())
}

func TestInputBuffer_Movement(t *testing.T) {
	b := NewInputBuffer()
	b.SetValue("abc")

	b.MoveLeft()
	assert.Equal(t, 2, b.Cursor())

	b.MoveRight()
	assert.Equal(t, 3, b.Cursor())

	b.MoveToStart()
	assert.Equal(t, 0, b.Cursor())

	b.MoveToEnd()
	assert.Equal(t, 3, b.Cursor())
}

func TestInputBuffer_MoveLeftAtStart(t *testing.T) {
	b := NewInputBuffer()
	b.SetValue("abc")
	b.SetCursor(0)
	b.MoveLeft()
	assert.Equal(t, 0, b.Cursor())
}

func TestInputBuffer_MoveRightAtEnd(t *testing.T) {
	b := NewInputBuffer()
	b.SetValue("abc")
	b.MoveRight()
	assert.Equal(t, 3, b.Cursor())
}

func TestInputBuffer_SetCursorClamped(t *testing.T) {
	b := NewInputBuffer()
	b.SetValue("abc")

	b.SetCursor(-5)
	assert.Equal(t, 0, b.Cursor())

	b.SetCursor(100)
	assert.Equal(t, 3, b.Cursor())
}

func TestInputBuffer_ConcurrentAccess(t *testing.T) {
	b := NewInputBuffer()
	done := make(chan struct{})

	go func() {
		for i := 0; i < 100; i++ {
			b.Insert("x")
		}
		close(done)
	}()

	for i := 0; i < 100; i++ {
		b.Value()
		b.Len()
		b.Cursor()
	}
	<-done
}
