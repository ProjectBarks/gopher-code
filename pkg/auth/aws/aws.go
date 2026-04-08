// Package aws provides AWS authentication utilities for Bedrock.
// Source: utils/aws.ts, utils/awsAuthStatusManager.ts
package aws

import (
	"encoding/json"
	"os"
	"os/exec"
	"strings"
	"sync"
)

// Credentials holds AWS short-term credentials.
// Source: utils/aws.ts — AwsCredentials
type Credentials struct {
	AccessKeyID     string `json:"AccessKeyId"`
	SecretAccessKey string `json:"SecretAccessKey"`
	SessionToken    string `json:"SessionToken"`
	Expiration      string `json:"Expiration,omitempty"`
}

// STSOutput is the output of `aws sts get-session-token` or `aws sts assume-role`.
type STSOutput struct {
	Credentials Credentials `json:"Credentials"`
}

// IsValidSTSOutput validates that the STS output has all required credential fields.
// Source: utils/aws.ts — isValidAwsStsOutput
func IsValidSTSOutput(data []byte) bool {
	var out STSOutput
	if err := json.Unmarshal(data, &out); err != nil {
		return false
	}
	return out.Credentials.AccessKeyID != "" &&
		out.Credentials.SecretAccessKey != "" &&
		out.Credentials.SessionToken != ""
}

// CheckCallerIdentity calls `aws sts get-caller-identity` to verify credentials.
// Returns an error if the identity check fails.
// Source: utils/aws.ts — checkStsCallerIdentity
func CheckCallerIdentity() error {
	cmd := exec.Command("aws", "sts", "get-caller-identity")
	return cmd.Run()
}

// AuthStatus tracks the state of an AWS auth refresh operation.
// Source: utils/awsAuthStatusManager.ts — AwsAuthStatus
type AuthStatus struct {
	IsAuthenticating bool
	Output           []string
	Error            string
}

// AuthStatusManager is a thread-safe singleton for cloud-provider auth status.
// Used by both AWS Bedrock and GCP Vertex auth flows.
// Source: utils/awsAuthStatusManager.ts — AwsAuthStatusManager
type AuthStatusManager struct {
	mu       sync.RWMutex
	status   AuthStatus
	onChange func(AuthStatus) // optional notification callback
}

var (
	globalManager     *AuthStatusManager
	globalManagerOnce sync.Once
)

// GetAuthStatusManager returns the singleton auth status manager.
func GetAuthStatusManager() *AuthStatusManager {
	globalManagerOnce.Do(func() {
		globalManager = &AuthStatusManager{}
	})
	return globalManager
}

// OnChange registers a callback for status changes.
func (m *AuthStatusManager) OnChange(fn func(AuthStatus)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onChange = fn
}

// GetStatus returns a snapshot of the current auth status.
func (m *AuthStatusManager) GetStatus() AuthStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]string, len(m.status.Output))
	copy(out, m.status.Output)
	return AuthStatus{
		IsAuthenticating: m.status.IsAuthenticating,
		Output:           out,
		Error:            m.status.Error,
	}
}

// StartAuthentication begins a new auth refresh cycle.
func (m *AuthStatusManager) StartAuthentication() {
	m.mu.Lock()
	m.status = AuthStatus{IsAuthenticating: true}
	cb := m.onChange
	s := m.GetStatusUnsafe()
	m.mu.Unlock()
	if cb != nil {
		cb(s)
	}
}

// AddOutput appends a line of output from the auth process.
func (m *AuthStatusManager) AddOutput(line string) {
	m.mu.Lock()
	m.status.Output = append(m.status.Output, line)
	cb := m.onChange
	s := m.GetStatusUnsafe()
	m.mu.Unlock()
	if cb != nil {
		cb(s)
	}
}

// SetError records an authentication error.
func (m *AuthStatusManager) SetError(err string) {
	m.mu.Lock()
	m.status.Error = err
	cb := m.onChange
	s := m.GetStatusUnsafe()
	m.mu.Unlock()
	if cb != nil {
		cb(s)
	}
}

// EndAuthentication completes the auth cycle. On success, clears all state.
func (m *AuthStatusManager) EndAuthentication(success bool) {
	m.mu.Lock()
	if success {
		m.status = AuthStatus{}
	} else {
		m.status.IsAuthenticating = false
	}
	cb := m.onChange
	s := m.GetStatusUnsafe()
	m.mu.Unlock()
	if cb != nil {
		cb(s)
	}
}

// GetStatusUnsafe returns status without locking (caller must hold lock).
func (m *AuthStatusManager) GetStatusUnsafe() AuthStatus {
	out := make([]string, len(m.status.Output))
	copy(out, m.status.Output)
	return AuthStatus{
		IsAuthenticating: m.status.IsAuthenticating,
		Output:           out,
		Error:            m.status.Error,
	}
}

// IsBedrockEnabled returns true if Bedrock mode is configured.
func IsBedrockEnabled() bool {
	v := strings.ToLower(os.Getenv("CLAUDE_CODE_USE_BEDROCK"))
	return v == "1" || v == "true"
}
