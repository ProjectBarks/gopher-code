package components

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/projectbarks/gopher-code/pkg/ui/theme"
)

// ThinkingEventMsg carries thinking token updates.
type ThinkingEventMsg struct {
	TokensUsed   int
	BudgetTokens int
}

// ThinkingIndicator displays extended thinking budget usage.
type ThinkingIndicator struct {
	budgetTokens int
	usedTokens   int
	active       bool
	theme        theme.Theme
	width        int
	height       int
	focused      bool
}

// NewThinkingIndicator creates a new thinking indicator.
func NewThinkingIndicator(t theme.Theme) *ThinkingIndicator {
	return &ThinkingIndicator{
		theme: t,
		width: 80,
	}
}

// SetBudget sets the thinking token budget.
func (ti *ThinkingIndicator) SetBudget(tokens int) {
	ti.budgetTokens = tokens
}

// SetActive marks the indicator as active/inactive.
func (ti *ThinkingIndicator) SetActive(active bool) {
	ti.active = active
}

// Init initializes the component.
func (ti *ThinkingIndicator) Init() tea.Cmd { return nil }

// Update handles thinking event messages.
func (ti *ThinkingIndicator) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case ThinkingEventMsg:
		ti.usedTokens = msg.TokensUsed
		ti.budgetTokens = msg.BudgetTokens
		ti.active = true
	}
	return ti, nil
}

// View renders the thinking indicator with progress bar.
func (ti *ThinkingIndicator) View() tea.View {
	if !ti.active || ti.budgetTokens == 0 {
		return tea.NewView("")
	}

	cs := ti.theme.Colors()

	labelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(cs.Info)).
		Bold(true)
	valueStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(cs.TextPrimary))

	// Progress bar
	barWidth := ti.width - 30
	if barWidth < 10 {
		barWidth = 10
	}
	if barWidth > 40 {
		barWidth = 40
	}

	progress := float64(ti.usedTokens) / float64(ti.budgetTokens)
	if progress > 1 {
		progress = 1
	}
	filled := int(progress * float64(barWidth))

	bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)

	barStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(cs.Accent))

	label := labelStyle.Render("💭 Thinking: ")
	tokens := valueStyle.Render(fmt.Sprintf("%s / %s", formatTokens(ti.usedTokens), formatTokens(ti.budgetTokens)))
	progressBar := barStyle.Render(bar)

	return tea.NewView(label + tokens + " " + progressBar)
}

// UsedTokens returns the current used token count.
func (ti *ThinkingIndicator) UsedTokens() int { return ti.usedTokens }

// BudgetTokens returns the budget token count.
func (ti *ThinkingIndicator) BudgetTokens() int { return ti.budgetTokens }

// IsActive returns whether the indicator is active.
func (ti *ThinkingIndicator) IsActive() bool { return ti.active }

// Reset resets the indicator.
func (ti *ThinkingIndicator) Reset() {
	ti.usedTokens = 0
	ti.active = false
}

// SetSize sets the dimensions.
func (ti *ThinkingIndicator) SetSize(width, height int) {
	ti.width = width
	ti.height = height
}

func (ti *ThinkingIndicator) Focus()        { ti.focused = true }
func (ti *ThinkingIndicator) Blur()         { ti.focused = false }
func (ti *ThinkingIndicator) Focused() bool { return ti.focused }

var _ tea.Model = (*ThinkingIndicator)(nil)
