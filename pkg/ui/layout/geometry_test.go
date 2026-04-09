package layout

import "testing"

func TestPoint(t *testing.T) {
	p := Point{X: 10, Y: 20}
	if p.X != 10 || p.Y != 20 {
		t.Error("wrong")
	}
}

func TestSize(t *testing.T) {
	s := Size{Width: 80, Height: 24}
	if s.Width != 80 || s.Height != 24 {
		t.Error("wrong")
	}
}

func TestUniformEdges(t *testing.T) {
	e := UniformEdges(5)
	if e.Top != 5 || e.Right != 5 || e.Bottom != 5 || e.Left != 5 {
		t.Errorf("got %+v", e)
	}
}

func TestSymmetricEdges(t *testing.T) {
	e := SymmetricEdges(2, 4)
	if e.Top != 2 || e.Bottom != 2 || e.Left != 4 || e.Right != 4 {
		t.Errorf("got %+v", e)
	}
}

func TestEdges_Horizontal(t *testing.T) {
	e := Edges{Left: 3, Right: 5}
	if e.Horizontal() != 8 {
		t.Errorf("horizontal = %d", e.Horizontal())
	}
}

func TestEdges_Vertical(t *testing.T) {
	e := Edges{Top: 2, Bottom: 4}
	if e.Vertical() != 6 {
		t.Errorf("vertical = %d", e.Vertical())
	}
}

func TestEdges_Add(t *testing.T) {
	a := Edges{Top: 1, Right: 2, Bottom: 3, Left: 4}
	b := Edges{Top: 5, Right: 6, Bottom: 7, Left: 8}
	r := a.Add(b)
	if r.Top != 6 || r.Right != 8 || r.Bottom != 10 || r.Left != 12 {
		t.Errorf("got %+v", r)
	}
}

func TestEdges_InnerSize(t *testing.T) {
	e := Edges{Top: 1, Right: 2, Bottom: 1, Left: 2}
	inner := e.InnerSize(Size{Width: 80, Height: 24})
	if inner.Width != 76 || inner.Height != 22 {
		t.Errorf("inner = %+v", inner)
	}
}

func TestEdges_InnerSize_Clamp(t *testing.T) {
	e := UniformEdges(50)
	inner := e.InnerSize(Size{Width: 10, Height: 5})
	if inner.Width != 0 || inner.Height != 0 {
		t.Errorf("should clamp to 0: %+v", inner)
	}
}

func TestRect_ContainsPoint(t *testing.T) {
	r := Rect{X: 10, Y: 5, Width: 20, Height: 10}
	if !r.ContainsPoint(Point{X: 15, Y: 8}) {
		t.Error("should contain point inside")
	}
	if !r.ContainsPoint(Point{X: 10, Y: 5}) {
		t.Error("should contain top-left corner")
	}
	if r.ContainsPoint(Point{X: 30, Y: 15}) {
		t.Error("should not contain point at right-bottom edge")
	}
	if r.ContainsPoint(Point{X: 5, Y: 5}) {
		t.Error("should not contain point outside left")
	}
}

func TestRect_Union(t *testing.T) {
	a := Rect{X: 0, Y: 0, Width: 10, Height: 10}
	b := Rect{X: 5, Y: 5, Width: 10, Height: 10}
	u := a.Union(b)
	if u.X != 0 || u.Y != 0 || u.Width != 15 || u.Height != 15 {
		t.Errorf("union = %+v", u)
	}
}

func TestRect_Clamp(t *testing.T) {
	r := Rect{X: -5, Y: -3, Width: 20, Height: 30}
	c := r.Clamp(Size{Width: 10, Height: 10})
	if c.X != 0 || c.Y != 0 {
		t.Errorf("clamped origin should be 0,0: %+v", c)
	}
	if c.Width > 10 || c.Height > 10 {
		t.Errorf("clamped size should fit: %+v", c)
	}
}

func TestRect_Inset(t *testing.T) {
	r := Rect{X: 0, Y: 0, Width: 80, Height: 24}
	i := r.Inset(Edges{Top: 1, Right: 2, Bottom: 1, Left: 2})
	if i.X != 2 || i.Y != 1 || i.Width != 76 || i.Height != 22 {
		t.Errorf("inset = %+v", i)
	}
}

func TestWithinBounds(t *testing.T) {
	s := Size{Width: 80, Height: 24}
	if !WithinBounds(s, Point{X: 0, Y: 0}) {
		t.Error("origin should be within bounds")
	}
	if !WithinBounds(s, Point{X: 79, Y: 23}) {
		t.Error("max corner should be within bounds")
	}
	if WithinBounds(s, Point{X: 80, Y: 0}) {
		t.Error("at width should be out of bounds")
	}
	if WithinBounds(s, Point{X: -1, Y: 0}) {
		t.Error("negative should be out of bounds")
	}
}

func TestClampInt(t *testing.T) {
	if ClampInt(5, 0, 10) != 5 {
		t.Error("in range should pass through")
	}
	if ClampInt(-1, 0, 10) != 0 {
		t.Error("below min should clamp")
	}
	if ClampInt(15, 0, 10) != 10 {
		t.Error("above max should clamp")
	}
}

func TestSplitHeight(t *testing.T) {
	fixed, flex := SplitHeight(24, 5)
	if fixed != 5 || flex != 19 {
		t.Errorf("got %d, %d", fixed, flex)
	}

	fixed, flex = SplitHeight(3, 10)
	if fixed != 3 || flex != 0 {
		t.Errorf("overflow: got %d, %d", fixed, flex)
	}
}

func TestSplitWidth(t *testing.T) {
	fixed, flex := SplitWidth(80, 20)
	if fixed != 20 || flex != 60 {
		t.Errorf("got %d, %d", fixed, flex)
	}
}

func TestPadString(t *testing.T) {
	s := PadString("hello\nworld", Edges{Left: 2, Right: 1})
	lines := splitLines(s)
	for _, line := range lines {
		if line != "" && line[:2] != "  " {
			t.Errorf("should have 2-space left pad: %q", line)
		}
	}
}

func TestTruncateHeight(t *testing.T) {
	s := "a\nb\nc\nd\ne"
	got := TruncateHeight(s, 3)
	if countLines(got) != 3 {
		t.Errorf("should have 3 lines, got %d", countLines(got))
	}

	// No truncation needed
	if TruncateHeight(s, 10) != s {
		t.Error("should not truncate when within limit")
	}

	if TruncateHeight(s, 0) != "" {
		t.Error("0 height should return empty")
	}
}

func TestMeasureString(t *testing.T) {
	s := MeasureString("hello\nworld!\nhi")
	if s.Width != 6 { // "world!" is longest
		t.Errorf("width = %d, want 6", s.Width)
	}
	if s.Height != 3 {
		t.Errorf("height = %d, want 3", s.Height)
	}
}

func TestMeasureString_Empty(t *testing.T) {
	s := MeasureString("")
	if s.Width != 0 || s.Height != 0 {
		t.Errorf("empty should be 0x0: %+v", s)
	}
}
