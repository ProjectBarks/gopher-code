// Package bridge — inbound attachment resolution for bridge sessions.
// Source: src/bridge/inboundAttachments.ts
package bridge

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"
)

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

// DownloadTimeoutMS is the per-file download timeout (30 seconds).
const DownloadTimeoutMS = 30_000

// ---------------------------------------------------------------------------
// InboundAttachment — wire type for file_attachments entries
// ---------------------------------------------------------------------------

// InboundAttachment describes a single file attachment on an inbound bridge
// message. The fields come from the web composer's upload flow.
type InboundAttachment struct {
	FileUUID string `json:"file_uuid"`
	FileName string `json:"file_name"`
}

// ---------------------------------------------------------------------------
// Deps — injectable dependencies for resolve operations
// ---------------------------------------------------------------------------

// AttachmentDeps bundles external dependencies needed by the resolve pipeline.
// Tests supply stubs; production code wires real implementations.
type AttachmentDeps struct {
	// GetAccessToken returns the bridge OAuth token. ("", false) = not logged in.
	GetAccessToken func() (string, bool)
	// GetBaseURL returns the bridge API base URL.
	GetBaseURL func() string
	// GetConfigDir returns the Claude config home directory (~/.claude).
	GetConfigDir func() string
	// GetSessionID returns the current session ID.
	GetSessionID func() string
	// HTTPClient is the HTTP client used for downloads. Nil = http.DefaultClient.
	HTTPClient *http.Client
}

func (d *AttachmentDeps) httpClient() *http.Client {
	if d.HTTPClient != nil {
		return d.HTTPClient
	}
	return http.DefaultClient
}

// ---------------------------------------------------------------------------
// Debug logging
// ---------------------------------------------------------------------------

func debugAttach(msg string) {
	slog.Debug(fmt.Sprintf("[bridge:inbound-attach] %s", msg))
}

// ---------------------------------------------------------------------------
// extractInboundAttachments
// ---------------------------------------------------------------------------

// ExtractInboundAttachments pulls the file_attachments array off a loosely-typed
// inbound message (raw JSON map). Returns nil when no valid attachments exist.
func ExtractInboundAttachments(raw json.RawMessage) []InboundAttachment {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil
	}
	fa, ok := m["file_attachments"]
	if !ok {
		return nil
	}
	var atts []InboundAttachment
	if err := json.Unmarshal(fa, &atts); err != nil {
		return nil
	}
	// Validate: each entry must have non-empty file_uuid and file_name.
	valid := atts[:0]
	for _, a := range atts {
		if a.FileUUID != "" && a.FileName != "" {
			valid = append(valid, a)
		}
	}
	if len(valid) == 0 {
		return nil
	}
	return valid
}

// ---------------------------------------------------------------------------
// sanitizeFileName
// ---------------------------------------------------------------------------

var fileNameUnsafe = regexp.MustCompile(`[^a-zA-Z0-9._-]`)

// SanitizeFileName strips path components and replaces non-safe characters
// with underscores. Falls back to "attachment" when the result is empty.
func SanitizeFileName(name string) string {
	base := filepath.Base(name)
	safe := fileNameUnsafe.ReplaceAllString(base, "_")
	if safe == "" {
		return "attachment"
	}
	return safe
}

// ---------------------------------------------------------------------------
// uploadsDir
// ---------------------------------------------------------------------------

// UploadsDir returns the directory for downloaded attachments:
// <configDir>/uploads/<sessionID>
func UploadsDir(configDir, sessionID string) string {
	return filepath.Join(configDir, "uploads", sessionID)
}

// ---------------------------------------------------------------------------
// resolveOne
// ---------------------------------------------------------------------------

// ResolveOne fetches and writes a single attachment to disk.
// Returns the absolute path on success, or ("", nil) on any retriable/skip error.
// Only returns a non-nil error for truly unexpected conditions.
func ResolveOne(ctx context.Context, att InboundAttachment, deps *AttachmentDeps) (string, error) {
	token, ok := deps.GetAccessToken()
	if !ok || token == "" {
		debugAttach("skip: no oauth token")
		return "", nil
	}

	// Build fetch URL.
	baseURL := deps.GetBaseURL()
	fetchURL := fmt.Sprintf("%s/api/oauth/files/%s/content", baseURL, url.PathEscape(att.FileUUID))

	// Create request with timeout.
	dlCtx, cancel := context.WithTimeout(ctx, time.Duration(DownloadTimeoutMS)*time.Millisecond)
	defer cancel()

	req, err := http.NewRequestWithContext(dlCtx, http.MethodGet, fetchURL, nil)
	if err != nil {
		debugAttach(fmt.Sprintf("fetch %s threw: %v", att.FileUUID, err))
		return "", nil
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := deps.httpClient().Do(req)
	if err != nil {
		debugAttach(fmt.Sprintf("fetch %s threw: %v", att.FileUUID, err))
		return "", nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		debugAttach(fmt.Sprintf("fetch %s failed: status=%d", att.FileUUID, resp.StatusCode))
		return "", nil
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		debugAttach(fmt.Sprintf("fetch %s threw: %v", att.FileUUID, err))
		return "", nil
	}

	// Build output path with uuid-prefix collision avoidance.
	safeName := SanitizeFileName(att.FileName)
	prefix := att.FileUUID
	if len(prefix) > 8 {
		prefix = prefix[:8]
	}
	if prefix == "" {
		b := make([]byte, 4)
		_, _ = rand.Read(b)
		prefix = hex.EncodeToString(b)
	}
	// Sanitize prefix too.
	prefix = fileNameUnsafe.ReplaceAllString(prefix, "_")

	dir := UploadsDir(deps.GetConfigDir(), deps.GetSessionID())
	outPath := filepath.Join(dir, prefix+"-"+safeName)

	if err := os.MkdirAll(dir, 0o755); err != nil {
		debugAttach(fmt.Sprintf("write %s failed: %v", outPath, err))
		return "", nil
	}
	if err := os.WriteFile(outPath, data, 0o644); err != nil {
		debugAttach(fmt.Sprintf("write %s failed: %v", outPath, err))
		return "", nil
	}

	debugAttach(fmt.Sprintf("resolved %s → %s (%d bytes)", att.FileUUID, outPath, len(data)))
	return outPath, nil
}

// ---------------------------------------------------------------------------
// ResolveInboundAttachments
// ---------------------------------------------------------------------------

// ResolveInboundAttachments resolves all attachments in parallel and returns
// a prefix string of @"path" refs. Returns "" when none resolved.
func ResolveInboundAttachments(ctx context.Context, attachments []InboundAttachment, deps *AttachmentDeps) string {
	if len(attachments) == 0 {
		return ""
	}
	debugAttach(fmt.Sprintf("resolving %d attachment(s)", len(attachments)))

	paths := make([]string, len(attachments))
	g, gctx := errgroup.WithContext(ctx)
	for i, att := range attachments {
		i, att := i, att
		g.Go(func() error {
			p, _ := ResolveOne(gctx, att, deps)
			paths[i] = p
			return nil // never fail the group
		})
	}
	_ = g.Wait()

	var ok []string
	for _, p := range paths {
		if p != "" {
			ok = append(ok, p)
		}
	}
	if len(ok) == 0 {
		return ""
	}
	// Quoted @-ref format: spaces-safe for home dirs with spaces.
	var b strings.Builder
	for _, p := range ok {
		b.WriteString(`@"`)
		b.WriteString(p)
		b.WriteString(`" `)
	}
	return b.String()
}

// ---------------------------------------------------------------------------
// PrependPathRefs
// ---------------------------------------------------------------------------

// PrependPathRefs prepends @path refs to content. Content may be a plain
// string or a slice of ContentBlock. Targets the LAST text block in a slice,
// since processUserInputBase reads from the last block.
//
// When prefix is empty, returns content unchanged.
func PrependPathRefs(content any, prefix string) any {
	if prefix == "" {
		return content
	}
	switch c := content.(type) {
	case string:
		return prefix + c
	case []ContentBlock:
		return prependToBlocks(c, prefix)
	default:
		return content
	}
}

func prependToBlocks(blocks []ContentBlock, prefix string) []ContentBlock {
	// Find last text block.
	lastIdx := -1
	for i := len(blocks) - 1; i >= 0; i-- {
		if blocks[i].Type == "text" {
			lastIdx = i
			break
		}
	}

	out := make([]ContentBlock, len(blocks))
	copy(out, blocks)

	if lastIdx != -1 {
		out[lastIdx] = ContentBlock{
			Type: "text",
			Text: prefix + out[lastIdx].Text,
		}
		return out
	}

	// No text block — append one at the end.
	return append(out, ContentBlock{
		Type: "text",
		Text: strings.TrimRight(prefix, " "),
	})
}

// ---------------------------------------------------------------------------
// ResolveAndPrepend
// ---------------------------------------------------------------------------

// ResolveAndPrepend is the convenience pipeline: extract + resolve + prepend.
// No-op (fast path) when the message has no file_attachments.
func ResolveAndPrepend(ctx context.Context, rawMsg json.RawMessage, content any, deps *AttachmentDeps) any {
	attachments := ExtractInboundAttachments(rawMsg)
	if len(attachments) == 0 {
		return content
	}
	prefix := ResolveInboundAttachments(ctx, attachments, deps)
	return PrependPathRefs(content, prefix)
}
