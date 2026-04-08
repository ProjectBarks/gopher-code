// Package errors provides numeric error IDs for tracking error sources in
// production. These obfuscated identifiers let us trace which logError() call
// generated an error without leaking internal names to external builds.
//
// ADDING A NEW ERROR ID:
//  1. Add a constant based on Next ID.
//  2. Increment Next ID.
//
// Next ID: 346
package errors

import "fmt"

const (
	EToolUseSummaryGenerationFailed = 344
)

// FormatErrorID returns a string representation of an error ID suitable for
// inclusion in structured log attributes (e.g. "error_id=344"). The numeric
// form matches the TS source so production traces stay consistent across
// builds.
func FormatErrorID(id int) string {
	return fmt.Sprintf("%d", id)
}
