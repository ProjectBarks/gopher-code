package hooks

import (
	"errors"
	"sync"
)

// VerificationStatus represents the API key verification state.
// Source: hooks/useApiKeyVerification.ts
type VerificationStatus string

const (
	VerificationLoading VerificationStatus = "loading"
	VerificationValid   VerificationStatus = "valid"
	VerificationInvalid VerificationStatus = "invalid"
	VerificationMissing VerificationStatus = "missing"
	VerificationError   VerificationStatus = "error"
)

// ErrAPIKeyHelperInvalid is returned when the API key helper runs but
// does not produce a valid key.
var ErrAPIKeyHelperInvalid = errors.New("API key helper did not return a valid key")

// KeySource identifies where an API key came from.
type KeySource string

const (
	KeySourceEnv       KeySource = "env"
	KeySourceConfig    KeySource = "config"
	KeySourceHelper    KeySource = "apiKeyHelper"
	KeySourceNone      KeySource = ""
)

// KeyResult is returned by the key provider function.
type KeyResult struct {
	Key    string
	Source KeySource
}

// APIKeyVerifier verifies an API key by calling the provider.
// Returns true if the key is valid.
type APIKeyVerifier func(apiKey string, silent bool) (bool, error)

// KeyProvider retrieves the API key and its source.
// When skipHelper is true, the apiKeyHelper executable must NOT be invoked
// (security: prevents RCE via settings.json before trust dialog).
type KeyProvider func(skipHelper bool) KeyResult

// HelperWarmer warms the apiKeyHelper cache. Called once after trust dialog.
type HelperWarmer func(nonInteractive bool) error

// ApiKeyVerification implements the 5-state API key verification state machine.
// Source: hooks/useApiKeyVerification.ts
type ApiKeyVerification struct {
	mu     sync.Mutex
	status VerificationStatus
	err    error

	// Dependencies (injected).
	authEnabled    func() bool
	isSubscriber   func() bool
	keyProvider    KeyProvider
	helperWarmer   HelperWarmer
	verifier       APIKeyVerifier
	nonInteractive func() bool
}

// ApiKeyVerificationConfig holds the injectable dependencies for ApiKeyVerification.
type ApiKeyVerificationConfig struct {
	AuthEnabled    func() bool
	IsSubscriber   func() bool
	KeyProvider    KeyProvider
	HelperWarmer   HelperWarmer
	Verifier       APIKeyVerifier
	NonInteractive func() bool
}

// NewApiKeyVerification creates and initializes the verification state machine.
// The initial status is determined synchronously without invoking the apiKeyHelper
// (security: skipHelper=true to prevent RCE before trust dialog).
func NewApiKeyVerification(cfg ApiKeyVerificationConfig) *ApiKeyVerification {
	a := &ApiKeyVerification{
		authEnabled:    cfg.AuthEnabled,
		isSubscriber:   cfg.IsSubscriber,
		keyProvider:    cfg.KeyProvider,
		helperWarmer:   cfg.HelperWarmer,
		verifier:       cfg.Verifier,
		nonInteractive: cfg.NonInteractive,
	}
	a.status = a.initialStatus()
	return a
}

// Status returns the current verification status.
func (a *ApiKeyVerification) Status() VerificationStatus {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.status
}

// Err returns the last error, if any.
func (a *ApiKeyVerification) Err() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.err
}

// Reverify performs a full verification cycle, including warming the
// apiKeyHelper cache. This should be called after the trust dialog.
func (a *ApiKeyVerification) Reverify() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !a.authEnabled() || a.isSubscriber() {
		a.status = VerificationValid
		return nil
	}

	// Warm the helper cache (no-op if no helper configured).
	if a.helperWarmer != nil {
		if err := a.helperWarmer(a.nonInteractive()); err != nil {
			a.status = VerificationError
			a.err = err
			return err
		}
	}

	result := a.keyProvider(false) // skipHelper=false: helper is now safe
	if result.Key == "" {
		if result.Source == KeySourceHelper {
			a.status = VerificationError
			a.err = ErrAPIKeyHelperInvalid
			return ErrAPIKeyHelperInvalid
		}
		a.status = VerificationMissing
		return nil
	}

	valid, err := a.verifier(result.Key, false)
	if err != nil {
		a.status = VerificationError
		a.err = err
		return err
	}
	if valid {
		a.status = VerificationValid
	} else {
		a.status = VerificationInvalid
	}
	return nil
}

// initialStatus computes the starting status without running the helper.
func (a *ApiKeyVerification) initialStatus() VerificationStatus {
	if !a.authEnabled() || a.isSubscriber() {
		return VerificationValid
	}
	result := a.keyProvider(true) // skipHelper=true (security)
	if result.Key != "" || result.Source == KeySourceHelper {
		return VerificationLoading
	}
	return VerificationMissing
}
