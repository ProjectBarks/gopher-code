package prompt

import "sync"

// Section is a lazy-evaluated text block that contributes to the system prompt.
// Cached sections compute once and reuse until ClearSystemPromptSections.
// Uncached (cache-breaking) sections recompute every resolve call.
// Source: constants/systemPromptSections.ts
type Section struct {
	Name       string
	compute    func() *string
	cacheBreak bool
}

// SystemPromptSection creates a memoized section. Computed once, cached until
// /clear or /compact calls ClearSystemPromptSections.
func SystemPromptSection(name string, compute func() *string) Section {
	return Section{Name: name, compute: compute, cacheBreak: false}
}

// UncachedSystemPromptSection creates a volatile section that recomputes every
// turn. This WILL break the prompt cache when the value changes. The reason
// parameter documents why cache-breaking is necessary (not stored at runtime).
func UncachedSystemPromptSection(name string, compute func() *string, _ string) Section {
	return Section{Name: name, compute: compute, cacheBreak: true}
}

// --- section cache ---

var (
	sectionMu    sync.Mutex
	sectionCache = map[string]cachedEntry{}
)

// cachedEntry distinguishes "not cached" from "cached nil".
type cachedEntry struct {
	value *string
	ok    bool
}

// ResolveSystemPromptSections resolves all sections, returning one *string per
// section. Cached sections reuse prior results; uncached sections always
// recompute. Results are stored in the cache for subsequent calls.
func ResolveSystemPromptSections(sections []Section) []*string {
	sectionMu.Lock()
	defer sectionMu.Unlock()

	results := make([]*string, len(sections))
	for i, s := range sections {
		if !s.cacheBreak {
			if entry, ok := sectionCache[s.Name]; ok && entry.ok {
				results[i] = entry.value
				continue
			}
		}
		v := s.compute()
		sectionCache[s.Name] = cachedEntry{value: v, ok: true}
		results[i] = v
	}
	return results
}

// --- beta header latches ---

var betaLatches = map[string]*bool{}

// SetBetaHeaderLatch sets a beta header latch by key.
func SetBetaHeaderLatch(key string, val *bool) {
	sectionMu.Lock()
	defer sectionMu.Unlock()
	betaLatches[key] = val
}

// GetBetaHeaderLatch returns a beta header latch value, or nil if unset.
func GetBetaHeaderLatch(key string) *bool {
	sectionMu.Lock()
	defer sectionMu.Unlock()
	return betaLatches[key]
}

// clearBetaHeaderLatches resets all beta header latches so a fresh conversation
// gets fresh evaluation of AFK/fast-mode/cache-editing headers.
func clearBetaHeaderLatches() {
	// Clear all entries (the TS sets each to null; we delete all keys).
	for k := range betaLatches {
		delete(betaLatches, k)
	}
}

// ClearSystemPromptSections clears the section cache and resets beta header
// latches. Called on /clear and /compact.
func ClearSystemPromptSections() {
	sectionMu.Lock()
	defer sectionMu.Unlock()
	sectionCache = map[string]cachedEntry{}
	clearBetaHeaderLatches()
}
