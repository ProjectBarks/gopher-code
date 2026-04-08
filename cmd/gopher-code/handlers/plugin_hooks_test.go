package handlers

import (
	"testing"

	pluginhooks "github.com/projectbarks/gopher-code/pkg/ui/hooks/plugins"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPluginHooks_Integration exercises the MCP/plugin hooks through the
// real code path that the binary uses. This ensures pkg/ui/hooks/plugins
// is reachable from cmd/gopher-code via the handlers package.
//
// Source: T405 integration test

func TestPluginHooks_NewAndDefaults(t *testing.T) {
	ph := pluginhooks.NewPluginHooks()
	require.NotNil(t, ph)
	require.NotNil(t, ph.MCPClients)
	require.NotNil(t, ph.State)
	require.NotNil(t, ph.LspRec)
	require.NotNil(t, ph.LspRecBase)

	// Defaults: no servers, no plugins, no recommendations.
	assert.Empty(t, ph.MCPClients.All())
	assert.Empty(t, ph.MCPClients.Names())
	assert.Equal(t, 0, ph.State.Count())
	assert.Nil(t, ph.LspRecBase.Recommendation())
}

func TestPluginHooks_MergedClientsIntegration(t *testing.T) {
	ph := pluginhooks.NewPluginHooks()

	// Add dynamic servers (simulates plugin-provided MCP servers).
	ph.MCPClients.SetDynamic([]pluginhooks.MCPServerConnection{
		{Name: "plugin-server-a"},
		{Name: "plugin-server-b"},
	})
	names := ph.MCPClients.Names()
	assert.Len(t, names, 2)
	assert.Contains(t, names, "plugin-server-a")
	assert.Contains(t, names, "plugin-server-b")
}

func TestPluginHooks_PluginStateIntegration(t *testing.T) {
	ph := pluginhooks.NewPluginHooks()

	ph.State.Set(pluginhooks.PluginInfo{
		Name:   "test-plugin",
		Source: "test@marketplace",
	}, pluginhooks.StatusEnabled)

	assert.Equal(t, 1, ph.State.Count())
	enabled := ph.State.Enabled()
	require.Len(t, enabled, 1)
	assert.Equal(t, "test-plugin", enabled[0].Name)
}

func TestPluginHooks_LspRecommendationIntegration(t *testing.T) {
	// Create hooks with a custom recommender that has a real registry.
	ph := pluginhooks.NewPluginHooks()
	ph.LspRec = pluginhooks.NewLspRecommender([]pluginhooks.LspPluginEntry{
		{
			PluginID:    "gopls-lsp@official",
			PluginName:  "Go LSP",
			Description: "Go language server via gopls",
			IsOfficial:  true,
			Extensions:  []string{".go"},
			Command:     "gopls",
		},
	}, pluginhooks.LspRecommenderOpts{
		IsBinaryInstalled: func(cmd string) bool { return true },
	})

	// Exercise the full chain: file check -> recommender -> base gate.
	rec := ph.CheckLspRecommendation("/project/main.go")
	require.NotNil(t, rec)
	assert.Equal(t, "gopls-lsp@official", rec.PluginID)
	assert.Equal(t, ".go", rec.FileExtension)

	// Second call should return the cached recommendation (one per session).
	rec2 := ph.CheckLspRecommendation("/project/other.go")
	// The recommender won't fire again (one-per-session), but the base
	// still holds the previous recommendation.
	assert.NotNil(t, rec2)
	assert.Equal(t, "gopls-lsp@official", rec2.PluginID)
}

func TestPluginHooks_LspRecommendationNilSafe(t *testing.T) {
	// Nil recommender/base should not panic.
	ph := &pluginhooks.PluginHooks{}
	rec := ph.CheckLspRecommendation("/project/main.go")
	assert.Nil(t, rec)
}
