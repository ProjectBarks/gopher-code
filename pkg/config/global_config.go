package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
)

// Source: src/utils/config.ts — GlobalConfig, ProjectConfig, save/load, trust, constants
// Source: src/utils/configConstants.ts — NotificationChannels, EditorModes, TeammateModes

// ConfigWriteDisplayThreshold is the threshold for warning about runaway config writes.
// Source: config.ts:887
const ConfigWriteDisplayThreshold = 20

// NotificationChannels lists valid notification channel values.
// Source: configConstants.ts:4-12
var NotificationChannels = []string{
	"auto", "iterm2", "iterm2_with_bell", "terminal_bell",
	"kitty", "ghostty", "notifications_disabled",
}

// EditorModes lists valid editor modes (excludes deprecated 'emacs').
// Source: configConstants.ts:15
var EditorModes = []string{"normal", "vim"}

// TeammateModes lists valid teammate spawn modes.
// Source: configConstants.ts:20
var TeammateModes = []string{"auto", "tmux", "in-process"}

// GlobalConfigKeys is the list of user-configurable keys in the global config.
// Source: config.ts:627-666
var GlobalConfigKeys = []string{
	"apiKeyHelper",
	"installMethod",
	"autoUpdates",
	"autoUpdatesProtectedForNative",
	"theme",
	"verbose",
	"preferredNotifChannel",
	"shiftEnterKeyBindingInstalled",
	"editorMode",
	"hasUsedBackslashReturn",
	"autoCompactEnabled",
	"showTurnDuration",
	"diffTool",
	"env",
	"tipsHistory",
	"todoFeatureEnabled",
	"showExpandedTodos",
	"messageIdleNotifThresholdMs",
	"autoConnectIde",
	"autoInstallIdeExtension",
	"fileCheckpointingEnabled",
	"terminalProgressBarEnabled",
	"showStatusInTerminalTab",
	"taskCompleteNotifEnabled",
	"inputNeededNotifEnabled",
	"agentPushNotifEnabled",
	"respectGitignore",
	"claudeInChromeDefaultEnabled",
	"hasCompletedClaudeInChromeOnboarding",
	"lspRecommendationDisabled",
	"lspRecommendationNeverPlugins",
	"lspRecommendationIgnoredCount",
	"copyFullResponse",
	"copyOnSelect",
	"permissionExplainerEnabled",
	"prStatusFooterEnabled",
	"remoteControlAtStartup",
	"remoteDialogSeen",
}

// ProjectConfigKeys is the list of user-configurable keys in a project config.
// Source: config.ts:674-678
var ProjectConfigKeys = []string{
	"allowedTools",
	"hasTrustDialogAccepted",
	"hasCompletedProjectOnboarding",
}

// IsGlobalConfigKey returns true if key is a user-configurable global config key.
func IsGlobalConfigKey(key string) bool {
	for _, k := range GlobalConfigKeys {
		if k == key {
			return true
		}
	}
	return false
}

// IsProjectConfigKey returns true if key is a user-configurable project config key.
func IsProjectConfigKey(key string) bool {
	for _, k := range ProjectConfigKeys {
		if k == key {
			return true
		}
	}
	return false
}

// AccountInfo represents OAuth account information.
// Source: config.ts:161-174
type AccountInfo struct {
	AccountUUID        string `json:"accountUuid"`
	EmailAddress       string `json:"emailAddress"`
	OrganizationUUID   string `json:"organizationUuid,omitempty"`
	OrganizationName   string `json:"organizationName,omitempty"`
	OrganizationRole   string `json:"organizationRole,omitempty"`
	WorkspaceRole      string `json:"workspaceRole,omitempty"`
	DisplayName        string `json:"displayName,omitempty"`
	HasExtraUsageEnabled bool  `json:"hasExtraUsageEnabled,omitempty"`
	BillingType        string `json:"billingType,omitempty"`
	AccountCreatedAt   string `json:"accountCreatedAt,omitempty"`
	SubscriptionCreatedAt string `json:"subscriptionCreatedAt,omitempty"`
}

// CustomApiKeyResponses tracks user approval/rejection of custom API keys.
// Source: config.ts:220-223
type CustomApiKeyResponses struct {
	Approved []string `json:"approved,omitempty"`
	Rejected []string `json:"rejected,omitempty"`
}

// ProjectConfig holds per-project configuration stored in ~/.claude.json under "projects".
// Source: config.ts:76-136
type ProjectConfig struct {
	AllowedTools                          []string `json:"allowedTools,omitempty"`
	McpContextUris                        []string `json:"mcpContextUris,omitempty"`
	HasTrustDialogAccepted                bool     `json:"hasTrustDialogAccepted,omitempty"`
	HasCompletedProjectOnboarding         bool     `json:"hasCompletedProjectOnboarding,omitempty"`
	ProjectOnboardingSeenCount            int      `json:"projectOnboardingSeenCount,omitempty"`
	HasClaudeMdExternalIncludesApproved   bool     `json:"hasClaudeMdExternalIncludesApproved,omitempty"`
	HasClaudeMdExternalIncludesWarningShown bool   `json:"hasClaudeMdExternalIncludesWarningShown,omitempty"`
	LastSessionID                         string   `json:"lastSessionId,omitempty"`
}

// GlobalConfig holds the full ~/.claude.json configuration.
// Source: config.ts:183-578
type GlobalConfig struct {
	NumStartups            int                        `json:"numStartups"`
	InstallMethod          string                     `json:"installMethod,omitempty"`
	AutoUpdates            *bool                      `json:"autoUpdates,omitempty"`
	Theme                  string                     `json:"theme"`
	HasCompletedOnboarding bool                       `json:"hasCompletedOnboarding,omitempty"`
	Verbose                bool                       `json:"verbose"`
	PreferredNotifChannel  string                     `json:"preferredNotifChannel"`
	EditorMode             string                     `json:"editorMode,omitempty"`
	AutoCompactEnabled     bool                       `json:"autoCompactEnabled"`
	ShowTurnDuration       bool                       `json:"showTurnDuration"`
	DiffTool               string                     `json:"diffTool,omitempty"`
	TodoFeatureEnabled     bool                       `json:"todoFeatureEnabled"`
	ShowExpandedTodos      bool                       `json:"showExpandedTodos,omitempty"`
	MessageIdleNotifThresholdMs int                   `json:"messageIdleNotifThresholdMs"`
	AutoConnectIde         bool                       `json:"autoConnectIde"`
	AutoInstallIdeExtension bool                      `json:"autoInstallIdeExtension"`
	FileCheckpointingEnabled bool                     `json:"fileCheckpointingEnabled"`
	TerminalProgressBarEnabled bool                   `json:"terminalProgressBarEnabled"`
	RespectGitignore       bool                       `json:"respectGitignore"`
	CopyFullResponse       bool                       `json:"copyFullResponse"`
	MemoryUsageCount       int                        `json:"memoryUsageCount"`
	PromptQueueUseCount    int                        `json:"promptQueueUseCount"`
	BtwUseCount            int                        `json:"btwUseCount"`
	UserID                 string                     `json:"userID,omitempty"`
	PrimaryApiKey          string                     `json:"primaryApiKey,omitempty"`
	OauthAccount           *AccountInfo               `json:"oauthAccount,omitempty"`
	CustomApiKeyResponses  *CustomApiKeyResponses     `json:"customApiKeyResponses,omitempty"`
	Env                    map[string]string           `json:"env,omitempty"`
	Projects               map[string]*ProjectConfig  `json:"projects,omitempty"`

	// UI tracking
	HasUsedBackslashReturn bool  `json:"hasUsedBackslashReturn,omitempty"`
	HasSeenTasksHint       bool  `json:"hasSeenTasksHint,omitempty"`
	HasUsedStash           bool  `json:"hasUsedStash,omitempty"`
	HasUsedBackgroundTask  bool  `json:"hasUsedBackgroundTask,omitempty"`
	QueuedCommandUpHintCount int `json:"queuedCommandUpHintCount,omitempty"`

	// Terminal setup
	ShiftEnterKeyBindingInstalled bool `json:"shiftEnterKeyBindingInstalled,omitempty"`

	// Feature flags / caches (kept for parity; not exhaustive)
	CachedStatsigGates        map[string]bool           `json:"cachedStatsigGates,omitempty"`
	CachedGrowthBookFeatures  map[string]interface{}     `json:"cachedGrowthBookFeatures,omitempty"`

	// Remote control
	RemoteControlAtStartup *bool `json:"remoteControlAtStartup,omitempty"`
}

// DefaultGlobalConfig returns a fresh default GlobalConfig matching the TS factory.
// Source: config.ts:585-623
func DefaultGlobalConfig() GlobalConfig {
	return GlobalConfig{
		NumStartups:                0,
		Theme:                      "dark",
		PreferredNotifChannel:      "auto",
		Verbose:                    false,
		EditorMode:                 "normal",
		AutoCompactEnabled:         true,
		ShowTurnDuration:           true,
		DiffTool:                   "auto",
		TodoFeatureEnabled:         true,
		ShowExpandedTodos:          false,
		MessageIdleNotifThresholdMs: 60000,
		AutoConnectIde:             false,
		AutoInstallIdeExtension:    true,
		FileCheckpointingEnabled:   true,
		TerminalProgressBarEnabled: true,
		RespectGitignore:           true,
		CopyFullResponse:           false,
		MemoryUsageCount:           0,
		PromptQueueUseCount:        0,
		BtwUseCount:                0,
	}
}

// DefaultProjectConfig returns a fresh default ProjectConfig.
// Source: config.ts:138-148
func DefaultProjectConfig() ProjectConfig {
	return ProjectConfig{
		AllowedTools:           []string{},
		McpContextUris:         []string{},
		HasTrustDialogAccepted: false,
		ProjectOnboardingSeenCount: 0,
	}
}

// NormalizePathForConfigKey normalizes a path for use as a JSON key in the config.
// Resolves . and .. segments and removes trailing slashes.
// Source: src/utils/path.ts:149-155
func NormalizePathForConfigKey(p string) string {
	cleaned := filepath.Clean(p)
	// Convert to forward slashes for consistency (matters on Windows)
	return strings.ReplaceAll(cleaned, "\\", "/")
}

// GlobalConfigStore manages reading/writing the global ~/.claude.json config.
// Thread-safe. Caches the parsed config in memory.
type GlobalConfigStore struct {
	path       string
	mu         sync.RWMutex
	cache      *GlobalConfig
	writeCount atomic.Int64
	enabled    bool
}

// NewGlobalConfigStore creates a store for the given config file path.
func NewGlobalConfigStore(path string) *GlobalConfigStore {
	return &GlobalConfigStore{path: path}
}

// Enable validates the config file and marks the store as ready.
// Idempotent. Source: config.ts:1334-1356
func (s *GlobalConfigStore) Enable() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.enabled {
		return
	}
	s.enabled = true
	// Force a load to validate the file early
	s.loadLocked()
}

// Get returns the current global config, loading from disk on first access.
// Source: config.ts:1044-1086
func (s *GlobalConfigStore) Get() GlobalConfig {
	s.mu.RLock()
	if s.cache != nil {
		c := *s.cache
		s.mu.RUnlock()
		return c
	}
	s.mu.RUnlock()

	s.mu.Lock()
	defer s.mu.Unlock()
	// Double-check after acquiring write lock
	if s.cache != nil {
		return *s.cache
	}
	return s.loadLocked()
}

// loadLocked reads and parses the config file. Must hold s.mu write lock.
func (s *GlobalConfigStore) loadLocked() GlobalConfig {
	cfg := DefaultGlobalConfig()

	data, err := os.ReadFile(s.path)
	if err != nil {
		s.cache = &cfg
		return cfg
	}

	// Strip BOM (PowerShell 5.x adds BOM to UTF-8 files)
	data = stripBOM(data)

	if err := json.Unmarshal(data, &cfg); err != nil {
		// Corrupted config — return defaults
		s.cache = &cfg
		cfg = DefaultGlobalConfig()
		s.cache = &cfg
		return cfg
	}

	// Apply defaults for fields that have zero values but should have non-zero defaults.
	// json.Unmarshal only fills present fields; missing fields stay at Go zero values.
	applyDefaults(&cfg)

	s.cache = &cfg
	return cfg
}

// applyDefaults fills in default values for fields that are missing from the JSON.
func applyDefaults(cfg *GlobalConfig) {
	defaults := DefaultGlobalConfig()
	if cfg.Theme == "" {
		cfg.Theme = defaults.Theme
	}
	if cfg.PreferredNotifChannel == "" {
		cfg.PreferredNotifChannel = defaults.PreferredNotifChannel
	}
	if cfg.EditorMode == "" {
		cfg.EditorMode = defaults.EditorMode
	}
	if cfg.DiffTool == "" {
		cfg.DiffTool = defaults.DiffTool
	}
	// Bool defaults where true is the default — JSON won't set these if missing.
	// We use a trick: unmarshal into defaults so the defaults are the starting point.
	// But since we unmarshal into DefaultGlobalConfig(), this is already handled
	// if the caller starts from defaults. The issue is json.Unmarshal into an
	// existing struct: it leaves missing fields at their current value (which IS
	// the default since we start from DefaultGlobalConfig()). So this is a no-op
	// for fields that were not in the JSON. But for explicitly-set false, it stays false.
}

// Save applies an updater function to the current config and writes to disk.
// Source: config.ts:797-866
func (s *GlobalConfigStore) Save(updater func(c *GlobalConfig)) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Load current state
	cfg := DefaultGlobalConfig()
	if s.cache != nil {
		cfg = *s.cache
	} else {
		data, err := os.ReadFile(s.path)
		if err == nil {
			data = stripBOM(data)
			json.Unmarshal(data, &cfg) //nolint:errcheck
			applyDefaults(&cfg)
		}
	}

	// Apply the updater
	updater(&cfg)

	// Ensure directory exists
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return
	}

	// Marshal and write with secure permissions
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return
	}

	if err := os.WriteFile(s.path, data, 0600); err != nil {
		return
	}

	s.cache = &cfg
	s.writeCount.Add(1)
}

// WriteCount returns the number of writes performed this session.
// Source: config.ts:883-885
func (s *GlobalConfigStore) WriteCount() int64 {
	return s.writeCount.Load()
}

// IsPathTrusted checks if dir (or any ancestor) has trust accepted in the config.
// Source: config.ts:752-761
func (s *GlobalConfigStore) IsPathTrusted(dir string) bool {
	cfg := s.Get()
	if cfg.Projects == nil {
		return false
	}

	currentPath := NormalizePathForConfigKey(filepath.Clean(dir))
	for {
		if pc, ok := cfg.Projects[currentPath]; ok && pc.HasTrustDialogAccepted {
			return true
		}
		parent := NormalizePathForConfigKey(filepath.Dir(currentPath))
		if parent == currentPath {
			return false
		}
		currentPath = parent
	}
}

// GetCustomApiKeyStatus returns "approved", "rejected", or "new" for a given truncated API key.
// Source: config.ts:1103-1114
func (s *GlobalConfigStore) GetCustomApiKeyStatus(truncatedKey string) string {
	cfg := s.Get()
	if cfg.CustomApiKeyResponses != nil {
		for _, k := range cfg.CustomApiKeyResponses.Approved {
			if k == truncatedKey {
				return "approved"
			}
		}
		for _, k := range cfg.CustomApiKeyResponses.Rejected {
			if k == truncatedKey {
				return "rejected"
			}
		}
	}
	return "new"
}

// stripBOM removes a UTF-8 BOM prefix if present.
func stripBOM(data []byte) []byte {
	if len(data) >= 3 && data[0] == 0xEF && data[1] == 0xBB && data[2] == 0xBF {
		return data[3:]
	}
	return data
}
