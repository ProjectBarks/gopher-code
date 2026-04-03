package components

import (
	"strings"
	"testing"

	"github.com/projectbarks/gopher-code/pkg/ui/theme"
)

func TestErrorDisplayCreation(t *testing.T) {
	ed := NewErrorDisplay(theme.Current())
	if ed == nil {
		t.Fatal("ErrorDisplay should not be nil")
	}
	if ed.HasErrors() {
		t.Error("Should have no errors initially")
	}
}

func TestErrorDisplayAddError(t *testing.T) {
	ed := NewErrorDisplay(theme.Current())
	ed.AddError(ErrorInfo{
		Type:     ErrorTypeGeneral,
		Severity: SeverityError,
		Message:  "something failed",
	})
	if !ed.HasErrors() {
		t.Error("Should have errors after AddError")
	}
	if len(ed.Errors()) != 1 {
		t.Errorf("Expected 1 error, got %d", len(ed.Errors()))
	}
}

func TestErrorDisplayClear(t *testing.T) {
	ed := NewErrorDisplay(theme.Current())
	ed.AddError(ErrorInfo{Message: "err1"})
	ed.AddError(ErrorInfo{Message: "err2"})
	ed.Clear()
	if ed.HasErrors() {
		t.Error("Should have no errors after Clear")
	}
}

func TestErrorDisplayViewEmpty(t *testing.T) {
	ed := NewErrorDisplay(theme.Current())
	view := ed.View()
	if view.Content != "" {
		t.Error("Empty display should render empty string")
	}
}

func TestErrorDisplayViewError(t *testing.T) {
	ed := NewErrorDisplay(theme.Current())
	ed.AddError(ErrorInfo{
		Type:     ErrorTypeNetwork,
		Severity: SeverityError,
		Message:  "connection refused",
		Suggestions: []string{"Check network"},
	})
	view := ed.View()
	plain := stripANSI(view.Content)
	if !strings.Contains(plain, "connection refused") {
		t.Error("Expected error message in output")
	}
	if !strings.Contains(plain, "Check network") {
		t.Error("Expected suggestion in output")
	}
}

func TestErrorDisplaySeverityColors(t *testing.T) {
	ed := NewErrorDisplay(theme.Current())

	// Error severity
	ed.AddError(ErrorInfo{Severity: SeverityError, Message: "error"})
	view1 := ed.View()
	if view1.Content == "" {
		t.Error("Error severity should render")
	}

	ed.Clear()

	// Warning severity
	ed.AddError(ErrorInfo{Severity: SeverityWarning, Message: "warning"})
	view2 := ed.View()
	if view2.Content == "" {
		t.Error("Warning severity should render")
	}

	ed.Clear()

	// Info severity
	ed.AddError(ErrorInfo{Severity: SeverityInfo, Message: "info"})
	view3 := ed.View()
	if view3.Content == "" {
		t.Error("Info severity should render")
	}
}

func TestClassifyErrorRateLimit(t *testing.T) {
	info := ClassifyError("rate limit exceeded")
	if info.Type != ErrorTypeRateLimit {
		t.Errorf("Expected rate_limit, got %s", info.Type)
	}
	if info.Severity != SeverityWarning {
		t.Error("Rate limit should be warning severity")
	}
}

func TestClassifyErrorAuth(t *testing.T) {
	info := ClassifyError("invalid api key")
	if info.Type != ErrorTypeAuth {
		t.Errorf("Expected auth, got %s", info.Type)
	}
}

func TestClassifyErrorNetwork(t *testing.T) {
	info := ClassifyError("connection timeout")
	if info.Type != ErrorTypeNetwork {
		t.Errorf("Expected network, got %s", info.Type)
	}
}

func TestClassifyErrorPermission(t *testing.T) {
	info := ClassifyError("permission denied")
	if info.Type != ErrorTypePermission {
		t.Errorf("Expected permission, got %s", info.Type)
	}
}

func TestClassifyErrorGeneral(t *testing.T) {
	info := ClassifyError("unknown error occurred")
	if info.Type != ErrorTypeGeneral {
		t.Errorf("Expected general, got %s", info.Type)
	}
}
