package cli

import (
	"fmt"
	"strings"

	"charm.land/huh/v2"
)

// Source: components/PermissionDialog — Huh forms for permission prompts

// PermissionChoice is the user's response to a permission prompt.
type PermissionChoice string

const (
	PermissionAllow       PermissionChoice = "allow"
	PermissionDeny        PermissionChoice = "deny"
	PermissionAlwaysAllow PermissionChoice = "always"
)

// ShowPermissionDialog presents a Huh form for tool permission approval.
// Returns the user's choice: allow, deny, or always allow.
func ShowPermissionDialog(toolName, description string) (PermissionChoice, error) {
	var choice string

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title(fmt.Sprintf("Allow %s?", toolName)).
				Description(truncateForDialog(description, 200)).
				Options(
					huh.NewOption("Yes, allow this time", "allow"),
					huh.NewOption("No, deny", "deny"),
					huh.NewOption("Always allow this tool", "always"),
				).
				Value(&choice),
		),
	)

	err := form.Run()
	if err != nil {
		return PermissionDeny, err
	}

	return PermissionChoice(choice), nil
}

// ShowConfirmDialog presents a simple yes/no confirmation.
func ShowConfirmDialog(title, description string) (bool, error) {
	var confirmed bool

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title(title).
				Description(description).
				Value(&confirmed),
		),
	)

	err := form.Run()
	return confirmed, err
}

// ShowTextInputDialog presents a text input form.
func ShowTextInputDialog(title, placeholder string) (string, error) {
	var value string

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title(title).
				Placeholder(placeholder).
				Value(&value),
		),
	)

	err := form.Run()
	return value, err
}

func truncateForDialog(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	// Truncate at last space before maxLen
	idx := strings.LastIndex(s[:maxLen], " ")
	if idx > maxLen/2 {
		return s[:idx] + "..."
	}
	return s[:maxLen] + "..."
}
