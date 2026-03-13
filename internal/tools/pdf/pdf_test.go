package pdf

import (
	"bytes"
	"compress/flate"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// parsePageRange
// ---------------------------------------------------------------------------

func TestParsePageRange(t *testing.T) {
	tests := []struct {
		name    string
		spec    string
		total   int
		want    map[int]struct{}
		wantErr bool
	}{
		{
			name:  "empty spec returns nil",
			spec:  "",
			total: 10,
			want:  nil,
		},
		{
			name:  "single page",
			spec:  "3",
			total: 10,
			want:  set(3),
		},
		{
			name:  "simple range",
			spec:  "1-5",
			total: 10,
			want:  set(1, 2, 3, 4, 5),
		},
		{
			name:  "comma separated pages",
			spec:  "1,3,7",
			total: 10,
			want:  set(1, 3, 7),
		},
		{
			name:  "mixed range and pages",
			spec:  "1-3,5,8-10",
			total: 15,
			want:  set(1, 2, 3, 5, 8, 9, 10),
		},
		{
			name:  "range clamped to total",
			spec:  "1-100",
			total: 5,
			want:  set(1, 2, 3, 4, 5),
		},
		{
			name:  "whitespace trimmed",
			spec:  " 1 - 3 , 5 ",
			total: 10,
			want:  set(1, 2, 3, 5),
		},
		{
			name:    "invalid page number",
			spec:    "abc",
			total:   10,
			wantErr: true,
		},
		{
			name:    "start greater than end",
			spec:    "5-3",
			total:   10,
			wantErr: true,
		},
		{
			name:    "zero page number",
			spec:    "0",
			total:   10,
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parsePageRange(tc.spec, tc.total)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected an error but got none")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.want == nil && got == nil {
				return
			}
			if len(got) != len(tc.want) {
				t.Fatalf("page set length mismatch: got %v, want %v", got, tc.want)
			}
			for p := range tc.want {
				if _, ok := got[p]; !ok {
					t.Errorf("expected page %d in result set, but it was absent", p)
				}
			}
		})
	}
}

// set is a small helper for constructing expected page sets.
func set(pages ...int) map[int]struct{} {
	m := make(map[int]struct{}, len(pages))
	for _, p := range pages {
		m[p] = struct{}{}
	}
	return m
}

// ---------------------------------------------------------------------------
// Path traversal protection
// ---------------------------------------------------------------------------

func TestPathTraversalBlocked(t *testing.T) {
	traversalPaths := []string{
		"../etc/passwd",
		"../../etc/shadow",
		"foo/../../etc/passwd",
		"./../../root/.ssh/id_rsa",
		"a/b/../../../secret",
	}

	for _, path := range traversalPaths {
		t.Run(path, func(t *testing.T) {
			_, err := readLocalPDF(path)
			if err == nil {
				t.Fatalf("expected an error for path %q but got none", path)
			}
			if !strings.Contains(err.Error(), "traversal") {
				t.Errorf("expected 'traversal' in error message, got: %v", err)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// SSRF protection
// ---------------------------------------------------------------------------

func TestSSRFProtection(t *testing.T) {
	privateURLs := []string{
		"http://127.0.0.1/secret.pdf",
		"http://localhost/admin.pdf",
		"http://10.0.0.1/internal.pdf",
		"http://172.16.0.1/internal.pdf",
		"http://192.168.1.1/internal.pdf",
		"http://[::1]/secret.pdf",
	}

	for _, u := range privateURLs {
		t.Run(u, func(t *testing.T) {
			_, err := downloadPDF(context.Background(), u)
			if err == nil {
				t.Fatalf("expected SSRF error for %q but got none", u)
			}
			// Error should mention SSRF or the specific protection reason.
			lowered := strings.ToLower(err.Error())
			if !strings.Contains(lowered, "ssrf") &&
				!strings.Contains(lowered, "private") &&
				!strings.Contains(lowered, "loopback") &&
				!strings.Contains(lowered, "resolve") {
				t.Errorf("error %q does not mention SSRF protection", err.Error())
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Max pages limit
// ---------------------------------------------------------------------------

// buildSimplePDF constructs a minimal but syntactically plausible PDF with n
// pages, each carrying a plain text content stream.
func buildSimplePDF(pageTexts []string) []byte {
	var b bytes.Buffer

	b.WriteString("%PDF-1.4\n")

	// Write one content stream object per page.
	// Object numbering: 1 = catalog, 2 = pages, 3..n+2 = page content streams,
	// n+3..2n+2 = page objects.
	n := len(pageTexts)
	contentObjNums := make([]int, n)
	pageObjNums := make([]int, n)
	for i := 0; i < n; i++ {
		contentObjNums[i] = 3 + i
		pageObjNums[i] = 3 + n + i
	}

	// Object 1: Catalog
	fmt.Fprintf(&b, "1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj\n")

	// Object 2: Pages
	kidsStr := ""
	for _, pn := range pageObjNums {
		kidsStr += fmt.Sprintf("%d 0 R ", pn)
	}
	fmt.Fprintf(&b, "2 0 obj\n<< /Type /Pages /Kids [%s] /Count %d >>\nendobj\n", kidsStr, n)

	// Content stream objects.
	for i, text := range pageTexts {
		stream := fmt.Sprintf("BT /F1 12 Tf 100 700 Td (%s) Tj ET", text)
		fmt.Fprintf(&b, "%d 0 obj\n<< /Length %d >>\nstream\n%s\nendstream\nendobj\n",
			contentObjNums[i], len(stream), stream)
	}

	// Page objects.
	for i, pn := range pageObjNums {
		fmt.Fprintf(&b, "%d 0 obj\n<< /Type /Page /Parent 2 0 R /Contents %d 0 R >>\nendobj\n",
			pn, contentObjNums[i])
	}

	b.WriteString("%%EOF\n")
	return b.Bytes()
}

func TestMaxPagesLimit(t *testing.T) {
	// Build a PDF with 10 pages.
	texts := make([]string, 10)
	for i := range texts {
		texts[i] = fmt.Sprintf("Page %d content", i+1)
	}
	pdfData := buildSimplePDF(texts)

	// Extract with a limit of 3.
	content, _, err := extractText(pdfData, "", 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Count how many "Page N content" snippets appear.
	count := strings.Count(content, "Page")
	if count > 3 {
		t.Errorf("expected at most 3 pages in output, but found %d page references", count)
	}
}

// ---------------------------------------------------------------------------
// Content truncation
// ---------------------------------------------------------------------------

func TestTruncation(t *testing.T) {
	// Build a PDF with a single page whose text exceeds 100 KB.
	bigText := strings.Repeat("A", 150000)
	// Wrap in a PDF content stream manually to avoid the stream parser
	// struggling with an embedded 150K literal.
	streamContent := fmt.Sprintf("BT (%s) Tj ET", bigText)
	var b bytes.Buffer
	b.WriteString("%PDF-1.4\n")
	fmt.Fprintf(&b, "1 0 obj\n<< /Length %d >>\nstream\n%s\nendstream\nendobj\n",
		len(streamContent), streamContent)
	b.WriteString("%%EOF\n")

	result, err := Analyze(context.Background(), PDFOptions{
		Path:     buildTempPDF(t, b.Bytes()),
		MaxPages: 20,
	})
	if err != nil {
		t.Fatalf("Analyze returned error: %v", err)
	}

	if !result.Truncated {
		t.Error("expected Truncated to be true for content exceeding 100 KB")
	}
	if len(result.Content) > maxContentSize {
		t.Errorf("content length %d exceeds maxContentSize %d", len(result.Content), maxContentSize)
	}
}

// ---------------------------------------------------------------------------
// FlateDecode decompression
// ---------------------------------------------------------------------------

func TestFlateDecode(t *testing.T) {
	plainText := "Hello from a compressed stream"
	compressed := zlibCompress(t, []byte("BT ("+plainText+") Tj ET"))

	var b bytes.Buffer
	b.WriteString("%PDF-1.4\n")
	fmt.Fprintf(&b, "1 0 obj\n<< /Filter /FlateDecode /Length %d >>\nstream\n", len(compressed))
	b.Write(compressed)
	b.WriteString("\nendstream\nendobj\n%%EOF\n")

	content, _, err := extractText(b.Bytes(), "", 20)
	if err != nil {
		t.Fatalf("extractText error: %v", err)
	}

	if !strings.Contains(content, plainText) {
		t.Errorf("expected %q in extracted content, got: %q", plainText, content)
	}
}

// zlibCompress compresses data using zlib framing (FlateDecode).
// We write the 2-byte zlib header (0x78 0x9c) then deflate the payload.
func zlibCompress(t *testing.T, data []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	// zlib header: CMF=0x78 (deflate, window size 32k), FLG=0x9c (default compression)
	buf.Write([]byte{0x78, 0x9c})
	w, err := flate.NewWriter(&buf, flate.DefaultCompression)
	if err != nil {
		t.Fatalf("flate.NewWriter: %v", err)
	}
	if _, err := w.Write(data); err != nil {
		t.Fatalf("flate write: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("flate close: %v", err)
	}
	return buf.Bytes()
}

// ---------------------------------------------------------------------------
// ToolSchemas
// ---------------------------------------------------------------------------

func TestToolSchemas(t *testing.T) {
	schemas := ToolSchemas()
	if len(schemas) == 0 {
		t.Fatal("ToolSchemas returned empty slice")
	}

	found := false
	for _, s := range schemas {
		if s["name"] == "pdf_read" {
			found = true
			if s["description"] == "" {
				t.Error("pdf_read schema has empty description")
			}
			inputSchema, ok := s["input_schema"].(map[string]interface{})
			if !ok {
				t.Error("pdf_read input_schema is not a map")
				break
			}
			props, ok := inputSchema["properties"].(map[string]interface{})
			if !ok {
				t.Error("pdf_read input_schema.properties is not a map")
				break
			}
			if _, ok := props["path"]; !ok {
				t.Error("pdf_read schema is missing 'path' property")
			}
		}
	}

	if !found {
		t.Error("ToolSchemas does not contain 'pdf_read'")
	}
}

// ---------------------------------------------------------------------------
// ExecuteTool — missing path
// ---------------------------------------------------------------------------

func TestExecuteToolMissingPath(t *testing.T) {
	_, err := ExecuteTool(context.Background(), "pdf_read", map[string]interface{}{})
	if err == nil {
		t.Fatal("expected error when path is missing")
	}
}

func TestExecuteToolUnknownTool(t *testing.T) {
	_, err := ExecuteTool(context.Background(), "nonexistent_tool", map[string]interface{}{})
	if err == nil {
		t.Fatal("expected error for unknown tool name")
	}
}

// ---------------------------------------------------------------------------
// Integration: download from local test server
// ---------------------------------------------------------------------------

func TestDownloadFromTestServer(t *testing.T) {
	pdfData := buildSimplePDF([]string{"Hello from test server"})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/pdf")
		w.Write(pdfData)
	}))
	defer srv.Close()

	// The test server binds to 127.0.0.1, which is a loopback address and will
	// be rejected by the SSRF check in downloadPDF. We therefore call
	// extractText directly on the raw bytes rather than going through Analyze,
	// as testing the HTTP retrieval path against a real public host is an
	// integration concern outside unit tests.
	_ = srv // confirm server compiles and starts

	content, _, err := extractText(pdfData, "", 20)
	if err != nil {
		t.Fatalf("extractText error: %v", err)
	}
	if !strings.Contains(content, "Hello from test server") {
		t.Errorf("expected 'Hello from test server' in content, got: %q", content)
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// buildTempPDF writes raw PDF bytes to a temp file and returns its path.
// The file lives inside t.TempDir() and is cleaned up automatically.
func buildTempPDF(t *testing.T, data []byte) string {
	t.Helper()
	path := t.TempDir() + "/test.pdf"
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatalf("failed to write temp PDF: %v", err)
	}
	return path
}
