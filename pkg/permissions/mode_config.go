package permissions

// Source: utils/permissions/PermissionMode.ts

// PauseIcon is the symbol used for plan mode.
const PauseIcon = "⏸"

// ModeColorKey identifies a semantic color for a permission mode.
type ModeColorKey string

const (
	ColorText       ModeColorKey = "text"
	ColorPlanMode   ModeColorKey = "planMode"
	ColorPermission ModeColorKey = "permission"
	ColorAutoAccept ModeColorKey = "autoAccept"
	ColorError      ModeColorKey = "error"
	ColorWarning    ModeColorKey = "warning"
)

// ModeConfig holds display configuration for a permission mode.
// Source: PermissionMode.ts:34-40
type ModeConfig struct {
	Title      string
	ShortTitle string
	Symbol     string
	Color      ModeColorKey
	External   PermissionMode // maps internal mode → external SDK mode
}

// modeConfigs maps each mode to its display configuration.
// Source: PermissionMode.ts:42-91
var modeConfigs = map[PermissionMode]ModeConfig{
	ModeDefault: {
		Title:      "Default",
		ShortTitle: "Default",
		Symbol:     "",
		Color:      ColorText,
		External:   ModeDefault,
	},
	ModePlan: {
		Title:      "Plan Mode",
		ShortTitle: "Plan",
		Symbol:     PauseIcon,
		Color:      ColorPlanMode,
		External:   ModePlan,
	},
	ModeAcceptEdits: {
		Title:      "Accept edits",
		ShortTitle: "Accept",
		Symbol:     "⏵⏵",
		Color:      ColorAutoAccept,
		External:   ModeAcceptEdits,
	},
	ModeBypassPermissions: {
		Title:      "Bypass Permissions",
		ShortTitle: "Bypass",
		Symbol:     "⏵⏵",
		Color:      ColorError,
		External:   ModeBypassPermissions,
	},
	ModeDontAsk: {
		Title:      "Don't Ask",
		ShortTitle: "DontAsk",
		Symbol:     "⏵⏵",
		Color:      ColorError,
		External:   ModeDontAsk,
	},
	ModeAuto: {
		Title:      "Auto mode",
		ShortTitle: "Auto",
		Symbol:     "⏵⏵",
		Color:      ColorWarning,
		External:   ModeDefault, // auto maps to default for external/SDK
	},
}

// getModeConfig returns the config for a mode, falling back to default.
func getModeConfig(mode PermissionMode) ModeConfig {
	if cfg, ok := modeConfigs[mode]; ok {
		return cfg
	}
	return modeConfigs[ModeDefault]
}

// PermissionModeTitle returns the display title for a mode.
// Source: PermissionMode.ts:123-125
func PermissionModeTitle(mode PermissionMode) string {
	return getModeConfig(mode).Title
}

// PermissionModeShortTitle returns the short display title.
// Source: PermissionMode.ts:131-133
func PermissionModeShortTitle(mode PermissionMode) string {
	return getModeConfig(mode).ShortTitle
}

// PermissionModeSymbol returns the symbol for a mode.
// Source: PermissionMode.ts:135-137
func PermissionModeSymbol(mode PermissionMode) string {
	return getModeConfig(mode).Symbol
}

// GetModeColor returns the semantic color key for a mode.
// Source: PermissionMode.ts:139-141
func GetModeColor(mode PermissionMode) ModeColorKey {
	return getModeConfig(mode).Color
}

// ExternalPermissionModes are the modes exposed to external users/SDK.
// Source: types/permissions.ts EXTERNAL_PERMISSION_MODES
var ExternalPermissionModes = []PermissionMode{
	ModeDefault,
	ModePlan,
	ModeAcceptEdits,
	ModeBypassPermissions,
	ModeDontAsk,
}

// AllPermissionModes includes both external and internal modes.
var AllPermissionModes = []PermissionMode{
	ModeDefault,
	ModePlan,
	ModeAcceptEdits,
	ModeBypassPermissions,
	ModeDontAsk,
	ModeAuto,
}

// IsExternalPermissionMode checks if a mode is an external (non-internal) mode.
// Source: PermissionMode.ts:97-105
func IsExternalPermissionMode(mode PermissionMode) bool {
	return mode != ModeAuto
}

// ToExternalPermissionMode converts an internal mode to its external equivalent.
// Source: PermissionMode.ts:111-115
func ToExternalPermissionMode(mode PermissionMode) PermissionMode {
	return getModeConfig(mode).External
}

// PermissionModeFromString parses a string into a PermissionMode, defaulting to ModeDefault.
// Source: PermissionMode.ts:117-121
func PermissionModeFromString(s string) PermissionMode {
	for _, m := range AllPermissionModes {
		if string(m) == s {
			return m
		}
	}
	return ModeDefault
}

// IsDefaultMode checks if a mode is the default mode (or empty).
// Source: PermissionMode.ts:127-129
func IsDefaultMode(mode PermissionMode) bool {
	return mode == ModeDefault || mode == ""
}
