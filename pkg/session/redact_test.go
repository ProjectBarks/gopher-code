package session

import "testing"

// Source: services/teamMemorySync/secretScanner.ts

func TestRedactSecrets(t *testing.T) {
	// Source: secretScanner.ts:312-324

	t.Run("aws_access_key", func(t *testing.T) {
		// Source: secretScanner.ts:52
		input := "export AWS_ACCESS_KEY_ID=AKIAIOSFODNN7EXAMPLE"
		result := RedactSecrets(input)
		if result == input {
			t.Error("should have redacted AWS access key")
		}
		if !contains(result, "[REDACTED]") {
			t.Errorf("expected [REDACTED], got %q", result)
		}
	})

	t.Run("github_pat", func(t *testing.T) {
		// Source: secretScanner.ts:96
		input := "token: ghp_ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefgh01"
		result := RedactSecrets(input)
		if !contains(result, "[REDACTED]") {
			t.Errorf("expected GitHub PAT to be redacted, got %q", result)
		}
	})

	t.Run("github_fine_grained_pat", func(t *testing.T) {
		// Source: secretScanner.ts:100
		pat := "github_pat_" + repeatChar('a', 82)
		input := "GITHUB_TOKEN=" + pat
		result := RedactSecrets(input)
		if !contains(result, "[REDACTED]") {
			t.Errorf("expected fine-grained PAT to be redacted, got %q", result)
		}
	})

	t.Run("slack_bot_token", func(t *testing.T) {
		// Source: secretScanner.ts:126
		input := "SLACK_TOKEN=xoxb-1234567890123-1234567890123-abcdef"
		result := RedactSecrets(input)
		if !contains(result, "[REDACTED]") {
			t.Errorf("expected Slack bot token to be redacted, got %q", result)
		}
	})

	t.Run("npm_access_token", func(t *testing.T) {
		// Source: secretScanner.ts:148
		token := "npm_" + repeatChar('a', 36)
		input := "NPM_TOKEN=" + token
		result := RedactSecrets(input)
		if !contains(result, "[REDACTED]") {
			t.Errorf("expected npm token to be redacted, got %q", result)
		}
	})

	t.Run("stripe_key", func(t *testing.T) {
		// Source: secretScanner.ts:204
		input := "STRIPE_KEY=sk_test_" + repeatChar('a', 20)
		result := RedactSecrets(input)
		if !contains(result, "[REDACTED]") {
			t.Errorf("expected Stripe key to be redacted, got %q", result)
		}
	})

	t.Run("private_key", func(t *testing.T) {
		// Source: secretScanner.ts:219-222
		input := "-----BEGIN RSA PRIVATE KEY-----\n" + repeatChar('A', 100) + "\n-----END RSA PRIVATE KEY-----"
		result := RedactSecrets(input)
		if !contains(result, "[REDACTED]") {
			t.Errorf("expected private key to be redacted, got %q", result)
		}
	})

	t.Run("no_secret_unchanged", func(t *testing.T) {
		input := "just a normal string with no secrets"
		result := RedactSecrets(input)
		if result != input {
			t.Errorf("expected unchanged output, got %q", result)
		}
	})

	t.Run("preserves_surrounding_text", func(t *testing.T) {
		input := "before ghp_ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefgh01 after"
		result := RedactSecrets(input)
		if !contains(result, "before") || !contains(result, "after") {
			t.Errorf("expected surrounding text preserved, got %q", result)
		}
	})

	t.Run("bearer_token", func(t *testing.T) {
		input := "Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.payload.signature"
		result := RedactSecrets(input)
		if !contains(result, "[REDACTED]") {
			t.Errorf("expected bearer token to be redacted, got %q", result)
		}
	})
}

func TestContainsSecrets(t *testing.T) {
	t.Run("detects_github_pat", func(t *testing.T) {
		if !ContainsSecrets("ghp_ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefgh01") {
			t.Error("should detect GitHub PAT")
		}
	})

	t.Run("no_false_positive", func(t *testing.T) {
		if ContainsSecrets("just a normal string") {
			t.Error("should not detect secrets in normal text")
		}
	})
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		findSubstring(s, substr) >= 0)
}

func findSubstring(s, sub string) int {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func repeatChar(ch byte, n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = ch
	}
	return string(b)
}
