package analytics

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

// SessionInfo holds session-level metadata set once at startup.
type SessionInfo struct {
	SessionID string
	Model     string
	Version   string
	UserType  string // "ant", "external", etc.
	ClientType string // "cli", "vscode", etc.
	IsInteractive bool
	Cwd       string
}

// globalSession is the session info set at startup.
var (
	sessionMu   sync.RWMutex
	sessionInfo SessionInfo
)

// SetSessionInfo records the session metadata used to enrich all events.
func SetSessionInfo(info SessionInfo) {
	sessionMu.Lock()
	defer sessionMu.Unlock()
	sessionInfo = info
}

// GetSessionInfo returns the current session info.
func GetSessionInfo() SessionInfo {
	sessionMu.RLock()
	defer sessionMu.RUnlock()
	return sessionInfo
}

// EnvContext describes the runtime environment.
type EnvContext struct {
	Platform string // "darwin", "linux", "windows"
	Arch     string // "amd64", "arm64"
	GoVersion string
	Terminal  string
	IsCi     bool
	Version  string
}

// BuildEnvContext gathers environment context (memoized-safe: all values
// are stable for the process lifetime).
func BuildEnvContext() EnvContext {
	envCtxOnce.Do(func() {
		cachedEnvCtx = EnvContext{
			Platform:  runtime.GOOS,
			Arch:      runtime.GOARCH,
			GoVersion: runtime.Version(),
			Terminal:  os.Getenv("TERM"),
			IsCi:      isEnvTruthy(os.Getenv("CI")),
			Version:  os.Getenv("GOPHER_VERSION"),
		}
	})
	return cachedEnvCtx
}

var (
	envCtxOnce   sync.Once
	cachedEnvCtx EnvContext
)

// GetEventMetadata returns the enriched metadata map to attach to an event.
func GetEventMetadata() EventMetadata {
	info := GetSessionInfo()
	env := BuildEnvContext()

	m := EventMetadata{
		"platform":      env.Platform,
		"arch":          env.Arch,
		"goVersion":     env.GoVersion,
		"version":       env.Version,
		"sessionId":     info.SessionID,
		"model":         info.Model,
		"userType":      info.UserType,
		"clientType":    info.ClientType,
		"isInteractive": info.IsInteractive,
		"isCi":          env.IsCi,
	}
	if env.Terminal != "" {
		m["terminal"] = env.Terminal
	}
	return m
}

// SanitizeToolNameForAnalytics redacts MCP tool names to prevent PII
// exposure. Built-in tool names (Bash, Read, etc.) pass through unchanged.
func SanitizeToolNameForAnalytics(toolName string) string {
	if strings.HasPrefix(toolName, "mcp__") {
		return "mcp_tool"
	}
	return toolName
}

// ExtractMCPToolDetails parses a tool name in format mcp__<server>__<tool>.
// Returns server name and tool name, or empty strings if not an MCP tool.
func ExtractMCPToolDetails(toolName string) (serverName, mcpToolName string) {
	if !strings.HasPrefix(toolName, "mcp__") {
		return "", ""
	}
	parts := strings.SplitN(toolName, "__", 3)
	if len(parts) < 3 || parts[1] == "" || parts[2] == "" {
		return "", ""
	}
	return parts[1], parts[2]
}

// maxFileExtensionLength caps extension length to avoid logging
// potentially sensitive hash-based filenames.
const maxFileExtensionLength = 10

// GetFileExtensionForAnalytics extracts and sanitizes a file extension.
// Returns empty string if no extension or extension is too long.
func GetFileExtensionForAnalytics(filePath string) string {
	ext := strings.ToLower(filepath.Ext(filePath))
	if ext == "" || ext == "." {
		return ""
	}
	ext = ext[1:] // strip leading dot
	if len(ext) > maxFileExtensionLength {
		return "other"
	}
	return ext
}
