package plugins

import (
	"path/filepath"
	"strings"
	"sync"
)

// LspPluginEntry describes an LSP plugin available in a marketplace.
// Source: src/utils/plugins/lspRecommendation.ts — LspPluginRecommendation
type LspPluginEntry struct {
	PluginID    string   // e.g. "gopls-lsp@official"
	PluginName  string   // human-readable
	Marketplace string   // marketplace name
	Description string   // plugin description
	IsOfficial  bool     // from an official Anthropic marketplace
	Extensions  []string // file extensions (lowercase, with dot, e.g. ".go")
	Command     string   // LSP binary name (e.g. "gopls")
}

// LspRecommendation is the resolved recommendation shown to the user.
// Source: src/hooks/useLspPluginRecommendation.tsx — LspRecommendationState
type LspRecommendation struct {
	PluginID          string
	PluginName        string
	PluginDescription string
	FileExtension     string // extension that triggered the match
}

// MaxIgnoredCount is the threshold after which LSP recommendations are
// auto-disabled. Source: lspRecommendation.ts — MAX_IGNORED_COUNT
const MaxIgnoredCount = 5

// LspRecommender checks edited files against a registry of LSP plugins
// and produces at most one recommendation per session.
//
// Source: src/hooks/useLspPluginRecommendation.tsx + src/utils/plugins/lspRecommendation.ts
type LspRecommender struct {
	mu sync.Mutex

	// registry is the set of known LSP plugins (populated from marketplaces).
	registry []LspPluginEntry

	// installedPlugins tracks plugin IDs that are already installed.
	installedPlugins map[string]struct{}

	// neverSuggest is the set of plugin IDs the user asked to never see.
	neverSuggest map[string]struct{}

	// ignoredCount tracks how many times the user dismissed a recommendation.
	ignoredCount int

	// disabled is true when the user explicitly turned off recommendations.
	disabled bool

	// checkedFiles tracks files we already evaluated (no re-check).
	checkedFiles map[string]struct{}

	// shownThisSession is true once we've emitted one recommendation.
	shownThisSession bool

	// isBinaryInstalled is an injectable check for whether a command exists
	// on PATH. Default: nil (treat all binaries as available). Callers
	// should set this for production use.
	IsBinaryInstalled func(cmd string) bool
}

// NewLspRecommender creates a recommender with the given registry and config.
func NewLspRecommender(registry []LspPluginEntry, opts LspRecommenderOpts) *LspRecommender {
	ns := make(map[string]struct{}, len(opts.NeverSuggest))
	for _, id := range opts.NeverSuggest {
		ns[id] = struct{}{}
	}
	inst := make(map[string]struct{}, len(opts.InstalledPlugins))
	for _, id := range opts.InstalledPlugins {
		inst[id] = struct{}{}
	}
	return &LspRecommender{
		registry:          registry,
		installedPlugins:  inst,
		neverSuggest:      ns,
		ignoredCount:      opts.IgnoredCount,
		disabled:          opts.Disabled,
		checkedFiles:      make(map[string]struct{}),
		IsBinaryInstalled: opts.IsBinaryInstalled,
	}
}

// LspRecommenderOpts configures a new LspRecommender.
type LspRecommenderOpts struct {
	NeverSuggest     []string
	InstalledPlugins []string
	IgnoredCount     int
	Disabled         bool
	IsBinaryInstalled func(cmd string) bool
}

// CheckFile evaluates a file path and returns a recommendation if an LSP
// plugin matches. Returns nil when no recommendation should be shown (already
// checked, disabled, session limit reached, etc.).
//
// Source: useLspPluginRecommendation.tsx effect + getMatchingLspPlugins()
func (r *LspRecommender) CheckFile(filePath string) *LspRecommendation {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Gate: disabled or too many ignores.
	if r.disabled || r.ignoredCount >= MaxIgnoredCount {
		return nil
	}
	// Gate: one recommendation per session.
	if r.shownThisSession {
		return nil
	}
	// Gate: already checked this file.
	if _, seen := r.checkedFiles[filePath]; seen {
		return nil
	}
	r.checkedFiles[filePath] = struct{}{}

	ext := strings.ToLower(filepath.Ext(filePath))
	if ext == "" {
		return nil
	}

	// Find matching plugins, sorted official-first.
	type candidate struct {
		entry LspPluginEntry
	}
	var officials, others []candidate

	for _, entry := range r.registry {
		// Must match extension.
		if !extensionMatches(entry.Extensions, ext) {
			continue
		}
		// Skip if in never-suggest list.
		if _, skip := r.neverSuggest[entry.PluginID]; skip {
			continue
		}
		// Skip if already installed.
		if _, inst := r.installedPlugins[entry.PluginID]; inst {
			continue
		}
		// Binary check.
		if r.IsBinaryInstalled != nil && !r.IsBinaryInstalled(entry.Command) {
			continue
		}
		c := candidate{entry: entry}
		if entry.IsOfficial {
			officials = append(officials, c)
		} else {
			others = append(others, c)
		}
	}

	candidates := append(officials, others...)
	if len(candidates) == 0 {
		return nil
	}

	best := candidates[0].entry
	r.shownThisSession = true
	return &LspRecommendation{
		PluginID:          best.PluginID,
		PluginName:        best.PluginName,
		PluginDescription: best.Description,
		FileExtension:     ext,
	}
}

// AddToNeverSuggest records that the user never wants to see this plugin.
func (r *LspRecommender) AddToNeverSuggest(pluginID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.neverSuggest[pluginID] = struct{}{}
}

// IncrementIgnored bumps the ignored count. After MaxIgnoredCount ignores
// the recommender auto-disables.
func (r *LspRecommender) IncrementIgnored() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.ignoredCount++
}

// SetDisabled explicitly enables or disables recommendations.
func (r *LspRecommender) SetDisabled(v bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.disabled = v
}

// IsDisabled reports whether recommendations are disabled (explicit or by
// ignore count).
func (r *LspRecommender) IsDisabled() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.disabled || r.ignoredCount >= MaxIgnoredCount
}

// extensionMatches checks if ext (e.g. ".go") is in the list.
func extensionMatches(extensions []string, ext string) bool {
	for _, e := range extensions {
		if strings.EqualFold(e, ext) {
			return true
		}
	}
	return false
}
