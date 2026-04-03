// capture-tui captures a TUI application's rendered screen by running it in a
// real PTY and feeding output through a VT100 terminal emulator. This gives
// you the actual screen content (what your eyes see), not the raw ANSI stream.
//
// Usage:
//
//	go run ./scripts/capture-tui                        # capture claude
//	go run ./scripts/capture-tui -app gopher            # capture gopher
//	go run ./scripts/capture-tui -app claude -query "explain this code"
//	go run ./scripts/capture-tui -out /tmp/snapshots    # custom output dir
//	go run ./scripts/capture-tui -diff                  # capture both and diff
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
	app := flag.String("app", "claude", "App to capture: 'claude' or 'gopher'")
	query := flag.String("query", "say hi", "Query to type after startup")
	outDir := flag.String("out", "testdata/tui-snapshots", "Output directory for snapshots")
	cwd := flag.String("cwd", "", "Working directory (defaults to repo root)")
	doDiff := flag.Bool("diff", false, "Capture both apps and show diff")
	cols := flag.Int("cols", 80, "Terminal width")
	rows := flag.Int("rows", 24, "Terminal height")
	welcomeWait := flag.Duration("welcome-wait", 6*time.Second, "How long to wait for welcome screen")
	queryWait := flag.Duration("query-wait", 10*time.Second, "How long to wait for query response")
	flag.Parse()

	if *cwd == "" {
		home, _ := os.UserHomeDir()
		*cwd = filepath.Join(home, "claude-code-v2", "gopher")
	}

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
