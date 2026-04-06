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

const (
	EToolUseSummaryGenerationFailed = 344
)
