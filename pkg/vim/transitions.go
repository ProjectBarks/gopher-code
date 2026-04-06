package vim

import (
	"strconv"
	"strings"
)

// TransitionContext provides the callbacks a transition needs to modify text
// and interact with the editor.
//
// Source: src/vim/transitions.ts — TransitionContext
type TransitionContext struct {
	OperatorContext

	// OnUndo is called for the 'u' key.
	OnUndo func()
	// OnDotRepeat is called for the '.' key.
	OnDotRepeat func()
	// GetLastFind retrieves the most recent f/F/t/T find state.
	GetLastFind func() *LastFindState
	// SetLastFind records a find for ; and , repeat.
	SetLastFind func(ft FindType, char string)
}

// TransitionResult is the outcome of processing a key in normal mode.
//
// Source: src/vim/transitions.ts — TransitionResult
type TransitionResult struct {
	// Next is the new command state (nil means return to idle).
	Next *CommandState
	// Execute is an action to run (may be nil).
	Execute func()
}

// Transition dispatches a key press based on the current command state.
// This is the main entry point for the vim normal-mode state machine.
//
// Source: src/vim/transitions.ts — transition()
func Transition(state CommandState, input string, ctx *TransitionContext) TransitionResult {
	switch state.Type {
	case CmdIdle:
		return fromIdle(input, ctx)
	case CmdCount:
		return fromCount(state, input, ctx)
	case CmdOperator:
		return fromOperator(state, input, ctx)
	case CmdOperatorCount:
		return fromOperatorCount(state, input, ctx)
	case CmdOperatorFind:
		return fromOperatorFind(state, input, ctx)
	case CmdOperatorTextObj:
		return fromOperatorTextObj(state, input, ctx)
	case CmdFind:
		return fromFind(state, input, ctx)
	case CmdG:
		return fromG(state, input, ctx)
	case CmdOperatorG:
		return fromOperatorG(state, input, ctx)
	case CmdReplace:
		return fromReplace(state, input, ctx)
	case CmdIndent:
		return fromIndent(state, input, ctx)
	default:
		return TransitionResult{}
	}
}

// --- shared input handlers ---

// handleNormalInput handles input valid in both idle and count states.
// Returns nil if input is not recognized.
func handleNormalInput(input string, count int, ctx *TransitionContext) *TransitionResult {
	r := firstRune(input)

	// Operator keys: d, c, y
	if op, ok := Operators[r]; ok {
		return &TransitionResult{Next: &CommandState{Type: CmdOperator, Op: op, Count: count}}
	}

	// Simple motions
	if SimpleMotions[r] {
		return &TransitionResult{Execute: func() {
			target := ResolveMotion(input, ctx.Text, ctx.Cursor, count)
			ctx.SetOffset(target)
		}}
	}

	// Find keys: f, F, t, T
	if ft, ok := FindKeys[r]; ok {
		return &TransitionResult{Next: &CommandState{Type: CmdFind, Find: ft, Count: count}}
	}

	switch input {
	case "g":
		return &TransitionResult{Next: &CommandState{Type: CmdG, Count: count}}
	case "r":
		return &TransitionResult{Next: &CommandState{Type: CmdReplace, Count: count}}
	case ">", "<":
		return &TransitionResult{Next: &CommandState{Type: CmdIndent, Dir: r, Count: count}}
	case "~":
		return &TransitionResult{Execute: func() { ExecuteToggleCase(count, &ctx.OperatorContext) }}
	case "x":
		return &TransitionResult{Execute: func() { ExecuteX(count, &ctx.OperatorContext) }}
	case "J":
		return &TransitionResult{Execute: func() { ExecuteJoin(count, &ctx.OperatorContext) }}
	case "p":
		return &TransitionResult{Execute: func() { ExecutePaste(true, count, &ctx.OperatorContext) }}
	case "P":
		return &TransitionResult{Execute: func() { ExecutePaste(false, count, &ctx.OperatorContext) }}
	case "D":
		return &TransitionResult{Execute: func() { ExecuteOperatorMotion(OpDelete, "$", 1, &ctx.OperatorContext) }}
	case "C":
		return &TransitionResult{Execute: func() { ExecuteOperatorMotion(OpChange, "$", 1, &ctx.OperatorContext) }}
	case "Y":
		return &TransitionResult{Execute: func() { ExecuteLineOp(OpYank, count, &ctx.OperatorContext) }}
	case "G":
		return &TransitionResult{Execute: func() {
			if count == 1 {
				runes := []rune(ctx.Text)
				ctx.SetOffset(motionLastLine(runes))
			} else {
				ctx.SetOffset(GoToLine(ctx.Text, count))
			}
		}}
	case ".":
		return &TransitionResult{Execute: func() {
			if ctx.OnDotRepeat != nil {
				ctx.OnDotRepeat()
			}
		}}
	case ";":
		return &TransitionResult{Execute: func() { executeRepeatFind(false, count, ctx) }}
	case ",":
		return &TransitionResult{Execute: func() { executeRepeatFind(true, count, ctx) }}
	case "u":
		return &TransitionResult{Execute: func() {
			if ctx.OnUndo != nil {
				ctx.OnUndo()
			}
		}}
	case "i":
		return &TransitionResult{Execute: func() { ctx.EnterInsert(ctx.Cursor) }}
	case "I":
		return &TransitionResult{Execute: func() {
			runes := []rune(ctx.Text)
			ctx.EnterInsert(motionFirstNonBlank(runes, ctx.Cursor))
		}}
	case "a":
		return &TransitionResult{Execute: func() {
			runes := []rune(ctx.Text)
			newOffset := ctx.Cursor
			if ctx.Cursor < len(runes)-1 {
				newOffset = ctx.Cursor + 1
			}
			ctx.EnterInsert(newOffset)
		}}
	case "A":
		return &TransitionResult{Execute: func() {
			runes := []rune(ctx.Text)
			ctx.EnterInsert(motionEndOfLine(runes, ctx.Cursor))
		}}
	case "o":
		return &TransitionResult{Execute: func() { ExecuteOpenLine("below", &ctx.OperatorContext) }}
	case "O":
		return &TransitionResult{Execute: func() { ExecuteOpenLine("above", &ctx.OperatorContext) }}
	case "s":
		return &TransitionResult{Execute: func() {
			// s = delete char then enter insert (like cl)
			runes := []rune(ctx.Text)
			if ctx.Cursor < len(runes) {
				end := ctx.Cursor + count
				if end > len(runes) {
					end = len(runes)
				}
				deleted := string(runes[ctx.Cursor:end])
				ctx.SetRegister(deleted, false)
				newText := string(runes[:ctx.Cursor]) + string(runes[end:])
				ctx.SetText(newText)
			}
			ctx.EnterInsert(ctx.Cursor)
		}}
	case "S":
		return &TransitionResult{Execute: func() {
			// S = change whole line (like cc)
			ExecuteLineOp(OpChange, count, &ctx.OperatorContext)
		}}
	}

	return nil
}

// handleOperatorInput handles input after an operator key (d/c/y).
// Returns nil if input is not recognized.
func handleOperatorInput(op Operator, count int, input string, ctx *TransitionContext) *TransitionResult {
	r := firstRune(input)

	// Text object scope: i/a
	if scope, ok := TextObjScopes[r]; ok {
		return &TransitionResult{Next: &CommandState{
			Type: CmdOperatorTextObj, Op: op, Count: count, Scope: scope,
		}}
	}

	// Find keys
	if ft, ok := FindKeys[r]; ok {
		return &TransitionResult{Next: &CommandState{
			Type: CmdOperatorFind, Op: op, Count: count, Find: ft,
		}}
	}

	// Simple motions
	if SimpleMotions[r] {
		return &TransitionResult{Execute: func() {
			ExecuteOperatorMotion(op, input, count, &ctx.OperatorContext)
		}}
	}

	if input == "G" {
		return &TransitionResult{Execute: func() {
			ExecuteOperatorG(op, count, &ctx.OperatorContext)
		}}
	}

	if input == "g" {
		return &TransitionResult{Next: &CommandState{Type: CmdOperatorG, Op: op, Count: count}}
	}

	return nil
}

// --- per-state transition functions ---

func fromIdle(input string, ctx *TransitionContext) TransitionResult {
	// 0 is line-start motion in idle, not a count prefix
	if len(input) == 1 && input[0] >= '1' && input[0] <= '9' {
		return TransitionResult{Next: &CommandState{Type: CmdCount, Digits: input}}
	}
	if input == "0" {
		return TransitionResult{Execute: func() {
			runes := []rune(ctx.Text)
			ctx.SetOffset(motionStartOfLine(runes, ctx.Cursor))
		}}
	}

	if r := handleNormalInput(input, 1, ctx); r != nil {
		return *r
	}
	return TransitionResult{}
}

func fromCount(state CommandState, input string, ctx *TransitionContext) TransitionResult {
	if len(input) == 1 && input[0] >= '0' && input[0] <= '9' {
		newDigits := state.Digits + input
		n, _ := strconv.Atoi(newDigits)
		if n > MaxVimCount {
			n = MaxVimCount
		}
		return TransitionResult{Next: &CommandState{Type: CmdCount, Digits: strconv.Itoa(n)}}
	}

	count, _ := strconv.Atoi(state.Digits)
	if r := handleNormalInput(input, count, ctx); r != nil {
		return *r
	}
	return TransitionResult{Next: &CommandState{Type: CmdIdle, Count: 1}}
}

func fromOperator(state CommandState, input string, ctx *TransitionContext) TransitionResult {
	// dd, cc, yy = line operation
	if len(state.Op) > 0 && input == string(state.Op[0]) {
		return TransitionResult{Execute: func() {
			ExecuteLineOp(state.Op, state.Count, &ctx.OperatorContext)
		}}
	}

	if len(input) == 1 && input[0] >= '0' && input[0] <= '9' {
		return TransitionResult{Next: &CommandState{
			Type: CmdOperatorCount, Op: state.Op, Count: state.Count, Digits: input,
		}}
	}

	if r := handleOperatorInput(state.Op, state.Count, input, ctx); r != nil {
		return *r
	}
	return TransitionResult{Next: &CommandState{Type: CmdIdle, Count: 1}}
}

func fromOperatorCount(state CommandState, input string, ctx *TransitionContext) TransitionResult {
	if len(input) == 1 && input[0] >= '0' && input[0] <= '9' {
		newDigits := state.Digits + input
		n, _ := strconv.Atoi(newDigits)
		if n > MaxVimCount {
			n = MaxVimCount
		}
		return TransitionResult{Next: &CommandState{
			Type: CmdOperatorCount, Op: state.Op, Count: state.Count, Digits: strconv.Itoa(n),
		}}
	}

	motionCount, _ := strconv.Atoi(state.Digits)
	effectiveCount := state.Count * motionCount
	if r := handleOperatorInput(state.Op, effectiveCount, input, ctx); r != nil {
		return *r
	}
	return TransitionResult{Next: &CommandState{Type: CmdIdle, Count: 1}}
}

func fromOperatorFind(state CommandState, input string, ctx *TransitionContext) TransitionResult {
	return TransitionResult{Execute: func() {
		ExecuteOperatorFind(state.Op, state.Find, firstRune(input), state.Count, &ctx.OperatorContext)
	}}
}

func fromOperatorTextObj(state CommandState, input string, ctx *TransitionContext) TransitionResult {
	r := firstRune(input)
	if TextObjTypes[r] {
		return TransitionResult{Execute: func() {
			ExecuteOperatorTextObj(state.Op, state.Scope, r, &ctx.OperatorContext)
		}}
	}
	return TransitionResult{Next: &CommandState{Type: CmdIdle, Count: 1}}
}

func fromFind(state CommandState, input string, ctx *TransitionContext) TransitionResult {
	return TransitionResult{Execute: func() {
		r := firstRune(input)
		target := FindCharacter(ctx.Text, ctx.Cursor, r, state.Find, state.Count)
		if target != -1 {
			ctx.SetOffset(target)
			if ctx.SetLastFind != nil {
				ctx.SetLastFind(state.Find, input)
			}
		}
	}}
}

func fromG(state CommandState, input string, ctx *TransitionContext) TransitionResult {
	if input == "j" || input == "k" {
		return TransitionResult{Execute: func() {
			target := ResolveMotion("g"+input, ctx.Text, ctx.Cursor, state.Count)
			ctx.SetOffset(target)
		}}
	}
	if input == "g" {
		if state.Count > 1 {
			return TransitionResult{Execute: func() {
				lines := strings.Split(ctx.Text, "\n")
				targetLine := state.Count - 1
				if targetLine >= len(lines) {
					targetLine = len(lines) - 1
				}
				offset := 0
				for i := 0; i < targetLine; i++ {
					offset += len([]rune(lines[i])) + 1
				}
				ctx.SetOffset(offset)
			}}
		}
		return TransitionResult{Execute: func() {
			ctx.SetOffset(0) // start of first line
		}}
	}
	return TransitionResult{Next: &CommandState{Type: CmdIdle, Count: 1}}
}

func fromOperatorG(state CommandState, input string, ctx *TransitionContext) TransitionResult {
	if input == "j" || input == "k" {
		return TransitionResult{Execute: func() {
			ExecuteOperatorMotion(state.Op, "g"+input, state.Count, &ctx.OperatorContext)
		}}
	}
	if input == "g" {
		return TransitionResult{Execute: func() {
			ExecuteOperatorGg(state.Op, state.Count, &ctx.OperatorContext)
		}}
	}
	return TransitionResult{Next: &CommandState{Type: CmdIdle, Count: 1}}
}

func fromReplace(state CommandState, input string, ctx *TransitionContext) TransitionResult {
	if input == "" {
		return TransitionResult{Next: &CommandState{Type: CmdIdle, Count: 1}}
	}
	return TransitionResult{Execute: func() {
		ExecuteReplace(firstRune(input), state.Count, &ctx.OperatorContext)
	}}
}

func fromIndent(state CommandState, input string, ctx *TransitionContext) TransitionResult {
	if firstRune(input) == state.Dir {
		return TransitionResult{Execute: func() {
			ExecuteIndent(state.Dir, state.Count, &ctx.OperatorContext)
		}}
	}
	return TransitionResult{Next: &CommandState{Type: CmdIdle, Count: 1}}
}

// --- helpers ---

func executeRepeatFind(reverse bool, count int, ctx *TransitionContext) {
	if ctx.GetLastFind == nil {
		return
	}
	last := ctx.GetLastFind()
	if last == nil {
		return
	}

	ft := last.Type
	if reverse {
		switch ft {
		case FindF:
			ft = FindB
		case FindB:
			ft = FindF
		case FindT:
			ft = FindR
		case FindR:
			ft = FindT
		}
	}

	target := FindCharacter(ctx.Text, ctx.Cursor, firstRune(last.Char), ft, count)
	if target != -1 {
		ctx.SetOffset(target)
	}
}

func firstRune(s string) rune {
	for _, r := range s {
		return r
	}
	return 0
}
