package components

import (
	"strings"
	"testing"

	"github.com/projectbarks/gopher-code/pkg/ui/theme"
)

func TestThinkingIndicatorCreation(t *testing.T) {
	ti := NewThinkingIndicator(theme.Current())
	if ti == nil {
		t.Fatal("ThinkingIndicator should not be nil")
	}
	if ti.IsActive() {
		t.Error("Should not be active initially")
	}
}

func TestThinkingIndicatorSetBudget(t *testing.T) {
	ti := NewThinkingIndicator(theme.Current())
	ti.SetBudget(10000)
	if ti.BudgetTokens() != 10000 {
		t.Errorf("Expected budget 10000, got %d", ti.BudgetTokens())
	}
}

func TestThinkingIndicatorUpdate(t *testing.T) {
	ti := NewThinkingIndicator(theme.Current())
	ti.Update(ThinkingEventMsg{TokensUsed: 5000, BudgetTokens: 10000})
	if ti.UsedTokens() != 5000 {
		t.Errorf("Expected 5000 used, got %d", ti.UsedTokens())
	}
	if !ti.IsActive() {
		t.Error("Should be active after event")
	}
}

func TestThinkingIndicatorViewInactive(t *testing.T) {
	ti := NewThinkingIndicator(theme.Current())
	view := ti.View()
	if view.Content != "" {
		t.Error("Inactive indicator should render empty")
	}
}

func TestThinkingIndicatorViewActive(t *testing.T) {
	ti := NewThinkingIndicator(theme.Current())
	ti.SetSize(80, 1)
	ti.Update(ThinkingEventMsg{TokensUsed: 5000, BudgetTokens: 10000})
	view := ti.View()
	plain := stripANSI(view.Content)
	if !strings.Contains(plain, "Thinking") {
		t.Error("Expected 'Thinking' in output")
	}
	if !strings.Contains(plain, "5.0k") {
		t.Errorf("Expected '5.0k' in output, got %q", plain)
	}
}

func TestThinkingIndicatorProgressBar(t *testing.T) {
	ti := NewThinkingIndicator(theme.Current())
	ti.SetSize(80, 1)
	ti.Update(ThinkingEventMsg{TokensUsed: 5000, BudgetTokens: 10000})
	view := ti.View()
	// Should contain progress characters
	if !strings.Contains(view.Content, "█") || !strings.Contains(view.Content, "░") {
		t.Error("Expected progress bar characters")
	}
}

func TestThinkingIndicatorReset(t *testing.T) {
	ti := NewThinkingIndicator(theme.Current())
	ti.Update(ThinkingEventMsg{TokensUsed: 5000, BudgetTokens: 10000})
	ti.Reset()
	if ti.IsActive() {
		t.Error("Should not be active after reset")
	}
	if ti.UsedTokens() != 0 {
		t.Error("Used tokens should be zero after reset")
	}
}
