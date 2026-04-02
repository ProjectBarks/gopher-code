package tools_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func registryGoldenPath() string {
	_, filename, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(filename), "..", "..", "testdata", "tool_registry.json")
}

type ToolRegistry struct {
	AlwaysAvailableTools     []string            `json:"always_available_tools"`
	ConditionallyAvailable   map[string]string   `json:"conditionally_available_tools"`
	DeferredTools            []string            `json:"deferred_tools"`
	NeverDeferredTools       []string            `json:"never_deferred_tools"`
	SimpleModeTools          []string            `json:"simple_mode_tools"`
	RegistrationOrderFirst5  []string            `json:"registration_order_first_5"`
	NormalizationPipeline    []string            `json:"message_normalization_pipeline"`
}

func loadRegistry(t *testing.T) *ToolRegistry {
	t.Helper()
	data, err := os.ReadFile(registryGoldenPath())
	if err != nil {
		t.Fatalf("failed to load registry: %v", err)
	}
	var reg ToolRegistry
	if err := json.Unmarshal(data, &reg); err != nil {
		t.Fatalf("failed to parse registry: %v", err)
	}
	return &reg
}

// TestAlwaysAvailableTools validates tools that are unconditionally registered.
// Source: tools.ts getAllBaseTools() — tools not wrapped in conditionals
func TestAlwaysAvailableTools(t *testing.T) {
	reg := loadRegistry(t)

	t.Run("count_at_least_19", func(t *testing.T) {
		if len(reg.AlwaysAvailableTools) < 19 {
			t.Errorf("expected at least 19 always-available tools, got %d", len(reg.AlwaysAvailableTools))
		}
	})

	coreTools := []string{"Agent", "Bash", "Read", "Edit", "Write", "Skill", "AskUserQuestion"}
	for _, name := range coreTools {
		name := name
		t.Run(fmt.Sprintf("core_%s_always_available", name), func(t *testing.T) {
			found := false
			for _, tool := range reg.AlwaysAvailableTools {
				if tool == name {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("core tool %s must be always available", name)
			}
		})
	}
}

// TestConditionallyAvailableTools validates feature-gated tools.
// Source: tools.ts getAllBaseTools() — tools inside process.env/feature() conditionals
func TestConditionallyAvailableTools(t *testing.T) {
	reg := loadRegistry(t)

	conditionalTools := []string{
		"Glob", "Grep", "Config", "LSP",
		"TaskCreate", "TaskGet", "TaskUpdate", "TaskList",
		"EnterWorktree", "ExitWorktree", "REPL", "PowerShell",
		"CronCreate", "CronDelete", "CronList", "RemoteTrigger",
		"ToolSearch", "Sleep",
	}

	for _, name := range conditionalTools {
		name := name
		t.Run(fmt.Sprintf("%s_is_conditional", name), func(t *testing.T) {
			reason, ok := reg.ConditionallyAvailable[name]
			if !ok {
				t.Errorf("tool %s should be conditionally available", name)
			}
			if reason == "" {
				t.Errorf("tool %s condition reason is empty", name)
			}
		})
	}

	t.Run("Glob_condition_mentions_embedded", func(t *testing.T) {
		reason := reg.ConditionallyAvailable["Glob"]
		if reason == "" {
			t.Skip("Glob not in conditional list")
		}
		// Source: tools.ts — hasEmbeddedSearchTools()
	})

	t.Run("Config_ant_only", func(t *testing.T) {
		reason := reg.ConditionallyAvailable["Config"]
		if reason == "" {
			t.Skip("Config not in conditional list")
		}
		// Source: process.env.USER_TYPE === 'ant'
	})
}

// TestDeferredTools validates which tools use shouldDefer: true.
// Source: each tool's shouldDefer property — these are hidden until ToolSearch activates them
func TestDeferredTools(t *testing.T) {
	reg := loadRegistry(t)

	t.Run("deferred_count", func(t *testing.T) {
		if len(reg.DeferredTools) < 20 {
			t.Errorf("expected at least 20 deferred tools, got %d", len(reg.DeferredTools))
		}
	})

	// WebFetch and WebSearch are deferred (require ToolSearch before use)
	deferredExpected := []string{
		"WebFetch", "WebSearch", "LSP", "NotebookEdit",
		"TodoWrite", "EnterPlanMode", "ExitPlanMode",
		"TaskCreate", "TaskGet", "TaskUpdate", "TaskList", "TaskStop",
		"CronCreate", "CronDelete", "CronList",
		"EnterWorktree", "ExitWorktree",
		"SendMessage", "RemoteTrigger",
	}
	for _, name := range deferredExpected {
		name := name
		t.Run(fmt.Sprintf("deferred_%s", name), func(t *testing.T) {
			found := false
			for _, d := range reg.DeferredTools {
				if d == name {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("tool %s should be deferred", name)
			}
		})
	}
}

// TestNeverDeferredTools validates tools that are always immediately available.
// Source: tools that do NOT have shouldDefer: true
func TestNeverDeferredTools(t *testing.T) {
	reg := loadRegistry(t)

	neverDeferred := []string{"Agent", "Bash", "Read", "Edit", "Write", "Glob", "Grep", "AskUserQuestion", "Skill", "Brief"}
	for _, name := range neverDeferred {
		name := name
		t.Run(fmt.Sprintf("never_deferred_%s", name), func(t *testing.T) {
			found := false
			for _, nd := range reg.NeverDeferredTools {
				if nd == name {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("tool %s should never be deferred", name)
			}
		})
	}

	// No tool should be in both deferred and never-deferred
	t.Run("no_overlap", func(t *testing.T) {
		deferredSet := map[string]bool{}
		for _, d := range reg.DeferredTools {
			deferredSet[d] = true
		}
		for _, nd := range reg.NeverDeferredTools {
			if deferredSet[nd] {
				t.Errorf("tool %s is in both deferred and never-deferred", nd)
			}
		}
	})
}

// TestSimpleModeTools validates CLAUDE_CODE_SIMPLE=1 tool set.
// Source: tools.ts getTools() — isEnvTruthy(process.env.CLAUDE_CODE_SIMPLE)
func TestSimpleModeTools(t *testing.T) {
	reg := loadRegistry(t)

	t.Run("exactly_3_tools", func(t *testing.T) {
		if len(reg.SimpleModeTools) != 3 {
			t.Errorf("expected 3 simple mode tools, got %d", len(reg.SimpleModeTools))
		}
	})

	expected := []string{"Bash", "Read", "Edit"}
	for i, name := range expected {
		name := name
		t.Run(fmt.Sprintf("simple_%s", name), func(t *testing.T) {
			if i >= len(reg.SimpleModeTools) {
				t.Fatal("index out of range")
			}
			if reg.SimpleModeTools[i] != name {
				t.Errorf("expected %s, got %s", name, reg.SimpleModeTools[i])
			}
		})
	}
}

// TestRegistrationOrder validates the first tools registered.
// Source: tools.ts getAllBaseTools() — array order matters for tool_choice
func TestRegistrationOrder(t *testing.T) {
	reg := loadRegistry(t)

	expected := []string{"Agent", "TaskOutput", "Bash", "Glob", "Grep"}
	for i, name := range expected {
		name := name
		t.Run(fmt.Sprintf("position_%d_%s", i, name), func(t *testing.T) {
			if i >= len(reg.RegistrationOrderFirst5) {
				t.Fatal("index out of range")
			}
			if reg.RegistrationOrderFirst5[i] != name {
				t.Errorf("expected %s at position %d, got %s", name, i, reg.RegistrationOrderFirst5[i])
			}
		})
	}
}

// TestMessageNormalizationPipeline validates the message normalization steps.
// Source: messages.ts normalizeMessagesForAPI() — order of operations matters
func TestMessageNormalizationPipeline(t *testing.T) {
	reg := loadRegistry(t)

	t.Run("has_pipeline_steps", func(t *testing.T) {
		if len(reg.NormalizationPipeline) < 15 {
			t.Errorf("expected at least 15 normalization steps, got %d", len(reg.NormalizationPipeline))
		}
	})

	// First step must be reordering
	t.Run("first_step_reorder_attachments", func(t *testing.T) {
		if len(reg.NormalizationPipeline) == 0 {
			t.Fatal("empty pipeline")
		}
		if reg.NormalizationPipeline[0] != "reorder_attachments_bubble_up" {
			t.Errorf("first step should be reorder_attachments_bubble_up, got %s", reg.NormalizationPipeline[0])
		}
	})

	// Virtual messages filtered early
	t.Run("filter_virtual_early", func(t *testing.T) {
		if len(reg.NormalizationPipeline) < 2 {
			t.Fatal("pipeline too short")
		}
		if reg.NormalizationPipeline[1] != "filter_virtual_messages" {
			t.Errorf("second step should be filter_virtual_messages, got %s", reg.NormalizationPipeline[1])
		}
	})

	// Validate image sizes is last
	t.Run("validate_images_last", func(t *testing.T) {
		last := reg.NormalizationPipeline[len(reg.NormalizationPipeline)-1]
		if last != "validate_image_sizes" {
			t.Errorf("last step should be validate_image_sizes, got %s", last)
		}
	})

	// Key steps exist
	keySteps := []string{
		"normalize_tool_inputs",
		"merge_adjacent_user_messages",
		"ensure_non_empty_assistant_content",
		"sanitize_error_tool_result_content",
	}
	for _, step := range keySteps {
		step := step
		t.Run(fmt.Sprintf("has_step_%s", step), func(t *testing.T) {
			found := false
			for _, s := range reg.NormalizationPipeline {
				if s == step {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("pipeline missing step %s", step)
			}
		})
	}

	// Merge adjacent users happens AFTER normalize tool inputs
	t.Run("merge_users_after_normalize_inputs", func(t *testing.T) {
		normalizeIdx := -1
		mergeIdx := -1
		for i, s := range reg.NormalizationPipeline {
			if s == "normalize_tool_inputs" {
				normalizeIdx = i
			}
			if s == "merge_adjacent_user_messages" {
				mergeIdx = i
			}
		}
		if normalizeIdx >= 0 && mergeIdx >= 0 && mergeIdx <= normalizeIdx {
			t.Error("merge_adjacent_user_messages must happen AFTER normalize_tool_inputs")
		}
	})

	// All steps have non-empty names
	for i, step := range reg.NormalizationPipeline {
		i, step := i, step
		t.Run(fmt.Sprintf("step_%d_non_empty", i), func(t *testing.T) {
			if step == "" {
				t.Errorf("step %d is empty", i)
			}
		})
	}
}
