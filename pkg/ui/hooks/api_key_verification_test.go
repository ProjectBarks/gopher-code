package hooks

import (
	"errors"
	"testing"
)

func newTestVerification(opts ...func(*ApiKeyVerificationConfig)) *ApiKeyVerification {
	cfg := ApiKeyVerificationConfig{
		AuthEnabled:    func() bool { return true },
		IsSubscriber:   func() bool { return false },
		KeyProvider:    func(skipHelper bool) KeyResult { return KeyResult{} },
		HelperWarmer:   func(bool) error { return nil },
		Verifier:       func(string, bool) (bool, error) { return true, nil },
		NonInteractive: func() bool { return false },
	}
	for _, o := range opts {
		o(&cfg)
	}
	return NewApiKeyVerification(cfg)
}

func TestInitialStatus_AuthDisabled_Valid(t *testing.T) {
	v := newTestVerification(func(c *ApiKeyVerificationConfig) {
		c.AuthEnabled = func() bool { return false }
	})
	if v.Status() != VerificationValid {
		t.Fatalf("status = %q, want valid", v.Status())
	}
}

func TestInitialStatus_Subscriber_Valid(t *testing.T) {
	v := newTestVerification(func(c *ApiKeyVerificationConfig) {
		c.IsSubscriber = func() bool { return true }
	})
	if v.Status() != VerificationValid {
		t.Fatalf("status = %q, want valid", v.Status())
	}
}

func TestInitialStatus_NoKey_NoHelper_Missing(t *testing.T) {
	v := newTestVerification(func(c *ApiKeyVerificationConfig) {
		c.KeyProvider = func(skipHelper bool) KeyResult {
			return KeyResult{Key: "", Source: KeySourceNone}
		}
	})
	if v.Status() != VerificationMissing {
		t.Fatalf("status = %q, want missing", v.Status())
	}
}

func TestInitialStatus_HelperConfigured_Loading(t *testing.T) {
	v := newTestVerification(func(c *ApiKeyVerificationConfig) {
		c.KeyProvider = func(skipHelper bool) KeyResult {
			if skipHelper {
				return KeyResult{Key: "", Source: KeySourceHelper}
			}
			return KeyResult{Key: "sk-test", Source: KeySourceHelper}
		}
	})
	if v.Status() != VerificationLoading {
		t.Fatalf("status = %q, want loading", v.Status())
	}
}

func TestInitialStatus_HasKey_Loading(t *testing.T) {
	v := newTestVerification(func(c *ApiKeyVerificationConfig) {
		c.KeyProvider = func(bool) KeyResult {
			return KeyResult{Key: "sk-test", Source: KeySourceEnv}
		}
	})
	if v.Status() != VerificationLoading {
		t.Fatalf("status = %q, want loading", v.Status())
	}
}

func TestReverify_ValidKey(t *testing.T) {
	v := newTestVerification(func(c *ApiKeyVerificationConfig) {
		c.KeyProvider = func(skipHelper bool) KeyResult {
			if skipHelper {
				return KeyResult{Key: "", Source: KeySourceHelper}
			}
			return KeyResult{Key: "sk-valid", Source: KeySourceHelper}
		}
		c.Verifier = func(key string, _ bool) (bool, error) {
			return key == "sk-valid", nil
		}
	})

	if err := v.Reverify(); err != nil {
		t.Fatalf("Reverify() error: %v", err)
	}
	if v.Status() != VerificationValid {
		t.Fatalf("status = %q, want valid", v.Status())
	}
}

func TestReverify_InvalidKey(t *testing.T) {
	v := newTestVerification(func(c *ApiKeyVerificationConfig) {
		c.KeyProvider = func(bool) KeyResult {
			return KeyResult{Key: "sk-bad", Source: KeySourceEnv}
		}
		c.Verifier = func(string, bool) (bool, error) { return false, nil }
	})

	v.Reverify()
	if v.Status() != VerificationInvalid {
		t.Fatalf("status = %q, want invalid", v.Status())
	}
}

func TestReverify_HelperReturnsEmpty_Error(t *testing.T) {
	v := newTestVerification(func(c *ApiKeyVerificationConfig) {
		c.KeyProvider = func(skipHelper bool) KeyResult {
			return KeyResult{Key: "", Source: KeySourceHelper}
		}
	})

	v.Reverify()
	if v.Status() != VerificationError {
		t.Fatalf("status = %q, want error", v.Status())
	}
	if !errors.Is(v.Err(), ErrAPIKeyHelperInvalid) {
		t.Fatalf("err = %v, want ErrAPIKeyHelperInvalid", v.Err())
	}
}

func TestReverify_VerifierError(t *testing.T) {
	v := newTestVerification(func(c *ApiKeyVerificationConfig) {
		c.KeyProvider = func(bool) KeyResult {
			return KeyResult{Key: "sk-test", Source: KeySourceEnv}
		}
		c.Verifier = func(string, bool) (bool, error) {
			return false, errors.New("network error")
		}
	})

	v.Reverify()
	if v.Status() != VerificationError {
		t.Fatalf("status = %q, want error", v.Status())
	}
	if v.Err() == nil || v.Err().Error() != "network error" {
		t.Fatalf("err = %v, want 'network error'", v.Err())
	}
}

func TestReverify_NoKey_NoHelper_Missing(t *testing.T) {
	v := newTestVerification(func(c *ApiKeyVerificationConfig) {
		c.KeyProvider = func(bool) KeyResult {
			return KeyResult{Key: "", Source: KeySourceNone}
		}
	})

	v.Reverify()
	if v.Status() != VerificationMissing {
		t.Fatalf("status = %q, want missing", v.Status())
	}
}

func TestReverify_AuthDisabled_AlwaysValid(t *testing.T) {
	v := newTestVerification(func(c *ApiKeyVerificationConfig) {
		c.AuthEnabled = func() bool { return false }
	})

	v.Reverify()
	if v.Status() != VerificationValid {
		t.Fatalf("status = %q, want valid", v.Status())
	}
}

func TestReverify_HelperWarmerError(t *testing.T) {
	v := newTestVerification(func(c *ApiKeyVerificationConfig) {
		c.HelperWarmer = func(bool) error { return errors.New("helper failed") }
	})

	err := v.Reverify()
	if err == nil {
		t.Fatal("expected error from Reverify")
	}
	if v.Status() != VerificationError {
		t.Fatalf("status = %q, want error", v.Status())
	}
}

func TestSkipHelper_SecurityGate(t *testing.T) {
	var skipHelperValues []bool
	newTestVerification(func(c *ApiKeyVerificationConfig) {
		c.KeyProvider = func(skipHelper bool) KeyResult {
			skipHelperValues = append(skipHelperValues, skipHelper)
			return KeyResult{Key: "", Source: KeySourceNone}
		}
	})

	// Constructor should call with skipHelper=true.
	if len(skipHelperValues) != 1 || !skipHelperValues[0] {
		t.Fatalf("initial call skipHelper values = %v, want [true]", skipHelperValues)
	}
}
