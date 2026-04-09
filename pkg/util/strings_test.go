package util

import "testing"

func TestEscapeRegExp(t *testing.T) {
	got := EscapeRegExp("hello.world[0]")
	if got != `hello\.world\[0\]` {
		t.Errorf("got %q", got)
	}
}

func TestCapitalize(t *testing.T) {
	tests := map[string]string{
		"fooBar":      "FooBar",
		"hello world": "Hello world",
		"":            "",
		"A":           "A",
	}
	for input, want := range tests {
		if got := Capitalize(input); got != want {
			t.Errorf("Capitalize(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestPlural(t *testing.T) {
	if got := Plural(1, "file"); got != "file" {
		t.Errorf("got %q", got)
	}
	if got := Plural(3, "file"); got != "files" {
		t.Errorf("got %q", got)
	}
	if got := Plural(0, "error"); got != "errors" {
		t.Errorf("got %q", got)
	}
}

func TestPluralCustom(t *testing.T) {
	if got := PluralCustom(2, "entry", "entries"); got != "entries" {
		t.Errorf("got %q", got)
	}
	if got := PluralCustom(1, "entry", "entries"); got != "entry" {
		t.Errorf("got %q", got)
	}
}

func TestFirstLineOf(t *testing.T) {
	if got := FirstLineOf("hello\nworld"); got != "hello" {
		t.Errorf("got %q", got)
	}
	if got := FirstLineOf("no newline"); got != "no newline" {
		t.Errorf("got %q", got)
	}
}

func TestCountChar(t *testing.T) {
	if got := CountChar("hello world", 'l'); got != 3 {
		t.Errorf("got %d", got)
	}
	if got := CountChar("abc", 'z'); got != 0 {
		t.Errorf("got %d", got)
	}
}

func TestTruncateMiddle(t *testing.T) {
	if got := TruncateMiddle("short", 10); got != "short" {
		t.Errorf("got %q", got)
	}
	got := TruncateMiddle("abcdefghij", 7)
	if len([]rune(got)) > 7 {
		t.Errorf("too long: %q", got)
	}
}
