package provider

import (
	"os"
	"strings"
)

// Feature gates — build-time flags matching TS feature('...') from bun:bundle.
// Set via -ldflags at build time; override in tests.
// Source: constants/betas.ts:23-28
var (
	FeatureConnectorText        = false // feature('CONNECTOR_TEXT')
	FeatureTranscriptClassifier = false // feature('TRANSCRIPT_CLASSIFIER')
)

// BetaSummarizeConnectorText returns the beta header when CONNECTOR_TEXT is enabled.
// Source: constants/betas.ts:23-25
func BetaSummarizeConnectorText() string {
	if FeatureConnectorText {
		return "summarize-connector-text-2026-03-13"
	}
	return ""
}

// BetaAfkMode returns the beta header when TRANSCRIPT_CLASSIFIER is enabled.
// Source: constants/betas.ts:26-28
func BetaAfkMode() string {
	if FeatureTranscriptClassifier {
		return "afk-mode-2026-01-31"
	}
	return ""
}

// BetaCliInternal returns the beta header when USER_TYPE is "ant".
// Source: constants/betas.ts:29-30
func BetaCliInternal() string {
	if os.Getenv("USER_TYPE") == "ant" {
		return "cli-internal-2026-02-09"
	}
	return ""
}

// Beta header constants — Source: constants/betas.ts
const (
	BetaClaudeCode          = "claude-code-20250219"              // Source: betas.ts:3
	BetaInterleavedThinking = "interleaved-thinking-2025-05-14"   // Source: betas.ts:4-5
	BetaContext1M           = "context-1m-2025-08-07"             // Source: betas.ts:6
	BetaContextManagement   = "context-management-2025-06-27"     // Source: betas.ts:7
	BetaStructuredOutputs   = "structured-outputs-2025-12-15"     // Source: betas.ts:8
	BetaWebSearch           = "web-search-2025-03-05"             // Source: betas.ts:9
	BetaToolSearch1P        = "advanced-tool-use-2025-11-20"      // Source: betas.ts:13
	BetaToolSearch3P        = "tool-search-tool-2025-10-19"       // Source: betas.ts:14
	BetaEffort              = "effort-2025-11-24"                 // Source: betas.ts:15
	BetaTaskBudgets         = "task-budgets-2026-03-13"           // Source: betas.ts:16
	BetaPromptCachingScope  = "prompt-caching-scope-2026-01-05"   // Source: betas.ts:17-18
	BetaFastMode            = "fast-mode-2026-02-01"              // Source: betas.ts:19
	BetaRedactThinking       = "redact-thinking-2026-02-12"        // Source: betas.ts:20
	BetaTokenEfficientTools  = "token-efficient-tools-2026-03-28"  // Source: betas.ts:21-22
	BetaAdvisor              = "advisor-tool-2026-03-01"           // Source: betas.ts:31
)

// BedrockExtraParamsHeaders are betas that go in Bedrock extraBodyParams, not headers.
// Source: constants/betas.ts:38-42
var BedrockExtraParamsHeaders = map[string]bool{
	BetaInterleavedThinking: true,
	BetaContext1M:           true,
	BetaToolSearch3P:        true,
}

// VertexCountTokensAllowedBetas are the only betas allowed on Vertex countTokens API.
// Other betas will cause 400 errors.
// Source: constants/betas.ts:48-52
var VertexCountTokensAllowedBetas = map[string]bool{
	BetaClaudeCode:          true,
	BetaInterleavedThinking: true,
	BetaContextManagement:   true,
}

// GetToolSearchBetaHeader returns the provider-appropriate tool search beta.
// Source: utils/betas.ts:202-208
func GetToolSearchBetaHeader() string {
	p := GetAPIProvider()
	if p == ProviderVertex || p == ProviderBedrock {
		return BetaToolSearch3P
	}
	return BetaToolSearch1P
}

// ShouldIncludeFirstPartyOnlyBetas checks if experimental betas should be included.
// Source: utils/betas.ts:215-220
func ShouldIncludeFirstPartyOnlyBetas() bool {
	p := GetAPIProvider()
	return (p == ProviderFirstParty || p == ProviderFoundry) &&
		!isEnvTruthy(os.Getenv("CLAUDE_CODE_DISABLE_EXPERIMENTAL_BETAS"))
}

// ModelSupportsISP checks if a model supports interleaved thinking.
// Source: utils/betas.ts:92-112
func ModelSupportsISP(model string) bool {
	canonical := GetCanonicalName(model)
	p := GetAPIProvider()
	if p == ProviderFoundry {
		return true
	}
	if p == ProviderFirstParty {
		return !strings.Contains(canonical, "claude-3-")
	}
	// 3P: only Opus 4+ and Sonnet 4+
	return strings.Contains(canonical, "claude-opus-4") ||
		strings.Contains(canonical, "claude-sonnet-4")
}

// ModelSupportsContextManagement checks if context management is supported.
// Source: utils/betas.ts:125-139
func ModelSupportsContextManagement(model string) bool {
	canonical := GetCanonicalName(model)
	p := GetAPIProvider()
	if p == ProviderFoundry {
		return true
	}
	if p == ProviderFirstParty {
		return !strings.Contains(canonical, "claude-3-")
	}
	return strings.Contains(canonical, "claude-opus-4") ||
		strings.Contains(canonical, "claude-sonnet-4") ||
		strings.Contains(canonical, "claude-haiku-4")
}

// GetAllModelBetas builds the complete beta header array for a model.
// Source: utils/betas.ts:234-369
func GetAllModelBetas(model string) []string {
	var betas []string
	isHaiku := strings.Contains(GetCanonicalName(model), "haiku")
	includeFirstParty := ShouldIncludeFirstPartyOnlyBetas()
	provider := GetAPIProvider()

	// Non-Haiku: add claude-code beta
	if !isHaiku {
		betas = append(betas, BetaClaudeCode)
	}

	// 1M context
	if Has1mContext(model) {
		betas = append(betas, BetaContext1M)
	}

	// Interleaved thinking
	if !isEnvTruthy(os.Getenv("DISABLE_INTERLEAVED_THINKING")) && ModelSupportsISP(model) {
		betas = append(betas, BetaInterleavedThinking)
	}

	// Redact thinking for interactive sessions
	if includeFirstParty && ModelSupportsISP(model) {
		betas = append(betas, BetaRedactThinking)
	}

	// Context management
	if includeFirstParty && ModelSupportsContextManagement(model) {
		betas = append(betas, BetaContextManagement)
	}

	// Web search for Vertex Claude 4+ or Foundry
	if provider == ProviderVertex {
		canonical := GetCanonicalName(model)
		if strings.Contains(canonical, "claude-opus-4") ||
			strings.Contains(canonical, "claude-sonnet-4") ||
			strings.Contains(canonical, "claude-haiku-4") {
			betas = append(betas, BetaWebSearch)
		}
	}
	if provider == ProviderFoundry {
		betas = append(betas, BetaWebSearch)
	}

	// Prompt caching scope — always for 1P/Foundry
	if includeFirstParty {
		betas = append(betas, BetaPromptCachingScope)
	}

	// Honor ANTHROPIC_BETAS env var
	if envBetas := os.Getenv("ANTHROPIC_BETAS"); envBetas != "" {
		for _, b := range strings.Split(envBetas, ",") {
			b = strings.TrimSpace(b)
			if b != "" {
				betas = append(betas, b)
			}
		}
	}

	return betas
}

// GetModelBetas returns betas suitable for API headers (excluding Bedrock extras).
// Source: utils/betas.ts:371-377
func GetModelBetas(model string) []string {
	all := GetAllModelBetas(model)
	if GetAPIProvider() == ProviderBedrock {
		var filtered []string
		for _, b := range all {
			if !BedrockExtraParamsHeaders[b] {
				filtered = append(filtered, b)
			}
		}
		return filtered
	}
	return all
}

// GetBedrockExtraBodyParamsBetas returns betas for Bedrock extraBodyParams.
// Source: utils/betas.ts:379-384
func GetBedrockExtraBodyParamsBetas(model string) []string {
	all := GetAllModelBetas(model)
	var extra []string
	for _, b := range all {
		if BedrockExtraParamsHeaders[b] {
			extra = append(extra, b)
		}
	}
	return extra
}

// GetMergedBetas merges model betas with optional SDK-provided betas.
// Source: utils/betas.ts:397-428
func GetMergedBetas(model string, isAgenticQuery bool) []string {
	betas := make([]string, len(GetModelBetas(model)))
	copy(betas, GetModelBetas(model))

	// Agentic queries always need claude-code beta (even for Haiku)
	if isAgenticQuery {
		found := false
		for _, b := range betas {
			if b == BetaClaudeCode {
				found = true
				break
			}
		}
		if !found {
			betas = append(betas, BetaClaudeCode)
		}
	}

	return betas
}
