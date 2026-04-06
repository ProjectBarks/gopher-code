package hooks

import (
	"sync"
	"testing"
)

// T144: HookRegistry — register, get, clear lifecycle

func TestHookRegistry_NewIsEmpty(t *testing.T) {
	r := NewHookRegistry()
	if !r.IsEmpty() {
		t.Error("new registry should be empty")
	}
	if got := r.GetAll(); got != nil {
		t.Errorf("GetAll on empty registry = %v, want nil", got)
	}
	if got := r.Get(PreToolUse); got != nil {
		t.Errorf("Get(PreToolUse) on empty registry = %v, want nil", got)
	}
}

func TestHookRegistry_RegisterAndGet(t *testing.T) {
	r := NewHookRegistry()

	r.Register(map[HookEvent][]RegisteredHookCallback{
		PreToolUse: {
			{Matcher: "Bash", Callback: "cb1"},
		},
		Stop: {
			{Matcher: "", Callback: "cb2"},
		},
	})

	if r.IsEmpty() {
		t.Error("registry should not be empty after Register")
	}

	pre := r.Get(PreToolUse)
	if len(pre) != 1 {
		t.Fatalf("Get(PreToolUse) len = %d, want 1", len(pre))
	}
	if pre[0].Matcher != "Bash" {
		t.Errorf("matcher = %q, want Bash", pre[0].Matcher)
	}

	stop := r.Get(Stop)
	if len(stop) != 1 {
		t.Fatalf("Get(Stop) len = %d, want 1", len(stop))
	}

	// Unregistered event returns nil
	if got := r.Get(SessionEnd); got != nil {
		t.Errorf("Get(SessionEnd) = %v, want nil", got)
	}
}

func TestHookRegistry_RegisterMerges(t *testing.T) {
	r := NewHookRegistry()

	// First registration
	r.Register(map[HookEvent][]RegisteredHookCallback{
		PreToolUse: {{Matcher: "Bash", Callback: "cb1"}},
	})

	// Second registration appends, does not overwrite
	r.Register(map[HookEvent][]RegisteredHookCallback{
		PreToolUse: {{Matcher: "Read", Callback: "cb2"}},
		PostToolUse: {{Matcher: "*", Callback: "cb3"}},
	})

	pre := r.Get(PreToolUse)
	if len(pre) != 2 {
		t.Fatalf("Get(PreToolUse) len = %d, want 2", len(pre))
	}
	if pre[0].Matcher != "Bash" || pre[1].Matcher != "Read" {
		t.Errorf("matchers = [%q, %q], want [Bash, Read]", pre[0].Matcher, pre[1].Matcher)
	}

	post := r.Get(PostToolUse)
	if len(post) != 1 {
		t.Fatalf("Get(PostToolUse) len = %d, want 1", len(post))
	}
}

func TestHookRegistry_Clear(t *testing.T) {
	r := NewHookRegistry()

	r.Register(map[HookEvent][]RegisteredHookCallback{
		PreToolUse: {{Matcher: "Bash"}},
		Stop:       {{Matcher: ""}},
	})

	r.Clear()
	if !r.IsEmpty() {
		t.Error("registry should be empty after Clear")
	}
	if got := r.GetAll(); got != nil {
		t.Errorf("GetAll after Clear = %v, want nil", got)
	}
}

func TestHookRegistry_ClearPluginHooks(t *testing.T) {
	r := NewHookRegistry()

	r.Register(map[HookEvent][]RegisteredHookCallback{
		PreToolUse: {
			{Matcher: "Bash", PluginRoot: "/plugins/a", PluginName: "plugin-a"},
			{Matcher: "Read", Callback: "sdk-cb"},
		},
		Stop: {
			{Matcher: "", PluginRoot: "/plugins/b"},
		},
	})

	r.ClearPluginHooks()

	// PreToolUse should keep only the SDK callback (no PluginRoot)
	pre := r.Get(PreToolUse)
	if len(pre) != 1 {
		t.Fatalf("Get(PreToolUse) len = %d, want 1", len(pre))
	}
	if pre[0].Matcher != "Read" {
		t.Errorf("remaining matcher = %q, want Read", pre[0].Matcher)
	}

	// Stop had only plugin hooks, so it should be gone
	stop := r.Get(Stop)
	if stop != nil {
		t.Errorf("Get(Stop) = %v, want nil (all were plugin hooks)", stop)
	}

	if r.IsEmpty() {
		t.Error("registry should not be empty (PreToolUse SDK callback remains)")
	}
}

func TestHookRegistry_ClearPluginHooks_AllPlugin(t *testing.T) {
	r := NewHookRegistry()
	r.Register(map[HookEvent][]RegisteredHookCallback{
		PreToolUse: {{PluginRoot: "/p"}},
	})
	r.ClearPluginHooks()
	if !r.IsEmpty() {
		t.Error("registry should be empty when all hooks are plugin hooks")
	}
}

func TestHookRegistry_ClearPluginHooks_Empty(t *testing.T) {
	r := NewHookRegistry()
	r.ClearPluginHooks() // should not panic
	if !r.IsEmpty() {
		t.Error("registry should still be empty")
	}
}

func TestHookRegistry_RegisterNilIsNoop(t *testing.T) {
	r := NewHookRegistry()
	r.Register(nil)
	if !r.IsEmpty() {
		t.Error("Register(nil) should be a no-op")
	}
	r.Register(map[HookEvent][]RegisteredHookCallback{})
	if !r.IsEmpty() {
		t.Error("Register(empty) should be a no-op")
	}
}

func TestHookRegistry_ConcurrentAccess(t *testing.T) {
	r := NewHookRegistry()
	var wg sync.WaitGroup

	// Concurrent writers
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r.Register(map[HookEvent][]RegisteredHookCallback{
				PreToolUse: {{Matcher: "test"}},
			})
		}()
	}

	// Concurrent readers
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = r.Get(PreToolUse)
			_ = r.GetAll()
			_ = r.IsEmpty()
		}()
	}

	wg.Wait()

	pre := r.Get(PreToolUse)
	if len(pre) != 10 {
		t.Errorf("after 10 concurrent registers, len = %d, want 10", len(pre))
	}
}
