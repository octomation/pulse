// Package github reads the trending repositories of github.com.
package github

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"

	"go.octolab.org/toolset/pulse/internal/source/internal/page"
	"go.octolab.org/toolset/pulse/internal/trend"
)

const (
	base = "https://github.com/trending"
	menu = "Date range:"
)

// The periods as the "Date range" menu and the rows of the page name them.
var (
	ranges = map[string]trend.Period{
		"Today":      trend.Daily,
		"This week":  trend.Weekly,
		"This month": trend.Monthly,
	}
	gains = map[string]trend.Period{
		"today":      trend.Daily,
		"this week":  trend.Weekly,
		"this month": trend.Monthly,
	}
	gain = regexp.MustCompile(`^[0-9][0-9,.]* stars? (today|this week|this month)$`)
)

// Site is github.com/trending.
type Site struct{}

var _ trend.Site = Site{}

// New returns the site.
func New() Site { return Site{} }

// Source names the site.
func (Site) Source() trend.Source { return trend.GitHub }

// Request returns the URL of the trending page for the period.
func (Site) Request(period trend.Period) (string, error) {
	switch period {
	case trend.Daily, trend.Weekly, trend.Monthly:
		return base + "?since=" + string(period), nil
	default:
		return "", fmt.Errorf("github has no %q list", period)
	}
}

// Parse reads a trending page. The period comes from the page itself: from
// the selected item of its "Date range" menu and from what the rows count
// ("stars this week"). Both have to be there and to name the period want.
func (Site) Parse(body []byte, want trend.Period) (trend.Page, error) {
	doc, err := page.Parse(body)
	if err != nil {
		return trend.Page{}, err
	}

	selected, period, err := dateRange(doc)
	if err != nil {
		return trend.Page{}, err
	}

	if err := page.Period(selected, period, want); err != nil {
		return trend.Page{}, err
	}

	rows := page.Find(doc, func(n *html.Node) bool {
		return n.DataAtom == atom.Article && page.HasClass(n, "Box-row")
	})
	if len(rows) == 0 {
		return trend.Page{}, fmt.Errorf("the page has no repositories (%s)", selected)
	}

	entries := make([]trend.Entry, 0, len(rows))
	counted := ""
	for i, row := range rows {
		entry, err := read(row)
		if err != nil {
			return trend.Page{}, fmt.Errorf("row %d of the page: %w", i+1, err)
		}
		entries = append(entries, entry)

		said, found := gained(row)
		if !found {
			continue
		}
		if gains[said] != period {
			return trend.Page{}, fmt.Errorf(
				"the page names two periods: %q in its menu and \"stars %s\" for %s",
				selected, said, entry.Repo,
			)
		}
		counted = said
	}
	if counted == "" {
		return trend.Page{}, fmt.Errorf(
			"the rows of the page do not say which period their stars are of (%s)", selected,
		)
	}

	marker := selected + ", stars " + counted
	ranked, err := page.Rank(marker, entries)
	if err != nil {
		return trend.Page{}, err
	}
	return trend.Page{Marker: marker, Period: period, Entries: ranked}, nil
}

// dateRange returns the selected item of the "Date range" menu as the page
// wrote it, such as "Date range: This week", and its period.
func dateRange(doc *html.Node) (string, trend.Period, error) {
	selected := ""
	for _, summary := range page.Find(doc, func(n *html.Node) bool { return n.DataAtom == atom.Summary }) {
		text := page.Text(summary)
		if !strings.HasPrefix(text, menu) {
			continue
		}
		if selected != "" && selected != text {
			return "", "", fmt.Errorf("the page names two periods: %q and %q", selected, text)
		}
		selected = text
	}
	if selected == "" {
		return "", "", errors.New(`the page does not say which period it shows: it has no "Date range" menu`)
	}
	period, known := ranges[strings.TrimSpace(strings.TrimPrefix(selected, menu))]
	if !known {
		return "", "", fmt.Errorf("the page shows a period pulse does not watch: %q", selected)
	}
	return selected, period, nil
}

// read returns the repository of a row with its optional description and
// language.
func read(row *html.Node) (trend.Entry, error) {
	title := page.First(row, func(n *html.Node) bool { return n.DataAtom == atom.H2 })
	if title == nil {
		return trend.Entry{}, errors.New("no repository title")
	}
	link := page.First(title, func(n *html.Node) bool { return n.DataAtom == atom.A })
	if link == nil {
		return trend.Entry{}, errors.New("no link to the repository")
	}
	href, _ := page.Attr(link, "href")
	repo, err := trend.ParseRepo(strings.Trim(href, "/"))
	if err != nil {
		return trend.Entry{}, fmt.Errorf("the link %q is not a repository", href)
	}

	entry := trend.Entry{Repo: repo}
	for child := range row.ChildNodes() {
		if child.Type == html.ElementNode && child.DataAtom == atom.P {
			entry.Description = page.Text(child)
			break
		}
	}
	language := page.First(row, func(n *html.Node) bool {
		value, _ := page.Attr(n, "itemprop")
		return value == "programmingLanguage"
	})
	if language != nil {
		entry.Language = page.Text(language)
	}
	return entry, nil
}

// gained returns the period a row counts its new stars for: "today",
// "this week" or "this month". The title and the description of the
// repository are not looked at: only the counters of the row.
func gained(row *html.Node) (string, bool) {
	var said string
	var walk func(*html.Node) bool
	walk = func(n *html.Node) bool {
		for child := range n.ChildNodes() {
			switch {
			case child.Type == html.TextNode:
				text := strings.Join(strings.Fields(child.Data), " ")
				if match := gain.FindStringSubmatch(text); match != nil {
					said = match[1]
					return true
				}
			case child.Type != html.ElementNode,
				child.DataAtom == atom.H2,
				child.DataAtom == atom.P && child.Parent == row:
				continue
			default:
				if walk(child) {
					return true
				}
			}
		}
		return false
	}
	return said, walk(row)
}
