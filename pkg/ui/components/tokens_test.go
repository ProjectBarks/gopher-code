package components

import (
	"strings"
	"testing"

	"github.com/projectbarks/gopher-code/pkg/ui/theme"
)

func TestTokenUsageCreation(t *testing.T) {
	tud := NewTokenUsageDisplay(theme.Current())
	if tud == nil {
		t.Fatal("TokenUsageDisplay should not be nil")
	}
	if tud.InputTokens() != 0 || tud.OutputTokens() != 0 {
		t.Error("Initial tokens should be zero")
	}
}

func TestTokenUsageUpdate(t *testing.T) {
	tud := NewTokenUsageDisplay(theme.Current())
	tud.Update(UsageMsg{InputTokens: 100, OutputTokens: 50})
	if tud.InputTokens() != 100 {
		t.Errorf("Expected 100 input tokens, got %d", tud.InputTokens())
	}
	if tud.OutputTokens() != 50 {
		t.Errorf("Expected 50 output tokens, got %d", tud.OutputTokens())
	}
}

func TestTokenUsageAccumulates(t *testing.T) {
	tud := NewTokenUsageDisplay(theme.Current())
	tud.Update(UsageMsg{InputTokens: 100, OutputTokens: 50})
	tud.Update(UsageMsg{InputTokens: 200, OutputTokens: 75})
	if tud.InputTokens() != 300 {
		t.Errorf("Expected 300 input tokens, got %d", tud.InputTokens())
	}
	if tud.OutputTokens() != 125 {
		t.Errorf("Expected 125 output tokens, got %d", tud.OutputTokens())
	}
}

func TestTokenUsageView(t *testing.T) {
	tud := NewTokenUsageDisplay(theme.Current())
	tud.Update(UsageMsg{InputTokens: 1500, OutputTokens: 500})
	view := tud.View()
	plain := stripANSI(view.Content)
	if !strings.Contains(plain, "1.5k") {
		t.Errorf("Expected formatted token count, got %q", plain)
	}
}

func TestTokenUsageCostCalculation(t *testing.T) {
	tud := NewTokenUsageDisplay(theme.Current())
	tud.SetCostRates(&ModelCostRates{
		InputPerMillion:  3.0,
		OutputPerMillion: 15.0,
	})
	tud.Update(UsageMsg{InputTokens: 1_000_000, OutputTokens: 100_000})
	cost := tud.CalculateCost()
	expected := 3.0 + 1.5 // $3 input + $1.50 output
	if cost < expected-0.01 || cost > expected+0.01 {
		t.Errorf("Expected cost ~$%.2f, got $%.4f", expected, cost)
	}
}

func TestTokenUsageCostWithoutRates(t *testing.T) {
	tud := NewTokenUsageDisplay(theme.Current())
	tud.Update(UsageMsg{InputTokens: 1000, OutputTokens: 500})
	cost := tud.CalculateCost()
	if cost != 0 {
		t.Errorf("Expected 0 cost without rates, got %f", cost)
	}
}

func TestTokenUsageReset(t *testing.T) {
	tud := NewTokenUsageDisplay(theme.Current())
	tud.Update(UsageMsg{InputTokens: 100, OutputTokens: 50})
	tud.Reset()
	if tud.InputTokens() != 0 || tud.OutputTokens() != 0 {
		t.Error("Tokens should be zero after reset")
	}
}

func TestFormatTokens(t *testing.T) {
	tests := []struct {
		input    int
		expected string
	}{
		{0, "0"},
		{500, "500"},
		{1000, "1.0k"},
		{1500, "1.5k"},
		{1_000_000, "1.0M"},
		{2_500_000, "2.5M"},
	}
	for _, tt := range tests {
		result := formatTokens(tt.input)
		if result != tt.expected {
			t.Errorf("formatTokens(%d) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestTokenUsageCacheTokens(t *testing.T) {
	tud := NewTokenUsageDisplay(theme.Current())
	tud.Update(UsageMsg{InputTokens: 100, CacheCreation: 50, CacheRead: 200})
	view := tud.View()
	plain := stripANSI(view.Content)
	if !strings.Contains(plain, "Cache") {
		t.Error("Expected cache info in view when cache tokens present")
	}
}
