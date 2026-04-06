package vim

// FindTextObject locates the [Start, End) range of a text object at the given
// rune offset. Returns nil if the text object cannot be found.
//
// objectType: 'w','W','"','\'','`','(',')','{','}','[',']','<','>','b','B'
// inner: true for "inner" (i), false for "around" (a)
//
// Source: src/vim/textObjects.ts — findTextObject
func FindTextObject(text string, offset int, objectType rune, inner bool) *TextObjectRange {
	switch objectType {
	case 'w':
		return findWordObject(text, offset, inner, isWordChar)
	case 'W':
		return findWordObject(text, offset, inner, func(r rune) bool { return !isWhitespace(r) })
	default:
		pair, ok := pairs[objectType]
		if !ok {
			return nil
		}
		if pair.open == pair.close {
			return findQuoteObject(text, offset, pair.open, inner)
		}
		return findBracketObject(text, offset, pair.open, pair.close, inner)
	}
}

type delimPair struct {
	open, close rune
}

var pairs = map[rune]delimPair{
	'(':  {'(', ')'},
	')':  {'(', ')'},
	'b':  {'(', ')'},
	'[':  {'[', ']'},
	']':  {'[', ']'},
	'{':  {'{', '}'},
	'}':  {'{', '}'},
	'B':  {'{', '}'},
	'<':  {'<', '>'},
	'>':  {'<', '>'},
	'"':  {'"', '"'},
	'\'': {'\'', '\''},
	'`':  {'`', '`'},
}

func findWordObject(text string, offset int, inner bool, isWord func(rune) bool) *TextObjectRange {
	runes := []rune(text)
	if len(runes) == 0 {
		return nil
	}
	if offset >= len(runes) {
		offset = len(runes) - 1
	}
	if offset < 0 {
		offset = 0
	}

	start := offset
	end := offset

	ch := runes[offset]
	if isWord(ch) {
		for start > 0 && isWord(runes[start-1]) {
			start--
		}
		for end < len(runes)-1 && isWord(runes[end+1]) {
			end++
		}
		end++ // half-open
	} else if isWhitespace(ch) {
		for start > 0 && isWhitespace(runes[start-1]) {
			start--
		}
		for end < len(runes)-1 && isWhitespace(runes[end+1]) {
			end++
		}
		end++
		return &TextObjectRange{Start: start, End: end}
	} else {
		// punctuation
		for start > 0 && isPunctuation(runes[start-1]) {
			start--
		}
		for end < len(runes)-1 && isPunctuation(runes[end+1]) {
			end++
		}
		end++
	}

	if !inner {
		// Include surrounding whitespace
		if end < len(runes) && isWhitespace(runes[end]) {
			for end < len(runes) && isWhitespace(runes[end]) {
				end++
			}
		} else if start > 0 && isWhitespace(runes[start-1]) {
			for start > 0 && isWhitespace(runes[start-1]) {
				start--
			}
		}
	}

	return &TextObjectRange{Start: start, End: end}
}

func findQuoteObject(text string, offset int, quote rune, inner bool) *TextObjectRange {
	runes := []rune(text)
	// Find line bounds
	lineStart := 0
	for i := offset - 1; i >= 0; i-- {
		if runes[i] == '\n' {
			lineStart = i + 1
			break
		}
	}
	lineEnd := len(runes)
	for i := offset; i < len(runes); i++ {
		if runes[i] == '\n' {
			lineEnd = i
			break
		}
	}

	// Collect quote positions in this line
	var positions []int
	for i := lineStart; i < lineEnd; i++ {
		if runes[i] == quote {
			positions = append(positions, i)
		}
	}

	// Pair quotes: 0-1, 2-3, 4-5, ...
	for i := 0; i+1 < len(positions); i += 2 {
		qs := positions[i]
		qe := positions[i+1]
		if qs <= offset && offset <= qe {
			if inner {
				return &TextObjectRange{Start: qs + 1, End: qe}
			}
			return &TextObjectRange{Start: qs, End: qe + 1}
		}
	}
	return nil
}

func findBracketObject(text string, offset int, open, close rune, inner bool) *TextObjectRange {
	runes := []rune(text)
	// Search backward for matching open
	depth := 0
	start := -1
	for i := offset; i >= 0; i-- {
		if runes[i] == close && i != offset {
			depth++
		} else if runes[i] == open {
			if depth == 0 {
				start = i
				break
			}
			depth--
		}
	}
	if start == -1 {
		return nil
	}

	// Search forward for matching close
	depth = 0
	end := -1
	for i := start + 1; i < len(runes); i++ {
		if runes[i] == open {
			depth++
		} else if runes[i] == close {
			if depth == 0 {
				end = i
				break
			}
			depth--
		}
	}
	if end == -1 {
		return nil
	}

	if inner {
		return &TextObjectRange{Start: start + 1, End: end}
	}
	return &TextObjectRange{Start: start, End: end + 1}
}
