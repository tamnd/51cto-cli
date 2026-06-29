package fiftyone

import (
	"context"
	"fmt"
	"strings"
)

// Hot fetches the trending articles from the 51CTO home page.
// On bot-block it returns ErrBlocked.
func (c *Client) Hot(ctx context.Context, category string, limit int) ([]Article, error) {
	u := c.cfg.BaseURL
	if category != "" {
		// Category pages are under /technology/<slug>/ or /<slug>/
		u = c.cfg.BaseURL + "/" + strings.ToLower(strings.TrimSpace(category))
	}
	body, err := c.Get(ctx, u)
	if err != nil {
		return nil, err
	}
	arts := ParseHotArticles(body, c.cfg.BaseURL, limit)
	return arts, nil
}

// Search runs a keyword search on 51CTO and returns matching results.
func (c *Client) Search(ctx context.Context, query, contentType string, limit int) ([]SearchResult, error) {
	if query == "" {
		return nil, fmt.Errorf("query must not be empty")
	}

	// Map contentType parameter to the site's type parameter.
	typeParam := mapSearchType(contentType)

	u := fmt.Sprintf("%s/?q=%s&type=%s",
		c.cfg.SearchURL,
		urlEncode(query),
		typeParam,
	)
	body, err := c.Get(ctx, u)
	if err != nil {
		return nil, err
	}
	return ParseSearchResults(body, c.cfg.SearchURL, contentType, limit), nil
}

// Article fetches a single article by numeric ID.
func (c *Client) Article(ctx context.Context, id string) (*Article, error) {
	u := fmt.Sprintf("%s/article/%s.html", c.cfg.BaseURL, id)
	body, err := c.Get(ctx, u)
	if err != nil {
		// Also try without .html suffix.
		u2 := fmt.Sprintf("%s/article/%s", c.cfg.BaseURL, id)
		body2, err2 := c.Get(ctx, u2)
		if err2 != nil {
			return nil, err // return original error
		}
		return ParseArticle(body2, id, u2), nil
	}
	return ParseArticle(body, id, u), nil
}

// Blog fetches the public blog post list for a 51CTO user.
func (c *Client) Blog(ctx context.Context, username string, limit int) ([]BlogPost, error) {
	if username == "" {
		return nil, fmt.Errorf("username must not be empty")
	}

	// Try RSS first; fall back to HTML.
	rssURL := fmt.Sprintf("%s/%s/rss", c.cfg.BlogURL, username)
	body, err := c.Get(ctx, rssURL)
	if err == nil {
		// Got something: try to parse as RSS.
		arts := ParseRSSItems(body, limit)
		if len(arts) > 0 {
			// Convert Article to BlogPost.
			var posts []BlogPost
			for _, a := range arts {
				posts = append(posts, BlogPost{
					ID:        a.ID,
					Title:     a.Title,
					URL:       a.URL,
					Author:    username,
					Summary:   a.Summary,
					Published: a.Published,
				})
			}
			return posts, nil
		}
	}

	// Fall back to HTML listing.
	pageURL := fmt.Sprintf("%s/%s", c.cfg.BlogURL, username)
	body, err = c.Get(ctx, pageURL)
	if err != nil {
		return nil, err
	}
	return ParseBlogPosts(body, username, c.cfg.BlogURL, limit), nil
}

// mapSearchType maps a user-visible type name to the site's query parameter value.
func mapSearchType(t string) string {
	switch strings.ToLower(t) {
	case "blog", "post":
		return "post"
	case "qa", "question":
		return "answer"
	default:
		return "article"
	}
}

// urlEncode percent-encodes a query string using simple substitution.
// Uses manual encoding to avoid importing net/url just for this.
func urlEncode(s string) string {
	// Use net/url via strings replacement of the most common characters.
	// Full percent-encoding for query parameters.
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'A' && r <= 'Z',
			r >= 'a' && r <= 'z',
			r >= '0' && r <= '9',
			r == '-', r == '_', r == '.', r == '~':
			b.WriteRune(r)
		case r == ' ':
			b.WriteByte('+')
		default:
			// Encode as UTF-8 percent-escaped bytes.
			for _, b2 := range []byte(string(r)) {
				fmt.Fprintf(&b, "%%%02X", b2)
			}
		}
	}
	return b.String()
}
