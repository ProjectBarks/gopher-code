package prompt

import (
	"fmt"
	"runtime"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Constants — verbatim match against TS source
// ---------------------------------------------------------------------------

func TestSystemPromptDynamicBoundary_Verbatim(t *testing.T) {
	const want = "__SYSTEM_PROMPT_DYNAMIC_BOUNDARY__"
	if SystemPromptDynamicBoundary != want {
		t.Errorf("SystemPromptDynamicBoundary = %q, want %q", SystemPromptDynamicBoundary, want)
	}
}

func TestFrontierModelName_Verbatim(t *testing.T) {
	const want = "Claude Opus 4.6"
	if FrontierModelName != want {
		t.Errorf("FrontierModelName = %q, want %q", FrontierModelName, want)
	}
}

func TestClaudeCodeDocsMapURL_Verbatim(t *testing.T) {
	const want = "https://code.claude.com/docs/en/claude_code_docs_map.md"
	if ClaudeCodeDocsMapURL != want {
		t.Errorf("ClaudeCodeDocsMapURL = %q, want %q", ClaudeCodeDocsMapURL, want)
	}
}

func TestClaude46Or45ModelIDs_Verbatim(t *testing.T) {
	if Claude46Or45ModelIDs.Opus != "claude-opus-4-6" {
		t.Errorf("Opus = %q, want %q", Claude46Or45ModelIDs.Opus, "claude-opus-4-6")
	}
	if Claude46Or45ModelIDs.Sonnet != "claude-sonnet-4-6" {
		t.Errorf("Sonnet = %q, want %q", Claude46Or45ModelIDs.Sonnet, "claude-sonnet-4-6")
	}
	if Claude46Or45ModelIDs.Haiku != "claude-haiku-4-5-20251001" {
		t.Errorf("Haiku = %q, want %q", Claude46Or45ModelIDs.Haiku, "claude-haiku-4-5-20251001")
	}
}

func TestSummarizeToolResultsSection_Verbatim(t *testing.T) {
	const want = "When working with tool results, write down any important information you might need later in your response, as the original tool result may be cleared later."
	if SummarizeToolResultsSection != want {
		t.Errorf("SummarizeToolResultsSection mismatch:\ngot:  %q\nwant: %q", SummarizeToolResultsSection, want)
	}
}

func TestDefaultAgentPrompt_Verbatim(t *testing.T) {
	// Must contain key phrases from TS verbatim.
	mustContain := []string{
		"You are an agent for Claude Code, Anthropic's official CLI for Claude.",
		"Complete the task fully",
		"don't gold-plate, but don't leave it half-done",
		"respond with a concise report covering what was done and any key findings",
		"the caller will relay this to the user, so it only needs the essentials",
	}
	for _, phrase := range mustContain {
		if !strings.Contains(DefaultAgentPrompt, phrase) {
			t.Errorf("DefaultAgentPrompt missing phrase: %q", phrase)
		}
	}
}

func TestFastModeExplanation_Verbatim(t *testing.T) {
	want := fmt.Sprintf(
		"Fast mode for Claude Code uses the same %s model with faster output. It does NOT switch to a different model. It can be toggled with /fast.",
		FrontierModelName,
	)
	if FastModeExplanation != want {
		t.Errorf("FastModeExplanation mismatch:\ngot:  %q\nwant: %q", FastModeExplanation, want)
	}
}

// ---------------------------------------------------------------------------
// Section builders — key phrases
// ---------------------------------------------------------------------------

func TestHooksSection_Verbatim(t *testing.T) {
	got := HooksSection()
	mustContain := []string{
		"Users may configure 'hooks', shell commands that execute in response to events like tool calls, in settings.",
		"<user-prompt-submit-hook>",
		"ask the user to check their hooks configuration",
	}
	for _, phrase := range mustContain {
		if !strings.Contains(got, phrase) {
			t.Errorf("HooksSection missing phrase: %q", phrase)
		}
	}
}

func TestActionsSection_StartsWithHeading(t *testing.T) {
	got := ActionsSection()
	if !strings.HasPrefix(got, "# Executing actions with care") {
		t.Error("ActionsSection should start with '# Executing actions with care'")
	}
	mustContain := []string{
		"reversibility and blast radius",
		"measure twice, cut once",
		"Destructive operations:",
		"force-pushing",
	}
	for _, phrase := range mustContain {
		if !strings.Contains(got, phrase) {
			t.Errorf("ActionsSection missing phrase: %q", phrase)
		}
	}
}

func TestOutputEfficiencySection_StartsWithHeading(t *testing.T) {
	got := OutputEfficiencySection()
	if !strings.HasPrefix(got, "# Output efficiency") {
		t.Error("OutputEfficiencySection should start with '# Output efficiency'")
	}
	mustContain := []string{
		"Go straight to the point",
		"If you can say it in one sentence, don't use three",
		"This does not apply to code or tool calls.",
	}
	for _, phrase := range mustContain {
		if !strings.Contains(got, phrase) {
			t.Errorf("OutputEfficiencySection missing phrase: %q", phrase)
		}
	}
}

func TestToneAndStyleSection_StartsWithHeading(t *testing.T) {
	got := ToneAndStyleSection()
	if !strings.HasPrefix(got, "# Tone and style") {
		t.Error("ToneAndStyleSection should start with '# Tone and style'")
	}
	mustContain := []string{
		"Only use emojis if the user explicitly requests it",
		"file_path:line_number",
		"owner/repo#123",
		`"Let me read the file." with a period`,
	}
	for _, phrase := range mustContain {
		if !strings.Contains(got, phrase) {
			t.Errorf("ToneAndStyleSection missing phrase: %q", phrase)
		}
	}
}

// ---------------------------------------------------------------------------
// PrependBullets
// ---------------------------------------------------------------------------

func TestPrependBullets_Flat(t *testing.T) {
	got := PrependBullets([]string{"alpha", "beta"})
	if len(got) != 2 {
		t.Fatalf("expected 2 items, got %d", len(got))
	}
	if got[0] != " - alpha" {
		t.Errorf("got[0] = %q, want %q", got[0], " - alpha")
	}
	if got[1] != " - beta" {
		t.Errorf("got[1] = %q, want %q", got[1], " - beta")
	}
}

func TestPrependBulletsNested_Mixed(t *testing.T) {
	items := []any{
		"top-level",
		[]string{"nested-a", "nested-b"},
		"another-top",
	}
	got := PrependBulletsNested(items)
	want := []string{
		" - top-level",
		"  - nested-a",
		"  - nested-b",
		" - another-top",
	}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("got[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestPrependBullets_Empty(t *testing.T) {
	got := PrependBullets(nil)
	if len(got) != 0 {
		t.Errorf("expected empty, got %d items", len(got))
	}
}

// ---------------------------------------------------------------------------
// GetKnowledgeCutoff
// ---------------------------------------------------------------------------

func TestGetKnowledgeCutoff_KnownModels(t *testing.T) {
	tests := []struct {
		modelID string
		want    string
	}{
		{"claude-sonnet-4-6-20260401", "August 2025"},
		{"claude-opus-4-6-20260401", "May 2025"},
		{"claude-opus-4-5-20250401", "May 2025"},
		{"claude-haiku-4-5-20251001", "February 2025"},
		{"claude-opus-4-20250514", "January 2025"},
		{"claude-sonnet-4-20250514", "January 2025"},
	}
	for _, tt := range tests {
		t.Run(tt.modelID, func(t *testing.T) {
			got := GetKnowledgeCutoff(tt.modelID)
			if got != tt.want {
				t.Errorf("GetKnowledgeCutoff(%q) = %q, want %q", tt.modelID, got, tt.want)
			}
		})
	}
}

func TestGetKnowledgeCutoff_Unknown(t *testing.T) {
	got := GetKnowledgeCutoff("gpt-4o")
	if got != "" {
		t.Errorf("GetKnowledgeCutoff(unknown) = %q, want empty", got)
	}
}

// ---------------------------------------------------------------------------
// GetShellInfoLine
// ---------------------------------------------------------------------------

func TestGetShellInfoLine_ContainsShell(t *testing.T) {
	got := GetShellInfoLine()
	if !strings.HasPrefix(got, "Shell: ") {
		t.Errorf("GetShellInfoLine() = %q, expected 'Shell: ' prefix", got)
	}
}

// ---------------------------------------------------------------------------
// GetUnameSR
// ---------------------------------------------------------------------------

func TestGetUnameSR_NonEmpty(t *testing.T) {
	got := GetUnameSR()
	if got == "" {
		t.Error("GetUnameSR() returned empty string")
	}
	// On macOS it should contain "Darwin", on Linux "Linux".
	if runtime.GOOS == "darwin" && !strings.Contains(got, "Darwin") {
		t.Errorf("on darwin, GetUnameSR() = %q, expected to contain 'Darwin'", got)
	}
	if runtime.GOOS == "linux" && !strings.Contains(got, "Linux") {
		t.Errorf("on linux, GetUnameSR() = %q, expected to contain 'Linux'", got)
	}
}

// ---------------------------------------------------------------------------
// ComputeSimpleEnvInfo
// ---------------------------------------------------------------------------

func TestComputeSimpleEnvInfo_Structure(t *testing.T) {
	got := ComputeSimpleEnvInfo("/tmp/test", "claude-opus-4-6[1m]", true, false, nil)

	mustContain := []string{
		"# Environment",
		"You have been invoked in the following environment:",
		"Primary working directory: /tmp/test",
		"Is a git repository: true",
		fmt.Sprintf("Platform: %s", runtime.GOOS),
		"Shell: ",
		"OS Version: ",
		"claude-opus-4-6",
		"Assistant knowledge cutoff is May 2025.",
		"The most recent Claude model family is Claude 4.5/4.6.",
		fmt.Sprintf("Opus 4.6: '%s'", Claude46Or45ModelIDs.Opus),
		"Claude Code is available as a CLI in the terminal",
		"Fast mode for Claude Code",
	}
	for _, phrase := range mustContain {
		if !strings.Contains(got, phrase) {
			t.Errorf("ComputeSimpleEnvInfo missing phrase: %q\nin:\n%s", phrase, got)
		}
	}
}

func TestComputeSimpleEnvInfo_Worktree(t *testing.T) {
	got := ComputeSimpleEnvInfo("/tmp/wt", "claude-opus-4-6", true, true, nil)
	if !strings.Contains(got, "git worktree") {
		t.Error("ComputeSimpleEnvInfo with worktree should contain worktree guidance")
	}
}

func TestComputeSimpleEnvInfo_AdditionalDirs(t *testing.T) {
	got := ComputeSimpleEnvInfo("/tmp", "claude-opus-4-6", false, false, []string{"/extra/a", "/extra/b"})
	if !strings.Contains(got, "Additional working directories:") {
		t.Error("ComputeSimpleEnvInfo should show additional dirs header")
	}
	if !strings.Contains(got, "/extra/a") || !strings.Contains(got, "/extra/b") {
		t.Error("ComputeSimpleEnvInfo should list additional directories")
	}
}

// ---------------------------------------------------------------------------
// ComputeEnvInfo
// ---------------------------------------------------------------------------

func TestComputeEnvInfo_Structure(t *testing.T) {
	got := ComputeEnvInfo("/tmp/test", "claude-opus-4-6[1m]", true, nil)

	mustContain := []string{
		"<env>",
		"</env>",
		"Working directory: /tmp/test",
		"Is directory a git repo: Yes",
		fmt.Sprintf("Platform: %s", runtime.GOOS),
		"Shell: ",
		"OS Version: ",
		"Assistant knowledge cutoff is May 2025.",
	}
	for _, phrase := range mustContain {
		if !strings.Contains(got, phrase) {
			t.Errorf("ComputeEnvInfo missing phrase: %q", phrase)
		}
	}
}

func TestComputeEnvInfo_NotGit(t *testing.T) {
	got := ComputeEnvInfo("/tmp", "claude-opus-4-6", false, nil)
	if !strings.Contains(got, "Is directory a git repo: No") {
		t.Error("ComputeEnvInfo should show 'No' when not git")
	}
}

func TestComputeEnvInfo_AdditionalDirs(t *testing.T) {
	got := ComputeEnvInfo("/tmp", "claude-opus-4-6", false, []string{"/a", "/b"})
	if !strings.Contains(got, "Additional working directories: /a, /b") {
		t.Errorf("ComputeEnvInfo should include additional dirs, got:\n%s", got)
	}
}

func TestComputeEnvInfo_UnknownModelNoCutoff(t *testing.T) {
	got := ComputeEnvInfo("/tmp", "gpt-4o", false, nil)
	if strings.Contains(got, "knowledge cutoff") {
		t.Error("ComputeEnvInfo should not include cutoff for unknown models")
	}
}

// ---------------------------------------------------------------------------
// EnhanceSystemPromptWithEnvDetails
// ---------------------------------------------------------------------------

func TestEnhanceSystemPromptWithEnvDetails_AppendsNotesAndEnv(t *testing.T) {
	base := []string{"You are an agent."}
	got := EnhanceSystemPromptWithEnvDetails(base, "claude-opus-4-6", "/tmp", true, nil)

	if len(got) != 3 {
		t.Fatalf("expected 3 elements, got %d", len(got))
	}
	if got[0] != "You are an agent." {
		t.Error("first element should be original prompt")
	}
	if !strings.Contains(got[1], "Agent threads always have their cwd reset") {
		t.Error("second element should be the notes block")
	}
	if !strings.Contains(got[1], "MUST avoid using emojis") {
		t.Error("notes should contain emoji instruction")
	}
	if !strings.Contains(got[2], "<env>") {
		t.Error("third element should be env info")
	}
}

func TestEnhanceSystemPromptWithEnvDetails_DoesNotMutateInput(t *testing.T) {
	base := []string{"prompt"}
	original := make([]string, len(base))
	copy(original, base)

	EnhanceSystemPromptWithEnvDetails(base, "claude-opus-4-6", "/tmp", true, nil)

	if len(base) != len(original) {
		t.Error("EnhanceSystemPromptWithEnvDetails should not mutate input slice")
	}
}
