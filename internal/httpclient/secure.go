package httpclient

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"
)

// SecureClientConfig defines security settings for HTTP clients
type SecureClientConfig struct {
	// Timeout settings
	ConnectTimeout  time.Duration
	TLSTimeout      time.Duration
	ResponseTimeout time.Duration
	TotalTimeout    time.Duration

	// Security settings
	MaxRedirects     int
	AllowPrivateIPs  bool
	AllowInsecureTLS bool

	// Custom user agent
	UserAgent string
}

// DefaultSecureConfig returns secure defaults for API clients
func DefaultSecureConfig() SecureClientConfig {
	return SecureClientConfig{
		ConnectTimeout:   10 * time.Second,
		TLSTimeout:       10 * time.Second,
		ResponseTimeout:  30 * time.Second,
		TotalTimeout:     60 * time.Second,
		MaxRedirects:     3,
		AllowPrivateIPs:  false,
		AllowInsecureTLS: false,
		UserAgent:        "SoulGate/0.1",
	}
}

// NewSecureClient creates an HTTP client with security hardening
func NewSecureClient(cfg SecureClientConfig) *http.Client {
	// Custom dialer with timeout and IP filtering
	dialer := &net.Dialer{
		Timeout:   cfg.ConnectTimeout,
		KeepAlive: 30 * time.Second,
		DualStack: true,
	}

	// Wrap dialer with IP filtering
	var dialFunc func(context.Context, string, string) (net.Conn, error)
	if !cfg.AllowPrivateIPs {
		dialFunc = func(ctx context.Context, network, addr string) (net.Conn, error) {
			// Resolve address
			host, _, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, fmt.Errorf("invalid address: %w", err)
			}

			// Resolve to IP
			ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
			if err != nil {
				return nil, fmt.Errorf("failed to resolve host: %w", err)
			}

			// Check for private/internal IPs
			for _, ip := range ips {
				if isPrivateIP(ip.IP) {
					return nil, fmt.Errorf("connections to private IP addresses are blocked: %s", ip.IP)
				}
			}

			return dialer.DialContext(ctx, network, addr)
		}
	} else {
		dialFunc = dialer.DialContext
	}

	// Custom transport with security settings
	transport := &http.Transport{
		DialContext:           dialFunc,
		TLSHandshakeTimeout:   cfg.TLSTimeout,
		ResponseHeaderTimeout: cfg.ResponseTimeout,
		ExpectContinueTimeout: 1 * time.Second,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,

		// TLS configuration
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: cfg.AllowInsecureTLS,
			MinVersion:         tls.VersionTLS12,
		},

		// Disable compression to prevent decompression bombs
		DisableCompression: false,

		// Limit connection pooling
		MaxIdleConnsPerHost: 10,
	}

	// Create client with redirect policy
	client := &http.Client{
		Transport: transport,
		Timeout:   cfg.TotalTimeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= cfg.MaxRedirects {
				return fmt.Errorf("too many redirects (max: %d)", cfg.MaxRedirects)
			}

			// Block redirect to private IPs if configured
			if !cfg.AllowPrivateIPs {
				if err := validatePublicURL(req.URL); err != nil {
					return fmt.Errorf("redirect blocked: %w", err)
				}
			}

			return nil
		},
	}

	return client
}

// isPrivateIP checks if an IP is private/internal
func isPrivateIP(ip net.IP) bool {
	// Check for loopback
	if ip.IsLoopback() {
		return true
	}

	// Check for link-local
	if ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return true
	}

	// Check for private ranges (RFC 1918)
	privateRanges := []string{
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		"169.254.0.0/16", // Link-local
		"127.0.0.0/8",    // Loopback
		"::1/128",        // IPv6 loopback
		"fc00::/7",       // IPv6 private
		"fe80::/10",      // IPv6 link-local
	}

	for _, cidr := range privateRanges {
		_, network, _ := net.ParseCIDR(cidr)
		if network != nil && network.Contains(ip) {
			return true
		}
	}

	return false
}

// validatePublicURL validates that a URL does not point to private resources
func validatePublicURL(u *url.URL) error {
	// Resolve hostname
	ips, err := net.LookupIP(u.Hostname())
	if err != nil {
		return fmt.Errorf("failed to resolve hostname: %w", err)
	}

	// Check all resolved IPs
	for _, ip := range ips {
		if isPrivateIP(ip) {
			return fmt.Errorf("URL resolves to private IP: %s", ip)
		}
	}

	return nil
}

// RoundTripperWithUserAgent adds a custom user agent
type RoundTripperWithUserAgent struct {
	Transport http.RoundTripper
	UserAgent string
}

func (r *RoundTripperWithUserAgent) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("User-Agent", r.UserAgent)
	return r.Transport.RoundTrip(req)
}
