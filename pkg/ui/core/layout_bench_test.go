package core

import (
	"testing"
)

// BenchmarkVerticalStackCreate measures the time to create a VerticalStack.
// Target: <100µs per stack creation
func BenchmarkVerticalStackCreate(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = NewVerticalStack(
			&MockComponent{},
			&MockComponent{},
			&MockComponent{},
			&MockComponent{},
		)
	}
}

// BenchmarkVerticalStackSetSize measures the time to set size on a VerticalStack.
// Target: <100µs per SetSize call
func BenchmarkVerticalStackSetSize(b *testing.B) {
	stack := NewVerticalStack(
		&MockComponent{},
		&MockComponent{},
		&MockComponent{},
		&MockComponent{},
	)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		stack.SetSize(80, 24)
	}
}

// BenchmarkVerticalStackSetSizeManyComponents measures SetSize with more components.
// Target: <500µs per SetSize call (linear in number of components)
func BenchmarkVerticalStackSetSizeManyComponents(b *testing.B) {
	// Create stack with 20 components
	components := make([]Component, 20)
	for i := 0; i < 20; i++ {
		components[i] = &MockComponent{}
	}
	stack := NewVerticalStack(components...)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		stack.SetSize(80, 24)
	}
}

// BenchmarkVerticalStackAdd measures adding a component to a stack.
// Target: <10µs per Add call
func BenchmarkVerticalStackAdd(b *testing.B) {
	stack := NewVerticalStack(&MockComponent{})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		stack.Add(&MockComponent{})
	}
}

// BenchmarkVerticalStackAddFlexible measures adding a flexible component.
// Target: <10µs per AddFlexible call
func BenchmarkVerticalStackAddFlexible(b *testing.B) {
	stack := NewVerticalStack(&MockComponent{})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		stack.AddFlexible(&MockComponent{}, 1)
	}
}

// BenchmarkFocusManagerCreate measures the time to create a FocusManager.
// Target: <100µs per manager creation
func BenchmarkFocusManagerCreate(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = NewFocusManager()
	}
}

// BenchmarkFocusManagerAdd measures the time to add a component.
// Target: <10µs per add call
func BenchmarkFocusManagerAdd(b *testing.B) {
	fm := NewFocusManager()
	comp := &MockComponent{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		fm.Add(comp)
	}
}

// BenchmarkFocusManagerNext measures the time to advance focus.
// Target: <10µs per Next call
func BenchmarkFocusManagerNext(b *testing.B) {
	components := make([]Focusable, 10)
	for i := 0; i < 10; i++ {
		components[i] = &MockComponent{}
	}
	fm := NewFocusManager(components...)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		fm.Next()
	}
}

// BenchmarkFocusManagerNextMany measures Next with many components.
// Target: <50µs per Next call (linear in number of components)
func BenchmarkFocusManagerNextMany(b *testing.B) {
	components := make([]Focusable, 50)
	for i := 0; i < 50; i++ {
		components[i] = &MockComponent{}
	}
	fm := NewFocusManager(components...)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		fm.Next()
	}
}
