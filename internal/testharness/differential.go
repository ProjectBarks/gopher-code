package testharness

import (
	"context"
	"fmt"
	"strings"

	"github.com/projectbarks/gopher-code/pkg/message"
	"github.com/projectbarks/gopher-code/pkg/provider"
	"github.com/projectbarks/gopher-code/pkg/query"
	"github.com/projectbarks/gopher-code/pkg/session"
	"github.com/projectbarks/gopher-code/pkg/tools"
)

// DiffResult captures behavioral differences between two query runs.
type DiffResult struct {
	Scenario     string
	MessageDiffs []MessageDiff
	ToolDiffs    []ToolDiff
	TokenDiffs   []TokenDiff
	ErrorDiff    *ErrorDiff
}

// MessageDiff captures a difference in message sequences.
type MessageDiff struct {
	Index   int
	Field   string
	ValueA  string
	ValueB  string
}

// ToolDiff captures a difference in tool call sequences.
type ToolDiff struct {
	TurnIndex int
	Field     string
	ValueA    string
	ValueB    string
}

// TokenDiff captures a difference in token accounting.
type TokenDiff struct {
	Field  string
	ValueA int
	ValueB int
}

// ErrorDiff captures a difference in error handling.
type ErrorDiff struct {
	ErrorA string
	ErrorB string
}

// HasDiffs returns true if any differences were found.
func (d *DiffResult) HasDiffs() bool {
	return len(d.MessageDiffs) > 0 ||
		len(d.ToolDiffs) > 0 ||
		len(d.TokenDiffs) > 0 ||
		d.ErrorDiff != nil
}

// Summary returns a human-readable diff summary.
func (d *DiffResult) Summary() string {
	if !d.HasDiffs() {
		return fmt.Sprintf("Scenario %q: no differences", d.Scenario)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Scenario %q: %d diffs\n", d.Scenario,
		len(d.MessageDiffs)+len(d.ToolDiffs)+len(d.TokenDiffs))
	for _, md := range d.MessageDiffs {
		fmt.Fprintf(&b, "  msg[%d].%s: %q vs %q\n", md.Index, md.Field, md.ValueA, md.ValueB)
	}
	for _, td := range d.ToolDiffs {
		fmt.Fprintf(&b, "  tool[%d].%s: %q vs %q\n", td.TurnIndex, td.Field, td.ValueA, td.ValueB)
	}
	for _, td := range d.TokenDiffs {
		fmt.Fprintf(&b, "  %s: %d vs %d\n", td.Field, td.ValueA, td.ValueB)
	}
	if d.ErrorDiff != nil {
		fmt.Fprintf(&b, "  error: %q vs %q\n", d.ErrorDiff.ErrorA, d.ErrorDiff.ErrorB)
	}
	return b.String()
}

// QueryFunc is the signature for a query function to compare.
type QueryFunc func(
	ctx context.Context,
	sess *session.SessionState,
	prov provider.ModelProvider,
	registry *tools.ToolRegistry,
	orchestrator *tools.ToolOrchestrator,
	onEvent query.EventCallback,
) error

// RunDifferential runs the same scenario through two query implementations
// and returns the behavioral differences.
func RunDifferential(
	scenario string,
	sessConfig session.SessionConfig,
	cwd string,
	userMessage string,
	turns []TurnScript,
	registry *tools.ToolRegistry,
	queryA QueryFunc,
	queryB QueryFunc,
) *DiffResult {
	result := &DiffResult{Scenario: scenario}

	// Run A
	provA := NewScriptedProvider(turns...)
	sessA := session.New(sessConfig, cwd)
	sessA.PushMessage(message.UserMessage(userMessage))
	orchA := tools.NewOrchestrator(registry)
	var eventsA []query.QueryEvent
	cbA := func(e query.QueryEvent) { eventsA = append(eventsA, e) }
	errA := queryA(context.Background(), sessA, provA, registry, orchA, cbA)

	// Run B with fresh copies
	provB := NewScriptedProvider(turns...)
	sessB := session.New(sessConfig, cwd)
	sessB.PushMessage(message.UserMessage(userMessage))
	orchB := tools.NewOrchestrator(registry)
	var eventsB []query.QueryEvent
	cbB := func(e query.QueryEvent) { eventsB = append(eventsB, e) }
	errB := queryB(context.Background(), sessB, provB, registry, orchB, cbB)

	// Compare errors
	errStrA, errStrB := "", ""
	if errA != nil {
		errStrA = errA.Error()
	}
	if errB != nil {
		errStrB = errB.Error()
	}
	if errStrA != errStrB {
		result.ErrorDiff = &ErrorDiff{ErrorA: errStrA, ErrorB: errStrB}
	}

	// Compare message counts
	if len(sessA.Messages) != len(sessB.Messages) {
		result.MessageDiffs = append(result.MessageDiffs, MessageDiff{
			Index: -1, Field: "count",
			ValueA: fmt.Sprintf("%d", len(sessA.Messages)),
			ValueB: fmt.Sprintf("%d", len(sessB.Messages)),
		})
	}

	// Compare messages
	minLen := len(sessA.Messages)
	if len(sessB.Messages) < minLen {
		minLen = len(sessB.Messages)
	}
	for i := 0; i < minLen; i++ {
		ma, mb := sessA.Messages[i], sessB.Messages[i]
		if ma.Role != mb.Role {
			result.MessageDiffs = append(result.MessageDiffs, MessageDiff{
				Index: i, Field: "role",
				ValueA: string(ma.Role), ValueB: string(mb.Role),
			})
		}
		if len(ma.Content) != len(mb.Content) {
			result.MessageDiffs = append(result.MessageDiffs, MessageDiff{
				Index: i, Field: "content_count",
				ValueA: fmt.Sprintf("%d", len(ma.Content)),
				ValueB: fmt.Sprintf("%d", len(mb.Content)),
			})
		}
	}

	// Compare token accounting
	if sessA.TotalInputTokens != sessB.TotalInputTokens {
		result.TokenDiffs = append(result.TokenDiffs, TokenDiff{
			Field: "total_input_tokens",
			ValueA: sessA.TotalInputTokens, ValueB: sessB.TotalInputTokens,
		})
	}
	if sessA.TotalOutputTokens != sessB.TotalOutputTokens {
		result.TokenDiffs = append(result.TokenDiffs, TokenDiff{
			Field: "total_output_tokens",
			ValueA: sessA.TotalOutputTokens, ValueB: sessB.TotalOutputTokens,
		})
	}
	if sessA.TurnCount != sessB.TurnCount {
		result.TokenDiffs = append(result.TokenDiffs, TokenDiff{
			Field: "turn_count",
			ValueA: sessA.TurnCount, ValueB: sessB.TurnCount,
		})
	}

	return result
}
