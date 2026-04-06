package errors

import "testing"

func TestErrorIDConstants(t *testing.T) {
	t.Run("E_TOOL_USE_SUMMARY_GENERATION_FAILED equals 344", func(t *testing.T) {
		if EToolUseSummaryGenerationFailed != 344 {
			t.Errorf("expected 344, got %d", EToolUseSummaryGenerationFailed)
		}
	})

	t.Run("error IDs are unique", func(t *testing.T) {
		ids := map[int]string{
			EToolUseSummaryGenerationFailed: "EToolUseSummaryGenerationFailed",
		}
		// Verify the map has the expected number of entries (catches duplicates).
		if len(ids) != 1 {
			t.Errorf("expected 1 unique error ID, got %d", len(ids))
		}
	})
}
