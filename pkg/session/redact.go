package session

import (
	"regexp"
	"sync"
)

// Source: services/teamMemorySync/secretScanner.ts

// secretRule defines a secret detection pattern.
// Source: secretScanner.ts:23-30
type secretRule struct {
	id     string
	source string
	flags  string
}

// secretRules is the curated set of high-confidence secret detection patterns
// from gitleaks. Only rules with distinctive prefixes are included.
// Source: secretScanner.ts:48-224
var secretRules = []secretRule{
	// Cloud providers
	{id: "aws-access-token", source: `\b((?:A3T[A-Z0-9]|AKIA|ASIA|ABIA|ACCA)[A-Z2-7]{16})\b`},
	{id: "gcp-api-key", source: `\b(AIza[\w-]{35})(?:[\x60'";\s]|\\[nr]|$)`},

	// AI APIs
	{id: "anthropic-api-key", source: `\b(sk-ant-api03-[a-zA-Z0-9_\-]{93}AA)(?:[\x60'"\s;]|\\[nr]|$)`},
	{id: "openai-api-key", source: `\b(sk-(?:proj|svcacct|admin)-(?:[A-Za-z0-9_-]{74}|[A-Za-z0-9_-]{58})T3BlbkFJ(?:[A-Za-z0-9_-]{74}|[A-Za-z0-9_-]{58})\b|sk-[a-zA-Z0-9]{20}T3BlbkFJ[a-zA-Z0-9]{20})(?:[\x60'"\s;]|\\[nr]|$)`},
	{id: "huggingface-access-token", source: `\b(hf_[a-zA-Z]{34})(?:[\x60'"\s;]|\\[nr]|$)`},

	// Version control
	{id: "github-pat", source: `ghp_[0-9a-zA-Z]{36}`},
	{id: "github-fine-grained-pat", source: `github_pat_\w{82}`},
	{id: "github-app-token", source: `(?:ghu|ghs)_[0-9a-zA-Z]{36}`},
	{id: "github-oauth", source: `gho_[0-9a-zA-Z]{36}`},
	{id: "github-refresh-token", source: `ghr_[0-9a-zA-Z]{36}`},
	{id: "gitlab-pat", source: `glpat-[\w-]{20}`},

	// Communication
	{id: "slack-bot-token", source: `xoxb-[0-9]{10,13}-[0-9]{10,13}[a-zA-Z0-9-]*`},
	{id: "slack-user-token", source: `xox[pe](?:-[0-9]{10,13}){3}-[a-zA-Z0-9-]{28,34}`},

	// Dev tooling
	{id: "npm-access-token", source: `\b(npm_[a-zA-Z0-9]{36})(?:[\x60'"\s;]|\\[nr]|$)`},
	{id: "pypi-upload-token", source: `pypi-AgEIcHlwaS5vcmc[\w-]{50,1000}`},

	// Payment
	{id: "stripe-access-token", source: `\b((?:sk|rk)_(?:test|live|prod)_[a-zA-Z0-9]{10,99})(?:[\x60'"\s;]|\\[nr]|$)`},
	{id: "shopify-access-token", source: `shpat_[a-fA-F0-9]{32}`},

	// Crypto
	{id: "private-key", source: `-----BEGIN[ A-Z0-9_-]{0,100}PRIVATE KEY(?: BLOCK)?-----[\s\S-]{64,}?-----END[ A-Z0-9_-]{0,100}PRIVATE KEY(?: BLOCK)?-----`, flags: "i"},

	// Generic bearer/API key patterns (additional high-signal patterns)
	{id: "bearer-token", source: `\b[Bb]earer\s+[a-zA-Z0-9_\-.]{20,500}\b`},
}

var (
	compiledOnce    sync.Once
	compiledRegexps []*regexp.Regexp
)

func getCompiledRules() []*regexp.Regexp {
	compiledOnce.Do(func() {
		compiledRegexps = make([]*regexp.Regexp, 0, len(secretRules))
		for _, rule := range secretRules {
			flags := ""
			if rule.flags == "i" {
				flags = "(?i)"
			}
			re, err := regexp.Compile(flags + rule.source)
			if err != nil {
				continue // Skip rules that don't compile in Go regex
			}
			compiledRegexps = append(compiledRegexps, re)
		}
	})
	return compiledRegexps
}

// RedactSecrets replaces detected secrets with [REDACTED].
// Source: secretScanner.ts:312-324
func RedactSecrets(content string) string {
	for _, re := range getCompiledRules() {
		content = re.ReplaceAllStringFunc(content, func(match string) string {
			// Try to replace only captured group if present
			subs := re.FindStringSubmatch(match)
			if len(subs) > 1 && subs[1] != "" {
				return replaceFirst(match, subs[1], "[REDACTED]")
			}
			return "[REDACTED]"
		})
	}
	return content
}

// ContainsSecrets returns true if the content contains any detected secrets.
func ContainsSecrets(content string) bool {
	for _, re := range getCompiledRules() {
		if re.MatchString(content) {
			return true
		}
	}
	return false
}

// replaceFirst replaces the first occurrence of old in s with new.
func replaceFirst(s, old, new string) string {
	idx := len(s) // fallback: no match
	for i := 0; i <= len(s)-len(old); i++ {
		if s[i:i+len(old)] == old {
			idx = i
			break
		}
	}
	if idx >= len(s) {
		return new
	}
	return s[:idx] + new + s[idx+len(old):]
}
