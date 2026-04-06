package hooks

import "testing"

func TestGetSemverPart(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"1.2.3", "1.2.3"},
		{"v1.2.3", "1.2.3"},
		{"1.2.3-beta.1", "1.2.3"},
		{"1.2.3+build123", "1.2.3"},
		{"1.2.3-rc.1+meta", "1.2.3"},
		{"1.0", "1.0.0"},   // loose: missing patch
		{"2", "2.0.0"},     // loose: missing minor+patch
		{"", ""},           // empty
		{"abc", ""},        // non-numeric
	}
	for _, tt := range tests {
		got := getSemverPart(tt.input)
		if got != tt.want {
			t.Errorf("getSemverPart(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestUpdateNotification_DifferentVersion(t *testing.T) {
	u := NewUpdateNotification("1.0.0")

	// Same version: no notification.
	if got := u.Check("1.0.0"); got != "" {
		t.Fatalf("same version returned %q, want empty", got)
	}

	// New version: notification.
	if got := u.Check("1.1.0"); got != "1.1.0" {
		t.Fatalf("new version returned %q, want 1.1.0", got)
	}

	// Same new version again: deduped.
	if got := u.Check("1.1.0"); got != "" {
		t.Fatalf("deduped returned %q, want empty", got)
	}
}

func TestUpdateNotification_EmptyUpdatedVersion(t *testing.T) {
	u := NewUpdateNotification("1.0.0")
	if got := u.Check(""); got != "" {
		t.Fatalf("empty input returned %q, want empty", got)
	}
}

func TestUpdateNotification_PreReleaseIgnored(t *testing.T) {
	u := NewUpdateNotification("1.0.0")
	// 1.0.0-beta normalizes to 1.0.0, same as initial: no notification.
	if got := u.Check("1.0.0-beta"); got != "" {
		t.Fatalf("pre-release same base returned %q, want empty", got)
	}
}

func TestUpdateNotification_SuccessiveUpdates(t *testing.T) {
	u := NewUpdateNotification("1.0.0")

	if got := u.Check("2.0.0"); got != "2.0.0" {
		t.Fatalf("got %q, want 2.0.0", got)
	}
	if got := u.Check("3.0.0"); got != "3.0.0" {
		t.Fatalf("got %q, want 3.0.0", got)
	}
}
