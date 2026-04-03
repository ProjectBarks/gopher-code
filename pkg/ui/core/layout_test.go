package core

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// TestVerticalStackBasic tests basic vertical stacking.
func TestVerticalStackBasic(t *testing.T) {
	c1 := &MockComponent{}
	c2 := &MockComponent{}
	c3 := &MockComponent{}

	vs := NewVerticalStack(c1, c2, c3)

	if len(vs.children) != 3 {
		t.Errorf("Expected 3 children, got %d", len(vs.children))
	}
}

// TestVerticalStackSetSize tests height distribution.
func TestVerticalStackSetSize(t *testing.T) {
	c1 := &MockComponent{}
	c2 := &MockComponent{}
	c3 := &MockComponent{}

	vs := NewVerticalStack()
	vs.AddFixed(c1, 5)
	vs.AddFlexible(c2, 1)
	vs.AddFixed(c3, 3)

	vs.SetSize(80, 20)

	// c1: 5, c3: 3, c2: remaining 12
	if c1.height != 5 {
		t.Errorf("c1 height: got %d, want 5", c1.height)
	}
	if c3.height != 3 {
		t.Errorf("c3 height: got %d, want 3", c3.height)
	}
	if c2.height != 12 {
		t.Errorf("c2 height: got %d, want 12", c2.height)
	}
	if c1.width != 80 || c2.width != 80 || c3.width != 80 {
		t.Error("All children should have full width")
	}
}

// TestVerticalStackFlexible tests flex weight distribution.
func TestVerticalStackFlexible(t *testing.T) {
	c1 := &MockComponent{}
	c2 := &MockComponent{}
	c3 := &MockComponent{}

	vs := NewVerticalStack()
	vs.AddFlexible(c1, 1)
	vs.AddFlexible(c2, 2)
	vs.AddFlexible(c3, 1)

	vs.SetSize(80, 20)

	// Proportions: 1/4, 2/4, 1/4
	// Heights: 5, 10, 5
	if c1.height != 5 {
		t.Errorf("c1 height: got %d, want 5", c1.height)
	}
	if c2.height != 10 {
		t.Errorf("c2 height: got %d, want 10", c2.height)
	}
	if c3.height != 5 {
		t.Errorf("c3 height: got %d, want 5", c3.height)
	}
}

// TestVerticalStackView tests rendering.
func TestVerticalStackView(t *testing.T) {
	c1 := &testRenderComponent{text: "Header"}
	c2 := &testRenderComponent{text: "Content"}
	c3 := &testRenderComponent{text: "Footer"}

	vs := NewVerticalStack(c1, c2, c3)
	vs.SetSize(80, 3)

	view := vs.View()
	lines := strings.Split(view.Content, "\n")

	if len(lines) != 3 {
		t.Errorf("Expected 3 lines, got %d", len(lines))
	}
	if lines[0] != "Header" || lines[1] != "Content" || lines[2] != "Footer" {
		t.Errorf("View output mismatch: %v", lines)
	}
}

// TestVerticalStackUpdate routes messages to children.
func TestVerticalStackUpdate(t *testing.T) {
	c1 := &MockComponent{}
	c2 := &MockComponent{}

	vs := NewVerticalStack(c1, c2)
	vs.Update(nil)

	if c1.updates != 1 || c2.updates != 1 {
		t.Errorf("Update routing failed: c1=%d, c2=%d", c1.updates, c2.updates)
	}
}

// TestVerticalStackInit initializes children.
func TestVerticalStackInit(t *testing.T) {
	c1 := &MockComponent{}
	c2 := &MockComponent{}

	vs := NewVerticalStack(c1, c2)
	vs.Init()

	if !c1.init || !c2.init {
		t.Errorf("Init routing failed: c1=%v, c2=%v", c1.init, c2.init)
	}
}

// TestHorizontalStackBasic tests basic horizontal stacking.
func TestHorizontalStackBasic(t *testing.T) {
	c1 := &MockComponent{}
	c2 := &MockComponent{}

	hs := NewHorizontalStack(c1, c2)

	if len(hs.children) != 2 {
		t.Errorf("Expected 2 children, got %d", len(hs.children))
	}
}

// TestHorizontalStackSetSize tests width distribution.
func TestHorizontalStackSetSize(t *testing.T) {
	c1 := &MockComponent{}
	c2 := &MockComponent{}
	c3 := &MockComponent{}

	hs := NewHorizontalStack()
	hs.AddFixed(c1, 10)
	hs.AddFlexible(c2, 1)
	hs.AddFixed(c3, 10)

	hs.SetSize(40, 10)

	// c1: 10, c3: 10, c2: remaining 20
	if c1.width != 10 {
		t.Errorf("c1 width: got %d, want 10", c1.width)
	}
	if c3.width != 10 {
		t.Errorf("c3 width: got %d, want 10", c3.width)
	}
	if c2.width != 20 {
		t.Errorf("c2 width: got %d, want 20", c2.width)
	}
	if c1.height != 10 || c2.height != 10 || c3.height != 10 {
		t.Error("All children should have full height")
	}
}

// TestHorizontalStackInit tests initialization of children.
func TestHorizontalStackInit(t *testing.T) {
	c1 := &MockComponent{}
	c2 := &MockComponent{}

	hs := NewHorizontalStack(c1, c2)
	hs.Init()

	if !c1.init || !c2.init {
		t.Errorf("Init routing failed: c1=%v, c2=%v", c1.init, c2.init)
	}
}

// TestHorizontalStackUpdate tests message routing to children.
func TestHorizontalStackUpdate(t *testing.T) {
	c1 := &MockComponent{}
	c2 := &MockComponent{}

	hs := NewHorizontalStack(c1, c2)
	hs.Update(nil)

	if c1.updates != 1 || c2.updates != 1 {
		t.Errorf("Update routing failed: c1=%d, c2=%d", c1.updates, c2.updates)
	}
}

// TestHorizontalStackView tests rendering.
func TestHorizontalStackView(t *testing.T) {
	c1 := &testRenderComponent{text: "Left"}
	c2 := &testRenderComponent{text: "Right"}

	hs := NewHorizontalStack(c1, c2)
	view := hs.View()

	if view.Content != "LeftRight" {
		t.Errorf("Expected 'LeftRight', got %q", view.Content)
	}
}

// TestHorizontalStackAdd tests adding children.
func TestHorizontalStackAdd(t *testing.T) {
	hs := NewHorizontalStack()
	c1 := &MockComponent{}
	hs.Add(c1)
	if len(hs.children) != 1 {
		t.Error("Add should append child")
	}
}

// TestVerticalStackAdd tests adding children.
func TestVerticalStackAdd(t *testing.T) {
	vs := NewVerticalStack()
	c1 := &MockComponent{}
	vs.Add(c1)
	if len(vs.children) != 1 {
		t.Error("Add should append child")
	}
}

// TestVerticalStackEmptySetSize tests SetSize with no children.
func TestVerticalStackEmptySetSize(t *testing.T) {
	vs := NewVerticalStack()
	// Should not panic
	vs.SetSize(80, 24)
}

// TestHorizontalStackEmptySetSize tests SetSize with no children.
func TestHorizontalStackEmptySetSize(t *testing.T) {
	hs := NewHorizontalStack()
	// Should not panic
	hs.SetSize(80, 24)
}

// TestHorizontalStackFlexibleWeights tests flex width distribution.
func TestHorizontalStackFlexibleWeights(t *testing.T) {
	c1 := &MockComponent{}
	c2 := &MockComponent{}

	hs := NewHorizontalStack()
	hs.AddFlexible(c1, 1)
	hs.AddFlexible(c2, 3)
	hs.SetSize(40, 10)

	// 1:3 ratio = 10:30
	if c1.width != 10 {
		t.Errorf("c1 width: got %d, want 10", c1.width)
	}
	if c2.width != 30 {
		t.Errorf("c2 width: got %d, want 30", c2.width)
	}
}

// TestFocusManagerModalActive tests ModalActive method.
func TestFocusManagerModalActiveMethod(t *testing.T) {
	fm := NewFocusManager(&MockComponent{})
	if fm.ModalActive() {
		t.Error("Should not have active modal initially")
	}
	fm.PushModal(&MockComponent{})
	if !fm.ModalActive() {
		t.Error("Should have active modal after PushModal")
	}
}

// testRenderComponent is a test component that renders a specific text.
type testRenderComponent struct {
	text string
}

func (t *testRenderComponent) Init() tea.Cmd     { return nil }
func (t *testRenderComponent) Update(tea.Msg) (tea.Model, tea.Cmd) { return t, nil }
func (t *testRenderComponent) View() tea.View    { return tea.NewView(t.text) }
func (t *testRenderComponent) SetSize(int, int)  {}
