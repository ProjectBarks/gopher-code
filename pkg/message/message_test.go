package message

import "testing"

func TestNoContentMessageConstant(t *testing.T) {
	if NoContentMessage != "(no content)" {
		t.Fatalf("expected %q, got %q", "(no content)", NoContentMessage)
	}
}
