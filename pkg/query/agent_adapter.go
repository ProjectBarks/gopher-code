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
