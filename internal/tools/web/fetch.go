package web

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// FetchOptions configures a URL fetch request.
type FetchOptions struct {
	URL      string `json:"url"`
	MaxChars int    `json:"max_chars,omitempty"` // default 50000; 0 means use default
	Raw      bool   `json:"raw,omitempty"`       // return raw HTML instead of extracted text

	// HTTPClient overrides the default fetchClient when non-nil. Intended for
	// testing only. When set, SSRF protection is bypassed because the caller is
	// assumed to control the transport. Production callers must leave this nil.
	HTTPClient *http.Client `json:"-"`
}

// FetchResult holds the content retrieved from a URL.
type FetchResult struct {
	URL         string `json:"url"`
	Title       string `json:"title"`
	Content     string `json:"content"`
	ContentType string `json:"content_type"`
	StatusCode  int    `json:"status_code"`
}

const (
	defaultMaxChars  = 50_000
	fetchTimeout     = 30 * time.Second
	maxRedirects     = 5
	fetchUserAgent   = "SoulGate/1.0 (+https://github.com/M4MEET/soulgate)"
	maxBodyReadBytes = 10 * 1024 * 1024 // 10 MB — safety cap before content extraction
)

// fetchClient is shared across all Fetch calls.
// Redirects are handled manually so we can re-check each hop for SSRF.
var fetchClient = &http.Client{
	Timeout: fetchTimeout,
	// Disable automatic redirect following; we do it ourselves.
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	},
}

// Fetch retrieves the content at opts.URL and returns it as readable text
// (or raw HTML when opts.Raw is true).
//
// SSRF protection: private, loopback, and link-local addresses are rejected
// both on the initial URL and on every redirect target.
func Fetch(ctx context.Context, opts FetchOptions) (*FetchResult, error) {
	if opts.URL == "" {
		return nil, fmt.Errorf("fetch: url is required")
	}
	if opts.MaxChars <= 0 {
		opts.MaxChars = defaultMaxChars
	}

	// Use a caller-supplied client when provided (test injection). Otherwise
	// use the package-level default which disables automatic redirects.
	client := opts.HTTPClient
	if client == nil {
		client = fetchClient
	}
	// ssrfEnabled is true in production (no injected client) and false in tests.
	ssrfEnabled := opts.HTTPClient == nil

	currentURL := opts.URL
	var lastResp *http.Response

	for hop := 0; hop <= maxRedirects; hop++ {
		parsed, err := url.Parse(currentURL)
		if err != nil {
			return nil, fmt.Errorf("fetch: invalid URL %q: %w", currentURL, err)
		}

		// Validate scheme before we attempt any DNS resolution.
		if parsed.Scheme != "http" && parsed.Scheme != "https" {
			return nil, fmt.Errorf("fetch: unsupported scheme %q (only http/https allowed)", parsed.Scheme)
		}

		// SSRF check: resolve the hostname and reject private ranges.
		// Skipped when a custom HTTPClient is injected (test mode).
		if ssrfEnabled {
			if err := checkSSRF(ctx, parsed.Hostname()); err != nil {
				return nil, err
			}
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, currentURL, nil)
		if err != nil {
			return nil, fmt.Errorf("fetch: failed to build request: %w", err)
		}
		req.Header.Set("User-Agent", fetchUserAgent)
		req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
		req.Header.Set("Accept-Language", "en-US,en;q=0.5")

		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("fetch: request to %q failed: %w", currentURL, err)
		}

		if isRedirect(resp.StatusCode) {
			resp.Body.Close()
			loc := resp.Header.Get("Location")
			if loc == "" {
				return nil, fmt.Errorf("fetch: redirect with no Location header from %q", currentURL)
			}
			// Resolve relative redirect targets against the current URL.
			locParsed, err := url.Parse(loc)
			if err != nil {
				return nil, fmt.Errorf("fetch: invalid redirect location %q: %w", loc, err)
			}
			currentURL = parsed.ResolveReference(locParsed).String()

			if hop == maxRedirects {
				return nil, fmt.Errorf("fetch: exceeded %d redirects", maxRedirects)
			}
			continue
		}

		lastResp = resp
		break
	}

	if lastResp == nil {
		return nil, fmt.Errorf("fetch: no response received")
	}
	defer lastResp.Body.Close()

	contentType := lastResp.Header.Get("Content-Type")

	// Read body up to the safety cap so we do not buffer huge pages in memory.
	bodyBytes, err := io.ReadAll(io.LimitReader(lastResp.Body, maxBodyReadBytes))
	if err != nil {
		return nil, fmt.Errorf("fetch: failed to read response body: %w", err)
	}

	bodyStr := string(bodyBytes)
	var title, content string

	if opts.Raw {
		content = bodyStr
	} else {
		title = extractTitle(bodyStr)
		content = extractText(bodyStr)
	}

	// Apply the caller's character limit.
	if len(content) > opts.MaxChars {
		content = content[:opts.MaxChars]
	}

	return &FetchResult{
		URL:         currentURL,
		Title:       title,
		Content:     content,
		ContentType: contentType,
		StatusCode:  lastResp.StatusCode,
	}, nil
}

// --------------------------------------------------------------------------
// SSRF protection
// --------------------------------------------------------------------------

// checkSSRF resolves hostname and rejects any address in a private or
// otherwise sensitive range.
func checkSSRF(ctx context.Context, hostname string) error {
	// Literal IP addresses are parsed directly; hostnames are resolved.
	addrs, err := resolveHost(ctx, hostname)
	if err != nil {
		return fmt.Errorf("fetch: failed to resolve host %q: %w", hostname, err)
	}

	for _, addr := range addrs {
		ip := net.ParseIP(addr)
		if ip == nil {
			continue
		}
		if isPrivateIP(ip) {
			return fmt.Errorf("fetch: SSRF protection: address %s is not allowed", addr)
		}
	}

	return nil
}

// resolveHost returns the list of IP address strings for a hostname.
// If hostname is already a numeric IP it is returned directly.
func resolveHost(ctx context.Context, hostname string) ([]string, error) {
	// Strip IPv6 brackets if present.
	hostname = strings.Trim(hostname, "[]")

	if ip := net.ParseIP(hostname); ip != nil {
		return []string{ip.String()}, nil
	}

	resolver := &net.Resolver{}
	addrs, err := resolver.LookupHost(ctx, hostname)
	if err != nil {
		return nil, err
	}
	return addrs, nil
}

// isPrivateIP returns true for addresses that must never be reached by an
// agent-driven HTTP request (loopback, private RFC 1918 / RFC 4193, link-local,
// and the IPv6 loopback).
func isPrivateIP(ip net.IP) bool {
	// Normalise to 16-byte representation for uniform IPv6 handling.
	if ip4 := ip.To4(); ip4 != nil {
		ip = ip4
	}

	private4 := []net.IPNet{
		// 127.0.0.0/8 — loopback
		{IP: net.IP{127, 0, 0, 0}, Mask: net.CIDRMask(8, 32)},
		// 10.0.0.0/8 — RFC 1918
		{IP: net.IP{10, 0, 0, 0}, Mask: net.CIDRMask(8, 32)},
		// 172.16.0.0/12 — RFC 1918
		{IP: net.IP{172, 16, 0, 0}, Mask: net.CIDRMask(12, 32)},
		// 192.168.0.0/16 — RFC 1918
		{IP: net.IP{192, 168, 0, 0}, Mask: net.CIDRMask(16, 32)},
		// 169.254.0.0/16 — link-local
		{IP: net.IP{169, 254, 0, 0}, Mask: net.CIDRMask(16, 32)},
	}

	for i := range private4 {
		if private4[i].Contains(ip) {
			return true
		}
	}

	// IPv6 checks
	if ip.Equal(net.IPv6loopback) {
		return true
	}

	// fd00::/8 — unique local (RFC 4193)
	if ip[0] == 0xfd {
		return true
	}

	// fc00::/7 — broader unique local range
	if ip[0] == 0xfc {
		return true
	}

	// fe80::/10 — link-local
	if ip[0] == 0xfe && (ip[1]&0xc0) == 0x80 {
		return true
	}

	// ::1 — loopback (covered by net.IPv6loopback above, but be explicit)
	if ip.Equal(net.ParseIP("::1")) {
		return true
	}

	return false
}

// isRedirect returns true for HTTP redirect status codes.
func isRedirect(code int) bool {
	return code == http.StatusMovedPermanently ||
		code == http.StatusFound ||
		code == http.StatusSeeOther ||
		code == http.StatusTemporaryRedirect ||
		code == http.StatusPermanentRedirect
}

// --------------------------------------------------------------------------
// HTML content extraction
// --------------------------------------------------------------------------

// scriptStyleRe matches <script> and <style> blocks (including their content)
// so they can be removed before tag-stripping.
var scriptStyleRe = regexp.MustCompile(`(?is)<(script|style)[^>]*>.*?</(script|style)>`)

// htmlTagRe matches any HTML tag.
var htmlTagRe = regexp.MustCompile(`<[^>]+>`)

// whitespaceRe matches runs of whitespace characters.
var whitespaceRe = regexp.MustCompile(`\s+`)

// titleRe extracts the content of the <title> element.
var titleRe = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)

// extractTitle returns the text content of the first <title> element, or an
// empty string when none is found.
func extractTitle(html string) string {
	m := titleRe.FindStringSubmatch(html)
	if len(m) < 2 {
		return ""
	}
	return strings.TrimSpace(m[1])
}

// extractText strips HTML markup and returns normalised plain text that is
// suitable for inclusion in a model context window.
//
// The implementation uses a simple regex pipeline:
//  1. Remove <script> and <style> blocks entirely (they are not readable).
//  2. Strip all remaining HTML tags.
//  3. Decode common HTML entities.
//  4. Collapse whitespace.
func extractText(html string) string {
	// 1. Drop <script> / <style> content.
	text := scriptStyleRe.ReplaceAllString(html, " ")

	// 2. Strip tags.
	text = htmlTagRe.ReplaceAllString(text, " ")

	// 3. Decode a small set of common HTML entities.
	text = decodeEntities(text)

	// 4. Collapse whitespace and trim.
	text = whitespaceRe.ReplaceAllString(text, " ")
	text = strings.TrimSpace(text)

	return text
}

// decodeEntities decodes common HTML character references.
// A full HTML5 entity table is not needed here; the set below covers the
// majority of what appears in visible page text.
func decodeEntities(s string) string {
	replacer := strings.NewReplacer(
		"&amp;", "&",
		"&lt;", "<",
		"&gt;", ">",
		"&quot;", `"`,
		"&#39;", "'",
		"&apos;", "'",
		"&nbsp;", " ",
		"&mdash;", "—",
		"&ndash;", "–",
		"&lsquo;", "\u2018",
		"&rsquo;", "\u2019",
		"&ldquo;", "\u201C",
		"&rdquo;", "\u201D",
		"&hellip;", "…",
		"&copy;", "©",
		"&reg;", "®",
		"&trade;", "™",
	)
	return replacer.Replace(s)
}
