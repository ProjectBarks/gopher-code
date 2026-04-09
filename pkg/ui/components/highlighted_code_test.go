package components

import (
	"strings"
	"testing"
)

func TestHighlightCode_Go(t *testing.T) {
	code := `package main

import "fmt"

func main() {
	fmt.Println("hello")
}`
	result := HighlightCode(code, "main.go")
	if result == "" {
		t.Fatal("should produce output")
	}
	// Should contain ANSI escape codes (syntax coloring)
	if !strings.Contains(result, "\033[") {
		t.Error("should contain ANSI escape codes for Go")
	}
	// Should contain the original code text
	if !strings.Contains(result, "package") {
		t.Error("should contain 'package'")
	}
}

func TestHighlightCode_Python(t *testing.T) {
	code := `def hello():
    print("world")
`
	result := HighlightCode(code, "script.py")
	if !strings.Contains(result, "\033[") {
		t.Error("should highlight Python code")
	}
}

func TestHighlightCode_UnknownLanguage(t *testing.T) {
	code := "just plain text"
	result := HighlightCode(code, "unknown.xyz")
	// Should return plain text (no ANSI codes or same as input)
	if result == "" {
		t.Error("should return something")
	}
}

func TestHighlightCodeWithTheme(t *testing.T) {
	code := `const x = 42;`
	result := HighlightCodeWithTheme(code, "file.js", "dracula")
	if result == "" {
		t.Fatal("should produce output")
	}
}

func TestSupportedLanguage(t *testing.T) {
	if !SupportedLanguage("main.go") {
		t.Error("Go should be supported")
	}
	if !SupportedLanguage("script.py") {
		t.Error("Python should be supported")
	}
	if !SupportedLanguage("style.css") {
		t.Error("CSS should be supported")
	}
}

func TestDetectLanguage(t *testing.T) {
	if lang := DetectLanguage("main.go"); lang != "Go" {
		t.Errorf("Go detected as %q", lang)
	}
	if lang := DetectLanguage("script.py"); lang != "Python" {
		t.Errorf("Python detected as %q", lang)
	}
	if lang := DetectLanguage("unknown.xyz"); lang != "" {
		t.Errorf("unknown should be empty, got %q", lang)
	}
}
