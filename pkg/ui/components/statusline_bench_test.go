package components

import (
	"testing"

	"github.com/projectbarks/gopher-code/pkg/session"
)

// BenchmarkStatusLineCreate measures the time to create a StatusLine component.
// Target: <1ms per StatusLine creation
func BenchmarkStatusLineCreate(b *testing.B) {
	config := session.DefaultConfig()
	state := session.New(config, "/home/user")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = NewStatusLine(state)
	}
}

// BenchmarkStatusLineSetSize measures the time to set size on a StatusLine.
// Target: <100µs per SetSize call (sub-millisecond)
func BenchmarkStatusLineSetSize(b *testing.B) {
	config := session.DefaultConfig()
	state := session.New(config, "/home/user")
	sl := NewStatusLine(state)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sl.SetSize(80, 1)
	}
}

// BenchmarkStatusLineView measures the time to render the StatusLine.
// Target: <100µs per View call (sub-millisecond)
func BenchmarkStatusLineView(b *testing.B) {
	config := session.DefaultConfig()
	state := session.New(config, "/home/user")
	sl := NewStatusLine(state)
	sl.SetSize(80, 1)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = sl.View()
	}
}

// BenchmarkStatusLineViewLongModelName measures rendering with long model names.
// Target: <200µs per View call (still sub-millisecond)
func BenchmarkStatusLineViewLongModelName(b *testing.B) {
	config := session.DefaultConfig()
	config.Model = "very-long-model-name-that-should-be-abbreviated-for-display"
	state := session.New(config, "/home/user")
	sl := NewStatusLine(state)
	sl.SetSize(80, 1)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = sl.View()
	}
}

// BenchmarkStatusLineUpdate measures the time to process a mode change message.
// Target: <100µs per Update call (sub-millisecond)
func BenchmarkStatusLineUpdate(b *testing.B) {
	config := session.DefaultConfig()
	state := session.New(config, "/home/user")
	sl := NewStatusLine(state)
	sl.SetSize(80, 1)

	msg := ModeChangeMsg{Mode: ModeStreaming}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sl.Update(msg)
	}
}
