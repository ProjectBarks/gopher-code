package components

import (
	"fmt"
	"math/rand"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/projectbarks/gopher-code/pkg/ui/theme"
)

// Spinner glyph animation frames (from Claude Code's src/components/Spinner/utils.ts).
// These cycle to create a spinning star animation.
var SpinnerGlyphs = []string{"·", "✢", "✳", "✶", "✻", "✽"}

// SpinnerVerbs are random action words shown during thinking.
// Complete list from Claude Code's src/constants/spinnerVerbs.ts (188 verbs).
var SpinnerVerbs = []string{
	"Accomplishing", "Actioning", "Actualizing", "Architecting",
	"Baking", "Beaming", "Beboppin'", "Befuddling", "Billowing", "Blanching",
	"Bloviating", "Boogieing", "Boondoggling", "Booping", "Bootstrapping",
	"Brewing", "Bunning", "Burrowing",
	"Calculating", "Canoodling", "Caramelizing", "Cascading", "Catapulting",
	"Cerebrating", "Channeling", "Channelling", "Choreographing", "Churning",
	"Clauding", "Coalescing", "Cogitating", "Combobulating", "Composing",
	"Computing", "Concocting", "Considering", "Contemplating", "Cooking",
	"Crafting", "Creating", "Crunching", "Crystallizing", "Cultivating",
	"Deciphering", "Deliberating", "Determining", "Dilly-dallying",
	"Discombobulating", "Doing", "Doodling", "Drizzling",
	"Ebbing", "Effecting", "Elucidating", "Embellishing", "Enchanting",
	"Envisioning", "Evaporating",
	"Fermenting", "Fiddle-faddling", "Finagling", "Flambéing",
	"Flibbertigibbeting", "Flowing", "Flummoxing", "Fluttering", "Forging",
	"Forming", "Frolicking", "Frosting",
	"Gallivanting", "Galloping", "Garnishing", "Generating", "Gesticulating",
	"Germinating", "Gitifying", "Grooving", "Gusting",
	"Harmonizing", "Hashing", "Hatching", "Herding", "Honking",
	"Hullaballooing", "Hyperspacing",
	"Ideating", "Imagining", "Improvising", "Incubating", "Inferring",
	"Infusing", "Ionizing",
	"Jitterbugging", "Julienning",
	"Kneading",
	"Leavening", "Levitating", "Lollygagging",
	"Manifesting", "Marinating", "Meandering", "Metamorphosing", "Misting",
	"Moonwalking", "Moseying", "Mulling", "Mustering", "Musing",
	"Nebulizing", "Nesting", "Newspapering", "Noodling", "Nucleating",
	"Orbiting", "Orchestrating", "Osmosing",
	"Perambulating", "Percolating", "Perusing", "Philosophising",
	"Photosynthesizing", "Pollinating", "Pondering", "Pontificating",
	"Pouncing", "Precipitating", "Prestidigitating", "Processing",
	"Proofing", "Propagating", "Puttering", "Puzzling",
	"Quantumizing",
	"Razzle-dazzling", "Razzmatazzing", "Recombobulating", "Reticulating",
	"Roosting", "Ruminating",
	"Sautéing", "Scampering", "Schlepping", "Scurrying", "Seasoning",
	"Shenaniganing", "Shimmying", "Simmering", "Skedaddling", "Sketching",
	"Slithering", "Smooshing", "Sock-hopping", "Spelunking", "Spinning",
	"Sprouting", "Stewing", "Sublimating", "Swirling", "Swooping",
	"Symbioting", "Synthesizing",
	"Tempering", "Thinking", "Thundering", "Tinkering", "Tomfoolering",
	"Topsy-turvying", "Transfiguring", "Transmuting", "Twisting",
	"Undulating", "Unfurling", "Unravelling",
	"Vibing",
	"Waddling", "Wandering", "Warping", "Whatchamacalliting", "Whirlpooling",
	"Whirring", "Whisking", "Wibbling", "Working", "Wrangling",
	"Zesting", "Zigzagging",
}

// TurnCompletionVerbs are past tense verbs for turn completion messages.
// These work naturally with "for [duration]" (e.g., "Worked for 5s").
// Source: constants/turnCompletionVerbs.ts
var TurnCompletionVerbs = []string{
	"Baked", "Brewed", "Churned", "Cogitated",
	"Cooked", "Crunched", "Sautéed", "Worked",
}

// Effort level icons (from Claude Code's src/constants/figures.ts).
const (
	EffortLow    = "○" // U+25CB WHITE CIRCLE
	EffortMedium = "◐" // U+25D0 CIRCLE WITH LEFT HALF BLACK
	EffortHigh   = "●" // U+25CF BLACK CIRCLE
	EffortMax    = "◉" // U+25C9 FISHEYE
)

// Tips shown below the spinner during thinking.
var SpinnerTips = []string{
	"Did you know you can drag and drop image files into your terminal?",
	"Use /help to see all available commands.",
	"Press Tab to cycle focus between input and conversation.",
	"Use Ctrl+K to clear text to end of line.",
	"Use /model to switch between AI models.",
	"Use Up arrow to recall previous commands.",
	"Use /clear to reset the conversation.",
	"Press Ctrl+C to interrupt a running query.",
	"Use /compact to reduce context window usage.",
}

// SpinnerTickMsg triggers a spinner animation frame advance.
type SpinnerTickMsg struct{}

// ThinkingSpinner manages the animated verb display during streaming.
type ThinkingSpinner struct {
	verb      string
	tip       string
	frame     int // Current glyph animation frame
	effort    string
	active    bool
	startTime time.Time
	elapsed   time.Duration
	theme     theme.Theme
	width     int
}

// NewThinkingSpinner creates a spinner with a random verb and tip.
func NewThinkingSpinner(t theme.Theme) *ThinkingSpinner {
	return &ThinkingSpinner{
		verb:   SpinnerVerbs[rand.Intn(len(SpinnerVerbs))],
		tip:    SpinnerTips[rand.Intn(len(SpinnerTips))],
		effort: "",
		theme:  t,
		width:  80,
	}
}

// Start activates the spinner with a new random verb.
func (ts *ThinkingSpinner) Start() {
	ts.active = true
	ts.verb = SpinnerVerbs[rand.Intn(len(SpinnerVerbs))]
	ts.tip = SpinnerTips[rand.Intn(len(SpinnerTips))]
	ts.frame = 0
	ts.startTime = time.Now()
	ts.elapsed = 0
}

// Stop deactivates the spinner and records elapsed time.
func (ts *ThinkingSpinner) Stop() {
	ts.active = false
	ts.elapsed = time.Since(ts.startTime)
}

// SetEffort sets the effort level suffix.
func (ts *ThinkingSpinner) SetEffort(level string) {
	switch strings.ToLower(level) {
	case "low":
		ts.effort = EffortLow
	case "medium":
		ts.effort = EffortMedium
	case "high":
		ts.effort = EffortHigh
	case "max":
		ts.effort = EffortMax
	default:
		ts.effort = ""
	}
}

// IsActive returns whether the spinner is running.
func (ts *ThinkingSpinner) IsActive() bool { return ts.active }

// Tick advances the animation frame.
func (ts *ThinkingSpinner) Tick() tea.Cmd {
	return tea.Tick(50*time.Millisecond, func(time.Time) tea.Msg {
		return SpinnerTickMsg{}
	})
}

// Update handles tick messages.
func (ts *ThinkingSpinner) Update(msg tea.Msg) {
	if _, ok := msg.(SpinnerTickMsg); ok && ts.active {
		ts.frame = (ts.frame + 1) % len(SpinnerGlyphs)
	}
}

// View renders the spinner line.
// Active: "✻ Cogitating… (thinking)"
// Complete: "✻ thought for 3s"
func (ts *ThinkingSpinner) View() string {
	cs := ts.theme.Colors()

	glyphStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(cs.Accent))
	verbStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(cs.TextPrimary))
	suffixStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(cs.TextSecondary))

	glyph := SpinnerGlyphs[ts.frame%len(SpinnerGlyphs)]

	if ts.active {
		suffix := "(thinking)"
		if ts.effort != "" {
			suffix = fmt.Sprintf("(thinking %s)", ts.effort)
		}
		return glyphStyle.Render(glyph) + " " +
			verbStyle.Render(ts.verb+"…") + " " +
			suffixStyle.Render(suffix)
	}

	// Completed state — use random turn completion verb.
	// Source: constants/turnCompletionVerbs.ts
	secs := int(ts.elapsed.Seconds())
	if secs < 1 {
		secs = 1
	}
	completionVerb := TurnCompletionVerbs[rand.Intn(len(TurnCompletionVerbs))]
	return glyphStyle.Render(glyph) + " " +
		suffixStyle.Render(fmt.Sprintf("%s for %ds", completionVerb, secs))
}

// TipView renders the tip line below the spinner.
// Format: "  └ Tip: {text}"
func (ts *ThinkingSpinner) TipView() string {
	cs := ts.theme.Colors()
	connectorStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(cs.TextSecondary))
	tipStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(cs.TextMuted))

	return connectorStyle.Render(ResponseConnector) + tipStyle.Render("Tip: "+ts.tip)
}

// Verb returns the current verb.
func (ts *ThinkingSpinner) Verb() string { return ts.verb }

// Tip returns the current tip.
func (ts *ThinkingSpinner) Tip() string { return ts.tip }

// Frame returns the current animation frame index.
func (ts *ThinkingSpinner) Frame() int { return ts.frame }
