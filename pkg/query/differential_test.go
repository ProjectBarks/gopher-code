package query_test

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/projectbarks/gopher-code/internal/testharness"
	"github.com/projectbarks/gopher-code/pkg/compact"
	"github.com/projectbarks/gopher-code/pkg/message"
	"github.com/projectbarks/gopher-code/pkg/permissions"
	"github.com/projectbarks/gopher-code/pkg/provider"
	"github.com/projectbarks/gopher-code/pkg/query"
	"github.com/projectbarks/gopher-code/pkg/session"
	"github.com/projectbarks/gopher-code/pkg/tools"
)

// TestDifferentialSelfConsistency runs the same query implementation twice
// with identical inputs and verifies deterministic behavior.
// This validates the differential test infrastructure itself.
func TestDifferentialSelfConsistency(t *testing.T) {
	cfg := session.SessionConfig{
		Model:          "test-model",
		MaxTurns:       100,
		TokenBudget:    compact.DefaultBudget(),
		PermissionMode: permissions.AutoApprove,
	}
	registry := tools.NewRegistry()

	scenarios := []struct {
		name   string
		turns  []testharness.TurnScript
	}{
		{
			"text_only",
			[]testharness.TurnScript{
				testharness.MakeTextTurn("Hello!", provider.StopReasonEndTurn),
			},
		},
		{
			"tool_then_text",
			func() []testharness.TurnScript {
				spy := testharness.NewSpyTool("my_tool", false)
				registry.Register(spy)
				return []testharness.TurnScript{
					testharness.MakeToolTurn("t1", "my_tool", json.RawMessage(`{}`), provider.StopReasonToolUse),
					testharness.MakeTextTurn("Done", provider.StopReasonEndTurn),
				}
			}(),
		},
	}

	for _, sc := range scenarios {
		sc := sc
		t.Run(sc.name, func(t *testing.T) {
			diff := testharness.RunDifferential(
				sc.name, cfg, os.TempDir(), "hello",
				sc.turns, registry,
				query.Query, query.Query, // same implementation twice
			)

			t.Run("no_message_diffs", func(t *testing.T) {
				if len(diff.MessageDiffs) > 0 {
					t.Errorf("self-consistency should have no message diffs: %s", diff.Summary())
				}
			})
			t.Run("no_token_diffs", func(t *testing.T) {
				if len(diff.TokenDiffs) > 0 {
					t.Errorf("self-consistency should have no token diffs: %s", diff.Summary())
				}
			})
			t.Run("no_error_diff", func(t *testing.T) {
				if diff.ErrorDiff != nil {
					t.Errorf("self-consistency should have no error diff: %s", diff.Summary())
				}
			})
		})
	}
}

// TestDifferentialInfrastructure validates the differential runner works correctly.
func TestDifferentialInfrastructure(t *testing.T) {
	cfg := session.SessionConfig{
		Model:          "test-model",
		MaxTurns:       100,
		TokenBudget:    compact.DefaultBudget(),
		PermissionMode: permissions.AutoApprove,
	}
	registry := tools.NewRegistry()

	t.Run("detects_error_difference", func(t *testing.T) {
		turns := []testharness.TurnScript{
			testharness.MakeTextTurn("ok", provider.StopReasonEndTurn),
		}

		// queryA succeeds, queryB always fails
		queryFail := func(ctx context.Context, sess *session.SessionState, prov provider.ModelProvider, reg *tools.ToolRegistry, orch *tools.ToolOrchestrator, cb query.EventCallback) error {
			return &query.AgentError{Kind: query.ErrAborted}
		}

		diff := testharness.RunDifferential(
			"error_diff", cfg, os.TempDir(), "hello",
			turns, registry,
			query.Query, queryFail,
		)

		if diff.ErrorDiff == nil {
			t.Error("should detect error difference")
		}
		if !diff.HasDiffs() {
			t.Error("HasDiffs should be true")
		}
		if diff.Summary() == "" {
			t.Error("Summary should not be empty")
		}
	})

	t.Run("no_diffs_when_identical", func(t *testing.T) {
		turns := []testharness.TurnScript{
			testharness.MakeTextTurn("ok", provider.StopReasonEndTurn),
		}

		diff := testharness.RunDifferential(
			"identical", cfg, os.TempDir(), "hello",
			turns, registry,
			query.Query, query.Query,
		)

		if diff.HasDiffs() {
			t.Errorf("should not have diffs: %s", diff.Summary())
		}
	})

	t.Run("summary_format", func(t *testing.T) {
		turns := []testharness.TurnScript{
			testharness.MakeTextTurn("ok", provider.StopReasonEndTurn),
		}
		diff := testharness.RunDifferential(
			"test_summary", cfg, os.TempDir(), "hello",
			turns, registry,
			query.Query, query.Query,
		)
		summary := diff.Summary()
		if summary == "" {
			t.Error("summary should not be empty")
		}
		if diff.HasDiffs() {
			t.Logf("unexpected diffs: %s", summary)
		}
	})
}

// TestRecorderInfrastructure validates the record/replay infrastructure.
func TestRecorderInfrastructure(t *testing.T) {
	t.Run("scripted_to_replay_roundtrip", func(t *testing.T) {
		// Create a scripted session
		sp := testharness.NewScriptedProvider(
			testharness.MakeTextTurn("hello", provider.StopReasonEndTurn),
		)

		// Wrap with recorder
		recorder := testharness.NewRecordingProvider(sp)

		// Run through the provider
		cfg := session.SessionConfig{
			Model: "test", MaxTurns: 100,
			TokenBudget:    compact.DefaultBudget(),
			PermissionMode: permissions.AutoApprove,
		}
		registry := tools.NewRegistry()
		_ = tools.NewOrchestrator(registry)
		sess := session.New(cfg, os.TempDir())
		sess.PushMessage(message.UserMessage("hi"))

		// Just call Stream directly to record
		ch, err := recorder.Stream(context.Background(), provider.ModelRequest{
			Model:     "test",
			Messages:  sess.ToRequestMessages(),
			MaxTokens: 16000,
		})
		if err != nil {
			t.Fatalf("stream failed: %v", err)
		}
		for range ch {
			// drain
		}

		// Get recorded session
		recorded := recorder.Session("test")
		t.Run("has_turns", func(t *testing.T) {
			if len(recorded.Turns) == 0 {
				t.Error("should have recorded turns")
			}
		})
		t.Run("has_version", func(t *testing.T) {
			if recorded.Version != "1.0" {
				t.Errorf("expected version 1.0, got %s", recorded.Version)
			}
		})
		t.Run("has_source", func(t *testing.T) {
			if recorded.Source != "test" {
				t.Errorf("expected source 'test', got %s", recorded.Source)
			}
		})

		// Save and reload
		tmpFile := t.TempDir() + "/recorded.json"
		if err := recorder.SaveSession(tmpFile, "test"); err != nil {
			t.Fatalf("save failed: %v", err)
		}

		replay, err := testharness.LoadReplayProvider(tmpFile)
		if err != nil {
			t.Fatalf("load failed: %v", err)
		}
		t.Run("replay_provider_created", func(t *testing.T) {
			if replay == nil {
				t.Fatal("replay provider is nil")
			}
		})
		t.Run("replay_name", func(t *testing.T) {
			if replay.Name() != "replay:test" {
				t.Errorf("expected replay:test, got %s", replay.Name())
			}
		})
	})
}
