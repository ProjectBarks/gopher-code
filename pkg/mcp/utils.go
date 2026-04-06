package mcp

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strings"
)

// Source: services/mcp/utils.ts

// ScopeLabel returns a human-readable label for a config scope.
// Source: services/mcp/utils.ts:282-299
func ScopeLabel(scope ConfigScope) string {
	switch scope {
	case ScopeLocal:
		return "Local config (private to you in this project)"
	case ScopeProject:
		return "Project config (shared via .mcp.json)"
	case ScopeUser:
		return "User config (available in all your projects)"
	case ScopeDynamic:
		return "Dynamic config (from command line)"
	case ScopeEnterprise:
		return "Enterprise config (managed by your organization)"
	case ScopeClaudeAI:
		return "claude.ai config"
	default:
		return string(scope)
	}
}

// ScopeClaudeAI is the scope for Claude.ai-proxied servers.
const ScopeClaudeAI ConfigScope = "claudeai"

// EnsureConfigScope validates and normalizes a config scope string.
// Returns ScopeLocal if scope is empty.
// Source: services/mcp/utils.ts:301-311
func EnsureConfigScope(scope string) (ConfigScope, error) {
	if scope == "" {
		return ScopeLocal, nil
	}
	valid := []ConfigScope{
		ScopeLocal, ScopeUser, ScopeProject, ScopeDynamic,
		ScopeEnterprise, ScopeManaged, ScopeClaudeAI,
	}
	for _, v := range valid {
		if ConfigScope(scope) == v {
			return v, nil
		}
	}
	names := make([]string, len(valid))
	for i, v := range valid {
		names[i] = string(v)
	}
	return "", fmt.Errorf("invalid scope: %s. Must be one of: %s", scope, strings.Join(names, ", "))
}

// EnsureTransport validates and normalizes a transport type string.
// Returns TransportStdio if type is empty.
// Source: services/mcp/utils.ts:313-323
func EnsureTransport(tp string) (Transport, error) {
	if tp == "" {
		return TransportStdio, nil
	}
	switch Transport(tp) {
	case TransportStdio, TransportSSE, TransportHTTP:
		return Transport(tp), nil
	default:
		return "", fmt.Errorf("invalid transport type: %s. Must be one of: stdio, sse, http", tp)
	}
}

// ParseHeaders parses CLI --header "Key: value" arguments into a map.
// Source: services/mcp/utils.ts:325-349
func ParseHeaders(headers []string) (map[string]string, error) {
	result := make(map[string]string, len(headers))
	for _, h := range headers {
		idx := strings.Index(h, ":")
		if idx < 0 {
			return nil, fmt.Errorf("invalid header format: %q. Expected format: \"Header-Name: value\"", h)
		}
		key := strings.TrimSpace(h[:idx])
		value := strings.TrimSpace(h[idx+1:])
		if key == "" {
			return nil, fmt.Errorf("invalid header: %q. Header name cannot be empty", h)
		}
		result[key] = value
	}
	return result, nil
}

// HashMCPConfig produces a stable 16-hex-char hash of an MCP server config
// for change detection. The scope field is excluded since it represents
// provenance, not content.
// Source: services/mcp/utils.ts:157-169
func HashMCPConfig(cfg ScopedServerConfig) string {
	// Create a copy without scope for hashing
	inner := cfg.ServerConfig
	data := sortedJSON(inner)
	h := sha256.Sum256(data)
	return fmt.Sprintf("%x", h[:8]) // 16 hex chars
}

// sortedJSON marshals v to JSON with map keys sorted for stability.
func sortedJSON(v interface{}) []byte {
	data, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	// Re-parse and re-marshal with sorted keys
	var raw interface{}
	if json.Unmarshal(data, &raw) != nil {
		return data
	}
	sorted := sortValue(raw)
	out, _ := json.Marshal(sorted)
	return out
}

func sortValue(v interface{}) interface{} {
	switch val := v.(type) {
	case map[string]interface{}:
		keys := make([]string, 0, len(val))
		for k := range val {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		sorted := make(orderedMap, len(keys))
		for i, k := range keys {
			sorted[i] = orderedEntry{Key: k, Value: sortValue(val[k])}
		}
		return sorted
	case []interface{}:
		for i, item := range val {
			val[i] = sortValue(item)
		}
		return val
	default:
		return v
	}
}

type orderedEntry struct {
	Key   string
	Value interface{}
}

type orderedMap []orderedEntry

func (o orderedMap) MarshalJSON() ([]byte, error) {
	var buf strings.Builder
	buf.WriteByte('{')
	for i, entry := range o {
		if i > 0 {
			buf.WriteByte(',')
		}
		key, _ := json.Marshal(entry.Key)
		val, _ := json.Marshal(entry.Value)
		buf.Write(key)
		buf.WriteByte(':')
		buf.Write(val)
	}
	buf.WriteByte('}')
	return []byte(buf.String()), nil
}

// GetLoggingSafeMCPBaseURL strips query strings and trailing slashes from
// a server config URL for safe analytics logging. Returns empty string for
// stdio/sdk servers or if URL parsing fails.
// Source: services/mcp/utils.ts:561-575
func GetLoggingSafeMCPBaseURL(cfg ServerConfig) string {
	if cfg.URL == "" {
		return ""
	}
	u, err := url.Parse(cfg.URL)
	if err != nil {
		return ""
	}
	u.RawQuery = ""
	u.Fragment = ""
	result := u.String()
	return strings.TrimRight(result, "/")
}

// IsToolFromMCPServer checks if a tool name belongs to a specific MCP server.
// Source: services/mcp/utils.ts:232-238
func IsToolFromMCPServer(toolName, serverName string) bool {
	server, _, ok := ParseMCPToolName(toolName)
	return ok && server == serverName
}

// ErrInvalidScope is returned when an invalid scope is provided.
var ErrInvalidScope = errors.New("invalid scope")

// ErrInvalidTransport is returned when an invalid transport type is provided.
var ErrInvalidTransport = errors.New("invalid transport")
