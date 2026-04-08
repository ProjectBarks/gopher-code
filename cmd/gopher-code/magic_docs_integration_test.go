package main

import (
	"testing"

	"github.com/projectbarks/gopher-code/pkg/services"
	"github.com/projectbarks/gopher-code/pkg/session"
)

// T486: MagicDocs integration test
func TestMagicDocs_WiredIntoBinary(t *testing.T) {
	// Verify detection.
	header := services.DetectMagicDocHeader("# MAGIC DOC: Architecture Overview\n*Keep this doc updated with major design decisions*\n\nContent here...")
	if header == nil {
		t.Fatal("should detect magic doc header")
	}
	if header.Title != "Architecture Overview" {
		t.Errorf("Title = %q, want 'Architecture Overview'", header.Title)
	}
	if header.Instructions != "Keep this doc updated with major design decisions" {
		t.Errorf("Instructions = %q", header.Instructions)
	}

	// Non-magic doc.
	if services.DetectMagicDocHeader("# Regular README\nNot a magic doc") != nil {
		t.Error("should not detect non-magic doc")
	}

	// Verify tracker is wired into session.
	sess := session.New(session.DefaultConfig(), t.TempDir())
	if sess.MagicDocs == nil {
		t.Fatal("MagicDocs tracker should be initialized")
	}
	sess.MagicDocs.Track("/path/to/doc.md", "Test Doc")
	if sess.MagicDocs.Count() != 1 {
		t.Errorf("Count = %d, want 1", sess.MagicDocs.Count())
	}
	sess.MagicDocs.Clear()
	if sess.MagicDocs.Count() != 0 {
		t.Error("Count should be 0 after Clear")
	}
}
