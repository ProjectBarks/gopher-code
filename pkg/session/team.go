package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Source: utils/swarm/teamHelpers.ts, utils/swarm/constants.ts

// TeamLeadName is the default name for the team leader.
// Source: utils/swarm/constants.ts:1
const TeamLeadName = "team-lead"

// TeamMember describes a member of a team.
// Source: utils/swarm/teamHelpers.ts:72-80
type TeamMember struct {
	AgentID   string `json:"agentId"`
	Name      string `json:"name"`
	Color     string `json:"color,omitempty"`
	Role      string `json:"role,omitempty"`
	SessionID string `json:"sessionId,omitempty"`
}

// TeamFile is the team configuration stored on disk.
// Source: utils/swarm/teamHelpers.ts:64-80
type TeamFile struct {
	Name          string       `json:"name"`
	Description   string       `json:"description,omitempty"`
	CreatedAt     int64        `json:"createdAt"` // Unix timestamp ms
	LeadAgentID   string       `json:"leadAgentId"`
	LeadSessionID string       `json:"leadSessionId,omitempty"`
	Members       []TeamMember `json:"members"`
}

// SpawnTeamResult is returned after creating a team.
// Source: utils/swarm/teamHelpers.ts:45-49
type SpawnTeamResult struct {
	TeamName     string `json:"team_name"`
	TeamFilePath string `json:"team_file_path"`
	LeadAgentID  string `json:"lead_agent_id"`
}

// getTeamsDir returns the directory for team data. Overridable for testing.
var getTeamsDir = func() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".claude", "teams")
}

// GetTeamFilePath returns the path to a team's config file.
func GetTeamFilePath(teamName string) string {
	safe := sanitizePathComponent(teamName)
	return filepath.Join(getTeamsDir(), safe, "team.json")
}

// SpawnTeam creates a new team with the given configuration.
// Source: utils/swarm/teamHelpers.ts — spawnTeam operation
func SpawnTeam(name, description, leadAgentID string) (*SpawnTeamResult, error) {
	if name == "" {
		return nil, fmt.Errorf("team_name is required for spawnTeam")
	}

	teamDir := filepath.Join(getTeamsDir(), sanitizePathComponent(name))
	if err := os.MkdirAll(teamDir, 0755); err != nil {
		return nil, fmt.Errorf("create team dir: %w", err)
	}

	// Create inboxes directory
	if err := os.MkdirAll(filepath.Join(teamDir, "inboxes"), 0755); err != nil {
		return nil, fmt.Errorf("create inboxes dir: %w", err)
	}

	teamFile := TeamFile{
		Name:        name,
		Description: description,
		CreatedAt:   time.Now().UnixMilli(),
		LeadAgentID: leadAgentID,
		Members: []TeamMember{
			{
				AgentID: leadAgentID,
				Name:    TeamLeadName,
				Role:    "leader",
			},
		},
	}

	teamFilePath := GetTeamFilePath(name)
	data, err := json.MarshalIndent(teamFile, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(teamFilePath, data, 0644); err != nil {
		return nil, err
	}

	return &SpawnTeamResult{
		TeamName:     name,
		TeamFilePath: teamFilePath,
		LeadAgentID:  leadAgentID,
	}, nil
}

// ReadTeamFile reads a team's configuration from disk using the global teams dir.
func ReadTeamFile(teamName string) (*TeamFile, error) {
	path := GetTeamFilePath(teamName)
	return readTeamFileAt(path)
}

// ReadTeamFileFromDir reads a team's configuration from an explicit teams directory.
// This is used by components (e.g. SendMessageTool broadcast) that carry their own
// teamsDir rather than relying on the global getTeamsDir.
func ReadTeamFileFromDir(teamsDir, teamName string) (*TeamFile, error) {
	safe := sanitizePathComponent(teamName)
	path := filepath.Join(teamsDir, safe, "team.json")
	return readTeamFileAt(path)
}

func readTeamFileAt(path string) (*TeamFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var tf TeamFile
	if err := json.Unmarshal(data, &tf); err != nil {
		return nil, err
	}
	return &tf, nil
}

// AddTeamMember adds a member to a team.
func AddTeamMember(teamName string, member TeamMember) error {
	tf, err := ReadTeamFile(teamName)
	if err != nil {
		return err
	}

	tf.Members = append(tf.Members, member)

	data, err := json.MarshalIndent(tf, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(GetTeamFilePath(teamName), data, 0644)
}

// CleanupTeam removes a team's directory.
// Source: utils/swarm/teamHelpers.ts — cleanup operation
func CleanupTeam(teamName string) error {
	teamDir := filepath.Join(getTeamsDir(), sanitizePathComponent(teamName))
	return os.RemoveAll(teamDir)
}
