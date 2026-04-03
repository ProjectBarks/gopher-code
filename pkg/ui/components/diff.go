package components

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/projectbarks/gopher-code/pkg/ui/theme"
)

// DiffLine represents a single line in a diff.
type DiffLine struct {
	Type    DiffLineType
	Content string
	OldNum  int // Line number in old file (0 = not applicable)
	NewNum  int // Line number in new file (0 = not applicable)
}

// DiffLineType identifies the type of diff line.
type DiffLineType int

const (
	DiffContext  DiffLineType = iota // Unchanged line
	DiffAdded                        // Added line
	DiffRemoved                      // Removed line
	DiffHeader                       // File header / hunk header
)

// DiffViewMode controls the display format.
type DiffViewMode int

const (
	DiffUnified    DiffViewMode = iota // Unified diff format
	DiffSideBySide                      // Side-by-side format
)

// DiffViewer displays diffs with syntax highlighting and scrolling.
type DiffViewer struct {
	lines      []DiffLine
	mode       DiffViewMode
	scrollPos  int
	width      int
	height     int
	focused    bool
	theme      theme.Theme
	fileName   string
}

// NewDiffViewer creates a new diff viewer.
func NewDiffViewer(t theme.Theme) *DiffViewer {
	return &DiffViewer{
		lines: make([]DiffLine, 0),
		mode:  DiffUnified,
		theme: t,
		width: 80,
		height: 20,
	}
}

// SetDiff parses and sets the diff content.
func (dv *DiffViewer) SetDiff(diffText string) {
	dv.lines = parseDiffLines(diffText)
	dv.scrollPos = 0
}

// SetFileName sets the file name for the diff header.
func (dv *DiffViewer) SetFileName(name string) {
	dv.fileName = name
}

// ToggleMode switches between unified and side-by-side display.
func (dv *DiffViewer) ToggleMode() {
	if dv.mode == DiffUnified {
		dv.mode = DiffSideBySide
	} else {
		dv.mode = DiffUnified
	}
}

// Init initializes the component.
func (dv *DiffViewer) Init() tea.Cmd { return nil }

// Update handles key presses for scrolling and mode toggle.
func (dv *DiffViewer) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.Code {
		case tea.KeyUp, 'k':
			if dv.scrollPos > 0 {
				dv.scrollPos--
			}
		case tea.KeyDown, 'j':
			maxScroll := len(dv.lines) - dv.height
			if maxScroll < 0 {
				maxScroll = 0
			}
			if dv.scrollPos < maxScroll {
				dv.scrollPos++
			}
		case 'm':
			dv.ToggleMode()
		}
	case tea.WindowSizeMsg:
		dv.SetSize(msg.Width, msg.Height)
	}
	return dv, nil
}

// View renders the diff.
func (dv *DiffViewer) View() tea.View {
	if len(dv.lines) == 0 {
		return tea.NewView("No diff content")
	}

	cs := dv.theme.Colors()
	var output []string

	// Header
	if dv.fileName != "" {
		headerStyle := lipgloss.NewStyle().Bold(true).
			Foreground(lipgloss.Color(cs.TextPrimary))
		output = append(output, headerStyle.Render("📄 "+dv.fileName))
	}

	// Visible lines
	end := dv.scrollPos + dv.height
	if end > len(dv.lines) {
		end = len(dv.lines)
	}

	for i := dv.scrollPos; i < end; i++ {
		output = append(output, dv.renderLine(dv.lines[i], cs))
	}

	return tea.NewView(strings.Join(output, "\n"))
}

func (dv *DiffViewer) renderLine(line DiffLine, cs theme.ColorScheme) string {
	var style lipgloss.Style
	var prefix string

	switch line.Type {
	case DiffAdded:
		style = lipgloss.NewStyle().Foreground(lipgloss.Color(cs.DiffAdded))
		prefix = "+"
	case DiffRemoved:
		style = lipgloss.NewStyle().Foreground(lipgloss.Color(cs.DiffRemoved))
		prefix = "-"
	case DiffHeader:
		style = lipgloss.NewStyle().Foreground(lipgloss.Color(cs.Info)).Bold(true)
		return style.Render(line.Content)
	default:
		style = lipgloss.NewStyle().Foreground(lipgloss.Color(cs.TextSecondary))
		prefix = " "
	}

	// Line numbers
	lineNum := ""
	if line.OldNum > 0 || line.NewNum > 0 {
		lineNum = fmt.Sprintf("%4d ", line.NewNum)
		if line.NewNum == 0 {
			lineNum = fmt.Sprintf("%4d ", line.OldNum)
		}
	}

	content := prefix + lineNum + line.Content
	if dv.width > 0 && len(content) > dv.width {
		content = content[:dv.width]
	}

	return style.Render(content)
}

// SetSize sets the dimensions.
func (dv *DiffViewer) SetSize(width, height int) {
	dv.width = width
	dv.height = height
}

// Focus gives focus.
func (dv *DiffViewer) Focus()        { dv.focused = true }
func (dv *DiffViewer) Blur()         { dv.focused = false }
func (dv *DiffViewer) Focused() bool { return dv.focused }

// Lines returns the diff lines.
func (dv *DiffViewer) Lines() []DiffLine { return dv.lines }

// Mode returns the current view mode.
func (dv *DiffViewer) Mode() DiffViewMode { return dv.mode }

// parseDiffLines parses a unified diff string into DiffLine structs.
func parseDiffLines(diffText string) []DiffLine {
	var lines []DiffLine
	oldLine, newLine := 0, 0

	for _, raw := range strings.Split(diffText, "\n") {
		if raw == "" {
			continue
		}

		switch {
		case strings.HasPrefix(raw, "@@"):
			lines = append(lines, DiffLine{Type: DiffHeader, Content: raw})
			// Parse hunk header for line numbers
			fmt.Sscanf(raw, "@@ -%d", &oldLine)
			fmt.Sscanf(raw, "@@ %*s +%d", &newLine)
		case strings.HasPrefix(raw, "---"), strings.HasPrefix(raw, "+++"):
			lines = append(lines, DiffLine{Type: DiffHeader, Content: raw})
		case strings.HasPrefix(raw, "+"):
			newLine++
			lines = append(lines, DiffLine{Type: DiffAdded, Content: raw[1:], NewNum: newLine})
		case strings.HasPrefix(raw, "-"):
			oldLine++
			lines = append(lines, DiffLine{Type: DiffRemoved, Content: raw[1:], OldNum: oldLine})
		default:
			oldLine++
			newLine++
			content := raw
			if len(content) > 0 && content[0] == ' ' {
				content = content[1:]
			}
			lines = append(lines, DiffLine{Type: DiffContext, Content: content, OldNum: oldLine, NewNum: newLine})
		}
	}
	return lines
}

var _ tea.Model = (*DiffViewer)(nil)
