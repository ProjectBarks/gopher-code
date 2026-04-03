package components

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

// BenchmarkInputPaneCreate measures the time to create an InputPane.
// Target: <500µs per InputPane creation
func BenchmarkInputPaneCreate(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = NewInputPane()
	}
}

// BenchmarkInputPaneSetSize measures the time to set size on an InputPane.
// Target: <100µs per SetSize call
func BenchmarkInputPaneSetSize(b *testing.B) {
	pane := NewInputPane()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pane.SetSize(80, 3)
	}
}

// BenchmarkInputPaneView measures the time to render the InputPane.
// Target: <100µs per View call
func BenchmarkInputPaneView(b *testing.B) {
	pane := NewInputPane()
	pane.SetSize(80, 3)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = pane.View()
	}
}

// BenchmarkInputPaneViewWithText measures rendering with user-typed text.
// Target: <200µs per View call (more content)
func BenchmarkInputPaneViewWithText(b *testing.B) {
	pane := NewInputPane()
	pane.SetSize(80, 3)
	pane.SetValue("This is some user-typed text that is being displayed in the input pane")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = pane.View()
	}
}

// BenchmarkInputPaneUpdate measures processing keyboard input.
// Target: <100µs per Update call
func BenchmarkInputPaneUpdate(b *testing.B) {
	pane := NewInputPane()
	pane.SetSize(80, 3)
	pane.Focus()

	// Simulate character input
	msg := tea.KeyPressMsg{Code: 'a'}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pane.Update(msg)
	}
}

// BenchmarkInputPaneSetValue measures setting the input value.
// Target: <100µs per SetValue call
func BenchmarkInputPaneSetValue(b *testing.B) {
	pane := NewInputPane()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pane.SetValue("test command")
	}
}

// BenchmarkInputPaneSetValueLong measures setting a long input value.
// Target: <200µs per SetValue call (longer text)
func BenchmarkInputPaneSetValueLong(b *testing.B) {
	pane := NewInputPane()
	longText := "This is a very long command that contains multiple lines and a lot of text " +
		"that should be handled efficiently by the input pane. " +
		"It tests the performance of setting large amounts of text."

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pane.SetValue(longText)
	}
}

// BenchmarkInputPaneValue measures getting the current input value.
// Target: <10µs per Value() call
func BenchmarkInputPaneValue(b *testing.B) {
	pane := NewInputPane()
	pane.SetValue("test input value")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = pane.Value()
	}
}

// BenchmarkInputPaneFocus measures setting focus.
// Target: <10µs per Focus call
func BenchmarkInputPaneFocus(b *testing.B) {
	pane := NewInputPane()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pane.Focus()
	}
}

// BenchmarkInputPaneBlur measures removing focus.
// Target: <10µs per Blur call
func BenchmarkInputPaneBlur(b *testing.B) {
	pane := NewInputPane()
	pane.Focus()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pane.Blur()
	}
}
