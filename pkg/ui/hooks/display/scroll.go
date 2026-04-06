package display

// Virtual scrolling in the Go TUI is handled by bubbles/v2/viewport.
//
// The TS codebase has a 300+ LOC useVirtualScroll hook that manages item
// measurement, overscan, slide-stepping, and scroll quantization — all
// necessary because Ink's layout engine has no built-in scrolling primitive.
//
// In Go, bubbles/v2/viewport provides:
//   - Efficient content windowing with SetContent / viewport.Model
//   - Keyboard navigation (PgUp, PgDn, Home, End, mouse wheel)
//   - Configurable dimensions via SetWidth/SetHeight
//   - YOffset-based scrolling with GotoTop / GotoBottom
//
// Usage:
//
//	import "charm.land/bubbles/v2/viewport"
//
//	vp := viewport.New()
//	vp.SetWidth(80)
//	vp.SetHeight(24)
//	vp.SetContent(renderedMarkdown)
//
// For message-list virtual scrolling (lazy rendering of only visible
// messages), compose viewport with a slice window:
//
//	visible := messages[startIdx:endIdx]
//	vp.SetContent(renderMessages(visible))
//
// There is intentionally no wrapper struct here. Adding indirection over
// viewport.Model would violate the "compose, don't rebuild" principle from
// the implementation guide.
