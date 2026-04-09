package layout

// Source: ink/layout/geometry.ts, ink/layout/node.ts
//
// In TS, Ink uses Yoga (flexbox) for layout. In Go/bubbletea, layout is
// done via lipgloss Join/Place and explicit size calculations.
// This file provides the geometry types and layout helpers that components
// use for positioning and sizing.

import "strings"

// Point is a 2D coordinate.
type Point struct {
	X, Y int
}

// Size is a 2D dimension.
type Size struct {
	Width, Height int
}

// Rect is a positioned rectangle.
type Rect struct {
	X, Y          int
	Width, Height int
}

// Edges represents spacing on all four sides (padding, margin, border).
type Edges struct {
	Top, Right, Bottom, Left int
}

// UniformEdges creates edges with the same value on all sides.
func UniformEdges(all int) Edges {
	return Edges{Top: all, Right: all, Bottom: all, Left: all}
}

// SymmetricEdges creates edges with vertical and horizontal values.
func SymmetricEdges(vertical, horizontal int) Edges {
	return Edges{Top: vertical, Right: horizontal, Bottom: vertical, Left: horizontal}
}

// ZeroEdges is the zero-value edges constant.
var ZeroEdges = Edges{}

// Horizontal returns the total horizontal spacing (Left + Right).
func (e Edges) Horizontal() int { return e.Left + e.Right }

// Vertical returns the total vertical spacing (Top + Bottom).
func (e Edges) Vertical() int { return e.Top + e.Bottom }

// Add combines two edge values.
func (e Edges) Add(other Edges) Edges {
	return Edges{
		Top:    e.Top + other.Top,
		Right:  e.Right + other.Right,
		Bottom: e.Bottom + other.Bottom,
		Left:   e.Left + other.Left,
	}
}

// InnerSize subtracts edges from an outer size.
func (e Edges) InnerSize(outer Size) Size {
	w := outer.Width - e.Horizontal()
	h := outer.Height - e.Vertical()
	if w < 0 {
		w = 0
	}
	if h < 0 {
		h = 0
	}
	return Size{Width: w, Height: h}
}

// ContainsPoint checks if a point is within the rectangle.
func (r Rect) ContainsPoint(p Point) bool {
	return p.X >= r.X && p.X < r.X+r.Width &&
		p.Y >= r.Y && p.Y < r.Y+r.Height
}

// Union returns the smallest rectangle containing both rects.
func (r Rect) Union(other Rect) Rect {
	minX := min(r.X, other.X)
	minY := min(r.Y, other.Y)
	maxX := max(r.X+r.Width, other.X+other.Width)
	maxY := max(r.Y+r.Height, other.Y+other.Height)
	return Rect{X: minX, Y: minY, Width: maxX - minX, Height: maxY - minY}
}

// Clamp restricts a rect to fit within a size.
func (r Rect) Clamp(bounds Size) Rect {
	x := max(0, r.X)
	y := max(0, r.Y)
	endX := min(bounds.Width, r.X+r.Width)
	endY := min(bounds.Height, r.Y+r.Height)
	w := max(0, endX-x)
	h := max(0, endY-y)
	return Rect{X: x, Y: y, Width: w, Height: h}
}

// Inset shrinks a rect by the given edges.
func (r Rect) Inset(e Edges) Rect {
	return Rect{
		X:      r.X + e.Left,
		Y:      r.Y + e.Top,
		Width:  max(0, r.Width-e.Horizontal()),
		Height: max(0, r.Height-e.Vertical()),
	}
}

// WithinBounds checks if a point is within a size (0,0 → width,height).
func WithinBounds(s Size, p Point) bool {
	return p.X >= 0 && p.Y >= 0 && p.X < s.Width && p.Y < s.Height
}

// Clamp constrains a value between min and max.
func ClampInt(value, lo, hi int) int {
	if value < lo {
		return lo
	}
	if value > hi {
		return hi
	}
	return value
}

// ---------------------------------------------------------------------------
// Text layout helpers — simple positioning without Yoga
// ---------------------------------------------------------------------------

// FlexDirection is the layout direction.
type FlexDirection int

const (
	FlexColumn FlexDirection = iota
	FlexRow
)

// SplitHeight divides available height between a fixed and flex region.
// Returns (fixedHeight, flexHeight).
func SplitHeight(totalHeight, fixedHeight int) (int, int) {
	if fixedHeight > totalHeight {
		return totalHeight, 0
	}
	return fixedHeight, totalHeight - fixedHeight
}

// SplitWidth divides available width between a fixed and flex region.
func SplitWidth(totalWidth, fixedWidth int) (int, int) {
	if fixedWidth > totalWidth {
		return totalWidth, 0
	}
	return fixedWidth, totalWidth - fixedWidth
}

// PadString adds left/right padding to each line of a multi-line string.
func PadString(s string, padding Edges) string {
	if s == "" {
		return s
	}
	lines := strings.Split(s, "\n")
	leftPad := strings.Repeat(" ", padding.Left)
	rightPad := strings.Repeat(" ", padding.Right)

	for i, line := range lines {
		lines[i] = leftPad + line + rightPad
	}

	// Add top padding
	topLines := make([]string, padding.Top)
	for i := range topLines {
		topLines[i] = ""
	}
	// Add bottom padding
	bottomLines := make([]string, padding.Bottom)
	for i := range bottomLines {
		bottomLines[i] = ""
	}

	result := make([]string, 0, len(topLines)+len(lines)+len(bottomLines))
	result = append(result, topLines...)
	result = append(result, lines...)
	result = append(result, bottomLines...)
	return strings.Join(result, "\n")
}

// TruncateHeight limits a multi-line string to maxHeight lines.
func TruncateHeight(s string, maxHeight int) string {
	if maxHeight <= 0 {
		return ""
	}
	lines := strings.Split(s, "\n")
	if len(lines) <= maxHeight {
		return s
	}
	return strings.Join(lines[:maxHeight], "\n")
}

// MeasureString returns the width and height of a multi-line string.
// Width is the length of the longest line.
func MeasureString(s string) Size {
	if s == "" {
		return Size{}
	}
	lines := strings.Split(s, "\n")
	maxWidth := 0
	for _, line := range lines {
		if len(line) > maxWidth {
			maxWidth = len(line)
		}
	}
	return Size{Width: maxWidth, Height: len(lines)}
}
