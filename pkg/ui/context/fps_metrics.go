// Package context provides UI context types (fps, modal, notifications, etc.).
package context

import (
	"math"
	"sort"
	"sync"
	"time"
)

// Source: utils/fpsTracker.ts

// FpsMetrics holds computed FPS statistics.
type FpsMetrics struct {
	AverageFps float64
	Low1PctFps float64
}

// FpsTracker records frame durations and computes FPS metrics.
type FpsTracker struct {
	mu              sync.Mutex
	frameDurations  []float64 // ms
	firstRenderTime time.Time
	lastRenderTime  time.Time
	hasFirst        bool
}

// NewFpsTracker creates a new FPS tracker.
func NewFpsTracker() *FpsTracker {
	return &FpsTracker{}
}

// Record logs a frame render duration in milliseconds.
func (t *FpsTracker) Record(durationMs float64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	now := time.Now()
	if !t.hasFirst {
		t.firstRenderTime = now
		t.hasFirst = true
	}
	t.lastRenderTime = now
	t.frameDurations = append(t.frameDurations, durationMs)
}

// GetMetrics computes average FPS and low 1% FPS from recorded frames.
// Returns nil if no frames have been recorded.
func (t *FpsTracker) GetMetrics() *FpsMetrics {
	t.mu.Lock()
	defer t.mu.Unlock()

	if len(t.frameDurations) == 0 || !t.hasFirst {
		return nil
	}

	totalTimeMs := t.lastRenderTime.Sub(t.firstRenderTime).Seconds() * 1000
	if totalTimeMs <= 0 {
		return nil
	}

	totalFrames := float64(len(t.frameDurations))
	averageFps := totalFrames / (totalTimeMs / 1000)

	// Sort descending for p99.
	sorted := make([]float64, len(t.frameDurations))
	copy(sorted, t.frameDurations)
	sort.Sort(sort.Reverse(sort.Float64Slice(sorted)))

	p99Index := int(math.Max(0, math.Ceil(float64(len(sorted))*0.01)-1))
	p99FrameTimeMs := sorted[p99Index]
	var low1PctFps float64
	if p99FrameTimeMs > 0 {
		low1PctFps = 1000 / p99FrameTimeMs
	}

	return &FpsMetrics{
		AverageFps: math.Round(averageFps*100) / 100,
		Low1PctFps: math.Round(low1PctFps*100) / 100,
	}
}
