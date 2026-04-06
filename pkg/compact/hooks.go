package compact

// Source: services/compact/compact.ts:374-381

// MergeHookInstructions merges user-supplied custom instructions with
// hook-provided instructions. User instructions come first; hook instructions
// are appended. Empty strings normalize to empty.
// Source: compact.ts:374-381
func MergeHookInstructions(userInstructions, hookInstructions string) string {
	if hookInstructions == "" {
		return userInstructions
	}
	if userInstructions == "" {
		return hookInstructions
	}
	return userInstructions + "\n\n" + hookInstructions
}
