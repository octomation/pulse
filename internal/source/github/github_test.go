package github_test

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	. "go.octolab.org/toolset/pulse/internal/source/github"
	"go.octolab.org/toolset/pulse/internal/source/internal/fixture"
	"go.octolab.org/toolset/pulse/internal/trend"
)

const testdata = "../testdata"

var periods = []trend.Period{trend.Daily, trend.Weekly, trend.Monthly}

func TestSite_Source(t *testing.T) {
	assert.Equal(t, trend.GitHub, New().Source())
}

func TestSite_Request(t *testing.T) {
	tests := map[trend.Period]string{
		trend.Daily:   "https://github.com/trending?since=daily",
		trend.Weekly:  "https://github.com/trending?since=weekly",
		trend.Monthly: "https://github.com/trending?since=monthly",
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
			saved := fixture.Load(t, testdata, trend.List{Source: trend.GitHub, Period: period})
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
		})
	}
}

func TestSite_Parse_extras(t *testing.T) {
	saved := fixture.Load(t, testdata, trend.List{Source: trend.GitHub, Period: trend.Weekly})

	page, err := New().Parse(saved.Body, trend.Weekly)
	require.NoError(t, err)
	require.NotEmpty(t, page.Entries)

	first := page.Entries[0]
	assert.Equal(t, trend.Repo{Owner: "backnotprop", Name: "plannotator"}, first.Repo)
	assert.True(t, strings.HasPrefix(first.Description, "The workspace to stay in the loop"), first.Description)
	assert.False(t, strings.HasSuffix(first.Description, " "), "the description is trimmed")
	assert.Equal(t, "TypeScript", first.Language)
}

func TestSite_Parse_anotherPeriod(t *testing.T) {
	for _, period := range periods {
		saved := fixture.Load(t, testdata, trend.List{Source: trend.GitHub, Period: period})
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
		saved := fixture.Load(t, testdata, trend.List{Source: trend.GitHub, Period: period})
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

func TestSite_Parse_fixtureWithoutMarkers(t *testing.T) {
	saved := fixture.Load(t, testdata, trend.List{Source: trend.GitHub, Period: trend.Daily})

	t.Run("no date range", func(t *testing.T) {
		body := bytes.ReplaceAll(saved.Body, []byte("Date range:"), nil)
		require.NotEqual(t, saved.Body, body)

		_, err := New().Parse(body, trend.Daily)
		require.ErrorContains(t, err, "does not say which period it shows")
	})

	t.Run("no period in the rows", func(t *testing.T) {
		body := bytes.ReplaceAll(saved.Body, []byte(" stars today"), []byte(" stars"))
		require.NotEqual(t, saved.Body, body)

		_, err := New().Parse(body, trend.Daily)
		require.ErrorContains(t, err, "do not say which period their stars are of")
	})

	t.Run("rows of another period", func(t *testing.T) {
		body := bytes.ReplaceAll(saved.Body, []byte(" stars today"), []byte(" stars this week"))
		require.NotEqual(t, saved.Body, body)

		_, err := New().Parse(body, trend.Daily)
		require.ErrorContains(t, err, "the page names two periods")
	})
}

type row struct {
	href        string
	description string
	language    string
	gained      string
}

func document(selected string, rows ...row) []byte {
	var b strings.Builder
	b.WriteString("<!DOCTYPE html><html><head><title>Trending</title></head><body><main>")
	if selected != "" {
		b.WriteString("<details id=\"select-menu-date\"><summary class=\"select-menu-button\">    Date range:\n\n")
		b.WriteString("    <span data-menu-button=\"\" class=\"text-bold\">\n      " + selected + "\n</span>\n</summary></details>")
	}
	b.WriteString("<details><summary>Language: <span>Any</span></summary></details><div class=\"Box\">")
	for _, r := range rows {
		b.WriteString("<article class=\"Box-row\">\n<div class=\"float-right\"><a href=\"/login?return_to=%2Fother%2Frepo\">Star</a></div>\n")
		if r.href != "" {
			owner, name, _ := strings.Cut(strings.Trim(r.href, "/"), "/")
			b.WriteString("<h2 class=\"h3 lh-condensed\">\n<a href=\"" + r.href + "\">\n<span class=\"text-normal\">\n" +
				owner + " /\n</span>\n" + name + "</a>  </h2>\n")
		}
		if r.description != "" {
			b.WriteString("<p class=\"col-9\">\n      " + r.description + "\n    </p>\n")
		}
		b.WriteString("<div class=\"f6\">")
		if r.language != "" {
			b.WriteString("<span><span itemprop=\"programmingLanguage\">" + r.language + "</span></span>")
		}
		b.WriteString("<a href=\"" + r.href + "/stargazers\">\n 1,234</a>")
		if r.gained != "" {
			b.WriteString("<span class=\"float-sm-right\">\n<svg></svg>\n        " + r.gained + "\n</span>")
		}
		b.WriteString("</div>\n</article>")
	}
	b.WriteString("</div></main></body></html>")
	return []byte(b.String())
}

func TestSite_Parse(t *testing.T) {
	one := row{href: "/one/first", description: "The first.", language: "Go", gained: "1,204 stars this week"}
	two := row{href: "/two/second", gained: "1 star this week"}

	tests := map[string]struct {
		body []byte
		want []trend.Entry
	}{
		"rows in the order of the page": {
			body: document("This week", one, two),
			want: []trend.Entry{
				{Rank: 1, Repo: trend.Repo{Owner: "one", Name: "first"}, Description: "The first.", Language: "Go"},
				{Rank: 2, Repo: trend.Repo{Owner: "two", Name: "second"}},
			},
		},
		"a repeated row stays once at its first rank": {
			body: document("This week", one, two, row{href: "/One/First", gained: "3 stars this week"},
				row{href: "/three/third", gained: "2 stars this week"}),
			want: []trend.Entry{
				{Rank: 1, Repo: trend.Repo{Owner: "one", Name: "first"}, Description: "The first.", Language: "Go"},
				{Rank: 2, Repo: trend.Repo{Owner: "two", Name: "second"}},
				{Rank: 3, Repo: trend.Repo{Owner: "three", Name: "third"}},
			},
		},
		"a row may leave its period out when another names it": {
			body: document("This week", row{href: "/two/second"}, one),
			want: []trend.Entry{
				{Rank: 1, Repo: trend.Repo{Owner: "two", Name: "second"}},
				{Rank: 2, Repo: trend.Repo{Owner: "one", Name: "first"}, Description: "The first.", Language: "Go"},
			},
		},
		"a description that looks like a counter is not a marker": {
			body: document("This week", row{href: "/two/second", description: "5 stars today", gained: "7 stars this week"}),
			want: []trend.Entry{
				{Rank: 1, Repo: trend.Repo{Owner: "two", Name: "second"}, Description: "5 stars today"},
			},
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			page, err := New().Parse(test.body, trend.Weekly)
			require.NoError(t, err)
			assert.Equal(t, "Date range: This week, stars this week", page.Marker)
			assert.Equal(t, trend.Weekly, page.Period)
			assert.Equal(t, test.want, page.Entries)
		})
	}
}

func TestSite_Parse_errors(t *testing.T) {
	weekly := row{href: "/one/first", gained: "12 stars this week"}

	tests := map[string]struct {
		body []byte
		want string
	}{
		"empty body": {
			body: nil,
			want: "the answer is empty",
		},
		"white space only": {
			body: []byte(" \n\t "),
			want: "the answer is empty",
		},
		"not a trending page": {
			body: []byte("<html><body><h1>Sign in</h1></body></html>"),
			want: "does not say which period it shows",
		},
		"marker but no rows": {
			body: document("This week"),
			want: "the page has no repositories (Date range: This week)",
		},
		"rows but no date range": {
			body: document("", weekly),
			want: `it has no "Date range" menu`,
		},
		"rows that do not name their period": {
			body: document("This week", row{href: "/one/first"}),
			want: "do not say which period their stars are of",
		},
		"menu and rows of different periods": {
			body: document("This week", weekly, row{href: "/two/second", gained: "5 stars today"}),
			want: `the page names two periods: "Date range: This week" in its menu and "stars today" for two/second`,
		},
		"a period pulse does not watch": {
			body: document("This year", weekly),
			want: `a period pulse does not watch: "Date range: This year"`,
		},
		"a row without a repository": {
			body: document("This week", weekly, row{gained: "5 stars this week"}),
			want: "row 2 of the page: no repository title",
		},
		"a row that links to an owner": {
			body: document("This week", row{href: "/sponsors", gained: "5 stars this week"}),
			want: `row 1 of the page: the link "/sponsors" is not a repository`,
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			page, err := New().Parse(test.body, trend.Weekly)
			require.ErrorContains(t, err, test.want)
			assert.Zero(t, page)
		})
	}

	t.Run("two date ranges", func(t *testing.T) {
		body := append(document("This week", weekly), document("Today")...)
		_, err := New().Parse(body, trend.Weekly)
		require.ErrorContains(t, err, "the page names two periods")
	})
}
