package theme

// palette.go — Global color constants for the Gopher blue design system.
//
// All colors are defined as lipgloss-compatible color strings. For 256-color
// terminals these map to ANSI 256 codes; for true-color terminals the hex
// values are used directly. Lipgloss v2 negotiates the best representation
// automatically via the colorprofile package.
//
// Naming convention: {Hue}{Shade} where shade goes from darkest (900) to
// lightest (50). Semantic constants are provided separately.

// ---------------------------------------------------------------------------
// Primary blues — the core of the Gopher identity
// ---------------------------------------------------------------------------

const (
	// Deep navy backgrounds and panels.
	Blue900 = "#0a1929" // Darkest — main background in dark mode
	Blue800 = "#0d2137" // Panel background, secondary surfaces
	Blue700 = "#132f4c" // Elevated surfaces, cards
	Blue600 = "#1a3a52" // Borders, dividers on dark surfaces
	Blue500 = "#1e4976" // Muted interactive elements
	Blue400 = "#2a6496" // Default interactive elements
	Blue300 = "#3d8bd4" // Hovered interactive elements
	Blue200 = "#5fa8d3" // Active ring, selection highlight
	Blue100 = "#90caf9" // Prominent accents, links
	Blue50  = "#bbdefb" // Lightest blue, subtle highlights
)

// ---------------------------------------------------------------------------
// Cyan / Teal accents — active and selected states
// ---------------------------------------------------------------------------

const (
	Cyan900 = "#004d5a"
	Cyan800 = "#00695c"
	Cyan700 = "#00838f"
	Cyan600 = "#0097a7"
	Cyan500 = "#00acc1"
	Cyan400 = "#00bcd4" // Active/selected in dark mode
	Cyan300 = "#00d7ff" // Bright cyan — primary accent
	Cyan200 = "#4dd0e1"
	Cyan100 = "#80deea"
	Cyan50  = "#b2ebf2"
)

// ---------------------------------------------------------------------------
// Indigo accents — secondary accent, focus rings, special states
// ---------------------------------------------------------------------------

const (
	Indigo700 = "#303f9f"
	Indigo600 = "#3949ab"
	Indigo500 = "#3f51b5"
	Indigo400 = "#5c6bc0"
	Indigo300 = "#7986cb"
	Indigo200 = "#9fa8da"
	Indigo100 = "#c5cae9"
)

// ---------------------------------------------------------------------------
// Neutral grays — text, borders, disabled states
// ---------------------------------------------------------------------------

const (
	Gray950 = "#0a0a0a" // Near black
	Gray900 = "#121212" // Dark mode base (true black alternative)
	Gray850 = "#1a1a1a" // Elevated dark surface
	Gray800 = "#262626" // Card on dark background
	Gray700 = "#3a3a3a" // Subtle borders on dark
	Gray600 = "#525252" // Disabled text on dark
	Gray500 = "#6b6b6b" // Placeholder text
	Gray400 = "#8b8b8b" // Secondary text
	Gray300 = "#a3a3a3" // Muted text
	Gray200 = "#c4c4c4" // Tertiary text
	Gray100 = "#e0e0e0" // Primary text on dark
	Gray50  = "#f5f5f5" // Primary text on very dark / light bg

	White = "#ffffff"
	Black = "#000000"
)

// ---------------------------------------------------------------------------
// Semantic / status colors
// ---------------------------------------------------------------------------

const (
	// Success — confirmations, passing tests, completed tasks
	Green700 = "#2e7d32"
	Green600 = "#388e3c"
	Green500 = "#43a047"
	Green400 = "#00d787" // Primary success in dark mode
	Green300 = "#66bb6a"
	Green200 = "#81c784"
	Green100 = "#a5d6a7"

	// Warning — caution, pending actions, approaching limits
	Yellow700 = "#f57f17"
	Yellow600 = "#f9a825"
	Yellow500 = "#fbc02d"
	Yellow400 = "#ffd700" // Primary warning in dark mode
	Yellow300 = "#fff176"
	Yellow200 = "#fff59d"
	Yellow100 = "#fff9c4"

	// Error — failures, destructive actions, critical alerts
	Red700 = "#c62828"
	Red600 = "#d32f2f"
	Red500 = "#e53935"
	Red400 = "#ff5555" // Primary error in dark mode
	Red300 = "#ef5350"
	Red200 = "#ef9a9a"
	Red100 = "#ffcdd2"

	// Info — informational messages, links, hints
	Info700 = "#1565c0"
	Info600 = "#1976d2"
	Info500 = "#1e88e5"
	Info400 = "#5f87ff" // Primary info in dark mode
	Info300 = "#64b5f6"
	Info200 = "#90caf9"
	Info100 = "#bbdefb"
)

// ---------------------------------------------------------------------------
// Teammate / agent accent colors (maps to session.AgentColorName)
// ---------------------------------------------------------------------------

const (
	AgentRed    = "#ff6b6b"
	AgentBlue   = "#5fa8d3"
	AgentGreen  = "#69db7c"
	AgentYellow = "#ffd43b"
	AgentPurple = "#b197fc"
	AgentOrange = "#ffa94d"
	AgentPink   = "#f783ac"
	AgentCyan   = "#66d9e8"
)

// AgentColorMap maps agent color names to their hex values.
var AgentColorMap = map[string]string{
	"red":    AgentRed,
	"blue":   AgentBlue,
	"green":  AgentGreen,
	"yellow": AgentYellow,
	"purple": AgentPurple,
	"orange": AgentOrange,
	"pink":   AgentPink,
	"cyan":   AgentCyan,
}

// ---------------------------------------------------------------------------
// Accent blue — buttons, links, primary actions
// ---------------------------------------------------------------------------

const (
	AccentBlue    = "#0087ff" // Primary action color
	AccentBlueDim = "#005faf" // Pressed / active state
)
