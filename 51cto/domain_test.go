package fiftyone

import (
	"testing"
)

// Domain tests exercise the pure string functions (Classify, Locate) and
// the DomainInfo — no network access needed.

func TestDomainInfo(t *testing.T) {
	info := Domain{}.Info()
	if info.Scheme != "cto" {
		t.Errorf("Scheme = %q, want cto", info.Scheme)
	}
	if len(info.Hosts) == 0 || info.Hosts[0] != Host {
		t.Errorf("Hosts = %v, want [%s, ...]", info.Hosts, Host)
	}
	if info.Identity.Binary != "cto" {
		t.Errorf("Identity.Binary = %q, want cto", info.Identity.Binary)
	}
}

func TestClassify_NumericID(t *testing.T) {
	typ, id, err := Domain{}.Classify("786963")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if typ != "article" {
		t.Errorf("type = %q, want article", typ)
	}
	if id != "786963" {
		t.Errorf("id = %q, want 786963", id)
	}
}

func TestClassify_ArticleURL(t *testing.T) {
	cases := []struct {
		input string
		id    string
	}{
		{"https://www.51cto.com/article/786963.html", "786963"},
		{"https://www.51cto.com/article/786963", "786963"},
		{"http://www.51cto.com/article/100001.html", "100001"},
	}
	for _, tc := range cases {
		typ, id, err := Domain{}.Classify(tc.input)
		if err != nil {
			t.Errorf("Classify(%q): unexpected error: %v", tc.input, err)
			continue
		}
		if typ != "article" {
			t.Errorf("Classify(%q) type = %q, want article", tc.input, typ)
		}
		if id != tc.id {
			t.Errorf("Classify(%q) id = %q, want %q", tc.input, id, tc.id)
		}
	}
}

func TestClassify_BlogURL(t *testing.T) {
	typ, id, err := Domain{}.Classify("https://blog.51cto.com/superwen/1234567")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if typ != "blog" {
		t.Errorf("type = %q, want blog", typ)
	}
	if id != "superwen/1234567" {
		t.Errorf("id = %q, want superwen/1234567", id)
	}
}

func TestClassify_Empty(t *testing.T) {
	_, _, err := Domain{}.Classify("")
	if err == nil {
		t.Error("expected error for empty input")
	}
}

func TestClassify_Unknown(t *testing.T) {
	_, _, err := Domain{}.Classify("not-a-valid-reference")
	if err == nil {
		t.Error("expected error for unknown reference")
	}
}

func TestLocate_Article(t *testing.T) {
	got, err := Domain{}.Locate("article", "786963")
	want := "https://www.51cto.com/article/786963.html"
	if err != nil || got != want {
		t.Errorf("Locate(article, 786963) = (%q, %v), want (%q, nil)", got, err, want)
	}
}

func TestLocate_Blog(t *testing.T) {
	got, err := Domain{}.Locate("blog", "superwen/1234567")
	want := "https://blog.51cto.com/superwen/1234567"
	if err != nil || got != want {
		t.Errorf("Locate(blog, superwen/1234567) = (%q, %v), want (%q, nil)", got, err, want)
	}
}

func TestLocate_Unknown(t *testing.T) {
	_, err := Domain{}.Locate("unknown", "foo")
	if err == nil {
		t.Error("expected error for unknown type")
	}
}

func TestResolveArticleRef(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"786963", "786963"},
		{"https://www.51cto.com/article/786963.html", "786963"},
		{"https://www.51cto.com/article/786963", "786963"},
		{"notanid", ""},
		{"", ""},
	}
	for _, tc := range cases {
		got := resolveArticleRef(tc.input)
		if got != tc.want {
			t.Errorf("resolveArticleRef(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}
