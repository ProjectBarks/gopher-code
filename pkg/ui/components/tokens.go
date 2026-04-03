package components

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/projectbarks/gopher-code/pkg/ui/theme"
)

// UsageMsg carries token usage data.
type UsageMsg struct {
	InputTokens   int
	OutputTokens  int
	CacheCreation int
	CacheRead     int
}

// ModelCostRates holds per-model cost rates (per million tokens).
type ModelCostRates struct {
	InputPerMillion  float64
	OutputPerMillion float64
	CachePerMillion  float64
}

// TokenUsageDisplay shows input/output/cache token tracking with cost.
type TokenUsageDisplay struct {
	inputTokens   int
	outputTokens  int
	cacheCreation int
	cacheRead     int
	costRates     *ModelCostRates
	theme         theme.Theme
	width         int
	height        int
	focused       bool
}

// NewTokenUsageDisplay creates a new token usage display.
func NewTokenUsageDisplay(t theme.Theme) *TokenUsageDisplay {
	return &TokenUsageDisplay{
		theme: t,
		width: 80,
	}
}

// SetCostRates sets the per-model cost rates for cost calculation.
func (tud *TokenUsageDisplay) SetCostRates(rates *ModelCostRates) {
	tud.costRates = rates
}

// Init initializes the component.
func (tud *TokenUsageDisplay) Init() tea.Cmd { return nil }

// Update handles usage messages.
func (tud *TokenUsageDisplay) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case UsageMsg:
		tud.inputTokens += msg.InputTokens
		tud.outputTokens += msg.OutputTokens
		tud.cacheCreation += msg.CacheCreation
		tud.cacheRead += msg.CacheRead
	}
	return tud, nil
}

// View renders the token usage display.
func (tud *TokenUsageDisplay) View() tea.View {
	cs := tud.theme.Colors()
	var parts []string

	labelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(cs.TextSecondary))
	valueStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(cs.TextPrimary)).
		Bold(true)

	parts = append(parts, labelStyle.Render("In: ")+valueStyle.Render(formatTokens(tud.inputTokens)))
	parts = append(parts, labelStyle.Render("Out: ")+valueStyle.Render(formatTokens(tud.outputTokens)))

	if tud.cacheCreation > 0 || tud.cacheRead > 0 {
		parts = append(parts, labelStyle.Render("Cache: ")+
			valueStyle.Render(fmt.Sprintf("%s/%s", formatTokens(tud.cacheCreation), formatTokens(tud.cacheRead))))
	}

	// Cost calculation
	if tud.costRates != nil {
		cost := tud.CalculateCost()
		if cost > 0 {
			costStyle := lipgloss.NewStyle().
				Foreground(lipgloss.Color(cs.Accent))
			parts = append(parts, costStyle.Render(fmt.Sprintf("$%.4f", cost)))
		}
	}

	return tea.NewView(strings.Join(parts, " │ "))
}

// CalculateCost computes the total cost based on token usage and rates.
func (tud *TokenUsageDisplay) CalculateCost() float64 {
	if tud.costRates == nil {
		return 0
	}
	inputCost := float64(tud.inputTokens) / 1_000_000 * tud.costRates.InputPerMillion
	outputCost := float64(tud.outputTokens) / 1_000_000 * tud.costRates.OutputPerMillion
	cacheCost := float64(tud.cacheCreation) / 1_000_000 * tud.costRates.CachePerMillion
	return inputCost + outputCost + cacheCost
}

// InputTokens returns the current input token count.
func (tud *TokenUsageDisplay) InputTokens() int { return tud.inputTokens }

// OutputTokens returns the current output token count.
func (tud *TokenUsageDisplay) OutputTokens() int { return tud.outputTokens }

// Reset resets all token counts.
func (tud *TokenUsageDisplay) Reset() {
	tud.inputTokens = 0
	tud.outputTokens = 0
	tud.cacheCreation = 0
	tud.cacheRead = 0
}

// SetSize sets the dimensions.
func (tud *TokenUsageDisplay) SetSize(width, height int) {
	tud.width = width
	tud.height = height
}

func (tud *TokenUsageDisplay) Focus()        { tud.focused = true }
func (tud *TokenUsageDisplay) Blur()         { tud.focused = false }
func (tud *TokenUsageDisplay) Focused() bool { return tud.focused }

// formatTokens formats a token count for display.
func formatTokens(n int) string {
	if n >= 1_000_000 {
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	}
	if n >= 1_000 {
		return fmt.Sprintf("%.1fk", float64(n)/1_000)
	}
	return fmt.Sprintf("%d", n)
}

var _ tea.Model = (*TokenUsageDisplay)(nil)
