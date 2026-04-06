package mcp

import (
	"os"
	"regexp"
	"strings"
)

// Source: services/mcp/envExpansion.ts

// envVarPattern matches ${VAR} and ${VAR:-default} syntax.
var envVarPattern = regexp.MustCompile(`\$\{([^}]+)\}`)

// EnvExpansionResult holds the expanded string and any missing variable names.
type EnvExpansionResult struct {
	Expanded    string
	MissingVars []string
}

// ExpandEnvVarsInString expands environment variable references in a string.
// Supports ${VAR} and ${VAR:-default} syntax (bash-style).
//
// When a variable is not set and has no default, it is added to MissingVars
// and the original ${VAR} reference is preserved in the output.
//
// Source: services/mcp/envExpansion.ts:10-38
func ExpandEnvVarsInString(value string) EnvExpansionResult {
	var missingVars []string

	expanded := envVarPattern.ReplaceAllStringFunc(value, func(match string) string {
		// Strip ${ and }
		inner := match[2 : len(match)-1]

		// Split on :- to support default values (limit to 2 parts)
		varName := inner
		defaultValue := ""
		hasDefault := false

		if idx := strings.Index(inner, ":-"); idx >= 0 {
			varName = inner[:idx]
			defaultValue = inner[idx+2:]
			hasDefault = true
		}

		if envValue, ok := os.LookupEnv(varName); ok {
			return envValue
		}
		if hasDefault {
			return defaultValue
		}

		missingVars = append(missingVars, varName)
		return match // preserve original for debugging
	})

	return EnvExpansionResult{
		Expanded:    expanded,
		MissingVars: missingVars,
	}
}

// ExpandEnvInServerConfig expands environment variables in all string fields
// of a ServerConfig's env map values. Returns the expanded config and any
// missing variable names encountered.
func ExpandEnvInServerConfig(cfg ServerConfig) (ServerConfig, []string) {
	var allMissing []string

	// Expand env map values
	if len(cfg.Env) > 0 {
		expanded := make(map[string]string, len(cfg.Env))
		for k, v := range cfg.Env {
			result := ExpandEnvVarsInString(v)
			expanded[k] = result.Expanded
			allMissing = append(allMissing, result.MissingVars...)
		}
		cfg.Env = expanded
	}

	// Expand command
	if cfg.Command != "" {
		result := ExpandEnvVarsInString(cfg.Command)
		cfg.Command = result.Expanded
		allMissing = append(allMissing, result.MissingVars...)
	}

	// Expand args
	if len(cfg.Args) > 0 {
		expandedArgs := make([]string, len(cfg.Args))
		for i, arg := range cfg.Args {
			result := ExpandEnvVarsInString(arg)
			expandedArgs[i] = result.Expanded
			allMissing = append(allMissing, result.MissingVars...)
		}
		cfg.Args = expandedArgs
	}

	// Expand URL
	if cfg.URL != "" {
		result := ExpandEnvVarsInString(cfg.URL)
		cfg.URL = result.Expanded
		allMissing = append(allMissing, result.MissingVars...)
	}

	// Expand headers values
	if len(cfg.Headers) > 0 {
		expanded := make(map[string]string, len(cfg.Headers))
		for k, v := range cfg.Headers {
			result := ExpandEnvVarsInString(v)
			expanded[k] = result.Expanded
			allMissing = append(allMissing, result.MissingVars...)
		}
		cfg.Headers = expanded
	}

	return cfg, allMissing
}
