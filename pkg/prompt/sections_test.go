package prompt

import (
	"testing"
)

func TestSystemPromptSection_CachesResult(t *testing.T) {
	calls := 0
	s := SystemPromptSection("greeting", func() *string {
		calls++
		v := "hello"
		return &v
	})

	// Resolve twice — compute should only run once (cached).
	results := ResolveSystemPromptSections([]Section{s, s})
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if *results[0] != "hello" || *results[1] != "hello" {
		t.Error("expected both results to be 'hello'")
	}
	if calls != 1 {
		t.Errorf("cached section should compute once, got %d calls", calls)
	}
}

func TestUncachedSection_RecomputesEveryTime(t *testing.T) {
	calls := 0
	s := UncachedSystemPromptSection("counter", func() *string {
		calls++
		v := "val"
		return &v
	}, "test: always recompute")

	ResolveSystemPromptSections([]Section{s})
	ResolveSystemPromptSections([]Section{s})
	if calls != 2 {
		t.Errorf("uncached section should compute every call, got %d", calls)
	}
}

func TestResolveSystemPromptSections_NilResults(t *testing.T) {
	s := SystemPromptSection("empty", func() *string {
		return nil
	})

	results := ResolveSystemPromptSections([]Section{s})
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0] != nil {
		t.Error("expected nil result")
	}
}

func TestResolveSystemPromptSections_NilCachedCorrectly(t *testing.T) {
	calls := 0
	s := SystemPromptSection("nil-cached", func() *string {
		calls++
		return nil
	})

	// First call computes, second should use cache (nil).
	ResolveSystemPromptSections([]Section{s})
	ResolveSystemPromptSections([]Section{s})
	if calls != 1 {
		t.Errorf("nil result should be cached, got %d calls", calls)
	}
}

func TestClearSystemPromptSections_ResetsCache(t *testing.T) {
	calls := 0
	s := SystemPromptSection("resettable", func() *string {
		calls++
		v := "v"
		return &v
	})

	ResolveSystemPromptSections([]Section{s})
	if calls != 1 {
		t.Fatalf("expected 1 call before clear, got %d", calls)
	}

	ClearSystemPromptSections()

	ResolveSystemPromptSections([]Section{s})
	if calls != 2 {
		t.Errorf("after clear, section should recompute; got %d calls", calls)
	}
}

func TestClearSystemPromptSections_ResetsBetaHeaderLatches(t *testing.T) {
	// Set some latches, then clear. Verify they're nil after.
	v := true
	SetBetaHeaderLatch("afkMode", &v)
	SetBetaHeaderLatch("fastMode", &v)
	SetBetaHeaderLatch("cacheEditing", &v)

	ClearSystemPromptSections()

	if GetBetaHeaderLatch("afkMode") != nil {
		t.Error("afkMode latch should be nil after clear")
	}
	if GetBetaHeaderLatch("fastMode") != nil {
		t.Error("fastMode latch should be nil after clear")
	}
	if GetBetaHeaderLatch("cacheEditing") != nil {
		t.Error("cacheEditing latch should be nil after clear")
	}
}

func TestResolveSystemPromptSections_MixedCachedAndUncached(t *testing.T) {
	cachedCalls := 0
	uncachedCalls := 0

	cached := SystemPromptSection("cached", func() *string {
		cachedCalls++
		v := "C"
		return &v
	})
	uncached := UncachedSystemPromptSection("uncached", func() *string {
		uncachedCalls++
		v := "U"
		return &v
	}, "test reason")

	ResolveSystemPromptSections([]Section{cached, uncached})
	ResolveSystemPromptSections([]Section{cached, uncached})

	if cachedCalls != 1 {
		t.Errorf("cached section: expected 1 call, got %d", cachedCalls)
	}
	if uncachedCalls != 2 {
		t.Errorf("uncached section: expected 2 calls, got %d", uncachedCalls)
	}
}

func TestResolveSystemPromptSections_Empty(t *testing.T) {
	results := ResolveSystemPromptSections(nil)
	if len(results) != 0 {
		t.Errorf("expected empty results for nil sections, got %d", len(results))
	}
}
