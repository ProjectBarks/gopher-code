package plugins

// PluginHooks aggregates all MCP/plugin hook objects into a single
// session-scoped bundle. It is the top-level entry point that the binary
// creates at startup to wire merged MCP clients, UI plugin state, and
// LSP recommendation logic into the main code path.
//
// Source: src/hooks/useMergedClients.ts, src/hooks/useManagePlugins.ts,
// src/hooks/useLspPluginRecommendation.tsx, src/hooks/usePluginRecommendationBase.tsx
type PluginHooks struct {
	MCPClients    *MergedClients
	State         *PluginState
	LspRec        *LspRecommender
	LspRecBase    *RecommendationBase[LspRecommendation]
}

// NewPluginHooks creates a PluginHooks with sensible zero-value defaults.
// Callers can replace individual fields after creation for production use.
func NewPluginHooks() *PluginHooks {
	return &PluginHooks{
		MCPClients: NewMergedClients(nil),
		State:      NewPluginState(),
		LspRec: NewLspRecommender(nil, LspRecommenderOpts{}),
		LspRecBase: NewRecommendationBase[LspRecommendation](false),
	}
}

// CheckLspRecommendation evaluates a file path through the full hook chain
// (recommender + base gate) and returns a recommendation, or nil.
func (ph *PluginHooks) CheckLspRecommendation(filePath string) *LspRecommendation {
	if ph.LspRec == nil || ph.LspRecBase == nil {
		return nil
	}
	rec := ph.LspRec.CheckFile(filePath)
	if rec != nil {
		ph.LspRecBase.SetRecommendation(rec)
	}
	return ph.LspRecBase.Recommendation()
}
