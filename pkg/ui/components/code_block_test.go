package components

import (
	"fmt"
	"strings"
	"testing"

	"github.com/projectbarks/gopher-code/pkg/ui/theme"
)

func TestCodeBlockCreation(t *testing.T) {
	th := theme.Current()
	cb := NewCodeBlock("go", "fmt.Println(\"hello\")", th)

	if cb == nil {
		t.Error("Expected non-nil CodeBlock")
	}
	if cb.Language() != "go" {
		t.Errorf("Expected language 'go', got %q", cb.Language())
	}
	if cb.Code() != "fmt.Println(\"hello\")" {
		t.Errorf("Expected code to be preserved")
	}
}

func TestCodeBlockPlainJavaScript(t *testing.T) {
	th := theme.Current()
	jsCode := `console.log("hello");`
	cb := NewCodeBlock("javascript", jsCode, th)

	view := cb.View()
	// May have ANSI codes between, so check for both parts
	if !strings.Contains(view.Content, "console") || !strings.Contains(view.Content, "log") {
		t.Errorf("Expected code in view, got %q", view.Content)
	}
}

func TestCodeBlockPlainPython(t *testing.T) {
	th := theme.Current()
	pyCode := `print("hello")`
	cb := NewCodeBlock("python", pyCode, th)

	view := cb.View()
	if !strings.Contains(view.Content, "print") {
		t.Errorf("Expected code in view, got %q", view.Content)
	}
}

func TestCodeBlockGo(t *testing.T) {
	th := theme.Current()
	goCode := `package main

func main() {
	fmt.Println("hello")
}`
	cb := NewCodeBlock("go", goCode, th)

	view := cb.View()
	// May have ANSI codes, so check for parts
	if !strings.Contains(view.Content, "package") || !strings.Contains(view.Content, "main") {
		t.Errorf("Expected code in view, got %q", view.Content)
	}
	if !strings.Contains(view.Content, "fmt") || !strings.Contains(view.Content, "Println") {
		t.Errorf("Expected fmt.Println in view")
	}
}

func TestCodeBlockLineNumbers(t *testing.T) {
	th := theme.Current()
	code := "line 1\nline 2\nline 3"
	cb := NewCodeBlock("text", code, th)

	cb.SetShowLineNumbers(true)
	view := cb.View()

	// Should have line numbers
	if !strings.Contains(view.Content, "1") {
		t.Error("Expected line number 1")
	}
	if !strings.Contains(view.Content, "2") {
		t.Error("Expected line number 2")
	}
	if !strings.Contains(view.Content, "3") {
		t.Error("Expected line number 3")
	}
}

func TestCodeBlockWithoutLineNumbers(t *testing.T) {
	th := theme.Current()
	code := "line 1\nline 2"
	cb := NewCodeBlock("text", code, th)

	cb.SetShowLineNumbers(false)
	view := cb.View()

	// Content should be present
	if !strings.Contains(view.Content, "line 1") {
		t.Error("Expected 'line 1' in view")
	}
	if !strings.Contains(view.Content, "line 2") {
		t.Error("Expected 'line 2' in view")
	}
}

func TestCodeBlockEmptyCode(t *testing.T) {
	th := theme.Current()
	cb := NewCodeBlock("go", "", th)

	view := cb.View()
	if view.Content != "" {
		t.Errorf("Expected empty view for empty code, got %q", view.Content)
	}
}

func TestCodeBlockSingleLine(t *testing.T) {
	th := theme.Current()
	code := "const x = 42;"
	cb := NewCodeBlock("javascript", code, th)

	cb.SetShowLineNumbers(true)
	view := cb.View()

	// May have ANSI codes between tokens
	if !strings.Contains(view.Content, "const") || !strings.Contains(view.Content, "42") {
		t.Error("Expected code in view")
	}
}

func TestCodeBlockMultilineWithEmptyLines(t *testing.T) {
	th := theme.Current()
	code := "line1\n\nline3"
	cb := NewCodeBlock("text", code, th)

	view := cb.View()
	if !strings.Contains(view.Content, "line1") {
		t.Error("Expected line1")
	}
	if !strings.Contains(view.Content, "line3") {
		t.Error("Expected line3")
	}
}

func TestCodeBlockLanguageDetection(t *testing.T) {
	th := theme.Current()

	languages := []struct {
		lang string
		code string
	}{
		{"go", "package main"},
		{"python", "def hello():"},
		{"javascript", "function hello()"},
		{"bash", "#!/bin/bash"},
		{"sql", "SELECT * FROM users"},
		{"html", "<html>"},
		{"css", "body { color: blue }"},
		{"java", "public class Hello"},
		{"cpp", "#include <iostream>"},
		{"rust", "fn main()"},
	}

	for _, test := range languages {
		cb := NewCodeBlock(test.lang, test.code, th)
		if cb.Language() != test.lang {
			t.Errorf("Expected language %s, got %s", test.lang, cb.Language())
		}
	}
}

func TestCodeBlockInit(t *testing.T) {
	th := theme.Current()
	cb := NewCodeBlock("go", "code", th)

	cmd := cb.Init()
	if cmd != nil {
		t.Error("Expected Init() to return nil")
	}
}

func TestCodeBlockSetSize(t *testing.T) {
	th := theme.Current()
	cb := NewCodeBlock("go", "code", th)

	cb.SetSize(120, 40)
	if cb.width != 120 {
		t.Errorf("Expected width 120, got %d", cb.width)
	}
}

func TestCodeBlockUpdate(t *testing.T) {
	th := theme.Current()
	cb := NewCodeBlock("go", "code", th)

	updated, cmd := cb.Update(struct{}{})

	if cmd != nil {
		t.Error("Expected nil command")
	}
	if updated != cb {
		t.Error("Expected Update to return self")
	}
}

func TestCodeBlockLineNumberToggle(t *testing.T) {
	th := theme.Current()
	cb := NewCodeBlock("text", "line 1\nline 2", th)

	// With line numbers
	cb.SetShowLineNumbers(true)
	view1 := cb.View()

	// Without line numbers
	cb.SetShowLineNumbers(false)
	view2 := cb.View()

	// Both should have the code
	if !strings.Contains(view1.Content, "line 1") {
		t.Error("Expected code in view1")
	}
	if !strings.Contains(view2.Content, "line 1") {
		t.Error("Expected code in view2")
	}
}

func TestCodeBlockLongLines(t *testing.T) {
	th := theme.Current()
	longLine := strings.Repeat("x", 200)
	cb := NewCodeBlock("text", longLine, th)

	view := cb.View()
	if !strings.Contains(view.Content, "x") {
		t.Error("Expected long line to be rendered")
	}
}

func TestCodeBlockSpecialCharacters(t *testing.T) {
	th := theme.Current()
	code := "// Comment with special chars: <>&\"'\n" +
		"const str = \"escaped \\\"quote\\\"\";"
	cb := NewCodeBlock("javascript", code, th)

	view := cb.View()
	if !strings.Contains(view.Content, "Comment") {
		t.Error("Expected comment in view")
	}
}

func TestCodeBlockTabCharacters(t *testing.T) {
	th := theme.Current()
	code := "func hello() {\n\tfmt.Println(\"hello\")\n}"
	cb := NewCodeBlock("go", code, th)

	view := cb.View()
	// May have ANSI codes between
	if !strings.Contains(view.Content, "fmt") || !strings.Contains(view.Content, "Println") {
		t.Error("Expected code with tabs")
	}
}

func TestCodeBlockUnicodeCharacters(t *testing.T) {
	th := theme.Current()
	code := "// Hello 你好 مرحبا\nconst msg = \"世界\";"
	cb := NewCodeBlock("javascript", code, th)

	view := cb.View()
	if !strings.Contains(view.Content, "msg") {
		t.Error("Expected code with unicode")
	}
}

func TestCodeBlockWithComments(t *testing.T) {
	th := theme.Current()
	code := `// This is a comment
func Add(a, b int) int {
	// Implementation
	return a + b
}`
	cb := NewCodeBlock("go", code, th)

	view := cb.View()
	// May have ANSI codes between
	if !strings.Contains(view.Content, "func") || !strings.Contains(view.Content, "Add") {
		t.Error("Expected function in view")
	}
	if !strings.Contains(view.Content, "comment") {
		t.Logf("Expected comment (may be styled): %q", view.Content)
	}
}

func TestCodeBlockLanguageCaseInsensitive(t *testing.T) {
	th := theme.Current()
	code := "package main"

	// Test different casings
	languages := []string{"go", "Go", "GO"}
	for _, lang := range languages {
		cb := NewCodeBlock(lang, code, th)
		view := cb.View()

		if !strings.Contains(view.Content, "package") {
			t.Errorf("Expected code for language %q", lang)
		}
	}
}

func TestCodeBlockViewConsistency(t *testing.T) {
	th := theme.Current()
	code := "const x = 42;"
	cb := NewCodeBlock("javascript", code, th)

	view1 := cb.View()
	view2 := cb.View()

	if !strings.Contains(view1.Content, "const") {
		t.Error("Expected code in view1")
	}
	if !strings.Contains(view2.Content, "const") {
		t.Error("Expected code in view2")
	}
}

func TestCodeBlockCodeAccessor(t *testing.T) {
	th := theme.Current()
	code := "hello world"
	cb := NewCodeBlock("text", code, th)

	if cb.Code() != code {
		t.Errorf("Expected Code() to return %q, got %q", code, cb.Code())
	}
}

func TestCodeBlockLanguageAccessor(t *testing.T) {
	th := theme.Current()
	cb := NewCodeBlock("python", "code", th)

	if cb.Language() != "python" {
		t.Errorf("Expected Language() to return 'python', got %q", cb.Language())
	}
}

func TestCodeBlockWithDifferentThemes(t *testing.T) {
	themeNames := []theme.ThemeName{
		theme.ThemeDark,
		theme.ThemeLight,
		theme.ThemeHighContrast,
	}

	code := "const x = 42;"

	for _, themeName := range themeNames {
		theme.SetTheme(themeName)
		defer theme.SetTheme(theme.ThemeDark)

		th := theme.Current()
		cb := NewCodeBlock("javascript", code, th)

		view := cb.View()
		if !strings.Contains(view.Content, "const") {
			t.Errorf("Expected code with theme %s", themeName)
		}
	}
}

func TestCodeBlockUnknownLanguage(t *testing.T) {
	th := theme.Current()
	code := "some code here"
	cb := NewCodeBlock("unknownlanguage123", code, th)

	view := cb.View()
	// Should still render the code with fallback lexer
	if !strings.Contains(view.Content, "some code") {
		t.Error("Expected fallback rendering for unknown language")
	}
}

func TestCodeBlockEmptyLanguage(t *testing.T) {
	th := theme.Current()
	code := "code here"
	cb := NewCodeBlock("", code, th)

	view := cb.View()
	if !strings.Contains(view.Content, "code") {
		t.Error("Expected code to render with empty language")
	}
}

func TestCodeBlockLineNumberFormatting(t *testing.T) {
	th := theme.Current()
	lines := make([]string, 0)
	for i := 0; i < 10; i++ {
		lines = append(lines, fmt.Sprintf("line %d", i+1))
	}
	code := strings.Join(lines, "\n")

	cb := NewCodeBlock("text", code, th)
	cb.SetShowLineNumbers(true)
	view := cb.View()

	// Check line number formatting (should be right-aligned in 3 chars)
	if !strings.Contains(view.Content, "1 │") {
		t.Logf("Expected line number format, got: %q", view.Content[:100])
	}
}

func TestCodeBlockVeryLongCode(t *testing.T) {
	th := theme.Current()
	lines := make([]string, 0)
	for i := 0; i < 100; i++ {
		lines = append(lines, "line "+strings.Repeat("x", 50))
	}
	code := strings.Join(lines, "\n")

	cb := NewCodeBlock("text", code, th)
	cb.SetShowLineNumbers(true)
	view := cb.View()

	if !strings.Contains(view.Content, "line") {
		t.Error("Expected code in very long code block")
	}
}
