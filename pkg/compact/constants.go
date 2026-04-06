package compact

// Source: services/compact/compact.ts:122-130

// Post-compact restoration budget constants.
const (
	// PostCompactMaxFilesToRestore is the maximum number of recently-read files
	// re-injected after compaction so the model retains file context.
	// Source: compact.ts:122
	PostCompactMaxFilesToRestore = 5

	// PostCompactTokenBudget is the total token budget for all post-compact
	// restored content (files + skills + attachments).
	// Source: compact.ts:123
	PostCompactTokenBudget = 50_000

	// PostCompactMaxTokensPerFile caps the token cost of a single restored file.
	// Source: compact.ts:124
	PostCompactMaxTokensPerFile = 5_000

	// PostCompactMaxTokensPerSkill caps the token cost of a single restored skill.
	// Skills can be large (verify=18.7KB, claude-api=20.1KB). Per-skill truncation
	// beats dropping — instructions at the top of a skill file are the critical part.
	// Source: compact.ts:129
	PostCompactMaxTokensPerSkill = 5_000

	// PostCompactSkillsTokenBudget is the total budget for all restored skills.
	// Sized to hold ~5 skills at the per-skill cap.
	// Source: compact.ts:130
	PostCompactSkillsTokenBudget = 25_000
)
