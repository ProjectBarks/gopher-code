package server

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"
)

// shortSockDir returns a short temp directory suitable for unix sockets.
// macOS has a 104-byte path limit for unix socket addresses, and t.TempDir()
// paths can exceed that. We use /tmp with a short prefix instead.
func shortSockDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "gs")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return dir
}

// ---------------------------------------------------------------------------
// T95: Unix socket + TCP listener integration tests
// ---------------------------------------------------------------------------

func TestListen_UnixSocket(t *testing.T) {
	dir := shortSockDir(t)
	sockPath := filepath.Join(dir, "t.sock")

	cfg := ServerConfig{Unix: sockPath}

	ln, err := Listen(cfg)
	if err != nil {
		t.Fatalf("Listen() error: %v", err)
	}
	defer ln.Close()

	// Verify socket file exists.
	info, err := os.Lstat(sockPath)
	if err != nil {
		t.Fatalf("socket file missing: %v", err)
	}
	if info.Mode().Type() != os.ModeSocket {
		t.Errorf("expected socket file, got mode %v", info.Mode())
	}

	// Verify we can connect to it.
	conn, err := net.Dial("unix", sockPath)
	if err != nil {
		t.Fatalf("Dial() error: %v", err)
	}
	conn.Close()
}

func TestListen_UnixSocket_RemovesStaleFile(t *testing.T) {
	dir := shortSockDir(t)
	sockPath := filepath.Join(dir, "s.sock")

	// Create a stale file at the socket path.
	os.WriteFile(sockPath, []byte("stale"), 0o644)

	cfg := ServerConfig{Unix: sockPath}
	ln, err := Listen(cfg)
	if err != nil {
		t.Fatalf("Listen() should remove stale file: %v", err)
	}
	defer ln.Close()

	// Verify it's now a real socket.
	conn, err := net.Dial("unix", sockPath)
	if err != nil {
		t.Fatalf("Dial() error: %v", err)
	}
	conn.Close()
}

func TestListen_UnixSocket_CreatesParentDir(t *testing.T) {
	dir := shortSockDir(t)
	nested := filepath.Join(dir, "n")
	sockPath := filepath.Join(nested, "d.sock")

	cfg := ServerConfig{Unix: sockPath}
	ln, err := Listen(cfg)
	if err != nil {
		t.Fatalf("Listen() should create parent dirs: %v", err)
	}
	defer ln.Close()

	conn, err := net.Dial("unix", sockPath)
	if err != nil {
		t.Fatalf("Dial() error: %v", err)
	}
	conn.Close()
}

func TestListen_TCP(t *testing.T) {
	cfg := ServerConfig{
		Host: "127.0.0.1",
		Port: 0, // port 0 = OS picks a free port
	}

	ln, err := Listen(cfg)
	if err != nil {
		t.Fatalf("Listen() error: %v", err)
	}
	defer ln.Close()

	addr := ln.Addr().(*net.TCPAddr)
	if addr.Port == 0 {
		t.Error("expected assigned port, got 0")
	}

	// Verify we can connect.
	conn, err := net.Dial("tcp", addr.String())
	if err != nil {
		t.Fatalf("Dial() error: %v", err)
	}
	conn.Close()
}

func TestListen_UnixPreferredOverTCP(t *testing.T) {
	dir := shortSockDir(t)
	sockPath := filepath.Join(dir, "p.sock")

	cfg := ServerConfig{
		Host: "127.0.0.1",
		Port: 9999,
		Unix: sockPath,
	}

	ln, err := Listen(cfg)
	if err != nil {
		t.Fatalf("Listen() error: %v", err)
	}
	defer ln.Close()

	// Should be a Unix listener, not TCP.
	if ln.Addr().Network() != "unix" {
		t.Errorf("expected unix network, got %q", ln.Addr().Network())
	}
}

func TestListen_TCPAddress(t *testing.T) {
	cfg := ServerConfig{
		Host: "127.0.0.1",
		Port: 0,
	}

	ln, err := Listen(cfg)
	if err != nil {
		t.Fatalf("Listen() error: %v", err)
	}
	defer ln.Close()

	addr := ln.Addr().(*net.TCPAddr)
	expected := fmt.Sprintf("127.0.0.1:%d", addr.Port)
	if addr.String() != expected {
		t.Errorf("addr = %q, want %q", addr.String(), expected)
	}
}
