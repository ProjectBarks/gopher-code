package tools

import "testing"

// Source: utils/pdfUtils.ts

func TestParsePDFPageRange(t *testing.T) {
	// Source: utils/pdfUtils.ts:16-50

	t.Run("single_page", func(t *testing.T) {
		// Source: pdfUtils.ts:34-40
		r := ParsePDFPageRange("5")
		if r == nil || r.FirstPage != 5 || r.LastPage != 5 {
			t.Errorf("expected {5,5}, got %v", r)
		}
	})

	t.Run("range", func(t *testing.T) {
		// Source: pdfUtils.ts:43-49
		r := ParsePDFPageRange("1-10")
		if r == nil || r.FirstPage != 1 || r.LastPage != 10 {
			t.Errorf("expected {1,10}, got %v", r)
		}
	})

	t.Run("open_ended", func(t *testing.T) {
		// Source: pdfUtils.ts:25-31
		r := ParsePDFPageRange("3-")
		if r == nil || r.FirstPage != 3 || r.LastPage != -1 {
			t.Errorf("expected {3,-1}, got %v", r)
		}
	})

	t.Run("empty_string", func(t *testing.T) {
		if ParsePDFPageRange("") != nil {
			t.Error("empty should return nil")
		}
	})

	t.Run("zero_page", func(t *testing.T) {
		if ParsePDFPageRange("0") != nil {
			t.Error("page 0 should return nil (1-indexed)")
		}
	})

	t.Run("inverted_range", func(t *testing.T) {
		if ParsePDFPageRange("10-5") != nil {
			t.Error("inverted range should return nil")
		}
	})

	t.Run("non_numeric", func(t *testing.T) {
		if ParsePDFPageRange("abc") != nil {
			t.Error("non-numeric should return nil")
		}
	})

	t.Run("whitespace_trimmed", func(t *testing.T) {
		r := ParsePDFPageRange("  3  ")
		if r == nil || r.FirstPage != 3 {
			t.Errorf("should trim whitespace, got %v", r)
		}
	})
}

func TestPDFMaxPagesPerRead(t *testing.T) {
	// Source: constants/apiLimits.ts:77
	if PDFMaxPagesPerRead != 20 {
		t.Errorf("expected 20, got %d", PDFMaxPagesPerRead)
	}
}
