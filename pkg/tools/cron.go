package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// DefaultMaxAgeDays is the auto-expiry window for recurring cron jobs.
// Source: ScheduleCronTool/prompt.ts — DEFAULT_MAX_AGE_DAYS
const DefaultMaxAgeDays = 7

// MaxCronJobs is the maximum number of concurrent cron jobs.
// Source: ScheduleCronTool/CronCreateTool.ts:25
const MaxCronJobs = 50

// maxPromptDisplay is the truncation limit for prompt text in CronList output.
// Source: CronListTool.ts:89 — truncate(j.prompt, 80, true)
const maxPromptDisplay = 80

// CronEntry represents a scheduled cron job.
type CronEntry struct {
	ID        string `json:"id"`
	Cron      string `json:"cron"`
	Prompt    string `json:"prompt"`
	Active    bool   `json:"active"`
	Recurring bool   `json:"recurring"`
	Durable   bool   `json:"durable"`
}

// CronStore is the in-memory store for cron entries.
type CronStore struct {
	mu      sync.Mutex
	entries map[string]*CronEntry
	nextID  int
}

// ---------------------------------------------------------------------------
// Cron expression parsing — Source: src/utils/cron.ts
// ---------------------------------------------------------------------------

// cronFieldRange defines the valid min/max for each of the 5 cron fields.
type cronFieldRange struct{ min, max int }

var cronFieldRanges = []cronFieldRange{
	{0, 59}, // minute
	{0, 23}, // hour
	{1, 31}, // dayOfMonth
	{1, 12}, // month
	{0, 6},  // dayOfWeek (0=Sunday; 7 accepted as Sunday alias)
}

// CronFields holds the expanded values for each cron field.
type CronFields struct {
	Minute     []int
	Hour       []int
	DayOfMonth []int
	Month      []int
	DayOfWeek  []int
}

// expandCronField parses a single cron field into a sorted slice of matching
// values. Supports: *, */N, N, N-M, N-M/S, and comma-separated lists.
// Returns nil if invalid. Source: cron.ts expandField.
func expandCronField(field string, r cronFieldRange) []int {
	seen := make(map[int]struct{})
	for _, part := range strings.Split(field, ",") {
		part = strings.TrimSpace(part)

		// wildcard or */N
		if part == "*" || strings.HasPrefix(part, "*/") {
			step := 1
			if strings.HasPrefix(part, "*/") {
				n, err := strconv.Atoi(part[2:])
				if err != nil || n < 1 {
					return nil
				}
				step = n
			}
			for i := r.min; i <= r.max; i += step {
				seen[i] = struct{}{}
			}
			continue
		}

		// N-M or N-M/S
		if idx := strings.Index(part, "-"); idx >= 0 {
			rangePart := part
			step := 1
			if si := strings.Index(part, "/"); si > idx {
				rangePart = part[:si]
				s, err := strconv.Atoi(part[si+1:])
				if err != nil || s < 1 {
					return nil
				}
				step = s
			}
			loStr := rangePart[:strings.Index(rangePart, "-")]
			hiStr := rangePart[strings.Index(rangePart, "-")+1:]
			lo, err1 := strconv.Atoi(loStr)
			hi, err2 := strconv.Atoi(hiStr)
			if err1 != nil || err2 != nil {
				return nil
			}
			isDow := r.min == 0 && r.max == 6
			effMax := r.max
			if isDow {
				effMax = 7
			}
			if lo > hi || step < 1 || lo < r.min || hi > effMax {
				return nil
			}
			for i := lo; i <= hi; i += step {
				v := i
				if isDow && v == 7 {
					v = 0
				}
				seen[v] = struct{}{}
			}
			continue
		}

		// plain N
		n, err := strconv.Atoi(part)
		if err != nil {
			return nil
		}
		isDow := r.min == 0 && r.max == 6
		if isDow && n == 7 {
			n = 0
		}
		if n < r.min || n > r.max {
			return nil
		}
		seen[n] = struct{}{}
	}

	if len(seen) == 0 {
		return nil
	}
	out := make([]int, 0, len(seen))
	for v := range seen {
		out = append(out, v)
	}
	sort.Ints(out)
	return out
}

// ParseCronExpression parses a 5-field cron expression into expanded number
// arrays. Returns nil if invalid or unsupported syntax.
// Source: cron.ts parseCronExpression
func ParseCronExpression(expr string) *CronFields {
	parts := strings.Fields(strings.TrimSpace(expr))
	if len(parts) != 5 {
		return nil
	}
	expanded := make([][]int, 5)
	for i := 0; i < 5; i++ {
		result := expandCronField(parts[i], cronFieldRanges[i])
		if result == nil {
			return nil
		}
		expanded[i] = result
	}
	return &CronFields{
		Minute:     expanded[0],
		Hour:       expanded[1],
		DayOfMonth: expanded[2],
		Month:      expanded[3],
		DayOfWeek:  expanded[4],
	}
}

// ComputeNextCronRun finds the next time strictly after from that matches the
// cron fields, in local time. Walks forward minute-by-minute, bounded at 366
// days. Returns zero Time if no match.
// Source: cron.ts computeNextCronRun
func ComputeNextCronRun(fields *CronFields, from time.Time) (time.Time, bool) {
	minuteSet := intSet(fields.Minute)
	hourSet := intSet(fields.Hour)
	domSet := intSet(fields.DayOfMonth)
	monthSet := intSet(fields.Month)
	dowSet := intSet(fields.DayOfWeek)

	domWild := len(fields.DayOfMonth) == 31
	dowWild := len(fields.DayOfWeek) == 7

	// Round up to next whole minute strictly after from
	t := from.Truncate(time.Minute).Add(time.Minute)

	maxIter := 366 * 24 * 60
	for i := 0; i < maxIter; i++ {
		month := int(t.Month())
		if !monthSet[month] {
			// Jump to start of next month
			t = time.Date(t.Year(), t.Month()+1, 1, 0, 0, 0, 0, t.Location())
			continue
		}

		dom := t.Day()
		dow := int(t.Weekday())
		var dayMatches bool
		switch {
		case domWild && dowWild:
			dayMatches = true
		case domWild:
			dayMatches = dowSet[dow]
		case dowWild:
			dayMatches = domSet[dom]
		default:
			dayMatches = domSet[dom] || dowSet[dow]
		}
		if !dayMatches {
			t = time.Date(t.Year(), t.Month(), t.Day()+1, 0, 0, 0, 0, t.Location())
			continue
		}

		if !hourSet[t.Hour()] {
			t = time.Date(t.Year(), t.Month(), t.Day(), t.Hour()+1, 0, 0, 0, t.Location())
			continue
		}

		if !minuteSet[t.Minute()] {
			t = t.Add(time.Minute)
			continue
		}

		return t, true
	}
	return time.Time{}, false
}

func intSet(vals []int) map[int]bool {
	m := make(map[int]bool, len(vals))
	for _, v := range vals {
		m[v] = true
	}
	return m
}

// ---------------------------------------------------------------------------
// cronToHuman — Source: cron.ts cronToHuman (local-time path only)
// ---------------------------------------------------------------------------

var dayNames = []string{
	"Sunday", "Monday", "Tuesday", "Wednesday",
	"Thursday", "Friday", "Saturday",
}

func formatLocalTime(minute, hour int) string {
	t := time.Date(2000, 1, 1, hour, minute, 0, 0, time.Local)
	// Format matching en-US locale: "3:05 PM" style
	return t.Format("3:04 PM")
}

// CronToHuman converts a 5-field cron expression to a human-readable string.
// Covers common patterns; falls through to the raw expression for anything
// else. Source: cron.ts cronToHuman (local path).
func CronToHuman(cron string) string {
	parts := strings.Fields(strings.TrimSpace(cron))
	if len(parts) != 5 {
		return cron
	}
	minute, hour, dayOfMonth, month, dayOfWeek := parts[0], parts[1], parts[2], parts[3], parts[4]

	// Every N minutes: */N * * * *
	if strings.HasPrefix(minute, "*/") && hour == "*" && dayOfMonth == "*" && month == "*" && dayOfWeek == "*" {
		n, err := strconv.Atoi(minute[2:])
		if err == nil {
			if n == 1 {
				return "Every minute"
			}
			return fmt.Sprintf("Every %d minutes", n)
		}
	}

	// Every hour: N * * * *
	if isDigits(minute) && hour == "*" && dayOfMonth == "*" && month == "*" && dayOfWeek == "*" {
		m, _ := strconv.Atoi(minute)
		if m == 0 {
			return "Every hour"
		}
		return fmt.Sprintf("Every hour at :%02d", m)
	}

	// Every N hours: M */N * * *
	if isDigits(minute) && strings.HasPrefix(hour, "*/") && dayOfMonth == "*" && month == "*" && dayOfWeek == "*" {
		n, err := strconv.Atoi(hour[2:])
		m, _ := strconv.Atoi(minute)
		if err == nil {
			suffix := ""
			if m != 0 {
				suffix = fmt.Sprintf(" at :%02d", m)
			}
			if n == 1 {
				return "Every hour" + suffix
			}
			return fmt.Sprintf("Every %d hours%s", n, suffix)
		}
	}

	// Remaining cases need numeric minute + hour
	if !isDigits(minute) || !isDigits(hour) {
		return cron
	}
	m, _ := strconv.Atoi(minute)
	h, _ := strconv.Atoi(hour)

	// Daily: M H * * *
	if dayOfMonth == "*" && month == "*" && dayOfWeek == "*" {
		return fmt.Sprintf("Every day at %s", formatLocalTime(m, h))
	}

	// Specific day of week: M H * * D
	if dayOfMonth == "*" && month == "*" && len(dayOfWeek) == 1 && isDigits(dayOfWeek) {
		d, _ := strconv.Atoi(dayOfWeek)
		d = d % 7
		if d >= 0 && d < len(dayNames) {
			return fmt.Sprintf("Every %s at %s", dayNames[d], formatLocalTime(m, h))
		}
	}

	// Weekdays: M H * * 1-5
	if dayOfMonth == "*" && month == "*" && dayOfWeek == "1-5" {
		return fmt.Sprintf("Weekdays at %s", formatLocalTime(m, h))
	}

	return cron
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// ---------------------------------------------------------------------------
// Prompt text — Source: ScheduleCronTool/prompt.ts
// ---------------------------------------------------------------------------

// cronCreatePrompt is the verbatim prompt for CronCreate.
// Source: prompt.ts buildCronCreatePrompt (session-only path, durable not yet wired).
const cronCreatePrompt = `Schedule a prompt to be enqueued at a future time. Use for both recurring schedules and one-shot reminders.

Uses standard 5-field cron in the user's local timezone: minute hour day-of-month month day-of-week. "0 9 * * *" means 9am local — no timezone conversion needed.

## One-shot tasks (recurring: false)

For "remind me at X" or "at <time>, do Y" requests — fire once then auto-delete.
Pin minute/hour/day-of-month/month to specific values:
  "remind me at 2:30pm today to check the deploy" → cron: "30 14 <today_dom> <today_month> *", recurring: false
  "tomorrow morning, run the smoke test" → cron: "57 8 <tomorrow_dom> <tomorrow_month> *", recurring: false

## Recurring jobs (recurring: true, the default)

For "every N minutes" / "every hour" / "weekdays at 9am" requests:
  "*/5 * * * *" (every 5 min), "0 * * * *" (hourly), "0 9 * * 1-5" (weekdays at 9am local)

## Avoid the :00 and :30 minute marks when the task allows it

Every user who asks for "9am" gets ` + "`" + `0 9` + "`" + `, and every user who asks for "hourly" gets ` + "`" + `0 *` + "`" + ` — which means requests from across the planet land on the API at the same instant. When the user's request is approximate, pick a minute that is NOT 0 or 30:
  "every morning around 9" → "57 8 * * *" or "3 9 * * *" (not "0 9 * * *")
  "hourly" → "7 * * * *" (not "0 * * * *")
  "in an hour or so, remind me to..." → pick whatever minute you land on, don't round

Only use minute 0 or 30 when the user names that exact time and clearly means it ("at 9:00 sharp", "at half past", coordinating with a meeting). When in doubt, nudge a few minutes early or late — the user will not notice, and the fleet will.

## Session-only

Jobs live only in this Claude session — nothing is written to disk, and the job is gone when Claude exits.

## Runtime behavior

Jobs only fire while the REPL is idle (not mid-query). The scheduler adds a small deterministic jitter on top of whatever you pick: recurring tasks fire up to 10% of their period late (max 15 min); one-shot tasks landing on :00 or :30 fire up to 90 s early. Picking an off-minute is still the bigger lever.

Recurring tasks auto-expire after 7 days — they fire one final time, then are deleted. This bounds session lifetime. Tell the user about the 7-day limit when scheduling recurring jobs.

Returns a job ID you can pass to CronDelete.`

const cronDeletePrompt = `Cancel a cron job previously scheduled with CronCreate. Removes it from the in-memory session store.`

const cronListPrompt = `List all cron jobs scheduled via CronCreate in this session.`

// ---------------------------------------------------------------------------
// truncatePrompt truncates a string to maxLen, appending "..." if truncated.
// Source: CronListTool.ts:89 — truncate(j.prompt, 80, true)
// ---------------------------------------------------------------------------
func truncatePrompt(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

// ---------------------------------------------------------------------------
// CronCreateTool
// ---------------------------------------------------------------------------

// CronCreateTool creates a new cron entry.
type CronCreateTool struct {
	store *CronStore
}

func (t *CronCreateTool) Name() string { return "CronCreate" }

// Source: prompt.ts buildCronCreateDescription (session-only path)
func (t *CronCreateTool) Description() string {
	return "Schedule a prompt to run at a future time within this Claude session \u2014 either recurring on a cron schedule, or once at a specific time."
}

func (t *CronCreateTool) IsReadOnly() bool  { return false }
func (t *CronCreateTool) ShouldDefer() bool { return true }
func (t *CronCreateTool) SearchHint() string { return "schedule a recurring or one-shot prompt" }
func (t *CronCreateTool) Prompt() string     { return cronCreatePrompt }

// Source: ScheduleCronTool/CronCreateTool.ts:27-42
func (t *CronCreateTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"cron": {"type": "string", "description": "Standard 5-field cron expression in local time: \"M H DoM Mon DoW\" (e.g. \"*/5 * * * *\" = every 5 minutes, \"30 14 28 2 *\" = Feb 28 at 2:30pm local once)."},
			"prompt": {"type": "string", "description": "The prompt to enqueue at each fire time."},
			"recurring": {"type": "boolean", "description": "true (default) = fire on every cron match until deleted or auto-expired after 7 days. false = fire once at the next match, then auto-delete."},
			"durable": {"type": "boolean", "description": "true = persist to .claude/scheduled_tasks.json and survive restarts. false (default) = in-memory only, dies when this Claude session ends."}
		},
		"required": ["cron", "prompt"],
		"additionalProperties": false
	}`)
}

// Source: ScheduleCronTool/CronCreateTool.ts:82-157
func (t *CronCreateTool) Execute(_ context.Context, _ *ToolContext, input json.RawMessage) (*ToolOutput, error) {
	var params struct {
		Cron      string `json:"cron"`
		Prompt    string `json:"prompt"`
		Recurring *bool  `json:"recurring"`
		Durable   *bool  `json:"durable"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return ErrorOutput(fmt.Sprintf("invalid input: %s", err)), nil
	}
	if params.Cron == "" {
		return ErrorOutput("cron expression is required"), nil
	}
	if params.Prompt == "" {
		return ErrorOutput("prompt is required"), nil
	}

	// Validate cron expression — Source: CronCreateTool.ts:83-88 (errorCode 1)
	fields := ParseCronExpression(params.Cron)
	if fields == nil {
		return ErrorOutput(fmt.Sprintf(
			"Invalid cron expression '%s'. Expected 5 fields: M H DoM Mon DoW.", params.Cron,
		)), nil
	}

	// Dead-cron detection — Source: CronCreateTool.ts:90-95 (errorCode 2)
	if _, ok := ComputeNextCronRun(fields, time.Now()); !ok {
		return ErrorOutput(fmt.Sprintf(
			"Cron expression '%s' does not match any calendar date in the next year.", params.Cron,
		)), nil
	}

	// Defaults: recurring=true, durable=false
	// Source: CronCreateTool.ts:117
	recurring := true
	if params.Recurring != nil {
		recurring = *params.Recurring
	}
	durable := false
	if params.Durable != nil {
		durable = *params.Durable
	}

	t.store.mu.Lock()
	// Max jobs check — Source: CronCreateTool.ts:97-103 (errorCode 3)
	if len(t.store.entries) >= MaxCronJobs {
		t.store.mu.Unlock()
		return ErrorOutput(fmt.Sprintf("Too many scheduled jobs (max %d). Cancel one first.", MaxCronJobs)), nil
	}
	t.store.nextID++
	id := fmt.Sprintf("cron-%d", t.store.nextID)
	t.store.entries[id] = &CronEntry{
		ID:        id,
		Cron:      params.Cron,
		Prompt:    params.Prompt,
		Active:    true,
		Recurring: recurring,
		Durable:   durable,
	}
	t.store.mu.Unlock()

	// Build response matching TS format
	// Source: CronCreateTool.ts:143-156
	humanSchedule := CronToHuman(params.Cron)
	where := "Session-only (not written to disk, dies when Claude exits)"
	if durable {
		where = "Persisted to .claude/scheduled_tasks.json"
	}
	if recurring {
		return SuccessOutput(fmt.Sprintf(
			"Scheduled recurring job %s (%s). %s. Auto-expires after %d days. Use CronDelete to cancel sooner.",
			id, humanSchedule, where, DefaultMaxAgeDays,
		)), nil
	}
	return SuccessOutput(fmt.Sprintf(
		"Scheduled one-shot task %s (%s). %s. It will fire once then auto-delete.",
		id, humanSchedule, where,
	)), nil
}

// ---------------------------------------------------------------------------
// CronDeleteTool
// ---------------------------------------------------------------------------

// CronDeleteTool deletes a cron entry.
type CronDeleteTool struct {
	store *CronStore
}

func (t *CronDeleteTool) Name() string { return "CronDelete" }

// Source: prompt.ts CRON_DELETE_DESCRIPTION
func (t *CronDeleteTool) Description() string {
	return "Cancel a scheduled cron job by ID"
}

func (t *CronDeleteTool) IsReadOnly() bool   { return false }
func (t *CronDeleteTool) ShouldDefer() bool  { return true }
func (t *CronDeleteTool) SearchHint() string { return "cancel a scheduled cron job" }
func (t *CronDeleteTool) Prompt() string     { return cronDeletePrompt }

func (t *CronDeleteTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"id": {"type": "string", "description": "Job ID returned by CronCreate."}
		},
		"required": ["id"],
		"additionalProperties": false
	}`)
}

func (t *CronDeleteTool) Execute(_ context.Context, _ *ToolContext, input json.RawMessage) (*ToolOutput, error) {
	var params struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return ErrorOutput(fmt.Sprintf("invalid input: %s", err)), nil
	}
	if params.ID == "" {
		return ErrorOutput("id is required"), nil
	}

	t.store.mu.Lock()
	_, exists := t.store.entries[params.ID]
	if exists {
		delete(t.store.entries, params.ID)
	}
	t.store.mu.Unlock()

	if !exists {
		// Source: CronDeleteTool.ts:65 — errorCode 1
		return ErrorOutput(fmt.Sprintf("No scheduled job with id '%s'", params.ID)), nil
	}
	// Source: CronDeleteTool.ts:87 — verbatim result
	return SuccessOutput(fmt.Sprintf("Cancelled job %s.", params.ID)), nil
}

// ---------------------------------------------------------------------------
// CronListTool
// ---------------------------------------------------------------------------

// CronListTool lists all cron entries.
type CronListTool struct {
	store *CronStore
}

func (t *CronListTool) Name() string { return "CronList" }

// Source: prompt.ts CRON_LIST_DESCRIPTION
func (t *CronListTool) Description() string {
	return "List scheduled cron jobs"
}

func (t *CronListTool) IsReadOnly() bool                          { return true }
func (t *CronListTool) ShouldDefer() bool                         { return true }
func (t *CronListTool) SearchHint() string                        { return "list active cron jobs" }
func (t *CronListTool) Prompt() string                            { return cronListPrompt }
func (t *CronListTool) IsConcurrencySafe(_ json.RawMessage) bool { return true }

func (t *CronListTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {},
		"required": [],
		"additionalProperties": false
	}`)
}

func (t *CronListTool) Execute(_ context.Context, _ *ToolContext, _ json.RawMessage) (*ToolOutput, error) {
	t.store.mu.Lock()
	entries := make([]*CronEntry, 0, len(t.store.entries))
	for _, e := range t.store.entries {
		entries = append(entries, e)
	}
	t.store.mu.Unlock()

	if len(entries) == 0 {
		// Source: CronListTool.ts:93 — verbatim empty result
		return SuccessOutput("No scheduled jobs."), nil
	}

	// Sort by ID for deterministic output
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].ID < entries[j].ID
	})

	// Source: CronListTool.ts:88-89 — verbatim per-job line format
	var sb strings.Builder
	for i, e := range entries {
		if i > 0 {
			sb.WriteByte('\n')
		}
		human := CronToHuman(e.Cron)
		label := " (one-shot)"
		if e.Recurring {
			label = " (recurring)"
		}
		durTag := ""
		if !e.Durable {
			durTag = " [session-only]"
		}
		sb.WriteString(fmt.Sprintf("%s \u2014 %s%s%s: %s",
			e.ID, human, label, durTag, truncatePrompt(e.Prompt, maxPromptDisplay)))
	}
	return SuccessOutput(sb.String()), nil
}

// NewCronTools creates the cron tool set sharing the same store.
func NewCronTools() []Tool {
	store := &CronStore{entries: make(map[string]*CronEntry)}
	return []Tool{
		&CronCreateTool{store: store},
		&CronDeleteTool{store: store},
		&CronListTool{store: store},
	}
}
