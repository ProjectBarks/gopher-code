package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/hinshun/vt10x"
)

// Frame represents a single captured screen state.
type Frame struct {
	Timestamp   time.Duration `json:"timestamp_ms"` // since app start
	ScreenText  string        `json:"screen_text"`
	CursorX     int           `json:"cursor_x"`
	CursorY     int           `json:"cursor_y"`
	CursorVis   bool          `json:"cursor_visible"`
	CellChanges int           `json:"cell_changes"` // cells changed from prior frame
	Trigger     string        `json:"trigger"`      // what caused this: "init", "input:X", "poll"
}

// InputEvent records an input sent to the PTY.
type InputEvent struct {
	Timestamp time.Duration `json:"timestamp_ms"`
	Action    string        `json:"action"` // "type" or "key"
	Value     string        `json:"value"`
}

// CellData captures per-cell attributes from the VT terminal.
type CellData struct {
	X     int    `json:"x"`
	Y     int    `json:"y"`
	Char  string `json:"char"`
	FG    int32  `json:"fg"`
	BG    int32  `json:"bg"`
	Attrs int16  `json:"attrs"` // vt10x glyph mode bits
}

// Recording is the complete data from a scenario run.
type Recording struct {
	AppName    string       `json:"app_name"`
	Scenario   string       `json:"scenario"`
	Cols       int          `json:"cols"`
	Rows       int          `json:"rows"`
	StartTime  time.Time    `json:"start_time"`
	Duration   time.Duration `json:"duration_ms"`
	Frames     []Frame      `json:"frames"`
	Inputs     []InputEvent `json:"inputs"`
	Snapshots  map[string]string `json:"snapshots"` // name → screen text
}

// Recorder monitors a VT terminal for screen changes and records frames.
type Recorder struct {
	term     vt10x.Terminal
	cols     int
	rows     int
	start    time.Time
	mu       sync.Mutex
	frames   []Frame
	inputs   []InputEvent
	snapshots map[string]string
	prevScreen string
	pollInterval time.Duration
	stopCh   chan struct{}
	wg       sync.WaitGroup
}

// NewRecorder creates a recorder that will monitor the given terminal.
func NewRecorder(term vt10x.Terminal, cols, rows int) *Recorder {
	return &Recorder{
		term:         term,
		cols:         cols,
		rows:         rows,
		start:        time.Now(),
		snapshots:    make(map[string]string),
		pollInterval: 50 * time.Millisecond,
		stopCh:       make(chan struct{}),
	}
}

// Start begins polling the terminal for screen changes in a background goroutine.
func (r *Recorder) Start() {
	// Capture initial frame
	screen := r.readScreen()
	r.mu.Lock()
	r.prevScreen = screen
	r.frames = append(r.frames, Frame{
		Timestamp:   time.Since(r.start),
		ScreenText:  screen,
		CursorX:     r.cursorX(),
		CursorY:     r.cursorY(),
		CursorVis:   r.cursorVisible(),
		CellChanges: r.cols * r.rows, // all cells are "new"
		Trigger:     "init",
	})
	r.mu.Unlock()

	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		ticker := time.NewTicker(r.pollInterval)
		defer ticker.Stop()
		for {
			select {
			case <-r.stopCh:
				return
			case <-ticker.C:
				r.poll("poll")
			}
		}
	}()
}

// Stop ends polling and returns. Safe to call multiple times.
func (r *Recorder) Stop() {
	select {
	case <-r.stopCh:
		return // already stopped
	default:
		close(r.stopCh)
	}
	r.wg.Wait()
}

// RecordInput logs an input event and forces a poll after a brief delay.
func (r *Recorder) RecordInput(action, value string) {
	r.mu.Lock()
	r.inputs = append(r.inputs, InputEvent{
		Timestamp: time.Since(r.start),
		Action:    action,
		Value:     value,
	})
	r.mu.Unlock()

	// Give the app a moment to process, then capture the resulting frame
	time.Sleep(100 * time.Millisecond)
	r.poll(fmt.Sprintf("input:%s", value))
}

// Snapshot captures the current screen with a name.
func (r *Recorder) Snapshot(name string) string {
	screen := r.readScreen()
	r.mu.Lock()
	defer r.mu.Unlock()
	r.snapshots[name] = screen
	return screen
}

// SnapshotCells captures cell-level data for the current screen.
func (r *Recorder) SnapshotCells() []CellData {
	r.term.Lock()
	defer r.term.Unlock()

	var cells []CellData
	for y := 0; y < r.rows; y++ {
		for x := 0; x < r.cols; x++ {
			g := r.term.Cell(x, y)
			if g.Char == 0 {
				continue // skip empty cells to save space
			}
			cells = append(cells, CellData{
				X:     x,
				Y:     y,
				Char:  string(g.Char),
				FG:    int32(g.FG),
				BG:    int32(g.BG),
				Attrs: int16(g.Mode),
			})
		}
	}
	return cells
}

// CurrentScreen returns the current rendered screen text.
func (r *Recorder) CurrentScreen() string {
	return r.readScreen()
}

// Result produces the final Recording.
func (r *Recorder) Result(appName, scenarioName string) *Recording {
	r.mu.Lock()
	defer r.mu.Unlock()
	return &Recording{
		AppName:   appName,
		Scenario:  scenarioName,
		Cols:      r.cols,
		Rows:      r.rows,
		StartTime: r.start,
		Duration:  time.Since(r.start),
		Frames:    r.frames,
		Inputs:    r.inputs,
		Snapshots: r.snapshots,
	}
}

// poll checks if the screen has changed and records a frame if so.
func (r *Recorder) poll(trigger string) {
	screen := r.readScreen()
	r.mu.Lock()
	defer r.mu.Unlock()

	if screen == r.prevScreen {
		return
	}

	changes := countChanges(r.prevScreen, screen)
	r.prevScreen = screen

	r.frames = append(r.frames, Frame{
		Timestamp:   time.Since(r.start),
		ScreenText:  screen,
		CursorX:     r.cursorX(),
		CursorY:     r.cursorY(),
		CursorVis:   r.cursorVisible(),
		CellChanges: changes,
		Trigger:     trigger,
	})
}

// readScreen extracts the terminal screen as a string.
func (r *Recorder) readScreen() string {
	r.term.Lock()
	defer r.term.Unlock()

	var lines []string
	for y := 0; y < r.rows; y++ {
		var line strings.Builder
		for x := 0; x < r.cols; x++ {
			g := r.term.Cell(x, y)
			if g.Char == 0 {
				line.WriteRune(' ')
			} else {
				line.WriteRune(g.Char)
			}
		}
		lines = append(lines, strings.TrimRight(line.String(), " "))
	}

	// Trim trailing empty lines
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	return strings.Join(lines, "\n") + "\n"
}

func (r *Recorder) cursorX() int {
	r.term.Lock()
	defer r.term.Unlock()
	return r.term.Cursor().X
}

func (r *Recorder) cursorY() int {
	r.term.Lock()
	defer r.term.Unlock()
	return r.term.Cursor().Y
}

func (r *Recorder) cursorVisible() bool {
	r.term.Lock()
	defer r.term.Unlock()
	return r.term.CursorVisible()
}

// countChanges counts character-level differences between two screen strings.
func countChanges(prev, curr string) int {
	prevRunes := []rune(prev)
	currRunes := []rune(curr)
	maxLen := len(prevRunes)
	if len(currRunes) > maxLen {
		maxLen = len(currRunes)
	}
	changes := 0
	for i := 0; i < maxLen; i++ {
		var a, b rune
		if i < len(prevRunes) {
			a = prevRunes[i]
		}
		if i < len(currRunes) {
			b = currRunes[i]
		}
		if a != b {
			changes++
		}
	}
	return changes
}

// WriteRecording saves a Recording to a directory.
func WriteRecording(rec *Recording, outDir string) error {
	dir := filepath.Join(outDir, rec.Scenario)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// Write frames as JSONL
	framesPath := filepath.Join(dir, "frames.jsonl")
	f, err := os.Create(framesPath)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(f)
	for _, frame := range rec.Frames {
		if err := enc.Encode(frame); err != nil {
			f.Close()
			return err
		}
	}
	f.Close()

	// Write metadata
	meta := map[string]interface{}{
		"app_name":     rec.AppName,
		"scenario":     rec.Scenario,
		"cols":         rec.Cols,
		"rows":         rec.Rows,
		"start_time":   rec.StartTime.Format(time.RFC3339Nano),
		"duration_ms":  rec.Duration.Milliseconds(),
		"frame_count":  len(rec.Frames),
		"input_count":  len(rec.Inputs),
		"snapshot_count": len(rec.Snapshots),
	}
	metaData, _ := json.MarshalIndent(meta, "", "  ")
	os.WriteFile(filepath.Join(dir, "metadata.json"), metaData, 0644)

	// Write named snapshots
	if len(rec.Snapshots) > 0 {
		snapDir := filepath.Join(dir, "snapshots")
		os.MkdirAll(snapDir, 0755)
		for name, screen := range rec.Snapshots {
			os.WriteFile(filepath.Join(snapDir, name+".txt"), []byte(screen), 0644)
		}
	}

	// Write inputs as JSONL
	if len(rec.Inputs) > 0 {
		inputsPath := filepath.Join(dir, "inputs.jsonl")
		f2, err := os.Create(inputsPath)
		if err != nil {
			return err
		}
		enc2 := json.NewEncoder(f2)
		for _, input := range rec.Inputs {
			enc2.Encode(input)
		}
		f2.Close()
	}

	fmt.Fprintf(os.Stderr, "  [%s] %d frames, %d inputs, %d snapshots → %s\n",
		rec.Scenario, len(rec.Frames), len(rec.Inputs), len(rec.Snapshots), dir)
	return nil
}
