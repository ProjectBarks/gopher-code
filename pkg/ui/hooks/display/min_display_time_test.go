package display

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestMinDisplayTime_ImmediateUpdateWhenNoMinElapsed(t *testing.T) {
	m := NewMinDisplayTimeWith[string](1, 200*time.Millisecond, "init")
	// shownAt is zero, so any update is "long enough ago".
	cmd := m.Update("second")
	assert.Nil(t, cmd, "should update immediately when min time has passed")
	assert.Equal(t, "second", m.Displayed())
}

func TestMinDisplayTime_DefersUpdateWhenTooSoon(t *testing.T) {
	now := time.Now()
	m := NewMinDisplayTimeWith[string](1, 500*time.Millisecond, "first")
	m.nowFn = func() time.Time { return now }

	// First update sets shownAt to now.
	cmd := m.Update("first-change")
	assert.Nil(t, cmd)
	assert.Equal(t, "first-change", m.Displayed())

	// Immediately try another update — should be deferred.
	cmd = m.Update("second-change")
	assert.NotNil(t, cmd, "should return a tea.Cmd to schedule the flush")
	assert.Equal(t, "first-change", m.Displayed(), "displayed should not change yet")
}

func TestMinDisplayTime_FlushPromotesPending(t *testing.T) {
	now := time.Now()
	m := NewMinDisplayTimeWith[string](1, 500*time.Millisecond, "a")
	m.nowFn = func() time.Time { return now }

	m.Update("b") // immediate (shownAt was zero)
	assert.Equal(t, "b", m.Displayed())

	m.Update("c") // deferred
	assert.Equal(t, "b", m.Displayed())

	// Simulate the timer firing — advance time.
	m.nowFn = func() time.Time { return now.Add(600 * time.Millisecond) }
	changed := m.Flush()
	assert.True(t, changed)
	assert.Equal(t, "c", m.Displayed())
}

func TestMinDisplayTime_FlushNoOpWithoutPending(t *testing.T) {
	m := NewMinDisplayTimeWith[string](1, 200*time.Millisecond, "only")
	changed := m.Flush()
	assert.False(t, changed)
	assert.Equal(t, "only", m.Displayed())
}

func TestMinDisplayTime_SameValueClearsPending(t *testing.T) {
	now := time.Now()
	m := NewMinDisplayTimeWith[string](1, 500*time.Millisecond, "a")
	m.nowFn = func() time.Time { return now }

	m.Update("b") // immediate
	m.Update("c") // deferred
	assert.Equal(t, "b", m.Displayed())

	// Update back to the currently displayed value — should clear pending.
	cmd := m.Update("b")
	assert.Nil(t, cmd)
	assert.Equal(t, "b", m.Displayed())

	// Flush should be a no-op now.
	changed := m.Flush()
	assert.False(t, changed)
}

func TestMinDisplayTime_MultipleRapidUpdatesKeepsLatest(t *testing.T) {
	now := time.Now()
	m := NewMinDisplayTimeWith[string](1, 500*time.Millisecond, "a")
	m.nowFn = func() time.Time { return now }

	m.Update("b") // immediate
	m.Update("c") // deferred, timer started
	m.Update("d") // deferred, replaces pending (no new timer)
	m.Update("e") // deferred, replaces pending (no new timer)

	// Flush should show the latest pending.
	m.nowFn = func() time.Time { return now.Add(600 * time.Millisecond) }
	changed := m.Flush()
	assert.True(t, changed)
	assert.Equal(t, "e", m.Displayed())
}

func TestMinDisplayTime_IntValues(t *testing.T) {
	m := NewMinDisplayTime[int](1, 100*time.Millisecond)
	assert.Equal(t, 0, m.Displayed(), "zero value of int")

	m.Update(42)
	assert.Equal(t, 42, m.Displayed())
}
