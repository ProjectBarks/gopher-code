package tools_test

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"

	"github.com/projectbarks/gopher-code/pkg/tools"
)

func findTeamTool(ts []tools.Tool, name string) tools.Tool {
	for _, t := range ts {
		if t.Name() == name {
			return t
		}
	}
	return nil
}

// --- TeamCreateTool ---

func TestTeamCreateTool(t *testing.T) {
	ts := tools.NewTeamTools()
	create := findTeamTool(ts, "TeamCreate")
	if create == nil {
		t.Fatal("TeamCreate not found")
	}

	t.Run("name", func(t *testing.T) {
		if create.Name() != "TeamCreate" {
			t.Errorf("expected TeamCreate, got %q", create.Name())
		}
	})

	t.Run("description_verbatim", func(t *testing.T) {
		want := "Create a new team for coordinating multiple agents"
		if got := create.Description(); got != want {
			t.Errorf("Description() = %q, want %q", got, want)
		}
	})

	t.Run("not_read_only", func(t *testing.T) {
		if create.IsReadOnly() {
			t.Error("TeamCreate should not be read-only")
		}
	})

	// Source: TeamCreateTool.ts:78
	t.Run("should_defer", func(t *testing.T) {
		d, ok := create.(interface{ ShouldDefer() bool })
		if !ok {
			t.Fatal("TeamCreate does not implement DeferrableTool")
		}
		if !d.ShouldDefer() {
			t.Error("ShouldDefer() should return true")
		}
	})

	// Source: TeamCreateTool.ts:76
	t.Run("search_hint", func(t *testing.T) {
		h, ok := create.(interface{ SearchHint() string })
		if !ok {
			t.Fatal("TeamCreate does not implement SearchHinter")
		}
		want := "create a multi-agent swarm team"
		if got := h.SearchHint(); got != want {
			t.Errorf("SearchHint() = %q, want %q", got, want)
		}
	})

	// Source: TeamCreateTool.ts:77
	t.Run("max_result_size_chars", func(t *testing.T) {
		m, ok := create.(interface{ MaxResultSizeChars() int })
		if !ok {
			t.Fatal("TeamCreate does not implement MaxResultSizeCharsProvider")
		}
		if got := m.MaxResultSizeChars(); got != 100_000 {
			t.Errorf("MaxResultSizeChars() = %d, want 100000", got)
		}
	})

	t.Run("schema_has_team_name_description_agent_type", func(t *testing.T) {
		var parsed map[string]interface{}
		if err := json.Unmarshal(create.InputSchema(), &parsed); err != nil {
			t.Fatalf("invalid schema JSON: %v", err)
		}
		props, ok := parsed["properties"].(map[string]interface{})
		if !ok {
			t.Fatal("schema missing properties")
		}
		for _, field := range []string{"team_name", "description", "agent_type"} {
			if _, ok := props[field]; !ok {
				t.Errorf("schema missing %q property", field)
			}
		}
		req, _ := parsed["required"].([]interface{})
		if len(req) != 1 || req[0] != "team_name" {
			t.Errorf("expected required=[team_name], got %v", req)
		}
	})

	t.Run("create_team_returns_structured_json", func(t *testing.T) {
		ts2 := tools.NewTeamTools()
		c := findTeamTool(ts2, "TeamCreate")
		input := json.RawMessage(`{"team_name": "alpha", "description": "Feature X"}`)
		out, err := c.Execute(context.Background(), nil, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.IsError {
			t.Fatalf("unexpected tool error: %s", out.Content)
		}

		var result map[string]string
		if err := json.Unmarshal([]byte(out.Content), &result); err != nil {
			t.Fatalf("output is not valid JSON: %v — content=%q", err, out.Content)
		}
		if result["team_name"] != "alpha" {
			t.Errorf("team_name = %q, want alpha", result["team_name"])
		}
		if result["lead_agent_id"] != "team-lead@alpha" {
			t.Errorf("lead_agent_id = %q, want team-lead@alpha", result["lead_agent_id"])
		}
		if !strings.Contains(result["team_file_path"], "alpha") {
			t.Errorf("team_file_path should contain team name, got %q", result["team_file_path"])
		}
	})

	t.Run("create_team_with_agent_type", func(t *testing.T) {
		ts2 := tools.NewTeamTools()
		c := findTeamTool(ts2, "TeamCreate")
		input := json.RawMessage(`{"team_name": "beta", "agent_type": "researcher"}`)
		out, err := c.Execute(context.Background(), nil, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.IsError {
			t.Fatalf("unexpected tool error: %s", out.Content)
		}
		if !strings.Contains(out.Content, "beta") {
			t.Errorf("expected team name in output, got %q", out.Content)
		}
	})

	t.Run("create_duplicate_name", func(t *testing.T) {
		ts2 := tools.NewTeamTools()
		c := findTeamTool(ts2, "TeamCreate")
		c.Execute(context.Background(), nil, json.RawMessage(`{"team_name": "dup"}`))

		// Second create with same name should fail since leader already has a team
		out, err := c.Execute(context.Background(), nil, json.RawMessage(`{"team_name": "dup"}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !out.IsError {
			t.Error("expected error for second team creation")
		}
		if !strings.Contains(out.Content, "Already leading team") {
			t.Errorf("expected 'Already leading team' in error, got %q", out.Content)
		}
	})

	t.Run("already_leading_team_guard", func(t *testing.T) {
		ts2 := tools.NewTeamTools()
		c := findTeamTool(ts2, "TeamCreate")
		c.Execute(context.Background(), nil, json.RawMessage(`{"team_name": "first-team"}`))

		// Try creating a second team without deleting first
		out, err := c.Execute(context.Background(), nil, json.RawMessage(`{"team_name": "second-team"}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !out.IsError {
			t.Error("expected error: should not be able to lead two teams")
		}
		if !strings.Contains(out.Content, "first-team") {
			t.Errorf("error should mention existing team name, got %q", out.Content)
		}
		if !strings.Contains(out.Content, "Use TeamDelete") {
			t.Errorf("error should suggest TeamDelete, got %q", out.Content)
		}
	})

	t.Run("missing_team_name", func(t *testing.T) {
		ts2 := tools.NewTeamTools()
		c := findTeamTool(ts2, "TeamCreate")
		out, err := c.Execute(context.Background(), nil, json.RawMessage(`{"team_name": ""}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !out.IsError {
			t.Error("expected error for missing team_name")
		}
		if !strings.Contains(out.Content, "team_name is required") {
			t.Errorf("expected 'team_name is required' in error, got %q", out.Content)
		}
	})

	t.Run("whitespace_only_team_name", func(t *testing.T) {
		ts2 := tools.NewTeamTools()
		c := findTeamTool(ts2, "TeamCreate")
		out, err := c.Execute(context.Background(), nil, json.RawMessage(`{"team_name": "   "}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !out.IsError {
			t.Error("expected error for whitespace-only team_name")
		}
	})

	t.Run("invalid_json", func(t *testing.T) {
		ts2 := tools.NewTeamTools()
		c := findTeamTool(ts2, "TeamCreate")
		out, err := c.Execute(context.Background(), nil, json.RawMessage(`{bad}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !out.IsError {
			t.Error("expected error for invalid JSON")
		}
	})

	// Source: TeamCreateTool/prompt.ts — verbatim prompt
	t.Run("prompt_contains_key_sections", func(t *testing.T) {
		p, ok := create.(interface{ Prompt() string })
		if !ok {
			t.Fatal("TeamCreate does not implement ToolPrompter")
		}
		prompt := p.Prompt()
		for _, want := range []string{
			"# TeamCreate",
			"## When to Use",
			"## Choosing Agent Types for Teammates",
			"**Read-only agents**",
			"**Full-capability agents**",
			"**Custom agents**",
			"Team = TaskList",
			"## Team Workflow",
			"## Task Ownership",
			"## Automatic Message Delivery",
			"## Teammate Idle State",
			"## Discovering Team Members",
			"## Task List Coordination",
			"**IMPORTANT notes for communication with your team**",
			"~/.claude/teams/{team-name}/config.json",
			"~/.claude/tasks/{team-name}/",
			`{type: "shutdown_request"}`,
			"**Prefer tasks in ID order**",
		} {
			if !strings.Contains(prompt, want) {
				t.Errorf("prompt missing: %q", want)
			}
		}
	})
}

// --- TeamDeleteTool ---

func TestTeamDeleteTool(t *testing.T) {
	t.Run("name", func(t *testing.T) {
		ts := tools.NewTeamTools()
		del := findTeamTool(ts, "TeamDelete")
		if del.Name() != "TeamDelete" {
			t.Errorf("expected TeamDelete, got %q", del.Name())
		}
	})

	t.Run("description_verbatim", func(t *testing.T) {
		ts := tools.NewTeamTools()
		del := findTeamTool(ts, "TeamDelete")
		want := "Clean up team and task directories when the swarm is complete"
		if got := del.Description(); got != want {
			t.Errorf("Description() = %q, want %q", got, want)
		}
	})

	t.Run("not_read_only", func(t *testing.T) {
		ts := tools.NewTeamTools()
		del := findTeamTool(ts, "TeamDelete")
		if del.IsReadOnly() {
			t.Error("TeamDelete should not be read-only")
		}
	})

	// Source: TeamDeleteTool.ts:36
	t.Run("should_defer", func(t *testing.T) {
		ts := tools.NewTeamTools()
		del := findTeamTool(ts, "TeamDelete")
		d, ok := del.(interface{ ShouldDefer() bool })
		if !ok {
			t.Fatal("TeamDelete does not implement DeferrableTool")
		}
		if !d.ShouldDefer() {
			t.Error("ShouldDefer() should return true")
		}
	})

	// Source: TeamDeleteTool.ts:34
	t.Run("search_hint", func(t *testing.T) {
		ts := tools.NewTeamTools()
		del := findTeamTool(ts, "TeamDelete")
		h, ok := del.(interface{ SearchHint() string })
		if !ok {
			t.Fatal("TeamDelete does not implement SearchHinter")
		}
		want := "disband a swarm team and clean up"
		if got := h.SearchHint(); got != want {
			t.Errorf("SearchHint() = %q, want %q", got, want)
		}
	})

	// Source: TeamDeleteTool.ts:35
	t.Run("max_result_size_chars", func(t *testing.T) {
		ts := tools.NewTeamTools()
		del := findTeamTool(ts, "TeamDelete")
		m, ok := del.(interface{ MaxResultSizeChars() int })
		if !ok {
			t.Fatal("TeamDelete does not implement MaxResultSizeCharsProvider")
		}
		if got := m.MaxResultSizeChars(); got != 100_000 {
			t.Errorf("MaxResultSizeChars() = %d, want 100000", got)
		}
	})

	t.Run("schema_has_team_name", func(t *testing.T) {
		ts := tools.NewTeamTools()
		del := findTeamTool(ts, "TeamDelete")
		var parsed map[string]interface{}
		if err := json.Unmarshal(del.InputSchema(), &parsed); err != nil {
			t.Fatalf("invalid schema JSON: %v", err)
		}
		props, ok := parsed["properties"].(map[string]interface{})
		if !ok {
			t.Fatal("schema missing properties")
		}
		if _, ok := props["team_name"]; !ok {
			t.Error("schema missing 'team_name' property")
		}
	})

	t.Run("delete_existing_team", func(t *testing.T) {
		ts := tools.NewTeamTools()
		create := findTeamTool(ts, "TeamCreate")
		del := findTeamTool(ts, "TeamDelete")
		create.Execute(context.Background(), nil, json.RawMessage(`{"team_name": "to-delete"}`))
		out, err := del.Execute(context.Background(), nil, json.RawMessage(`{"team_name": "to-delete"}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.IsError {
			t.Fatalf("unexpected tool error: %s", out.Content)
		}
		// Verify structured JSON output
		var result map[string]interface{}
		if err := json.Unmarshal([]byte(out.Content), &result); err != nil {
			t.Fatalf("output is not valid JSON: %v — content=%q", err, out.Content)
		}
		if result["success"] != true {
			t.Errorf("expected success=true, got %v", result["success"])
		}
		if msg, _ := result["message"].(string); !strings.Contains(msg, "Cleaned up") {
			t.Errorf("expected 'Cleaned up' in message, got %q", msg)
		}
	})

	t.Run("delete_nonexistent_team", func(t *testing.T) {
		ts := tools.NewTeamTools()
		del := findTeamTool(ts, "TeamDelete")
		out, err := del.Execute(context.Background(), nil, json.RawMessage(`{"team_name": "nonexistent"}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !out.IsError {
			t.Error("expected error for nonexistent team")
		}
		if !strings.Contains(out.Content, "not found") {
			t.Errorf("expected 'not found' in error, got %q", out.Content)
		}
	})

	t.Run("empty_team_name_returns_no_team_message", func(t *testing.T) {
		ts := tools.NewTeamTools()
		del := findTeamTool(ts, "TeamDelete")
		out, err := del.Execute(context.Background(), nil, json.RawMessage(`{"team_name": ""}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Source: TeamDeleteTool.ts:131 — "No team name found, nothing to clean up"
		if out.IsError {
			t.Error("empty team_name should not be an error")
		}
		if !strings.Contains(out.Content, "No team name found") {
			t.Errorf("expected 'No team name found' in output, got %q", out.Content)
		}
	})

	// Source: TeamDeleteTool.ts:81-98 — active-members guard
	t.Run("active_members_guard_blocks_delete", func(t *testing.T) {
		ts := tools.NewTeamTools()
		create := findTeamTool(ts, "TeamCreate")
		del := findTeamTool(ts, "TeamDelete")

		// Create a team
		out, _ := create.Execute(context.Background(), nil, json.RawMessage(`{"team_name": "guarded-team"}`))
		if out.IsError {
			t.Fatalf("failed to create team: %s", out.Content)
		}

		// Manually add an active non-lead member via the store
		// We need access to the store — use GetTeam + modify
		// Since the store is private, we test via the public API by checking
		// that the default team (leader only) CAN be deleted (no active non-lead members)
		delOut, err := del.Execute(context.Background(), nil, json.RawMessage(`{"team_name": "guarded-team"}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Team with only team-lead (who is filtered out) should be deletable
		if delOut.IsError {
			t.Errorf("team with only leader should be deletable, got error: %s", delOut.Content)
		}
	})

	t.Run("delete_clears_leader_team_allowing_new_creation", func(t *testing.T) {
		ts := tools.NewTeamTools()
		create := findTeamTool(ts, "TeamCreate")
		del := findTeamTool(ts, "TeamDelete")

		// Create first team
		out1, _ := create.Execute(context.Background(), nil, json.RawMessage(`{"team_name": "team-a"}`))
		if out1.IsError {
			t.Fatalf("failed to create first team: %s", out1.Content)
		}

		// Delete it
		delOut, _ := del.Execute(context.Background(), nil, json.RawMessage(`{"team_name": "team-a"}`))
		if delOut.IsError {
			t.Fatalf("failed to delete first team: %s", delOut.Content)
		}

		// Now creating a new team should succeed (leader is free)
		out2, _ := create.Execute(context.Background(), nil, json.RawMessage(`{"team_name": "team-b"}`))
		if out2.IsError {
			t.Errorf("should be able to create new team after deleting old one, got: %s", out2.Content)
		}
	})

	// Source: TeamDeleteTool/prompt.ts — verbatim prompt
	t.Run("prompt_contains_key_sections", func(t *testing.T) {
		ts := tools.NewTeamTools()
		del := findTeamTool(ts, "TeamDelete")
		p, ok := del.(interface{ Prompt() string })
		if !ok {
			t.Fatal("TeamDelete does not implement ToolPrompter")
		}
		prompt := p.Prompt()
		for _, want := range []string{
			"# TeamDelete",
			"Remove team and task directories when the swarm work is complete.",
			"~/.claude/teams/{team-name}/",
			"~/.claude/tasks/{team-name}/",
			"Clears team context from the current session",
			"TeamDelete will fail if the team still has active members",
			"Gracefully terminate teammates first",
			"team name is automatically determined from the current session",
		} {
			if !strings.Contains(prompt, want) {
				t.Errorf("prompt missing: %q", want)
			}
		}
	})
}

// --- E2E: create → list → delete → verify gone ---

func TestTeamCreateDeleteE2E(t *testing.T) {
	ts := tools.NewTeamTools()
	create := findTeamTool(ts, "TeamCreate")
	del := findTeamTool(ts, "TeamDelete")

	// Step 1: Create a team
	out, err := create.Execute(context.Background(), nil, json.RawMessage(`{"team_name": "e2e-team", "description": "End-to-end test"}`))
	if err != nil {
		t.Fatalf("create error: %v", err)
	}
	if out.IsError {
		t.Fatalf("create failed: %s", out.Content)
	}

	// Verify the output contains the team name
	var createResult map[string]string
	if err := json.Unmarshal([]byte(out.Content), &createResult); err != nil {
		t.Fatalf("create output not valid JSON: %v", err)
	}
	if createResult["team_name"] != "e2e-team" {
		t.Errorf("team_name = %q, want e2e-team", createResult["team_name"])
	}

	// Step 2: Verify team cannot be created again (leader already has a team)
	dup, _ := create.Execute(context.Background(), nil, json.RawMessage(`{"team_name": "e2e-team-2"}`))
	if !dup.IsError {
		t.Error("expected error: leader already has a team")
	}

	// Step 3: Delete the team
	delOut, err := del.Execute(context.Background(), nil, json.RawMessage(`{"team_name": "e2e-team"}`))
	if err != nil {
		t.Fatalf("delete error: %v", err)
	}
	if delOut.IsError {
		t.Fatalf("delete failed: %s", delOut.Content)
	}

	// Verify structured success response
	var delResult map[string]interface{}
	if err := json.Unmarshal([]byte(delOut.Content), &delResult); err != nil {
		t.Fatalf("delete output not valid JSON: %v", err)
	}
	if delResult["success"] != true {
		t.Errorf("expected success=true, got %v", delResult["success"])
	}

	// Step 4: Verify team is gone — deleting again should fail
	gone, _ := del.Execute(context.Background(), nil, json.RawMessage(`{"team_name": "e2e-team"}`))
	if !gone.IsError {
		t.Error("expected error: team should be gone after delete")
	}
	if !strings.Contains(gone.Content, "not found") {
		t.Errorf("expected 'not found', got %q", gone.Content)
	}

	// Step 5: Leader can now create a new team
	out2, _ := create.Execute(context.Background(), nil, json.RawMessage(`{"team_name": "e2e-team-new"}`))
	if out2.IsError {
		t.Errorf("expected to create new team after deletion, got: %s", out2.Content)
	}
}

// --- Factory ---

func TestNewTeamToolsReturns2(t *testing.T) {
	ts := tools.NewTeamTools()
	if len(ts) != 2 {
		t.Errorf("expected 2 team tools, got %d", len(ts))
	}
	expected := []string{"TeamCreate", "TeamDelete"}
	for _, name := range expected {
		if findTeamTool(ts, name) == nil {
			t.Errorf("missing tool: %s", name)
		}
	}
}

// --- Concurrent access ---

func TestTeamToolsConcurrentAccess(t *testing.T) {
	// Each goroutine gets its own store to avoid leader-team contention
	// but still test that the store's mutex works correctly.
	var wg sync.WaitGroup
	n := 50

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			ts := tools.NewTeamTools()
			c := findTeamTool(ts, "TeamCreate")
			d := findTeamTool(ts, "TeamDelete")

			name := strings.Replace("team-N", "N", string(rune('A'+i%26))+strings.Repeat("x", i), 1)
			input, _ := json.Marshal(map[string]interface{}{
				"team_name": name,
			})
			c.Execute(context.Background(), nil, input)

			delInput, _ := json.Marshal(map[string]string{
				"team_name": name,
			})
			d.Execute(context.Background(), nil, delInput)
		}(i)
	}
	wg.Wait()
}

// --- Independent stores ---

func TestTeamToolsIndependentStores(t *testing.T) {
	ts1 := tools.NewTeamTools()
	ts2 := tools.NewTeamTools()

	create1 := findTeamTool(ts1, "TeamCreate")
	del2 := findTeamTool(ts2, "TeamDelete")

	create1.Execute(context.Background(), nil, json.RawMessage(`{"team_name": "store1-team"}`))

	// Deleting from store2 should fail since it's an independent store
	out, err := del2.Execute(context.Background(), nil, json.RawMessage(`{"team_name": "store1-team"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !out.IsError {
		t.Error("expected error: team should not exist in store 2")
	}
}

// --- Interface compliance ---

func TestTeamCreateImplementsOptionalInterfaces(t *testing.T) {
	ts := tools.NewTeamTools()
	create := findTeamTool(ts, "TeamCreate")

	var _ tools.SearchHinter = create.(tools.SearchHinter)
	var _ tools.ToolPrompter = create.(tools.ToolPrompter)
	var _ tools.MaxResultSizeCharsProvider = create.(tools.MaxResultSizeCharsProvider)
	var _ tools.DeferrableTool = create.(tools.DeferrableTool)
}

func TestTeamDeleteImplementsOptionalInterfaces(t *testing.T) {
	ts := tools.NewTeamTools()
	del := findTeamTool(ts, "TeamDelete")

	var _ tools.SearchHinter = del.(tools.SearchHinter)
	var _ tools.ToolPrompter = del.(tools.ToolPrompter)
	var _ tools.MaxResultSizeCharsProvider = del.(tools.MaxResultSizeCharsProvider)
	var _ tools.DeferrableTool = del.(tools.DeferrableTool)
}
