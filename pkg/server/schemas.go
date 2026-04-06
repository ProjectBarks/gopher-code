// Package server — validation schemas for server types.
// Source: src/server/types.ts (connectResponseSchema)
package server

import "fmt"

// ValidateConnectResponse validates a ConnectResponse struct.
// Mirrors the connectResponseSchema Zod validator from TS:
//
//	z.object({ session_id: z.string(), ws_url: z.string(), work_dir: z.string().optional() })
//
// Returns nil if valid, or an error describing the first invalid field.
func ValidateConnectResponse(r ConnectResponse) error {
	if r.SessionID == "" {
		return fmt.Errorf("connect response: session_id is required")
	}
	if r.WSURL == "" {
		return fmt.Errorf("connect response: ws_url is required")
	}
	// work_dir is optional — no validation needed.
	return nil
}
