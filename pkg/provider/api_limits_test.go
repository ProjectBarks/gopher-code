package provider

import "testing"

func TestAPILimitsConstants(t *testing.T) {
	tests := []struct {
		name string
		got  int
		want int
	}{
		{"APIImageMaxBase64Size", APIImageMaxBase64Size, 5 * 1024 * 1024},
		{"ImageTargetRawSize", ImageTargetRawSize, (5 * 1024 * 1024 * 3) / 4},
		{"ImageMaxWidth", ImageMaxWidth, 2000},
		{"ImageMaxHeight", ImageMaxHeight, 2000},
		{"PDFTargetRawSize", PDFTargetRawSize, 20 * 1024 * 1024},
		{"APIPDFMaxPages", APIPDFMaxPages, 100},
		{"PDFExtractSizeThreshold", PDFExtractSizeThreshold, 3 * 1024 * 1024},
		{"PDFMaxExtractSize", PDFMaxExtractSize, 100 * 1024 * 1024},
		{"PDFMaxPagesPerRead", PDFMaxPagesPerRead, 20},
		{"PDFAtMentionInlineThreshold", PDFAtMentionInlineThreshold, 10},
		{"APIMaxMediaPerRequest", APIMaxMediaPerRequest, 100},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("%s = %d, want %d", tt.name, tt.got, tt.want)
			}
		})
	}
}
