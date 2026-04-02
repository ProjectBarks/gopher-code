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

	t.Run("not_read_only", func(t *testing.T) {
		if create.IsReadOnly() {
			t.Error("TeamCreate should not be read-only")
		}
	})

	t.Run("schema_valid", func(t *testing.T) {
		var parsed map[string]interface{}
		if err := json.Unmarshal(create.InputSchema(), &parsed); err != nil {
			t.Fatalf("invalid schema JSON: %v", err)
		}
		props, ok := parsed["properties"].(map[string]interface{})
		if !ok {
			t.Fatal("schema missing properties")
		}
		if _, ok := props["name"]; !ok {
			t.Error("schema missing 'name' property")
		}
		if _, ok := props["members"]; !ok {
			t.Error("schema missing 'members' property")
		}
	})

	t.Run("create_team", func(t *testing.T) {
		ts2 := tools.NewTeamTools()
		c := findTeamTool(ts2, "TeamCreate")
		input := json.RawMessage(`{"name": "alpha", "members": ["researcher", "coder"]}`)
		out, err := c.Execute(context.Background(), nil, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.IsError {
			t.Fatalf("unexpected tool error: %s", out.Content)
		}
		if !strings.Contains(out.Content, "alpha") {
			t.Errorf("expected team name in output, got %q", out.Content)
		}
		if !strings.Contains(out.Content, "2 member(s)") {
			t.Errorf("expected member count in output, got %q", out.Content)
		}
	})

	t.Run("create_team_no_members", func(t *testing.T) {
		ts2 := tools.NewTeamTools()
		c := findTeamTool(ts2, "TeamCreate")
		input := json.RawMessage(`{"name": "solo"}`)
		out, err := c.Execute(context.Background(), nil, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.IsError {
			t.Fatalf("unexpected tool error: %s", out.Content)
		}
		if !strings.Contains(out.Content, "0 members") {
			t.Errorf("expected '0 members' in output, got %q", out.Content)
		}
	})

	t.Run("create_duplicate_name", func(t *testing.T) {
		ts2 := tools.NewTeamTools()
		c := findTeamTool(ts2, "TeamCreate")
		c.Execute(context.Background(), nil, json.RawMessage(`{"name": "dup"}`))
		out, err := c.Execute(context.Background(), nil, json.RawMessage(`{"name": "dup"}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !out.IsError {
			t.Error("expected error for duplicate team name")
		}
		if !strings.Contains(out.Content, "already exists") {
			t.Errorf("expected 'already exists' in error, got %q", out.Content)
		}
	})

	t.Run("missing_name", func(t *testing.T) {
		out, err := create.Execute(context.Background(), nil, json.RawMessage(`{"name": ""}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !out.IsError {
			t.Error("expected error for missing name")
		}
	})

	t.Run("invalid_json", func(t *testing.T) {
		out, err := create.Execute(context.Background(), nil, json.RawMessage(`{bad}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !out.IsError {
			t.Error("expected error for invalid JSON")
		}
	})
}

func TestTeamDeleteTool(t *testing.T) {
	ts := tools.NewTeamTools()
	create := findTeamTool(ts, "TeamCreate")
	del := findTeamTool(ts, "TeamDelete")
	if del == nil {
		t.Fatal("TeamDelete not found")
	}

	t.Run("name", func(t *testing.T) {
		if del.Name() != "TeamDelete" {
			t.Errorf("expected TeamDelete, got %q", del.Name())
		}
	})

	t.Run("not_read_only", func(t *testing.T) {
		if del.IsReadOnly() {
			t.Error("TeamDelete should not be read-only")
		}
	})

	t.Run("schema_valid", func(t *testing.T) {
		var parsed map[string]interface{}
		if err := json.Unmarshal(del.InputSchema(), &parsed); err != nil {
			t.Fatalf("invalid schema JSON: %v", err)
		}
	})

	t.Run("delete_existing_team", func(t *testing.T) {
		create.Execute(context.Background(), nil, json.RawMessage(`{"name": "to-delete"}`))
		out, err := del.Execute(context.Background(), nil, json.RawMessage(`{"name": "to-delete"}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.IsError {
			t.Fatalf("unexpected tool error: %s", out.Content)
		}
		if !strings.Contains(out.Content, "deleted") {
			t.Errorf("expected 'deleted' in output, got %q", out.Content)
		}
	})

	t.Run("delete_nonexistent_team", func(t *testing.T) {
		out, err := del.Execute(context.Background(), nil, json.RawMessage(`{"name": "nonexistent"}`))
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

	t.Run("missing_name", func(t *testing.T) {
		out, err := del.Execute(context.Background(), nil, json.RawMessage(`{"name": ""}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !out.IsError {
			t.Error("expected error for missing name")
		}
	})
}

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

func TestTeamToolsConcurrentAccess(t *testing.T) {
	ts := tools.NewTeamTools()
	create := findTeamTool(ts, "TeamCreate")
	del := findTeamTool(ts, "TeamDelete")

	var wg sync.WaitGroup
	n := 50

	// Concurrently create teams
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			input, _ := json.Marshal(map[string]interface{}{
				"name":    strings.Replace("team-N", "N", string(rune('A'+i%26))+strings.Repeat("x", i), 1),
				"members": []string{"agent"},
			})
			create.Execute(context.Background(), nil, input)
		}(i)
	}
	wg.Wait()

	// Concurrently delete some teams (some may not exist, that's fine)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			input, _ := json.Marshal(map[string]string{
				"name": strings.Replace("team-N", "N", string(rune('A'+i%26))+strings.Repeat("x", i), 1),
			})
			del.Execute(context.Background(), nil, input)
		}(i)
	}
	wg.Wait()
}

func TestTeamToolsIndependentStores(t *testing.T) {
	ts1 := tools.NewTeamTools()
	ts2 := tools.NewTeamTools()

	create1 := findTeamTool(ts1, "TeamCreate")
	del2 := findTeamTool(ts2, "TeamDelete")

	create1.Execute(context.Background(), nil, json.RawMessage(`{"name": "store1-team"}`))

	// Deleting from store2 should fail since it's an independent store
	out, err := del2.Execute(context.Background(), nil, json.RawMessage(`{"name": "store1-team"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !out.IsError {
		t.Error("expected error: team should not exist in store 2")
	}
}
