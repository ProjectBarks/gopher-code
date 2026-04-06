package analytics

import (
	"os"
	"strings"
)

const (
	growthBookKeyAntDev  = "sdk-yZQvlplybuXjYh6L"
	growthBookKeyAntProd = "sdk-xRVcrliHIlrg4og4"
	growthBookKeyExt     = "sdk-zAZezfDKGoZuXXKe"
)

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
