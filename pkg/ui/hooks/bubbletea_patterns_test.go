package hooks

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

// --- IntervalTicker ---

func TestIntervalTicker_Start(t *testing.T) {
	ticker := NewIntervalTicker("refresh", 100*time.Millisecond)
	cmd := ticker.Start()
	if cmd == nil {
		t.Error("Start should return a cmd")
	}
	if !ticker.Active {
		t.Error("should be active")
	}
}

func TestIntervalTicker_Stop(t *testing.T) {
	ticker := NewIntervalTicker("t", time.Second)
	ticker.Stop()
	if ticker.Active {
		t.Error("should be inactive after Stop")
	}
	if ticker.Start() != nil {
		t.Error("stopped ticker should return nil cmd")
	}
}

func TestIntervalTicker_Resume(t *testing.T) {
	ticker := NewIntervalTicker("t", time.Second)
	ticker.Stop()
	ticker.Resume()
	if !ticker.Active {
		t.Error("should be active after Resume")
	}
}

// --- AnimationFrame ---

func TestAnimationFrame_Start(t *testing.T) {
	af := NewAnimationFrame()
	cmd := af.Start()
	if cmd == nil {
		t.Error("Start should return a cmd")
	}
	if !af.IsActive() {
		t.Error("should be active")
	}
}

func TestAnimationFrame_Stop(t *testing.T) {
	af := NewAnimationFrame()
	af.Stop()
	if af.IsActive() {
		t.Error("should not be active after Stop")
	}
}

func TestAnimationFrame_Frame(t *testing.T) {
	af := NewAnimationFrame()
	if af.Frame() != 0 {
		t.Error("should start at frame 0")
	}
	af.Next()
	if af.Frame() != 1 {
		t.Errorf("after Next: frame = %d", af.Frame())
	}
}

// --- TabStatus ---

func TestTabStatus_Set(t *testing.T) {
	var buf bytes.Buffer
	ts := NewTabStatus(&buf)

	ts.Set(TabStatusBusy)
	got := buf.String()
	if !strings.Contains(got, "21337") {
		t.Error("should contain OSC 21337")
	}
	if !strings.Contains(got, "Working") {
		t.Error("busy should show Working")
	}
	if ts.Kind() != TabStatusBusy {
		t.Errorf("kind = %q", ts.Kind())
	}
}

func TestTabStatus_Clear(t *testing.T) {
	var buf bytes.Buffer
	ts := NewTabStatus(&buf)

	ts.Set(TabStatusIdle)
	buf.Reset()
	ts.Clear()
	got := buf.String()
	if !strings.Contains(got, "21337") {
		t.Error("clear should use OSC 21337")
	}
	if !strings.Contains(got, "indicator=;") {
		t.Error("clear should have empty indicator")
	}
}

func TestTabStatus_NilWriter(t *testing.T) {
	ts := NewTabStatus(nil)
	ts.Set(TabStatusBusy)  // should not panic
	ts.Clear()             // should not panic
}

func TestTabStatus_Idle(t *testing.T) {
	var buf bytes.Buffer
	ts := NewTabStatus(&buf)
	ts.Set(TabStatusIdle)
	if !strings.Contains(buf.String(), "Idle") {
		t.Error("idle should show Idle")
	}
}

func TestTabStatus_Waiting(t *testing.T) {
	var buf bytes.Buffer
	ts := NewTabStatus(&buf)
	ts.Set(TabStatusWaiting)
	if !strings.Contains(buf.String(), "Waiting") {
		t.Error("waiting should show Waiting")
	}
}

// --- CursorPosition ---

func TestDefaultCursorPosition(t *testing.T) {
	cp := DefaultCursorPosition()
	if cp.Visible {
		t.Error("default should be hidden")
	}
}

// --- ViewportState ---

func TestNewViewportState(t *testing.T) {
	vs := NewViewportState(80, 24)
	if vs.Width != 80 || vs.Height != 24 {
		t.Errorf("got %dx%d", vs.Width, vs.Height)
	}
}

func TestViewportState_HandleResize(t *testing.T) {
	vs := NewViewportState(80, 24)
	vs.HandleResize(120, 40)
	if vs.Width != 120 || vs.Height != 40 {
		t.Errorf("after resize: %dx%d", vs.Width, vs.Height)
	}
}

func TestTabStatusKind_Constants(t *testing.T) {
	if TabStatusIdle != "idle" {
		t.Error("wrong")
	}
	if TabStatusBusy != "busy" {
		t.Error("wrong")
	}
	if TabStatusWaiting != "waiting" {
		t.Error("wrong")
	}
}
