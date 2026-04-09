package components

import (
	"testing"
)

func TestNewTextInput(t *testing.T) {
	ti := NewTextInput("Type here...")

	if ti.Value() != "" {
		t.Error("initial value should be empty")
	}
	if ti.placeholder != "Type here..." {
		t.Errorf("placeholder = %q", ti.placeholder)
	}
	if ti.Focused() {
		t.Error("should not be focused initially")
	}
}

func TestTextInput_SetValue(t *testing.T) {
	ti := NewTextInput("")
	ti.SetValue("hello world")
	if ti.Value() != "hello world" {
		t.Errorf("Value() = %q, want 'hello world'", ti.Value())
	}
}

func TestTextInput_IsEmpty(t *testing.T) {
	ti := NewTextInput("")
	if !ti.IsEmpty() {
		t.Error("should be empty initially")
	}
	ti.SetValue("x")
	if ti.IsEmpty() {
		t.Error("should not be empty after SetValue")
	}
}

func TestTextInput_Reset(t *testing.T) {
	ti := NewTextInput("")
	ti.SetValue("something")
	ti.Reset()
	if ti.Value() != "" {
		t.Errorf("after Reset, Value() = %q", ti.Value())
	}
}

func TestTextInput_FocusBlur(t *testing.T) {
	ti := NewTextInput("")
	ti.Focus()
	if !ti.Focused() {
		t.Error("should be focused after Focus()")
	}
	ti.Blur()
	if ti.Focused() {
		t.Error("should not be focused after Blur()")
	}
}

func TestTextInput_SetWidth(t *testing.T) {
	ti := NewTextInput("")
	ti.SetWidth(120)
	if ti.width != 120 {
		t.Errorf("width = %d, want 120", ti.width)
	}
}

func TestTextInput_View(t *testing.T) {
	ti := NewTextInput("Enter text")
	v := ti.View()
	// View should return a string (may be empty without focus)
	_ = v
}
