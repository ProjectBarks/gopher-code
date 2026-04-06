package tools_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/projectbarks/gopher-code/pkg/tools"
)

// ---------------------------------------------------------------------------
// Helper: create a fresh tool set and return the three tools by name.
// ---------------------------------------------------------------------------
func newCronToolSet(t *testing.T) (create, del, list tools.Tool) {
	t.Helper()
	cronTools := tools.NewCronTools()
	for _, ct := range cronTools {
		switch ct.Name() {
		case "CronCreate":
			create = ct
		case "CronDelete":
			del = ct
		case "CronList":
			list = ct
		}
	}
	if create == nil || del == nil || list == nil {
		t.Fatal("NewCronTools must return CronCreate, CronDelete, and CronList")
	}
	return
}

// exec is a shorthand for tool.Execute in tests.
func exec(t *testing.T, tool tools.Tool, inputJSON string) *tools.ToolOutput {
	t.Helper()
	out, err := tool.Execute(context.Background(), nil, json.RawMessage(inputJSON))
	if err != nil {
		t.Fatalf("unexpected Go error from %s: %v", tool.Name(), err)
	}
	return out
}

// extractID pulls the cron-N ID out of a CronCreate success message.
func extractID(t *testing.T, content string) string {
	t.Helper()
	parts := strings.Fields(content)
	for i, p := range parts {
		if (p == "job" || p == "task") && i+1 < len(parts) {
			return strings.TrimRight(parts[i+1], ":()")
		}
	}
	t.Fatalf("could not extract ID from: %s", content)
	return ""
}

// ---------------------------------------------------------------------------
// Tool metadata
// ---------------------------------------------------------------------------
func TestCronTools(t *testing.T) {
	create, del, list := newCronToolSet(t)

	t.Run("names", func(t *testing.T) {
		if create.Name() != "CronCreate" {
			t.Errorf("got %q", create.Name())
		}
		if del.Name() != "CronDelete" {
			t.Errorf("got %q", del.Name())
		}
		if list.Name() != "CronList" {
			t.Errorf("got %q", list.Name())
		}
	})

	t.Run("read_only", func(t *testing.T) {
		if create.IsReadOnly() {
			t.Error("CronCreate should not be read-only")
		}
		if del.IsReadOnly() {
			t.Error("CronDelete should not be read-only")
		}
		if !list.IsReadOnly() {
			t.Error("CronList should be read-only")
		}
	})

	t.Run("returns_three_tools", func(t *testing.T) {
		if n := len(tools.NewCronTools()); n != 3 {
			t.Errorf("expected 3, got %d", n)
		}
	})

	t.Run("schemas_valid_json", func(t *testing.T) {
		for _, tool := range tools.NewCronTools() {
			var m map[string]interface{}
			if err := json.Unmarshal(tool.InputSchema(), &m); err != nil {
				t.Fatalf("%s schema invalid: %v", tool.Name(), err)
			}
		}
	})

	// Source: prompt.ts — descriptions must match TS verbatim strings
	t.Run("descriptions", func(t *testing.T) {
		wantCreate := "Schedule a prompt to run at a future time within this Claude session \u2014 either recurring on a cron schedule, or once at a specific time."
		if got := create.Description(); got != wantCreate {
			t.Errorf("CronCreate description:\n  got:  %q\n  want: %q", got, wantCreate)
		}
		if got := del.Description(); got != "Cancel a scheduled cron job by ID" {
			t.Errorf("CronDelete description: got %q", got)
		}
		if got := list.Description(); got != "List scheduled cron jobs" {
			t.Errorf("CronList description: got %q", got)
		}
	})

	// ShouldDefer — all three tools are deferred
	t.Run("should_defer", func(t *testing.T) {
		for _, tool := range tools.NewCronTools() {
			d, ok := tool.(interface{ ShouldDefer() bool })
			if !ok {
				t.Errorf("%s does not implement ShouldDefer", tool.Name())
				continue
			}
			if !d.ShouldDefer() {
				t.Errorf("%s.ShouldDefer() = false", tool.Name())
			}
		}
	})

	// SearchHint — Source: CronCreateTool.ts:58, CronDeleteTool.ts:38, CronListTool.ts:39
	t.Run("search_hints", func(t *testing.T) {
		hints := map[string]string{
			"CronCreate": "schedule a recurring or one-shot prompt",
			"CronDelete": "cancel a scheduled cron job",
			"CronList":   "list active cron jobs",
		}
		for _, tool := range tools.NewCronTools() {
			h, ok := tool.(interface{ SearchHint() string })
			if !ok {
				t.Errorf("%s does not implement SearchHint", tool.Name())
				continue
			}
			if got := h.SearchHint(); got != hints[tool.Name()] {
				t.Errorf("%s.SearchHint() = %q, want %q", tool.Name(), got, hints[tool.Name()])
			}
		}
	})

	// Prompt — all three provide system-prompt guidance
	t.Run("prompts_nonempty", func(t *testing.T) {
		for _, tool := range tools.NewCronTools() {
			p, ok := tool.(interface{ Prompt() string })
			if !ok {
				t.Errorf("%s does not implement Prompt", tool.Name())
				continue
			}
			if p.Prompt() == "" {
				t.Errorf("%s.Prompt() is empty", tool.Name())
			}
		}
	})

	// CronCreate prompt contains stampede-avoidance guidance (load-bearing)
	t.Run("create_prompt_stampede_avoidance", func(t *testing.T) {
		p := create.(interface{ Prompt() string }).Prompt()
		needles := []string{
			"Avoid the :00 and :30 minute marks",
			`"57 8 * * *"`,
			`"3 9 * * *"`,
			`"7 * * * *"`,
			"auto-expire after 7 days",
		}
		for _, n := range needles {
			if !strings.Contains(p, n) {
				t.Errorf("CronCreate prompt missing %q", n)
			}
		}
	})

	// CronList implements ConcurrencySafeChecker
	t.Run("list_concurrency_safe", func(t *testing.T) {
		cs, ok := list.(interface{ IsConcurrencySafe(json.RawMessage) bool })
		if !ok {
			t.Fatal("CronList does not implement IsConcurrencySafe")
		}
		if !cs.IsConcurrencySafe(nil) {
			t.Error("CronList.IsConcurrencySafe should return true")
		}
	})
}

// ---------------------------------------------------------------------------
// ParseCronExpression — validation
// ---------------------------------------------------------------------------
func TestParseCronExpression(t *testing.T) {
	valid := []string{
		"*/5 * * * *",
		"0 9 * * 1-5",
		"30 14 28 2 *",
		"0 0 1 1 *",
		"0,15,30,45 * * * *",
		"0 */2 * * *",
		"0 9 * * 0",   // Sunday
		"0 9 * * 7",   // Sunday alias
		"0 9 * * 1,3", // Mon,Wed
	}
	for _, expr := range valid {
		if f := tools.ParseCronExpression(expr); f == nil {
			t.Errorf("ParseCronExpression(%q) = nil, expected valid", expr)
		}
	}

	invalid := []string{
		"",
		"* * *",
		"* * * * * *",       // 6 fields
		"60 * * * *",        // minute 60 out of range
		"* 24 * * *",        // hour 24 out of range
		"* * 0 * *",         // day-of-month 0 out of range
		"* * * 13 *",        // month 13 out of range
		"* * * * 8",         // day-of-week 8 out of range
		"abc * * * *",       // non-numeric
		"*/0 * * * *",       // step 0
		"5-3 * * * *",       // lo > hi
		"* * * * * extra",   // extra text
	}
	for _, expr := range invalid {
		if f := tools.ParseCronExpression(expr); f != nil {
			t.Errorf("ParseCronExpression(%q) = non-nil, expected nil (invalid)", expr)
		}
	}
}

// ---------------------------------------------------------------------------
// CronToHuman — human-readable formatting
// ---------------------------------------------------------------------------
func TestCronToHuman(t *testing.T) {
	tests := []struct {
		cron string
		want string
	}{
		{"*/5 * * * *", "Every 5 minutes"},
		{"*/1 * * * *", "Every minute"},
		{"0 * * * *", "Every hour"},
		{"7 * * * *", "Every hour at :07"},
		{"0 */2 * * *", "Every 2 hours"},
		{"15 */3 * * *", "Every 3 hours at :15"},
		// The day/time cases use formatLocalTime which depends on locale,
		// so just check they contain the day name or "Weekdays"
	}
	for _, tc := range tests {
		if got := tools.CronToHuman(tc.cron); got != tc.want {
			t.Errorf("CronToHuman(%q) = %q, want %q", tc.cron, got, tc.want)
		}
	}

	// Daily at 9am — should contain "Every day at"
	daily := tools.CronToHuman("0 9 * * *")
	if !strings.HasPrefix(daily, "Every day at ") {
		t.Errorf("CronToHuman(0 9 * * *) = %q, expected 'Every day at ...'", daily)
	}

	// Weekdays — should start with "Weekdays at"
	wd := tools.CronToHuman("0 9 * * 1-5")
	if !strings.HasPrefix(wd, "Weekdays at ") {
		t.Errorf("CronToHuman(0 9 * * 1-5) = %q, expected 'Weekdays at ...'", wd)
	}

	// Specific day of week — Sunday
	sun := tools.CronToHuman("0 9 * * 0")
	if !strings.Contains(sun, "Sunday") {
		t.Errorf("CronToHuman(0 9 * * 0) = %q, expected to contain 'Sunday'", sun)
	}

	// Fallthrough: complex expression returns raw
	raw := tools.CronToHuman("0,30 9-17 * * 1-5")
	if raw != "0,30 9-17 * * 1-5" {
		t.Errorf("expected fallthrough to raw, got %q", raw)
	}
}

// ---------------------------------------------------------------------------
// CronCreate: validation errors (errorCodes 1-3)
// ---------------------------------------------------------------------------
func TestCronCreate_ValidationErrors(t *testing.T) {
	create, _, _ := newCronToolSet(t)

	t.Run("errorCode1_invalid_cron_expression", func(t *testing.T) {
		out := exec(t, create, `{"cron": "bad cron", "prompt": "test"}`)
		if !out.IsError {
			t.Fatal("expected error for invalid cron")
		}
		if !strings.Contains(out.Content, "Invalid cron expression 'bad cron'") {
			t.Errorf("wrong error message: %q", out.Content)
		}
		if !strings.Contains(out.Content, "Expected 5 fields: M H DoM Mon DoW") {
			t.Errorf("missing field guidance: %q", out.Content)
		}
	})

	t.Run("errorCode1_wrong_field_count", func(t *testing.T) {
		out := exec(t, create, `{"cron": "* * *", "prompt": "test"}`)
		if !out.IsError {
			t.Fatal("expected error for 3-field cron")
		}
		if !strings.Contains(out.Content, "Invalid cron expression") {
			t.Errorf("wrong error: %q", out.Content)
		}
	})

	t.Run("errorCode2_dead_cron_feb30", func(t *testing.T) {
		// Feb 30 never exists — dead cron
		out := exec(t, create, `{"cron": "0 0 30 2 *", "prompt": "test"}`)
		if !out.IsError {
			t.Fatal("expected error for dead cron (Feb 30)")
		}
		if !strings.Contains(out.Content, "does not match any calendar date in the next year") {
			t.Errorf("wrong error: %q", out.Content)
		}
	})

	t.Run("errorCode3_max_jobs", func(t *testing.T) {
		// Create a fresh set and fill to MaxCronJobs
		create2, _, _ := newCronToolSet(t)
		for i := 0; i < tools.MaxCronJobs; i++ {
			out := exec(t, create2, fmt.Sprintf(`{"cron": "*/5 * * * *", "prompt": "job %d"}`, i))
			if out.IsError {
				t.Fatalf("failed to create job %d: %s", i, out.Content)
			}
		}
		// The 51st should fail
		out := exec(t, create2, `{"cron": "*/5 * * * *", "prompt": "one too many"}`)
		if !out.IsError {
			t.Fatal("expected error at max jobs cap")
		}
		if !strings.Contains(out.Content, "Too many scheduled jobs (max 50)") {
			t.Errorf("wrong error: %q", out.Content)
		}
	})

	t.Run("empty_cron", func(t *testing.T) {
		out := exec(t, create, `{"cron": "", "prompt": "test"}`)
		if !out.IsError {
			t.Fatal("expected error for empty cron")
		}
	})

	t.Run("empty_prompt", func(t *testing.T) {
		out := exec(t, create, `{"cron": "* * * * *", "prompt": ""}`)
		if !out.IsError {
			t.Fatal("expected error for empty prompt")
		}
	})

	t.Run("invalid_json", func(t *testing.T) {
		out := exec(t, create, `{bad}`)
		if !out.IsError {
			t.Fatal("expected error for invalid JSON")
		}
	})
}

// ---------------------------------------------------------------------------
// CronCreate: success output uses humanSchedule
// ---------------------------------------------------------------------------
func TestCronCreate_HumanScheduleInOutput(t *testing.T) {
	create, _, _ := newCronToolSet(t)

	out := exec(t, create, `{"cron": "*/5 * * * *", "prompt": "check status"}`)
	if out.IsError {
		t.Fatalf("unexpected error: %s", out.Content)
	}
	// Output should contain human schedule not raw cron
	if !strings.Contains(out.Content, "Every 5 minutes") {
		t.Errorf("expected humanSchedule 'Every 5 minutes' in output, got %q", out.Content)
	}
	if !strings.Contains(out.Content, "Scheduled recurring job") {
		t.Errorf("expected 'Scheduled recurring job' in output, got %q", out.Content)
	}
	if !strings.Contains(out.Content, "Auto-expires after 7 days") {
		t.Errorf("expected auto-expire notice, got %q", out.Content)
	}
	if !strings.Contains(out.Content, "Session-only") {
		t.Errorf("expected 'Session-only' (default durable=false), got %q", out.Content)
	}
}

func TestCronCreate_OneShotOutput(t *testing.T) {
	create, _, _ := newCronToolSet(t)

	out := exec(t, create, `{"cron": "30 14 15 6 *", "prompt": "deploy check", "recurring": false}`)
	if out.IsError {
		t.Fatalf("unexpected error: %s", out.Content)
	}
	if !strings.Contains(out.Content, "Scheduled one-shot task") {
		t.Errorf("expected 'Scheduled one-shot task', got %q", out.Content)
	}
	if !strings.Contains(out.Content, "fire once then auto-delete") {
		t.Errorf("expected one-shot wording, got %q", out.Content)
	}
}

func TestCronCreate_DurableOutput(t *testing.T) {
	create, _, _ := newCronToolSet(t)

	out := exec(t, create, `{"cron": "0 9 * * 1-5", "prompt": "standup", "durable": true}`)
	if out.IsError {
		t.Fatalf("unexpected error: %s", out.Content)
	}
	if !strings.Contains(out.Content, "Persisted to .claude/scheduled_tasks.json") {
		t.Errorf("expected durable persistence note, got %q", out.Content)
	}
}

// ---------------------------------------------------------------------------
// CronCreate + CronList round-trip
// ---------------------------------------------------------------------------
func TestCronCreateAndList(t *testing.T) {
	create, _, list := newCronToolSet(t)

	out := exec(t, create, `{"cron": "*/5 * * * *", "prompt": "check status"}`)
	if out.IsError {
		t.Fatalf("create failed: %s", out.Content)
	}
	id := extractID(t, out.Content)

	listOut := exec(t, list, `{}`)
	if listOut.IsError {
		t.Fatalf("list failed: %s", listOut.Content)
	}
	// Verbatim format: "{id} — {human} (recurring) [session-only]: {prompt}"
	if !strings.Contains(listOut.Content, id) {
		t.Errorf("list missing ID %s: %q", id, listOut.Content)
	}
	if !strings.Contains(listOut.Content, "Every 5 minutes") {
		t.Errorf("list missing humanSchedule: %q", listOut.Content)
	}
	if !strings.Contains(listOut.Content, "(recurring)") {
		t.Errorf("list missing (recurring) label: %q", listOut.Content)
	}
	if !strings.Contains(listOut.Content, "[session-only]") {
		t.Errorf("list missing [session-only] tag: %q", listOut.Content)
	}
	if !strings.Contains(listOut.Content, "check status") {
		t.Errorf("list missing prompt text: %q", listOut.Content)
	}
	// Verify em-dash separator
	if !strings.Contains(listOut.Content, "\u2014") {
		t.Errorf("list missing em-dash separator: %q", listOut.Content)
	}
}

// ---------------------------------------------------------------------------
// CronList: empty result uses verbatim string
// ---------------------------------------------------------------------------
func TestCronList_EmptyVerbatim(t *testing.T) {
	_, _, list := newCronToolSet(t)

	out := exec(t, list, `{}`)
	// Source: CronListTool.ts:93 — "No scheduled jobs."
	if out.Content != "No scheduled jobs." {
		t.Errorf("expected 'No scheduled jobs.', got %q", out.Content)
	}
}

// ---------------------------------------------------------------------------
// CronList: one-shot + durable labels
// ---------------------------------------------------------------------------
func TestCronList_OneShotAndDurableLabels(t *testing.T) {
	create, _, list := newCronToolSet(t)

	// One-shot session-only
	exec(t, create, `{"cron": "30 14 15 6 *", "prompt": "once", "recurring": false}`)
	// Recurring durable — should NOT have [session-only]
	exec(t, create, `{"cron": "*/5 * * * *", "prompt": "forever", "durable": true}`)

	out := exec(t, list, `{}`)
	lines := strings.Split(out.Content, "\n")
	if len(lines) < 2 {
		t.Fatalf("expected 2 lines, got %d: %q", len(lines), out.Content)
	}

	// Find the one-shot line
	var foundOneShot, foundDurable bool
	for _, line := range lines {
		if strings.Contains(line, "(one-shot)") {
			foundOneShot = true
			if !strings.Contains(line, "[session-only]") {
				t.Errorf("one-shot line missing [session-only]: %q", line)
			}
		}
		if strings.Contains(line, "forever") && strings.Contains(line, "(recurring)") {
			foundDurable = true
			if strings.Contains(line, "[session-only]") {
				t.Errorf("durable line should NOT have [session-only]: %q", line)
			}
		}
	}
	if !foundOneShot {
		t.Errorf("missing one-shot entry in list: %q", out.Content)
	}
	if !foundDurable {
		t.Errorf("missing durable entry in list: %q", out.Content)
	}
}

// ---------------------------------------------------------------------------
// CronList: 80-char prompt truncation
// ---------------------------------------------------------------------------
func TestCronList_PromptTruncation(t *testing.T) {
	create, _, list := newCronToolSet(t)

	longPrompt := strings.Repeat("a", 120)
	exec(t, create, fmt.Sprintf(`{"cron": "*/5 * * * *", "prompt": %q}`, longPrompt))

	out := exec(t, list, `{}`)
	// The full 120-char prompt should NOT appear
	if strings.Contains(out.Content, longPrompt) {
		t.Error("list should truncate prompts > 80 chars")
	}
	// Should end with "..."
	if !strings.Contains(out.Content, "...") {
		t.Error("truncated prompt should end with '...'")
	}
}

// ---------------------------------------------------------------------------
// CronDelete: create then delete, verify list empty
// ---------------------------------------------------------------------------
func TestCronCreateAndDelete(t *testing.T) {
	create, del, list := newCronToolSet(t)

	out := exec(t, create, `{"cron": "0 * * * *", "prompt": "hourly check"}`)
	id := extractID(t, out.Content)

	// Delete it
	delOut := exec(t, del, fmt.Sprintf(`{"id": %q}`, id))
	if delOut.IsError {
		t.Fatalf("delete failed: %s", delOut.Content)
	}
	// Source: CronDeleteTool.ts:87 — "Cancelled job {id}."
	want := fmt.Sprintf("Cancelled job %s.", id)
	if delOut.Content != want {
		t.Errorf("delete result:\n  got:  %q\n  want: %q", delOut.Content, want)
	}

	// List should be empty
	listOut := exec(t, list, `{}`)
	if listOut.Content != "No scheduled jobs." {
		t.Errorf("expected empty list after delete, got %q", listOut.Content)
	}
}

// ---------------------------------------------------------------------------
// CronDelete: verbatim error for nonexistent job
// ---------------------------------------------------------------------------
func TestCronDelete_NotFound(t *testing.T) {
	_, del, _ := newCronToolSet(t)

	out := exec(t, del, `{"id": "cron-999"}`)
	if !out.IsError {
		t.Fatal("expected error for nonexistent job")
	}
	// Source: CronDeleteTool.ts:65 — "No scheduled job with id '{id}'"
	if !strings.Contains(out.Content, "No scheduled job with id 'cron-999'") {
		t.Errorf("wrong error: %q", out.Content)
	}
}

func TestCronDelete_MissingID(t *testing.T) {
	_, del, _ := newCronToolSet(t)

	out := exec(t, del, `{"id": ""}`)
	if !out.IsError {
		t.Fatal("expected error for missing id")
	}
}

func TestCronDelete_InvalidJSON(t *testing.T) {
	_, del, _ := newCronToolSet(t)

	out := exec(t, del, `{bad}`)
	if !out.IsError {
		t.Fatal("expected error for invalid JSON")
	}
}

// ---------------------------------------------------------------------------
// ComputeNextCronRun — basic smoke test
// ---------------------------------------------------------------------------
func TestComputeNextCronRun(t *testing.T) {
	now := time.Now()

	// Every 5 minutes — should always find a next run
	fields := tools.ParseCronExpression("*/5 * * * *")
	if fields == nil {
		t.Fatal("failed to parse */5 * * * *")
	}
	next, ok := tools.ComputeNextCronRun(fields, now)
	if !ok {
		t.Fatal("expected next run for */5 * * * *")
	}
	if !next.After(now) {
		t.Errorf("next run %v should be after now %v", next, now)
	}
	if next.Minute()%5 != 0 {
		t.Errorf("expected minute divisible by 5, got %d", next.Minute())
	}

	// Feb 30 — should never match (dead cron)
	deadFields := tools.ParseCronExpression("0 0 30 2 *")
	if deadFields == nil {
		t.Fatal("failed to parse 0 0 30 2 *")
	}
	_, ok = tools.ComputeNextCronRun(deadFields, now)
	if ok {
		t.Error("expected no match for Feb 30 cron")
	}

	// Weekdays at 9am — next run should be on a weekday
	wdFields := tools.ParseCronExpression("0 9 * * 1-5")
	if wdFields == nil {
		t.Fatal("failed to parse 0 9 * * 1-5")
	}
	wdNext, ok := tools.ComputeNextCronRun(wdFields, now)
	if !ok {
		t.Fatal("expected next run for weekday cron")
	}
	wd := wdNext.Weekday()
	if wd == time.Sunday || wd == time.Saturday {
		t.Errorf("weekday cron fired on %s", wd)
	}
}
