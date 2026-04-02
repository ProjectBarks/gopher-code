package session

import (
	"os"
	"path/filepath"
	"testing"
)

// Source: utils/swarm/teamHelpers.ts, utils/swarm/constants.ts

func TestTeamLeadName(t *testing.T) {
	// Source: utils/swarm/constants.ts:1
	if TeamLeadName != "team-lead" {
		t.Errorf("expected 'team-lead', got %q", TeamLeadName)
	}
}

func TestSpawnTeam(t *testing.T) {
	// Source: utils/swarm/teamHelpers.ts — spawnTeam operation

	// Override teams dir for testing
	origTeamsDir := getTeamsDir
	tmpDir := t.TempDir()
	getTeamsDir = func() string { return tmpDir }
	defer func() { getTeamsDir = origTeamsDir }()

	t.Run("creates_team_with_leader", func(t *testing.T) {
		result, err := SpawnTeam("my-team", "A test team", "agent-001")
		if err != nil {
			t.Fatalf("spawn failed: %v", err)
		}
		if result.TeamName != "my-team" {
			t.Errorf("name = %q", result.TeamName)
		}
		if result.LeadAgentID != "agent-001" {
			t.Errorf("lead = %q", result.LeadAgentID)
		}

		// Verify file exists
		if _, err := os.Stat(result.TeamFilePath); os.IsNotExist(err) {
			t.Error("team file should exist")
		}

		// Verify team file content
		tf, err := ReadTeamFile("my-team")
		if err != nil {
			t.Fatalf("read failed: %v", err)
		}
		if len(tf.Members) != 1 {
			t.Fatalf("expected 1 member, got %d", len(tf.Members))
		}
		if tf.Members[0].Name != TeamLeadName {
			t.Errorf("leader name = %q, want %q", tf.Members[0].Name, TeamLeadName)
		}
		if tf.Members[0].Role != "leader" {
			t.Errorf("leader role = %q", tf.Members[0].Role)
		}
	})

	t.Run("creates_inboxes_directory", func(t *testing.T) {
		SpawnTeam("inbox-team", "", "agent-002")
		inboxDir := filepath.Join(tmpDir, "inbox-team", "inboxes")
		if _, err := os.Stat(inboxDir); os.IsNotExist(err) {
			t.Error("inboxes directory should exist")
		}
	})

	t.Run("empty_name_errors", func(t *testing.T) {
		_, err := SpawnTeam("", "desc", "agent-003")
		if err == nil {
			t.Error("expected error for empty name")
		}
	})
}

func TestAddTeamMember(t *testing.T) {
	tmpDir := t.TempDir()
	origTeamsDir := getTeamsDir
	getTeamsDir = func() string { return tmpDir }
	defer func() { getTeamsDir = origTeamsDir }()

	SpawnTeam("add-team", "", "agent-001")
	err := AddTeamMember("add-team", TeamMember{
		AgentID: "agent-002",
		Name:    "researcher",
		Role:    "analyst",
		Color:   "blue",
	})
	if err != nil {
		t.Fatalf("add member failed: %v", err)
	}

	tf, _ := ReadTeamFile("add-team")
	if len(tf.Members) != 2 {
		t.Fatalf("expected 2 members, got %d", len(tf.Members))
	}
	if tf.Members[1].Name != "researcher" {
		t.Errorf("second member = %q", tf.Members[1].Name)
	}
}

func TestCleanupTeam(t *testing.T) {
	// Source: utils/swarm/teamHelpers.ts — cleanup operation
	tmpDir := t.TempDir()
	origTeamsDir := getTeamsDir
	getTeamsDir = func() string { return tmpDir }
	defer func() { getTeamsDir = origTeamsDir }()

	SpawnTeam("cleanup-team", "", "agent-001")
	err := CleanupTeam("cleanup-team")
	if err != nil {
		t.Fatalf("cleanup failed: %v", err)
	}

	teamDir := filepath.Join(tmpDir, "cleanup-team")
	if _, err := os.Stat(teamDir); !os.IsNotExist(err) {
		t.Error("team directory should be removed after cleanup")
	}
}
