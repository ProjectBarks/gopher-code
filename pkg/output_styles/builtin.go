package output_styles

// Source: constants/outputStyles.ts
//
// Built-in output styles and the merged style resolver.

import "github.com/projectbarks/gopher-code/pkg/config"

// DefaultOutputStyleName is the name of the default (no style) output style.
const DefaultOutputStyleName = "default"

// BuiltInStyles are the hard-coded output styles.
// Source: constants/outputStyles.ts:41-135
var BuiltInStyles = map[string]*OutputStyleConfig{
	DefaultOutputStyleName: nil, // default = no style applied
	"Explanatory": {
		Name:                   "Explanatory",
		Description:            "Claude explains its implementation choices and codebase patterns",
		Source:                 "built-in",
		KeepCodingInstructions: boolPtr(true),
		Prompt: `You are an interactive CLI tool that helps users with software engineering tasks. In addition to software engineering tasks, you should provide educational insights about the codebase along the way.

You should be clear and educational, providing helpful explanations while remaining focused on the task. Balance educational content with task completion.

# Explanatory Style Active
## Insights
Before and after writing code, provide brief educational explanations about implementation choices.`,
	},
	"Learning": {
		Name:                   "Learning",
		Description:            "Claude pauses and asks you to write small pieces of code for hands-on practice",
		Source:                 "built-in",
		KeepCodingInstructions: boolPtr(true),
		Prompt: `You are an interactive CLI tool that helps users with software engineering tasks. In addition to software engineering tasks, you should help users learn more about the codebase through hands-on practice and educational insights.

You should be collaborative and encouraging. Balance task completion with learning by requesting user input for meaningful design decisions while handling routine implementation yourself.

# Learning Style Active
## Requesting Human Contributions
Ask the human to contribute 2-10 line code pieces when generating 20+ lines involving:
- Design decisions (error handling, data structures)
- Business logic with multiple valid approaches
- Key algorithms or interface definitions`,
	},
}

func boolPtr(b bool) *bool { return &b }

// GetAllOutputStyles returns all available styles: built-in + user/project/managed.
// Source: constants/outputStyles.ts:137-180
func GetAllOutputStyles(cwd string) map[string]*OutputStyleConfig {
	all := make(map[string]*OutputStyleConfig)

	// Start with built-in
	for k, v := range BuiltInStyles {
		all[k] = v
	}

	// Layer custom styles (managed < user < project)
	customStyles := GetOutputStyleDirStyles(cwd)
	for _, s := range customStyles {
		all[s.Name] = &s
	}

	return all
}

// GetOutputStyleConfig resolves the active output style from settings + overrides.
// Returns nil for the default style (no prompt modification).
// Source: constants/outputStyles.ts:194-240
func GetOutputStyleConfig(cwd string) *OutputStyleConfig {
	// Check all sources in priority order
	all := GetAllOutputStyles(cwd)

	// Check for forced plugin style
	for _, s := range all {
		if s != nil && s.ForceForPlugin {
			return s
		}
	}

	// Check settings for user-selected style
	settings := config.Load(cwd)
	if settings != nil {
		// settings.OutputStyle is the user's choice
		// For now, return nil (default)
	}
	_ = settings

	return nil
}

// GetOutputStyleNames returns the names of all available styles.
func GetOutputStyleNames(cwd string) []string {
	all := GetAllOutputStyles(cwd)
	names := make([]string, 0, len(all))
	for k := range all {
		names = append(names, k)
	}
	return names
}
