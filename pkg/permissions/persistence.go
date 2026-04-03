package permissions

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// Source: utils/permissions/PermissionUpdate.ts, utils/permissions/PermissionUpdateSchema.ts

// PermissionUpdateDestination identifies where rules should be saved.
// Source: PermissionUpdateSchema.ts:28-39
type PermissionUpdateDestination string

const (
	DestUserSettings    PermissionUpdateDestination = "userSettings"
	DestProjectSettings PermissionUpdateDestination = "projectSettings"
	DestLocalSettings   PermissionUpdateDestination = "localSettings"
	DestSession         PermissionUpdateDestination = "session"
	DestCLIArg          PermissionUpdateDestination = "cliArg"
)

// PermissionBehavior is the behavior for a rule (allow, deny, ask).
// Source: PermissionRule.ts — permissionBehaviorSchema
type PermissionBehavior string

const (
	BehaviorAllow PermissionBehavior = "allow"
	BehaviorDeny  PermissionBehavior = "deny"
	BehaviorAsk   PermissionBehavior = "ask"
)

// PermissionUpdateType is the kind of permission update.
// Source: PermissionUpdateSchema.ts:43-77
type PermissionUpdateType string

const (
	UpdateAddRules         PermissionUpdateType = "addRules"
	UpdateReplaceRules     PermissionUpdateType = "replaceRules"
	UpdateRemoveRules      PermissionUpdateType = "removeRules"
	UpdateSetMode          PermissionUpdateType = "setMode"
	UpdateAddDirectories   PermissionUpdateType = "addDirectories"
	UpdateRemoveDirectories PermissionUpdateType = "removeDirectories"
)

// PermissionUpdate describes a single permission change.
// Source: PermissionUpdateSchema.ts:42-78
type PermissionUpdate struct {
	Type        PermissionUpdateType        `json:"type"`
	Rules       []string                    `json:"rules,omitempty"`       // rule strings for add/replace/remove
	Behavior    PermissionBehavior          `json:"behavior,omitempty"`    // allow/deny/ask
	Destination PermissionUpdateDestination `json:"destination"`
	Mode        string                      `json:"mode,omitempty"`        // for setMode
	Directories []string                    `json:"directories,omitempty"` // for add/removeDirectories
}

// PermissionsConfig is the permissions block in settings.json.
// Source: settings/types.ts — PermissionsSchema
type PermissionsConfig struct {
	Allow                []string `json:"allow,omitempty"`
	Deny                 []string `json:"deny,omitempty"`
	Ask                  []string `json:"ask,omitempty"`
	DefaultMode          string   `json:"defaultMode,omitempty"`
	AdditionalDirectories []string `json:"additionalDirectories,omitempty"`
}

// ToolPermissionContext holds the runtime permission state with rules from multiple sources.
// Source: Tool.ts — ToolPermissionContext
type ToolPermissionContext struct {
	Mode                        string
	AlwaysAllowRules            map[PermissionUpdateDestination][]string
	AlwaysDenyRules             map[PermissionUpdateDestination][]string
	AlwaysAskRules              map[PermissionUpdateDestination][]string
	AdditionalWorkingDirectories map[string]string // path → source
}

// NewToolPermissionContext creates a fresh permission context.
func NewToolPermissionContext(mode string) *ToolPermissionContext {
	return &ToolPermissionContext{
		Mode:                        mode,
		AlwaysAllowRules:            make(map[PermissionUpdateDestination][]string),
		AlwaysDenyRules:             make(map[PermissionUpdateDestination][]string),
		AlwaysAskRules:              make(map[PermissionUpdateDestination][]string),
		AdditionalWorkingDirectories: make(map[string]string),
	}
}

// AllAllowRules returns all allow rules flattened from all sources.
func (c *ToolPermissionContext) AllAllowRules() []string {
	var rules []string
	for _, rs := range c.AlwaysAllowRules {
		rules = append(rules, rs...)
	}
	return rules
}

// AllDenyRules returns all deny rules flattened from all sources.
func (c *ToolPermissionContext) AllDenyRules() []string {
	var rules []string
	for _, rs := range c.AlwaysDenyRules {
		rules = append(rules, rs...)
	}
	return rules
}

// ApplyPermissionUpdate applies a permission update to the context.
// Source: PermissionUpdate.ts:55-188
func (c *ToolPermissionContext) ApplyPermissionUpdate(update PermissionUpdate) {
	switch update.Type {
	case UpdateSetMode:
		c.Mode = update.Mode

	case UpdateAddRules:
		rulesMap := c.rulesMapForBehavior(update.Behavior)
		existing := rulesMap[update.Destination]
		rulesMap[update.Destination] = append(existing, update.Rules...)

	case UpdateReplaceRules:
		rulesMap := c.rulesMapForBehavior(update.Behavior)
		rulesMap[update.Destination] = update.Rules

	case UpdateRemoveRules:
		rulesMap := c.rulesMapForBehavior(update.Behavior)
		existing := rulesMap[update.Destination]
		toRemove := make(map[string]bool)
		for _, r := range update.Rules {
			toRemove[r] = true
		}
		var filtered []string
		for _, r := range existing {
			if !toRemove[r] {
				filtered = append(filtered, r)
			}
		}
		rulesMap[update.Destination] = filtered

	case UpdateAddDirectories:
		for _, dir := range update.Directories {
			c.AdditionalWorkingDirectories[dir] = string(update.Destination)
		}

	case UpdateRemoveDirectories:
		for _, dir := range update.Directories {
			delete(c.AdditionalWorkingDirectories, dir)
		}
	}
}

func (c *ToolPermissionContext) rulesMapForBehavior(b PermissionBehavior) map[PermissionUpdateDestination][]string {
	switch b {
	case BehaviorAllow:
		return c.AlwaysAllowRules
	case BehaviorDeny:
		return c.AlwaysDenyRules
	case BehaviorAsk:
		return c.AlwaysAskRules
	default:
		return c.AlwaysAllowRules
	}
}

// SupportsPersistence checks if a destination supports disk persistence.
// Source: PermissionUpdate.ts:208-216
func SupportsPersistence(dest PermissionUpdateDestination) bool {
	return dest == DestUserSettings || dest == DestProjectSettings || dest == DestLocalSettings
}

// PermissionRulePersister manages reading/writing permission rules to settings files.
type PermissionRulePersister struct {
	mu      sync.Mutex
	homeDir string
	cwd     string
}

// NewPermissionRulePersister creates a persister for the given directories.
func NewPermissionRulePersister(homeDir, cwd string) *PermissionRulePersister {
	return &PermissionRulePersister{homeDir: homeDir, cwd: cwd}
}

// PersistPermissionUpdate writes a permission update to the settings file.
// Source: PermissionUpdate.ts:222-325
func (p *PermissionRulePersister) PersistPermissionUpdate(update PermissionUpdate) error {
	if !SupportsPersistence(update.Destination) {
		return nil // session/cliArg rules are in-memory only
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	settingsPath := p.settingsPath(update.Destination)
	if settingsPath == "" {
		return fmt.Errorf("unknown destination: %s", update.Destination)
	}

	// Load existing settings
	existing := p.loadPermissions(settingsPath)

	switch update.Type {
	case UpdateAddRules:
		// Source: PermissionUpdate.ts:230-241
		rules := p.rulesForBehavior(existing, update.Behavior)
		// Deduplicate
		ruleSet := make(map[string]bool)
		for _, r := range rules {
			ruleSet[r] = true
		}
		for _, r := range update.Rules {
			if !ruleSet[r] {
				rules = append(rules, r)
				ruleSet[r] = true
			}
		}
		p.setRulesForBehavior(existing, update.Behavior, rules)

	case UpdateReplaceRules:
		p.setRulesForBehavior(existing, update.Behavior, update.Rules)

	case UpdateRemoveRules:
		// Source: PermissionUpdate.ts:268-294
		rules := p.rulesForBehavior(existing, update.Behavior)
		toRemove := make(map[string]bool)
		for _, r := range update.Rules {
			toRemove[r] = true
		}
		var filtered []string
		for _, r := range rules {
			if !toRemove[r] {
				filtered = append(filtered, r)
			}
		}
		p.setRulesForBehavior(existing, update.Behavior, filtered)

	case UpdateSetMode:
		// Source: PermissionUpdate.ts:317-325
		existing.DefaultMode = update.Mode

	case UpdateAddDirectories:
		// Source: PermissionUpdate.ts:244-266
		dirSet := make(map[string]bool)
		for _, d := range existing.AdditionalDirectories {
			dirSet[d] = true
		}
		for _, d := range update.Directories {
			if !dirSet[d] {
				existing.AdditionalDirectories = append(existing.AdditionalDirectories, d)
				dirSet[d] = true
			}
		}

	case UpdateRemoveDirectories:
		// Source: PermissionUpdate.ts:297-315
		toRemove := make(map[string]bool)
		for _, d := range update.Directories {
			toRemove[d] = true
		}
		var filtered []string
		for _, d := range existing.AdditionalDirectories {
			if !toRemove[d] {
				filtered = append(filtered, d)
			}
		}
		existing.AdditionalDirectories = filtered
	}

	return p.savePermissions(settingsPath, existing)
}

// LoadPermissionContext loads permission rules from all settings sources into a context.
func (p *PermissionRulePersister) LoadPermissionContext(mode string) *ToolPermissionContext {
	ctx := NewToolPermissionContext(mode)

	// Load from each settings source
	for _, dest := range []PermissionUpdateDestination{DestUserSettings, DestProjectSettings, DestLocalSettings} {
		path := p.settingsPath(dest)
		if path == "" {
			continue
		}
		perms := p.loadPermissions(path)
		if len(perms.Allow) > 0 {
			ctx.AlwaysAllowRules[dest] = perms.Allow
		}
		if len(perms.Deny) > 0 {
			ctx.AlwaysDenyRules[dest] = perms.Deny
		}
		if len(perms.Ask) > 0 {
			ctx.AlwaysAskRules[dest] = perms.Ask
		}
	}

	return ctx
}

func (p *PermissionRulePersister) settingsPath(dest PermissionUpdateDestination) string {
	switch dest {
	case DestUserSettings:
		return filepath.Join(p.homeDir, ".claude", "settings.json")
	case DestProjectSettings:
		if p.cwd == "" {
			return ""
		}
		return filepath.Join(p.cwd, ".claude", "settings.json")
	case DestLocalSettings:
		if p.cwd == "" {
			return ""
		}
		return filepath.Join(p.cwd, ".claude", "settings.local.json")
	default:
		return ""
	}
}

// settingsFile is the full settings.json structure (partial).
type settingsFile struct {
	Permissions *PermissionsConfig `json:"permissions,omitempty"`
	// Other fields preserved as raw JSON
	Other map[string]json.RawMessage `json:"-"`
}

func (p *PermissionRulePersister) loadPermissions(path string) *PermissionsConfig {
	data, err := os.ReadFile(path)
	if err != nil {
		return &PermissionsConfig{}
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return &PermissionsConfig{}
	}
	permsData, ok := raw["permissions"]
	if !ok {
		return &PermissionsConfig{}
	}
	var perms PermissionsConfig
	if err := json.Unmarshal(permsData, &perms); err != nil {
		return &PermissionsConfig{}
	}
	return &perms
}

func (p *PermissionRulePersister) savePermissions(path string, perms *PermissionsConfig) error {
	// Read existing file to preserve other fields
	var raw map[string]json.RawMessage

	data, err := os.ReadFile(path)
	if err == nil {
		json.Unmarshal(data, &raw)
	}
	if raw == nil {
		raw = make(map[string]json.RawMessage)
	}

	// Marshal permissions
	permsData, err := json.Marshal(perms)
	if err != nil {
		return fmt.Errorf("marshal permissions: %w", err)
	}
	raw["permissions"] = permsData

	// Write back
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("create dir: %w", err)
	}
	out, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal settings: %w", err)
	}
	return os.WriteFile(path, out, 0644)
}

func (p *PermissionRulePersister) rulesForBehavior(perms *PermissionsConfig, b PermissionBehavior) []string {
	switch b {
	case BehaviorAllow:
		return perms.Allow
	case BehaviorDeny:
		return perms.Deny
	case BehaviorAsk:
		return perms.Ask
	default:
		return nil
	}
}

func (p *PermissionRulePersister) setRulesForBehavior(perms *PermissionsConfig, b PermissionBehavior, rules []string) {
	switch b {
	case BehaviorAllow:
		perms.Allow = rules
	case BehaviorDeny:
		perms.Deny = rules
	case BehaviorAsk:
		perms.Ask = rules
	}
}
