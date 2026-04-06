package cli

import (
	"encoding/json"
	"testing"
)

func TestNdjsonSafeStringify(t *testing.T) {
	tests := []struct {
		name  string
		input any
		want  string
	}{
		{
			name:  "string with U+2028 is escaped",
			input: "hello\u2028world",
			want:  `"hello\u2028world"`,
		},
		{
			name:  "string with U+2029 is escaped",
			input: "hello\u2029world",
			want:  `"hello\u2029world"`,
		},
		{
			name:  "string with both U+2028 and U+2029 is escaped",
			input: "a\u2028b\u2029c",
			want:  `"a\u2028b\u2029c"`,
		},
		{
			name:  "normal string unchanged",
			input: "hello world",
			want:  `"hello world"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NdjsonSafeStringify(tt.input)
			if got != tt.want {
				t.Errorf("NdjsonSafeStringify(%q) = %q, want %q", tt.input, got, tt.want)
			}
			// Verify round-trip: the escaped output must parse back to the original.
			var decoded string
			if err := json.Unmarshal([]byte(got), &decoded); err != nil {
				t.Fatalf("output is not valid JSON: %v", err)
			}
			if decoded != tt.input.(string) {
				t.Errorf("round-trip mismatch: got %q, want %q", decoded, tt.input)
			}
		})
	}
}
