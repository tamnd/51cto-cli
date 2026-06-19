package fiftyone

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/tamnd/any-cli/kit"
	"github.com/tamnd/any-cli/kit/errs"
)

// domain.go registers the fiftyone kit Domain so a blank import in a
// multi-domain host enables the driver:
//
//	import _ "github.com/tamnd/51cto-cli/51cto"
//
// The same Domain also builds the standalone cto binary (see cli.NewApp).
func init() { kit.Register(Domain{}) }

// Domain is the 51CTO driver. It carries no state; the per-run client is
// built by the factory Register hands kit.
type Domain struct{}

// Info describes the scheme, the hostnames, and the binary identity.
func (Domain) Info() kit.DomainInfo {
	return kit.DomainInfo{
		Scheme:  "cto",
		Aliases: []string{"fiftyone"},
		Hosts:   []string{Host, BlogHost, SearchHost},
		Identity: kit.Identity{
			Binary: "cto",
			Short:  "Browse 51CTO, China's IT community",
			Long: `cto reads public 51CTO data over plain HTTPS and prints clean structured records.

51CTO (51cto.com) is China's largest IT professional platform: technical articles,
blog posts, Q&A, courses, and videos for millions of Chinese developers.

Note: 51CTO is protected by Tencent Cloud EdgeOne bot-protection. Commands may
return exit code 5 (blocked) when run from datacenter IPs. Residential IPs and
browser sessions typically have better access.

Quick start:
  cto hot                     trending articles
  cto hot --category golang   trending Go articles
  cto search kubernetes       search for articles
  cto article 786963          fetch a single article
  cto blog superwen           list a user's blog posts`,
			Site: Host,
			Repo: "https://github.com/tamnd/51cto-cli",
		},
	}
}

// Register installs the client factory and operations onto app.
func (Domain) Register(app *kit.App) {
	app.SetClient(newClient)

	kit.Handle(app, kit.OpMeta{
		Name:    "hot",
		Group:   "browse",
		Summary: "List trending articles",
	}, hotHandler)

	kit.Handle(app, kit.OpMeta{
		Name:    "search",
		Group:   "browse",
		Summary: "Search 51CTO for articles and blog posts",
		Args:    []kit.Arg{{Name: "query", Help: "search keywords"}},
	}, searchHandler)

	kit.Handle(app, kit.OpMeta{
		Name:     "article",
		Group:    "read",
		Single:   true,
		Resolver: true,
		URIType:  "article",
		Summary:  "Fetch a single article by ID or URL",
		Args:     []kit.Arg{{Name: "ref", Help: "article ID or URL"}},
	}, articleHandler)

	kit.Handle(app, kit.OpMeta{
		Name:    "blog",
		Group:   "read",
		Summary: "List blog posts for a 51CTO user",
		Args:    []kit.Arg{{Name: "username", Help: "51CTO blog username"}},
	}, blogHandler)
}

// newClient builds a Client from the resolved kit Config.
func newClient(_ context.Context, cfg kit.Config) (any, error) {
	c := DefaultConfig()
	if cfg.UserAgent != "" {
		c.UserAgent = cfg.UserAgent
	}
	if cfg.Rate > 0 {
		c.Rate = cfg.Rate
	}
	if cfg.Retries > 0 {
		c.Retries = cfg.Retries
	}
	if cfg.Timeout > 0 {
		c.Timeout = cfg.Timeout
	}
	return NewClient(c), nil
}

// --- input structs ---

type hotInput struct {
	Category string  `kit:"flag" help:"technology category (golang, python, cloud, ...)"`
	Limit    int     `kit:"flag,inherit" help:"max articles" default:"20"`
	Client   *Client `kit:"inject"`
}

type searchInput struct {
	Query   string  `kit:"arg" help:"search keywords"`
	Type    string  `kit:"flag" help:"content type: article, blog, qa" default:"article"`
	Limit   int     `kit:"flag,inherit" help:"max results" default:"20"`
	Client  *Client `kit:"inject"`
}

type articleInput struct {
	Ref    string  `kit:"arg" help:"article ID or URL"`
	Client *Client `kit:"inject"`
}

type blogInput struct {
	Username string  `kit:"arg" help:"51CTO blog username"`
	Limit    int     `kit:"flag,inherit" help:"max posts" default:"20"`
	Client   *Client `kit:"inject"`
}

// --- handlers ---

func hotHandler(ctx context.Context, in hotInput, emit func(Article) error) error {
	arts, err := in.Client.Hot(ctx, in.Category, in.Limit)
	if err != nil {
		return mapErr(err)
	}
	if len(arts) == 0 {
		return errs.NotFound("no articles found")
	}
	for _, a := range arts {
		if err := emit(a); err != nil {
			return err
		}
	}
	return nil
}

func searchHandler(ctx context.Context, in searchInput, emit func(SearchResult) error) error {
	results, err := in.Client.Search(ctx, in.Query, in.Type, in.Limit)
	if err != nil {
		return mapErr(err)
	}
	if len(results) == 0 {
		return errs.NotFound("no results found for %q", in.Query)
	}
	for _, r := range results {
		if err := emit(r); err != nil {
			return err
		}
	}
	return nil
}

func articleHandler(ctx context.Context, in articleInput, emit func(*Article) error) error {
	id := resolveArticleRef(in.Ref)
	if id == "" {
		return errs.Usage("invalid article reference: %q (want numeric ID or 51cto.com URL)", in.Ref)
	}
	art, err := in.Client.Article(ctx, id)
	if err != nil {
		return mapErr(err)
	}
	return emit(art)
}

func blogHandler(ctx context.Context, in blogInput, emit func(BlogPost) error) error {
	posts, err := in.Client.Blog(ctx, in.Username, in.Limit)
	if err != nil {
		return mapErr(err)
	}
	if len(posts) == 0 {
		return errs.NotFound("no blog posts found for user %q", in.Username)
	}
	for _, p := range posts {
		if err := emit(p); err != nil {
			return err
		}
	}
	return nil
}

// --- Resolver ---

var articleIDRE = regexp.MustCompile(`^\d+$`)

// Classify turns any accepted input into the canonical (uriType, id).
func (Domain) Classify(input string) (uriType, id string, err error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return "", "", errs.Usage("cto: empty reference")
	}
	// Numeric → article.
	if articleIDRE.MatchString(input) {
		return "article", input, nil
	}
	// Full URL → classify by path.
	if u, e := url.Parse(input); e == nil && (u.Scheme == "http" || u.Scheme == "https") {
		if id := articleIDFromURL(u.Path); id != "" {
			return "article", id, nil
		}
		if m := blogLinkRE.FindStringSubmatch(u.Path); len(m) >= 3 {
			return "blog", fmt.Sprintf("%s/%s", m[1], m[2]), nil
		}
		return "", "", errs.Usage("cto: unrecognized URL path: %q", u.Path)
	}
	return "", "", errs.Usage("cto: unrecognized reference: %q", input)
}

// Locate returns the canonical URL for a (uriType, id).
func (Domain) Locate(uriType, id string) (string, error) {
	switch uriType {
	case "article":
		return fmt.Sprintf("https://%s/article/%s.html", Host, id), nil
	case "blog":
		return fmt.Sprintf("https://%s/%s", BlogHost, id), nil
	default:
		return "", errs.Usage("cto has no resource type %q", uriType)
	}
}

// --- helpers ---

// resolveArticleRef converts a user-supplied reference to a numeric article ID.
// Accepts: bare numeric ID, full 51cto.com article URL.
func resolveArticleRef(ref string) string {
	ref = strings.TrimSpace(ref)
	if articleIDRE.MatchString(ref) {
		return ref
	}
	if u, e := url.Parse(ref); e == nil && (u.Scheme == "http" || u.Scheme == "https") {
		return articleIDFromURL(u.Path)
	}
	return ""
}

// mapErr converts library errors to kit error kinds with the right exit codes.
func mapErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, ErrBlocked) {
		return errs.RateLimited("51CTO requires browser verification (HTTP 567 / EdgeOne JS challenge); try from a residential IP or pass --cookie")
	}
	if errors.Is(err, ErrNotFound) {
		return errs.NotFound("%s", err.Error())
	}
	return err
}
