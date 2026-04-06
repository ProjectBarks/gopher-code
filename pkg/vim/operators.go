package vim

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// ExecuteOperatorMotion applies operator `op` with motion `motion` repeated
// `count` times. Modifies text via ctx callbacks.
//
// Source: src/vim/operators.ts — executeOperatorMotion
func ExecuteOperatorMotion(op Operator, motion string, count int, ctx *OperatorContext) {
	target := ResolveMotion(motion, ctx.Text, ctx.Cursor, count)
	if target == ctx.Cursor {
		return
	}
	from, to, linewise := getOperatorRange(ctx.Cursor, target, motion, op, count, ctx.Text)
	applyOperator(op, from, to, ctx, linewise)
}

// ExecuteOperatorFind applies operator with a find-motion (f/F/t/T + char).
func ExecuteOperatorFind(op Operator, ft FindType, char rune, count int, ctx *OperatorContext) {
	target := FindCharacter(ctx.Text, ctx.Cursor, char, ft, count)
	if target == -1 {
		return
	}
	from, to := getOperatorRangeForFind(ctx.Cursor, target)
	applyOperator(op, from, to, ctx, false)
}

// ExecuteOperatorTextObj applies operator with a text object.
func ExecuteOperatorTextObj(op Operator, scope TextObjScope, objType rune, ctx *OperatorContext) {
	r := FindTextObject(ctx.Text, ctx.Cursor, objType, scope == ScopeInner)
	if r == nil {
		return
	}
	applyOperator(op, r.Start, r.End, ctx, false)
}

// ExecuteLineOp executes a line operation (dd, cc, yy).
func ExecuteLineOp(op Operator, count int, ctx *OperatorContext) {
	runes := []rune(ctx.Text)
	if len(runes) == 0 {
		return
	}

	// Find current logical line
	currentLine := 0
	for i := 0; i < ctx.Cursor && i < len(runes); i++ {
		if runes[i] == '\n' {
			currentLine++
		}
	}

	lines := strings.Split(ctx.Text, "\n")
	linesToAffect := count
	if linesToAffect > len(lines)-currentLine {
		linesToAffect = len(lines) - currentLine
	}

	lineStart := lineStartOffset(lines, currentLine)
	lineEnd := lineStart
	for i := 0; i < linesToAffect; i++ {
		idx := strings.Index(ctx.Text[lineEnd:], "\n")
		if idx == -1 {
			lineEnd = len(ctx.Text)
		} else {
			lineEnd += idx + 1
		}
	}

	// Rune-based offsets for the content
	runeLineStart := len([]rune(ctx.Text[:lineStart]))
	runeLineEnd := len([]rune(ctx.Text[:lineEnd]))

	content := string(runes[runeLineStart:runeLineEnd])
	if !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	ctx.SetRegister(content, true)

	switch op {
	case OpYank:
		ctx.SetOffset(runeLineStart)
	case OpDelete:
		deleteStart := lineStart
		deleteEnd := lineEnd

		if deleteEnd == len(ctx.Text) && deleteStart > 0 && ctx.Text[deleteStart-1] == '\n' {
			deleteStart--
		}
		newText := ctx.Text[:deleteStart] + ctx.Text[deleteEnd:]
		ctx.SetText(newText)
		newRunes := []rune(newText)
		maxOff := maxOffset(newRunes)
		ds := len([]rune(ctx.Text[:deleteStart]))
		if ds > maxOff {
			ds = maxOff
		}
		ctx.SetOffset(ds)
	case OpChange:
		if len(lines) == 1 {
			ctx.SetText("")
			ctx.EnterInsert(0)
		} else {
			before := lines[:currentLine]
			after := lines[currentLine+linesToAffect:]
			newLines := make([]string, 0, len(before)+1+len(after))
			newLines = append(newLines, before...)
			newLines = append(newLines, "")
			newLines = append(newLines, after...)
			newText := strings.Join(newLines, "\n")
			ctx.SetText(newText)
			ctx.EnterInsert(runeLineStart)
		}
	}
}

// ExecuteX executes the x (delete char) command.
func ExecuteX(count int, ctx *OperatorContext) {
	runes := []rune(ctx.Text)
	if ctx.Cursor >= len(runes) {
		return
	}
	end := ctx.Cursor + count
	if end > len(runes) {
		end = len(runes)
	}
	deleted := string(runes[ctx.Cursor:end])
	newText := string(runes[:ctx.Cursor]) + string(runes[end:])
	ctx.SetRegister(deleted, false)
	ctx.SetText(newText)
	newRunes := []rune(newText)
	maxOff := maxOffset(newRunes)
	off := ctx.Cursor
	if off > maxOff {
		off = maxOff
	}
	ctx.SetOffset(off)
}

// ExecuteReplace executes the r (replace char) command.
func ExecuteReplace(char rune, count int, ctx *OperatorContext) {
	runes := []rune(ctx.Text)
	offset := ctx.Cursor
	for i := 0; i < count && offset < len(runes); i++ {
		runes[offset] = char
		offset++
	}
	ctx.SetText(string(runes))
	if offset > 0 {
		ctx.SetOffset(offset - 1)
	}
}

// ExecuteToggleCase executes the ~ (toggle case) command.
func ExecuteToggleCase(count int, ctx *OperatorContext) {
	runes := []rune(ctx.Text)
	if ctx.Cursor >= len(runes) {
		return
	}
	offset := ctx.Cursor
	toggled := 0
	for offset < len(runes) && toggled < count {
		r := runes[offset]
		if unicode.IsUpper(r) {
			runes[offset] = unicode.ToLower(r)
		} else if unicode.IsLetter(r) {
			runes[offset] = unicode.ToUpper(r)
		}
		offset++
		toggled++
	}
	ctx.SetText(string(runes))
	ctx.SetOffset(offset)
}

// ExecuteJoin executes the J (join lines) command.
func ExecuteJoin(count int, ctx *OperatorContext) {
	lines := strings.Split(ctx.Text, "\n")
	currentLine := 0
	runes := []rune(ctx.Text)
	for i := 0; i < ctx.Cursor && i < len(runes); i++ {
		if runes[i] == '\n' {
			currentLine++
		}
	}
	if currentLine >= len(lines)-1 {
		return
	}
	linesToJoin := count
	if linesToJoin > len(lines)-currentLine-1 {
		linesToJoin = len(lines) - currentLine - 1
	}

	joinedLine := lines[currentLine]
	cursorPos := len([]rune(joinedLine))

	for i := 1; i <= linesToJoin; i++ {
		nextLine := strings.TrimLeft(lines[currentLine+i], " \t")
		if len(nextLine) > 0 {
			if len(joinedLine) > 0 && !strings.HasSuffix(joinedLine, " ") {
				joinedLine += " "
			}
			joinedLine += nextLine
		}
	}

	newLines := make([]string, 0, len(lines)-linesToJoin)
	newLines = append(newLines, lines[:currentLine]...)
	newLines = append(newLines, joinedLine)
	newLines = append(newLines, lines[currentLine+linesToJoin+1:]...)

	newText := strings.Join(newLines, "\n")
	ctx.SetText(newText)
	off := lineStartRuneOffset(newLines, currentLine) + cursorPos
	ctx.SetOffset(off)
}

// ExecutePaste executes the p/P (paste) command.
func ExecutePaste(after bool, count int, ctx *OperatorContext) {
	register := ctx.GetRegister()
	if register == "" {
		return
	}
	isLinewise := strings.HasSuffix(register, "\n")
	content := register
	if isLinewise {
		content = register[:len(register)-1]
	}

	if isLinewise {
		lines := strings.Split(ctx.Text, "\n")
		runes := []rune(ctx.Text)
		currentLine := 0
		for i := 0; i < ctx.Cursor && i < len(runes); i++ {
			if runes[i] == '\n' {
				currentLine++
			}
		}
		insertLine := currentLine
		if after {
			insertLine = currentLine + 1
		}

		contentLines := strings.Split(content, "\n")
		var repeated []string
		for i := 0; i < count; i++ {
			repeated = append(repeated, contentLines...)
		}

		newLines := make([]string, 0, len(lines)+len(repeated))
		newLines = append(newLines, lines[:insertLine]...)
		newLines = append(newLines, repeated...)
		newLines = append(newLines, lines[insertLine:]...)

		newText := strings.Join(newLines, "\n")
		ctx.SetText(newText)
		ctx.SetOffset(lineStartRuneOffset(newLines, insertLine))
	} else {
		textToInsert := strings.Repeat(content, count)
		runes := []rune(ctx.Text)
		insertPoint := ctx.Cursor
		if after && ctx.Cursor < len(runes) {
			insertPoint = ctx.Cursor + 1
		}
		newText := string(runes[:insertPoint]) + textToInsert + string(runes[insertPoint:])
		newRunes := []rune(textToInsert)
		newOffset := insertPoint + len(newRunes) - 1
		if newOffset < insertPoint {
			newOffset = insertPoint
		}
		ctx.SetText(newText)
		ctx.SetOffset(newOffset)
	}
}

// ExecuteIndent executes >> or << (indent/dedent).
func ExecuteIndent(dir rune, count int, ctx *OperatorContext) {
	lines := strings.Split(ctx.Text, "\n")
	runes := []rune(ctx.Text)
	currentLine := 0
	for i := 0; i < ctx.Cursor && i < len(runes); i++ {
		if runes[i] == '\n' {
			currentLine++
		}
	}
	linesToAffect := count
	if linesToAffect > len(lines)-currentLine {
		linesToAffect = len(lines) - currentLine
	}
	indent := "  " // two spaces

	for i := 0; i < linesToAffect; i++ {
		idx := currentLine + i
		if idx >= len(lines) {
			break
		}
		line := lines[idx]
		if dir == '>' {
			lines[idx] = indent + line
		} else {
			if strings.HasPrefix(line, indent) {
				lines[idx] = line[len(indent):]
			} else if strings.HasPrefix(line, "\t") {
				lines[idx] = line[1:]
			} else {
				// Remove up to indent-length of leading whitespace
				trimmed := strings.TrimLeftFunc(line, func(r rune) bool {
					return r == ' ' || r == '\t'
				})
				lines[idx] = trimmed
			}
		}
	}

	newText := strings.Join(lines, "\n")
	ctx.SetText(newText)
	// Position cursor at first non-blank of current line
	curLine := lines[currentLine]
	firstNonBlank := len(curLine) - len(strings.TrimLeft(curLine, " \t"))
	ctx.SetOffset(lineStartRuneOffset(lines, currentLine) + len([]rune(curLine[:firstNonBlank])))
}

// ExecuteOpenLine executes o/O (open line above/below).
func ExecuteOpenLine(direction string, ctx *OperatorContext) {
	lines := strings.Split(ctx.Text, "\n")
	runes := []rune(ctx.Text)
	currentLine := 0
	for i := 0; i < ctx.Cursor && i < len(runes); i++ {
		if runes[i] == '\n' {
			currentLine++
		}
	}

	insertLine := currentLine + 1
	if direction == "above" {
		insertLine = currentLine
	}

	newLines := make([]string, 0, len(lines)+1)
	newLines = append(newLines, lines[:insertLine]...)
	newLines = append(newLines, "")
	newLines = append(newLines, lines[insertLine:]...)

	newText := strings.Join(newLines, "\n")
	ctx.SetText(newText)
	ctx.EnterInsert(lineStartRuneOffset(newLines, insertLine))
}

// ExecuteOperatorG executes dG/cG/yG.
func ExecuteOperatorG(op Operator, count int, ctx *OperatorContext) {
	runes := []rune(ctx.Text)
	var target int
	if count == 1 {
		target = motionLastLine(runes)
	} else {
		target = GoToLine(ctx.Text, count)
	}
	if target == ctx.Cursor {
		return
	}
	from, to, linewise := getOperatorRange(ctx.Cursor, target, "G", op, count, ctx.Text)
	applyOperator(op, from, to, ctx, linewise)
}

// ExecuteOperatorGg executes dgg/cgg/ygg.
func ExecuteOperatorGg(op Operator, count int, ctx *OperatorContext) {
	var target int
	if count == 1 {
		target = 0
	} else {
		target = GoToLine(ctx.Text, count)
	}
	if target == ctx.Cursor {
		return
	}
	from, to, linewise := getOperatorRange(ctx.Cursor, target, "gg", op, count, ctx.Text)
	applyOperator(op, from, to, ctx, linewise)
}

// --- internal helpers ---

func getOperatorRange(cursor, target int, motion string, op Operator, count int, text string) (from, to int, linewise bool) {
	from = cursor
	to = target
	if from > to {
		from, to = to, from
	}

	runes := []rune(text)

	// cw/cW: change to end of word, not start of next
	if op == OpChange && (motion == "w" || motion == "W") {
		bigWord := motion == "W"
		wordCursor := cursor
		for i := 0; i < count-1; i++ {
			wordCursor = motionNextWord(runes, wordCursor, bigWord)
		}
		wordEnd := motionEndWord(runes, wordCursor, bigWord)
		to = wordEnd + 1
		if to > len(runes) {
			to = len(runes)
		}
		from = cursor
		return from, to, false
	}

	if IsLinewiseMotion(motion) {
		linewise = true
		// Extend to full lines
		nextNL := -1
		for i := to; i < len(runes); i++ {
			if runes[i] == '\n' {
				nextNL = i
				break
			}
		}
		if nextNL == -1 {
			to = len(runes)
			if from > 0 && runes[from-1] == '\n' {
				from--
			}
		} else {
			to = nextNL + 1
		}
	} else if IsInclusiveMotion(motion) && cursor <= target {
		to++
		if to > len(runes) {
			to = len(runes)
		}
	}

	return from, to, linewise
}

func getOperatorRangeForFind(cursor, target int) (from, to int) {
	from = cursor
	to = target
	if from > to {
		from, to = to, from
	}
	to++ // inclusive
	return from, to
}

func applyOperator(op Operator, from, to int, ctx *OperatorContext, linewise bool) {
	runes := []rune(ctx.Text)
	if from < 0 {
		from = 0
	}
	if to > len(runes) {
		to = len(runes)
	}
	if from >= to {
		return
	}

	content := string(runes[from:to])
	if linewise && !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	ctx.SetRegister(content, linewise)

	switch op {
	case OpYank:
		ctx.SetOffset(from)
	case OpDelete:
		newText := string(runes[:from]) + string(runes[to:])
		ctx.SetText(newText)
		newRunes := []rune(newText)
		maxOff := maxOffset(newRunes)
		off := from
		if off > maxOff {
			off = maxOff
		}
		ctx.SetOffset(off)
	case OpChange:
		newText := string(runes[:from]) + string(runes[to:])
		ctx.SetText(newText)
		ctx.EnterInsert(from)
	}
}

func maxOffset(runes []rune) int {
	if len(runes) == 0 {
		return 0
	}
	return len(runes) - 1
}

func lineStartOffset(lines []string, lineIndex int) int {
	offset := 0
	for i := 0; i < lineIndex && i < len(lines); i++ {
		offset += len(lines[i]) + 1 // +1 for \n
	}
	return offset
}

func lineStartRuneOffset(lines []string, lineIndex int) int {
	offset := 0
	for i := 0; i < lineIndex && i < len(lines); i++ {
		offset += utf8.RuneCountInString(lines[i]) + 1
	}
	return offset
}
