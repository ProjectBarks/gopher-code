package auth

import (
	"os"
	"testing"
)

func TestAPIKeyWithSource_EnvVar(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "sk-test-key")
	key, source := APIKeyWithSource()
	if key != "sk-test-key" {
		t.Errorf("key = %q, want sk-test-key", key)
	}
	if source != SourceEnvVar {
		t.Errorf("source = %q, want env_var", source)
	}
}

func TestAPIKeyWithSource_AuthToken(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("ANTHROPIC_AUTH_TOKEN", "auth-tok-123")
	key, source := APIKeyWithSource()
	if key != "auth-tok-123" {
		t.Errorf("key = %q, want auth-tok-123", key)
	}
	if source != SourceEnvVar {
		t.Errorf("source = %q, want env_var", source)
	}
}

func TestGetSubscriptionType_MockEnv(t *testing.T) {
	t.Setenv("MOCK_SUBSCRIPTION_TYPE", "max_20")
	sub := GetSubscriptionType()
	if sub != SubMax20 {
		t.Errorf("sub = %q, want max_20", sub)
	}
}

func TestIsMaxSubscriber(t *testing.T) {
	t.Setenv("MOCK_SUBSCRIPTION_TYPE", "max_5")
	if !IsMaxSubscriber() {
		t.Error("max_5 should be a max subscriber")
	}

	t.Setenv("MOCK_SUBSCRIPTION_TYPE", "pro")
	if IsMaxSubscriber() {
		t.Error("pro should not be a max subscriber")
	}
}

func TestIsClaudeAISubscriber(t *testing.T) {
	t.Setenv("MOCK_SUBSCRIPTION_TYPE", "pro")
	if !IsClaudeAISubscriber() {
		t.Error("pro should be a claude AI subscriber")
	}

	t.Setenv("MOCK_SUBSCRIPTION_TYPE", "free")
	if IsClaudeAISubscriber() {
		t.Error("free should not be a claude AI subscriber")
	}

	t.Setenv("MOCK_SUBSCRIPTION_TYPE", "")
	if IsClaudeAISubscriber() {
		t.Error("empty should not be a claude AI subscriber")
	}
}

func TestGetSubscriptionName(t *testing.T) {
	tests := map[SubscriptionType]string{
		SubFree:       "Free",
		SubPro:        "Pro",
		SubMax5:       "Max (5x)",
		SubMax20:      "Max (20x)",
		SubTeam:       "Team",
		SubEnterprise: "Enterprise",
		"":            "Unknown",
	}
	for sub, want := range tests {
		os.Setenv("MOCK_SUBSCRIPTION_TYPE", string(sub))
		got := GetSubscriptionName()
		if got != want {
			t.Errorf("GetSubscriptionName(%q) = %q, want %q", sub, got, want)
		}
	}
	os.Unsetenv("MOCK_SUBSCRIPTION_TYPE")
}

func TestIsUsing3PServices(t *testing.T) {
	t.Setenv("ANTHROPIC_BASE_URL", "")
	t.Setenv("CLAUDE_CODE_USE_BEDROCK", "")
	t.Setenv("CLAUDE_CODE_USE_VERTEX", "")
	if IsUsing3PServices() {
		t.Error("should be false with no 3P env vars")
	}

	t.Setenv("CLAUDE_CODE_USE_BEDROCK", "1")
	if !IsUsing3PServices() {
		t.Error("should be true with Bedrock")
	}
}
