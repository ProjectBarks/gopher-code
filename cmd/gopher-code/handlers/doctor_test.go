package handlers

import (
	"bytes"
	"testing"
)

func TestDoctor_HandlerExists(t *testing.T) {
	var buf bytes.Buffer
	code := Doctor(DoctorOpts{Output: &buf})
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	out := buf.String()
	if out == "" {
		t.Error("doctor handler produced no output")
	}
}

func TestDoctor_ReturnsZero(t *testing.T) {
	var buf bytes.Buffer
	code := Doctor(DoctorOpts{Output: &buf})
	if code != 0 {
		t.Errorf("expected exit code 0, got %d", code)
	}
}
