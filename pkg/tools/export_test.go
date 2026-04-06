package tools

// Test exports for grep.go internals.

// ApplyGrepHeadLimitForTest exposes applyGrepHeadLimit for testing.
func ApplyGrepHeadLimitForTest(items []string, limit *int, offset int) ([]string, string) {
	return applyGrepHeadLimit(items, limit, offset)
}

// SplitGlobPatternsForTest exposes splitGlobPatterns for testing.
func SplitGlobPatternsForTest(glob string) []string {
	return splitGlobPatterns(glob)
}

// PluralForTest exposes plural for testing.
func PluralForTest(n int, word string) string {
	return plural(n, word)
}
