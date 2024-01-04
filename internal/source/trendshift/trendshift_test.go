package trendshift_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.octolab.org/toolset/pulse/internal/source/internal/fixture"
	. "go.octolab.org/toolset/pulse/internal/source/trendshift"
	"go.octolab.org/toolset/pulse/internal/trend"
)

const testdata = "../testdata"

var periods = []trend.Period{trend.Daily, trend.Weekly, trend.Monthly}

// What the saved pages show next to the ranking: the sponsored and featured
// repositories, and the repositories of the "Live mentions" block.
var (
	sponsored = []string{
		"GetBusbar/busbar", "maximhq/bifrost", "ModernRelay/omnigraph", "OfinTech/ofinTech-Dask",
	}
	mentioned = []string{
		"inworld-ai/sava", "inworld-ai/voice-isolator", "openstyles/stylus",
		"TensorTonic/llm-optimization-handbook", "tulios/json-viewer",
	}
)

func TestSite_Source(t *testing.T) {
	assert.Equal(t, trend.Trendshift, New().Source())
}

func TestSite_Request(t *testing.T) {
	tests := map[trend.Period]string{
		trend.Daily:   "https://trendshift.io/",
		trend.Weekly:  "https://trendshift.io/weekly",
		trend.Monthly: "https://trendshift.io/monthly",
	}
	for period, want := range tests {
		t.Run(string(period), func(t *testing.T) {
			request, err := New().Request(period)
			require.NoError(t, err)
			assert.Equal(t, want, request)
		})
	}

	t.Run("unknown period", func(t *testing.T) {
		request, err := New().Request("yearly")
		require.ErrorContains(t, err, `no "yearly" list`)
		assert.Empty(t, request)
	})
}

func TestSite_Parse_fixtures(t *testing.T) {
	for _, period := range periods {
		t.Run(string(period), func(t *testing.T) {
			saved := fixture.Load(t, testdata, trend.List{Source: trend.Trendshift, Period: period})
			request, err := New().Request(period)
			require.NoError(t, err)
			assert.Equal(t, saved.Request, request)

			page, err := New().Parse(saved.Body, period)
			require.NoError(t, err)

			assert.Equal(t, saved.Marker, page.Marker)
			assert.Equal(t, period, page.Period)
			assert.Equal(t, saved.Repos, fixture.Names(page.Entries))

			seen := make(map[string]int)
			for i, entry := range page.Entries {
				assert.Equal(t, i+1, entry.Rank)
				assert.Zero(t, seen[entry.Repo.Key()], "%s is in the list twice", entry.Repo)
				seen[entry.Repo.Key()] = entry.Rank
			}

			for _, outside := range append(append([]string{}, sponsored...), mentioned...) {
				require.Contains(t, string(saved.Body), outside, "the saved page no longer shows it")
				assert.NotContains(t, seen, strings.ToLower(outside), "it is not a part of the ranking")
			}
		})
	}
}

func TestSite_Parse_extras(t *testing.T) {
	saved := fixture.Load(t, testdata, trend.List{Source: trend.Trendshift, Period: trend.Weekly})

	page, err := New().Parse(saved.Body, trend.Weekly)
	require.NoError(t, err)
	require.NotEmpty(t, page.Entries)

	first := page.Entries[0]
	assert.Equal(t, trend.Repo{Owner: "BaaRai", Name: "ARTEX"}, first.Repo)
	assert.True(t, strings.HasPrefix(first.Description, "ARTEX - AI autonomous penetration testing"), first.Description)
	assert.Equal(t, "Go", first.Language)
}

func TestSite_Parse_anotherPeriod(t *testing.T) {
	for _, period := range periods {
		saved := fixture.Load(t, testdata, trend.List{Source: trend.Trendshift, Period: period})
		for _, want := range periods {
			if want == period {
				continue
			}
			t.Run(fmt.Sprintf("%s as %s", period, want), func(t *testing.T) {
				page, err := New().Parse(saved.Body, want)
				require.Error(t, err)
				assert.ErrorContains(t, err, fmt.Sprintf("the page shows the %s list", period))
				assert.ErrorContains(t, err, fmt.Sprintf("not the %s one", want))
				assert.Zero(t, page)
			})
		}
	}
}

func TestSite_Parse_periodsDiffer(t *testing.T) {
	rankings := make(map[trend.Period][]string, len(periods))
	for _, period := range periods {
		saved := fixture.Load(t, testdata, trend.List{Source: trend.Trendshift, Period: period})
		page, err := New().Parse(saved.Body, period)
		require.NoError(t, err)
		rankings[period] = fixture.Keys(page.Entries)
	}
	for i, one := range periods {
		for _, other := range periods[i+1:] {
			assert.NotEqual(t, rankings[one], rankings[other],
				"%s and %s hold the same ranking: the same list under two labels", one, other)
		}
	}
}

func TestSite_Parse_fixtureChanged(t *testing.T) {
	saved := fixture.Load(t, testdata, trend.List{Source: trend.Trendshift, Period: trend.Weekly})

	tests := map[string]struct {
		old, new string
		want     string
	}{
		"no canonical link": {
			old:  `rel="canonical"`,
			new:  `rel="alternate"`,
			want: "it has no canonical link",
		},
		"no current item in the navigation": {
			old:  ` aria-current="page"`,
			new:  ``,
			want: "its navigation has no current Daily, Weekly or Monthly item",
		},
		"canonical link of another period": {
			old:  `<link rel="canonical" href="https://trendshift.io/weekly"/>`,
			new:  `<link rel="canonical" href="https://trendshift.io/monthly"/>`,
			want: "the page names two periods",
		},
		"no structured list": {
			old:  `<script type="application/ld+json">`,
			new:  `<script type="application/json">`,
			want: "the page shows 25 ranked repositories but has no structured list to check them against",
		},
		"a card the structured list does not have": {
			old:  `"name":"BaaRai/ARTEX"`,
			new:  `"name":"BaaRai/other"`,
			want: "position 1 of the ranking is BaaRai/ARTEX on the page but BaaRai/other in its structured list",
		},
		"a card without its badge": {
			old:  `shrink-0 text-base">5</div>`,
			new:  `shrink-0 text-base"></div>`,
			want: "the ranking of the page is out of order: position 5 has the badge 6",
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			body := bytes.ReplaceAll(saved.Body, []byte(test.old), []byte(test.new))
			require.NotEqual(t, saved.Body, body, "the saved page has no %s", test.old)

			page, err := New().Parse(body, trend.Weekly)
			require.ErrorContains(t, err, test.want)
			assert.Zero(t, page)
		})
	}
}

// layout describes a page of the site for document.
type layout struct {
	canonical []string // canonical links
	current   []string // the current item of every copy of the navigation
	cards     []card   // the ranking as cards
	lists     []string // the JSON-LD scripts, as they are
}

type card struct {
	name  string
	badge string // the position by default
}

type listed struct {
	name        string
	repository  string
	description string
	language    string
	position    int // the place in the list by default
}

func itemList(number int, items ...listed) string {
	type item struct {
		Type        string `json:"@type"`
		Name        string `json:"name,omitempty"`
		Description string `json:"description,omitempty"`
		Repository  string `json:"codeRepository,omitempty"`
		Language    string `json:"programmingLanguage,omitempty"`
	}
	type element struct {
		Type     string `json:"@type"`
		Position int    `json:"position"`
		Item     item   `json:"item"`
	}
	list := struct {
		Type     string    `json:"@type"`
		Number   int       `json:"numberOfItems"`
		Elements []element `json:"itemListElement"`
	}{Type: "ItemList", Number: number, Elements: []element{}}
	for i, entry := range items {
		position := entry.position
		if position == 0 {
			position = i + 1
		}
		list.Elements = append(list.Elements, element{
			Type:     "ListItem",
			Position: position,
			Item: item{
				Type:        "SoftwareSourceCode",
				Name:        entry.name,
				Description: entry.description,
				Repository:  entry.repository,
				Language:    entry.language,
			},
		})
	}
	data, err := json.Marshal(list)
	if err != nil {
		panic(err)
	}
	return string(data)
}

func names(repos ...string) []listed {
	items := make([]listed, 0, len(repos))
	for _, repo := range repos {
		items = append(items, listed{name: repo})
	}
	return items
}

func cards(repos ...string) []card {
	ranking := make([]card, 0, len(repos))
	for _, repo := range repos {
		ranking = append(ranking, card{name: repo})
	}
	return ranking
}

func navigation(label, current string) string {
	var b strings.Builder
	b.WriteString(`<nav aria-label="` + label + `">`)
	for _, item := range []struct{ href, text string }{
		{"/", "Daily"}, {"/weekly", "Weekly"}, {"/monthly", "Monthly"}, {"/yearly", "Yearly"},
	} {
		b.WriteString("<a ")
		if item.text == current {
			b.WriteString(`aria-current="page" `)
		}
		b.WriteString(`class="inline-flex" href="` + item.href + `">` + item.text + "</a>")
	}
	b.WriteString(`<a aria-current="page" href="/github-trending-repositories">Repositories</a></nav>`)
	return b.String()
}

// document builds a page in the shape of the site: the sponsored links, the
// navigation, the ranking with a featured card in it, and the live mentions.
func document(page layout) []byte {
	var b strings.Builder
	b.WriteString(`<!DOCTYPE html><html lang="en"><head><meta charSet="utf-8"/>`)
	for _, href := range page.canonical {
		b.WriteString(`<link rel="canonical" href="` + href + `"/>`)
	}
	b.WriteString(`</head><body><main>`)
	b.WriteString(`<a href="https://github.com/ads/sponsor?utm_source=trendshift.io" rel="sponsored">Sponsor</a>`)
	for i, current := range page.current {
		b.WriteString(navigation("Site "+strconv.Itoa(i+1), current))
	}
	b.WriteString(`</main><div hidden id="S:2">`)
	for _, list := range page.lists {
		b.WriteString(`<script type="application/ld+json">` + list + `</script>`)
	}
	b.WriteString(`<div class="divide-y">`)
	for i, entry := range page.cards {
		position := strconv.Itoa(i + 1)
		id := strconv.Itoa(1000 + i)
		badge := entry.badge
		if badge == "" {
			badge = position
		}
		b.WriteString(`<div class="group"><div class="flex"><div class="flex min-w-0">`)
		if i < 3 && entry.badge == "" {
			b.WriteString(`<div class="flex w-6"><img alt="#` + badge + `" width="29" height="46" src="data:image/svg+xml,%3Csvg%3E` +
				badge + `%3C%2Fsvg%3E"/></div>`)
		} else {
			b.WriteString(`<div class="tabular-nums">` + badge + `</div>`)
		}
		b.WriteString(`<a class="text-foreground" href="/repositories/` + id + `">` + entry.name +
			`<span class="relative"><span aria-hidden="true"><svg></svg></span><span class="sr-only">Trending</span></span></a>`)
		b.WriteString(`</div><div><span>7.3k</span><span>Gained this week</span></div></div>`)
		b.WriteString(`<div><span>Mentioned on</span><a aria-label="Mentioned on X" href="/repositories/` + id + `"><svg></svg></a></div>`)
		b.WriteString(`<p>Card ` + position + `.</p></div>`)
		if i == 0 {
			b.WriteString(`<div class="group"><a href="https://github.com/ads/featured?utm_source=trendshift.io" rel="sponsored">` +
				`<div><img alt="" src="/sponsors/featured.png"/></div><span>Featured</span><span>Featured</span></a></div>`)
		}
	}
	b.WriteString(`</div></div><div hidden id="S:3"><ul>`)
	for i, repo := range []string{"live/mention", "live/other"} {
		b.WriteString(`<li><p><a class="font-semibold" href="/repositories/` + strconv.Itoa(9000+i) + `">` + repo + `</a></p></li>`)
	}
	b.WriteString(`</ul></div></body></html>`)
	return []byte(b.String())
}

func weekly(ranking []card, lists ...string) layout {
	return layout{
		canonical: []string{"https://trendshift.io/weekly"},
		current:   []string{"Weekly", "Weekly"},
		cards:     ranking,
		lists:     lists,
	}
}

func TestSite_Parse(t *testing.T) {
	tests := map[string]struct {
		page layout
		want []trend.Entry
	}{
		"the ranking only, in the order of the page": {
			page: weekly(
				cards("one/first", "two/second", "three/third", "four/fourth", "five/fifth"),
				`{"@type":"WebSite","name":"Trendshift"}`,
				itemList(5,
					listed{name: "one/first", description: " The\n first. ", language: "Go"},
					listed{name: "two/second"},
					listed{name: "three/third", language: "Rust"},
					listed{name: "four/fourth"},
					listed{name: "five/fifth"},
				),
				`not JSON at all`,
			),
			want: []trend.Entry{
				{Rank: 1, Repo: trend.Repo{Owner: "one", Name: "first"}, Description: "The first.", Language: "Go"},
				{Rank: 2, Repo: trend.Repo{Owner: "two", Name: "second"}},
				{Rank: 3, Repo: trend.Repo{Owner: "three", Name: "third"}, Language: "Rust"},
				{Rank: 4, Repo: trend.Repo{Owner: "four", Name: "fourth"}},
				{Rank: 5, Repo: trend.Repo{Owner: "five", Name: "fifth"}},
			},
		},
		"a repeated card stays once at its first rank": {
			page: weekly(
				cards("one/first", "two/second", "One/First", "three/third"),
				itemList(4, names("one/first", "two/second", "One/First", "three/third")...),
			),
			want: []trend.Entry{
				{Rank: 1, Repo: trend.Repo{Owner: "one", Name: "first"}},
				{Rank: 2, Repo: trend.Repo{Owner: "two", Name: "second"}},
				{Rank: 3, Repo: trend.Repo{Owner: "three", Name: "third"}},
			},
		},
		"the page and its structured list may differ in case": {
			page: weekly(cards("One/First"), itemList(1, listed{name: "one/first"})),
			want: []trend.Entry{
				{Rank: 1, Repo: trend.Repo{Owner: "One", Name: "First"}},
			},
		},
		"the structured list may name a repository by its URL": {
			page: weekly(cards("one/first"),
				itemList(1, listed{name: "First", repository: "https://github.com/one/first"})),
			want: []trend.Entry{
				{Rank: 1, Repo: trend.Repo{Owner: "one", Name: "first"}},
			},
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			page, err := New().Parse(document(test.page), trend.Weekly)
			require.NoError(t, err)
			assert.Equal(t, "https://trendshift.io/weekly Weekly", page.Marker)
			assert.Equal(t, trend.Weekly, page.Period)
			assert.Equal(t, test.want, page.Entries)
		})
	}

	t.Run("the daily page", func(t *testing.T) {
		for _, canonical := range []string{"https://trendshift.io", "https://trendshift.io/"} {
			page, err := New().Parse(document(layout{
				canonical: []string{canonical},
				current:   []string{"Daily"},
				cards:     cards("one/first"),
				lists:     []string{itemList(1, names("one/first")...)},
			}), trend.Daily)
			require.NoError(t, err)
			assert.Equal(t, canonical+" Daily", page.Marker)
			assert.Equal(t, trend.Daily, page.Period)
		}
	})
}

func TestSite_Parse_errors(t *testing.T) {
	three := cards("one/first", "two/second", "three/third")
	with := func(change func(*layout)) []byte {
		page := weekly(three, itemList(3, names("one/first", "two/second", "three/third")...))
		change(&page)
		return document(page)
	}

	tests := map[string]struct {
		body []byte
		want string
	}{
		"empty body": {
			body: nil,
			want: "the answer is empty",
		},
		"white space only": {
			body: []byte("\n  \n"),
			want: "the answer is empty",
		},
		"not a ranking page": {
			body: []byte("<html><body><h1>Just a moment...</h1></body></html>"),
			want: "does not say which period it shows",
		},
		"marker but no cards": {
			body: document(weekly(nil)),
			want: "the page has no repositories (https://trendshift.io/weekly Weekly)",
		},
		"marker but an empty ranking": {
			body: document(weekly(nil, itemList(0))),
			want: "the page has no repositories (https://trendshift.io/weekly Weekly)",
		},
		"no canonical link": {
			body: with(func(page *layout) { page.canonical = nil }),
			want: "it has no canonical link",
		},
		"two canonical links": {
			body: with(func(page *layout) {
				page.canonical = []string{"https://trendshift.io/weekly", "https://trendshift.io/monthly"}
			}),
			want: "the page names two periods: canonical links https://trendshift.io/weekly and https://trendshift.io/monthly",
		},
		"canonical link of another site": {
			body: with(func(page *layout) { page.canonical = []string{"https://example.org/weekly"} }),
			want: `the canonical link of the page is not on trendshift.io: "https://example.org/weekly"`,
		},
		"a period pulse does not watch": {
			body: with(func(page *layout) { page.canonical = []string{"https://trendshift.io/yearly"} }),
			want: "a period pulse does not watch: canonical link https://trendshift.io/yearly",
		},
		"an archive of the period": {
			body: with(func(page *layout) { page.canonical = []string{"https://trendshift.io/weekly/2026/46"} }),
			want: "a period pulse does not watch",
		},
		"no current item in the navigation": {
			body: with(func(page *layout) { page.current = []string{"", ""} }),
			want: "its navigation has no current Daily, Weekly or Monthly item",
		},
		"the current item is not a period pulse watches": {
			body: with(func(page *layout) { page.current = []string{"Yearly"} }),
			want: "its navigation has no current Daily, Weekly or Monthly item",
		},
		"the navigation names two periods": {
			body: with(func(page *layout) { page.current = []string{"Weekly", "Monthly"} }),
			want: "the page names two periods in its navigation: Weekly and Monthly",
		},
		"the link and the navigation name different periods": {
			body: with(func(page *layout) { page.current = []string{"Monthly"} }),
			want: `the page names two periods: weekly in its canonical link https://trendshift.io/weekly and monthly as "Monthly"`,
		},
		"cards without a structured list": {
			body: with(func(page *layout) { page.lists = nil }),
			want: "the page shows 3 ranked repositories but has no structured list to check them against",
		},
		"a card the structured list does not have": {
			body: with(func(page *layout) { page.lists = []string{itemList(2, names("one/first", "two/second")...)} }),
			want: "the page shows 3 ranked repositories but its structured list has 2",
		},
		"a repository of the structured list without a card": {
			body: with(func(page *layout) { page.cards = cards("one/first", "two/second") }),
			want: "the page shows 2 ranked repositories but its structured list has 3",
		},
		"no card has a badge": {
			body: with(func(page *layout) {
				page.cards = []card{{name: "one/first", badge: "New"}, {name: "two/second", badge: "New"}, {name: "three/third", badge: "New"}}
			}),
			want: "the page shows 0 ranked repositories but its structured list has 3",
		},
		"another order": {
			body: with(func(page *layout) { page.cards = cards("one/first", "three/third", "two/second") }),
			want: "position 2 of the ranking is three/third on the page but two/second in its structured list",
		},
		"badges out of order": {
			body: with(func(page *layout) {
				page.cards = []card{{name: "one/first"}, {name: "two/second"}, {name: "three/third", badge: "4"}}
			}),
			want: "the ranking of the page is out of order: position 3 has the badge 4",
		},
		"a second ranking on the page": {
			body: append(with(func(*layout) {}), with(func(page *layout) { page.lists = nil })...),
			want: "the ranking of the page is out of order: position 4 has the badge 1",
		},
		"a card that is not a repository": {
			body: with(func(page *layout) { page.cards = cards("one/first", "second") }),
			want: `position 2 of the ranking is not a repository: "second"`,
		},
		"the structured list out of order": {
			body: with(func(page *layout) {
				page.lists = []string{itemList(3,
					listed{name: "one/first"}, listed{name: "two/second", position: 3}, listed{name: "three/third", position: 2})}
			}),
			want: "the structured list of the page is out of order: position 2 is numbered 3",
		},
		"the structured list miscounts itself": {
			body: with(func(page *layout) {
				page.lists = []string{itemList(25, names("one/first", "two/second", "three/third")...)}
			}),
			want: "the structured list of the page says it has 25 repositories but holds 3",
		},
		"two structured lists": {
			body: with(func(page *layout) { page.lists = append(page.lists, page.lists[0]) }),
			want: "the page has 2 structured lists, not one",
		},
		"the structured list holds something else": {
			body: with(func(page *layout) {
				page.lists = []string{itemList(3, listed{name: "one/first"}, listed{name: "Second"}, listed{name: "three/third"})}
			}),
			want: `position 2 of the structured list of the page is not a repository: "Second"`,
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			page, err := New().Parse(test.body, trend.Weekly)
			require.ErrorContains(t, err, test.want)
			assert.Zero(t, page)
		})
	}
}
