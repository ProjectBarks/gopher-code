// Source: src/cli/handlers/util.tsx — doctorHandler
package handlers

import (
	"fmt"
	"io"
)

// DoctorOpts configures the doctor handler.
type DoctorOpts struct {
	Output io.Writer // defaults to os.Stdout
}

// Doctor handles `claude doctor`.
// It launches the diagnostics screen (Doctor TUI component).
// Source: src/cli/handlers/util.tsx — doctorHandler
func Doctor(opts DoctorOpts) int {
	w := output(opts.Output)

	// Analytics: tengu_doctor_command (stub — T-analytics)

	// TODO(T244): launch Doctor screen TUI component here.
	// For now the handler acknowledges the command.
	fmt.Fprintln(w, "Running diagnostics...")

	return 0
}
