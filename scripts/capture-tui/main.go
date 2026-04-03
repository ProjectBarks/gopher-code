// capture-tui captures a TUI application's terminal output by running it in a
// real PTY, scripting interactions, and saving text snapshots at each stage.
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
	"regexp"
	"strings"
	"time"

	"github.com/creack/pty"
)

// cursorMoveRe matches cursor-forward sequences like \x1b[1C (move right 1)
var cursorMoveRe = regexp.MustCompile(`\x1b\[(\d*)C`)
var ansiRe = regexp.MustCompile(`\x1b\[[0-9;?]*[a-zA-Z]|\x1b\][^\x07]*\x07|\x1b>[0-9]*[a-zA-Z]|\x1b\[[<>=][^\n]*`)

func stripANSI(b []byte) string {
	s := string(b)
	// Replace cursor-forward moves with spaces (e.g. \x1b[1C → " ", \x1b[3C → "   ")
	s = cursorMoveRe.ReplaceAllStringFunc(s, func(match string) string {
		sub := cursorMoveRe.FindStringSubmatch(match)
		n := 1
		if len(sub) > 1 && sub[1] != "" {
			fmt.Sscanf(sub[1], "%d", &n)
		}
		return strings.Repeat(" ", n)
	})
	// Strip remaining ANSI sequences
	s = ansiRe.ReplaceAllString(s, "")
	var sb strings.Builder
	for _, r := range s {
		if r == '\r' || r == 0 {
			continue
		}
		sb.WriteRune(r)
	}
	return sb.String()
}

type snapshot struct {
	name    string
	raw     []byte
	cleaned string
}

func captureApp(appName, query, cwd string, waitWelcome, waitQuery time.Duration) ([]snapshot, error) {
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
		"COLUMNS=80",
		"LINES=24",
	)

	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{Rows: 24, Cols: 80})
	if err != nil {
		return nil, fmt.Errorf("pty start: %w", err)
	}
	defer ptmx.Close()

	// Read output continuously
	output := make([]byte, 0, 256*1024)
	done := make(chan struct{})
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := ptmx.Read(buf)
			if n > 0 {
				output = append(output, buf[:n]...)
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
		name:    "01-welcome",
		raw:     append([]byte(nil), output...),
		cleaned: stripANSI(output),
	})

	// 2. Type query if provided
	if query != "" {
		fmt.Fprintf(os.Stderr, "[%s] Typing: %s\n", appName, query)
		io.WriteString(ptmx, query+"\r")

		fmt.Fprintf(os.Stderr, "[%s] Waiting %s for response...\n", appName, waitQuery)
		time.Sleep(waitQuery)
		snaps = append(snaps, snapshot{
			name:    "02-response",
			raw:     append([]byte(nil), output...),
			cleaned: stripANSI(output),
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
		name:    "03-final",
		raw:     append([]byte(nil), output...),
		cleaned: stripANSI(output),
	})

	return snaps, nil
}

func writeSnapshots(outDir, appName string, snaps []snapshot) error {
	dir := filepath.Join(outDir, appName)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	for _, s := range snaps {
		rawPath := filepath.Join(dir, s.name+".raw.txt")
		cleanPath := filepath.Join(dir, s.name+".txt")
		os.WriteFile(rawPath, s.raw, 0644)
		os.WriteFile(cleanPath, []byte(s.cleaned), 0644)
		fmt.Printf("  %s/%s.txt (%d bytes clean, %d bytes raw)\n", appName, s.name, len(s.cleaned), len(s.raw))
	}
	return nil
}

func main() {
	app := flag.String("app", "claude", "App to capture: 'claude' or 'gopher'")
	query := flag.String("query", "say hi", "Query to type after startup")
	outDir := flag.String("out", "testdata/tui-snapshots", "Output directory for snapshots")
	cwd := flag.String("cwd", "", "Working directory (defaults to repo root)")
	doDiff := flag.Bool("diff", false, "Capture both apps and show diff")
	welcomeWait := flag.Duration("welcome-wait", 6*time.Second, "How long to wait for welcome screen")
	queryWait := flag.Duration("query-wait", 10*time.Second, "How long to wait for query response")
	flag.Parse()

	if *cwd == "" {
		// Default to the gopher repo root
		home, _ := os.UserHomeDir()
		*cwd = filepath.Join(home, "claude-code-v2", "gopher")
	}

	if *doDiff {
		// Capture both and diff
		fmt.Println("Capturing Claude Code...")
		cSnaps, err := captureApp("claude", *query, *cwd, *welcomeWait, *queryWait)
		if err != nil {
			fmt.Fprintf(os.Stderr, "claude capture failed: %v\n", err)
			os.Exit(1)
		}
		writeSnapshots(*outDir, "claude", cSnaps)

		fmt.Println("\nCapturing Gopher...")
		gSnaps, err := captureApp("gopher", *query, *cwd, *welcomeWait, *queryWait)
		if err != nil {
			fmt.Fprintf(os.Stderr, "gopher capture failed: %v\n", err)
			os.Exit(1)
		}
		writeSnapshots(*outDir, "gopher", gSnaps)

		// Show diff for each snapshot
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
			diffCmd.Run() // ignore exit code (diff returns 1 if files differ)
		}
		return
	}

	// Single app capture
	fmt.Printf("Capturing %s...\n", *app)
	snaps, err := captureApp(*app, *query, *cwd, *welcomeWait, *queryWait)
	if err != nil {
		fmt.Fprintf(os.Stderr, "capture failed: %v\n", err)
		os.Exit(1)
	}
	writeSnapshots(*outDir, *app, snaps)
	fmt.Println("\nDone. Snapshots saved to", *outDir)
}
