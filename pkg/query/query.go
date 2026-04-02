package query

import (
	"context"

	"github.com/projectbarks/gopher-code/pkg/provider"
	"github.com/projectbarks/gopher-code/pkg/session"
	"github.com/projectbarks/gopher-code/pkg/tools"
)

// Query is the recursive agent loop — the beating heart of the runtime.
//
// This drives multi-turn conversations by:
// 1. Building the model request from session state
// 2. Streaming the model response
// 3. Collecting assistant text and tool_use blocks
// 4. Executing tool calls via the orchestrator
// 5. Appending tool_result messages
// 6. Looping if the model wants to continue
//
// STUB: Returns error so all tests fail (red phase of TDD).
func Query(
	ctx context.Context,
	sess *session.SessionState,
	prov provider.ModelProvider,
	registry *tools.ToolRegistry,
	orchestrator *tools.ToolOrchestrator,
	onEvent EventCallback,
) error {
	return &AgentError{Kind: ErrProvider, Detail: "not yet implemented"}
}
