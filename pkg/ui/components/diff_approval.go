package components

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/projectbarks/gopher-code/pkg/ui/theme"
)

// ApprovalResult is the outcome of a diff approval dialog.
type ApprovalResult int

const (
	ApprovalPending ApprovalResult = iota
	ApprovalApproved
	ApprovalRejected
	ApprovalAlways
)

// ApprovalResponseMsg carries the result of a diff approval.
type ApprovalResponseMsg struct {
	ToolUseID string
	Result    ApprovalResult
}

// DiffApprovalDialog shows a diff with approve/reject controls.
type DiffApprovalDialog struct {
	diff      *DiffViewer
	toolName  string
	toolID    string
	result    ApprovalResult
	theme     theme.Theme
	width     int
	height    int
	focused   bool
	responseCh chan<- ApprovalResult
}

// NewDiffApprovalDialog creates a new diff approval dialog.
func NewDiffApprovalDialog(toolName, toolID, diffText string, t theme.Theme, ch chan<- ApprovalResult) *DiffApprovalDialog {
	dv := NewDiffViewer(t)
	dv.SetDiff(diffText)
	dv.SetFileName(toolName)

	return &DiffApprovalDialog{
		diff:       dv,
		toolName:   toolName,
		toolID:     toolID,
		result:     ApprovalPending,
		theme:      t,
		width:      80,
		height:     24,
		responseCh: ch,
	}
}

// Init initializes the component.
func (dad *DiffApprovalDialog) Init() tea.Cmd { return nil }

// Update handles key presses for approval/rejection.
func (dad *DiffApprovalDialog) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.Code {
		case tea.KeyEnter, 'y':
			dad.result = ApprovalApproved
			dad.sendResult(ApprovalApproved)
			return dad, func() tea.Msg {
				return ApprovalResponseMsg{ToolUseID: dad.toolID, Result: ApprovalApproved}
			}
		case 'n':
			dad.result = ApprovalRejected
			dad.sendResult(ApprovalRejected)
			return dad, func() tea.Msg {
				return ApprovalResponseMsg{ToolUseID: dad.toolID, Result: ApprovalRejected}
			}
		case 'a':
			dad.result = ApprovalAlways
			dad.sendResult(ApprovalAlways)
			return dad, func() tea.Msg {
				return ApprovalResponseMsg{ToolUseID: dad.toolID, Result: ApprovalAlways}
			}
		default:
			// Forward scroll keys to diff viewer
			dad.diff.Update(msg)
		}
	}
	return dad, nil
}

func (dad *DiffApprovalDialog) sendResult(result ApprovalResult) {
	if dad.responseCh != nil {
		select {
		case dad.responseCh <- result:
		default:
		}
	}
}

// View renders the diff with approval controls.
func (dad *DiffApprovalDialog) View() tea.View {
	cs := dad.theme.Colors()

	// Title
	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(cs.TextPrimary)).
		Bold(true)
	title := titleStyle.Render(fmt.Sprintf("Permission required: %s", dad.toolName))

	// Diff content
	diffHeight := dad.height - 5
	if diffHeight < 5 {
		diffHeight = 5
	}
	dad.diff.SetSize(dad.width-4, diffHeight)
	diffView := dad.diff.View().Content

	// Controls
	approveStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(cs.Success)).Bold(true)
	rejectStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(cs.Error)).Bold(true)
	alwaysStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(cs.Info)).Bold(true)

	controls := fmt.Sprintf("%s  %s  %s",
		approveStyle.Render("[y] Approve"),
		rejectStyle.Render("[n] Reject"),
		alwaysStyle.Render("[a] Always"),
	)

	return tea.NewView(title + "\n\n" + diffView + "\n\n" + controls)
}

// Result returns the current approval result.
func (dad *DiffApprovalDialog) Result() ApprovalResult {
	return dad.result
}

// SetSize sets the dimensions.
func (dad *DiffApprovalDialog) SetSize(width, height int) {
	dad.width = width
	dad.height = height
}

func (dad *DiffApprovalDialog) Focus()        { dad.focused = true }
func (dad *DiffApprovalDialog) Blur()         { dad.focused = false }
func (dad *DiffApprovalDialog) Focused() bool { return dad.focused }

var _ tea.Model = (*DiffApprovalDialog)(nil)
