// Source: src/cli/handlers/util.tsx — installHandler
package handlers

import (
	"fmt"
	"io"
	"strings"
)

// InstallOpts configures the install handler.
type InstallOpts struct {
	Target string    // positional: install target (e.g. "alpha", "beta")
	Force  bool      // --force flag
	Output io.Writer // defaults to os.Stdout

	// Installer is the pluggable install engine. Nil uses the default stub.
	// Returns a result string; if it contains "failed" the handler exits 1.
	// Source: src/commands/install.js — install.call(result => ...)
	Installer func(target string, force bool) (string, error)
}

// Install handles `claude install [target] [--force]`.
// Source: src/cli/handlers/util.tsx — installHandler
func Install(opts InstallOpts) int {
	w := output(opts.Output)

	installer := opts.Installer
	if installer == nil {
		installer = defaultInstaller
	}

	result, err := installer(opts.Target, opts.Force)
	if err != nil {
		fmt.Fprintf(w, "Install failed: %v\n", err)
		return 1
	}

	fmt.Fprintln(w, result)

	// Source: process.exit(result.includes('failed') ? 1 : 0)
	if strings.Contains(result, "failed") {
		return 1
	}
	return 0
}

// defaultInstaller is a stub install engine.
// TODO: replace with real install.Run(ctx, opts, args) from pkg/install.
func defaultInstaller(target string, force bool) (string, error) {
	msg := "Install complete"
	if target != "" {
		msg = fmt.Sprintf("Install complete for target %q", target)
	}
	if force {
		msg += " (forced)"
	}
	return msg, nil
}
