package config

// Source: utils/settings/constants.ts

// SettingSource identifies where a configuration setting comes from.
// Order matters: later sources override earlier ones.
// Source: utils/settings/constants.ts:7-22
type SettingSource string

const (
	SourceUser    SettingSource = "userSettings"
	SourceProject SettingSource = "projectSettings"
	SourceLocal   SettingSource = "localSettings"
	SourceFlag    SettingSource = "flagSettings"
	SourcePolicy  SettingSource = "policySettings"
)

// SettingSources is the ordered list of all setting sources.
// Later sources override earlier ones.
// Source: utils/settings/constants.ts:7-22
var SettingSources = []SettingSource{
	SourceUser,
	SourceProject,
	SourceLocal,
	SourceFlag,
	SourcePolicy,
}

// SourceDisplayName returns the short display name for a setting source.
// Source: utils/settings/constants.ts:26-39
func SourceDisplayName(source SettingSource) string {
	switch source {
	case SourceUser:
		return "user"
	case SourceProject:
		return "project"
	case SourceLocal:
		return "project, gitignored"
	case SourceFlag:
		return "cli flag"
	case SourcePolicy:
		return "managed"
	default:
		return string(source)
	}
}

// SourceDisplayNameCapitalized returns the capitalized display name.
// Source: utils/settings/constants.ts:100-121
func SourceDisplayNameCapitalized(source SettingSource) string {
	switch source {
	case SourceUser:
		return "User settings"
	case SourceProject:
		return "Shared project settings"
	case SourceLocal:
		return "Project local settings"
	case SourceFlag:
		return "Command line arguments"
	case SourcePolicy:
		return "Enterprise managed settings"
	default:
		return string(source)
	}
}

// ParseSettingSourcesFlag parses a comma-separated string like "user,project,local".
// Source: utils/settings/constants.ts:128-153
func ParseSettingSourcesFlag(flag string) ([]SettingSource, error) {
	if flag == "" {
		return nil, nil
	}

	var result []SettingSource
	for _, name := range splitAndTrim(flag) {
		switch name {
		case "user":
			result = append(result, SourceUser)
		case "project":
			result = append(result, SourceProject)
		case "local":
			result = append(result, SourceLocal)
		default:
			return nil, &InvalidSourceError{Name: name}
		}
	}
	return result, nil
}

// InvalidSourceError is returned when an unknown source name is parsed.
type InvalidSourceError struct {
	Name string
}

func (e *InvalidSourceError) Error() string {
	return "Invalid setting source: " + e.Name + ". Valid options are: user, project, local"
}

// GetEnabledSources returns the enabled sources with policy/flag always included.
// Source: utils/settings/constants.ts:159-167
func GetEnabledSources(allowed []SettingSource) []SettingSource {
	seen := make(map[SettingSource]bool)
	var result []SettingSource

	for _, s := range allowed {
		if !seen[s] {
			result = append(result, s)
			seen[s] = true
		}
	}

	// Always include policy and flag
	// Source: utils/settings/constants.ts:163-165
	if !seen[SourcePolicy] {
		result = append(result, SourcePolicy)
	}
	if !seen[SourceFlag] {
		result = append(result, SourceFlag)
	}

	return result
}

func splitAndTrim(s string) []string {
	var result []string
	start := 0
	for i := 0; i <= len(s); i++ {
		if i == len(s) || s[i] == ',' {
			part := s[start:i]
			// Trim spaces
			for len(part) > 0 && part[0] == ' ' {
				part = part[1:]
			}
			for len(part) > 0 && part[len(part)-1] == ' ' {
				part = part[:len(part)-1]
			}
			if part != "" {
				result = append(result, part)
			}
			start = i + 1
		}
	}
	return result
}
