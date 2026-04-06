package plugins

import (
	"testing"

	"github.com/projectbarks/gopher-code/pkg/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// MergeClients tests
// Source: src/hooks/useMergedClients.ts — mergeClients()
// ---------------------------------------------------------------------------

func conn(name string, scope mcp.ConfigScope) MCPServerConnection {
	return MCPServerConnection{
		Name:   name,
		Config: mcp.ScopedServerConfig{Scope: scope},
	}
}

func TestMergeClients_InitialOnly(t *testing.T) {
	initial := []MCPServerConnection{conn("a", mcp.ScopeUser), conn("b", mcp.ScopeProject)}
	got := MergeClients(initial, nil)
	assert.Equal(t, 2, len(got))
	assert.Equal(t, "a", got[0].Name)
	assert.Equal(t, "b", got[1].Name)
}

func TestMergeClients_DynamicOnly(t *testing.T) {
	got := MergeClients(nil, []MCPServerConnection{conn("x", mcp.ScopeManaged)})
	// With nil initial, dynamic entries should still appear.
	assert.Equal(t, 1, len(got))
	assert.Equal(t, "x", got[0].Name)
}

func TestMergeClients_DeduplicatesByName(t *testing.T) {
	initial := []MCPServerConnection{conn("srv", mcp.ScopeUser)}
	dynamic := []MCPServerConnection{conn("srv", mcp.ScopeManaged), conn("other", mcp.ScopeManaged)}
	got := MergeClients(initial, dynamic)

	// "srv" from initial wins; "other" from dynamic is appended.
	require.Len(t, got, 2)
	assert.Equal(t, "srv", got[0].Name)
	assert.Equal(t, mcp.ScopeUser, got[0].Config.Scope) // initial's scope wins
	assert.Equal(t, "other", got[1].Name)
}

func TestMergeClients_BothNil(t *testing.T) {
	got := MergeClients(nil, nil)
	assert.NotNil(t, got)
	assert.Empty(t, got)
}

func TestMergedClients_SetDynamic(t *testing.T) {
	mc := NewMergedClients([]MCPServerConnection{conn("a", mcp.ScopeUser)})
	assert.Equal(t, []string{"a"}, mc.Names())

	mc.SetDynamic([]MCPServerConnection{conn("b", mcp.ScopeManaged)})
	assert.Equal(t, []string{"a", "b"}, mc.Names())

	// Replace dynamic — "b" goes away, "c" appears.
	mc.SetDynamic([]MCPServerConnection{conn("c", mcp.ScopeManaged)})
	assert.Equal(t, []string{"a", "c"}, mc.Names())
}

// ---------------------------------------------------------------------------
// PluginState tests
// Source: src/hooks/useManagePlugins.ts — AppState.plugins
// ---------------------------------------------------------------------------

func info(name string) PluginInfo {
	return PluginInfo{Name: name, Source: name + "@marketplace"}
}

func TestPluginState_SetAndGet(t *testing.T) {
	ps := NewPluginState()
	ps.Set(info("alpha"), StatusEnabled)
	ps.Set(info("beta"), StatusDisabled)

	pi, status, ok := ps.Get("alpha")
	require.True(t, ok)
	assert.Equal(t, "alpha", pi.Name)
	assert.Equal(t, StatusEnabled, status)

	_, _, ok = ps.Get("nonexistent")
	assert.False(t, ok)
}

func TestPluginState_StatusTransitions(t *testing.T) {
	ps := NewPluginState()
	ps.Set(info("p"), StatusLoading)

	ok := ps.SetStatus("p", StatusEnabled)
	assert.True(t, ok)
	_, s, _ := ps.Get("p")
	assert.Equal(t, StatusEnabled, s)

	ok = ps.SetStatus("p", StatusErrored)
	assert.True(t, ok)
	_, s, _ = ps.Get("p")
	assert.Equal(t, StatusErrored, s)

	// Unknown plugin returns false.
	assert.False(t, ps.SetStatus("missing", StatusEnabled))
}

func TestPluginState_EnabledDisabledErrored(t *testing.T) {
	ps := NewPluginState()
	ps.Set(info("a"), StatusEnabled)
	ps.Set(info("b"), StatusEnabled)
	ps.Set(info("c"), StatusDisabled)
	ps.Set(info("d"), StatusErrored)
	ps.Set(info("e"), StatusLoading)

	assert.Len(t, ps.Enabled(), 2)
	assert.Len(t, ps.Disabled(), 1)
	assert.Len(t, ps.Errored(), 1)
	assert.Equal(t, 5, ps.Count())
}

func TestPluginState_Errors(t *testing.T) {
	ps := NewPluginState()
	ps.AddError(PluginError{Type: "generic-error", Source: "plugin-commands", Detail: "boom"})
	ps.AddError(PluginError{Type: "mcp-config-invalid", Source: "plugin:x", Plugin: "x", Detail: "bad"})

	errs := ps.Errors()
	require.Len(t, errs, 2)
	assert.Equal(t, "generic-error", errs[0].Type)
	assert.Contains(t, errs[0].Error(), "boom")
	assert.Contains(t, errs[1].Error(), "x")

	ps.ClearErrors()
	assert.Empty(t, ps.Errors())
}

func TestPluginState_NeedsRefresh(t *testing.T) {
	ps := NewPluginState()
	assert.False(t, ps.NeedsRefresh())
	ps.SetNeedsRefresh(true)
	assert.True(t, ps.NeedsRefresh())
	ps.SetNeedsRefresh(false)
	assert.False(t, ps.NeedsRefresh())
}

func TestPluginState_All(t *testing.T) {
	ps := NewPluginState()
	ps.Set(info("x"), StatusEnabled)
	ps.Set(info("y"), StatusDisabled)
	all := ps.All()
	assert.Equal(t, StatusEnabled, all["x"])
	assert.Equal(t, StatusDisabled, all["y"])
}

// ---------------------------------------------------------------------------
// LSP Recommendation tests
// Source: src/hooks/useLspPluginRecommendation.tsx + lspRecommendation.ts
// ---------------------------------------------------------------------------

var testRegistry = []LspPluginEntry{
	{
		PluginID:    "gopls-lsp@official",
		PluginName:  "Go LSP",
		Marketplace: "official",
		Description: "Go language server via gopls",
		IsOfficial:  true,
		Extensions:  []string{".go"},
		Command:     "gopls",
	},
	{
		PluginID:    "tsserver-lsp@official",
		PluginName:  "TypeScript LSP",
		Marketplace: "official",
		Description: "TypeScript language server",
		IsOfficial:  true,
		Extensions:  []string{".ts", ".tsx", ".js", ".jsx"},
		Command:     "typescript-language-server",
	},
	{
		PluginID:    "pyright-lsp@community",
		PluginName:  "Python LSP",
		Marketplace: "community",
		Description: "Python language server via pyright",
		IsOfficial:  false,
		Extensions:  []string{".py"},
		Command:     "pyright-langserver",
	},
}

func alwaysInstalled(cmd string) bool { return true }

func TestLspRecommender_GoFile(t *testing.T) {
	r := NewLspRecommender(testRegistry, LspRecommenderOpts{
		IsBinaryInstalled: alwaysInstalled,
	})
	rec := r.CheckFile("/home/user/project/main.go")
	require.NotNil(t, rec)
	assert.Equal(t, "gopls-lsp@official", rec.PluginID)
	assert.Equal(t, "Go LSP", rec.PluginName)
	assert.Equal(t, ".go", rec.FileExtension)
}

func TestLspRecommender_TypeScriptFile(t *testing.T) {
	r := NewLspRecommender(testRegistry, LspRecommenderOpts{
		IsBinaryInstalled: alwaysInstalled,
	})
	rec := r.CheckFile("/project/src/index.ts")
	require.NotNil(t, rec)
	assert.Equal(t, "tsserver-lsp@official", rec.PluginID)
	assert.Equal(t, ".ts", rec.FileExtension)
}

func TestLspRecommender_TSXFile(t *testing.T) {
	r := NewLspRecommender(testRegistry, LspRecommenderOpts{
		IsBinaryInstalled: alwaysInstalled,
	})
	rec := r.CheckFile("/project/App.tsx")
	require.NotNil(t, rec)
	assert.Equal(t, "tsserver-lsp@official", rec.PluginID)
	assert.Equal(t, ".tsx", rec.FileExtension)
}

func TestLspRecommender_OnePerSession(t *testing.T) {
	r := NewLspRecommender(testRegistry, LspRecommenderOpts{
		IsBinaryInstalled: alwaysInstalled,
	})
	rec1 := r.CheckFile("/project/main.go")
	require.NotNil(t, rec1)

	// Second file in the same session: no new recommendation.
	rec2 := r.CheckFile("/project/index.ts")
	assert.Nil(t, rec2)
}

func TestLspRecommender_SkipAlreadyInstalled(t *testing.T) {
	r := NewLspRecommender(testRegistry, LspRecommenderOpts{
		InstalledPlugins:  []string{"gopls-lsp@official"},
		IsBinaryInstalled: alwaysInstalled,
	})
	rec := r.CheckFile("/project/main.go")
	assert.Nil(t, rec)
}

func TestLspRecommender_SkipNeverSuggest(t *testing.T) {
	r := NewLspRecommender(testRegistry, LspRecommenderOpts{
		NeverSuggest:      []string{"gopls-lsp@official"},
		IsBinaryInstalled: alwaysInstalled,
	})
	rec := r.CheckFile("/project/main.go")
	assert.Nil(t, rec)
}

func TestLspRecommender_SkipWhenDisabled(t *testing.T) {
	r := NewLspRecommender(testRegistry, LspRecommenderOpts{
		Disabled:          true,
		IsBinaryInstalled: alwaysInstalled,
	})
	rec := r.CheckFile("/project/main.go")
	assert.Nil(t, rec)
}

func TestLspRecommender_DisabledAfterMaxIgnores(t *testing.T) {
	r := NewLspRecommender(testRegistry, LspRecommenderOpts{
		IgnoredCount:      MaxIgnoredCount,
		IsBinaryInstalled: alwaysInstalled,
	})
	assert.True(t, r.IsDisabled())
	rec := r.CheckFile("/project/main.go")
	assert.Nil(t, rec)
}

func TestLspRecommender_BinaryNotInstalled(t *testing.T) {
	r := NewLspRecommender(testRegistry, LspRecommenderOpts{
		IsBinaryInstalled: func(cmd string) bool { return false },
	})
	rec := r.CheckFile("/project/main.go")
	assert.Nil(t, rec)
}

func TestLspRecommender_NoExtension(t *testing.T) {
	r := NewLspRecommender(testRegistry, LspRecommenderOpts{
		IsBinaryInstalled: alwaysInstalled,
	})
	rec := r.CheckFile("/project/Makefile")
	assert.Nil(t, rec)
}

func TestLspRecommender_OfficialFirst(t *testing.T) {
	// Add a community Go LSP to test sorting.
	registry := append([]LspPluginEntry{
		{
			PluginID:   "community-go@community",
			PluginName: "Community Go",
			Extensions: []string{".go"},
			Command:    "gopls",
			IsOfficial: false,
		},
	}, testRegistry...)

	r := NewLspRecommender(registry, LspRecommenderOpts{
		IsBinaryInstalled: alwaysInstalled,
	})
	rec := r.CheckFile("/project/main.go")
	require.NotNil(t, rec)
	// Official plugin should be recommended over community.
	assert.Equal(t, "gopls-lsp@official", rec.PluginID)
}

func TestLspRecommender_AddToNeverSuggest(t *testing.T) {
	r := NewLspRecommender(testRegistry, LspRecommenderOpts{
		IsBinaryInstalled: alwaysInstalled,
	})
	r.AddToNeverSuggest("gopls-lsp@official")
	rec := r.CheckFile("/project/main.go")
	assert.Nil(t, rec)
}

func TestLspRecommender_IncrementIgnored(t *testing.T) {
	r := NewLspRecommender(testRegistry, LspRecommenderOpts{
		IgnoredCount:      MaxIgnoredCount - 1,
		IsBinaryInstalled: alwaysInstalled,
	})
	assert.False(t, r.IsDisabled())
	r.IncrementIgnored()
	assert.True(t, r.IsDisabled())
}

func TestLspRecommender_SameFileNotRechecked(t *testing.T) {
	calls := 0
	r := NewLspRecommender(testRegistry, LspRecommenderOpts{
		IsBinaryInstalled: func(cmd string) bool {
			calls++
			// First call returns false, would return true later — but file
			// should not be re-checked.
			return calls > 1
		},
	})
	rec := r.CheckFile("/project/main.go")
	assert.Nil(t, rec) // binary not installed on first check
	rec = r.CheckFile("/project/main.go")
	assert.Nil(t, rec) // file already checked, even though binary now exists
}

// ---------------------------------------------------------------------------
// RecommendationBase tests
// Source: src/hooks/usePluginRecommendationBase.tsx
// ---------------------------------------------------------------------------

func TestRecommendationBase_TryResolve(t *testing.T) {
	rb := NewRecommendationBase[string](false)
	assert.Nil(t, rb.Recommendation())

	rb.TryResolve(func() *string {
		s := "hello"
		return &s
	})
	require.NotNil(t, rb.Recommendation())
	assert.Equal(t, "hello", *rb.Recommendation())
}

func TestRecommendationBase_NilResolveNoChange(t *testing.T) {
	rb := NewRecommendationBase[string](false)
	rb.TryResolve(func() *string { return nil })
	assert.Nil(t, rb.Recommendation())
}

func TestRecommendationBase_NoDoubleResolve(t *testing.T) {
	rb := NewRecommendationBase[string](false)
	s1 := "first"
	rb.TryResolve(func() *string { return &s1 })

	s2 := "second"
	rb.TryResolve(func() *string { return &s2 })

	// Should still be "first" — second resolve is gated.
	assert.Equal(t, "first", *rb.Recommendation())
}

func TestRecommendationBase_ClearAllowsReResolve(t *testing.T) {
	rb := NewRecommendationBase[string](false)
	s1 := "first"
	rb.TryResolve(func() *string { return &s1 })
	assert.Equal(t, "first", *rb.Recommendation())

	rb.Clear()
	assert.Nil(t, rb.Recommendation())

	s2 := "second"
	rb.TryResolve(func() *string { return &s2 })
	assert.Equal(t, "second", *rb.Recommendation())
}

func TestRecommendationBase_RemoteModeSuppresses(t *testing.T) {
	rb := NewRecommendationBase[string](true) // remote mode
	s := "nope"
	rb.TryResolve(func() *string { return &s })
	assert.Nil(t, rb.Recommendation())
}

func TestIsPluginInstalled(t *testing.T) {
	installed := map[string]struct{}{
		"foo@bar": {},
		"baz@qux": {},
	}
	assert.True(t, IsPluginInstalled("foo@bar", installed))
	assert.False(t, IsPluginInstalled("missing@nope", installed))
}
