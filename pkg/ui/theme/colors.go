package theme

// colors.go — ColorScheme defines the complete set of semantic color roles
// that a theme must provide. Components reference these roles rather than
// hard-coded palette values, so themes can be swapped at runtime.

// ColorScheme holds every color role that components may reference.
// All values are lipgloss-compatible color strings (hex "#rrggbb" or
// ANSI "123").
type ColorScheme struct {
	// --- Surfaces -----------------------------------------------------------

	// Background is the root terminal background.
	Background string
	// Surface is the default panel / card background.
	Surface string
	// SurfaceElevated is a raised surface (modals, dropdowns, floating panels).
	SurfaceElevated string
	// SurfaceOverlay is for overlays that sit on top of everything (dialogs).
	SurfaceOverlay string

	// --- Text ---------------------------------------------------------------

	// TextPrimary is the main body text color.
	TextPrimary string
	// TextSecondary is de-emphasized text (descriptions, timestamps).
	TextSecondary string
	// TextMuted is very low-contrast text (placeholders, disabled hints).
	TextMuted string
	// TextInverse is text on a colored/accent background.
	TextInverse string

	// --- Borders & Dividers -------------------------------------------------

	// Border is the default border color.
	Border string
	// BorderFocused is the border color for focused/active elements.
	BorderFocused string
	// BorderSubtle is for subtle internal dividers.
	BorderSubtle string

	// --- Primary action (blue) ----------------------------------------------

	// Primary is the main action color (buttons, links).
	Primary string
	// PrimaryHover is the hover state of the primary color.
	PrimaryHover string
	// PrimaryMuted is a low-contrast version for backgrounds/badges.
	PrimaryMuted string

	// --- Accent (cyan) — selected, active, highlight ------------------------

	// Accent is the bright accent for selected/active elements.
	Accent string
	// AccentMuted is a low-contrast accent for subtle highlights.
	AccentMuted string

	// --- Semantic status colors ---------------------------------------------

	// Success is for positive outcomes (pass, created, completed).
	Success string
	// SuccessMuted is a dim success for backgrounds.
	SuccessMuted string

	// Warning is for caution states (pending, approaching limit).
	Warning string
	// WarningMuted is a dim warning for backgrounds.
	WarningMuted string

	// Error is for failures, destructive actions, critical.
	Error string
	// ErrorMuted is a dim error for backgrounds.
	ErrorMuted string

	// Info is for informational messages, hints, links.
	Info string
	// InfoMuted is a dim info for backgrounds.
	InfoMuted string

	// --- Diff colors --------------------------------------------------------

	// DiffAdded is for added lines in diffs.
	DiffAdded string
	// DiffRemoved is for removed lines in diffs.
	DiffRemoved string
	// DiffContext is for unchanged context lines.
	DiffContext string

	// --- Spinner / progress -------------------------------------------------

	// Spinner is the spinner animation color.
	Spinner string

	// --- Selection / cursor -------------------------------------------------

	// Cursor is the text cursor / caret color.
	Cursor string
	// Selection is the background of selected text.
	Selection string

	// --- Component-specific roles -------------------------------------------

	// ToolName is the color for tool names in streaming output.
	ToolName string
	// ToolBorder is the border color for tool call boxes.
	ToolBorder string
	// Prompt is the color of the input prompt character (">").
	Prompt string
	// StatusBarBg is the status bar background.
	StatusBarBg string
	// StatusBarFg is the status bar foreground text.
	StatusBarFg string
	// TabActive is the active tab indicator color.
	TabActive string
	// TabInactive is the inactive tab color.
	TabInactive string
}

// FocusColors holds the color set for a component in various focus states.
type FocusColors struct {
	// Normal is the color when the component has no focus.
	Normal string
	// Focused is the color when the component is focused.
	Focused string
	// Active is the color when the component is being interacted with.
	Active string
	// Disabled is the color when the component is disabled.
	Disabled string
}
