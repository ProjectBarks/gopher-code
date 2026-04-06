package main

import (
	"fmt"
	"io"
	"os"
)

// exitFunc is the function called to terminate the process.
// Tests replace this to capture the exit code without killing the process.
var exitFunc = os.Exit

// stderr and stdout writers; tests replace these to capture output.
var (
	stderr io.Writer = os.Stderr
	stdout io.Writer = os.Stdout
)

// cliError writes msg to stderr (if non-empty) and exits with code 1.
func cliError(msg string) {
	if msg != "" {
		fmt.Fprintln(stderr, msg)
	}
	exitFunc(1)
}

// cliOk writes msg to stdout (if non-empty) and exits with code 0.
func cliOk(msg string) {
	if msg != "" {
		fmt.Fprintln(stdout, msg)
	}
	exitFunc(0)
}
