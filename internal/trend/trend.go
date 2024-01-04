// Package trend holds the vocabulary of trend-watch: the lists pulse watches,
// the repositories they rank and the snapshots of their loads.
package trend

import (
	"fmt"
	"net/url"
	"strings"
	"time"
)

// Source is a site that ranks trending repositories.
type Source string

// The sources pulse watches.
const (
	Trendshift Source = "trendshift"
	GitHub     Source = "github"
)

// Period is the window a ranking covers.
type Period string

// The periods of every source.
const (
	Daily   Period = "daily"
	Weekly  Period = "weekly"
	Monthly Period = "monthly"
)

// List is one ranking: a source and a period.
type List struct {
	Source Source `json:"source"`
	Period Period `json:"period"`
}

// String returns the name of the list, such as "trendshift/daily".
func (l List) String() string { return string(l.Source) + "/" + string(l.Period) }

// Lists returns the six lists in the order the page shows them.
func Lists() []List {
	return []List{
		{Trendshift, Daily}, {Trendshift, Weekly}, {Trendshift, Monthly},
		{GitHub, Daily}, {GitHub, Weekly}, {GitHub, Monthly},
	}
}

// Repo is a GitHub repository as a list wrote it.
type Repo struct {
	Owner string `json:"owner"`
	Name  string `json:"name"`
}

// Key identifies the repository: owner/name without case.
func (r Repo) Key() string { return strings.ToLower(r.Owner + "/" + r.Name) }

// String returns owner/name in the case of the list.
func (r Repo) String() string { return r.Owner + "/" + r.Name }

// URL returns the address of the repository on GitHub.
func (r Repo) URL() string { return "https://github.com/" + r.Owner + "/" + r.Name }

// ParseRepo reads owner/name or a URL of a repository on github.com. A URL of
// an owner, of another host or with a deeper path is not a repository.
func ParseRepo(s string) (Repo, error) {
	s = strings.TrimSpace(s)
	if strings.Contains(s, "://") {
		u, err := url.Parse(s)
		if err != nil {
			return Repo{}, fmt.Errorf("repository %q: %w", s, err)
		}
		if host := strings.ToLower(u.Hostname()); host != "github.com" && host != "www.github.com" {
			return Repo{}, fmt.Errorf("repository %q: not on github.com", s)
		}
		s = strings.TrimSuffix(strings.Trim(u.Path, "/"), ".git")
	}
	owner, name, found := strings.Cut(s, "/")
	if !found || owner == "" || name == "" || strings.ContainsAny(s, " \t\n") || strings.Contains(name, "/") {
		return Repo{}, fmt.Errorf("repository %q: want owner/name", s)
	}
	return Repo{Owner: owner, Name: name}, nil
}

// Entry is a repository at its position in a list.
type Entry struct {
	Rank        int    `json:"rank"`
	Repo        Repo   `json:"repo"`
	Description string `json:"description,omitempty"`
	Language    string `json:"language,omitempty"`
}

// Page is what a parser reads from the answer of a source.
type Page struct {
	// Marker is the period as the source named it in the answer, such as
	// "Date range: This week, stars this week" or
	// "https://trendshift.io/weekly Weekly".
	Marker string
	// Period is the period the marker stands for.
	Period Period
	// Entries are the repositories in the order of the list.
	Entries []Entry
}

// Snapshot is one successful load of a list.
type Snapshot struct {
	List      List      `json:"list"`
	Request   string    `json:"request"`
	Marker    string    `json:"marker"`
	FetchedAt time.Time `json:"fetched_at"`
	Entries   []Entry   `json:"entries"`
}

// Site knows how to ask a source for a period and how to read its answer.
type Site interface {
	// Source names the site.
	Source() Source
	// Request returns the URL of the list for the period.
	Request(Period) (string, error)
	// Parse reads the answer. It fails when the answer does not name its
	// period, names another one than want, or holds no repository.
	Parse(body []byte, want Period) (Page, error)
}
