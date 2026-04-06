// Package analytics provides event telemetry for Claude Code.
//
// Events flow through a sink interface that routes to Datadog (general-access)
// and first-party (PII-privileged) backends. Events logged before the sink is
// attached are queued and drained on attach.
package analytics

import "os"

// IsAnalyticsDisabled returns true when analytics should be suppressed.
// Analytics is disabled in test environments, third-party cloud providers
// (Bedrock/Vertex/Foundry), and when telemetry is opted out.
func IsAnalyticsDisabled() bool {
	return isTestEnv() ||
		isEnvTruthy(os.Getenv("CLAUDE_CODE_USE_BEDROCK")) ||
		isEnvTruthy(os.Getenv("CLAUDE_CODE_USE_VERTEX")) ||
		isEnvTruthy(os.Getenv("CLAUDE_CODE_USE_FOUNDRY")) ||
		IsTelemetryDisabled()
}

// IsFeedbackSurveyDisabled returns true when the feedback survey prompt
// should be suppressed. Unlike IsAnalyticsDisabled, this does NOT block on
// third-party providers — the survey is a local UI prompt with no transcript
// data; enterprises capture responses via their own OTEL pipeline.
func IsFeedbackSurveyDisabled() bool {
	return isTestEnv() || IsTelemetryDisabled()
}

// IsTelemetryDisabled checks whether the user has opted out of telemetry
// via the CLAUDE_CODE_DISABLE_TELEMETRY environment variable.
func IsTelemetryDisabled() bool {
	return isEnvTruthy(os.Getenv("CLAUDE_CODE_DISABLE_TELEMETRY"))
}

// isTestEnv returns true if running in a test environment.
func isTestEnv() bool {
	// Go test sets -test.run; also honour the TS convention.
	if os.Getenv("GO_TEST") == "1" {
		return true
	}
	if os.Getenv("NODE_ENV") == "test" {
		return true
	}
	return false
}

// Note: isEnvTruthy is defined in growthbook.go (same package).
