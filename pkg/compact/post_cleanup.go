package compact

// Source: services/compact/postCompactCleanup.ts

// QuerySource identifies the origin of a compaction request.
// Used to discriminate main-thread from subagent compacts.
// Source: constants/querySource.ts
type QuerySource string

const (
	// QuerySourceSDK is the SDK query source.
	QuerySourceSDK QuerySource = "sdk"
)

// CleanupFunc is a function called during post-compact cleanup.
// Each registered cleanup function is responsible for clearing one cache
// or resetting one piece of tracking state.
type CleanupFunc func()

// PostCompactCleaner orchestrates cache/tracking resets after compaction.
// Clears all registered cleanup functions, respecting the main-thread vs
// subagent boundary to avoid corrupting main-thread state from subagent
// compacts.
//
// Intentionally does NOT clear:
//   - invokedSkillContent: must survive across compactions so that
//     createSkillAttachmentIfNeeded() can include the full skill text.
//   - sentSkillNames: re-injecting the full skill_listing (~4K tokens)
//     post-compact is pure cache_creation waste.
//
// Source: postCompactCleanup.ts:31-77
type PostCompactCleaner struct {
	// always are cleanup functions that run for ALL compacts (main + subagent).
	always []CleanupFunc
	// mainOnly are cleanup functions that run ONLY for main-thread compacts.
	// Subagents share the same process and module-level state; resetting
	// these from a subagent would corrupt the main thread.
	mainOnly []CleanupFunc
}

// NewPostCompactCleaner creates a cleaner with no registered functions.
func NewPostCompactCleaner() *PostCompactCleaner {
	return &PostCompactCleaner{}
}

// RegisterAlways adds a cleanup function that runs for all compacts.
func (c *PostCompactCleaner) RegisterAlways(fn CleanupFunc) {
	c.always = append(c.always, fn)
}

// RegisterMainOnly adds a cleanup function that runs only for main-thread compacts.
func (c *PostCompactCleaner) RegisterMainOnly(fn CleanupFunc) {
	c.mainOnly = append(c.mainOnly, fn)
}

// Run executes all registered cleanup functions.
// querySource determines whether main-thread-only cleanups run.
// Source: postCompactCleanup.ts:31-77
func (c *PostCompactCleaner) Run(querySource QuerySource) {
	isMainThread := IsMainThreadCompact(querySource)

	for _, fn := range c.always {
		fn()
	}

	if isMainThread {
		for _, fn := range c.mainOnly {
			fn()
		}
	}
}

// IsMainThreadCompact returns true if the query source indicates a
// main-thread compact (not a subagent).
// Source: postCompactCleanup.ts:36-39
func IsMainThreadCompact(qs QuerySource) bool {
	if qs == "" {
		return true // undefined / empty = main thread (for /compact, /clear)
	}
	if qs == QuerySourceSDK {
		return true
	}
	// startsWith("repl_main_thread")
	return len(qs) >= 16 && string(qs)[:16] == "repl_main_thread"
}
