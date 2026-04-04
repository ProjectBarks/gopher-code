package cli

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"encoding/json"

	"github.com/projectbarks/gopher-code/pkg/auth"
	"github.com/projectbarks/gopher-code/pkg/config"
	mcpPkg "github.com/projectbarks/gopher-code/pkg/mcp"
	"github.com/projectbarks/gopher-code/pkg/message"
	"github.com/projectbarks/gopher-code/pkg/permissions"
	"github.com/projectbarks/gopher-code/pkg/provider"
	"github.com/projectbarks/gopher-code/pkg/query"
	"github.com/projectbarks/gopher-code/pkg/session"
	"github.com/projectbarks/gopher-code/pkg/skills"
	"github.com/projectbarks/gopher-code/pkg/tools"
)

// Version is the current gopher-code version (kept in sync with main).
const Version = "0.2.0"

// RunREPL starts an interactive read-eval-print loop.
func RunREPL(ctx context.Context, sess *session.SessionState, prov provider.ModelProvider, registry *tools.ToolRegistry, verbose bool, hookRunner tools.HookRunner, noSessionPersist bool, prefill string, planState *tools.PlanState) {
	scanner := bufio.NewScanner(os.Stdin)
	orchestrator := tools.NewOrchestrator(registry)
	if hookRunner != nil {
		orchestrator.SetHookRunner(hookRunner)
	}

	resuming := sess.TurnCount > 0
	fmt.Printf("\n\033[1mgopher-code\033[0m v%s\n", Version)
	fmt.Printf("Model: \033[36m%s\033[0m\n", sess.Config.Model)
	fmt.Printf("CWD:   %s\n", sess.CWD)
	if resuming {
		fmt.Printf("Session: %s (resumed, %d turns)\n", sess.ID[:8], sess.TurnCount)
	} else {
		fmt.Printf("Session: %s\n", sess.ID[:8])
	}
	fmt.Println()
	fmt.Println("Type your message or /help for commands.")
	fmt.Println()

	// Show prefill if provided (user must press enter to submit)
	if prefill != "" {
		fmt.Printf("> %s", prefill)
	}

	for {
		if prefill == "" {
			fmt.Print("> ")
		} else {
			prefill = "" // Only show prefill on the first prompt
		}
		if !scanner.Scan() {
			break
		}
		input := scanner.Text()

		// Multi-line: if line ends with \, continue reading
		for strings.HasSuffix(strings.TrimSpace(input), "\\") {
			input = strings.TrimSuffix(strings.TrimSpace(input), "\\")
			fmt.Print("... ")
			if !scanner.Scan() {
				break
			}
			input += "\n" + scanner.Text()
		}
		input = strings.TrimSpace(input)
		if input == "" {
			continue
		}

		// Slash commands
		switch {
		case input == "/exit" || input == "/quit":
			return
		case input == "/help":
			printHelp()
			continue
		case input == "/clear":
			sess.Messages = sess.Messages[:0]
			sess.TurnCount = 0
			fmt.Println("Conversation cleared.")
			continue
		case input == "/cost":
			costStr := provider.FormatCost(sess.TotalCostUSD)
			fmt.Printf("Cost: %s (input: %d, output: %d, cache read: %d, cache write: %d)\n",
				costStr, sess.TotalInputTokens, sess.TotalOutputTokens,
				sess.TotalCacheReadTokens, sess.TotalCacheCreationTokens)
			continue
		case input == "/compact":
			query.CompactSession(sess)
			fmt.Println("Session compacted.")
			continue
		case strings.HasPrefix(input, "/model"):
			parts := strings.Fields(input)
			if len(parts) > 1 {
				newModel := resolveModelAliasREPL(parts[1])
				sess.Config.Model = newModel
				costs := provider.GetModelCosts(newModel)
				fmt.Printf("Model set to: %s (%s)\n", newModel, provider.FormatModelPricing(costs))
				// Persist model change to user settings.
				// Source: state/onChangeAppState.ts lines 104-112
				settings := config.Load(sess.CWD)
				settings.Model = newModel
				if err := settings.Save(); err != nil {
					fmt.Fprintf(os.Stderr, "Warning: could not persist model to settings: %v\n", err)
				}
			} else {
				costs := provider.GetModelCosts(sess.Config.Model)
				fmt.Printf("Current model: %s (%s)\n", sess.Config.Model, provider.FormatModelPricing(costs))
			}
			continue
		case input == "/status":
			fmt.Printf("Session: %s\n", sess.ID)
			fmt.Printf("Model: %s\n", sess.Config.Model)
			fmt.Printf("CWD: %s\n", sess.CWD)
			fmt.Printf("Turns: %d / %d\n", sess.TurnCount, sess.Config.MaxTurns)
			fmt.Printf("Messages: %d\n", len(sess.Messages))
			fmt.Printf("Input tokens: %d\n", sess.TotalInputTokens)
			fmt.Printf("Output tokens: %d\n", sess.TotalOutputTokens)
			continue
		case input == "/context":
			budget := sess.Config.TokenBudget
			used := sess.TotalInputTokens
			total := budget.InputBudget()
			pct := 0.0
			if total > 0 {
				pct = float64(used) / float64(total) * 100
			}
			fmt.Printf("Context window: %d tokens\n", budget.ContextWindow)
			fmt.Printf("Input budget: %d tokens\n", total)
			fmt.Printf("Used: ~%d tokens (%.1f%%)\n", used, pct)
			fmt.Printf("Output limit: %d tokens\n", budget.MaxOutputTokens)
			if budget.ShouldCompact(used) {
				fmt.Println("Warning: approaching compact threshold")
			}
			continue
		case input == "/diff":
			cmd := exec.Command("git", "diff", "--stat")
			cmd.Dir = sess.CWD
			out, err := cmd.Output()
			if err != nil {
				fmt.Printf("Error: %v\n", err)
			} else if len(out) == 0 {
				fmt.Println("No changes")
			} else {
				fmt.Print(string(out))
			}
			continue
		case input == "/resume":
			metas, err := session.ListSessions()
			if err != nil || len(metas) == 0 {
				fmt.Println("No saved sessions found.")
			} else {
				fmt.Println("Saved sessions (most recent first):")
				limit := len(metas)
				if limit > 10 {
					limit = 10
				}
				for i := 0; i < limit; i++ {
					m := metas[i]
					fmt.Printf("  %s  %s  turns=%d  %s\n",
						m.ID[:8], m.CWD, m.TurnCount,
						m.UpdatedAt.Format("2006-01-02 15:04"))
				}
				fmt.Println("Use --resume <id> to resume a session.")
			}
			continue
		case input == "/save":
			if err := sess.Save(); err != nil {
				fmt.Fprintf(os.Stderr, "Save failed: %v\n", err)
			} else {
				fmt.Printf("Session %s saved.\n", sess.ID[:8])
			}
			continue
		case strings.HasPrefix(input, "/permissions"):
			parts := strings.Fields(input)
			if len(parts) > 1 {
				switch parts[1] {
				case "auto":
					sess.Config.PermissionMode = permissions.AutoApprove
					fmt.Println("Permission mode: auto-approve")
				case "deny":
					sess.Config.PermissionMode = permissions.Deny
					fmt.Println("Permission mode: deny all")
				case "interactive":
					sess.Config.PermissionMode = permissions.Interactive
					fmt.Println("Permission mode: interactive")
				default:
					fmt.Printf("Unknown mode: %s (use auto, deny, interactive)\n", parts[1])
				}
			} else {
				modes := map[permissions.PermissionMode]string{
					permissions.AutoApprove: "auto-approve",
					permissions.Deny:       "deny",
					permissions.Interactive: "interactive",
				}
				fmt.Printf("Permission mode: %s\n", modes[sess.Config.PermissionMode])
			}
			continue
		case strings.HasPrefix(input, "!"):
			cmd := strings.TrimSpace(strings.TrimPrefix(input, "!"))
			if cmd == "" {
				continue
			}
			shellCmd := exec.Command("sh", "-c", cmd)
			shellCmd.Dir = sess.CWD
			shellCmd.Stdout = os.Stdout
			shellCmd.Stderr = os.Stderr
			shellCmd.Stdin = os.Stdin
			shellCmd.Run()
			continue
		case input == "/doctor":
			runDoctor(sess)
			continue
		case input == "/config":
			cfg := config.Load(sess.CWD)
			data, _ := json.MarshalIndent(cfg, "", "  ")
			fmt.Println(string(data))
			continue
		case input == "/init":
			claudeDir := filepath.Join(sess.CWD, ".claude")
			if err := os.MkdirAll(claudeDir, 0755); err != nil {
				fmt.Fprintf(os.Stderr, "Error creating .claude directory: %v\n", err)
				continue
			}
			// Create CLAUDE.md if it doesn't exist
			claudeMD := filepath.Join(sess.CWD, "CLAUDE.md")
			if _, err := os.Stat(claudeMD); os.IsNotExist(err) {
				if err := os.WriteFile(claudeMD, []byte("# Project Instructions\n\nAdd project-specific instructions here.\n"), 0644); err != nil {
					fmt.Fprintf(os.Stderr, "Error creating CLAUDE.md: %v\n", err)
				} else {
					fmt.Printf("Created %s\n", claudeMD)
				}
			} else {
				fmt.Println("CLAUDE.md already exists")
			}
			// Create settings.json if it doesn't exist
			settingsPath := filepath.Join(claudeDir, "settings.json")
			if _, err := os.Stat(settingsPath); os.IsNotExist(err) {
				if err := os.WriteFile(settingsPath, []byte("{\n}\n"), 0644); err != nil {
					fmt.Fprintf(os.Stderr, "Error creating settings.json: %v\n", err)
				} else {
					fmt.Printf("Created %s\n", settingsPath)
				}
			}
			fmt.Println("Project initialized.")
			continue
		case strings.HasPrefix(input, "/export"):
			parts := strings.Fields(input)
			format := "json"
			if len(parts) > 1 {
				format = parts[1]
			}
			switch format {
			case "json":
				data, _ := json.MarshalIndent(sess.Messages, "", "  ")
				fmt.Println(string(data))
			case "markdown", "md":
				for _, msg := range sess.Messages {
					fmt.Printf("## %s\n\n", msg.Role)
					for _, block := range msg.Content {
						switch block.Type {
						case message.ContentText:
							fmt.Println(block.Text)
						case message.ContentToolUse:
							fmt.Printf("**Tool: %s**\n```json\n%s\n```\n", block.Name, string(block.Input))
						case message.ContentToolResult:
							fmt.Printf("**Result:**\n%s\n", block.Content)
						}
					}
					fmt.Println()
				}
			default:
				fmt.Printf("Unknown format: %s (use json or markdown)\n", format)
			}
			continue
		case input == "/usage":
			fmt.Println("Token Usage:")
			fmt.Printf("  Input:          %d tokens\n", sess.TotalInputTokens)
			fmt.Printf("  Output:         %d tokens\n", sess.TotalOutputTokens)
			fmt.Printf("  Cache creation: %d tokens\n", sess.TotalCacheCreationTokens)
			fmt.Printf("  Cache read:     %d tokens\n", sess.TotalCacheReadTokens)
			fmt.Printf("  Turns:          %d\n", sess.TurnCount)
			fmt.Printf("  Total cost:     %s\n", provider.FormatCost(sess.TotalCostUSD))
			if sess.TotalLinesAdded > 0 || sess.TotalLinesRemoved > 0 {
				fmt.Printf("  Lines changed:  +%d / -%d\n", sess.TotalLinesAdded, sess.TotalLinesRemoved)
			}
			// Per-model breakdown — Source: bootstrap/state.ts — modelUsage
			if len(sess.ModelUsage) > 0 {
				fmt.Println("  Per-model:")
				for model, entry := range sess.ModelUsage {
					modelCosts := provider.GetModelCosts(model)
					fmt.Printf("    %s (%s): in=%d out=%d cost=%s\n",
						model, provider.FormatModelPricing(modelCosts),
						entry.InputTokens, entry.OutputTokens,
						provider.FormatCost(entry.CostUSD))
				}
			}
			continue
		case input == "/session":
			fmt.Printf("Session ID:   %s\n", sess.ID)
			fmt.Printf("Created:      %s\n", sess.CreatedAt.Format(time.RFC3339))
			fmt.Printf("Model:        %s\n", sess.Config.Model)
			fmt.Printf("CWD:          %s\n", sess.CWD)
			fmt.Printf("Turns:        %d / %d\n", sess.TurnCount, sess.Config.MaxTurns)
			fmt.Printf("Messages:     %d\n", len(sess.Messages))
			if sess.Config.ThinkingEnabled {
				fmt.Printf("Thinking:     enabled (budget: %d)\n", sess.Config.ThinkingBudget)
			}
			continue
		case strings.HasPrefix(input, "/rename"):
			parts := strings.SplitN(input, " ", 2)
			if len(parts) < 2 || strings.TrimSpace(parts[1]) == "" {
				fmt.Println("Usage: /rename <name>")
			} else {
				sess.Name = strings.TrimSpace(parts[1])
				fmt.Printf("Session renamed to: %s\n", sess.Name)
			}
			continue
		case input == "/plan":
			if planState == nil {
				fmt.Println("Plan mode is not available.")
			} else if planState.InPlanMode {
				fmt.Println("Already in plan mode. The assistant will outline a plan before executing.")
			} else {
				planState.InPlanMode = true
				fmt.Println("Entering plan mode. The assistant will outline a plan before executing.")
			}
			continue
		case input == "/tasks":
			store := tools.GetTaskStoreFromRegistry(registry)
			if store == nil {
				fmt.Println("Task tracking is not available.")
			} else {
				fmt.Println(tools.FormatTaskList(store))
			}
			continue
		case input == "/login":
			fmt.Print("Enter your Anthropic API key: ")
			if scanner.Scan() {
				key := strings.TrimSpace(scanner.Text())
				if key == "" {
					fmt.Println("No key provided.")
				} else if err := auth.SaveAPIKey(key); err != nil {
					fmt.Printf("Error saving key: %v\n", err)
				} else {
					fmt.Println("API key saved to ~/.claude/auth.json")
				}
			}
			continue
		case input == "/logout":
			home, _ := os.UserHomeDir()
			os.Remove(filepath.Join(home, ".claude", "auth.json"))
			fmt.Println("Logged out. API key removed.")
			continue
		case input == "/skills":
			loaded := skills.LoadSkills(sess.CWD)
			if len(loaded) == 0 {
				fmt.Println("No skills found. Add .md files to ~/.claude/skills/ or .claude/skills/")
			} else {
				fmt.Println("Available skills:")
				for _, s := range loaded {
					fmt.Printf("  /%s - %s [%s]\n", s.Name, s.Description, s.Source)
				}
			}
			continue

		case input == "/memory":
			claudeMD := query.LoadClaudeMDPublic(sess.CWD)
			if claudeMD == "" {
				fmt.Println("No CLAUDE.md found. Use /init to create one.")
			} else {
				fmt.Println("=== CLAUDE.md ===")
				fmt.Println(claudeMD)
			}
			continue

		case input == "/mcp":
			fmt.Println("MCP servers:")
			mcpCfg, _ := mcpPkg.LoadConfig()
			if len(mcpCfg.Servers) == 0 {
				fmt.Println("  No servers configured. Add to ~/.claude/mcp.json")
			} else {
				for name, sc := range mcpCfg.Servers {
					fmt.Printf("  %s: %s %v\n", name, sc.Command, sc.Args)
				}
			}
			continue

		case input == "/hooks":
			settings := config.Load(sess.CWD)
			if len(settings.Hooks) == 0 {
				fmt.Println("No hooks configured. Add to ~/.claude/settings.json")
			} else {
				fmt.Println("Hooks:")
				for _, h := range settings.Hooks {
					fmt.Printf("  %s [%s]: %s\n", h.Type, h.Matcher, h.Command)
				}
			}
			continue

		case input == "/agents":
			fmt.Println("Agent types: general-purpose, explore, plan, verification")
			fmt.Println("Use the Agent tool to spawn sub-agents for parallel work.")
			continue

		case strings.HasPrefix(input, "/theme"):
			parts := strings.Fields(input)
			if len(parts) > 1 {
				fmt.Printf("Theme set to: %s\n", parts[1])
			} else {
				fmt.Println("Current theme: default")
				fmt.Println("Available: default, dark, light")
			}
			continue

		case strings.HasPrefix(input, "/effort"):
			parts := strings.Fields(input)
			if len(parts) > 1 {
				switch parts[1] {
				case "low":
					sess.Config.ThinkingEnabled = false
					fmt.Println("Effort: low (thinking disabled)")
				case "medium":
					sess.Config.ThinkingEnabled = true
					sess.Config.ThinkingBudget = 5000
					fmt.Println("Effort: medium (thinking budget: 5000)")
				case "high":
					sess.Config.ThinkingEnabled = true
					sess.Config.ThinkingBudget = 16000
					fmt.Println("Effort: high (thinking budget: 16000)")
				case "max":
					sess.Config.ThinkingEnabled = true
					sess.Config.ThinkingBudget = 32000
					fmt.Println("Effort: max (thinking budget: 32000)")
				default:
					fmt.Printf("Unknown effort level: %s (use low, medium, high, max)\n", parts[1])
				}
			} else {
				if sess.Config.ThinkingEnabled {
					fmt.Printf("Current effort: thinking enabled (budget: %d)\n", sess.Config.ThinkingBudget)
				} else {
					fmt.Println("Current effort: low (thinking disabled)")
				}
			}
			continue

		case input == "/files":
			entries, err := os.ReadDir(sess.CWD)
			if err != nil {
				fmt.Printf("Error: %v\n", err)
			} else {
				for _, e := range entries {
					if e.IsDir() {
						fmt.Printf("  %s/\n", e.Name())
					} else {
						fmt.Printf("  %s\n", e.Name())
					}
				}
			}
			continue
		}

		sess.PushMessage(message.UserMessage(input))

		// Create per-query context (Ctrl+C cancels query, not REPL)
		queryCtx, queryCancel := context.WithCancel(ctx)
		err := query.Query(queryCtx, sess, prov, registry, orchestrator, PrintEvent)
		queryCancel()

		if err != nil {
			fmt.Fprintf(os.Stderr, "\nError: %v\n", err)
		}

		// Auto-save after each exchange (unless disabled)
		if !noSessionPersist {
			if saveErr := sess.Save(); saveErr != nil {
				if verbose {
					fmt.Fprintf(os.Stderr, "Auto-save failed: %v\n", saveErr)
				}
			}
		}

		fmt.Println()
	}
}

// resolveModelAliasREPL maps short aliases to full model IDs.
// Source: cli.tsx model alias resolution
var modelAliasesREPL = map[string]string{
	"haiku":  "claude-haiku-4-5-20251001",
	"sonnet": "claude-sonnet-4-6",
	"opus":   "claude-opus-4-6",
}

func resolveModelAliasREPL(model string) string {
	if resolved, ok := modelAliasesREPL[model]; ok {
		return resolved
	}
	return model
}

func printHelp() {
	fmt.Println("Commands:")
	fmt.Println("  /help              Show this help")
	fmt.Println("  /clear             Clear conversation history")
	fmt.Println("  /compact           Compact conversation to save context")
	fmt.Println("  /cost              Show token usage")
	fmt.Println("  /usage             Detailed token usage and cost estimate")
	fmt.Println("  /context           Context window usage")
	fmt.Println("  /status            Session info")
	fmt.Println("  /session           Detailed session info")
	fmt.Println("  /model [name]      Show or set model")
	fmt.Println("  /effort [level]    Show or set effort (low/medium/high/max)")
	fmt.Println("  /permissions       Show or set permission mode")
	fmt.Println("  /diff              Git diff in CWD")
	fmt.Println("  /files             List files in CWD")
	fmt.Println("  /save              Save session")
	fmt.Println("  /resume            List saved sessions")
	fmt.Println("  /rename <name>     Rename session")
	fmt.Println("  /memory            Show loaded CLAUDE.md content")
	fmt.Println("  /init              Initialize .claude/ project config")
	fmt.Println("  /export [format]   Export conversation (json or markdown)")
	fmt.Println("  /doctor            System health check")
	fmt.Println("  /config            Show loaded settings")
	fmt.Println("  /plan              Enter plan mode")
	fmt.Println("  /tasks             Show task list")
	fmt.Println("  /skills            List available skills")
	fmt.Println("  /agents            Show agent types")
	fmt.Println("  /mcp               Show MCP server status")
	fmt.Println("  /hooks             Show configured hooks")
	fmt.Println("  /theme [name]      Show or set theme")
	fmt.Println("  /login             Save API key")
	fmt.Println("  /logout            Remove saved API key")
	fmt.Println("  /exit              Exit")
	fmt.Println()
	fmt.Println("  ! <command>        Run a shell command")
	fmt.Println("  line ending \\      Continue input on next line")
}

// runDoctor performs comprehensive system diagnostics.
// Source: screens/Doctor.tsx
func runDoctor(sess *session.SessionState) {
	fmt.Println("gopher-code doctor")
	fmt.Printf("Version: %s\n", Version)
	fmt.Println()

	// Section 1: Authentication
	fmt.Println("Authentication")
	if _, authErr := auth.GetAPIKey(); authErr == nil {
		fmt.Printf("  [ok] %s\n", auth.Status())
	} else {
		fmt.Println("  [missing] No API key configured")
	}
	fmt.Println()

	// Section 2: Tools
	fmt.Println("Tools")
	if rgPath, err := exec.LookPath("rg"); err == nil {
		// Check ripgrep version
		out, verErr := exec.Command(rgPath, "--version").Output()
		if verErr == nil {
			ver := strings.TrimSpace(strings.Split(string(out), "\n")[0])
			fmt.Printf("  [ok] ripgrep: %s (%s)\n", ver, rgPath)
		} else {
			fmt.Printf("  [ok] ripgrep found (%s)\n", rgPath)
		}
	} else {
		fmt.Println("  [warn] ripgrep (rg) not found (grep will use fallback)")
	}
	if gitPath, err := exec.LookPath("git"); err == nil {
		out, verErr := exec.Command(gitPath, "--version").Output()
		if verErr == nil {
			fmt.Printf("  [ok] %s (%s)\n", strings.TrimSpace(string(out)), gitPath)
		} else {
			fmt.Printf("  [ok] git found (%s)\n", gitPath)
		}
	} else {
		fmt.Println("  [missing] git not found")
	}
	fmt.Println()

	// Section 3: Environment
	fmt.Println("Environment")
	fmt.Printf("  [ok] Go %s\n", runtime.Version())
	fmt.Printf("  [ok] Platform: %s/%s\n", runtime.GOOS, runtime.GOARCH)

	// Check environment variable overrides — Source: Doctor.tsx env var validation
	for _, envVar := range []string{"BASH_MAX_OUTPUT_LENGTH", "TASK_MAX_OUTPUT_LENGTH", "CLAUDE_CODE_MAX_OUTPUT_TOKENS"} {
		if val := os.Getenv(envVar); val != "" {
			fmt.Printf("  [info] %s=%s\n", envVar, val)
		}
	}
	if apiURL := os.Getenv("ANTHROPIC_BASE_URL"); apiURL != "" {
		fmt.Printf("  [info] ANTHROPIC_BASE_URL=%s\n", apiURL)
	}
	fmt.Println()

	// Section 4: Configuration
	fmt.Println("Configuration")
	home, _ := os.UserHomeDir()
	globalSettings := filepath.Join(home, ".claude", "settings.json")
	projectSettings := filepath.Join(sess.CWD, ".claude", "settings.json")

	if _, err := os.Stat(globalSettings); err == nil {
		fmt.Printf("  [ok] Global settings: %s\n", globalSettings)
	} else {
		fmt.Printf("  [info] No global settings: %s\n", globalSettings)
	}
	if _, err := os.Stat(projectSettings); err == nil {
		fmt.Printf("  [ok] Project settings: %s\n", projectSettings)
	} else {
		fmt.Printf("  [info] No project settings: %s\n", projectSettings)
	}

	// Validate settings can parse without error
	cfg := config.Load(sess.CWD)
	if cfg.Model != "" {
		fmt.Printf("  [ok] Configured model: %s\n", cfg.Model)
	}
	fmt.Println()

	// Section 5: CLAUDE.md files — Source: Doctor.tsx context warnings
	fmt.Println("CLAUDE.md Files")
	claudeMDLocations := []string{
		filepath.Join(home, ".claude", "CLAUDE.md"),
		filepath.Join(sess.CWD, "CLAUDE.md"),
		filepath.Join(sess.CWD, ".claude", "CLAUDE.md"),
	}
	foundClaudeMD := false
	for _, path := range claudeMDLocations {
		if info, err := os.Stat(path); err == nil {
			foundClaudeMD = true
			size := info.Size()
			status := "ok"
			// Warn if CLAUDE.md is very large (>40k chars threshold from TS)
			if size > 40000 {
				status = "warn"
				fmt.Printf("  [%s] %s (%d bytes — large file may use significant context)\n", status, path, size)
			} else {
				fmt.Printf("  [%s] %s (%d bytes)\n", status, path, size)
			}
		}
	}
	if !foundClaudeMD {
		fmt.Println("  [info] No CLAUDE.md files found")
	}
	fmt.Println()

	// Section 6: Session info
	fmt.Println("Session")
	fmt.Printf("  Model: %s\n", sess.Config.Model)
	fmt.Printf("  CWD: %s\n", sess.CWD)
	if sess.OriginalCWD != "" && sess.OriginalCWD != sess.CWD {
		fmt.Printf("  Original CWD: %s\n", sess.OriginalCWD)
	}
	if sess.ProjectRoot != "" && sess.ProjectRoot != sess.CWD {
		fmt.Printf("  Project root: %s\n", sess.ProjectRoot)
	}
	duration := time.Since(sess.CreatedAt)
	fmt.Printf("  Uptime: %s\n", duration.Truncate(time.Second))
	modes := map[permissions.PermissionMode]string{
		permissions.AutoApprove: "auto-approve",
		permissions.Deny:       "deny",
		permissions.Interactive: "interactive",
	}
	fmt.Printf("  Permission mode: %s\n", modes[sess.Config.PermissionMode])
	if sess.Config.ThinkingEnabled {
		fmt.Printf("  Thinking: enabled (budget: %d tokens)\n", sess.Config.ThinkingBudget)
	}
	fmt.Println()
}
