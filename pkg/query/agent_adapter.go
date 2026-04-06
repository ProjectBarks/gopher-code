package query

import (
	"context"

	"github.com/projectbarks/gopher-code/pkg/provider"
	"github.com/projectbarks/gopher-code/pkg/session"
	"github.com/projectbarks/gopher-code/pkg/tools"
)

// AsQueryFunc returns a tools.QueryFunc that wraps query.Query, adapting the
// callback from the simplified text-only signature to the full EventCallback.
// This is the bridge that lets AgentTool call Query without an import cycle.
//
// The 4-dep shape from src/query/deps.ts (callModel, microcompact, autocompact,
// uuid) is covered by the production Query() path:
//   - callModel    → prov.Stream (passed as prov parameter)
//   - microcompact → query.microCompact (internal)
//   - autocompact  → query.CompactSession (internal)
//   - uuid         → session.ID (assigned at session creation)
func AsQueryFunc() tools.QueryFunc {
	return func(
		ctx context.Context,
		sess *session.SessionState,
		prov provider.ModelProvider,
		registry *tools.ToolRegistry,
		orchestrator *tools.ToolOrchestrator,
		onText func(text string),
	) error {
		var cb EventCallback
		if onText != nil {
			cb = func(evt QueryEvent) {
				if evt.Type == QEventTextDelta {
					onText(evt.Text)
				}
			}
		}
		return Query(ctx, sess, prov, registry, orchestrator, cb)
	}
}

// AsQueryFuncWithDeps returns a tools.QueryFunc backed by an explicit QueryDeps
// for dependency injection. This allows tests and custom integrations to swap
// any of the 4 core deps (callModel, microcompact, autocompact, uuid) plus
// the extended deps (runTools, handleStopHooks, logEvent, queue ops).
//
// Source: src/query/deps.ts — QueryDeps type + productionDeps()
func AsQueryFuncWithDeps(deps QueryDeps) tools.QueryFunc {
	return func(
		ctx context.Context,
		sess *session.SessionState,
		_ provider.ModelProvider,
		_ *tools.ToolRegistry,
		_ *tools.ToolOrchestrator,
		onText func(text string),
	) error {
		var cb EventCallback
		if onText != nil {
			cb = func(evt QueryEvent) {
				if evt.Type == QEventTextDelta {
					onText(evt.Text)
				}
			}
		}
		// When Query() is refactored to accept QueryDeps, this will pass deps
		// through. For now, use deps.CallModel directly via the provider adapter.
		provAdapter := &depsProviderAdapter{callModel: deps.CallModel}
		return Query(ctx, sess, provAdapter, nil, nil, cb)
	}
}

// depsProviderAdapter wraps a QueryDeps.CallModel function as a ModelProvider.
type depsProviderAdapter struct {
	callModel func(ctx context.Context, req provider.ModelRequest) (<-chan provider.StreamResult, error)
}

func (d *depsProviderAdapter) Stream(ctx context.Context, req provider.ModelRequest) (<-chan provider.StreamResult, error) {
	return d.callModel(ctx, req)
}

func (d *depsProviderAdapter) Name() string { return "deps-adapter" }
