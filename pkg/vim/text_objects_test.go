package vim

import "testing"

func TestFindTextObject_iw(t *testing.T) {
	text := "hello world"
	r := FindTextObject(text, 2, 'w', true) // inside "hello"
	if r == nil {
		t.Fatal("expected non-nil")
	}
	if r.Start != 0 || r.End != 5 {
		t.Fatalf("iw at 2 in 'hello world': want [0,5), got [%d,%d)", r.Start, r.End)
	}
}

func TestFindTextObject_aw(t *testing.T) {
	text := "hello world"
	r := FindTextObject(text, 2, 'w', false) // around "hello"
	if r == nil {
		t.Fatal("expected non-nil")
	}
	// "aw" includes trailing space: "hello "
	if r.Start != 0 || r.End != 6 {
		t.Fatalf("aw at 2: want [0,6), got [%d,%d)", r.Start, r.End)
	}
}

func TestFindTextObject_innerQuote(t *testing.T) {
	text := `say "hello" please`
	r := FindTextObject(text, 6, '"', true) // inside quotes
	if r == nil {
		t.Fatal("expected non-nil")
	}
	if r.Start != 5 || r.End != 10 {
		t.Fatalf("i\" at 6: want [5,10), got [%d,%d)", r.Start, r.End)
	}
}

func TestFindTextObject_aroundQuote(t *testing.T) {
	text := `say "hello" please`
	r := FindTextObject(text, 6, '"', false)
	if r == nil {
		t.Fatal("expected non-nil")
	}
	if r.Start != 4 || r.End != 11 {
		t.Fatalf("a\" at 6: want [4,11), got [%d,%d)", r.Start, r.End)
	}
}

func TestFindTextObject_innerParen(t *testing.T) {
	text := "foo(bar, baz)"
	r := FindTextObject(text, 5, '(', true) // inside parens
	if r == nil {
		t.Fatal("expected non-nil")
	}
	if r.Start != 4 || r.End != 12 {
		t.Fatalf("i( at 5: want [4,12), got [%d,%d)", r.Start, r.End)
	}
}

func TestFindTextObject_aroundParen(t *testing.T) {
	text := "foo(bar, baz)"
	r := FindTextObject(text, 5, '(', false)
	if r == nil {
		t.Fatal("expected non-nil")
	}
	if r.Start != 3 || r.End != 13 {
		t.Fatalf("a( at 5: want [3,13), got [%d,%d)", r.Start, r.End)
	}
}

func TestFindTextObject_innerBrace(t *testing.T) {
	text := "x{abc}y"
	r := FindTextObject(text, 3, '{', true)
	if r == nil {
		t.Fatal("expected non-nil")
	}
	if r.Start != 2 || r.End != 5 {
		t.Fatalf("i{ at 3: want [2,5), got [%d,%d)", r.Start, r.End)
	}
}

func TestFindTextObject_nested(t *testing.T) {
	text := "(a(b)c)"
	r := FindTextObject(text, 3, '(', true) // on 'b', inner paren is (b)
	if r == nil {
		t.Fatal("expected non-nil")
	}
	if r.Start != 3 || r.End != 4 {
		t.Fatalf("i( nested at 3: want [3,4), got [%d,%d)", r.Start, r.End)
	}
}

func TestFindTextObject_noMatch(t *testing.T) {
	r := FindTextObject("hello", 2, '(', true)
	if r != nil {
		t.Fatal("expected nil for no match")
	}
}

func TestFindTextObject_bigWord(t *testing.T) {
	text := "foo.bar baz"
	r := FindTextObject(text, 2, 'W', true) // "foo.bar" is one WORD
	if r == nil {
		t.Fatal("expected non-nil")
	}
	if r.Start != 0 || r.End != 7 {
		t.Fatalf("iW at 2: want [0,7), got [%d,%d)", r.Start, r.End)
	}
}

func TestFindTextObject_singleQuote(t *testing.T) {
	// Test with a properly paired single-quote string
	text := "say 'hello' now"
	r := FindTextObject(text, 6, '\'', true) // inside 'hello'
	if r == nil {
		t.Fatal("expected non-nil")
	}
	if r.Start != 5 || r.End != 10 {
		t.Fatalf("i' at 6: want [5,10), got [%d,%d)", r.Start, r.End)
	}
}

func TestFindTextObject_singleQuote_noMatch(t *testing.T) {
	// Offset 8 is between two quote pairs: (2,5) is one pair, 12 is unpaired
	text := "it's 'quoted' here"
	r := FindTextObject(text, 8, '\'', true)
	// Pairing: positions of ' are 2,5,12 -> pairs (2,5), 12 unpaired
	// offset 8 is not inside pair (2,5), so nil
	if r != nil {
		t.Fatal("expected nil — offset 8 is not inside any quote pair")
	}
}

func TestFindTextObject_backtick(t *testing.T) {
	text := "use `code` here"
	r := FindTextObject(text, 6, '`', true)
	if r == nil {
		t.Fatal("expected non-nil")
	}
	if r.Start != 5 || r.End != 9 {
		t.Fatalf("i` at 6: want [5,9), got [%d,%d)", r.Start, r.End)
	}
}
