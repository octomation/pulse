// Package trendshift reads the trending repositories of trendshift.io.
package trendshift

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"

	"go.octolab.org/toolset/pulse/internal/source/internal/page"
	"go.octolab.org/toolset/pulse/internal/trend"
)

const (
	host = "trendshift.io"
	base = "https://" + host
)

// The periods as the canonical link and the navigation of the page name
// them.
var (
	paths = map[string]trend.Period{
		"":         trend.Daily,
		"/weekly":  trend.Weekly,
		"/monthly": trend.Monthly,
	}
	labels = map[string]trend.Period{
		"Daily":   trend.Daily,
		"Weekly":  trend.Weekly,
		"Monthly": trend.Monthly,
	}
	card = regexp.MustCompile(`^/repositories/[0-9]+/?$`)
)

// Site is trendshift.io.
type Site struct{}

var _ trend.Site = Site{}

// New returns the site.
func New() Site { return Site{} }

// Source names the site.
func (Site) Source() trend.Source { return trend.Trendshift }

// Request returns the URL of the ranking for the period.
func (Site) Request(period trend.Period) (string, error) {
	switch period {
	case trend.Daily:
		return base + "/", nil
	case trend.Weekly, trend.Monthly:
		return base + "/" + string(period), nil
	default:
		return "", fmt.Errorf("trendshift has no %q list", period)
	}
}

// Parse reads a ranking page. The period comes from the page itself: from
// its canonical link and from the current item of its navigation, which
// have to agree and to name the period want.
//
// The list is the ranking only. The page states it twice: as cards, each
// with the badge of its position, and as a structured list (JSON-LD
// ItemList). Parse reads both and fails when they differ, so a block that
// is not the ranking (sponsored and featured repositories, live mentions)
// cannot get into the list and a ranked repository cannot drop out of it
// without an error.
func (Site) Parse(body []byte, want trend.Period) (trend.Page, error) {
	doc, err := page.Parse(body)
	if err != nil {
		return trend.Page{}, err
	}

	marker, period, err := shown(doc)
	if err != nil {
		return trend.Page{}, err
	}
	if err := page.Period(marker, period, want); err != nil {
		return trend.Page{}, err
	}

	cards, err := ranking(doc)
	if err != nil {
		return trend.Page{}, err
	}
	items, listed, err := itemList(doc)
	if err != nil {
		return trend.Page{}, err
	}
	if len(cards) == 0 && len(items) == 0 {
		return trend.Page{}, fmt.Errorf("the page has no repositories (%s)", marker)
	}
	if !listed {
		return trend.Page{}, fmt.Errorf(
			"the page shows %d ranked repositories but has no structured list to check them against",
			len(cards),
		)
	}
	if len(cards) != len(items) {
		return trend.Page{}, fmt.Errorf(
			"the page shows %d ranked repositories but its structured list has %d",
			len(cards), len(items),
		)
	}

	entries := make([]trend.Entry, 0, len(cards))
	for i, repo := range cards {
		item := items[i]
		if repo.Key() != item.repo.Key() {
			return trend.Page{}, fmt.Errorf(
				"position %d of the ranking is %s on the page but %s in its structured list",
				i+1, repo, item.repo,
			)
		}
		entries = append(entries, trend.Entry{
			Repo:        repo,
			Description: item.description,
			Language:    item.language,
		})
	}

	ranked, err := page.Rank(marker, entries)
	if err != nil {
		return trend.Page{}, err
	}
	return trend.Page{Marker: marker, Period: period, Entries: ranked}, nil
}

// shown returns what the page says about its period, such as
// "https://trendshift.io/weekly Weekly", and the period.
func shown(doc *html.Node) (string, trend.Period, error) {
	canonical, linked, err := canonical(doc)
	if err != nil {
		return "", "", err
	}
	label, current, err := current(doc)
	if err != nil {
		return "", "", err
	}
	if linked != current {
		return "", "", fmt.Errorf(
			"the page names two periods: %s in its canonical link %s and %s as %q in its navigation",
			linked, canonical, current, label,
		)
	}
	return canonical + " " + label, linked, nil
}

// canonical returns the canonical link of the page and its period.
func canonical(doc *html.Node) (string, trend.Period, error) {
	href := ""
	for _, link := range page.Find(doc, func(n *html.Node) bool { return n.DataAtom == atom.Link }) {
		rel, _ := page.Attr(link, "rel")
		if !slices.Contains(strings.Fields(strings.ToLower(rel)), "canonical") {
			continue
		}
		value, _ := page.Attr(link, "href")
		value = strings.TrimSpace(value)
		if href != "" && href != value {
			return "", "", fmt.Errorf("the page names two periods: canonical links %s and %s", href, value)
		}
		href = value
	}
	if href == "" {
		return "", "", errors.New("the page does not say which period it shows: it has no canonical link")
	}

	address, err := url.Parse(href)
	if err != nil || !strings.EqualFold(address.Hostname(), host) {
		return "", "", fmt.Errorf("the canonical link of the page is not on %s: %q", host, href)
	}
	period, known := paths[strings.TrimRight(address.Path, "/")]
	if !known {
		return "", "", fmt.Errorf("the page shows a period pulse does not watch: canonical link %s", href)
	}
	return href, period, nil
}

// current returns the label of the current item of the period navigation,
// "Daily", "Weekly" or "Monthly", and its period. The page repeats the
// navigation, so every copy has to name the same period.
func current(doc *html.Node) (string, trend.Period, error) {
	label := ""
	for _, link := range page.Find(doc, func(n *html.Node) bool { return n.DataAtom == atom.A }) {
		if value, _ := page.Attr(link, "aria-current"); value != "page" {
			continue
		}
		text := page.Text(link)
		if _, known := labels[text]; !known {
			continue
		}
		if label != "" && label != text {
			return "", "", fmt.Errorf("the page names two periods in its navigation: %s and %s", label, text)
		}
		label = text
	}
	if label == "" {
		return "", "", errors.New(
			"the page does not say which period it shows: its navigation has no current Daily, Weekly or Monthly item",
		)
	}
	return label, labels[label], nil
}

// ranking returns the repositories of the ranking cards in the order of the
// page. A card is a link to a repository page of the site that stands right
// after the badge of its position: a medal for the first three, a number
// for the rest. Links to repositories without a badge belong to other
// blocks of the page. The badges have to count 1, 2, 3 and so on.
func ranking(doc *html.Node) ([]trend.Repo, error) {
	var repos []trend.Repo
	for _, link := range page.Find(doc, func(n *html.Node) bool { return n.DataAtom == atom.A }) {
		href, _ := page.Attr(link, "href")
		if !card.MatchString(href) {
			continue
		}
		position, ranked := badge(link)
		if !ranked {
			continue
		}
		if position != len(repos)+1 {
			return nil, fmt.Errorf(
				"the ranking of the page is out of order: position %d has the badge %d",
				len(repos)+1, position,
			)
		}
		name := page.OwnText(link)
		repo, err := trend.ParseRepo(name)
		if err != nil {
			return nil, fmt.Errorf("position %d of the ranking is not a repository: %q", position, name)
		}
		repos = append(repos, repo)
	}
	return repos, nil
}

// badge returns the position the element before the link shows: a number
// as text, or the medal image named "#1", "#2" or "#3".
func badge(link *html.Node) (int, bool) {
	before := link.PrevSibling
	for before != nil && before.Type != html.ElementNode {
		before = before.PrevSibling
	}
	if before == nil {
		return 0, false
	}
	text := page.Text(before)
	if text == "" {
		medal := page.First(before, func(n *html.Node) bool { return n.DataAtom == atom.Img })
		if medal == nil {
			return 0, false
		}
		alt, _ := page.Attr(medal, "alt")
		text = strings.TrimPrefix(strings.TrimSpace(alt), "#")
	}
	position, err := strconv.Atoi(text)
	if err != nil || position < 1 {
		return 0, false
	}
	return position, true
}

// item is a repository of the structured list.
type item struct {
	repo        trend.Repo
	description string
	language    string
}

// itemList returns the repositories of the JSON-LD ItemList of the page in
// its order, and whether the page has such a list.
func itemList(doc *html.Node) ([]item, bool, error) {
	type list struct {
		Type     string `json:"@type"`
		Number   *int   `json:"numberOfItems"`
		Elements []struct {
			Position int `json:"position"`
			Item     struct {
				Name        string `json:"name"`
				Description string `json:"description"`
				Repository  string `json:"codeRepository"`
				Language    string `json:"programmingLanguage"`
			} `json:"item"`
		} `json:"itemListElement"`
	}

	var lists []list
	for _, script := range page.Find(doc, func(n *html.Node) bool { return n.DataAtom == atom.Script }) {
		if kind, _ := page.Attr(script, "type"); !strings.EqualFold(strings.TrimSpace(kind), "application/ld+json") {
			continue
		}
		var data strings.Builder
		for child := range script.ChildNodes() {
			if child.Type == html.TextNode {
				data.WriteString(child.Data)
			}
		}
		var candidate list
		if err := json.Unmarshal([]byte(data.String()), &candidate); err != nil || candidate.Type != "ItemList" {
			continue
		}
		lists = append(lists, candidate)
	}
	switch len(lists) {
	case 0:
		return nil, false, nil
	case 1:
	default:
		return nil, true, fmt.Errorf("the page has %d structured lists, not one", len(lists))
	}

	ranking := lists[0]
	if ranking.Number != nil && *ranking.Number != len(ranking.Elements) {
		return nil, true, fmt.Errorf(
			"the structured list of the page says it has %d repositories but holds %d",
			*ranking.Number, len(ranking.Elements),
		)
	}
	items := make([]item, 0, len(ranking.Elements))
	for i, element := range ranking.Elements {
		if element.Position != i+1 {
			return nil, true, fmt.Errorf(
				"the structured list of the page is out of order: position %d is numbered %d",
				i+1, element.Position,
			)
		}
		repo, err := trend.ParseRepo(element.Item.Name)
		if err != nil {
			repo, err = trend.ParseRepo(element.Item.Repository)
		}
		if err != nil {
			return nil, true, fmt.Errorf(
				"position %d of the structured list of the page is not a repository: %q",
				i+1, element.Item.Name,
			)
		}
		items = append(items, item{
			repo:        repo,
			description: strings.Join(strings.Fields(element.Item.Description), " "),
			language:    strings.TrimSpace(element.Item.Language),
		})
	}
	return items, true, nil
}
