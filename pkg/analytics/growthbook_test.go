package analytics

import (
	"testing"
)

func TestIsScratchpadEnabled(t *testing.T) {
	t.Run("returns false when no gate checker is set", func(t *testing.T) {
		featureGateMu.Lock()
		old := featureGateChecker
		featureGateChecker = nil
		featureGateMu.Unlock()
		defer func() {
			featureGateMu.Lock()
			featureGateChecker = old
			featureGateMu.Unlock()
		}()

		if IsScratchpadEnabled() {
			t.Error("IsScratchpadEnabled() = true, want false when no checker set")
		}
	})

	t.Run("returns true when gate checker enables tengu_scratch", func(t *testing.T) {
		SetFeatureGateChecker(func(gate string) bool {
			return gate == ScratchpadGateName
		})
		defer SetFeatureGateChecker(nil)

		if !IsScratchpadEnabled() {
			t.Error("IsScratchpadEnabled() = false, want true")
		}
	})

	t.Run("returns false when gate checker denies tengu_scratch", func(t *testing.T) {
		SetFeatureGateChecker(func(gate string) bool {
			return false
		})
		defer SetFeatureGateChecker(nil)

		if IsScratchpadEnabled() {
			t.Error("IsScratchpadEnabled() = true, want false")
		}
	})

	t.Run("constant matches expected gate name", func(t *testing.T) {
		if ScratchpadGateName != "tengu_scratch" {
			t.Errorf("ScratchpadGateName = %q, want %q", ScratchpadGateName, "tengu_scratch")
		}
	})
}

func TestGetGrowthBookClientKey(t *testing.T) {
	tests := []struct {
		name              string
		userType          string
		enableGBDev       string
		wantKey           string
	}{
		{
			name:        "ant user with dev enabled (1)",
			userType:    "ant",
			enableGBDev: "1",
			wantKey:     "sdk-yZQvlplybuXjYh6L",
		},
		{
			name:        "ant user with dev enabled (true)",
			userType:    "ant",
			enableGBDev: "true",
			wantKey:     "sdk-yZQvlplybuXjYh6L",
		},
		{
			name:        "ant user with dev enabled (yes)",
			userType:    "ant",
			enableGBDev: "yes",
			wantKey:     "sdk-yZQvlplybuXjYh6L",
		},
		{
			name:        "ant user with dev disabled",
			userType:    "ant",
			enableGBDev: "",
			wantKey:     "sdk-xRVcrliHIlrg4og4",
		},
		{
			name:        "ant user with dev explicitly false",
			userType:    "ant",
			enableGBDev: "false",
			wantKey:     "sdk-xRVcrliHIlrg4og4",
		},
		{
			name:        "external user",
			userType:    "",
			enableGBDev: "",
			wantKey:     "sdk-zAZezfDKGoZuXXKe",
		},
		{
			name:        "external user ignores dev flag",
			userType:    "external",
			enableGBDev: "1",
			wantKey:     "sdk-zAZezfDKGoZuXXKe",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("USER_TYPE", tt.userType)
			t.Setenv("ENABLE_GROWTHBOOK_DEV", tt.enableGBDev)

			got := GetGrowthBookClientKey()
			if got != tt.wantKey {
				t.Errorf("GetGrowthBookClientKey() = %q, want %q", got, tt.wantKey)
			}
		})
	}
}
