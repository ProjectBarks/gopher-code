package core

import tea "charm.land/bubbletea/v2"

// FocusManager implements a focus ring for cycling through focusable children.
type FocusManager struct {
	children   []Focusable
	current    int
	modalStack []Focusable // Modal override: top of stack takes focus
}

// NewFocusManager creates a new focus manager.
func NewFocusManager(children ...Focusable) *FocusManager {
	fm := &FocusManager{
		children:   children,
		current:    0,
		modalStack: []Focusable{},
	}
	if len(children) > 0 {
		children[0].Focus()
	}
	return fm
}

// Add appends a focusable child.
func (fm *FocusManager) Add(f Focusable) {
	fm.children = append(fm.children, f)
	if len(fm.children) == 1 {
		f.Focus()
		fm.current = 0
	}
}

// Focused returns the currently focused element.
func (fm *FocusManager) Focused() Focusable {
	if len(fm.modalStack) > 0 {
		return fm.modalStack[len(fm.modalStack)-1]
	}
	if fm.current < len(fm.children) {
		return fm.children[fm.current]
	}
	return nil
}

// Next moves focus to the next child in the ring.
func (fm *FocusManager) Next() {
	if len(fm.modalStack) > 0 {
		return // Modal is focused, don't cycle
	}
	if len(fm.children) == 0 {
		return
	}

	fm.children[fm.current].Blur()
	fm.current = (fm.current + 1) % len(fm.children)
	fm.children[fm.current].Focus()
}

// Prev moves focus to the previous child in the ring.
func (fm *FocusManager) Prev() {
	if len(fm.modalStack) > 0 {
		return // Modal is focused, don't cycle
	}
	if len(fm.children) == 0 {
		return
	}

	fm.children[fm.current].Blur()
	fm.current = (fm.current - 1 + len(fm.children)) % len(fm.children)
	fm.children[fm.current].Focus()
}

// PushModal adds a modal to the stack. The modal takes focus.
func (fm *FocusManager) PushModal(m Focusable) {
	if focused := fm.Focused(); focused != nil {
		focused.Blur()
	}
	fm.modalStack = append(fm.modalStack, m)
	m.Focus()
}

// PopModal removes the top modal from the stack and restores focus to the previous element.
func (fm *FocusManager) PopModal() {
	if len(fm.modalStack) == 0 {
		return
	}

	modal := fm.modalStack[len(fm.modalStack)-1]
	modal.Blur()
	fm.modalStack = fm.modalStack[:len(fm.modalStack)-1]

	if len(fm.modalStack) > 0 {
		fm.modalStack[len(fm.modalStack)-1].Focus()
	} else if fm.current < len(fm.children) {
		fm.children[fm.current].Focus()
	}
}

// ModalActive returns true if there's an active modal.
func (fm *FocusManager) ModalActive() bool {
	return len(fm.modalStack) > 0
}

// Route routes messages to the focused element.
func (fm *FocusManager) Route(msg tea.Msg) tea.Cmd {
	focused := fm.Focused()
	if focused == nil {
		return nil
	}

	_, cmd := focused.Update(msg)
	return cmd
}
