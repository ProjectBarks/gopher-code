package services

import (
	"math/rand"
)

// Source: services/tips/tips.ts

// Tip is a helpful hint shown to users between turns.
type Tip struct {
	Text     string
	Category string // "shortcut", "feature", "workflow"
}

// DefaultTips are the built-in tips matching the TS source.
// Source: services/tips/tips.ts
var DefaultTips = []Tip{
	{Text: "Use Ctrl+R to search command history", Category: "shortcut"},
	{Text: "Use @filename to reference files in your prompt", Category: "feature"},
	{Text: "Use /compact to reduce context window usage", Category: "feature"},
	{Text: "Use /help to see all available commands", Category: "feature"},
	{Text: "Use Shift+Tab to accept a file suggestion", Category: "shortcut"},
	{Text: "Use Ctrl+C twice to exit", Category: "shortcut"},
	{Text: "Create a CLAUDE.md file to give Claude project context", Category: "workflow"},
	{Text: "Use /doctor to diagnose configuration issues", Category: "feature"},
	{Text: "Use Escape to cancel a running query", Category: "shortcut"},
	{Text: "Use /theme to change the color theme", Category: "feature"},
}

// RandomTip returns a random tip from the default set.
func RandomTip() Tip {
	return DefaultTips[rand.Intn(len(DefaultTips))]
}

// TipManager tracks which tips have been shown to avoid repetition.
type TipManager struct {
	shown map[int]bool
}

// NewTipManager creates a new tip manager.
func NewTipManager() *TipManager {
	return &TipManager{shown: make(map[int]bool)}
}

// Next returns a tip that hasn't been shown yet, or a random one if all shown.
func (m *TipManager) Next() Tip {
	// Find unshown tips.
	var candidates []int
	for i := range DefaultTips {
		if !m.shown[i] {
			candidates = append(candidates, i)
		}
	}
	if len(candidates) == 0 {
		// All shown, reset and pick random.
		m.shown = make(map[int]bool)
		return RandomTip()
	}
	idx := candidates[rand.Intn(len(candidates))]
	m.shown[idx] = true
	return DefaultTips[idx]
}
