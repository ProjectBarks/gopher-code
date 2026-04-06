package analytics

import "sync"

// SinkName identifies an analytics sink that can be independently killed.
type SinkName string

const (
	SinkDatadog    SinkName = "datadog"
	SinkFirstParty SinkName = "firstParty"
)

// sinkKillswitchConfigName is the GrowthBook dynamic config key.
// Mangled name is intentional (ops-only obscurity).
const sinkKillswitchConfigName = "tengu_frond_boric"

// KillswitchProvider is a function that returns the killswitch config.
// In production this queries GrowthBook; tests can inject a stub.
type KillswitchProvider func() map[SinkName]bool

var (
	ksMu       sync.RWMutex
	ksProvider KillswitchProvider
)

// SetKillswitchProvider registers the function used to check sink killswitches.
func SetKillswitchProvider(p KillswitchProvider) {
	ksMu.Lock()
	defer ksMu.Unlock()
	ksProvider = p
}

// IsSinkKilled returns true if the given sink has been remotely disabled.
// Fail-open: if the provider is unset or the config is missing, the sink
// stays active. Must NOT be called from is1PEventLoggingEnabled (recursion).
func IsSinkKilled(name SinkName) bool {
	ksMu.RLock()
	p := ksProvider
	ksMu.RUnlock()

	if p == nil {
		return false
	}
	config := p()
	return config[name]
}
