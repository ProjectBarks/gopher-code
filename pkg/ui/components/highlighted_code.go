package components

import (
	"path/filepath"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/formatters"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
)

// Source: components/HighlightedCode.tsx
//
// In TS, syntax highlighting uses tree-sitter via a native module.
// In Go, we use chroma (Alec Thomas' syntax highlighter) which is
// already in go.mod. Chroma has 200+ lexers and outputs ANSI.

// HighlightCode returns syntax-highlighted code as an ANSI string.
// Falls back to plain text if the language can't be detected.
func HighlightCode(code, filePath string) string {
	lexer := detectLexer(filePath, code)
	if lexer == nil {
		return code
	}

	style := styles.Get("monokai")
	if style == nil {
		style = styles.Fallback
	}

	formatter := formatters.Get("terminal256")
	if formatter == nil {
		formatter = formatters.Fallback
	}

	iterator, err := lexer.Tokenise(nil, code)
	if err != nil {
		return code
	}

	var sb strings.Builder
	if err := formatter.Format(&sb, style, iterator); err != nil {
		return code
	}
	return strings.TrimRight(sb.String(), "\n")
}

// HighlightCodeWithTheme highlights code using a named chroma style.
// Supported styles: monokai, dracula, github, vs, solarized-dark, etc.
func HighlightCodeWithTheme(code, filePath, themeName string) string {
	lexer := detectLexer(filePath, code)
	if lexer == nil {
		return code
	}

	style := styles.Get(themeName)
	if style == nil {
		style = styles.Get("monokai")
	}

	formatter := formatters.Get("terminal256")
	iterator, err := lexer.Tokenise(nil, code)
	if err != nil {
		return code
	}

	var sb strings.Builder
	if err := formatter.Format(&sb, style, iterator); err != nil {
		return code
	}
	return strings.TrimRight(sb.String(), "\n")
}

// detectLexer finds the appropriate lexer for the code.
func detectLexer(filePath, code string) chroma.Lexer {
	// Try by filename first
	if filePath != "" {
		lexer := lexers.Match(filepath.Base(filePath))
		if lexer != nil {
			return chroma.Coalesce(lexer)
		}
	}

	// Try content analysis
	lexer := lexers.Analyse(code)
	if lexer != nil {
		return chroma.Coalesce(lexer)
	}

	return nil
}

// SupportedLanguage returns true if chroma has a lexer for the file extension.
func SupportedLanguage(filePath string) bool {
	return lexers.Match(filepath.Base(filePath)) != nil
}

// DetectLanguage returns the detected language name, or "" if unknown.
func DetectLanguage(filePath string) string {
	lexer := lexers.Match(filepath.Base(filePath))
	if lexer == nil {
		return ""
	}
	config := lexer.Config()
	if config == nil {
		return ""
	}
	return config.Name
}
