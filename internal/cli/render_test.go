package cli

import (
	"testing"

	pkgcli "github.com/projectbarks/gopher-code/pkg/cli"
)

func TestNdjsonSafeStringify(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "no special chars",
			input: "hello world",
			want:  `"hello world"`,
		},
		{
			name:  "U+2028 line separator",
			input: "hello\u2028world",
			want:  `"hello\u2028world"`,
		},
		{
			name:  "U+2029 paragraph separator",
			input: "hello\u2029world",
			want:  `"hello\u2029world"`,
		},
		{
			name:  "both separators",
			input: "a\u2028b\u2029c",
			want:  `"a\u2028b\u2029c"`,
		},
		{
			name:  "empty string",
			input: "",
			want:  `""`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := pkgcli.NdjsonSafeStringify(tt.input)
			if got != tt.want {
				t.Errorf("NdjsonSafeStringify(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
