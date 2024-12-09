package web_test

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.octolab.org/toolset/pulse/internal/details"
	"go.octolab.org/toolset/pulse/internal/trend"
)

// github answers the reader in place of GitHub.
type github struct {
	asked []string
	err   error
}

func (g *github) Details(_ context.Context, repo trend.Repo) (details.Details, error) {
	g.asked = append(g.asked, repo.String())
	if g.err != nil {
		return details.Details{}, g.err
	}
	rendered := `<article class="markdown-body"><h1>Ghostty</h1>` +
		`<p onclick="steal()">Fast. <a href="docs/install.md">Install</a> <img src="logo.png" alt="logo"></p>` +
		`<script>steal()</script></article>`
	return details.Details{
		Repo: trend.Repo{Owner: "ghostty-org", Name: "ghostty"}, Description: "Fast terminal, as GitHub describes it.",
		Homepage: "https://ghostty.org", Language: "Zig", License: "MIT", Topics: []string{"terminal", "zig"},
		Stars: 41234, Forks: 1502, OpenIssues: 87,
		CreatedAt: time.Date(2022, time.March, 29, 0, 0, 0, 0, time.UTC),
		PushedAt:  time.Date(2026, time.November, 17, 0, 0, 0, 0, time.UTC),
		Readme:    details.Clean([]byte(rendered), trend.Repo{Owner: "ghostty-org", Name: "ghostty"}),
	}, nil
}

// Opening a repository in the reader marks nothing and shows the facts and
// the README of GitHub; the README cannot run scripts, and its relative
// links and images point to github.com.
func TestReaderShowsGitHubAndMarksNothing(t *testing.T) {
	hub := &github{}
	s := start(t, "")
	s.github = hub
	s.restart()
	s.loader.Day(t, "testdata/today.json")
	before := s.open("/")

	// The address is without case, as the key of a repository is.
	answer := s.get("/repos/Ghostty-Org/Ghostty")
	require.Equal(t, 200, answer.Code, answer.Body.String())
	story := answer.Body.String()
	assert.Contains(t, story, `data-repo="ghostty-org/ghostty" data-state="shown" data-name="ghostty-org/ghostty"`)
	assert.Contains(t, story, "Fast terminal, as GitHub describes it.")
	for _, fact := range []string{
		"<dt>Stars</dt><dd>41,234</dd>", "<dt>Forks</dt><dd>1,502</dd>", "<dt>Open issues</dt><dd>87</dd>",
		"<dt>License</dt><dd>MIT</dd>", "March 2022", "Nov 17, 2026", "<li>terminal</li>", `href="https://ghostty.org"`,
		"trendshift daily <b>#1</b>", "github daily <b>#4</b>",
		`data-action="interesting"`, `data-action="skip"`,
	} {
		assert.Contains(t, story, fact)
	}
	frame := regexp.MustCompile(`<iframe[^>]*>`).FindString(story)
	assert.Contains(t, frame, `src="/repos/ghostty-org/ghostty/readme"`)
	assert.Contains(t, frame, `sandbox="allow-popups allow-popups-to-escape-sandbox"`)
	assert.NotContains(t, frame, "allow-scripts")
	assert.NotContains(t, frame, "allow-same-origin")

	readme := s.get("/repos/ghostty-org/ghostty/readme")
	require.Equal(t, 200, readme.Code)
	policy := readme.Header().Get("Content-Security-Policy")
	assert.Contains(t, policy, "default-src 'none'")
	assert.Contains(t, policy, "sandbox allow-popups")
	assert.NotContains(t, policy, "script-src")
	assert.NotContains(t, policy, "allow-scripts")
	body := readme.Body.String()
	assert.Contains(t, body, "<h1>Ghostty</h1>")
	assert.Contains(t, body, `href="https://github.com/ghostty-org/ghostty/blob/HEAD/docs/install.md"`)
	assert.Contains(t, body, `src="https://github.com/ghostty-org/ghostty/raw/HEAD/logo.png"`)
	assert.Contains(t, body, `target="_blank"`)
	assert.NotContains(t, body, "<script")
	assert.NotContains(t, body, "onclick")
	assert.NotContains(t, body, "steal")

	// Nothing is marked: the page is as it was, and so is the disk.
	after := s.open("/")
	assert.Equal(t, before.keys("shown"), after.keys("shown"))
	assert.Empty(t, s.store.Marks())
	assert.Equal(t, []string{"ghostty-org/ghostty", "ghostty-org/ghostty"}, hub.asked)

	// The main page may frame only itself.
	assert.Contains(t, s.get("/").Header().Get("Content-Security-Policy"), "frame-src 'self'")
	assert.Equal(t, 400, s.get("/repos/a%20b/c").Code)
}

// When GitHub does not answer, the reader shows what the lists know, the
// reason and the actions.
func TestReaderWithoutGitHub(t *testing.T) {
	hub := &github{err: errors.New("GitHub does not answer: connection refused")}
	s := start(t, "")
	s.github = hub
	s.restart()
	s.loader.Day(t, "testdata/today.json")
	s.open("/")

	answer := s.get("/repos/ghostty-org/ghostty")
	require.Equal(t, 200, answer.Code)
	story := answer.Body.String()
	assert.Contains(t, story, "A fast, native, feature-rich terminal emulator.")
	assert.Contains(t, story, "trendshift daily <b>#1</b>")
	assert.Contains(t, story, `data-story="error"`)
	assert.Contains(t, story, "GitHub does not answer: connection refused")
	assert.Contains(t, story, `data-action="interesting"`)
	assert.Contains(t, story, `data-action="skip"`)
	assert.NotContains(t, story, "<iframe")

	// A hidden repository keeps its reasons in the reader, with the answer
	// it has pressed, and Take back.
	require.Equal(t, 200, s.mark("skip", "ghostty-org/ghostty", "").Code)
	story = s.get("/repos/ghostty-org/ghostty").Body.String()
	assert.Contains(t, story, `data-state="hidden"`)
	assert.Contains(t, story, `data-reason="seen"`)
	assert.Contains(t, story, `data-stamp="skip">Skipped · until`)
	assert.Contains(t, story, `data-action="skip" aria-pressed="true" disabled`)
	assert.Contains(t, story, `data-action="clear"`)

	// Without anybody to ask, the reader says so.
	s.github = nil
	s.restart()
	story = s.get("/repos/rust-lang/rust").Body.String()
	assert.Contains(t, story, `data-story="offline"`)
	assert.Contains(t, s.get("/repos/rust-lang/rust/readme").Body.String(), "has no README")
}

// The front page and the sections hold every new repository exactly once.
func TestSectionsHoldEveryNewRepositoryOnce(t *testing.T) {
	s := start(t, "")
	s.loader.Day(t, "testdata/today.json")
	page := s.open("/")

	shown := page.keys("shown")
	require.Len(t, shown, 10)
	seen := map[string]int{}
	for _, key := range shown {
		seen[key]++
	}
	for key, times := range seen {
		assert.Equal(t, 1, times, key)
	}

	sections := regexp.MustCompile(`data-section="([a-z]+)"`).FindAllStringSubmatch(page.html, -1)
	require.NotEmpty(t, sections)
	assert.Equal(t, "front", sections[0][1])
	kinds := map[string]int{}
	for _, section := range sections {
		kinds[section[1]]++
	}
	for kind, times := range kinds {
		assert.Equal(t, 1, times, kind)
	}

	// The lead story is the repository in most lists, and there is one.
	assert.Equal(t, 1, strings.Count(page.html, " is-lead"))
	lead := regexp.MustCompile(`<article class="card is-shown is-lead" data-repo="([^"]+)"`).FindStringSubmatch(page.html)
	require.NotNil(t, lead)
	most := 0
	for _, key := range shown {
		most = max(most, len(page.card(t, key).badges))
	}
	assert.Len(t, page.card(t, lead[1]).badges, most)
	assert.Equal(t, shown[0], lead[1], "the lead opens the paper")
}
