package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/projectbarks/gopher-code/pkg/skills"
)

// SkillTool executes a skill (prompt-based command).
type SkillTool struct {
	skills []skills.Skill
}

// NewSkillTool creates a SkillTool with the given loaded skills.
func NewSkillTool(s []skills.Skill) *SkillTool {
	return &SkillTool{skills: s}
}

func (t *SkillTool) Name() string        { return "Skill" }
func (t *SkillTool) Description() string { return "Execute a skill (prompt-based command)" }
func (t *SkillTool) IsReadOnly() bool    { return true }

func (t *SkillTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"skill": {"type": "string", "description": "The skill name to execute"},
			"args": {"type": "string", "description": "Optional arguments"}
		},
		"required": ["skill"],
		"additionalProperties": false
	}`)
}

func (t *SkillTool) Execute(_ context.Context, _ *ToolContext, input json.RawMessage) (*ToolOutput, error) {
	var params struct {
		Skill string `json:"skill"`
		Args  string `json:"args"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return ErrorOutput(fmt.Sprintf("invalid input: %s", err)), nil
	}
	if params.Skill == "" {
		return ErrorOutput("skill name is required"), nil
	}

	for _, s := range t.skills {
		if s.Name == params.Skill {
			prompt := s.Prompt
			if params.Args != "" {
				prompt += "\n\nArguments: " + params.Args
			}
			return SuccessOutput(prompt), nil
		}
	}
	return ErrorOutput(fmt.Sprintf("skill not found: %s", params.Skill)), nil
}
