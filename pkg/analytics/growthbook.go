package analytics

import (
	"os"
	"strings"
	"sync"
)

const (
	growthBookKeyAntDev  = "sdk-yZQvlplybuXjYh6L"
	growthBookKeyAntProd = "sdk-xRVcrliHIlrg4og4"
	growthBookKeyExt     = "sdk-zAZezfDKGoZuXXKe"
)

// ScratchpadGateName is the GrowthBook feature gate controlling the
// per-session scratchpad directory. Maps to the TS tengu_scratch gate
// checked by isScratchpadGateEnabled() in coordinatorMode.ts and
// isScratchpadEnabled() in filesystem.ts.
const ScratchpadGateName = "tengu_scratch"

// featureGateMu protects featureGateChecker.
var featureGateMu sync.RWMutex

// featureGateChecker is the package-level feature gate function.
// Set via SetFeatureGateChecker at startup.
var featureGateChecker GateChecker

// SetFeatureGateChecker registers the global feature-gate checker used
// by IsScratchpadEnabled and other gate-gated helpers in this package.
func SetFeatureGateChecker(fn GateChecker) {
	featureGateMu.Lock()
	defer featureGateMu.Unlock()
	featureGateChecker = fn
}

// checkFeatureGate returns whether the named gate is enabled.
// Returns false if no checker has been set (fail-closed).
func checkFeatureGate(gate string) bool {
	featureGateMu.RLock()
	fn := featureGateChecker
	featureGateMu.RUnlock()
	if fn == nil {
		return false
	}
	return fn(gate)
}

// IsScratchpadEnabled reports whether the tengu_scratch feature gate is
// enabled. The scratchpad is a per-session temp directory for Claude to
// write temporary files without explicit user permission.
//
// Source: coordinatorMode.ts — isScratchpadGateEnabled()
// Source: filesystem.ts — isScratchpadEnabled()
func IsScratchpadEnabled() bool {
	return checkFeatureGate(ScratchpadGateName)
}

// GetGrowthBookClientKey returns the GrowthBook SDK client key based on
// USER_TYPE and ENABLE_GROWTHBOOK_DEV environment variables.
// Reads env lazily so that values set after module init are picked up.
func GetGrowthBookClientKey() string {
	if os.Getenv("USER_TYPE") == "ant" {
		if isEnvTruthy(os.Getenv("ENABLE_GROWTHBOOK_DEV")) {
			return growthBookKeyAntDev
		}
		return growthBookKeyAntProd
	}
	return growthBookKeyExt
}

// isEnvTruthy checks if a string looks truthy (1, true, yes).
func isEnvTruthy(val string) bool {
	switch strings.ToLower(val) {
	case "1", "true", "yes":
		return true
	}
	return false
}
