package provider

import (
	"os"
	"regexp"
	"strings"
)

// Source: utils/model/providers.ts

// APIProvider identifies the API backend.
type APIProvider string

const (
	ProviderFirstParty APIProvider = "firstParty"
	ProviderBedrock    APIProvider = "bedrock"
	ProviderVertex     APIProvider = "vertex"
	ProviderFoundry    APIProvider = "foundry"
)

// GetAPIProvider returns the active API provider based on environment variables.
// Source: utils/model/providers.ts:6-14
func GetAPIProvider() APIProvider {
	if isEnvTruthy(os.Getenv("CLAUDE_CODE_USE_BEDROCK")) {
		return ProviderBedrock
	}
	if isEnvTruthy(os.Getenv("CLAUDE_CODE_USE_VERTEX")) {
		return ProviderVertex
	}
	if isEnvTruthy(os.Getenv("CLAUDE_CODE_USE_FOUNDRY")) {
		return ProviderFoundry
	}
	return ProviderFirstParty
}

func isEnvTruthy(val string) bool {
	v := strings.ToLower(strings.TrimSpace(val))
	return v == "1" || v == "true" || v == "yes"
}

// Source: utils/model/configs.ts

// ModelConfig holds provider-specific model IDs for a single model version.
type ModelConfig struct {
	FirstParty string
	Bedrock    string
	Vertex     string
	Foundry    string
}

// ModelForProvider returns the model ID string for the given provider.
func (mc ModelConfig) ModelForProvider(p APIProvider) string {
	switch p {
	case ProviderBedrock:
		return mc.Bedrock
	case ProviderVertex:
		return mc.Vertex
	case ProviderFoundry:
		return mc.Foundry
	default:
		return mc.FirstParty
	}
}

// All model configs — Source: utils/model/configs.ts
var (
	// Source: configs.ts:9-14
	ModelConfigSonnet37 = ModelConfig{
		FirstParty: "claude-3-7-sonnet-20250219",
		Bedrock:    "us.anthropic.claude-3-7-sonnet-20250219-v1:0",
		Vertex:     "claude-3-7-sonnet@20250219",
		Foundry:    "claude-3-7-sonnet",
	}
	// Source: configs.ts:16-21
	ModelConfigSonnet35 = ModelConfig{
		FirstParty: "claude-3-5-sonnet-20241022",
		Bedrock:    "anthropic.claude-3-5-sonnet-20241022-v2:0",
		Vertex:     "claude-3-5-sonnet-v2@20241022",
		Foundry:    "claude-3-5-sonnet",
	}
	// Source: configs.ts:23-28
	ModelConfigHaiku35 = ModelConfig{
		FirstParty: "claude-3-5-haiku-20241022",
		Bedrock:    "us.anthropic.claude-3-5-haiku-20241022-v1:0",
		Vertex:     "claude-3-5-haiku@20241022",
		Foundry:    "claude-3-5-haiku",
	}
	// Source: configs.ts:30-35
	ModelConfigHaiku45 = ModelConfig{
		FirstParty: "claude-haiku-4-5-20251001",
		Bedrock:    "us.anthropic.claude-haiku-4-5-20251001-v1:0",
		Vertex:     "claude-haiku-4-5@20251001",
		Foundry:    "claude-haiku-4-5",
	}
	// Source: configs.ts:37-42
	ModelConfigSonnet4 = ModelConfig{
		FirstParty: "claude-sonnet-4-20250514",
		Bedrock:    "us.anthropic.claude-sonnet-4-20250514-v1:0",
		Vertex:     "claude-sonnet-4@20250514",
		Foundry:    "claude-sonnet-4",
	}
	// Source: configs.ts:44-49
	ModelConfigSonnet45 = ModelConfig{
		FirstParty: "claude-sonnet-4-5-20250929",
		Bedrock:    "us.anthropic.claude-sonnet-4-5-20250929-v1:0",
		Vertex:     "claude-sonnet-4-5@20250929",
		Foundry:    "claude-sonnet-4-5",
	}
	// Source: configs.ts:51-56
	ModelConfigOpus4 = ModelConfig{
		FirstParty: "claude-opus-4-20250514",
		Bedrock:    "us.anthropic.claude-opus-4-20250514-v1:0",
		Vertex:     "claude-opus-4@20250514",
		Foundry:    "claude-opus-4",
	}
	// Source: configs.ts:58-63
	ModelConfigOpus41 = ModelConfig{
		FirstParty: "claude-opus-4-1-20250805",
		Bedrock:    "us.anthropic.claude-opus-4-1-20250805-v1:0",
		Vertex:     "claude-opus-4-1@20250805",
		Foundry:    "claude-opus-4-1",
	}
	// Source: configs.ts:65-70
	ModelConfigOpus45 = ModelConfig{
		FirstParty: "claude-opus-4-5-20251101",
		Bedrock:    "us.anthropic.claude-opus-4-5-20251101-v1:0",
		Vertex:     "claude-opus-4-5@20251101",
		Foundry:    "claude-opus-4-5",
	}
	// Source: configs.ts:72-77
	ModelConfigOpus46 = ModelConfig{
		FirstParty: "claude-opus-4-6",
		Bedrock:    "us.anthropic.claude-opus-4-6-v1",
		Vertex:     "claude-opus-4-6",
		Foundry:    "claude-opus-4-6",
	}
	// Source: configs.ts:79-84
	ModelConfigSonnet46 = ModelConfig{
		FirstParty: "claude-sonnet-4-6",
		Bedrock:    "us.anthropic.claude-sonnet-4-6",
		Vertex:     "claude-sonnet-4-6",
		Foundry:    "claude-sonnet-4-6",
	}
)

// AllModelConfigs maps short keys to their ModelConfig.
// Source: configs.ts:87-99
var AllModelConfigs = map[string]ModelConfig{
	"haiku35":  ModelConfigHaiku35,
	"haiku45":  ModelConfigHaiku45,
	"sonnet35": ModelConfigSonnet35,
	"sonnet37": ModelConfigSonnet37,
	"sonnet40": ModelConfigSonnet4,
	"sonnet45": ModelConfigSonnet45,
	"sonnet46": ModelConfigSonnet46,
	"opus40":   ModelConfigOpus4,
	"opus41":   ModelConfigOpus41,
	"opus45":   ModelConfigOpus45,
	"opus46":   ModelConfigOpus46,
}

// GetModelString returns the provider-appropriate model ID for a short key.
func GetModelString(key string) string {
	cfg, ok := AllModelConfigs[key]
	if !ok {
		return ""
	}
	return cfg.ModelForProvider(GetAPIProvider())
}

// Source: utils/model/aliases.ts

// ModelAliases lists all recognized model aliases.
// Source: aliases.ts:1-9
var ModelAliases = []string{
	"sonnet", "opus", "haiku", "best", "sonnet[1m]", "opus[1m]", "opusplan",
}

// ModelFamilyAliases are bare family names that act as wildcards.
// Source: aliases.ts:21
var ModelFamilyAliases = []string{"sonnet", "opus", "haiku"}

// IsModelAlias checks if a string is a recognized model alias.
// Source: aliases.ts:12-14
func IsModelAlias(model string) bool {
	for _, a := range ModelAliases {
		if a == model {
			return true
		}
	}
	return false
}

// IsModelFamilyAlias checks if a string is a family alias.
// Source: aliases.ts:23-25
func IsModelFamilyAlias(model string) bool {
	for _, a := range ModelFamilyAliases {
		if a == model {
			return true
		}
	}
	return false
}

// Source: utils/model/model.ts

// GetDefaultSonnetModel returns the default Sonnet model.
// Source: model.ts:119-128
func GetDefaultSonnetModel() string {
	if env := os.Getenv("ANTHROPIC_DEFAULT_SONNET_MODEL"); env != "" {
		return env
	}
	if GetAPIProvider() != ProviderFirstParty {
		return GetModelString("sonnet45")
	}
	return GetModelString("sonnet46")
}

// GetDefaultOpusModel returns the default Opus model.
// Source: model.ts:105-116
func GetDefaultOpusModel() string {
	if env := os.Getenv("ANTHROPIC_DEFAULT_OPUS_MODEL"); env != "" {
		return env
	}
	return GetModelString("opus46")
}

// GetDefaultHaikuModel returns the default Haiku model.
// Source: model.ts:131-138
func GetDefaultHaikuModel() string {
	if env := os.Getenv("ANTHROPIC_DEFAULT_HAIKU_MODEL"); env != "" {
		return env
	}
	return GetModelString("haiku45")
}

// GetSmallFastModel returns the small fast model for compaction/classification.
// Source: model.ts:36-38
func GetSmallFastModel() string {
	if env := os.Getenv("ANTHROPIC_SMALL_FAST_MODEL"); env != "" {
		return env
	}
	return GetDefaultHaikuModel()
}

// GetBestModel returns the most capable model.
// Source: model.ts:100-102
func GetBestModel() string {
	return GetDefaultOpusModel()
}

// Has1mContext checks if a model string includes the [1m] suffix.
// Source: context.ts:71
func Has1mContext(model string) bool {
	return strings.HasSuffix(strings.ToLower(model), "[1m]")
}

// NormalizeModelStringForAPI strips context window suffixes for API calls.
// Source: model.ts:616-618
func NormalizeModelStringForAPI(model string) string {
	re := regexp.MustCompile(`(?i)\[(1|2)m\]`)
	return re.ReplaceAllString(model, "")
}

// ParseUserSpecifiedModel resolves a model alias or name to a full model ID.
// Supports [1m] suffix on any alias.
// Source: model.ts:445-506
func ParseUserSpecifiedModel(modelInput string) string {
	trimmed := strings.TrimSpace(modelInput)
	normalized := strings.ToLower(trimmed)

	has1m := Has1mContext(normalized)
	modelStr := normalized
	if has1m {
		modelStr = strings.TrimSpace(regexp.MustCompile(`(?i)\[1m\]$`).ReplaceAllString(normalized, ""))
	}

	suffix := ""
	if has1m {
		suffix = "[1m]"
	}

	if IsModelAlias(modelStr) {
		switch modelStr {
		case "opusplan":
			return GetDefaultSonnetModel() + suffix
		case "sonnet":
			return GetDefaultSonnetModel() + suffix
		case "haiku":
			return GetDefaultHaikuModel() + suffix
		case "opus":
			return GetDefaultOpusModel() + suffix
		case "best":
			return GetBestModel()
		}
	}

	// Legacy Opus 4/4.1 remap on firstParty
	// Source: model.ts:477-483
	if GetAPIProvider() == ProviderFirstParty && isLegacyOpusFirstParty(modelStr) && isLegacyModelRemapEnabled() {
		return GetDefaultOpusModel() + suffix
	}

	// Preserve original case for custom model names
	if has1m {
		base := regexp.MustCompile(`(?i)\[1m\]$`).ReplaceAllString(trimmed, "")
		return strings.TrimSpace(base) + "[1m]"
	}
	return trimmed
}

// Source: model.ts:538-543
var legacyOpusFirstParty = []string{
	"claude-opus-4-20250514",
	"claude-opus-4-1-20250805",
	"claude-opus-4-0",
	"claude-opus-4-1",
}

func isLegacyOpusFirstParty(model string) bool {
	for _, m := range legacyOpusFirstParty {
		if model == m {
			return true
		}
	}
	return false
}

// Source: model.ts:552-554
func isLegacyModelRemapEnabled() bool {
	return !isEnvTruthy(os.Getenv("CLAUDE_CODE_DISABLE_LEGACY_MODEL_REMAP"))
}

// FirstPartyNameToCanonical strips date/provider suffixes to get a canonical name.
// Input must be a 1P-format ID. Resolution order: more specific first.
// Source: model.ts:217-270
func FirstPartyNameToCanonical(name string) string {
	n := strings.ToLower(name)

	// Order matters: check more specific versions first (4-6 before 4-5 before 4)
	checks := []struct {
		substring string
		canonical string
	}{
		{"claude-opus-4-6", "claude-opus-4-6"},
		{"claude-opus-4-5", "claude-opus-4-5"},
		{"claude-opus-4-1", "claude-opus-4-1"},
		{"claude-opus-4", "claude-opus-4"},
		{"claude-sonnet-4-6", "claude-sonnet-4-6"},
		{"claude-sonnet-4-5", "claude-sonnet-4-5"},
		{"claude-sonnet-4", "claude-sonnet-4"},
		{"claude-haiku-4-5", "claude-haiku-4-5"},
		{"claude-3-7-sonnet", "claude-3-7-sonnet"},
		{"claude-3-5-sonnet", "claude-3-5-sonnet"},
		{"claude-3-5-haiku", "claude-3-5-haiku"},
		{"claude-3-opus", "claude-3-opus"},
		{"claude-3-sonnet", "claude-3-sonnet"},
		{"claude-3-haiku", "claude-3-haiku"},
	}
	for _, c := range checks {
		if strings.Contains(n, c.substring) {
			return c.canonical
		}
	}

	// Fallback regex
	re := regexp.MustCompile(`(claude-(\d+-\d+-)?[a-zA-Z]+)`)
	if m := re.FindString(n); m != "" {
		return m
	}
	return n
}

// GetCanonicalName maps any model string to its canonical form.
// Source: model.ts:279-283
func GetCanonicalName(fullModelName string) string {
	return FirstPartyNameToCanonical(fullModelName)
}

// GetPublicModelDisplayName returns a human-readable name for known models.
// Source: model.ts:349-384
func GetPublicModelDisplayName(model string) string {
	ms := func(key string) string { return GetModelString(key) }

	switch model {
	case ms("opus46"):
		return "Opus 4.6"
	case ms("opus46") + "[1m]":
		return "Opus 4.6 (1M context)"
	case ms("opus45"):
		return "Opus 4.5"
	case ms("opus41"):
		return "Opus 4.1"
	case ms("opus40"):
		return "Opus 4"
	case ms("sonnet46") + "[1m]":
		return "Sonnet 4.6 (1M context)"
	case ms("sonnet46"):
		return "Sonnet 4.6"
	case ms("sonnet45") + "[1m]":
		return "Sonnet 4.5 (1M context)"
	case ms("sonnet45"):
		return "Sonnet 4.5"
	case ms("sonnet40"):
		return "Sonnet 4"
	case ms("sonnet37"):
		return "Sonnet 3.7"
	case ms("sonnet35"):
		return "Sonnet 3.5"
	case ms("haiku45"):
		return "Haiku 4.5"
	case ms("haiku35"):
		return "Haiku 3.5"
	default:
		return ""
	}
}

// GetMarketingNameForModel returns marketing display name for a model.
// Source: model.ts:570-614
func GetMarketingNameForModel(modelID string) string {
	if GetAPIProvider() == ProviderFoundry {
		return ""
	}
	has1m := strings.Contains(strings.ToLower(modelID), "[1m]")
	canonical := GetCanonicalName(modelID)

	type nameCheck struct {
		substring string
		with1m    string
		without1m string
	}
	checks := []nameCheck{
		{"claude-opus-4-6", "Opus 4.6 (with 1M context)", "Opus 4.6"},
		{"claude-opus-4-5", "", "Opus 4.5"},
		{"claude-opus-4-1", "", "Opus 4.1"},
		{"claude-opus-4", "", "Opus 4"},
		{"claude-sonnet-4-6", "Sonnet 4.6 (with 1M context)", "Sonnet 4.6"},
		{"claude-sonnet-4-5", "Sonnet 4.5 (with 1M context)", "Sonnet 4.5"},
		{"claude-sonnet-4", "Sonnet 4 (with 1M context)", "Sonnet 4"},
		{"claude-3-7-sonnet", "", "Claude 3.7 Sonnet"},
		{"claude-3-5-sonnet", "", "Claude 3.5 Sonnet"},
		{"claude-haiku-4-5", "", "Haiku 4.5"},
		{"claude-3-5-haiku", "", "Claude 3.5 Haiku"},
	}
	for _, c := range checks {
		if strings.Contains(canonical, c.substring) {
			if has1m && c.with1m != "" {
				return c.with1m
			}
			return c.without1m
		}
	}
	return ""
}

// IsNonCustomOpusModel checks if a model is a known Opus variant.
// Source: model.ts:40-47
func IsNonCustomOpusModel(model string) bool {
	ms := func(key string) string { return GetModelString(key) }
	return model == ms("opus40") || model == ms("opus41") ||
		model == ms("opus45") || model == ms("opus46")
}

// GetPublicModelName returns a safe name for public display (e.g., git trailers).
// Source: model.ts:425-431
func GetPublicModelName(model string) string {
	name := GetPublicModelDisplayName(model)
	if name != "" {
		return "Claude " + name
	}
	return "Claude (" + model + ")"
}
