// Package pdf provides best-effort PDF text extraction for the SoulGate tool
// system. It handles both local file paths and remote URLs, enforcing size
// limits and SSRF protections consistent with the rest of the broker layer.
package pdf

import (
	"bytes"
	"compress/flate"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	maxFileSize     = 10 * 1024 * 1024 // 10 MB
	maxContentSize  = 100 * 1000       // 100 KB of extracted text
	defaultMaxPage  = 20
	downloadTimeout = 30 * time.Second
)

// PDFOptions controls how a PDF is read and processed.
type PDFOptions struct {
	// Path is a local file path or an http/https URL.
	Path string `json:"path"`
	// Pages is an optional page filter: "1-5", "1,3,7-9", etc.
	// Page numbers are 1-based.
	Pages string `json:"pages,omitempty"`
	// MaxPages limits how many pages are processed. Defaults to 20.
	MaxPages int `json:"max_pages,omitempty"`
}

// PDFResult contains the outcome of a PDF analysis operation.
type PDFResult struct {
	Path      string `json:"path"`
	PageCount int    `json:"page_count"`
	Title     string `json:"title,omitempty"`
	Content   string `json:"content"`
	Truncated bool   `json:"truncated"`
	Error     string `json:"error,omitempty"`
}

// Analyze reads a PDF from a local path or URL and extracts its text content.
// Text extraction is best-effort: it decodes FlateDecode content streams and
// parses PDF text-showing operators (Tj, TJ, ', ").
func Analyze(ctx context.Context, opts PDFOptions) (*PDFResult, error) {
	if opts.Path == "" {
		return nil, fmt.Errorf("pdf: path is required")
	}
	if opts.MaxPages <= 0 {
		opts.MaxPages = defaultMaxPage
	}

	var (
		data []byte
		err  error
	)

	if strings.HasPrefix(opts.Path, "http://") || strings.HasPrefix(opts.Path, "https://") {
		data, err = downloadPDF(ctx, opts.Path)
	} else {
		data, err = readLocalPDF(opts.Path)
	}
	if err != nil {
		return nil, err
	}

	content, pageCount, extractErr := extractText(data, opts.Pages, opts.MaxPages)
	if extractErr != nil {
		return &PDFResult{
			Path:  opts.Path,
			Error: extractErr.Error(),
		}, nil
	}

	truncated := false
	if len(content) > maxContentSize {
		content = content[:maxContentSize]
		truncated = true
	}

	title := extractTitle(data)

	return &PDFResult{
		Path:      opts.Path,
		PageCount: pageCount,
		Title:     title,
		Content:   content,
		Truncated: truncated,
	}, nil
}

// ---------------------------------------------------------------------------
// Retrieval helpers
// ---------------------------------------------------------------------------

// pdfClient is shared across calls. Its timeout enforces a hard 30-second cap
// on PDF downloads so a slow remote server cannot stall the agentic loop.
var pdfClient = &http.Client{
	Timeout: downloadTimeout,
}

// downloadPDF fetches a PDF from a URL after verifying the host is not a
// private or loopback address (SSRF protection).
func downloadPDF(ctx context.Context, rawURL string) ([]byte, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("pdf: invalid URL %q: %w", rawURL, err)
	}

	// Resolve the hostname to catch SSRF via DNS rebinding or bare IPs.
	host := u.Hostname()
	if err := assertPublicHost(host); err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("pdf: failed to build request: %w", err)
	}

	resp, err := pdfClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("pdf: download failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("pdf: unexpected HTTP status %d for %q", resp.StatusCode, rawURL)
	}

	limited := io.LimitReader(resp.Body, int64(maxFileSize)+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, fmt.Errorf("pdf: error reading response body: %w", err)
	}
	if len(data) > maxFileSize {
		return nil, fmt.Errorf("pdf: remote file exceeds maximum size of %d bytes", maxFileSize)
	}

	return data, nil
}

// assertPublicHost returns an error if host resolves to a loopback, private,
// link-local, or unspecified address. Bare IP literals are checked directly.
func assertPublicHost(host string) error {
	// Parse as an IP literal first (no DNS lookup needed).
	if ip := net.ParseIP(host); ip != nil {
		return assertPublicIP(ip)
	}

	// Resolve the hostname.
	addrs, err := net.LookupHost(host)
	if err != nil {
		return fmt.Errorf("pdf: SSRF check: cannot resolve host %q: %w", host, err)
	}
	for _, addr := range addrs {
		ip := net.ParseIP(addr)
		if ip == nil {
			continue
		}
		if err := assertPublicIP(ip); err != nil {
			return err
		}
	}
	return nil
}

// assertPublicIP rejects private, loopback, link-local, and unspecified IPs.
func assertPublicIP(ip net.IP) error {
	if ip.IsLoopback() {
		return fmt.Errorf("pdf: SSRF protection: loopback address %s is not allowed", ip)
	}
	if ip.IsPrivate() {
		return fmt.Errorf("pdf: SSRF protection: private address %s is not allowed", ip)
	}
	if ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return fmt.Errorf("pdf: SSRF protection: link-local address %s is not allowed", ip)
	}
	if ip.IsUnspecified() {
		return fmt.Errorf("pdf: SSRF protection: unspecified address %s is not allowed", ip)
	}
	return nil
}

// readLocalPDF reads a PDF from a local filesystem path. It rejects any path
// that contains ".." components to prevent directory traversal.
func readLocalPDF(path string) ([]byte, error) {
	// Guard against traversal before resolving anything.
	// filepath.Clean alone is insufficient because the caller controls the raw
	// string; we want to reject the intent, not just the result.
	if strings.Contains(path, "..") {
		return nil, fmt.Errorf("pdf: path traversal detected in %q", path)
	}

	cleaned := filepath.Clean(path)

	f, err := os.Open(cleaned)
	if err != nil {
		return nil, fmt.Errorf("pdf: cannot open %q: %w", cleaned, err)
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("pdf: cannot stat %q: %w", cleaned, err)
	}
	if info.Size() > int64(maxFileSize) {
		return nil, fmt.Errorf("pdf: file %q exceeds maximum size of %d bytes", cleaned, maxFileSize)
	}

	data, err := io.ReadAll(f)
	if err != nil {
		return nil, fmt.Errorf("pdf: error reading %q: %w", cleaned, err)
	}
	return data, nil
}

// ---------------------------------------------------------------------------
// PDF text extraction
// ---------------------------------------------------------------------------

// extractText performs a best-effort extraction of plain text from raw PDF
// bytes. It locates content streams (compressed or raw), decompresses
// FlateDecode streams using compress/flate, then scans for PDF text operators.
//
// pageRange controls which pages are included (empty = all). maxPages is a
// hard cap applied after filtering. The returned pageCount reflects the total
// number of pages found in the document, independent of filtering.
func extractText(data []byte, pageRange string, maxPages int) (string, int, error) {
	if !bytes.HasPrefix(data, []byte("%PDF")) {
		return "", 0, fmt.Errorf("not a PDF file (missing %%PDF header)")
	}

	pageCount := countPages(data)

	allowedPages, err := parsePageRange(pageRange, pageCount)
	if err != nil {
		return "", pageCount, fmt.Errorf("invalid page range: %w", err)
	}

	streams := collectStreams(data)

	var sb strings.Builder
	processed := 0

	for i, stream := range streams {
		if processed >= maxPages {
			break
		}

		// When a page range is specified, respect it. We map stream index to a
		// 1-based page number heuristically — not every stream is a page content
		// stream, but for simple PDFs this works well in practice.
		pageNum := i + 1
		if len(allowedPages) > 0 {
			if _, ok := allowedPages[pageNum]; !ok {
				continue
			}
		}

		text := parseContentStream(stream)
		if text == "" {
			continue
		}
		if sb.Len() > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString(text)
		processed++
	}

	return sb.String(), pageCount, nil
}

// countPages attempts to count the number of pages in the PDF by looking for
// the /Count entry in the Pages dictionary.
func countPages(data []byte) int {
	// Look for /Count followed by whitespace and digits.
	idx := bytes.Index(data, []byte("/Count"))
	if idx < 0 {
		return 0
	}

	rest := bytes.TrimLeft(data[idx+len("/Count"):], " \t\r\n")
	end := bytes.IndexAny(rest, " \t\r\n/>")
	if end < 0 {
		end = len(rest)
	}
	n, err := strconv.Atoi(string(rest[:end]))
	if err != nil || n < 0 {
		return 0
	}
	return n
}

// collectStreams finds all stream … endstream blocks in the PDF, attempting to
// decompress FlateDecode streams. Non-content streams (e.g. image data) are
// included but produce no text, which is harmless.
func collectStreams(data []byte) [][]byte {
	var streams [][]byte

	pos := 0
	for {
		// Find the next stream marker.
		streamIdx := bytes.Index(data[pos:], []byte("stream"))
		if streamIdx < 0 {
			break
		}
		streamIdx += pos

		// The byte immediately after "stream" must be \r\n or \n.
		start := streamIdx + len("stream")
		if start >= len(data) {
			break
		}
		if data[start] == '\r' {
			start++
		}
		if start >= len(data) || data[start] != '\n' {
			// Not a valid stream keyword; advance past this occurrence.
			pos = streamIdx + len("stream")
			continue
		}
		start++ // consume \n

		// Find matching endstream.
		endIdx := bytes.Index(data[start:], []byte("endstream"))
		if endIdx < 0 {
			break
		}
		endIdx += start

		rawStream := data[start:endIdx]

		// Check whether the object header before this stream declares
		// FlateDecode (zlib/deflate) compression.
		header := extractObjectHeader(data, streamIdx)
		if isFlateDecode(header) {
			if decompressed, err := decompressFlate(rawStream); err == nil {
				streams = append(streams, decompressed)
			}
			// If decompression fails, skip this stream — it's likely binary image data.
		} else if !isBinaryFilter(header) {
			// Only include streams without exotic filters so we don't feed
			// binary image or font data to the text parser.
			streams = append(streams, rawStream)
		}

		pos = endIdx + len("endstream")
	}

	return streams
}

// extractObjectHeader returns up to 512 bytes of the PDF object definition
// that precedes the stream keyword, used for filter detection.
func extractObjectHeader(data []byte, streamIdx int) []byte {
	const lookback = 512
	start := streamIdx - lookback
	if start < 0 {
		start = 0
	}
	return data[start:streamIdx]
}

// isFlateDecode reports whether a PDF object header requests FlateDecode.
func isFlateDecode(header []byte) bool {
	return bytes.Contains(header, []byte("FlateDecode")) ||
		bytes.Contains(header, []byte("Fl ")) ||
		bytes.Contains(header, []byte("/Fl\n")) ||
		bytes.Contains(header, []byte("/Fl\r"))
}

// isBinaryFilter reports whether a header requests a filter that produces
// binary output that we should not attempt to interpret as text.
func isBinaryFilter(header []byte) bool {
	binaryFilters := [][]byte{
		[]byte("DCTDecode"), // JPEG
		[]byte("CCITTFaxDecode"),
		[]byte("JBIG2Decode"),
		[]byte("JPXDecode"),
		[]byte("LZWDecode"),
		[]byte("RunLengthDecode"),
		[]byte("Crypt"),
	}
	for _, f := range binaryFilters {
		if bytes.Contains(header, f) {
			return true
		}
	}
	return false
}

// decompressFlate decompresses raw deflate/zlib data from a FlateDecode
// stream. PDF FlateDecode uses zlib wrapping (RFC 1950), but some producers
// omit the zlib header; we try both.
func decompressFlate(data []byte) ([]byte, error) {
	// Trim trailing whitespace that PDF producers sometimes append.
	trimmed := bytes.TrimRight(data, "\r\n \t")

	// Try zlib (2-byte header 0x78 ...) first.
	if len(trimmed) >= 2 && trimmed[0] == 0x78 {
		r := flate.NewReader(bytes.NewReader(trimmed[2:])) // skip zlib header
		defer r.Close()
		out, err := io.ReadAll(io.LimitReader(r, int64(maxFileSize)))
		if err == nil {
			return out, nil
		}
	}

	// Fall back to raw deflate.
	r := flate.NewReader(bytes.NewReader(trimmed))
	defer r.Close()
	out, err := io.ReadAll(io.LimitReader(r, int64(maxFileSize)))
	if err != nil {
		return nil, fmt.Errorf("flate decompress: %w", err)
	}
	return out, nil
}

// parseContentStream scans a PDF content stream for text-showing operators and
// returns the extracted plain text. Supported operators:
//
//   - Tj  — show string
//   - TJ  — show array of strings and spacing
//   - '   — move to next line and show string
//   - "   — set word/char spacing, move to next line, show string
func parseContentStream(stream []byte) string {
	var sb strings.Builder
	pos := 0
	data := stream

	for pos < len(data) {
		// Skip whitespace.
		if isWhitespace(data[pos]) {
			pos++
			continue
		}

		// Detect the start of a string literal: '('
		if data[pos] == '(' {
			str, advance := parsePDFString(data[pos:])
			pos += advance

			// Peek at the operator that follows the string.
			opStart := pos
			for opStart < len(data) && isWhitespace(data[opStart]) {
				opStart++
			}

			op, opLen := peekOperator(data[opStart:])
			switch op {
			case "Tj", "'", "\"":
				sb.WriteString(str)
				sb.WriteString(" ")
				pos = opStart + opLen
			default:
				pos = opStart
			}
			continue
		}

		// Detect the start of an array: '[' — used by TJ.
		if data[pos] == '[' {
			text, advance := parseTJArray(data[pos:])
			pos += advance

			// Peek for "TJ" operator.
			opStart := pos
			for opStart < len(data) && isWhitespace(data[opStart]) {
				opStart++
			}
			op, opLen := peekOperator(data[opStart:])
			if op == "TJ" {
				sb.WriteString(text)
				sb.WriteString(" ")
				pos = opStart + opLen
			}
			continue
		}

		// Skip over hex strings <...> (used for encoded fonts; skip for now).
		if data[pos] == '<' {
			end := bytes.IndexByte(data[pos:], '>')
			if end < 0 {
				break
			}
			pos += end + 1
			continue
		}

		// Skip over names /Name.
		if data[pos] == '/' {
			pos++
			for pos < len(data) && !isWhitespace(data[pos]) && data[pos] != '/' && data[pos] != '[' && data[pos] != '(' {
				pos++
			}
			continue
		}

		// Skip any other token (number, keyword, boolean, etc.).
		pos++
	}

	return strings.TrimSpace(sb.String())
}

// parsePDFString parses a PDF literal string starting at data[0] == '(', returning
// the decoded string and the number of bytes consumed (including the parentheses).
func parsePDFString(data []byte) (string, int) {
	if len(data) == 0 || data[0] != '(' {
		return "", 0
	}

	var sb strings.Builder
	depth := 0
	i := 0

	for i < len(data) {
		ch := data[i]
		switch {
		case ch == '(' && (i == 0 || data[i-1] != '\\'):
			depth++
			if depth > 1 {
				sb.WriteByte(ch)
			}
			i++
		case ch == ')' && (i == 0 || data[i-1] != '\\'):
			depth--
			if depth == 0 {
				i++
				return sb.String(), i
			}
			sb.WriteByte(ch)
			i++
		case ch == '\\' && i+1 < len(data):
			// PDF escape sequences.
			next := data[i+1]
			switch next {
			case 'n':
				sb.WriteByte('\n')
			case 'r':
				sb.WriteByte('\r')
			case 't':
				sb.WriteByte('\t')
			case 'b':
				sb.WriteByte('\b')
			case 'f':
				sb.WriteByte('\f')
			case '(':
				sb.WriteByte('(')
			case ')':
				sb.WriteByte(')')
			case '\\':
				sb.WriteByte('\\')
			default:
				// Octal escape \ddd
				if next >= '0' && next <= '7' {
					end := i + 2
					for end < i+5 && end < len(data) && data[end] >= '0' && data[end] <= '7' {
						end++
					}
					n, _ := strconv.ParseInt(string(data[i+1:end]), 8, 32)
					sb.WriteByte(byte(n))
					i = end
					continue
				}
				sb.WriteByte(next)
			}
			i += 2
		default:
			if depth > 0 {
				// Only emit printable ASCII; skip control chars and high bytes
				// that are font-encoded glyph indices.
				if ch >= 0x20 && ch < 0x7F {
					sb.WriteByte(ch)
				} else if ch == '\n' || ch == '\r' || ch == '\t' {
					sb.WriteByte(' ')
				}
			}
			i++
		}
	}

	return sb.String(), i
}

// parseTJArray parses a TJ array [ ... ] and concatenates all literal string
// operands, ignoring numeric kerning adjustments.
func parseTJArray(data []byte) (string, int) {
	if len(data) == 0 || data[0] != '[' {
		return "", 0
	}

	var sb strings.Builder
	i := 1 // skip '['

	for i < len(data) {
		ch := data[i]
		if isWhitespace(ch) {
			i++
			continue
		}
		if ch == ']' {
			return sb.String(), i + 1
		}
		if ch == '(' {
			str, advance := parsePDFString(data[i:])
			sb.WriteString(str)
			i += advance
			continue
		}
		// Number (kerning value) or other — skip token.
		i++
	}

	return sb.String(), i
}

// peekOperator reads the next PDF operator from data (which must start at a
// non-whitespace byte). Returns the operator string and its byte length.
func peekOperator(data []byte) (string, int) {
	end := 0
	for end < len(data) && !isWhitespace(data[end]) && data[end] != '(' && data[end] != '[' && data[end] != '/' {
		end++
	}
	if end == 0 {
		return "", 0
	}
	return string(data[:end]), end
}

// isWhitespace reports whether b is a PDF whitespace character.
func isWhitespace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\r' || b == '\n' || b == '\f' || b == 0x00
}

// ---------------------------------------------------------------------------
// Title extraction
// ---------------------------------------------------------------------------

// extractTitle tries to find the document title from the PDF Info dictionary.
func extractTitle(data []byte) string {
	idx := bytes.Index(data, []byte("/Title"))
	if idx < 0 {
		return ""
	}

	rest := bytes.TrimLeft(data[idx+len("/Title"):], " \t\r\n")
	if len(rest) == 0 {
		return ""
	}

	if rest[0] == '(' {
		title, _ := parsePDFString(rest)
		return strings.TrimSpace(title)
	}

	return ""
}

// ---------------------------------------------------------------------------
// Page range parsing
// ---------------------------------------------------------------------------

// parsePageRange converts a page range string like "1-5,7,9-11" into a set of
// 1-based page numbers. An empty string means all pages (returns nil map).
func parsePageRange(spec string, total int) (map[int]struct{}, error) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return nil, nil
	}

	pages := make(map[int]struct{})
	parts := strings.Split(spec, ",")

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		if dashIdx := strings.Index(part, "-"); dashIdx >= 0 {
			// Range: "start-end"
			startStr := strings.TrimSpace(part[:dashIdx])
			endStr := strings.TrimSpace(part[dashIdx+1:])

			start, err := strconv.Atoi(startStr)
			if err != nil || start < 1 {
				return nil, fmt.Errorf("invalid page number %q", startStr)
			}
			end, err := strconv.Atoi(endStr)
			if err != nil || end < 1 {
				return nil, fmt.Errorf("invalid page number %q", endStr)
			}
			if start > end {
				return nil, fmt.Errorf("invalid page range %q: start > end", part)
			}

			// Clamp to total if known.
			if total > 0 && end > total {
				end = total
			}

			for p := start; p <= end; p++ {
				pages[p] = struct{}{}
			}
		} else {
			// Single page number.
			n, err := strconv.Atoi(part)
			if err != nil || n < 1 {
				return nil, fmt.Errorf("invalid page number %q", part)
			}
			pages[n] = struct{}{}
		}
	}

	return pages, nil
}
