// Package vim implements vim-style motions, operators, and text objects for
// the TUI input component. It is a pure-Go port of src/vim/ from the TS
// codebase. All functions are pure — they accept state and return new state
// without side-effects.
//
// Tasks: T103 (motions), T104 (operators), T105 (text objects), T107 (types)
package vim

// Operator is a vim editing operation.
type Operator string

const (
	OpDelete Operator = "delete"
	OpChange Operator = "change"
	OpYank   Operator = "yank"
)

// FindType is a character-find motion (f/F/t/T).
type FindType string

const (
	FindF FindType = "f"
	FindB FindType = "F"
	FindT FindType = "t"
	FindR FindType = "T"
)

// TextObjScope indicates inner vs. around for text objects.
type TextObjScope string

const (
	ScopeInner  TextObjScope = "inner"
	ScopeAround TextObjScope = "around"
)

// Mode is the current vim editing mode.
type Mode string

const (
	ModeInsert Mode = "INSERT"
	ModeNormal Mode = "NORMAL"
)

// CommandType identifies the sub-state of a NORMAL mode command being parsed.
type CommandType string

const (
	CmdIdle           CommandType = "idle"
	CmdCount          CommandType = "count"
	CmdOperator       CommandType = "operator"
	CmdOperatorCount  CommandType = "operatorCount"
	CmdOperatorFind   CommandType = "operatorFind"
	CmdOperatorTextObj CommandType = "operatorTextObj"
	CmdFind           CommandType = "find"
	CmdG              CommandType = "g"
	CmdOperatorG      CommandType = "operatorG"
	CmdReplace        CommandType = "replace"
	CmdIndent         CommandType = "indent"
)

// CommandState tracks what input NORMAL mode is waiting for.
type CommandState struct {
	Type   CommandType
	Op     Operator     // set for operator/operatorCount/operatorFind/operatorTextObj/operatorG
	Count  int          // accumulated count
	Digits string       // partial digit accumulation
	Find   FindType     // set for find/operatorFind
	Scope  TextObjScope // set for operatorTextObj
	Dir    rune         // '>' or '<' for indent
}

// RecordedChange captures a change for dot-repeat.
type RecordedChange struct {
	Type      string       // "insert","operator","operatorTextObj","operatorFind","replace","x","toggleCase","indent","openLine","join"
	Op        Operator     // for operator-based changes
	Motion    string       // for operator+motion
	Count     int          // repeat count
	ObjType   string       // for operatorTextObj
	Scope     TextObjScope // for operatorTextObj
	Find      FindType     // for operatorFind
	Char      string       // for operatorFind / replace
	Dir       rune         // for indent
	Direction string       // for openLine ("above"/"below")
	Text      string       // for insert
}

// PersistentState survives across commands — the vim "memory".
type PersistentState struct {
	LastChange       *RecordedChange
	LastFind         *LastFindState
	Register         string
	RegisterLinewise bool
}

// LastFindState records the most recent f/F/t/T find.
type LastFindState struct {
	Type FindType
	Char string
}

// OperatorContext provides the callbacks an operator needs to modify text.
type OperatorContext struct {
	Cursor      int    // rune offset
	Text        string // full buffer as string
	SetText     func(string)
	SetOffset   func(int)
	EnterInsert func(int)
	GetRegister func() string
	SetRegister func(content string, linewise bool)
}

// TextObjectRange is a half-open [Start, End) range in the text.
type TextObjectRange struct {
	Start int
	End   int
}

// --- Constants matching TS ---

// Operators maps operator keys to Operator values.
var Operators = map[rune]Operator{
	'd': OpDelete,
	'c': OpChange,
	'y': OpYank,
}

// SimpleMotions is the set of single-key motions.
var SimpleMotions = map[rune]bool{
	'h': true, 'l': true, 'j': true, 'k': true,
	'w': true, 'b': true, 'e': true,
	'W': true, 'B': true, 'E': true,
	'0': true, '^': true, '$': true,
}

// FindKeys is the set of character-find keys.
var FindKeys = map[rune]FindType{
	'f': FindF,
	'F': FindB,
	't': FindT,
	'T': FindR,
}

// TextObjScopes maps i/a to scope.
var TextObjScopes = map[rune]TextObjScope{
	'i': ScopeInner,
	'a': ScopeAround,
}

// TextObjTypes is the set of valid text object type characters.
var TextObjTypes = map[rune]bool{
	'w': true, 'W': true,
	'"': true, '\'': true, '`': true,
	'(': true, ')': true, 'b': true,
	'[': true, ']': true,
	'{': true, '}': true, 'B': true,
	'<': true, '>': true,
}

// MaxVimCount caps the repeat count to prevent runaway loops.
const MaxVimCount = 10000

// NewPersistentState returns a zero-value persistent state.
func NewPersistentState() PersistentState {
	return PersistentState{}
}

// IdleCommand returns the idle command state.
func IdleCommand() CommandState {
	return CommandState{Type: CmdIdle, Count: 1}
}
