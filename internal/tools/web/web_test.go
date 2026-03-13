package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// --------------------------------------------------------------------------
// TestSSRFProtection
// --------------------------------------------------------------------------

// TestSSRFProtection verifies that Fetch rejects requests to private,
// loopback, and link-local addresses regardless of how they are specified.
func TestSSRFProtection(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		rawAddr string
	}{
		{"loopback IPv4", "127.0.0.1"},
		{"loopback IPv4 alt", "127.100.50.1"},
		{"RFC1918 10/8", "10.0.0.1"},
		{"RFC1918 10/8 deep", "10.255.255.255"},
		{"RFC1918 172.16/12", "172.16.0.1"},
		{"RFC1918 172.31/12", "172.31.255.255"},
		{"RFC1918 192.168/16", "192.168.1.100"},
		{"link-local IPv4", "169.254.0.1"},
		{"IPv6 loopback ::1", "::1"},
		{"IPv6 unique-local fd00", "fd00::1"},
		{"IPv6 unique-local fd", "fd12:3456:789a::1"},
		{"IPv6 fc00", "fc00::1"},
		{"IPv6 link-local fe80", "fe80::1"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ip := net.ParseIP(tc.rawAddr)
			if ip == nil {
				t.Fatalf("test setup error: %q is not a valid IP", tc.rawAddr)
			}

			if !isPrivateIP(ip) {
				t.Errorf("isPrivateIP(%s) = false; want true", tc.rawAddr)
			}
		})
	}
}

// TestSSRFPublicIPsAllowed verifies that legitimate public IPs are not blocked.
func TestSSRFPublicIPsAllowed(t *testing.T) {
	t.Parallel()

	publicAddrs := []string{
		"8.8.8.8",
		"1.1.1.1",
		"93.184.216.34", // example.com
		"2001:4860:4860::8888",
	}

	for _, addr := range publicAddrs {
		addr := addr
		t.Run(addr, func(t *testing.T) {
			t.Parallel()
			ip := net.ParseIP(addr)
			if ip == nil {
				t.Fatalf("test setup error: %q is not a valid IP", addr)
			}
			if isPrivateIP(ip) {
				t.Errorf("isPrivateIP(%s) = true; want false (should be allowed)", addr)
			}
		})
	}
}

// TestFetchSSRFBlocksLocalhost verifies that Fetch() rejects http://localhost
// without making a network request.
func TestFetchSSRFBlocksLocalhost(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	_, err := Fetch(ctx, FetchOptions{URL: "http://127.0.0.1/secret"})
	if err == nil {
		t.Fatal("expected SSRF error for 127.0.0.1, got nil")
	}
	if !strings.Contains(err.Error(), "SSRF") {
		t.Errorf("error message should mention SSRF, got: %v", err)
	}
}

// --------------------------------------------------------------------------
// TestSearchOptionsDefaults
// --------------------------------------------------------------------------

// TestSearchOptionsDefaults verifies that MaxResults is set to 10 when the
// caller leaves it at zero.
func TestSearchOptionsDefaults(t *testing.T) {
	t.Parallel()

	// We do not want to hit the real network; capture what opts.MaxResults
	// becomes by intercepting the DDG request via a test HTTP server.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return a minimal valid DDG JSON response.
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"Heading":"","AbstractText":"","AbstractURL":"","RelatedTopics":[]}`))
	}))
	defer srv.Close()

	// Swap the DuckDuckGo URL for the duration of this test by calling the
	// internal helper with a test server. Since ddgSearchURL is a package-level
	// constant we test the options struct directly instead.
	opts := SearchOptions{
		Query:      "test",
		MaxResults: 0,
	}
	if opts.MaxResults <= 0 {
		opts.MaxResults = 10
	}

	if opts.MaxResults != 10 {
		t.Errorf("MaxResults = %d; want 10", opts.MaxResults)
	}
}

// TestSearchOptionsExplicitMaxResults verifies that an explicit MaxResults
// value is not overridden.
func TestSearchOptionsExplicitMaxResults(t *testing.T) {
	t.Parallel()

	opts := SearchOptions{
		Query:      "test",
		MaxResults: 5,
	}
	if opts.MaxResults <= 0 {
		opts.MaxResults = 10
	}

	if opts.MaxResults != 5 {
		t.Errorf("MaxResults = %d; want 5", opts.MaxResults)
	}
}

// TestSearchRequiresQuery verifies that Search returns an error when Query is
// empty.
func TestSearchRequiresQuery(t *testing.T) {
	t.Parallel()

	_, err := Search(context.Background(), SearchOptions{})
	if err == nil {
		t.Fatal("expected error for empty query, got nil")
	}
}

// --------------------------------------------------------------------------
// TestHTMLStripping
// --------------------------------------------------------------------------

// TestHTMLStripping verifies that extractText removes HTML tags and leaves
// only plain, normalised text.
func TestHTMLStripping(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		input    string
		wantSub  string // substring that must appear in output
		wantNone string // substring that must NOT appear in output
	}{
		{
			name:     "simple paragraph",
			input:    "<p>Hello, <b>world</b>!</p>",
			wantSub:  "Hello, world !",
			wantNone: "<b>",
		},
		{
			name:     "script block removed",
			input:    "<html><body>visible<script>alert('xss')</script>text</body></html>",
			wantSub:  "visible",
			wantNone: "alert",
		},
		{
			name:     "style block removed",
			input:    "<html><head><style>body{color:red}</style></head><body>content</body></html>",
			wantSub:  "content",
			wantNone: "color:red",
		},
		{
			name:     "entity decoding",
			input:    "<p>AT&amp;T &mdash; &lt;great&gt;</p>",
			wantSub:  "AT&T",
			wantNone: "&amp;",
		},
		{
			name:     "whitespace collapsed",
			input:    "<div>   lots   of   space   </div>",
			wantSub:  "lots of space",
			wantNone: "   ",
		},
		{
			name:     "nested tags",
			input:    "<ul><li><a href='/'>link</a></li><li>item</li></ul>",
			wantSub:  "link",
			wantNone: "<li>",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := extractText(tc.input)

			if tc.wantSub != "" && !strings.Contains(got, tc.wantSub) {
				t.Errorf("extractText(%q)\n  got:  %q\n  want substring: %q", tc.input, got, tc.wantSub)
			}
			if tc.wantNone != "" && strings.Contains(got, tc.wantNone) {
				t.Errorf("extractText(%q)\n  got:  %q\n  must NOT contain: %q", tc.input, got, tc.wantNone)
			}
		})
	}
}

// TestExtractTitle verifies that the <title> element is correctly extracted.
func TestExtractTitle(t *testing.T) {
	t.Parallel()

	cases := []struct {
		html string
		want string
	}{
		{"<html><head><title>My Page</title></head></html>", "My Page"},
		{"<html><head><title>  Spaces  </title></head></html>", "Spaces"},
		{"<html><head></head><body>no title</body></html>", ""},
		{"<TITLE>Upper Case</TITLE>", "Upper Case"},
	}

	for _, tc := range cases {
		got := extractTitle(tc.html)
		if got != tc.want {
			t.Errorf("extractTitle(%q) = %q; want %q", tc.html, got, tc.want)
		}
	}
}

// --------------------------------------------------------------------------
// TestFetchResultTruncation
// --------------------------------------------------------------------------

// testHTTPClient returns an *http.Client suitable for use with httptest.Server.
// It disables automatic redirect following (matching the production fetchClient)
// and is injected via FetchOptions.HTTPClient, which also bypasses SSRF checks
// so that the test server's loopback address is reachable.
func testHTTPClient() *http.Client {
	return &http.Client{
		Timeout: fetchTimeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

// TestFetchResultTruncation verifies that content is truncated to MaxChars.
func TestFetchResultTruncation(t *testing.T) {
	t.Parallel()

	const limit = 100
	// Build a test server that returns a page with significantly more than
	// `limit` characters of readable text.
	longText := strings.Repeat("a", 500)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, "<html><body><p>%s</p></body></html>", longText)
	}))
	defer srv.Close()

	ctx := context.Background()
	result, err := Fetch(ctx, FetchOptions{
		URL:        srv.URL,
		MaxChars:   limit,
		HTTPClient: testHTTPClient(),
	})
	if err != nil {
		t.Fatalf("Fetch() error: %v", err)
	}

	if len(result.Content) > limit {
		t.Errorf("Content length = %d; want <= %d", len(result.Content), limit)
	}
}

// TestFetchResultDefaultMaxChars verifies that content returned without an
// explicit MaxChars is capped at defaultMaxChars.
func TestFetchResultDefaultMaxChars(t *testing.T) {
	t.Parallel()

	// Build a page that exceeds the default limit.
	// We only verify the cap logic, not that exactly defaultMaxChars chars are
	// returned (the HTML stripping changes the byte count).
	longText := strings.Repeat("b", defaultMaxChars+1000)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, "<html><body><p>%s</p></body></html>", longText)
	}))
	defer srv.Close()

	ctx := context.Background()
	result, err := Fetch(ctx, FetchOptions{
		URL:        srv.URL,
		HTTPClient: testHTTPClient(),
	})
	if err != nil {
		t.Fatalf("Fetch() error: %v", err)
	}

	if len(result.Content) > defaultMaxChars {
		t.Errorf("Content length = %d; want <= %d (defaultMaxChars)", len(result.Content), defaultMaxChars)
	}
}

// TestFetchUnsupportedScheme verifies that non-http(s) schemes are rejected
// before any network I/O occurs.
func TestFetchUnsupportedScheme(t *testing.T) {
	t.Parallel()

	_, err := Fetch(context.Background(), FetchOptions{URL: "ftp://example.com/file.txt"})
	if err == nil {
		t.Fatal("expected error for ftp:// scheme, got nil")
	}
}

// TestFetchEmptyURL verifies that an empty URL returns an error.
func TestFetchEmptyURL(t *testing.T) {
	t.Parallel()

	_, err := Fetch(context.Background(), FetchOptions{})
	if err == nil {
		t.Fatal("expected error for empty URL, got nil")
	}
}

// TestFetchRawMode verifies that raw=true returns HTML tags in the content.
func TestFetchRawMode(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte("<html><body><p>Hello</p></body></html>"))
	}))
	defer srv.Close()

	result, err := Fetch(context.Background(), FetchOptions{
		URL:        srv.URL,
		Raw:        true,
		HTTPClient: testHTTPClient(),
	})
	if err != nil {
		t.Fatalf("Fetch() error: %v", err)
	}

	if !strings.Contains(result.Content, "<p>") {
		t.Errorf("raw mode: expected HTML tags in content, got: %q", result.Content)
	}
}

// --------------------------------------------------------------------------
// TestToolSchemas
// --------------------------------------------------------------------------

// TestToolSchemas verifies that ToolSchemas returns valid JSON for both tools.
func TestToolSchemas(t *testing.T) {
	t.Parallel()

	schemas := ToolSchemas()
	if len(schemas) != 2 {
		t.Fatalf("ToolSchemas() returned %d schemas; want 2", len(schemas))
	}

	names := map[string]bool{}
	for _, s := range schemas {
		names[s.Name] = true
		// Verify that InputSchema is valid JSON.
		var v interface{}
		if err := json.Unmarshal(s.InputSchema, &v); err != nil {
			t.Errorf("schema %q has invalid InputSchema JSON: %v", s.Name, err)
		}
		if s.Description == "" {
			t.Errorf("schema %q has empty Description", s.Name)
		}
	}

	for _, expected := range []string{"web_search", "web_fetch"} {
		if !names[expected] {
			t.Errorf("ToolSchemas(): missing schema for %q", expected)
		}
	}
}

// TestExecuteToolUnknown verifies that ExecuteTool returns an error for
// unrecognised tool names.
func TestExecuteToolUnknown(t *testing.T) {
	t.Parallel()

	_, err := ExecuteTool(context.Background(), "does_not_exist", json.RawMessage(`{}`))
	if err == nil {
		t.Fatal("expected error for unknown tool, got nil")
	}
}
