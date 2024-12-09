package details_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	. "go.octolab.org/toolset/pulse/internal/details"
	"go.octolab.org/toolset/pulse/internal/trend"
)

var cobra = trend.Repo{Owner: "SPF13", Name: "Cobra"}

const facts = `{"name":"cobra","owner":{"login":"spf13"},"description":" A Commander ","homepage":"https://cobra.dev",
"language":"Go","license":{"spdx_id":"Apache-2.0","name":"Apache License 2.0"},"topics":["cli","go"],
"stargazers_count":44801,"forks_count":3785,"open_issues_count":512,"archived":false,
"created_at":"2013-09-03T20:40:26Z","pushed_at":"2026-07-11T00:43:07Z"}`

func TestDetails(t *testing.T) {
	var requests []string
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.URL.Path+" "+r.Header.Get("Accept")+" "+r.Header.Get("Authorization"))
		switch r.URL.Path {
		case "/repos/SPF13/Cobra":
			_, _ = w.Write([]byte(facts))
		case "/repos/SPF13/Cobra/readme":
			_, _ = w.Write([]byte(`<article><h1>Cobra</h1><a href="#usage">Usage</a><a href="javascript:alert(1)">x</a></article>`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(api.Close)

	now := time.Date(2026, time.November, 20, 8, 0, 0, 0, time.UTC)
	client := New(Options{Token: "secret", BaseURL: api.URL, Now: func() time.Time { return now }})

	got, err := client.Details(context.Background(), cobra)
	require.NoError(t, err)
	assert.Equal(t, trend.Repo{Owner: "spf13", Name: "cobra"}, got.Repo, "the name as GitHub writes it")
	assert.Equal(t, "A Commander", got.Description)
	assert.Equal(t, "https://cobra.dev", got.Homepage)
	assert.Equal(t, "Apache-2.0", got.License)
	assert.Equal(t, []string{"cli", "go"}, got.Topics)
	assert.Equal(t, [3]int{44801, 3785, 512}, [3]int{got.Stars, got.Forks, got.OpenIssues})
	assert.Equal(t, 2013, got.CreatedAt.Year())
	assert.Contains(t, got.Readme, `href="https://github.com/spf13/cobra#usage"`)
	assert.NotContains(t, got.Readme, "javascript")
	assert.Equal(t, []string{
		"/repos/SPF13/Cobra application/vnd.github+json Bearer secret",
		"/repos/SPF13/Cobra/readme application/vnd.github.html+json Bearer secret",
	}, requests)

	// A fresh answer is kept, for any spelling of the name.
	_, err = client.Details(context.Background(), trend.Repo{Owner: "spf13", Name: "cobra"})
	require.NoError(t, err)
	assert.Len(t, requests, 2)
	now = now.Add(DefaultTTL)
	_, err = client.Details(context.Background(), cobra)
	require.NoError(t, err)
	assert.Len(t, requests, 4)
}

func TestDetailsFailures(t *testing.T) {
	status, remaining, readme := http.StatusOK, "", http.StatusNotFound
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Empty(t, r.Header.Get("Authorization"), "no token, no header")
		if r.URL.Path == "/repos/SPF13/Cobra/readme" && status == http.StatusOK {
			w.WriteHeader(readme)
			return
		}
		if remaining != "" {
			w.Header().Set("X-RateLimit-Remaining", remaining)
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(facts))
	}))
	t.Cleanup(api.Close)
	client := New(Options{BaseURL: api.URL})

	// No README is not an error.
	got, err := client.Details(context.Background(), cobra)
	require.NoError(t, err)
	assert.Empty(t, got.Readme)
	assert.Equal(t, 44801, got.Stars)

	other := trend.Repo{Owner: "nobody", Name: "nothing"}
	status = http.StatusNotFound
	_, err = client.Details(context.Background(), other)
	require.ErrorIs(t, err, ErrNotFound)

	status, remaining = http.StatusForbidden, "0"
	_, err = client.Details(context.Background(), other)
	require.ErrorIs(t, err, ErrRateLimited)
	assert.Contains(t, err.Error(), "GITHUB_TOKEN")

	status, remaining = http.StatusBadGateway, ""
	_, err = client.Details(context.Background(), other)
	require.ErrorContains(t, err, "GitHub answered 502")

	api.Close()
	_, err = client.Details(context.Background(), other)
	require.ErrorContains(t, err, "GitHub does not answer")
}

func TestClean(t *testing.T) {
	repo := trend.Repo{Owner: "o", Name: "n"}
	got := Clean([]byte(`<div style="x" onmouseover="y()"><!-- c --><style>p{}</style><iframe src="https://e.example"></iframe>`+
		`<form action="/x"><input></form><a href="/o/n/issues" target="_self">i</a><a href="data:text/html,x">d</a>`+
		`<img src="docs/a b.png" srcset="x 2x"><img src="//cdn.example/i.png"><video src="v.mp4" poster="p.png"></video>`+
		`<a href="https://example.com/a?b=c#d">e</a></div>`), repo)
	for _, gone := range []string{"style", "onmouseover", "<!--", "iframe", "form", "data:", "_self"} {
		assert.NotContains(t, got, gone)
	}
	for _, kept := range []string{
		`href="https://github.com/o/n/issues"`, `src="https://github.com/o/n/raw/HEAD/docs/a%20b.png"`,
		`src="https://cdn.example/i.png"`, `src="https://github.com/o/n/raw/HEAD/v.mp4"`,
		`poster="https://github.com/o/n/raw/HEAD/p.png"`, `href="https://example.com/a?b=c#d"`,
		`rel="noopener noreferrer nofollow"`, `srcset="https://github.com/o/n/raw/HEAD/x 2x"`,
	} {
		assert.Contains(t, got, kept)
	}
}

// The pictures of a README show as on GitHub: a <picture> names them in
// srcset, and a picture the proxy of GitHub cannot fetch comes from its own
// address.
func TestCleanPictures(t *testing.T) {
	repo := trend.Repo{Owner: "o", Name: "n"}
	got := Clean([]byte(`<picture><source media="(prefers-color-scheme: dark)" srcset="https://github.com/user-attachments/assets/dark">`+
		`<source srcset="light.png 1x, javascript:alert(1) 2x, https://cdn.example/l@2x.png 2x"><img alt="Banner"></picture>`+
		`<img src="https://camo.githubusercontent.com/abc" data-canonical-src="https://trendshift.io/api/badge/1" class="js-gh-image-fallback">`+
		`<img src="https://camo.githubusercontent.com/def" data-canonical-src="https://example.com/b.svg">`+
		`<img src="https://camo.githubusercontent.com/ghi" data-canonical-src="javascript:x" class="js-gh-image-fallback">`), repo)
	for _, kept := range []string{
		`srcset="https://github.com/user-attachments/assets/dark"`,
		`srcset="https://github.com/o/n/raw/HEAD/light.png 1x, https://cdn.example/l@2x.png 2x"`,
		`<img src="https://trendshift.io/api/badge/1"`,
		`<img src="https://camo.githubusercontent.com/def"`,
		`<img src="https://camo.githubusercontent.com/ghi"`,
	} {
		assert.Contains(t, got, kept)
	}
	assert.NotContains(t, got, "javascript:alert")
	assert.NotContains(t, got, ` src="javascript`)
}
