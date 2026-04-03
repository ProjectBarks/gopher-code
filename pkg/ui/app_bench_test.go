package ui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/projectbarks/gopher-code/pkg/session"
)

// BenchmarkAppModelCreate measures the time to create an AppModel.
// Target: <5ms per AppModel creation (startup time is critical)
func BenchmarkAppModelCreate(b *testing.B) {
	config := session.DefaultConfig()
	state := session.New(config, "/home/user")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = NewAppModel(state, nil)
	}
}

// BenchmarkAppModelInit measures the time to initialize an AppModel.
// Target: <10ms per Init call (startup sequence)
func BenchmarkAppModelInit(b *testing.B) {
	config := session.DefaultConfig()
	state := session.New(config, "/home/user")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		app := NewAppModel(state, nil)
		_ = app.Init()
	}
}

// BenchmarkAppModelUpdateWindowSize measures processing a resize event.
// Target: <1ms per Update (resize should be fast)
func BenchmarkAppModelUpdateWindowSize(b *testing.B) {
	config := session.DefaultConfig()
	state := session.New(config, "/home/user")
	app := NewAppModel(state, nil)

	resizeMsg := tea.WindowSizeMsg{Width: 80, Height: 24}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		app.Update(resizeMsg)
	}
}

// BenchmarkAppModelUpdateWindowSizeWide measures resize with wider terminal.
// Target: <2ms per Update (more text rendering)
func BenchmarkAppModelUpdateWindowSizeWide(b *testing.B) {
	config := session.DefaultConfig()
	state := session.New(config, "/home/user")
	app := NewAppModel(state, nil)

	resizeMsg := tea.WindowSizeMsg{Width: 200, Height: 50}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		app.Update(resizeMsg)
	}
}

// BenchmarkAppModelView measures the time to render the full AppModel.
// Target: <2ms per View call (real-time responsiveness)
func BenchmarkAppModelView(b *testing.B) {
	config := session.DefaultConfig()
	state := session.New(config, "/home/user")
	app := NewAppModel(state, nil)
	// Initialize with a window size
	app.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = app.View()
	}
}

// BenchmarkAppModelViewWide measures View rendering with wide terminal.
// Target: <5ms per View call (more content to render)
func BenchmarkAppModelViewWide(b *testing.B) {
	config := session.DefaultConfig()
	state := session.New(config, "/home/user")
	app := NewAppModel(state, nil)
	// Initialize with a larger window size
	app.Update(tea.WindowSizeMsg{Width: 200, Height: 50})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = app.View()
	}
}

// BenchmarkAppModelUpdateKeyPress measures processing a key press event.
// Target: <100µs per Update (sub-millisecond)
func BenchmarkAppModelUpdateKeyPress(b *testing.B) {
	config := session.DefaultConfig()
	state := session.New(config, "/home/user")
	app := NewAppModel(state, nil)
	app.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

	keyMsg := tea.KeyPressMsg{Code: 'a'}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		app.Update(keyMsg)
	}
}

// BenchmarkAppModelUpdateTab measures processing Tab for focus change.
// Target: <100µs per Update (instant focus switch)
func BenchmarkAppModelUpdateTab(b *testing.B) {
	config := session.DefaultConfig()
	state := session.New(config, "/home/user")
	app := NewAppModel(state, nil)
	app.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

	tabMsg := tea.KeyPressMsg{Code: tea.KeyTab}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		app.Update(tabMsg)
	}
}
