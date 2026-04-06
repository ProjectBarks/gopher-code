package doctor

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/projectbarks/gopher-code/pkg/ui/theme"
)

// EnvBound describes the expected bounds for a numeric environment variable.
// Source: Doctor.tsx — validateBoundedIntEnvVar
type EnvBound struct {
	Name         string
	DefaultValue int
	UpperLimit   int
}

// EnvValidationResult holds a single env-var out-of-bounds finding.
type EnvValidationResult struct {
	Name    string
	Value   int
	Issue   string // e.g. "exceeds upper limit of 1000000"
}

// DefaultEnvBounds returns the standard env-var bounds checked by /doctor.
// Source: Doctor.tsx — BASH_MAX_OUTPUT_DEFAULT/UPPER_LIMIT, TASK_MAX_OUTPUT_DEFAULT/UPPER_LIMIT
func DefaultEnvBounds() []EnvBound {
	return []EnvBound{
		{Name: "BASH_MAX_OUTPUT", DefaultValue: 8000, UpperLimit: 1000000},
		{Name: "TASK_MAX_OUTPUT", DefaultValue: 8000, UpperLimit: 1000000},
	}
}

// ValidateEnvBounds checks environment variables against their expected bounds.
// Returns results only for vars that are set and out of bounds.
// Source: Doctor.tsx — validateBoundedIntEnvVar
func ValidateEnvBounds(bounds []EnvBound) []EnvValidationResult {
	var results []EnvValidationResult
	for _, b := range bounds {
		raw := os.Getenv(b.Name)
		if raw == "" {
			continue
		}
		val, err := strconv.Atoi(raw)
		if err != nil {
			results = append(results, EnvValidationResult{
				Name:  b.Name,
				Value: 0,
				Issue: fmt.Sprintf("not a valid integer: %q", raw),
			})
			continue
		}
		if val < 0 {
			results = append(results, EnvValidationResult{
				Name:  b.Name,
				Value: val,
				Issue: "must not be negative",
			})
		} else if val > b.UpperLimit {
			results = append(results, EnvValidationResult{
				Name:  b.Name,
				Value: val,
				Issue: fmt.Sprintf("exceeds upper limit of %d", b.UpperLimit),
			})
		}
	}
	return results
}

// RenderEnvValidation renders the env-var validation section.
// Source: Doctor.tsx — env-var bounds validation display
func RenderEnvValidation(results []EnvValidationResult) string {
	if len(results) == 0 {
		return ""
	}

	t := theme.Current()
	bold := t.TextPrimary().Bold(true)
	warn := t.TextWarning()
	dim := lipgloss.NewStyle().Faint(true)

	var lines []string
	lines = append(lines, bold.Render("Environment Variable Warnings"))
	for _, r := range results {
		lines = append(lines, fmt.Sprintf("└ %s",
			warn.Render(fmt.Sprintf("%s=%d: %s", r.Name, r.Value, r.Issue)),
		))
		lines = append(lines, dim.Render(fmt.Sprintf("  └ Current value: %d", r.Value)))
	}
	return strings.Join(lines, "\n")
}
