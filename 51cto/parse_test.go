package fiftyone

import (
	"strings"
	"testing"
)

func TestStripHTML(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"<b>hello</b>", "hello"},
		{"<a href='x'>link text</a>", "link text"},
		{"no tags", "no tags"},
		{"<p>  extra   spaces  </p>", "extra spaces"},
		{"&amp; &lt; &gt; &quot;", "& < > \""},
	}
	for _, tc := range cases {
		got := stripHTML(tc.in)
		if got != tc.want {
			t.Errorf("stripHTML(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestAbsURL(t *testing.T) {
	cases := []struct {
		href, base, want string
	}{
		{"/article/123", "https://www.51cto.com", "https://www.51cto.com/article/123"},
		{"https://example.com/page", "https://www.51cto.com", "https://example.com/page"},
		{"//cdn.example.com/img.png", "https://www.51cto.com", "https://cdn.example.com/img.png"},
		{"", "https://www.51cto.com", ""},
		{"#", "https://www.51cto.com", ""},
	}
	for _, tc := range cases {
		got := absURL(tc.href, tc.base)
		if got != tc.want {
			t.Errorf("absURL(%q, %q) = %q, want %q", tc.href, tc.base, got, tc.want)
		}
	}
}

func TestTruncate(t *testing.T) {
	if got := truncate("hello", 10); got != "hello" {
		t.Errorf("truncate short = %q", got)
	}
	long := strings.Repeat("a", 200)
	got := truncate(long, 50)
	// truncate cuts to n-1 chars and appends "..." = n+2 total
	if len([]rune(got)) != 52 {
		t.Errorf("truncate long = %d runes, want 52 (49 + '...')", len([]rune(got)))
	}
	if !strings.HasSuffix(got, "...") {
		t.Errorf("truncate long should end with ..., got %q", got)
	}
}

func TestArticleIDFromURL(t *testing.T) {
	cases := []struct {
		url, want string
	}{
		{"/article/786963.html", "786963"},
		{"/article/786963", "786963"},
		{"https://www.51cto.com/article/100001.html", "100001"},
		{"/other/path", ""},
		{"", ""},
	}
	for _, tc := range cases {
		got := articleIDFromURL(tc.url)
		if got != tc.want {
			t.Errorf("articleIDFromURL(%q) = %q, want %q", tc.url, got, tc.want)
		}
	}
}

func TestParseHotArticles(t *testing.T) {
	html := `<html><body>
<div class="article-item">
  <a href="/article/111111">First Article Title</a>
</div>
<div class="article-item">
  <a href="/article/222222.html">Second Article Title</a>
</div>
<a href="/other/page">Not an article</a>
<a href="/article/333333">Third Article</a>
</body></html>`

	arts := ParseHotArticles([]byte(html), "https://www.51cto.com", 0)
	if len(arts) != 3 {
		t.Errorf("want 3 articles, got %d: %+v", len(arts), arts)
	}
	if arts[0].ID != "111111" {
		t.Errorf("arts[0].ID = %q, want 111111", arts[0].ID)
	}
	if arts[1].ID != "222222" {
		t.Errorf("arts[1].ID = %q, want 222222", arts[1].ID)
	}
}

func TestParseHotArticles_Limit(t *testing.T) {
	html := `<html><body>
<a href="/article/1">Art 1</a>
<a href="/article/2">Art 2</a>
<a href="/article/3">Art 3</a>
</body></html>`

	arts := ParseHotArticles([]byte(html), "https://www.51cto.com", 2)
	if len(arts) != 2 {
		t.Errorf("want 2 articles (limit), got %d", len(arts))
	}
}

func TestParseHotArticles_Dedup(t *testing.T) {
	// Same ID appearing multiple times should only appear once.
	html := `<html><body>
<a href="/article/111111">First</a>
<a href="/article/111111.html">First again</a>
<a href="/article/222222">Second</a>
</body></html>`

	arts := ParseHotArticles([]byte(html), "https://www.51cto.com", 0)
	if len(arts) != 2 {
		t.Errorf("want 2 unique articles, got %d: %+v", len(arts), arts)
	}
}

func TestParseArticle(t *testing.T) {
	html := `<html><head>
<meta name="og:title" content="Golang Guide" />
<meta name="author" content="gopher" />
<meta name="description" content="A guide to Go." />
<meta name="article:published_time" content="2024-03-01T00:00:00Z" />
<meta name="og:keywords" content="golang,go,programming" />
</head><body>
<div class="article-content">This is the article body.</div>
</body></html>`

	art := ParseArticle([]byte(html), "786963", "https://www.51cto.com/article/786963.html")
	if art.ID != "786963" {
		t.Errorf("ID = %q, want 786963", art.ID)
	}
	if art.Author != "gopher" {
		t.Errorf("Author = %q, want gopher", art.Author)
	}
	if !strings.Contains(art.Summary, "guide") {
		t.Errorf("Summary = %q, should contain 'guide'", art.Summary)
	}
	if art.Published != "2024-03-01T00:00:00Z" {
		t.Errorf("Published = %q, want 2024-03-01T00:00:00Z", art.Published)
	}
}

func TestParseSearchResults(t *testing.T) {
	html := `<html><body>
<div class="result">
  <a href="/article/333333">Search Result One</a>
</div>
<div class="result">
  <a href="/article/444444.html">Search Result Two</a>
</div>
<a href="/unrelated">Not a result</a>
</body></html>`

	results := ParseSearchResults([]byte(html), "https://so.51cto.com", "article", 10)
	if len(results) != 2 {
		t.Errorf("want 2 results, got %d: %+v", len(results), results)
	}
	if results[0].ID != "333333" {
		t.Errorf("results[0].ID = %q, want 333333", results[0].ID)
	}
	if results[0].Type != "article" {
		t.Errorf("results[0].Type = %q, want article", results[0].Type)
	}
}

func TestParseBlogPosts(t *testing.T) {
	html := `<html><body>
<ul class="list-blog">
  <li><a href="/testuser/1234567">Post Alpha</a></li>
  <li><a href="/testuser/1234568">Post Beta</a></li>
  <li><a href="/other/page">Not a post</a></li>
</ul>
</body></html>`

	posts := ParseBlogPosts([]byte(html), "testuser", "https://blog.51cto.com", 10)
	if len(posts) != 2 {
		t.Errorf("want 2 posts, got %d: %+v", len(posts), posts)
	}
	if posts[0].ID != "1234567" {
		t.Errorf("posts[0].ID = %q, want 1234567", posts[0].ID)
	}
	if posts[0].Author != "testuser" {
		t.Errorf("posts[0].Author = %q, want testuser", posts[0].Author)
	}
}

func TestParseRSSItems(t *testing.T) {
	rss := `<?xml version="1.0"?>
<rss version="2.0">
<channel>
<title>Test Blog</title>
<item>
  <title>First Post</title>
  <link>https://blog.51cto.com/testuser/1234567</link>
  <pubDate>Mon, 01 Jan 2024 00:00:00 +0000</pubDate>
  <author>testauthor</author>
  <description>A description of the first post.</description>
</item>
<item>
  <title>Second Post</title>
  <link>https://blog.51cto.com/testuser/1234568</link>
  <pubDate>Tue, 02 Jan 2024 00:00:00 +0000</pubDate>
</item>
</channel>
</rss>`

	arts := ParseRSSItems([]byte(rss), 10)
	if len(arts) != 2 {
		t.Errorf("want 2 items, got %d: %+v", len(arts), arts)
	}
	if arts[0].Title != "First Post" {
		t.Errorf("arts[0].Title = %q, want First Post", arts[0].Title)
	}
	if arts[0].Author != "testauthor" {
		t.Errorf("arts[0].Author = %q, want testauthor", arts[0].Author)
	}
	if !strings.Contains(arts[0].Summary, "description") {
		t.Errorf("arts[0].Summary = %q, should contain 'description'", arts[0].Summary)
	}
}

func TestParseRSSItems_Limit(t *testing.T) {
	rss := `<rss><channel>
<item><title>A</title><link>https://blog.51cto.com/u/100001</link></item>
<item><title>B</title><link>https://blog.51cto.com/u/100002</link></item>
<item><title>C</title><link>https://blog.51cto.com/u/100003</link></item>
</channel></rss>`

	arts := ParseRSSItems([]byte(rss), 2)
	if len(arts) != 2 {
		t.Errorf("want 2 items (limit), got %d", len(arts))
	}
}

func TestParseIntFromString(t *testing.T) {
	cases := []struct {
		s    string
		want int
	}{
		{"12345次", 12345},
		{"12,345", 12345},
		{"0", 0},
		{"no number", 0},
		{"", 0},
	}
	for _, tc := range cases {
		got := parseIntFromString(tc.s)
		if got != tc.want {
			t.Errorf("parseIntFromString(%q) = %d, want %d", tc.s, got, tc.want)
		}
	}
}

func TestExtractMeta(t *testing.T) {
	html := `<html><head>
<meta name="author" content="gopher" />
<meta property="og:title" content="Test Title" />
<meta content="desc" name="description" />
</head></html>`

	if got := extractMeta(html, "author"); got != "gopher" {
		t.Errorf("extractMeta author = %q, want gopher", got)
	}
	if got := extractMeta(html, "og:title"); got != "Test Title" {
		t.Errorf("extractMeta og:title = %q, want Test Title", got)
	}
	if got := extractMeta(html, "description"); got != "desc" {
		t.Errorf("extractMeta description = %q, want desc", got)
	}
	if got := extractMeta(html, "missing"); got != "" {
		t.Errorf("extractMeta missing = %q, want empty", got)
	}
}

func TestClassifyURL(t *testing.T) {
	cases := []struct {
		url, typ, id string
	}{
		{"/article/786963", "article", "786963"},
		{"/article/786963.html", "article", "786963"},
		{"/superwen/1234567", "blog", "superwen/1234567"},
		{"/other/page", "", ""},
	}
	for _, tc := range cases {
		typ, id := classifyURL(tc.url)
		if typ != tc.typ || id != tc.id {
			t.Errorf("classifyURL(%q) = (%q, %q), want (%q, %q)", tc.url, typ, id, tc.typ, tc.id)
		}
	}
}
