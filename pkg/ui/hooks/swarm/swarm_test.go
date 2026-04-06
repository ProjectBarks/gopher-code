package swarm

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
)

// ---------------------------------------------------------------------------
// SwarmInit tests
// Source: useSwarmInitialization.ts
// ---------------------------------------------------------------------------

func TestSwarmInit_CreatesTeamsDir(t *testing.T) {
	tmp := t.TempDir()
	teamsDir := filepath.Join(tmp, "teams")

	si := &SwarmInit{
		Enabled:      true,
		TeamsDir:     teamsDir,
		Teammates:    []string{"researcher", "coder"},
		ColorPalette: []string{"red", "blue", "green"},
	}

	cmd := si.Init()
	if cmd == nil {
		t.Fatal("expected non-nil Cmd when Enabled=true")
	}

	msg := cmd()
	initMsg, ok := msg.(SwarmInitMsg)
	if !ok {
		t.Fatalf("expected SwarmInitMsg, got %T", msg)
	}
	if initMsg.Err != nil {
		t.Fatalf("unexpected error: %v", initMsg.Err)
	}

	// Teams dir must exist.
	info, err := os.Stat(teamsDir)
	if err != nil {
		t.Fatalf("teams dir not created: %v", err)
	}
	if !info.IsDir() {
		t.Fatal("teams path is not a directory")
	}

	// Teammates populated.
	if len(initMsg.Teammates) != 2 {
		t.Errorf("teammates = %d, want 2", len(initMsg.Teammates))
	}

	// Colors assigned round-robin.
	if initMsg.Colors["researcher"] != "red" {
		t.Errorf("researcher color = %q, want %q", initMsg.Colors["researcher"], "red")
	}
	if initMsg.Colors["coder"] != "blue" {
		t.Errorf("coder color = %q, want %q", initMsg.Colors["coder"], "blue")
	}

	// TeamsDir in message.
	if initMsg.TeamsDir != teamsDir {
		t.Errorf("TeamsDir = %q, want %q", initMsg.TeamsDir, teamsDir)
	}
}

func TestSwarmInit_DisabledReturnsNil(t *testing.T) {
	si := &SwarmInit{Enabled: false}
	cmd := si.Init()
	if cmd != nil {
		t.Fatal("expected nil Cmd when Enabled=false")
	}
}

func TestSwarmInit_DefaultPalette(t *testing.T) {
	tmp := t.TempDir()
	si := &SwarmInit{
		Enabled:   true,
		TeamsDir:  filepath.Join(tmp, "teams"),
		Teammates: []string{"a", "b", "c", "d", "e", "f", "g", "h", "i"},
	}
	msg := si.Init()().(SwarmInitMsg)
	if msg.Err != nil {
		t.Fatalf("unexpected error: %v", msg.Err)
	}
	// 9th teammate wraps to first color.
	if msg.Colors["i"] != defaultColorPalette[0] {
		t.Errorf("9th teammate color = %q, want %q (wrap)", msg.Colors["i"], defaultColorPalette[0])
	}
}

func TestSwarmInit_InvalidDirReturnsError(t *testing.T) {
	// Use a path under a file to trigger MkdirAll failure.
	tmp := t.TempDir()
	blocker := filepath.Join(tmp, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	si := &SwarmInit{
		Enabled:  true,
		TeamsDir: filepath.Join(blocker, "teams"),
	}
	msg := si.Init()().(SwarmInitMsg)
	if msg.Err == nil {
		t.Fatal("expected error when dir creation fails")
	}
}

// ---------------------------------------------------------------------------
// TaskWatcher tests
// Source: useTaskListWatcher.ts
// ---------------------------------------------------------------------------

func TestTaskWatcher_DetectsCompletion(t *testing.T) {
	tasks := []Task{
		{ID: "1", Subject: "fix bug", Status: TaskPending},
		{ID: "2", Subject: "add test", Status: TaskPending, BlockedBy: []string{"1"}},
	}

	claimed := false
	submitted := ""

	w := &TaskWatcher{
		AgentID:  "worker-1",
		Interval: 10 * time.Millisecond,
		ListTasks: func() ([]Task, error) {
			return tasks, nil
		},
		ClaimTask: func(taskID, agentID string) (bool, error) {
			claimed = true
			return true, nil
		},
		SubmitTask: func(prompt string) bool {
			submitted = prompt
			return true
		},
	}

	// First tick: should claim task 1 (only unblocked pending task).
	msg1 := w.check()
	if msg1.Err != nil {
		t.Fatalf("tick 1 error: %v", msg1.Err)
	}
	if !claimed {
		t.Fatal("expected task 1 to be claimed")
	}
	if msg1.Claimed == nil || msg1.Claimed.ID != "1" {
		t.Fatal("expected claimed task ID=1")
	}
	if submitted == "" {
		t.Fatal("expected prompt to be submitted")
	}

	// Simulate task 1 completion.
	tasks[0].Status = TaskCompleted

	// Second tick: should detect completion of task 1.
	claimed = false
	submitted = ""
	// Task 2 is still blocked by task 1, but task 1 is now completed
	// so task 2 should become available.
	msg2 := w.check()
	if msg2.Err != nil {
		t.Fatalf("tick 2 error: %v", msg2.Err)
	}
	if len(msg2.Completed) == 0 || msg2.Completed[0] != "1" {
		t.Errorf("expected completed=[1], got %v", msg2.Completed)
	}

	// Task 2 should now be claimed (blocker resolved).
	if !claimed {
		t.Fatal("expected task 2 to be claimed after task 1 completed")
	}
	if msg2.Claimed == nil || msg2.Claimed.ID != "2" {
		t.Fatal("expected claimed task ID=2")
	}
}

func TestTaskWatcher_SkipsBlockedTasks(t *testing.T) {
	tasks := []Task{
		{ID: "1", Subject: "first", Status: TaskPending, Owner: "other"},
		{ID: "2", Subject: "second", Status: TaskPending, BlockedBy: []string{"1"}},
	}

	w := &TaskWatcher{
		AgentID: "worker-1",
		ListTasks: func() ([]Task, error) {
			return tasks, nil
		},
		ClaimTask: func(taskID, agentID string) (bool, error) {
			t.Fatal("should not attempt to claim a blocked task")
			return false, nil
		},
	}

	msg := w.check()
	if msg.Err != nil {
		t.Fatalf("unexpected error: %v", msg.Err)
	}
	if msg.Claimed != nil {
		t.Fatal("expected no task claimed when all are blocked/owned")
	}
}

func TestTaskWatcher_SubmitFailureReleasesClaim(t *testing.T) {
	tasks := []Task{
		{ID: "1", Subject: "task", Status: TaskPending},
	}

	w := &TaskWatcher{
		AgentID: "worker-1",
		ListTasks: func() ([]Task, error) {
			return tasks, nil
		},
		ClaimTask: func(taskID, agentID string) (bool, error) {
			return true, nil
		},
		SubmitTask: func(prompt string) bool {
			return false // submission fails
		},
	}

	msg := w.check()
	if msg.Err != nil {
		t.Fatalf("unexpected error: %v", msg.Err)
	}
	// Claimed should be nil because submission failed and claim was released.
	if msg.Claimed != nil {
		t.Fatal("expected no claimed task after submit failure")
	}

	// currentTaskID should be empty.
	w.mu.Lock()
	cur := w.currentTaskID
	w.mu.Unlock()
	if cur != "" {
		t.Errorf("currentTaskID = %q, want empty after submit failure", cur)
	}
}

func TestTaskWatcher_TickReturnsCmd(t *testing.T) {
	w := &TaskWatcher{
		AgentID: "w",
		ListTasks: func() ([]Task, error) {
			return nil, nil
		},
	}
	cmd := w.Tick()
	if cmd == nil {
		t.Fatal("expected non-nil Cmd from Tick()")
	}
}

func TestFormatTaskPrompt(t *testing.T) {
	task := &Task{ID: "42", Subject: "fix the bug", Description: "it crashes on startup"}
	got := formatTaskPrompt(task)
	want := "Complete all open tasks. Start with task #42: \n\n fix the bug\n\nit crashes on startup"
	if got != want {
		t.Errorf("formatTaskPrompt =\n%q\nwant\n%q", got, want)
	}
}

func TestFormatTaskPrompt_NoDescription(t *testing.T) {
	task := &Task{ID: "7", Subject: "do something"}
	got := formatTaskPrompt(task)
	want := "Complete all open tasks. Start with task #7: \n\n do something"
	if got != want {
		t.Errorf("formatTaskPrompt = %q, want %q", got, want)
	}
}

func TestFindAvailableTask(t *testing.T) {
	tests := []struct {
		name  string
		tasks []Task
		want  string // expected task ID, "" for none
	}{
		{
			name:  "empty list",
			tasks: nil,
			want:  "",
		},
		{
			name: "single pending",
			tasks: []Task{
				{ID: "1", Subject: "x", Status: TaskPending},
			},
			want: "1",
		},
		{
			name: "skip owned",
			tasks: []Task{
				{ID: "1", Subject: "x", Status: TaskPending, Owner: "other"},
				{ID: "2", Subject: "y", Status: TaskPending},
			},
			want: "2",
		},
		{
			name: "skip blocked by unresolved",
			tasks: []Task{
				{ID: "1", Subject: "x", Status: TaskClaimed},
				{ID: "2", Subject: "y", Status: TaskPending, BlockedBy: []string{"1"}},
			},
			want: "",
		},
		{
			name: "unblocked when blocker completed",
			tasks: []Task{
				{ID: "1", Subject: "x", Status: TaskCompleted},
				{ID: "2", Subject: "y", Status: TaskPending, BlockedBy: []string{"1"}},
			},
			want: "2",
		},
		{
			name: "skip completed",
			tasks: []Task{
				{ID: "1", Subject: "x", Status: TaskCompleted},
			},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := findAvailableTask(tt.tasks)
			gotID := ""
			if got != nil {
				gotID = got.ID
			}
			if gotID != tt.want {
				t.Errorf("findAvailableTask() = %q, want %q", gotID, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// PermissionPoller tests
// Source: useSwarmPermissionPoller.ts
// ---------------------------------------------------------------------------

func TestPermissionPoller_SurfacesRequests(t *testing.T) {
	var allowedInput map[string]any
	var allowedFeedback string
	var rejectedFeedback string

	poller := &PermissionPoller{
		AgentName: "researcher",
		TeamName:  "alpha",
		Interval:  10 * time.Millisecond,
		Poll: func(requestID, agentName, teamName string) (*PermissionResponse, error) {
			if requestID == "req-1" {
				return &PermissionResponse{
					RequestID:    "req-1",
					Decision:     PermissionApproved,
					Feedback:     "looks good",
					UpdatedInput: map[string]any{"path": "/tmp/safe"},
				}, nil
			}
			if requestID == "req-2" {
				return &PermissionResponse{
					RequestID: "req-2",
					Decision:  PermissionRejected,
					Feedback:  "not allowed",
				}, nil
			}
			return nil, nil
		},
		RemoveResponse: func(requestID, agentName, teamName string) error {
			return nil
		},
	}

	poller.Register(&PermissionCallback{
		RequestID: "req-1",
		ToolUseID: "tool-1",
		OnAllow: func(input map[string]any, feedback string) {
			allowedInput = input
			allowedFeedback = feedback
		},
	})
	poller.Register(&PermissionCallback{
		RequestID: "req-2",
		ToolUseID: "tool-2",
		OnReject: func(feedback string) {
			rejectedFeedback = feedback
		},
	})

	if poller.PendingCount() != 2 {
		t.Fatalf("pending = %d, want 2", poller.PendingCount())
	}

	msg := poller.poll()
	if msg.Err != nil {
		t.Fatalf("poll error: %v", msg.Err)
	}

	if len(msg.Responses) != 2 {
		t.Fatalf("responses = %d, want 2", len(msg.Responses))
	}

	// Callbacks should have been invoked.
	if allowedInput == nil {
		t.Fatal("OnAllow not called for req-1")
	}
	if allowedInput["path"] != "/tmp/safe" {
		t.Errorf("updatedInput[path] = %v, want /tmp/safe", allowedInput["path"])
	}
	if allowedFeedback != "looks good" {
		t.Errorf("allow feedback = %q, want %q", allowedFeedback, "looks good")
	}
	if rejectedFeedback != "not allowed" {
		t.Errorf("reject feedback = %q, want %q", rejectedFeedback, "not allowed")
	}

	// Callbacks should be deregistered after processing.
	if poller.PendingCount() != 0 {
		t.Errorf("pending after poll = %d, want 0", poller.PendingCount())
	}
}

func TestPermissionPoller_SkipsWhenNoCallbacks(t *testing.T) {
	poller := &PermissionPoller{
		AgentName: "worker",
		TeamName:  "team",
		Poll: func(requestID, agentName, teamName string) (*PermissionResponse, error) {
			t.Fatal("should not poll when no callbacks registered")
			return nil, nil
		},
	}

	msg := poller.poll()
	if msg.Err != nil {
		t.Fatalf("unexpected error: %v", msg.Err)
	}
	if len(msg.Responses) != 0 {
		t.Errorf("expected no responses, got %d", len(msg.Responses))
	}
}

func TestPermissionPoller_SkipsWhenNoIdentity(t *testing.T) {
	poller := &PermissionPoller{
		// AgentName and TeamName are empty.
		Poll: func(requestID, agentName, teamName string) (*PermissionResponse, error) {
			t.Fatal("should not poll without identity")
			return nil, nil
		},
	}
	poller.Register(&PermissionCallback{RequestID: "req-1"})

	msg := poller.poll()
	if msg.Err != nil {
		t.Fatalf("unexpected error: %v", msg.Err)
	}
}

func TestPermissionPoller_RegisterUnregisterHas(t *testing.T) {
	poller := &PermissionPoller{}

	poller.Register(&PermissionCallback{RequestID: "r1"})
	if !poller.Has("r1") {
		t.Fatal("expected Has(r1)=true after Register")
	}
	if poller.Has("r2") {
		t.Fatal("expected Has(r2)=false")
	}

	poller.Unregister("r1")
	if poller.Has("r1") {
		t.Fatal("expected Has(r1)=false after Unregister")
	}
}

func TestPermissionPoller_Clear(t *testing.T) {
	poller := &PermissionPoller{}
	poller.Register(&PermissionCallback{RequestID: "r1"})
	poller.Register(&PermissionCallback{RequestID: "r2"})
	poller.Clear()
	if poller.PendingCount() != 0 {
		t.Errorf("pending after Clear = %d, want 0", poller.PendingCount())
	}
}

func TestPermissionPoller_ConcurrentRegister(t *testing.T) {
	poller := &PermissionPoller{}
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			poller.Register(&PermissionCallback{
				RequestID: fmt.Sprintf("req-%d", id),
			})
		}(i)
	}
	wg.Wait()
	if poller.PendingCount() != 100 {
		t.Errorf("pending = %d, want 100", poller.PendingCount())
	}
}

func TestPermissionPoller_TickReturnsCmd(t *testing.T) {
	poller := &PermissionPoller{
		AgentName: "w",
		TeamName:  "t",
		Poll: func(string, string, string) (*PermissionResponse, error) {
			return nil, nil
		},
	}
	cmd := poller.Tick()
	if cmd == nil {
		t.Fatal("expected non-nil Cmd from Tick()")
	}
}

// ---------------------------------------------------------------------------
// Verify message types implement tea.Msg (compile-time check)
// ---------------------------------------------------------------------------

var (
	_ tea.Msg = SwarmInitMsg{}
	_ tea.Msg = TaskWatchMsg{}
	_ tea.Msg = PermissionPollMsg{}
)
