package mcp

import (
	"os"
	"testing"
)

// Source: services/mcp/envExpansion.ts

func TestExpandEnvVarsInString_SimpleVar(t *testing.T) {
	t.Setenv("MCP_TEST_VAR", "hello")
	result := ExpandEnvVarsInString("prefix-${MCP_TEST_VAR}-suffix")
	if result.Expanded != "prefix-hello-suffix" {
		t.Errorf("Expanded = %q, want %q", result.Expanded, "prefix-hello-suffix")
	}
	if len(result.MissingVars) != 0 {
		t.Errorf("MissingVars = %v, want empty", result.MissingVars)
	}
}

func TestExpandEnvVarsInString_DefaultValue(t *testing.T) {
	// Unset to be sure
	os.Unsetenv("MCP_TEST_UNSET")
	result := ExpandEnvVarsInString("${MCP_TEST_UNSET:-fallback}")
	if result.Expanded != "fallback" {
		t.Errorf("Expanded = %q, want %q", result.Expanded, "fallback")
	}
	if len(result.MissingVars) != 0 {
		t.Errorf("MissingVars = %v, want empty (default used)", result.MissingVars)
	}
}

func TestExpandEnvVarsInString_DefaultNotUsedWhenSet(t *testing.T) {
	t.Setenv("MCP_TEST_SET", "real-value")
	result := ExpandEnvVarsInString("${MCP_TEST_SET:-ignored}")
	if result.Expanded != "real-value" {
		t.Errorf("Expanded = %q, want %q", result.Expanded, "real-value")
	}
}

func TestExpandEnvVarsInString_MissingVar(t *testing.T) {
	os.Unsetenv("MCP_TEST_MISSING")
	result := ExpandEnvVarsInString("${MCP_TEST_MISSING}")
	if result.Expanded != "${MCP_TEST_MISSING}" {
		t.Errorf("Expanded = %q, want original preserved", result.Expanded)
	}
	if len(result.MissingVars) != 1 || result.MissingVars[0] != "MCP_TEST_MISSING" {
		t.Errorf("MissingVars = %v, want [MCP_TEST_MISSING]", result.MissingVars)
	}
}

func TestExpandEnvVarsInString_MultipleVars(t *testing.T) {
	t.Setenv("MCP_A", "alpha")
	t.Setenv("MCP_B", "beta")
	result := ExpandEnvVarsInString("${MCP_A}/${MCP_B}")
	if result.Expanded != "alpha/beta" {
		t.Errorf("Expanded = %q, want %q", result.Expanded, "alpha/beta")
	}
}

func TestExpandEnvVarsInString_DefaultWithColonDash(t *testing.T) {
	// Source: envExpansion.ts:18 — split limit 2, so :- in default is preserved
	os.Unsetenv("MCP_TEST_X")
	result := ExpandEnvVarsInString("${MCP_TEST_X:-a:-b}")
	if result.Expanded != "a:-b" {
		t.Errorf("Expanded = %q, want %q (preserve :- in default)", result.Expanded, "a:-b")
	}
}

func TestExpandEnvVarsInString_NoVars(t *testing.T) {
	result := ExpandEnvVarsInString("no variables here")
	if result.Expanded != "no variables here" {
		t.Errorf("Expanded = %q, want unchanged", result.Expanded)
	}
	if len(result.MissingVars) != 0 {
		t.Errorf("MissingVars should be empty")
	}
}

func TestExpandEnvVarsInString_EmptyString(t *testing.T) {
	result := ExpandEnvVarsInString("")
	if result.Expanded != "" {
		t.Errorf("Expanded = %q, want empty", result.Expanded)
	}
}

func TestExpandEnvInServerConfig_ExpandsAll(t *testing.T) {
	t.Setenv("MCP_CMD", "/usr/bin/node")
	t.Setenv("MCP_TOKEN", "secret123")
	t.Setenv("MCP_HOST", "example.com")

	cfg := ServerConfig{
		Command: "${MCP_CMD}",
		Args:    []string{"--host", "${MCP_HOST}"},
		Env:     map[string]string{"TOKEN": "${MCP_TOKEN}"},
		URL:     "https://${MCP_HOST}/mcp",
		Headers: map[string]string{"Authorization": "Bearer ${MCP_TOKEN}"},
	}

	expanded, missing := ExpandEnvInServerConfig(cfg)
	if len(missing) != 0 {
		t.Errorf("missing = %v, want empty", missing)
	}
	if expanded.Command != "/usr/bin/node" {
		t.Errorf("Command = %q", expanded.Command)
	}
	if expanded.Args[1] != "example.com" {
		t.Errorf("Args[1] = %q", expanded.Args[1])
	}
	if expanded.Env["TOKEN"] != "secret123" {
		t.Errorf("Env[TOKEN] = %q", expanded.Env["TOKEN"])
	}
	if expanded.URL != "https://example.com/mcp" {
		t.Errorf("URL = %q", expanded.URL)
	}
	if expanded.Headers["Authorization"] != "Bearer secret123" {
		t.Errorf("Headers[Authorization] = %q", expanded.Headers["Authorization"])
	}
}

func TestExpandEnvInServerConfig_TracksMissing(t *testing.T) {
	os.Unsetenv("MCP_UNDEFINED_1")
	os.Unsetenv("MCP_UNDEFINED_2")

	cfg := ServerConfig{
		Command: "${MCP_UNDEFINED_1}",
		Env:     map[string]string{"X": "${MCP_UNDEFINED_2}"},
	}

	_, missing := ExpandEnvInServerConfig(cfg)
	if len(missing) != 2 {
		t.Errorf("missing = %v, want 2 entries", missing)
	}
}
