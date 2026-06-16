package fiftyone

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// --- HTML/RSS parsing helpers for 51CTO pages. ---
//
// The parsers use regexp and strings rather than an HTML parser to keep
// the dependency footprint minimal. Each function takes a raw HTML body
// and returns typed records. They return empty slices (not errors) when
// the page structure does not match, so command output degrades gracefully.

var (
	// hrefRE matches href attributes in anchor tags.
	hrefRE = regexp.MustCompile(`href="([^"]+)"`)

	// tagRE strips HTML tags.
	tagRE = regexp.MustCompile(`<[^>]+>`)

	// entityRE replaces common HTML entities.
	entityRE = regexp.MustCompile(`&[a-zA-Z0-9#]+;`)

	// articleLinkRE matches article paths like /article/123456 or /article/123456.html
	articleLinkRE = regexp.MustCompile(`/article/(\d+)(?:\.html)?`)

	// blogLinkRE matches blog post paths like /username/12345678
	blogLinkRE = regexp.MustCompile(`/([a-zA-Z0-9_-]+)/(\d{6,})(?:\.html)?$`)

	// intRE extracts a leading integer from a string like "12345次" or "12,345".
	intRE = regexp.MustCompile(`[\d,]+`)
)

// stripHTML removes all HTML tags and normalizes whitespace.
func stripHTML(s string) string {
	s = tagRE.ReplaceAllString(s, " ")
	s = decodeEntities(s)
	return strings.Join(strings.Fields(s), " ")
}

// decodeEntities replaces common HTML entities with their text equivalents.
func decodeEntities(s string) string {
	r := strings.NewReplacer(
		"&amp;", "&",
		"&lt;", "<",
		"&gt;", ">",
		"&quot;", `"`,
		"&#39;", "'",
		"&nbsp;", " ",
		"&apos;", "'",
	)
	return r.Replace(s)
}

// truncate cuts s to at most n runes.
func truncate(s string, n int) string {
	rs := []rune(s)
	if len(rs) <= n {
		return s
	}
	return string(rs[:n-1]) + "..."
}

// absURL resolves a possibly-relative href against a base host.
func absURL(href, baseSchemeHost string) string {
	href = strings.TrimSpace(href)
	if href == "" || href == "#" {
		return ""
	}
	if strings.HasPrefix(href, "http://") || strings.HasPrefix(href, "https://") {
		return href
	}
	if strings.HasPrefix(href, "//") {
		return "https:" + href
	}
	if strings.HasPrefix(href, "/") {
		return baseSchemeHost + href
	}
	return ""
}

// parseIntFromString extracts the first integer from a string like "12,345次".
func parseIntFromString(s string) int {
	m := intRE.FindString(s)
	if m == "" {
		return 0
	}
	m = strings.ReplaceAll(m, ",", "")
	n, _ := strconv.Atoi(m)
	return n
}

// articleIDFromURL extracts the numeric ID from a 51CTO article URL.
// e.g. "https://www.51cto.com/article/786963.html" -> "786963"
func articleIDFromURL(rawURL string) string {
	m := articleLinkRE.FindStringSubmatch(rawURL)
	if len(m) >= 2 {
		return m[1]
	}
	return ""
}

// ParseHotArticles extracts the list of hot articles from the 51CTO home page HTML.
// Returns whatever it can find; an empty slice is not an error.
func ParseHotArticles(body []byte, baseURL string, limit int) []Article {
	s := string(body)
	var out []Article
	seen := map[string]bool{}

	// Find all anchor tags that point to article pages.
	for _, m := range hrefRE.FindAllStringSubmatch(s, -1) {
		href := m[1]
		if !articleLinkRE.MatchString(href) {
			continue
		}
		absHref := absURL(href, baseURL)
		if absHref == "" {
			continue
		}
		id := articleIDFromURL(href)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true

		// Try to extract the title from the surrounding context.
		// Look for the title in a window of text around this link.
		title := extractTitleNearHref(s, href)

		out = append(out, Article{
			ID:    id,
			Title: title,
			URL:   absHref,
		})
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out
}

// extractTitleNearHref finds the text content of an anchor tag for the given href.
func extractTitleNearHref(html, href string) string {
	// Find the anchor tag with this href and extract its text content.
	escapedHref := regexp.QuoteMeta(href)
	re := regexp.MustCompile(`<a[^>]+href="` + escapedHref + `"[^>]*>(.*?)</a>`)
	m := re.FindStringSubmatch(html)
	if len(m) >= 2 {
		t := stripHTML(m[1])
		if t != "" {
			return truncate(t, 120)
		}
	}
	return href
}

// ParseArticle extracts a single article from its page HTML.
func ParseArticle(body []byte, id, rawURL string) *Article {
	s := string(body)

	title := extractMeta(s, "og:title")
	if title == "" {
		title = extractBetweenTags(s, "<title>", "</title>")
		title = stripHTML(title)
	}

	author := extractMeta(s, "author")
	if author == "" {
		author = extractMeta(s, "og:site_name")
	}

	published := extractMeta(s, "article:published_time")
	if published == "" {
		published = extractMeta(s, "og:article:published_time")
	}

	summary := extractMeta(s, "description")
	if summary == "" {
		summary = extractMeta(s, "og:description")
	}
	summary = truncate(summary, 200)

	// Extract tags from <a class="tag"> or similar patterns.
	tags := extractTags(s)

	// Extract body text from the main content area.
	body2 := extractBody(s)

	return &Article{
		ID:        id,
		Title:     title,
		URL:       rawURL,
		Author:    author,
		Summary:   summary,
		Published: published,
		Tags:      tags,
		Body:      body2,
	}
}

// extractMeta pulls a meta tag content by name or property.
func extractMeta(html, key string) string {
	// Try name= attribute
	re := regexp.MustCompile(
		`<meta[^>]+(?:name|property)="` + regexp.QuoteMeta(key) + `"[^>]+content="([^"]*)"`)
	if m := re.FindStringSubmatch(html); len(m) >= 2 {
		return decodeEntities(m[1])
	}
	// Try content= before name=
	re2 := regexp.MustCompile(
		`<meta[^>]+content="([^"]*)"[^>]+(?:name|property)="` + regexp.QuoteMeta(key) + `"`)
	if m := re2.FindStringSubmatch(html); len(m) >= 2 {
		return decodeEntities(m[1])
	}
	return ""
}

// extractBetweenTags returns the text between two tag strings.
func extractBetweenTags(html, open, close string) string {
	i := strings.Index(html, open)
	if i < 0 {
		return ""
	}
	i += len(open)
	j := strings.Index(html[i:], close)
	if j < 0 {
		return ""
	}
	return html[i : i+j]
}

// extractTags finds tag links in the article HTML.
func extractTags(html string) []string {
	// Look for patterns like <a ... class="tag">text</a>
	re := regexp.MustCompile(`class="[^"]*tag[^"]*"[^>]*>([^<]+)<`)
	var tags []string
	seen := map[string]bool{}
	for _, m := range re.FindAllStringSubmatch(html, 20) {
		t := strings.TrimSpace(m[1])
		if t != "" && !seen[t] {
			seen[t] = true
			tags = append(tags, t)
		}
	}
	return tags
}

// extractBody extracts the main text content from an article page.
func extractBody(html string) string {
	// Try to find a content div.
	for _, marker := range []string{
		`class="article-content"`,
		`class="post-content"`,
		`class="content-wrap"`,
		`id="article-content"`,
		`<article`,
	} {
		i := strings.Index(html, marker)
		if i < 0 {
			continue
		}
		// Find the containing element's close.
		start := strings.LastIndex(html[:i], "<")
		if start < 0 {
			continue
		}
		// Take up to 10KB of content from this point.
		end := start + 10240
		if end > len(html) {
			end = len(html)
		}
		text := stripHTML(html[start:end])
		text = truncate(text, 2000)
		if len(text) > 100 {
			return text
		}
	}
	return ""
}

// ParseSearchResults extracts search results from a 51CTO search page.
func ParseSearchResults(body []byte, baseURL, contentType string, limit int) []SearchResult {
	s := string(body)
	var out []SearchResult
	seen := map[string]bool{}

	// Search results typically appear as list items with title links.
	// We look for anchor tags pointing to article or blog URLs.
	for _, m := range hrefRE.FindAllStringSubmatch(s, -1) {
		href := m[1]
		if !isContentURL(href) {
			continue
		}
		absHref := absURL(href, baseURL)
		if absHref == "" {
			// Try absolute URL directly
			if strings.HasPrefix(href, "http") {
				absHref = href
			} else {
				continue
			}
		}

		itemType, id := classifyURL(href)
		if id == "" || seen[id] {
			continue
		}
		if contentType != "" && contentType != "all" && itemType != contentType {
			continue
		}
		seen[id] = true

		title := extractTitleNearHref(s, href)

		out = append(out, SearchResult{
			ID:    id,
			Type:  itemType,
			Title: title,
			URL:   absHref,
		})
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out
}

// isContentURL reports whether href points to a 51CTO article or blog post.
func isContentURL(href string) bool {
	return articleLinkRE.MatchString(href) ||
		blogLinkRE.MatchString(href)
}

// classifyURL returns the content type and ID from a 51CTO URL.
func classifyURL(href string) (contentType, id string) {
	if m := articleLinkRE.FindStringSubmatch(href); len(m) >= 2 {
		return "article", m[1]
	}
	if m := blogLinkRE.FindStringSubmatch(href); len(m) >= 3 {
		return "blog", fmt.Sprintf("%s/%s", m[1], m[2])
	}
	return "", ""
}

// ParseBlogPosts extracts blog posts from a 51CTO user blog page.
func ParseBlogPosts(body []byte, username, blogURL string, limit int) []BlogPost {
	s := string(body)
	var out []BlogPost
	seen := map[string]bool{}

	// Blog post links look like /username/12345678 or /username/12345678.html
	re := regexp.MustCompile(`href="((?:https?://blog\.51cto\.com)?/` +
		regexp.QuoteMeta(username) + `/(\d{6,})(?:\.html)?)"`)

	for _, m := range re.FindAllStringSubmatch(s, -1) {
		href := m[1]
		postID := m[2]
		if seen[postID] {
			continue
		}
		seen[postID] = true

		absHref := absURL(href, blogURL)
		if absHref == "" {
			absHref = blogURL + "/" + username + "/" + postID
		}

		title := extractTitleNearHref(s, href)

		out = append(out, BlogPost{
			ID:     postID,
			Title:  title,
			URL:    absHref,
			Author: username,
		})
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out
}

// ParseRSSItems extracts articles from an RSS 2.0 or Atom feed body.
func ParseRSSItems(body []byte, limit int) []Article {
	s := string(body)
	var out []Article

	// Split on <item> tags.
	parts := splitOnTag(s, "<item>", "</item>")
	for _, part := range parts {
		title := stripHTML(extractBetweenTags(part, "<title>", "</title>"))
		link := strings.TrimSpace(extractBetweenTags(part, "<link>", "</link>"))
		if link == "" {
			// Some feeds have <link href="..." />
			link = extractMeta(part, "href")
		}
		desc := stripHTML(extractBetweenTags(part, "<description>", "</description>"))
		pubDate := strings.TrimSpace(extractBetweenTags(part, "<pubDate>", "</pubDate>"))
		author := strings.TrimSpace(extractBetweenTags(part, "<author>", "</author>"))
		if author == "" {
			author = strings.TrimSpace(extractBetweenTags(part, "<dc:creator>", "</dc:creator>"))
		}

		id := articleIDFromURL(link)
		if id == "" {
			id = link
		}
		if id == "" {
			continue
		}

		summary := truncate(desc, 200)

		out = append(out, Article{
			ID:        id,
			Title:     title,
			URL:       link,
			Author:    author,
			Summary:   summary,
			Published: pubDate,
		})
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out
}

// splitOnTag splits an HTML/XML string on occurrences of open/close tag pairs.
func splitOnTag(s, open, close string) []string {
	var parts []string
	for {
		i := strings.Index(s, open)
		if i < 0 {
			break
		}
		s = s[i+len(open):]
		j := strings.Index(s, close)
		if j < 0 {
			break
		}
		parts = append(parts, s[:j])
		s = s[j+len(close):]
	}
	return parts
}
