// Bridge pointer persistence for crash-recovery of Remote Control sessions.
// Source: src/bridge/bridgePointer.ts
package bridge

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/projectbarks/gopher-code/pkg/session"
)

// BridgePointerTTL is the maximum age before a pointer is considered stale.
// Matches the backend's rolling BRIDGE_LAST_POLL_TTL (4 hours).
// Source: bridgePointer.ts:40
const BridgePointerTTL = 4 * time.Hour

// MaxWorktreeFanout caps the parallel stat burst when scanning worktree
// siblings for a pointer. Above this, --continue falls back to current-dir-only.
// Source: bridgePointer.ts:19
const MaxWorktreeFanout = 50

// PointerSource identifies how the bridge session was started.
type PointerSource string

const (
	PointerSourceStandalone PointerSource = "standalone"
	PointerSourceREPL       PointerSource = "repl"
)

// BridgePointer is the crash-recovery pointer written after bridge attach.
// Source: bridgePointer.ts:42-48
type BridgePointer struct {
	SessionID     string        `json:"sessionId"`
	EnvironmentID string        `json:"environmentId"`
	Source        PointerSource `json:"source"`
}

// BridgePointerWithAge is a BridgePointer together with its staleness.
type BridgePointerWithAge struct {
	BridgePointer
	AgeMs int64 `json:"-"`
}

// GetBridgePointerPath returns the filesystem path for the pointer file
// scoped to the given working directory.
// Source: bridgePointer.ts:52-54
func GetBridgePointerPath(dir string) string {
	return filepath.Join(session.GetProjectsDir(), session.SanitizeDirName(dir), "bridge-pointer.json")
}

// WriteBridgePointer persists (or refreshes) the pointer. Best-effort:
// logs and swallows errors so the crash-recovery file never itself causes a crash.
// Source: bridgePointer.ts:62-74
func WriteBridgePointer(dir string, ptr BridgePointer, log *BridgeDebug) error {
	if err := validatePointer(ptr); err != nil {
		if log != nil {
			log.log(LogLevelWarn, fmt.Sprintf("[bridge:pointer] write failed: %v", err), nil)
		}
		return err
	}
	path := GetBridgePointerPath(dir)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		if log != nil {
			log.log(LogLevelWarn, fmt.Sprintf("[bridge:pointer] write failed: %v", err), nil)
		}
		return err
	}
	data, err := json.Marshal(ptr)
	if err != nil {
		if log != nil {
			log.log(LogLevelWarn, fmt.Sprintf("[bridge:pointer] write failed: %v", err), nil)
		}
		return err
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		if log != nil {
			log.log(LogLevelWarn, fmt.Sprintf("[bridge:pointer] write failed: %v", err), nil)
		}
		return err
	}
	if log != nil {
		log.log(LogLevelDebug, fmt.Sprintf("[bridge:pointer] wrote %s", path), nil)
	}
	return nil
}

// ReadBridgePointer loads the pointer and checks staleness. Returns nil on
// any failure: missing file, bad JSON, schema mismatch, or stale (>4h).
// Stale/invalid pointers are deleted.
// Source: bridgePointer.ts:83-113
func ReadBridgePointer(dir string, log *BridgeDebug) *BridgePointerWithAge {
	path := GetBridgePointerPath(dir)

	info, err := os.Stat(path)
	if err != nil {
		return nil
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	var ptr BridgePointer
	if err := json.Unmarshal(raw, &ptr); err != nil {
		if log != nil {
			log.log(LogLevelDebug, fmt.Sprintf("[bridge:pointer] invalid schema, clearing: %s", path), nil)
		}
		ClearBridgePointer(dir, log)
		return nil
	}
	if err := validatePointer(ptr); err != nil {
		if log != nil {
			log.log(LogLevelDebug, fmt.Sprintf("[bridge:pointer] invalid schema, clearing: %s", path), nil)
		}
		ClearBridgePointer(dir, log)
		return nil
	}

	ageMs := time.Since(info.ModTime()).Milliseconds()
	if ageMs < 0 {
		ageMs = 0
	}
	if time.Duration(ageMs)*time.Millisecond > BridgePointerTTL {
		if log != nil {
			log.log(LogLevelDebug, fmt.Sprintf("[bridge:pointer] stale (>4h mtime), clearing: %s", path), nil)
		}
		ClearBridgePointer(dir, log)
		return nil
	}

	return &BridgePointerWithAge{
		BridgePointer: ptr,
		AgeMs:         ageMs,
	}
}

// ClearBridgePointer deletes the pointer. Idempotent: ENOENT is expected
// when the process shut down cleanly.
// Source: bridgePointer.ts:190-202
func ClearBridgePointer(dir string, log *BridgeDebug) {
	path := GetBridgePointerPath(dir)
	err := os.Remove(path)
	if err == nil {
		if log != nil {
			log.log(LogLevelDebug, fmt.Sprintf("[bridge:pointer] cleared %s", path), nil)
		}
		return
	}
	if errors.Is(err, os.ErrNotExist) {
		return
	}
	if log != nil {
		log.log(LogLevelWarn, fmt.Sprintf("[bridge:pointer] clear failed: %v", err), nil)
	}
}

// validatePointer checks that required fields are present and Source is valid.
func validatePointer(ptr BridgePointer) error {
	if ptr.SessionID == "" {
		return errors.New("sessionId is required")
	}
	if ptr.EnvironmentID == "" {
		return errors.New("environmentId is required")
	}
	switch ptr.Source {
	case PointerSourceStandalone, PointerSourceREPL:
		// ok
	default:
		return fmt.Errorf("invalid source: %q", ptr.Source)
	}
	return nil
}
