// Package details asks GitHub what it knows about a repository: its facts
// and its README, for the reader of trend-watch.
package details

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"

	"go.octolab.org/toolset/pulse/internal/trend"
)

const (
	// DefaultBaseURL is the GitHub API.
	DefaultBaseURL = "https://api.github.com"
	// DefaultTTL is how long an answer is kept: the README of GitHub links
	// images by addresses that expire in minutes.
	DefaultTTL = 5 * time.Minute
	// DefaultTimeout bounds the requests for one repository.
	DefaultTimeout = 20 * time.Second

	maxBody  = 4 << 20
	maxCache = 256
)

// The errors of GitHub the reader tells apart.
var (
	// ErrNotFound says that GitHub has no such repository, or hides it.
	ErrNotFound = errors.New("GitHub has no such repository")
	// ErrRateLimited says that GitHub takes no more requests for now.
	ErrRateLimited = errors.New("GitHub takes no more requests for now")
)

// Details is what GitHub knows about a repository.
type Details struct {
	Repo        trend.Repo
	Description string
	Homepage    string
	Language    string
	License     string
	Topics      []string
	Stars       int
	Forks       int
	OpenIssues  int
	Archived    bool
	CreatedAt   time.Time
	PushedAt    time.Time
	// Readme is the README as GitHub renders it, cleaned: no scripts, and
	// its links and images are absolute. It is empty without a README.
	Readme string
	// FetchedAt is the time of the answer.
	FetchedAt time.Time
}

// Options wire a Client.
type Options struct {
	// Token is sent to the API as a bearer token; empty asks anonymously.
	Token string
	// BaseURL is the API; empty is DefaultBaseURL.
	BaseURL string
	// HTTP is the client; nil is one with DefaultTimeout.
	HTTP *http.Client
	// Now is the clock; nil is time.Now.
	Now func() time.Time
	// TTL is how long an answer is kept; zero is DefaultTTL.
	TTL time.Duration
}

// Client asks GitHub and keeps the answers for a while.
type Client struct {
	token string
	base  string
	http  *http.Client
	now   func() time.Time
	ttl   time.Duration

	mu    sync.Mutex
	cache map[string]Details
}

// New returns the client.
func New(o Options) *Client {
	c := &Client{token: o.Token, base: strings.TrimSuffix(o.BaseURL, "/"), http: o.HTTP, now: o.Now, ttl: o.TTL,
		cache: map[string]Details{}}
	if c.base == "" {
		c.base = DefaultBaseURL
	}
	if c.http == nil {
		c.http = &http.Client{Timeout: DefaultTimeout}
	}
	if c.now == nil {
		c.now = time.Now
	}
	if c.ttl <= 0 {
		c.ttl = DefaultTTL
	}
	return c
}

// Details returns what GitHub knows about the repository, from the cache
// when the answer is fresh. A repository without a README is not an error.
func (c *Client) Details(ctx context.Context, repo trend.Repo) (Details, error) {
	key := repo.Key()
	c.mu.Lock()
	cached, found := c.cache[key]
	c.mu.Unlock()
	if found && c.now().Sub(cached.FetchedAt) < c.ttl {
		return cached, nil
	}

	path := "/repos/" + url.PathEscape(repo.Owner) + "/" + url.PathEscape(repo.Name)
	var facts struct {
		Name  string `json:"name"`
		Owner struct {
			Login string `json:"login"`
		} `json:"owner"`
		Description *string `json:"description"`
		Homepage    *string `json:"homepage"`
		Language    *string `json:"language"`
		License     *struct {
			SPDX string `json:"spdx_id"`
			Name string `json:"name"`
		} `json:"license"`
		Topics     []string  `json:"topics"`
		Stars      int       `json:"stargazers_count"`
		Forks      int       `json:"forks_count"`
		OpenIssues int       `json:"open_issues_count"`
		Archived   bool      `json:"archived"`
		CreatedAt  time.Time `json:"created_at"`
		PushedAt   time.Time `json:"pushed_at"`
	}
	body, err := c.get(ctx, path, "application/vnd.github+json")
	if err != nil {
		return Details{}, err
	}
	if err := json.Unmarshal(body, &facts); err != nil {
		return Details{}, fmt.Errorf("the answer of GitHub about %s is not what it documents: %w", repo, err)
	}
	d := Details{
		Repo: repo, Topics: facts.Topics, Stars: facts.Stars, Forks: facts.Forks, OpenIssues: facts.OpenIssues,
		Archived: facts.Archived, CreatedAt: facts.CreatedAt, PushedAt: facts.PushedAt, FetchedAt: c.now(),
	}
	if facts.Owner.Login != "" && facts.Name != "" {
		d.Repo = trend.Repo{Owner: facts.Owner.Login, Name: facts.Name}
	}
	if facts.Description != nil {
		d.Description = strings.TrimSpace(*facts.Description)
	}
	if facts.Homepage != nil {
		if u, err := url.Parse(strings.TrimSpace(*facts.Homepage)); err == nil && (u.Scheme == "https" || u.Scheme == "http") && u.Host != "" {
			d.Homepage = u.String()
		}
	}
	if facts.Language != nil {
		d.Language = *facts.Language
	}
	if facts.License != nil {
		d.License = facts.License.SPDX
		if d.License == "" || d.License == "NOASSERTION" {
			d.License = facts.License.Name
		}
	}

	switch readme, err := c.get(ctx, path+"/readme", "application/vnd.github.html+json"); {
	case err == nil:
		d.Readme = Clean(readme, d.Repo)
	case errors.Is(err, ErrNotFound):
	default:
		return Details{}, err
	}

	c.mu.Lock()
	if len(c.cache) >= maxCache {
		clear(c.cache)
	}
	c.cache[key] = d
	c.mu.Unlock()
	return d, nil
}

func (c *Client) get(ctx context.Context, path, accept string) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+path, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Accept", accept)
	request.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	request.Header.Set("User-Agent", "pulse-trend-watch")
	if c.token != "" {
		request.Header.Set("Authorization", "Bearer "+c.token)
	}
	response, err := c.http.Do(request)
	if err != nil {
		var failure *url.Error
		if errors.As(err, &failure) {
			// Without the address: it says nothing the reader needs.
			err = failure.Err
		}
		return nil, fmt.Errorf("GitHub does not answer: %w", err)
	}
	defer func() { _ = response.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(response.Body, maxBody+1))
	if err != nil {
		return nil, fmt.Errorf("GitHub does not answer: %w", err)
	}
	switch {
	case response.StatusCode == http.StatusOK && len(body) > maxBody:
		return nil, fmt.Errorf("the answer of GitHub is larger than %d MiB", maxBody>>20)
	case response.StatusCode == http.StatusOK:
		return body, nil
	case response.StatusCode == http.StatusNotFound:
		return nil, ErrNotFound
	case response.StatusCode == http.StatusTooManyRequests,
		response.StatusCode == http.StatusForbidden && response.Header.Get("X-RateLimit-Remaining") == "0":
		if c.token == "" {
			return nil, fmt.Errorf("%w: without a token it allows 60 an hour; start pulse with GITHUB_TOKEN", ErrRateLimited)
		}
		return nil, ErrRateLimited
	case response.StatusCode == http.StatusUnauthorized:
		return nil, errors.New("GitHub does not accept the token of GITHUB_TOKEN")
	default:
		return nil, fmt.Errorf("GitHub answered %s", response.Status)
	}
}

// dropped are the elements a README is shown without, with all they hold.
var dropped = map[atom.Atom]bool{
	atom.Script: true, atom.Style: true, atom.Iframe: true, atom.Object: true, atom.Embed: true,
	atom.Form: true, atom.Link: true, atom.Meta: true, atom.Base: true, atom.Noscript: true,
	atom.Template: true, atom.Frame: true, atom.Frameset: true, atom.Applet: true,
}

// Clean prepares a rendered README for the reader: it drops the elements
// and the attributes that run code, points relative links to the repository
// on github.com, and makes every link open outside the frame.
func Clean(rendered []byte, repo trend.Repo) string {
	root := &html.Node{Type: html.ElementNode, Data: "div", DataAtom: atom.Div}
	nodes, err := html.ParseFragment(bytes.NewReader(rendered), root)
	if err != nil {
		return ""
	}
	pages, _ := url.Parse(repo.URL() + "/blob/HEAD/")
	files, _ := url.Parse(repo.URL() + "/raw/HEAD/")

	var clean func(n *html.Node)
	clean = func(n *html.Node) {
		for child := n.FirstChild; child != nil; {
			next := child.NextSibling
			if child.Type == html.CommentNode || child.Type == html.ElementNode && dropped[child.DataAtom] {
				n.RemoveChild(child)
			} else {
				clean(child)
			}
			child = next
		}
		if n.Type != html.ElementNode {
			return
		}
		var attrs []html.Attribute
		for _, attr := range n.Attr {
			name := strings.ToLower(attr.Key)
			switch {
			case strings.HasPrefix(name, "on"), name == "style", name == "srcdoc", name == "target", name == "rel",
				name == "srcset", name == "formaction", name == "ping":
				continue
			case name == "href":
				if attr.Val = address(attr.Val, pages); attr.Val == "" {
					continue
				}
			case name == "src", name == "poster":
				if attr.Val = address(attr.Val, files); attr.Val == "" {
					continue
				}
			}
			attrs = append(attrs, attr)
		}
		if n.DataAtom == atom.A {
			attrs = append(attrs, html.Attribute{Key: "target", Val: "_blank"},
				html.Attribute{Key: "rel", Val: "noopener noreferrer nofollow"})
		}
		n.Attr = attrs
	}

	var out strings.Builder
	for _, node := range nodes {
		holder := &html.Node{Type: html.ElementNode, Data: "div", DataAtom: atom.Div}
		holder.AppendChild(node)
		clean(holder)
		for child := holder.FirstChild; child != nil; child = child.NextSibling {
			if err := html.Render(&out, child); err != nil {
				return ""
			}
		}
	}
	return strings.TrimSpace(out.String())
}

// address resolves a link of a README against base and keeps it only when
// it is an http(s) address; a link inside the document becomes a link to
// the README on github.com.
func address(value string, base *url.URL) string {
	u, err := url.Parse(strings.TrimSpace(value))
	if err != nil {
		return ""
	}
	if u.Scheme == "" && u.Host == "" && u.Path == "" {
		// "#section": the frame cannot follow it.
		readme := *base
		readme.Path = strings.TrimSuffix(strings.TrimSuffix(base.Path, "/blob/HEAD/"), "/raw/HEAD/")
		readme.Fragment = strings.TrimPrefix(u.Fragment, "user-content-")
		return readme.String()
	}
	u = base.ResolveReference(u)
	if u.Scheme != "https" && u.Scheme != "http" {
		return ""
	}
	return u.String()
}
