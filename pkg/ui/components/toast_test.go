package components

import (
	"strings"
	"testing"
	"time"

	"github.com/projectbarks/gopher-code/pkg/ui/theme"
)

func TestNotificationToastCreation(t *testing.T) {
	nt := NewNotificationToast(theme.Current())
	if nt == nil {
		t.Fatal("NotificationToast should not be nil")
	}
	if nt.HasToasts() {
		t.Error("Should have no toasts initially")
	}
}

func TestNotificationToastAdd(t *testing.T) {
	nt := NewNotificationToast(theme.Current())
	nt.Update(ToastMsg{
		Message:  "File saved",
		Type:     ToastSuccess,
		Duration: 5 * time.Second,
	})
	if !nt.HasToasts() {
		t.Error("Should have toast after adding")
	}
	if nt.Count() != 1 {
		t.Errorf("Expected 1 toast, got %d", nt.Count())
	}
}

func TestNotificationToastViewSuccess(t *testing.T) {
	nt := NewNotificationToast(theme.Current())
	nt.Update(ToastMsg{
		Message:  "Operation successful",
		Type:     ToastSuccess,
		Duration: 5 * time.Second,
	})
	view := nt.View()
	plain := stripANSI(view.Content)
	if !strings.Contains(plain, "Operation successful") {
		t.Error("Expected message in output")
	}
	if !strings.Contains(plain, "✓") {
		t.Error("Expected success icon")
	}
}

func TestNotificationToastViewError(t *testing.T) {
	nt := NewNotificationToast(theme.Current())
	nt.Update(ToastMsg{
		Message:  "Something failed",
		Type:     ToastError,
		Duration: 5 * time.Second,
	})
	view := nt.View()
	plain := stripANSI(view.Content)
	if !strings.Contains(plain, "Something failed") {
		t.Error("Expected error message")
	}
	if !strings.Contains(plain, "✗") {
		t.Error("Expected error icon")
	}
}

func TestNotificationToastViewInfo(t *testing.T) {
	nt := NewNotificationToast(theme.Current())
	nt.Update(ToastMsg{
		Message:  "FYI",
		Type:     ToastInfo,
		Duration: 5 * time.Second,
	})
	view := nt.View()
	plain := stripANSI(view.Content)
	if !strings.Contains(plain, "FYI") {
		t.Error("Expected info message")
	}
}

func TestNotificationToastEmpty(t *testing.T) {
	nt := NewNotificationToast(theme.Current())
	view := nt.View()
	if view.Content != "" {
		t.Error("Empty toast should render empty")
	}
}

func TestNotificationToastClear(t *testing.T) {
	nt := NewNotificationToast(theme.Current())
	nt.Update(ToastMsg{Message: "msg1", Duration: 5 * time.Second})
	nt.Update(ToastMsg{Message: "msg2", Duration: 5 * time.Second})
	nt.Clear()
	if nt.HasToasts() {
		t.Error("Should have no toasts after clear")
	}
}

func TestNotificationToastMultiple(t *testing.T) {
	nt := NewNotificationToast(theme.Current())
	nt.Update(ToastMsg{Message: "first", Duration: 5 * time.Second})
	nt.Update(ToastMsg{Message: "second", Duration: 5 * time.Second})
	if nt.Count() != 2 {
		t.Errorf("Expected 2 toasts, got %d", nt.Count())
	}
	// View shows the most recent
	view := nt.View()
	plain := stripANSI(view.Content)
	if !strings.Contains(plain, "second") {
		t.Error("Expected most recent toast in view")
	}
}

func TestNotificationToastDefaultDuration(t *testing.T) {
	nt := NewNotificationToast(theme.Current())
	nt.Update(ToastMsg{Message: "no duration"})
	if nt.Count() != 1 {
		t.Error("Should add toast with default duration")
	}
}
