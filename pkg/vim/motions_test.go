package vim

import "testing"

func TestResolveMotion_h(t *testing.T) {
	text := "hello"
	pos := ResolveMotion("h", text, 3, 1)
	if pos != 2 {
		t.Fatalf("h from 3: want 2, got %d", pos)
	}
}

func TestResolveMotion_h_atStart(t *testing.T) {
	pos := ResolveMotion("h", "hello", 0, 1)
	if pos != 0 {
		t.Fatalf("h at 0: want 0, got %d", pos)
	}
}

func TestResolveMotion_l(t *testing.T) {
	pos := ResolveMotion("l", "hello", 1, 1)
	if pos != 2 {
		t.Fatalf("l from 1: want 2, got %d", pos)
	}
}

func TestResolveMotion_l_atEnd(t *testing.T) {
	pos := ResolveMotion("l", "hello", 4, 1)
	if pos != 4 {
		t.Fatalf("l at end: want 4, got %d", pos)
	}
}

func TestResolveMotion_w(t *testing.T) {
	text := "hello world"
	pos := ResolveMotion("w", text, 0, 1)
	if pos != 6 {
		t.Fatalf("w from 0 in 'hello world': want 6, got %d", pos)
	}
}

func TestResolveMotion_b(t *testing.T) {
	text := "hello world"
	pos := ResolveMotion("b", text, 8, 1)
	if pos != 6 {
		t.Fatalf("b from 8 in 'hello world': want 6, got %d", pos)
	}
}

func TestResolveMotion_e(t *testing.T) {
	text := "hello world"
	pos := ResolveMotion("e", text, 0, 1)
	if pos != 4 {
		t.Fatalf("e from 0: want 4, got %d", pos)
	}
}

func TestResolveMotion_0(t *testing.T) {
	text := "hello world"
	pos := ResolveMotion("0", text, 5, 1)
	if pos != 0 {
		t.Fatalf("0 from 5: want 0, got %d", pos)
	}
}

func TestResolveMotion_dollar(t *testing.T) {
	text := "hello\nworld"
	pos := ResolveMotion("$", text, 0, 1)
	if pos != 4 {
		t.Fatalf("$ from 0 in 'hello\\nworld': want 4, got %d", pos)
	}
}

func TestResolveMotion_G(t *testing.T) {
	text := "line1\nline2\nline3"
	pos := ResolveMotion("G", text, 0, 1)
	if pos != 12 {
		t.Fatalf("G from 0: want 12 (start of line3), got %d", pos)
	}
}

func TestResolveMotion_gg(t *testing.T) {
	text := "line1\nline2\nline3"
	pos := ResolveMotion("gg", text, 14, 1)
	if pos != 0 {
		t.Fatalf("gg: want 0, got %d", pos)
	}
}

func TestResolveMotion_j(t *testing.T) {
	text := "abc\ndef\nghi"
	pos := ResolveMotion("j", text, 1, 1) // b -> e
	if pos != 5 {
		t.Fatalf("j from col 1 of line 0: want 5, got %d", pos)
	}
}

func TestResolveMotion_k(t *testing.T) {
	text := "abc\ndef\nghi"
	pos := ResolveMotion("k", text, 5, 1) // e -> b
	if pos != 1 {
		t.Fatalf("k from col 1 of line 1: want 1, got %d", pos)
	}
}

func TestResolveMotion_caret(t *testing.T) {
	text := "  hello"
	pos := ResolveMotion("^", text, 5, 1)
	if pos != 2 {
		t.Fatalf("^: want 2, got %d", pos)
	}
}

func TestResolveMotion_w_count(t *testing.T) {
	text := "one two three four"
	pos := ResolveMotion("w", text, 0, 3) // skip 3 words
	if pos != 14 {
		t.Fatalf("3w from 0: want 14, got %d", pos)
	}
}

func TestIsInclusiveMotion(t *testing.T) {
	if !IsInclusiveMotion("e") {
		t.Fatal("e should be inclusive")
	}
	if !IsInclusiveMotion("$") {
		t.Fatal("$ should be inclusive")
	}
	if IsInclusiveMotion("w") {
		t.Fatal("w should not be inclusive")
	}
}

func TestIsLinewiseMotion(t *testing.T) {
	if !IsLinewiseMotion("j") {
		t.Fatal("j should be linewise")
	}
	if !IsLinewiseMotion("gg") {
		t.Fatal("gg should be linewise")
	}
	if IsLinewiseMotion("w") {
		t.Fatal("w should not be linewise")
	}
}

func TestFindCharacter_f(t *testing.T) {
	text := "hello world"
	pos := FindCharacter(text, 0, 'o', FindF, 1)
	if pos != 4 {
		t.Fatalf("f o from 0: want 4, got %d", pos)
	}
}

func TestFindCharacter_t(t *testing.T) {
	text := "hello world"
	pos := FindCharacter(text, 0, 'o', FindT, 1)
	if pos != 3 {
		t.Fatalf("t o from 0: want 3, got %d", pos)
	}
}

func TestFindCharacter_F(t *testing.T) {
	text := "hello world"
	pos := FindCharacter(text, 10, 'o', FindB, 1)
	if pos != 7 {
		t.Fatalf("F o from 10: want 7, got %d", pos)
	}
}

func TestFindCharacter_notFound(t *testing.T) {
	pos := FindCharacter("hello", 0, 'z', FindF, 1)
	if pos != -1 {
		t.Fatalf("not found: want -1, got %d", pos)
	}
}

func TestGoToLine(t *testing.T) {
	text := "line1\nline2\nline3"
	pos := GoToLine(text, 2)
	if pos != 6 {
		t.Fatalf("GoToLine(2): want 6, got %d", pos)
	}
}
