package session

import (
	"os"
	"path/filepath"
)

// Source: utils/memdir/teamMemoryPaths.ts

// TeamMemoryDir returns the path to the team's shared memory directory.
// Team memories are stored per-team under ~/.claude/teams/{teamName}/memory/
func TeamMemoryDir(teamName string) string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".claude", "teams", teamName, "memory")
}

// TeamMemoryIndexPath returns the path to MEMORY.md for a team.
func TeamMemoryIndexPath(teamName string) string {
	return filepath.Join(TeamMemoryDir(teamName), "MEMORY.md")
}

// ListTeamMemoryFiles returns all .md files in the team memory directory.
func ListTeamMemoryFiles(teamName string) ([]string, error) {
	dir := TeamMemoryDir(teamName)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var files []string
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".md" {
			files = append(files, filepath.Join(dir, e.Name()))
		}
	}
	return files, nil
}
