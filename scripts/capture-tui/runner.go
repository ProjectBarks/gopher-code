package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"time"

	"github.com/creack/pty"
	"github.com/hinshun/vt10x"
)

// Runner executes scenarios against a TUI application.
type Runner struct {
	AppName string
	CWD     string
	Record  bool   // capture all frame transitions
	OutDir  string // output directory for recordings
}

// RunResult holds the outcome of a scenario execution.
type RunResult struct {
	Scenario  *Scenario
	Recording *Recording            // nil if Record=false
	Snapshots map[string]string     // named snapshots
	Cells     map[string][]CellData // named snapshot cell data
	Duration  time.Duration
	Error     error
}

// Run executes a single scenario and returns the result.
func (r *Runner) Run(s *Scenario) *RunResult {
	start := time.Now()
	result := &RunResult{
		Scenario:  s,
		Snapshots: make(map[string]string),
		Cells:     make(map[string][]CellData),
	}

	rec, err := r.execute(s, result)
	result.Duration = time.Since(start)
	result.Error = err
	if rec != nil {
		result.Recording = rec
	}
	return result
}

func (r *Runner) execute(s *Scenario, result *RunResult) (*Recording, error) {
	// Build command
	cmd, err := r.buildCommand(s)
	if err != nil {
		return nil, fmt.Errorf("build command: %w", err)
	}

	// Start in PTY
	cols, rows := s.Cols, s.Rows
	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{
		Rows: uint16(rows),
		Cols: uint16(cols),
	})
	if err != nil {
		return nil, fmt.Errorf("pty start: %w", err)
	}
	defer ptmx.Close()

	// Create VT terminal emulator
	term := vt10x.New(vt10x.WithSize(cols, rows))

	// Feed PTY output into the VT emulator
	ptyClosed := make(chan struct{})
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := ptmx.Read(buf)
			if n > 0 {
				term.Write(buf[:n])
			}
			if err != nil {
				break
			}
		}
		close(ptyClosed)
	}()

	// Create recorder
	var recorder *Recorder
	if r.Record {
		recorder = NewRecorder(term, cols, rows)
		recorder.Start()
		defer recorder.Stop()
	}

	// Helper to read screen (with or without recorder)
	readScreen := func() string {
		if recorder != nil {
			return recorder.CurrentScreen()
		}
		return termToString(term, cols, rows)
	}

	// Execute steps
	for i, step := range s.Steps {
		fmt.Fprintf(os.Stderr, "  [%s] step %d/%d: %s %s\n",
			s.Name, i+1, len(s.Steps), step.Action, truncate(step.Value, 40))

		switch step.Action {
		case "type":
			io.WriteString(ptmx, step.Value)
			if recorder != nil {
				recorder.RecordInput("type", step.Value)
			} else {
				time.Sleep(200 * time.Millisecond)
			}

		case "key":
			keyBytes, err := KeyToBytes(step.Value)
			if err != nil {
				return recorderResult(recorder, r.AppName, s.Name), err
			}
			ptmx.Write(keyBytes)
			if recorder != nil {
				recorder.RecordInput("key", step.Value)
			} else {
				time.Sleep(200 * time.Millisecond)
			}

		case "wait":
			dur, err := time.ParseDuration(step.Value)
			if err != nil {
				return recorderResult(recorder, r.AppName, s.Name),
					fmt.Errorf("step %d: invalid duration %q: %w", i, step.Value, err)
			}
			time.Sleep(dur)

		case "wait_for":
			timeout := 15 * time.Second
			if step.Timeout != "" {
				t, err := time.ParseDuration(step.Timeout)
				if err != nil {
					return recorderResult(recorder, r.AppName, s.Name),
						fmt.Errorf("step %d: invalid timeout %q: %w", i, step.Timeout, err)
				}
				timeout = t
			}
			pattern, err := regexp.Compile(step.Value)
			if err != nil {
				return recorderResult(recorder, r.AppName, s.Name),
					fmt.Errorf("step %d: invalid regex %q: %w", i, step.Value, err)
			}
			if err := waitForPattern(readScreen, pattern, timeout); err != nil {
				return recorderResult(recorder, r.AppName, s.Name),
					fmt.Errorf("step %d (wait_for %q): %w", i, step.Value, err)
			}

		case "snapshot":
			screen := readScreen()
			result.Snapshots[step.Value] = screen
			if recorder != nil {
				recorder.Snapshot(step.Value)
				result.Cells[step.Value] = recorder.SnapshotCells()
			}
		}
	}

	// Clean shutdown: send Ctrl+C, then kill
	io.WriteString(ptmx, "\x03")
	time.Sleep(500 * time.Millisecond)
	io.WriteString(ptmx, "\x03")
	time.Sleep(300 * time.Millisecond)
	cmd.Process.Kill()

	select {
	case <-ptyClosed:
	case <-time.After(3 * time.Second):
	}

	return recorderResult(recorder, r.AppName, s.Name), nil
}

func (r *Runner) buildCommand(s *Scenario) (*exec.Cmd, error) {
	var cmd *exec.Cmd
	switch r.AppName {
	case "claude":
		cmd = exec.Command("claude", "--debug")
	case "gopher":
		cmd = exec.Command("go", "run", "./cmd/gopher-code")
	default:
		return nil, fmt.Errorf("unknown app: %s (use 'claude' or 'gopher')", r.AppName)
	}

	cmd.Dir = r.CWD
	cmd.Env = append(os.Environ(),
		"TERM=xterm-256color",
		"CLAUDE_CODE_DEBUG_LOG_LEVEL=verbose",
		fmt.Sprintf("COLUMNS=%d", s.Cols),
		fmt.Sprintf("LINES=%d", s.Rows),
	)
	// Add scenario-specific env vars
	for k, v := range s.Env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}
	return cmd, nil
}

// waitForPattern polls the screen until the pattern matches or timeout expires.
func waitForPattern(readScreen func() string, pattern *regexp.Regexp, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	pollInterval := 100 * time.Millisecond

	for time.Now().Before(deadline) {
		screen := readScreen()
		if pattern.MatchString(screen) {
			return nil
		}
		time.Sleep(pollInterval)
	}
	return fmt.Errorf("timed out after %s waiting for pattern %q", timeout, pattern.String())
}

// recorderResult safely extracts a Recording from a recorder (may be nil).
func recorderResult(rec *Recorder, appName, scenarioName string) *Recording {
	if rec == nil {
		return nil
	}
	return rec.Result(appName, scenarioName)
}

// WriteResult saves a RunResult to disk.
func WriteResult(result *RunResult, outDir string) error {
	dir := filepath.Join(outDir, result.Scenario.Category, result.Scenario.Name)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// Write recording if available
	if result.Recording != nil {
		if err := WriteRecording(result.Recording, filepath.Join(outDir, result.Scenario.Category)); err != nil {
			return err
		}
	}

	// Write snapshots
	if len(result.Snapshots) > 0 {
		snapDir := filepath.Join(dir, "snapshots")
		os.MkdirAll(snapDir, 0755)
		for name, screen := range result.Snapshots {
			os.WriteFile(filepath.Join(snapDir, name+".txt"), []byte(screen), 0644)
		}
	}

	// Write cell data
	if len(result.Cells) > 0 {
		cellDir := filepath.Join(dir, "cells")
		os.MkdirAll(cellDir, 0755)
		for name, cells := range result.Cells {
			data, _ := json.MarshalIndent(cells, "", "  ")
			os.WriteFile(filepath.Join(cellDir, name+".json"), data, 0644)
		}
	}

	// Write result metadata
	meta := map[string]interface{}{
		"scenario":  result.Scenario.Name,
		"category":  result.Scenario.Category,
		"duration_ms": result.Duration.Milliseconds(),
		"snapshots": len(result.Snapshots),
		"success":   result.Error == nil,
	}
	if result.Error != nil {
		meta["error"] = result.Error.Error()
	}
	if result.Recording != nil {
		meta["frames"] = len(result.Recording.Frames)
		meta["inputs"] = len(result.Recording.Inputs)
	}
	metaData, _ := json.MarshalIndent(meta, "", "  ")
	os.WriteFile(filepath.Join(dir, "result.json"), metaData, 0644)

	return nil
}

// CopyDebugLog copies the latest Claude debug log to the output directory.
func CopyDebugLog(outDir, scenarioName, category string) {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	latestLink := filepath.Join(home, ".claude", "debug", "latest")
	target, err := os.Readlink(latestLink)
	if err != nil {
		return
	}
	// Resolve relative symlink
	if !filepath.IsAbs(target) {
		target = filepath.Join(filepath.Dir(latestLink), target)
	}
	data, err := os.ReadFile(target)
	if err != nil || len(data) == 0 {
		return
	}
	destDir := filepath.Join(outDir, category, scenarioName)
	os.MkdirAll(destDir, 0755)
	os.WriteFile(filepath.Join(destDir, "debug.txt"), data, 0644)
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}
