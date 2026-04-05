// capture-tui captures a TUI application's rendered screen by running it in a
// real PTY and feeding output through a VT100 terminal emulator. This gives
// you the actual screen content (what your eyes see), not the raw ANSI stream.
//
// Simple mode (original):
//
//	go run ./scripts/capture-tui                        # capture claude
//	go run ./scripts/capture-tui -app gopher            # capture gopher
//	go run ./scripts/capture-tui -diff                  # capture both and diff
//
// Scenario mode (new):
//
//	go run ./scripts/capture-tui -scenario scenarios/welcome.json -app claude
//	go run ./scripts/capture-tui -scenario-dir scenarios/ -app claude -record
//	go run ./scripts/capture-tui -scenario-dir scenarios/ -app claude -record -loop
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/creack/pty"
	"github.com/hinshun/vt10x"
)

type snapshot struct {
	name   string
	screen string
}

func captureApp(appName, query, cwd string, cols, rows int, waitWelcome, waitQuery time.Duration) ([]snapshot, error) {
	var cmd *exec.Cmd
	switch appName {
	case "claude":
		cmd = exec.Command("claude")
	case "gopher":
		cmd = exec.Command("go", "run", "./cmd/gopher-code")
	default:
		return nil, fmt.Errorf("unknown app: %s (use 'claude' or 'gopher')", appName)
	}

	cmd.Dir = cwd
	cmd.Env = append(os.Environ(),
		"TERM=xterm-256color",
		fmt.Sprintf("COLUMNS=%d", cols),
		fmt.Sprintf("LINES=%d", rows),
	)

	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{Rows: uint16(rows), Cols: uint16(cols)})
	if err != nil {
		return nil, fmt.Errorf("pty start: %w", err)
	}
	defer ptmx.Close()

	// Create a virtual terminal that processes all ANSI sequences
	term := vt10x.New(vt10x.WithSize(cols, rows))

	// Feed PTY output into the VT emulator continuously
	done := make(chan struct{})
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
		close(done)
	}()

	var snaps []snapshot

	// 1. Wait for welcome/startup screen
	fmt.Fprintf(os.Stderr, "[%s] Waiting %s for startup...\n", appName, waitWelcome)
	time.Sleep(waitWelcome)
	snaps = append(snaps, snapshot{
		name:   "01-welcome",
		screen: termToString(term, cols, rows),
	})

	// 2. Type query if provided
	if query != "" {
		fmt.Fprintf(os.Stderr, "[%s] Typing: %s\n", appName, query)
		io.WriteString(ptmx, query+"\r")

		fmt.Fprintf(os.Stderr, "[%s] Waiting %s for response...\n", appName, waitQuery)
		time.Sleep(waitQuery)
		snaps = append(snaps, snapshot{
			name:   "02-response",
			screen: termToString(term, cols, rows),
		})
	}

	// 3. Exit
	fmt.Fprintf(os.Stderr, "[%s] Sending Ctrl+C...\n", appName)
	io.WriteString(ptmx, "\x03")
	time.Sleep(1 * time.Second)
	io.WriteString(ptmx, "\x03")
	time.Sleep(500 * time.Millisecond)

	cmd.Process.Kill()
	<-done

	snaps = append(snaps, snapshot{
		name:   "03-final",
		screen: termToString(term, cols, rows),
	})

	return snaps, nil
}

// termToString reads the virtual terminal's screen buffer as a string,
// trimming trailing whitespace from each line.
func termToString(term vt10x.Terminal, cols, rows int) string {
	term.Lock()
	defer term.Unlock()

	var lines []string
	for y := 0; y < rows; y++ {
		var line strings.Builder
		for x := 0; x < cols; x++ {
			g := term.Cell(x, y)
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

func writeSnapshots(outDir, appName string, snaps []snapshot) error {
	dir := filepath.Join(outDir, appName)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	for _, s := range snaps {
		path := filepath.Join(dir, s.name+".txt")
		os.WriteFile(path, []byte(s.screen), 0644)
		fmt.Printf("  %s/%s.txt (%d bytes)\n", appName, s.name, len(s.screen))
	}
	return nil
}

func main() {
	// Simple mode flags (original)
	app := flag.String("app", "claude", "App to capture: 'claude' or 'gopher'")
	query := flag.String("query", "say hi", "Query to type after startup (simple mode)")
	outDir := flag.String("out", "testdata/tui-snapshots", "Output directory")
	cwd := flag.String("cwd", "", "Working directory (defaults to repo root)")
	doDiff := flag.Bool("diff", false, "Capture both apps and show diff (simple mode)")
	cols := flag.Int("cols", 80, "Terminal width")
	rows := flag.Int("rows", 24, "Terminal height")
	welcomeWait := flag.Duration("welcome-wait", 6*time.Second, "Welcome screen wait (simple mode)")
	queryWait := flag.Duration("query-wait", 10*time.Second, "Query response wait (simple mode)")

	// Scenario mode flags (new)
	scenarioPath := flag.String("scenario", "", "Path to a single scenario JSON file")
	scenarioDir := flag.String("scenario-dir", "", "Path to a directory of scenario JSON files")
	record := flag.Bool("record", false, "Record all frame transitions (not just named snapshots)")
	loop := flag.Bool("loop", false, "Loop: re-run failed scenarios until all pass or no progress")

	flag.Parse()

	if *cwd == "" {
		home, _ := os.UserHomeDir()
		*cwd = filepath.Join(home, "claude-code-v2", "gopher")
	}

	// Decide which mode to run
	if *scenarioPath != "" || *scenarioDir != "" {
		runScenarioMode(*app, *cwd, *outDir, *scenarioPath, *scenarioDir, *record, *loop)
		return
	}

	// --- Simple mode (original behavior) ---
	if *doDiff {
		fmt.Println("Capturing Claude Code...")
		cSnaps, err := captureApp("claude", *query, *cwd, *cols, *rows, *welcomeWait, *queryWait)
		if err != nil {
			fmt.Fprintf(os.Stderr, "claude capture failed: %v\n", err)
			os.Exit(1)
		}
		writeSnapshots(*outDir, "claude", cSnaps)

		fmt.Println("\nCapturing Gopher...")
		gSnaps, err := captureApp("gopher", *query, *cwd, *cols, *rows, *welcomeWait, *queryWait)
		if err != nil {
			fmt.Fprintf(os.Stderr, "gopher capture failed: %v\n", err)
			os.Exit(1)
		}
		writeSnapshots(*outDir, "gopher", gSnaps)

		fmt.Println("\n=== DIFFS ===")
		for i := range cSnaps {
			if i >= len(gSnaps) {
				break
			}
			cFile := filepath.Join(*outDir, "claude", cSnaps[i].name+".txt")
			gFile := filepath.Join(*outDir, "gopher", gSnaps[i].name+".txt")
			fmt.Printf("\n--- %s ---\n", cSnaps[i].name)
			diffCmd := exec.Command("diff", "-u", "--label", "claude", "--label", "gopher", cFile, gFile)
			diffCmd.Stdout = os.Stdout
			diffCmd.Run()
		}
		return
	}

	fmt.Printf("Capturing %s...\n", *app)
	snaps, err := captureApp(*app, *query, *cwd, *cols, *rows, *welcomeWait, *queryWait)
	if err != nil {
		fmt.Fprintf(os.Stderr, "capture failed: %v\n", err)
		os.Exit(1)
	}
	writeSnapshots(*outDir, *app, snaps)
	fmt.Println("\nDone. Snapshots saved to", *outDir)
}

// runScenarioMode loads and executes scenarios.
func runScenarioMode(appName, cwd, outDir, scenarioPath, scenarioDir string, record, loop bool) {
	// Load scenarios
	var scenarios []*Scenario
	var err error

	if scenarioPath != "" {
		s, err := LoadScenario(scenarioPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading scenario: %v\n", err)
			os.Exit(1)
		}
		scenarios = []*Scenario{s}
	} else {
		scenarios, err = LoadScenariosRecursive(scenarioDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading scenarios from %s: %v\n", scenarioDir, err)
			os.Exit(1)
		}
	}

	if len(scenarios) == 0 {
		fmt.Fprintf(os.Stderr, "No scenarios found\n")
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "Loaded %d scenario(s) for %s\n", len(scenarios), appName)

	runner := &Runner{
		AppName: appName,
		CWD:     cwd,
		Record:  record,
		OutDir:  outDir,
	}

	if loop {
		runLoop(runner, scenarios, outDir, appName)
	} else {
		runOnce(runner, scenarios, outDir, appName)
	}
}

// runOnce executes all scenarios once and reports results.
func runOnce(runner *Runner, scenarios []*Scenario, outDir, appName string) {
	var passed, failed int
	for i, s := range scenarios {
		fmt.Fprintf(os.Stderr, "\n[%d/%d] Running: %s\n", i+1, len(scenarios), s.Name)
		result := runner.Run(s)

		if err := WriteResult(result, outDir); err != nil {
			fmt.Fprintf(os.Stderr, "  Error writing result: %v\n", err)
		}

		// Copy debug log if capturing claude
		if appName == "claude" {
			CopyDebugLog(outDir, s.Name, s.Category)
		}

		if result.Error != nil {
			fmt.Fprintf(os.Stderr, "  FAIL: %v\n", result.Error)
			failed++
		} else {
			fmt.Fprintf(os.Stderr, "  PASS (%s, %d snapshots)\n",
				result.Duration.Round(time.Millisecond), len(result.Snapshots))
			passed++
		}
	}

	fmt.Fprintf(os.Stderr, "\n=== Results: %d passed, %d failed, %d total ===\n",
		passed, failed, len(scenarios))
	if failed > 0 {
		os.Exit(1)
	}
}

// runLoop re-runs failed scenarios until all pass or no progress is made.
func runLoop(runner *Runner, scenarios []*Scenario, outDir, appName string) {
	remaining := make([]*Scenario, len(scenarios))
	copy(remaining, scenarios)
	iteration := 0

	for len(remaining) > 0 {
		iteration++
		fmt.Fprintf(os.Stderr, "\n=== Loop iteration %d: %d scenario(s) remaining ===\n",
			iteration, len(remaining))

		var failed []*Scenario
		for i, s := range remaining {
			fmt.Fprintf(os.Stderr, "\n[%d/%d] Running: %s\n", i+1, len(remaining), s.Name)
			result := runner.Run(s)

			if err := WriteResult(result, outDir); err != nil {
				fmt.Fprintf(os.Stderr, "  Error writing result: %v\n", err)
			}

			if appName == "claude" {
				CopyDebugLog(outDir, s.Name, s.Category)
			}

			if result.Error != nil {
				fmt.Fprintf(os.Stderr, "  FAIL: %v\n", result.Error)
				failed = append(failed, s)
			} else {
				fmt.Fprintf(os.Stderr, "  PASS (%s)\n", result.Duration.Round(time.Millisecond))
			}
		}

		if len(failed) == 0 {
			fmt.Fprintf(os.Stderr, "\n=== All scenarios passed after %d iteration(s) ===\n", iteration)
			return
		}

		if len(failed) == len(remaining) {
			fmt.Fprintf(os.Stderr, "\n=== No progress: %d scenario(s) still failing after %d iteration(s) ===\n",
				len(failed), iteration)
			os.Exit(1)
		}

		remaining = failed
	}
}
