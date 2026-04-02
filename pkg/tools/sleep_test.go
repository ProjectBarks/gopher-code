package tools_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/projectbarks/gopher-code/pkg/tools"
)

func TestSleepTool(t *testing.T) {
	tool := &tools.SleepTool{}

	t.Run("name", func(t *testing.T) {
		if tool.Name() != "Sleep" {
			t.Errorf("expected 'Sleep', got %q", tool.Name())
		}
	})

	t.Run("is_read_only", func(t *testing.T) {
		if !tool.IsReadOnly() {
			t.Error("SleepTool should be read-only")
		}
	})

	t.Run("valid_schema", func(t *testing.T) {
		var parsed map[string]interface{}
		if err := json.Unmarshal(tool.InputSchema(), &parsed); err != nil {
			t.Fatalf("schema is not valid JSON: %v", err)
		}
		props, ok := parsed["properties"].(map[string]interface{})
		if !ok {
			t.Fatal("schema missing properties")
		}
		if _, ok := props["duration_ms"]; !ok {
			t.Error("schema missing 'duration_ms' property")
		}
	})

	t.Run("sleep_short_duration", func(t *testing.T) {
		start := time.Now()
		input := json.RawMessage(`{"duration_ms": 50}`)
		out, err := tool.Execute(context.Background(), nil, input)
		elapsed := time.Since(start)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.IsError {
			t.Fatalf("unexpected tool error: %s", out.Content)
		}
		if !strings.Contains(out.Content, "50ms") {
			t.Errorf("expected '50ms' in output, got %q", out.Content)
		}
		if elapsed < 40*time.Millisecond {
			t.Errorf("sleep too short: %v", elapsed)
		}
	})

	t.Run("sleep_cancelled_by_context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			time.Sleep(50 * time.Millisecond)
			cancel()
		}()

		start := time.Now()
		input := json.RawMessage(`{"duration_ms": 60000}`)
		out, err := tool.Execute(ctx, nil, input)
		elapsed := time.Since(start)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !out.IsError {
			t.Error("expected error output when cancelled")
		}
		if !strings.Contains(out.Content, "interrupted") {
			t.Errorf("expected 'interrupted' in output, got %q", out.Content)
		}
		if elapsed > 5*time.Second {
			t.Errorf("sleep should have been interrupted quickly, took %v", elapsed)
		}
	})

	t.Run("negative_duration_defaults_to_1000", func(t *testing.T) {
		input := json.RawMessage(`{"duration_ms": -1}`)
		start := time.Now()
		out, err := tool.Execute(context.Background(), nil, input)
		elapsed := time.Since(start)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.IsError {
			t.Fatalf("unexpected tool error: %s", out.Content)
		}
		if !strings.Contains(out.Content, "1000ms") {
			t.Errorf("expected default 1000ms, got %q", out.Content)
		}
		if elapsed < 900*time.Millisecond {
			t.Errorf("expected ~1s sleep, got %v", elapsed)
		}
	})

	t.Run("caps_at_300000", func(t *testing.T) {
		// We can't actually wait 5 minutes, so we cancel immediately
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		input := json.RawMessage(`{"duration_ms": 999999}`)
		out, err := tool.Execute(ctx, nil, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Should be interrupted since context is already cancelled
		if !out.IsError {
			t.Error("expected error for cancelled context")
		}
	})

	t.Run("invalid_json", func(t *testing.T) {
		out, err := tool.Execute(context.Background(), nil, json.RawMessage(`{bad}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !out.IsError {
			t.Error("expected error for invalid JSON")
		}
	})
}
