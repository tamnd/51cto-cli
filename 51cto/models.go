package fiftyone

// Article represents a 51CTO article (hot feed or single-article fetch).
type Article struct {
	ID        string   `json:"id"`
	Title     string   `json:"title"`
	URL       string   `json:"url"`
	Author    string   `json:"author"`
	Category  string   `json:"category"`
	Summary   string   `json:"summary"`
	Published string   `json:"published"`
	Views     int      `json:"views"`
	Comments  int      `json:"comments"`
	Body      string   `json:"body,omitempty"`
	Tags      []string `json:"tags,omitempty"`
}

// SearchResult represents one item from a search results page.
type SearchResult struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Title   string `json:"title"`
	URL     string `json:"url"`
	Author  string `json:"author"`
	Snippet string `json:"snippet"`
	Date    string `json:"date"`
}

// BlogPost represents a post on a 51CTO user blog.
type BlogPost struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	URL       string `json:"url"`
	Author    string `json:"author"`
	Summary   string `json:"summary"`
	Published string `json:"published"`
	Views     int    `json:"views"`
}
