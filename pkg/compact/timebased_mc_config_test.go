package compact

import "testing"

// Source: services/compact/timeBasedMCConfig.ts

func TestDefaultTimeBasedMCConfig(t *testing.T) {
	// Source: timeBasedMCConfig.ts:30-34
	cfg := DefaultTimeBasedMCConfig
	if cfg.Enabled {
		t.Error("default should be disabled")
	}
	if cfg.GapThresholdMinutes != 60 {
		t.Errorf("GapThresholdMinutes = %d, want 60", cfg.GapThresholdMinutes)
	}
	if cfg.KeepRecent != 5 {
		t.Errorf("KeepRecent = %d, want 5", cfg.KeepRecent)
	}
}

func TestGetDefaultTimeBasedMCConfig(t *testing.T) {
	cfg := GetDefaultTimeBasedMCConfig()
	if cfg != DefaultTimeBasedMCConfig {
		t.Errorf("GetDefaultTimeBasedMCConfig() = %+v, want %+v", cfg, DefaultTimeBasedMCConfig)
	}
}

func TestTimeBasedMCConfig_EnabledOverride(t *testing.T) {
	cfg := TimeBasedMCConfig{
		Enabled:             true,
		GapThresholdMinutes: 30,
		KeepRecent:          3,
	}
	if !cfg.Enabled {
		t.Error("expected enabled")
	}
	if cfg.GapThresholdMinutes != 30 {
		t.Errorf("GapThresholdMinutes = %d, want 30", cfg.GapThresholdMinutes)
	}
	if cfg.KeepRecent != 3 {
		t.Errorf("KeepRecent = %d, want 3", cfg.KeepRecent)
	}
}

func TestTimeBasedMCConfigProvider_Type(t *testing.T) {
	// Verify the provider type works as expected.
	var provider TimeBasedMCConfigProvider = func() TimeBasedMCConfig {
		return TimeBasedMCConfig{Enabled: true, GapThresholdMinutes: 15, KeepRecent: 2}
	}
	cfg := provider()
	if !cfg.Enabled {
		t.Error("provider should return enabled config")
	}
	if cfg.GapThresholdMinutes != 15 {
		t.Errorf("GapThresholdMinutes = %d, want 15", cfg.GapThresholdMinutes)
	}
}
