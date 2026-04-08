package components

import (
	"fmt"
)

// Source: components/ApproveApiKey.tsx

// ApproveApiKeyMsg is sent when the user approves or rejects a custom API key.
type ApproveApiKeyMsg struct {
	Approved           bool
	ApiKeyTruncated    string
	RememberForSession bool
}

// ApproveApiKeyChoice represents the user's choice for API key approval.
type ApproveApiKeyChoice int

const (
	ApproveApiKeyYes ApproveApiKeyChoice = iota
	ApproveApiKeyNo
	ApproveApiKeyNoRemember
)

// ApproveApiKeyOptions returns the option labels shown in the dialog.
// Source: ApproveApiKey.tsx — Select options
func ApproveApiKeyOptions() []string {
	return []string{
		"Yes, approve this key",
		"No, reject this key",
		"No, and don't ask again this session",
	}
}

// FormatApproveApiKeyPrompt returns the dialog prompt text.
func FormatApproveApiKeyPrompt(apiKeyTruncated string) string {
	return fmt.Sprintf("A custom API key (%s) was provided.\nDo you want to use it?", apiKeyTruncated)
}
