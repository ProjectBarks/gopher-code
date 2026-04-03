package components

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/formatters"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/projectbarks/gopher-code/pkg/ui/theme"
)

// CodeBlock displays syntax-highlighted code with line numbers.
type CodeBlock struct {
	code      string
	language  string
	theme     theme.Theme
	width     int
	showLines bool
}

// NewCodeBlock creates a new CodeBlock.
func NewCodeBlock(language, code string, t theme.Theme) *CodeBlock {
	return &CodeBlock{
		code:      code,
		language:  language,
		theme:     t,
		width:     80,
		showLines: true,
	}
}

// SetShowLineNumbers toggles line number display.
func (cb *CodeBlock) SetShowLineNumbers(show bool) {
	cb.showLines = show
}

// Update handles messages.
func (cb *CodeBlock) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return cb, nil
}

// View renders the syntax-highlighted code block.
func (cb *CodeBlock) View() tea.View {
	if cb.code == "" {
		return tea.NewView("")
	}

	// Try to get a lexer for the language
	lexer := lexers.Get(cb.language)
	if lexer == nil {
		lexer = lexers.Fallback
	}

	// Tokenize the code
	tokens, err := chroma.Tokenise(lexer, nil, cb.code)
	if err != nil {
		// Fallback: render without highlighting
		return cb.renderPlain()
	}

	// Format tokens with ANSI colors
	var buf strings.Builder
	formatter := formatters.Get("terminal256")
	if formatter == nil {
		formatter = formatters.Fallback
	}

	// Use the monokai style for dark backgrounds
	style := styles.Get("monokai")
	if style == nil {
		style = styles.Fallback
	}

	// Convert tokens slice to an Iterator using Literator
	tokenIterator := chroma.Literator(tokens...)
	err = formatter.Format(&buf, style, tokenIterator)
	if err != nil {
		// Fallback to plain rendering
		return cb.renderPlain()
	}

	highlighted := buf.String()

	// Add line numbers
	if cb.showLines {
		highlighted = cb.addLineNumbers(highlighted)
	}

	return tea.NewView(highlighted)
}

// renderPlain renders code without syntax highlighting.
func (cb *CodeBlock) renderPlain() tea.View {
	lines := strings.Split(cb.code, "\n")

	if cb.showLines {
		var output []string
		for i, line := range lines {
			lineNum := fmt.Sprintf("%3d", i+1)
			output = append(output, lineNum+" | "+line)
		}
		return tea.NewView(strings.Join(output, "\n"))
	}

	return tea.NewView(cb.code)
}

// addLineNumbers adds line numbers to syntax-highlighted code.
func (cb *CodeBlock) addLineNumbers(highlighted string) string {
	lines := strings.Split(highlighted, "\n")

	cs := cb.theme.Colors()
	lineNumStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(cs.TextSecondary)).
		Faint(true)

	var output []string
	for i, line := range lines {
		lineNum := fmt.Sprintf("%3d", i+1)
		styledNum := lineNumStyle.Render(lineNum)
		output = append(output, styledNum+" │ "+line)
	}

	return strings.Join(output, "\n")
}

// Init initializes the component.
func (cb *CodeBlock) Init() tea.Cmd {
	return nil
}

// SetSize sets the available width for rendering.
func (cb *CodeBlock) SetSize(width, height int) {
	cb.width = width
}

// Code returns the code content.
func (cb *CodeBlock) Code() string {
	return cb.code
}

// Language returns the detected language.
func (cb *CodeBlock) Language() string {
	return cb.language
}

// Ensure CodeBlock implements tea.Model.
var _ tea.Model = (*CodeBlock)(nil)
