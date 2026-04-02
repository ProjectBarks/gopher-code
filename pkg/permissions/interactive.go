package permissions

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
)

// InteractivePolicy prompts the user for permission on mutating tools.
type InteractivePolicy struct {
	reader      io.Reader
	mu          sync.Mutex
	alwaysAllow map[string]bool
}

func NewInteractivePolicy() *InteractivePolicy {
	return &InteractivePolicy{
		reader:      os.Stdin,
		alwaysAllow: make(map[string]bool),
	}
}

// NewInteractivePolicyWithReader creates a policy with a custom reader (for testing).
func NewInteractivePolicyWithReader(r io.Reader) *InteractivePolicy {
	return &InteractivePolicy{
		reader:      r,
		alwaysAllow: make(map[string]bool),
	}
}

func (p *InteractivePolicy) Check(_ context.Context, toolName string, _ string) PermissionDecision {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.alwaysAllow[toolName] {
		return AllowDecision{}
	}

	fmt.Printf("\n\033[33m⚠ Tool '%s' requires permission.\033[0m\n", toolName)
	fmt.Printf("  Allow? [\033[32my\033[0m]es / [\033[31mn\033[0m]o / [\033[36ma\033[0m]lways: ")

	scanner := bufio.NewScanner(p.reader)
	if !scanner.Scan() {
		return DenyDecision{Reason: "no input received"}
	}
	response := strings.TrimSpace(strings.ToLower(scanner.Text()))

	switch {
	case response == "" || response == "y" || response == "yes":
		return AllowDecision{}
	case response == "a" || response == "always":
		p.alwaysAllow[toolName] = true
		return AllowDecision{}
	default:
		return DenyDecision{Reason: "user denied permission"}
	}
}
