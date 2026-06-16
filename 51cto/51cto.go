// Package fiftyone is the library behind the cto command line:
// the HTTP client, request shaping, bot-detection, and the typed data models
// for 51cto.com.
//
// The entire 51cto.com ecosystem is protected by Tencent Cloud EdgeOne with a
// JavaScript-challenge mechanism. All routes return HTTP 567 (custom) and a
// JS-challenge page to unrecognized automated clients. This package detects
// that state and returns ErrBlocked so the caller can exit with code 5.
package fiftyone

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Host is the primary hostname for 51CTO.
const Host = "www.51cto.com"

// BlogHost is the blog subdomain.
const BlogHost = "blog.51cto.com"

// SearchHost is the search subdomain.
const SearchHost = "so.51cto.com"

// DefaultUserAgent is the User-Agent sent with every request.
const DefaultUserAgent = "Mozilla/5.0 (compatible; 51cto-cli/0.1.0)"

// BaseURL is the root every request is built from.
const BaseURL = "https://" + Host

// BlogURL is the root for blog requests.
const BlogURL = "https://" + BlogHost

// SearchURL is the root for search requests.
const SearchURL = "https://" + SearchHost

// ErrBlocked is returned when the site's bot-protection blocks the request.
// HTTP 567 from Tencent Cloud EdgeOne, or a JS-challenge response body.
var ErrBlocked = errors.New("site blocked: bot protection active (HTTP 567 / EdgeOne JS challenge)")

// ErrNotFound is returned when the requested resource does not exist.
var ErrNotFound = errors.New("not found")

// Config holds constructor parameters for Client.
type Config struct {
	BaseURL    string
	BlogURL    string
	SearchURL  string
	UserAgent  string
	Rate       time.Duration
	Retries    int
	Timeout    time.Duration
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() Config {
	return Config{
		BaseURL:   BaseURL,
		BlogURL:   BlogURL,
		SearchURL: SearchURL,
		UserAgent: DefaultUserAgent,
		Rate:      200 * time.Millisecond,
		Retries:   5,
		Timeout:   30 * time.Second,
	}
}

// Client is a rate-limited HTTP client for 51CTO.
type Client struct {
	cfg  Config
	http *http.Client
	mu   sync.Mutex
	last time.Time
}

// NewClient returns a Client with sensible defaults.
func NewClient(cfg Config) *Client {
	return &Client{
		cfg:  cfg,
		http: &http.Client{Timeout: cfg.Timeout},
	}
}

// Get fetches url and returns the response body. It paces and retries
// according to the client's settings, and detects bot-blocked responses.
func (c *Client) Get(ctx context.Context, url string) ([]byte, error) {
	var lastErr error
	for attempt := 0; attempt <= c.cfg.Retries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff(attempt)):
			}
		}
		body, retry, err := c.do(ctx, url)
		if err == nil {
			return body, nil
		}
		lastErr = err
		if !retry {
			return nil, err
		}
	}
	return nil, fmt.Errorf("get %s: %w", url, lastErr)
}

func (c *Client) do(ctx context.Context, url string) (body []byte, retry bool, err error) {
	c.pace()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("User-Agent", c.cfg.UserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, true, err
	}
	defer func() { _ = resp.Body.Close() }()

	// HTTP 567 is Tencent EdgeOne's custom bot-block code. No retry.
	if resp.StatusCode == 567 {
		return nil, false, ErrBlocked
	}
	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
		return nil, true, fmt.Errorf("http %d", resp.StatusCode)
	}
	if resp.StatusCode == http.StatusNotFound {
		return nil, false, ErrNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return nil, false, fmt.Errorf("http %d", resp.StatusCode)
	}

	b, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, true, err
	}

	// Detect JS challenge page even on HTTP 200: the EdgeOne challenge page
	// embeds a solveChallenge function call in the body.
	if isBlocked(b) {
		return nil, false, ErrBlocked
	}

	return b, false, nil
}

// isBlocked reports whether the response body is an EdgeOne JS challenge page.
func isBlocked(body []byte) bool {
	s := string(body)
	return strings.Contains(s, "solveChallenge") ||
		(strings.Contains(s, "EdgeOne") && strings.Contains(s, "requestId"))
}

// pace blocks until at least Rate has passed since the previous request.
func (c *Client) pace() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cfg.Rate <= 0 {
		return
	}
	if wait := c.cfg.Rate - time.Since(c.last); wait > 0 {
		time.Sleep(wait)
	}
	c.last = time.Now()
}

func backoff(attempt int) time.Duration {
	d := time.Duration(attempt) * 500 * time.Millisecond
	if d > 5*time.Second {
		d = 5 * time.Second
	}
	return d
}
