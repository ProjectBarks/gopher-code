package vim

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestCtx builds a TransitionContext over the given text at the given cursor.
// It returns the context plus getters for the final offset and text.
func newTestCtx(text string, cursor int) (ctx *TransitionContext, getOffset func() int, getText func() string, insertedAt func() int) {
	offset := cursor
	insertPos := -1
	reg := ""
	regLinewise := false
	var lastFind *LastFindState

	ctx = &TransitionContext{
		OperatorContext: OperatorContext{
			Cursor:      cursor,
			Text:        text,
			SetText:     func(s string) { ctx.Text = s },
			SetOffset:   func(o int) { offset = o },
			EnterInsert: func(o int) { insertPos = o },
			GetRegister: func() string { return reg },
			SetRegister: func(c string, lw bool) { reg = c; regLinewise = lw },
		},
		GetLastFind: func() *LastFindState { return lastFind },
		SetLastFind: func(ft FindType, ch string) { lastFind = &LastFindState{Type: ft, Char: ch} },
	}
	_ = regLinewise
	return ctx, func() int { return offset }, func() string { return ctx.Text }, func() int { return insertPos }
}

func TestTransition_IdleToCount(t *testing.T) {
	ctx, _, _, _ := newTestCtx("hello", 0)
	r := Transition(IdleCommand(), "3", ctx)
	require.NotNil(t, r.Next)
	assert.Equal(t, CmdCount, r.Next.Type)
	assert.Equal(t, "3", r.Next.Digits)
}

func TestTransition_CountToMotion(t *testing.T) {
	ctx, getOff, _, _ := newTestCtx("hello world foo", 0)
	// Build up count "2"
	r := Transition(IdleCommand(), "2", ctx)
	require.NotNil(t, r.Next)
	// Now press 'w' — should move forward 2 words
	r2 := Transition(*r.Next, "w", ctx)
	require.NotNil(t, r2.Execute)
	r2.Execute()
	assert.Equal(t, len([]rune("hello world ")), getOff()) // start of "foo"
}

func TestTransition_IdleOperator_dd(t *testing.T) {
	ctx, getOff, getText, _ := newTestCtx("line1\nline2\nline3", 0)
	r := Transition(IdleCommand(), "d", ctx)
	require.NotNil(t, r.Next)
	assert.Equal(t, CmdOperator, r.Next.Type)
	assert.Equal(t, OpDelete, r.Next.Op)

	r2 := Transition(*r.Next, "d", ctx)
	require.NotNil(t, r2.Execute)
	r2.Execute()
	assert.Equal(t, "line2\nline3", getText())
	assert.Equal(t, 0, getOff())
}

func TestTransition_InsertModeKeys(t *testing.T) {
	tests := []struct {
		key      string
		text     string
		cursor   int
		wantPos  int // expected insert position
	}{
		{"i", "hello", 2, 2},
		{"a", "hello", 2, 3},
		{"A", "hello\nworld", 2, 4},    // end of first line
		{"I", "  hello", 2, 2},          // first non-blank
		{"o", "hello", 0, -1},           // open line triggers EnterInsert via ExecuteOpenLine
		{"O", "hello", 0, -1},           // open line above
	}
	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			ctx, _, _, insertAt := newTestCtx(tt.text, tt.cursor)
			r := Transition(IdleCommand(), tt.key, ctx)
			require.NotNil(t, r.Execute, "key %q should produce an execute", tt.key)
			r.Execute()
			if tt.wantPos >= 0 {
				assert.Equal(t, tt.wantPos, insertAt(), "insert position for key %q", tt.key)
			} else {
				// For o/O, just verify EnterInsert was called
				assert.GreaterOrEqual(t, insertAt(), 0, "EnterInsert should have been called for %q", tt.key)
			}
		})
	}
}

func TestTransition_FindAndRepeat(t *testing.T) {
	ctx, getOff, _, _ := newTestCtx("abcdefg", 0)

	// f -> wait for char
	r := Transition(IdleCommand(), "f", ctx)
	require.NotNil(t, r.Next)
	assert.Equal(t, CmdFind, r.Next.Type)

	// press 'd' — should move to 'd' at index 3
	r2 := Transition(*r.Next, "d", ctx)
	require.NotNil(t, r2.Execute)
	r2.Execute()
	assert.Equal(t, 3, getOff())

	// Now ; should repeat the find
	ctx.Cursor = getOff()
	r3 := Transition(IdleCommand(), ";", ctx)
	require.NotNil(t, r3.Execute)
	// No second 'd' ahead, so offset stays
	r3.Execute()
}

func TestTransition_Replace(t *testing.T) {
	ctx, getOff, getText, _ := newTestCtx("hello", 1)
	r := Transition(IdleCommand(), "r", ctx)
	require.NotNil(t, r.Next)
	assert.Equal(t, CmdReplace, r.Next.Type)

	r2 := Transition(*r.Next, "X", ctx)
	require.NotNil(t, r2.Execute)
	r2.Execute()
	assert.Equal(t, "hXllo", getText())
	assert.Equal(t, 1, getOff())
}

func TestTransition_ReplaceEmptyInput(t *testing.T) {
	ctx, _, _, _ := newTestCtx("hello", 1)
	r := Transition(CommandState{Type: CmdReplace, Count: 1}, "", ctx)
	require.NotNil(t, r.Next)
	assert.Equal(t, CmdIdle, r.Next.Type, "empty input in replace should cancel")
}

func TestTransition_Indent(t *testing.T) {
	ctx, _, getText, _ := newTestCtx("hello\nworld", 0)
	r := Transition(IdleCommand(), ">", ctx)
	require.NotNil(t, r.Next)
	assert.Equal(t, CmdIndent, r.Next.Type)

	r2 := Transition(*r.Next, ">", ctx)
	require.NotNil(t, r2.Execute)
	r2.Execute()
	assert.Equal(t, "  hello\nworld", getText())
}

func TestTransition_IndentWrongKey(t *testing.T) {
	ctx, _, _, _ := newTestCtx("hello", 0)
	r := Transition(CommandState{Type: CmdIndent, Dir: '>', Count: 1}, "<", ctx)
	require.NotNil(t, r.Next)
	assert.Equal(t, CmdIdle, r.Next.Type, "wrong indent key should cancel")
}

func TestTransition_G_GoToLastLine(t *testing.T) {
	ctx, getOff, _, _ := newTestCtx("line1\nline2\nline3", 0)
	r := Transition(IdleCommand(), "G", ctx)
	require.NotNil(t, r.Execute)
	r.Execute()
	assert.Equal(t, 12, getOff()) // start of "line3"
}

func TestTransition_gg_GoToFirstLine(t *testing.T) {
	ctx, getOff, _, _ := newTestCtx("line1\nline2\nline3", 12)
	r := Transition(IdleCommand(), "g", ctx)
	require.NotNil(t, r.Next)
	r2 := Transition(*r.Next, "g", ctx)
	require.NotNil(t, r2.Execute)
	r2.Execute()
	assert.Equal(t, 0, getOff())
}

func TestTransition_OperatorTextObj(t *testing.T) {
	ctx, _, getText, _ := newTestCtx(`hello "world" end`, 8)
	// d -> i -> " = delete inner quotes
	r := Transition(IdleCommand(), "d", ctx)
	require.NotNil(t, r.Next)
	r2 := Transition(*r.Next, "i", ctx)
	require.NotNil(t, r2.Next)
	assert.Equal(t, CmdOperatorTextObj, r2.Next.Type)
	r3 := Transition(*r2.Next, "\"", ctx)
	require.NotNil(t, r3.Execute)
	r3.Execute()
	assert.Equal(t, `hello "" end`, getText())
}

func TestTransition_CountOverflow(t *testing.T) {
	ctx, _, _, _ := newTestCtx("hello", 0)
	// Accumulate digits that exceed MaxVimCount
	state := CommandState{Type: CmdCount, Digits: "9999"}
	r := Transition(state, "9", ctx)
	require.NotNil(t, r.Next)
	assert.Equal(t, CmdCount, r.Next.Type)
	// Should be capped at MaxVimCount
	assert.Equal(t, "10000", r.Next.Digits)
}

func TestTransition_Undo(t *testing.T) {
	undoCalled := false
	ctx, _, _, _ := newTestCtx("hello", 0)
	ctx.OnUndo = func() { undoCalled = true }
	r := Transition(IdleCommand(), "u", ctx)
	require.NotNil(t, r.Execute)
	r.Execute()
	assert.True(t, undoCalled)
}

func TestTransition_DotRepeat(t *testing.T) {
	dotCalled := false
	ctx, _, _, _ := newTestCtx("hello", 0)
	ctx.OnDotRepeat = func() { dotCalled = true }
	r := Transition(IdleCommand(), ".", ctx)
	require.NotNil(t, r.Execute)
	r.Execute()
	assert.True(t, dotCalled)
}

func TestTransition_D_DeleteToEnd(t *testing.T) {
	ctx, _, getText, _ := newTestCtx("hello world", 5)
	r := Transition(IdleCommand(), "D", ctx)
	require.NotNil(t, r.Execute)
	r.Execute()
	assert.Equal(t, "hello", getText())
}

func TestTransition_OperatorCountMultiplied(t *testing.T) {
	// 2d3w = delete 6 words worth
	ctx, _, _, _ := newTestCtx("a b c d e f g h", 0)
	r := Transition(IdleCommand(), "2", ctx)
	require.NotNil(t, r.Next)
	r2 := Transition(*r.Next, "d", ctx)
	require.NotNil(t, r2.Next)
	assert.Equal(t, CmdOperator, r2.Next.Type)
	assert.Equal(t, 2, r2.Next.Count)

	// Now press '3'
	r3 := Transition(*r2.Next, "3", ctx)
	require.NotNil(t, r3.Next)
	assert.Equal(t, CmdOperatorCount, r3.Next.Type)

	// Now press 'w'
	r4 := Transition(*r3.Next, "w", ctx)
	require.NotNil(t, r4.Execute)
}

func TestTransition_UnrecognizedInput(t *testing.T) {
	ctx, _, _, _ := newTestCtx("hello", 0)
	r := Transition(IdleCommand(), "Z", ctx)
	assert.Nil(t, r.Next)
	assert.Nil(t, r.Execute)
}

func TestTransition_S_ChangeWholeLine(t *testing.T) {
	ctx, _, _, insertAt := newTestCtx("hello\nworld", 2)
	r := Transition(IdleCommand(), "S", ctx)
	require.NotNil(t, r.Execute)
	r.Execute()
	assert.GreaterOrEqual(t, insertAt(), 0)
}

func TestTransition_s_DeleteCharEnterInsert(t *testing.T) {
	ctx, _, getText, insertAt := newTestCtx("hello", 1)
	r := Transition(IdleCommand(), "s", ctx)
	require.NotNil(t, r.Execute)
	r.Execute()
	assert.Equal(t, "hllo", getText())
	assert.Equal(t, 1, insertAt())
}
