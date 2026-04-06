// Package server — network listeners for the session daemon.
// Source: src/server/types.ts (unix socket support)
package server

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
)

// Listen creates a net.Listener based on the ServerConfig.
// If cfg.Unix is set, listens on a Unix domain socket (removing any stale socket file).
// Otherwise, listens on TCP at cfg.Host:cfg.Port.
func Listen(cfg ServerConfig) (net.Listener, error) {
	if cfg.Unix != "" {
		return listenUnix(cfg.Unix)
	}
	return listenTCP(cfg.Host, cfg.Port)
}

func listenTCP(host string, port int) (net.Listener, error) {
	addr := fmt.Sprintf("%s:%d", host, port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("listen tcp %s: %w", addr, err)
	}
	return ln, nil
}

func listenUnix(path string) (net.Listener, error) {
	// Ensure parent directory exists.
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create socket dir: %w", err)
	}

	// Remove stale socket file if it exists (unix sockets leave files behind).
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("remove stale socket: %w", err)
	}

	ln, err := net.Listen("unix", path)
	if err != nil {
		return nil, fmt.Errorf("listen unix %s: %w", path, err)
	}
	return ln, nil
}
