package provider

// Anthropic API server-side limits.
// Source: src/constants/apiLimits.ts (last verified 2025-12-22)

const (
	// Image limits

	// APIImageMaxBase64Size is the max base64-encoded image size (API enforced, 5 MB).
	APIImageMaxBase64Size = 5 * 1024 * 1024

	// ImageTargetRawSize is the max raw image size to stay under base64 limit (3.75 MB).
	ImageTargetRawSize = (APIImageMaxBase64Size * 3) / 4

	// ImageMaxWidth is the client-side max image width for resizing.
	ImageMaxWidth = 2000

	// ImageMaxHeight is the client-side max image height for resizing.
	ImageMaxHeight = 2000

	// PDF limits

	// PDFTargetRawSize is the max raw PDF size fitting within API request limit (20 MB).
	PDFTargetRawSize = 20 * 1024 * 1024

	// APIPDFMaxPages is the max pages in a PDF accepted by the API.
	APIPDFMaxPages = 100

	// PDFExtractSizeThreshold is the size above which PDFs are extracted to page images (3 MB).
	PDFExtractSizeThreshold = 3 * 1024 * 1024

	// PDFMaxExtractSize is the max PDF size for the extraction path (100 MB).
	PDFMaxExtractSize = 100 * 1024 * 1024

	// PDFMaxPagesPerRead is the max pages the Read tool extracts per call.
	PDFMaxPagesPerRead = 20

	// PDFAtMentionInlineThreshold is the page count above which PDFs get reference treatment on @mention.
	PDFAtMentionInlineThreshold = 10

	// Media limits

	// APIMaxMediaPerRequest is the max media items (images + PDFs) per API request.
	APIMaxMediaPerRequest = 100
)
