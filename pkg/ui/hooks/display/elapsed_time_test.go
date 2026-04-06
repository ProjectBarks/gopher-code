package display

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestElapsedTime_StartsStoppedWithZero(t *testing.T) {
	et := NewElapsedTime(1, time.Second)
	assert.False(t, et.IsRunning())
	assert.Equal(t, "0s", et.Format())
}

func TestElapsedTime_TracksElapsedAfterStart(t *testing.T) {
	et := NewElapsedTime(1, time.Second)
	et.StartFrom(time.Now().Add(-5 * time.Second))

	d := et.Elapsed()
	assert.InDelta(t, 5.0, d.Seconds(), 0.2, "should be ~5s elapsed")
	assert.Equal(t, "5s", et.Format())
}

func TestElapsedTime_StopFreezesElapsed(t *testing.T) {
	et := NewElapsedTime(1, time.Second)
	start := time.Now().Add(-10 * time.Second)
	et.StartFrom(start)
	et.StopAt(start.Add(3 * time.Second))

	assert.False(t, et.IsRunning())
	assert.Equal(t, "3s", et.Format())

	// Even after more real time passes, the format stays frozen.
	time.Sleep(10 * time.Millisecond)
	assert.Equal(t, "3s", et.Format())
}

func TestElapsedTime_PausedDurationSubtracted(t *testing.T) {
	et := NewElapsedTime(1, time.Second)
	start := time.Now().Add(-10 * time.Second)
	et.StartFrom(start)
	et.AddPaused(7 * time.Second)
	et.StopAt(start.Add(10 * time.Second))

	// 10s total - 7s paused = 3s
	assert.Equal(t, "3s", et.Format())
}

func TestElapsedTime_TickReturnsNilWhenStopped(t *testing.T) {
	et := NewElapsedTime(1, time.Second)
	assert.Nil(t, et.Tick(), "stopped timer should return nil cmd")
}

func TestElapsedTime_TickReturnsCmdWhenRunning(t *testing.T) {
	et := NewElapsedTime(1, time.Second)
	et.Start()
	cmd := et.Tick()
	assert.NotNil(t, cmd, "running timer should return a tick cmd")
}

func TestElapsedTime_DefaultIntervalIsOneSecond(t *testing.T) {
	et := NewElapsedTime(1, 0) // 0 triggers default
	assert.NotNil(t, et)
	et.Start()
	assert.NotNil(t, et.Tick())
}

func TestFormatDuration_Variants(t *testing.T) {
	tests := []struct {
		d    time.Duration
		want string
	}{
		{0, "0s"},
		{-1 * time.Second, "0s"},
		{500 * time.Millisecond, "0s"},
		{1 * time.Second, "1s"},
		{59 * time.Second, "59s"},
		{60 * time.Second, "1m"},
		{90 * time.Second, "1m 30s"},
		{3600 * time.Second, "1h"},
		{3661 * time.Second, "1h 1m"},
		{7200 * time.Second, "2h"},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, FormatDuration(tt.d), "FormatDuration(%v)", tt.d)
	}
}
