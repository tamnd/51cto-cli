package fiftyone

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func testClient(srv *httptest.Server) *Client {
	cfg := DefaultConfig()
	cfg.BaseURL = srv.URL
	cfg.BlogURL = srv.URL
	cfg.SearchURL = srv.URL
	cfg.Rate = 0
	cfg.Retries = 0
	return NewClient(cfg)
}

func TestGet_OK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") == "" {
			t.Error("request carried no User-Agent")
		}
		_, _ = w.Write([]byte("hello"))
	}))
	defer srv.Close()

	c := testClient(srv)
	body, err := c.Get(context.Background(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "hello" {
		t.Errorf("body = %q, want %q", body, "hello")
	}
}

func TestGet_Blocked567(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(567)
		_, _ = w.Write([]byte("blocked"))
	}))
	defer srv.Close()

	c := testClient(srv)
	_, err := c.Get(context.Background(), srv.URL)
	if err == nil {
		t.Fatal("expected ErrBlocked, got nil")
	}
	if !errors.Is(err, ErrBlocked) {
		t.Errorf("expected ErrBlocked in chain, got: %v", err)
	}
}

func TestGet_BlockedJSChallenge(t *testing.T) {
	// A body that looks like the EdgeOne JS challenge page.
	challengeBody := `<html><body><script>if(window.solveChallenge){var r=window.solveChallenge('xxx','yyy')}</script></body></html>`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write([]byte(challengeBody))
	}))
	defer srv.Close()

	c := testClient(srv)
	_, err := c.Get(context.Background(), srv.URL)
	if err == nil {
		t.Fatal("expected ErrBlocked for JS challenge page, got nil")
	}
	if !errors.Is(err, ErrBlocked) {
		t.Errorf("expected ErrBlocked, got: %v", err)
	}
}

func TestGet_NoRetryOn404(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	c := testClient(srv)
	c.cfg.Retries = 3
	_, err := c.Get(context.Background(), srv.URL)
	if err == nil {
		t.Fatal("expected error on 404")
	}
	if hits != 1 {
		t.Errorf("404 should not be retried, but server saw %d hits", hits)
	}
}

func TestGet_RetryOn503(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if hits < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte("recovered"))
	}))
	defer srv.Close()

	cfg := DefaultConfig()
	cfg.BaseURL = srv.URL
	cfg.Rate = 0
	cfg.Retries = 5
	c := NewClient(cfg)
	c.http.Timeout = 5 * time.Second

	body, err := c.Get(context.Background(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "recovered" {
		t.Errorf("body = %q after retries", body)
	}
	if hits != 3 {
		t.Errorf("server saw %d hits, want 3", hits)
	}
}

func TestIsBlocked(t *testing.T) {
	cases := []struct {
		body    string
		blocked bool
	}{
		{"<html>normal page</html>", false},
		{"window.solveChallenge('xxx')", true},
		{"EdgeOne requestId 12345", true},
		{"hello world", false},
	}
	for _, tc := range cases {
		got := isBlocked([]byte(tc.body))
		if got != tc.blocked {
			t.Errorf("isBlocked(%q) = %v, want %v", tc.body, got, tc.blocked)
		}
	}
}

func TestUrlEncode(t *testing.T) {
	cases := []struct {
		in, out string
	}{
		{"golang", "golang"},
		{"go lang", "go+lang"},
		{"go&lang", "go%26lang"},
		{"中文", "%E4%B8%AD%E6%96%87"},
	}
	for _, tc := range cases {
		got := urlEncode(tc.in)
		if got != tc.out {
			t.Errorf("urlEncode(%q) = %q, want %q", tc.in, got, tc.out)
		}
	}
}

func TestHot_Blocked(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(567)
		_, _ = w.Write([]byte("bot block"))
	}))
	defer srv.Close()

	c := testClient(srv)
	_, err := c.Hot(context.Background(), "", 10)
	if err == nil {
		t.Fatal("expected error on blocked request")
	}
	if !errors.Is(err, ErrBlocked) {
		t.Errorf("expected ErrBlocked, got: %v", err)
	}
}

func TestHot_OK(t *testing.T) {
	// Minimal HTML with two article links.
	html := `<html><body>
<a href="/article/111111">First Article</a>
<a href="/article/222222.html">Second Article</a>
<a href="/other/page">Not an article</a>
</body></html>`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(html))
	}))
	defer srv.Close()

	c := testClient(srv)
	arts, err := c.Hot(context.Background(), "", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(arts) != 2 {
		t.Errorf("want 2 articles, got %d", len(arts))
	}
	if arts[0].ID != "111111" {
		t.Errorf("art[0].ID = %q, want 111111", arts[0].ID)
	}
	if arts[1].ID != "222222" {
		t.Errorf("art[1].ID = %q, want 222222", arts[1].ID)
	}
}

func TestHot_Limit(t *testing.T) {
	html := `<html><body>
<a href="/article/1">Art 1</a>
<a href="/article/2">Art 2</a>
<a href="/article/3">Art 3</a>
</body></html>`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(html))
	}))
	defer srv.Close()

	c := testClient(srv)
	arts, err := c.Hot(context.Background(), "", 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(arts) != 2 {
		t.Errorf("want 2 articles (limit), got %d", len(arts))
	}
}

func TestSearch_OK(t *testing.T) {
	html := `<html><body>
<a href="/article/333333">Search Result One</a>
<a href="/article/444444.html">Search Result Two</a>
</body></html>`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("q")
		if !strings.Contains(q, "golang") && !strings.Contains(r.URL.RawQuery, "golang") {
			t.Errorf("search URL did not contain query, raw query: %q", r.URL.RawQuery)
		}
		_, _ = w.Write([]byte(html))
	}))
	defer srv.Close()

	c := testClient(srv)
	results, err := c.Search(context.Background(), "golang", "article", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Errorf("want 2 results, got %d: %+v", len(results), results)
	}
}

func TestSearch_Blocked(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(567)
	}))
	defer srv.Close()

	c := testClient(srv)
	_, err := c.Search(context.Background(), "test", "article", 10)
	if !errors.Is(err, ErrBlocked) {
		t.Errorf("want ErrBlocked, got %v", err)
	}
}

func TestArticle_OK(t *testing.T) {
	html := `<html><head>
<meta name="og:title" content="Test Article Title" />
<meta name="author" content="testauthor" />
<meta name="description" content="This is a test article." />
<meta name="article:published_time" content="2024-03-01T00:00:00Z" />
</head><body><h1>Test Article Title</h1>
<div class="article-content">Article body text here.</div>
</body></html>`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(html))
	}))
	defer srv.Close()

	c := testClient(srv)
	art, err := c.Article(context.Background(), "786963")
	if err != nil {
		t.Fatal(err)
	}
	if art.ID != "786963" {
		t.Errorf("art.ID = %q, want 786963", art.ID)
	}
}

func TestArticle_Blocked(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(567)
	}))
	defer srv.Close()

	c := testClient(srv)
	_, err := c.Article(context.Background(), "786963")
	if !errors.Is(err, ErrBlocked) {
		t.Errorf("want ErrBlocked, got %v", err)
	}
}

func TestBlog_RSS(t *testing.T) {
	rss := `<?xml version="1.0"?>
<rss version="2.0">
<channel>
<title>Test Blog</title>
<item><title>Post One</title><link>https://blog.51cto.com/testuser/1234567</link><pubDate>Mon, 01 Jan 2024 00:00:00 +0000</pubDate></item>
<item><title>Post Two</title><link>https://blog.51cto.com/testuser/1234568</link><pubDate>Tue, 02 Jan 2024 00:00:00 +0000</pubDate></item>
</channel>
</rss>`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		_, _ = w.Write([]byte(rss))
	}))
	defer srv.Close()

	c := testClient(srv)
	posts, err := c.Blog(context.Background(), "testuser", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(posts) != 2 {
		t.Errorf("want 2 posts, got %d: %+v", len(posts), posts)
	}
	if posts[0].Title != "Post One" {
		t.Errorf("posts[0].Title = %q, want %q", posts[0].Title, "Post One")
	}
}

func TestBlog_HTMLFallback(t *testing.T) {
	// When RSS path returns 404, fall back to HTML.
	html := `<html><body>
<a href="/testuser/1111111">Blog Post Alpha</a>
<a href="/testuser/2222222">Blog Post Beta</a>
</body></html>`

	mux := http.NewServeMux()
	mux.HandleFunc("/testuser/rss", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	mux.HandleFunc("/testuser", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(html))
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := testClient(srv)
	posts, err := c.Blog(context.Background(), "testuser", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(posts) != 2 {
		t.Errorf("want 2 posts from HTML fallback, got %d", len(posts))
	}
}

func TestBlog_Blocked(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(567)
	}))
	defer srv.Close()

	c := testClient(srv)
	_, err := c.Blog(context.Background(), "testuser", 10)
	if !errors.Is(err, ErrBlocked) {
		t.Errorf("want ErrBlocked, got %v", err)
	}
}
