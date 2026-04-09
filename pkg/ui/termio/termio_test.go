package termio

import (
	"strings"
	"testing"
)

func TestCSI(t *testing.T) {
	if CSI() != "\x1b[" {
		t.Error("empty CSI should be ESC [")
	}
	if CSI("2J") != "\x1b[2J" {
		t.Errorf("CSI(2J) = %q", CSI("2J"))
	}
	if CSI(5, "A") != "\x1b[5A" {
		t.Errorf("CSI(5,A) = %q", CSI(5, "A"))
	}
	if CSI(10, 20, "H") != "\x1b[10;20H" {
		t.Errorf("CSI(10,20,H) = %q", CSI(10, 20, "H"))
	}
}

func TestCursorMovement(t *testing.T) {
	if CursorUp(3) != "\x1b[3A" {
		t.Errorf("CursorUp(3) = %q", CursorUp(3))
	}
	if CursorDown(1) != "\x1b[1B" {
		t.Errorf("CursorDown(1) = %q", CursorDown(1))
	}
	if CursorForward(5) != "\x1b[5C" {
		t.Errorf("CursorForward(5) = %q", CursorForward(5))
	}
	if CursorBack(2) != "\x1b[2D" {
		t.Errorf("CursorBack(2) = %q", CursorBack(2))
	}
}

func TestCursorPosition(t *testing.T) {
	got := CursorPosition(5, 10)
	if got != "\x1b[5;10H" {
		t.Errorf("CursorPosition(5,10) = %q", got)
	}
}

func TestCursorVisibility(t *testing.T) {
	if !strings.Contains(CursorShow, "?25h") {
		t.Error("CursorShow should contain ?25h")
	}
	if !strings.Contains(CursorHide, "?25l") {
		t.Error("CursorHide should contain ?25l")
	}
}

func TestScreenClear(t *testing.T) {
	if ClearScreen != "\x1b[2J" {
		t.Errorf("ClearScreen = %q", ClearScreen)
	}
	if ClearLine != "\x1b[2K" {
		t.Errorf("ClearLine = %q", ClearLine)
	}
}

func TestAlternateScreen(t *testing.T) {
	if !strings.Contains(EnableAlternateScreen, "1049h") {
		t.Error("should contain 1049h")
	}
	if !strings.Contains(DisableAlternateScreen, "1049l") {
		t.Error("should contain 1049l")
	}
}

func TestOSC(t *testing.T) {
	got := OSC(0, "my title")
	if !strings.HasPrefix(got, "\x1b]0;") {
		t.Errorf("OSC(0,title) should start with ESC ]0;: %q", got)
	}
	if !strings.HasSuffix(got, "\x07") {
		t.Error("OSC should end with BEL")
	}
}

func TestSetTitle(t *testing.T) {
	got := SetTitle("hello")
	if !strings.Contains(got, "hello") {
		t.Error("should contain title")
	}
	if !strings.HasPrefix(got, "\x1b]0;") {
		t.Error("should start with OSC 0")
	}
}

func TestHyperlink(t *testing.T) {
	got := Hyperlink("https://example.com", "click here")
	if !strings.Contains(got, "https://example.com") {
		t.Error("should contain URL")
	}
	if !strings.Contains(got, "click here") {
		t.Error("should contain text")
	}
	if !strings.Contains(got, "8;;") {
		t.Error("should use OSC 8")
	}
}

func TestSGR(t *testing.T) {
	got := SGR(SGRBold)
	if got != "\x1b[1m" {
		t.Errorf("SGR(bold) = %q", got)
	}

	got = SGR(SGRFgRed, SGRBold)
	if got != "\x1b[31;1m" {
		t.Errorf("SGR(red,bold) = %q", got)
	}
}

func TestSGRReset(t *testing.T) {
	if ResetSGR != "\x1b[0m" {
		t.Errorf("ResetSGR = %q", ResetSGR)
	}
}

func TestFG256(t *testing.T) {
	got := FG256(196) // bright red
	if got != "\x1b[38;5;196m" {
		t.Errorf("FG256(196) = %q", got)
	}
}

func TestBG256(t *testing.T) {
	got := BG256(21) // blue
	if got != "\x1b[48;5;21m" {
		t.Errorf("BG256(21) = %q", got)
	}
}

func TestFGRGB(t *testing.T) {
	got := FGRGB(255, 128, 0)
	if got != "\x1b[38;2;255;128;0m" {
		t.Errorf("FGRGB(255,128,0) = %q", got)
	}
}

func TestBGRGB(t *testing.T) {
	got := BGRGB(0, 0, 255)
	if got != "\x1b[48;2;0;0;255m" {
		t.Errorf("BGRGB(0,0,255) = %q", got)
	}
}

func TestDecPrivateMode(t *testing.T) {
	got := SetDecPrivateMode(1049)
	if got != "\x1b[?1049h" {
		t.Errorf("SetDecPrivateMode(1049) = %q", got)
	}
	got = ResetDecPrivateMode(1049)
	if got != "\x1b[?1049l" {
		t.Errorf("ResetDecPrivateMode(1049) = %q", got)
	}
}

func TestWrapForTmux(t *testing.T) {
	seq := "\x1b]9;hello\x07"
	got := WrapForTmux(seq)
	if !strings.HasPrefix(got, "\x1bPtmux;") {
		t.Error("should start with DCS tmux")
	}
	if !strings.HasSuffix(got, "\x1b\\") {
		t.Error("should end with ST")
	}
	// ESC inside should be double-escaped
	if !strings.Contains(got, "\x1b\x1b]") {
		t.Error("ESC should be double-escaped")
	}
}

func TestScrollUpDown(t *testing.T) {
	if ScrollUp(3) != "\x1b[3S" {
		t.Errorf("ScrollUp(3) = %q", ScrollUp(3))
	}
	if ScrollDown(2) != "\x1b[2T" {
		t.Errorf("ScrollDown(2) = %q", ScrollDown(2))
	}
}

func TestMouseTracking(t *testing.T) {
	if !strings.Contains(EnableMouseTracking, "1000h") {
		t.Error("should contain 1000h")
	}
	if !strings.Contains(DisableMouseTracking, "1000l") {
		t.Error("should contain 1000l")
	}
}

func TestFocusReporting(t *testing.T) {
	if !strings.Contains(EnableFocusReporting, "1004h") {
		t.Error("should contain 1004h")
	}
	if !strings.Contains(DisableFocusReporting, "1004l") {
		t.Error("should contain 1004l")
	}
}
