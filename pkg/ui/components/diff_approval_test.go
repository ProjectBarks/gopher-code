package components

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/projectbarks/gopher-code/pkg/ui/theme"
)

func TestDiffApprovalDialogCreation(t *testing.T) {
	ch := make(chan ApprovalResult, 1)
	dad := NewDiffApprovalDialog("bash", "t1", testDiff, theme.Current(), ch)
	if dad == nil {
		t.Fatal("DiffApprovalDialog should not be nil")
	}
	if dad.Result() != ApprovalPending {
		t.Error("Initial result should be pending")
	}
}

func TestDiffApprovalDialogView(t *testing.T) {
	ch := make(chan ApprovalResult, 1)
	dad := NewDiffApprovalDialog("bash", "t1", testDiff, theme.Current(), ch)
	dad.SetSize(80, 24)
	view := dad.View()
	plain := stripANSI(view.Content)
	if !strings.Contains(plain, "bash") {
		t.Error("Expected tool name in view")
	}
	if !strings.Contains(plain, "Approve") {
		t.Error("Expected approve button")
	}
	if !strings.Contains(plain, "Reject") {
		t.Error("Expected reject button")
	}
}

func TestDiffApprovalDialogApprove(t *testing.T) {
	ch := make(chan ApprovalResult, 1)
	dad := NewDiffApprovalDialog("bash", "t1", "", theme.Current(), ch)
	dad.Update(tea.KeyPressMsg{Code: 'y'})
	if dad.Result() != ApprovalApproved {
		t.Error("Expected approved result")
	}
	// Should send to channel
	select {
	case result := <-ch:
		if result != ApprovalApproved {
			t.Error("Channel should receive ApprovalApproved")
		}
	default:
		t.Error("Expected result on channel")
	}
}

func TestDiffApprovalDialogReject(t *testing.T) {
	ch := make(chan ApprovalResult, 1)
	dad := NewDiffApprovalDialog("bash", "t1", "", theme.Current(), ch)
	dad.Update(tea.KeyPressMsg{Code: 'n'})
	if dad.Result() != ApprovalRejected {
		t.Error("Expected rejected result")
	}
	select {
	case result := <-ch:
		if result != ApprovalRejected {
			t.Error("Channel should receive ApprovalRejected")
		}
	default:
		t.Error("Expected result on channel")
	}
}

func TestDiffApprovalDialogAlways(t *testing.T) {
	ch := make(chan ApprovalResult, 1)
	dad := NewDiffApprovalDialog("bash", "t1", "", theme.Current(), ch)
	dad.Update(tea.KeyPressMsg{Code: 'a'})
	if dad.Result() != ApprovalAlways {
		t.Error("Expected always result")
	}
}

func TestDiffApprovalDialogEnterApproves(t *testing.T) {
	ch := make(chan ApprovalResult, 1)
	dad := NewDiffApprovalDialog("bash", "t1", "", theme.Current(), ch)
	dad.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if dad.Result() != ApprovalApproved {
		t.Error("Enter should approve")
	}
}

func TestDiffApprovalDialogNilChannel(t *testing.T) {
	// Should not panic with nil channel
	dad := NewDiffApprovalDialog("bash", "t1", "", theme.Current(), nil)
	dad.Update(tea.KeyPressMsg{Code: 'y'})
	if dad.Result() != ApprovalApproved {
		t.Error("Should still set result with nil channel")
	}
}

func TestDiffApprovalDialogFocus(t *testing.T) {
	dad := NewDiffApprovalDialog("bash", "t1", "", theme.Current(), nil)
	if dad.Focused() {
		t.Error("Should not be focused initially")
	}
	dad.Focus()
	if !dad.Focused() {
		t.Error("Should be focused after Focus()")
	}
}
