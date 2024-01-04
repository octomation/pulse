package web_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.octolab.org/toolset/pulse/internal/store"
	"go.octolab.org/toolset/pulse/internal/trend"
	"go.octolab.org/toolset/pulse/internal/watch/watchtest"
	. "go.octolab.org/toolset/pulse/internal/web"
)

var (
	trendshiftDaily = trend.List{Source: trend.Trendshift, Period: trend.Daily}
	githubDaily     = trend.List{Source: trend.GitHub, Period: trend.Daily}
	githubWeekly    = trend.List{Source: trend.GitHub, Period: trend.Weekly}
	githubMonthly   = trend.List{Source: trend.GitHub, Period: trend.Monthly}
)

// Yesterday's lists have 10 repositories, all marked as seen; today's have 8
// of them and 2 new ones: the page shows exactly those 2.
func TestYesterdaySeenTodayTwoNew(t *testing.T) {
	s := start(t, "")
	s.loader.Day(t, "testdata/yesterday.json")

	yesterday := s.open("/")
	require.Len(t, yesterday.keys("shown"), 10)
	assert.Equal(t, "10", yesterday.count(t, "new"))
	assert.Equal(t, "0", yesterday.count(t, "hidden"))

	response := s.seenAll(yesterday)
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	var answer struct {
		New, Hidden, Marked int
		Empty               string
	}
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &answer))
	assert.Equal(t, 0, answer.New)
	assert.Equal(t, 10, answer.Hidden)
	assert.Equal(t, 10, answer.Marked)
	assert.Contains(t, answer.Empty, `data-empty="clear"`)
	assert.Empty(t, s.open("/").keys("shown"))

	s.clock.Advance(24 * time.Hour)
	s.loader.Day(t, "testdata/today.json")
	s.restart()

	today := s.open("/")
	assert.Equal(t, []string{"ghostty-org/ghostty", "zed-industries/zed"}, today.keys("shown"))
	assert.Empty(t, today.keys("hidden"), "the hidden cards are not on the page without the switch")
	assert.Equal(t, "2", today.count(t, "new"))
	assert.Equal(t, "8", today.count(t, "hidden"))
	assert.False(t, today.saysNothingNew())

	hidden := s.open("/?hidden=1")
	assert.Equal(t, []string{"ghostty-org/ghostty", "zed-industries/zed"}, hidden.keys("shown"))
	assert.Len(t, hidden.keys("hidden"), 8)
	for _, key := range hidden.keys("hidden") {
		assert.Equal(t, "Marked as seen on Nov 18, 2026, 08:15", hidden.card(t, key).reasons["seen"], key)
	}
}

// "Mark all shown as seen" marks what the page showed when it was rendered,
// not what a later load added.
func TestSeenAllMarksOnlyWhatThePageShowed(t *testing.T) {
	s := start(t, "")
	s.loader.Day(t, "testdata/yesterday.json")
	rendered := s.open("/")
	require.Len(t, rendered.keys("shown"), 10)

	// Another tab refreshes before the button is clicked on this one.
	s.loader.Day(t, "testdata/today.json")
	require.Equal(t, http.StatusOK, s.post("/api/refresh", struct{}{}).Code)

	require.Equal(t, http.StatusOK, s.seenAll(rendered).Code)
	assert.Equal(t, []string{"ghostty-org/ghostty", "zed-industries/zed"}, s.open("/").keys("shown"))
}

// A repository from the vault bookmarks and a repository seen a month ago,
// both trending again, are not shown, and the hidden view names the reason
// for each.
func TestVaultAndMonthOldSeenStayHidden(t *testing.T) {
	s := start(t, "testdata/vault")
	s.loader.Day(t, "testdata/yesterday.json")

	before := s.open("/")
	require.Contains(t, before.keys("shown"), "tailscale/tailscale")
	require.Equal(t, http.StatusOK, s.mark("skip", "tailscale/tailscale", "").Code)

	s.clock.Advance(31 * 24 * time.Hour)
	s.loader.Day(t, "testdata/trending-again.json")
	s.restart()

	// The note of an owner hides nothing: its repository is the only card.
	page := s.open("/")
	assert.Equal(t, []string{"anthropics/claude-code"}, page.keys("shown"))
	assert.Equal(t, "1", page.count(t, "new"))
	assert.Equal(t, "3", page.count(t, "hidden"))
	assert.Contains(t, page.html, `data-vault="ok"`)
	assert.Contains(t, page.html, "Vault: 2 repositories in 5 notes")
	assert.Contains(t, page.html, "1 note could not be read")

	hidden := s.open("/?hidden=1")
	assert.Equal(t, []string{"anthropics/claude-code"}, hidden.keys("shown"))
	assert.Equal(t, []string{"snouzy/workout-cool", "tailscale/tailscale", "astral-sh/uv"}, hidden.keys("hidden"))

	byURL := hidden.card(t, "snouzy/workout-cool")
	assert.Equal(t, map[string]string{"vault": "In the vault: workout-cool.md"}, byURL.reasons)

	byAlias := hidden.card(t, "astral-sh/uv")
	assert.Equal(t, map[string]string{"vault": "In the vault: uv.md"}, byAlias.reasons)

	seen := hidden.card(t, "tailscale/tailscale")
	assert.Equal(t, map[string]string{"seen": "Skipped on Nov 18, 2026, 08:15"}, seen.reasons)
}

// The notes are read again for every page: a bookmark added to the vault hides
// its repository on the next load of the page.
func TestVaultIsReadForEveryPage(t *testing.T) {
	vault := t.TempDir()
	note := "---\nurl: https://github.com/ollama/ollama\n---\n"
	require.NoError(t, os.WriteFile(filepath.Join(vault, "ollama.md"), []byte(note), 0o600))

	s := start(t, vault)
	s.loader.Day(t, "testdata/yesterday.json")
	assert.Contains(t, s.open("/").keys("shown"), "golang/go")

	note = "---\naliases:\n  - golang/go\n---\n"
	require.NoError(t, os.WriteFile(filepath.Join(vault, "go.md"), []byte(note), 0o600))
	page := s.open("/?hidden=1")
	assert.NotContains(t, page.keys("shown"), "golang/go")
	assert.Equal(t, "In the vault: go.md", page.card(t, "golang/go").reasons["vault"])
}

// A vault directory that cannot be read, or names no repository, is an
// error on the page; without a directory the page says the filter is off.
func TestVaultStatus(t *testing.T) {
	t.Run("off", func(t *testing.T) {
		s := start(t, "")
		s.loader.Day(t, "testdata/yesterday.json")
		page := s.open("/")
		assert.Contains(t, page.html, `data-vault="off"`)
		assert.Contains(t, page.html, "The vault filter is off")
	})

	t.Run("missing directory", func(t *testing.T) {
		s := start(t, filepath.Join(t.TempDir(), "nowhere"))
		s.loader.Day(t, "testdata/yesterday.json")
		page := s.open("/")
		assert.Contains(t, page.html, `data-vault="error"`)
		assert.Contains(t, page.html, "The list may include what is already in the vault.")
		assert.Len(t, page.keys("shown"), 10, "the lists are shown without the filter")
	})

	t.Run("no repository", func(t *testing.T) {
		vault := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(vault, "note.md"), []byte("---\nurl: https://github.com/golang\n---\n"), 0o600))
		s := start(t, vault)
		s.loader.Day(t, "testdata/yesterday.json")
		page := s.open("/")
		assert.Contains(t, page.html, `data-vault="error"`)
		assert.Contains(t, page.html, "names a repository")
		assert.Contains(t, page.html, "The list may include what is already in the vault.")
	})
}

// A repository in several lists, under different letter case, is one card
// that names all its lists.
func TestOneCardForSeveralListsAndLetterCases(t *testing.T) {
	s := start(t, "")
	s.loader.Day(t, "testdata/today.json")

	page := s.open("/")
	assert.ElementsMatch(t, []string{
		"ghostty-org/ghostty", "anthropics/claude-code", "ollama/ollama", "astral-sh/uv", "denoland/deno",
		"rust-lang/rust", "tailscale/tailscale", "zed-industries/zed", "caddyserver/caddy", "charmbracelet/bubbletea",
	}, page.keys("shown"), "a card per repository")
	assert.Equal(t, 1, strings.Count(strings.ToLower(page.html), `data-repo="ghostty-org/ghostty"`))

	// trendshift/daily writes ghostty-org/ghostty, github/daily
	// Ghostty-Org/Ghostty.
	card := page.card(t, "ghostty-org/ghostty")
	assert.Equal(t, []string{"trendshift daily #1", "github daily #4"}, card.badges)
	// The name opens the reader; its address is the fallback without the
	// script, and it opens no new tab.
	assert.Contains(t, card.html, `href="https://github.com/ghostty-org/ghostty" data-action="read"`)
	assert.NotContains(t, card.html, `target="_blank"`)

	assert.Equal(t, []string{"trendshift weekly #3", "trendshift monthly #1", "github monthly #1"},
		page.card(t, "rust-lang/rust").badges)

	// What one list does not say comes from another one.
	assert.Contains(t, card.html, "A fast, native, feature-rich terminal emulator.")
	assert.Contains(t, card.html, `<p class="kicker">Zig <i>·</i> in 2 lists</p>`)

	// The mark of either spelling hides the one card.
	require.Equal(t, http.StatusOK, s.mark("skip", "GHOSTTY-ORG/Ghostty", "").Code)
	assert.NotContains(t, s.open("/").keys("shown"), "ghostty-org/ghostty")
}

// Opening the page marks nothing and loads a list at most once an hour;
// "Refresh" loads it now; the hour holds across a restart, and a failed
// attempt counts for it too.
func TestOpeningMarksNothingAndLoadsOnceAnHour(t *testing.T) {
	s := start(t, "")
	s.loader.Day(t, "testdata/yesterday.json")
	calls := func() []int {
		var calls []int
		for _, list := range trend.Lists() {
			calls = append(calls, s.loader.Calls(list))
		}
		return calls
	}
	each := func(n int) []int { return []int{n, n, n, n, n, n} }

	first := s.open("/")
	assert.Equal(t, each(1), calls(), "the first page loads every list")
	assert.Len(t, first.keys("shown"), 10)

	// Opening the page, the hidden view and the export mark nothing.
	s.open("/")
	s.open("/?hidden=1")
	require.Equal(t, http.StatusOK, s.get("/api/bookmarks").Code)
	require.Equal(t, http.StatusOK, s.get("/api/bookmarks?format=jsonl").Code)
	assert.Equal(t, each(1), calls())
	assert.Empty(t, s.store.Marks())
	assert.NoFileExists(t, filepath.Join(s.dir, store.MarksFile))
	assert.Len(t, s.open("/").keys("shown"), 10)

	s.clock.Advance(59 * time.Minute)
	s.open("/")
	assert.Equal(t, each(1), calls(), "within the hour nothing is loaded")

	// The hour is counted across a restart.
	s.restart()
	s.open("/")
	assert.Equal(t, each(1), calls(), "a restart within the hour loads nothing")

	s.clock.Advance(2 * time.Minute)
	s.open("/")
	assert.Equal(t, each(2), calls(), "after the hour the page loads again")

	// "Refresh" loads all six now, and marks nothing.
	response := s.post("/api/refresh", struct{}{})
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	assert.Equal(t, each(3), calls())
	s.open("/")
	assert.Equal(t, each(3), calls(), "the page after a refresh finds the lists fresh")
	assert.Empty(t, s.store.Marks())

	// A failed attempt counts for the hour, across a restart too.
	s.clock.Advance(61 * time.Minute)
	s.loader.Fail(githubDaily, errors.New("github/daily: the site answered 503 Service Unavailable"))
	failed := s.open("/")
	assert.Equal(t, each(4), calls())
	assert.Contains(t, failed.list(t, "github/daily"), "is-failed")

	s.clock.Advance(30 * time.Minute)
	s.open("/")
	s.restart()
	again := s.open("/")
	assert.Equal(t, each(4), calls(), "a failed list is not asked again within the hour")
	assert.Contains(t, again.list(t, "github/daily"), "Failed 10:17: the site answered 503 Service Unavailable")

	s.clock.Advance(31 * time.Minute)
	s.loader.Day(t, "testdata/yesterday.json")
	recovered := s.open("/")
	assert.Equal(t, each(5), calls())
	assert.NotContains(t, recovered.list(t, "github/daily"), "is-failed")
	assert.Empty(t, s.store.Marks())
}

// A failed or empty list is an error of that list, the others are shown,
// and the page does not say "nothing new".
func TestFailedAndEmptyListsAreErrors(t *testing.T) {
	s := start(t, "")
	s.loader.Day(t, "testdata/yesterday.json")
	s.loader.Fail(githubWeekly, errors.New("github/weekly: the page shows another period: Date range: Today"))
	s.loader.Set(githubMonthly)

	page := s.open("/")
	// The four lists that loaded are shown: golang/go and the others of
	// github/monthly are in them, junegunn/fzf too.
	assert.Len(t, page.keys("shown"), 10)
	assert.False(t, page.saysNothingNew())

	failed := page.list(t, "github/weekly")
	assert.Contains(t, failed, "is-failed")
	assert.Contains(t, failed, "is-missing")
	assert.Contains(t, failed, "no snapshot yet")
	assert.Contains(t, failed, "Failed 08:15: the page shows another period: Date range: Today")

	empty := page.list(t, "github/monthly")
	assert.Contains(t, empty, "is-failed")
	assert.Contains(t, empty, "Failed 08:15: the list has no repositories")

	loaded := page.list(t, "github/daily")
	assert.NotContains(t, loaded, "is-failed")
	assert.Contains(t, loaded, "08:15 · 3 repositories")

	// With everything seen the page still does not say "nothing new": it
	// names how many lists failed.
	require.Equal(t, http.StatusOK, s.seenAll(page).Code)
	seen := s.open("/")
	assert.Empty(t, seen.keys("shown"))
	assert.False(t, seen.saysNothingNew())
	assert.Contains(t, seen.html, `data-empty="partial"`)
	assert.Contains(t, seen.html, "No new repositories in the lists that loaded")
	assert.Contains(t, seen.html, "2 lists of 6 failed to load")

	t.Run("the last good snapshot of a failed list stays in the view", func(t *testing.T) {
		s := start(t, "")
		s.loader.Day(t, "testdata/yesterday.json")
		require.Len(t, s.open("/").keys("shown"), 10)

		s.clock.Advance(24 * time.Hour)
		s.loader.Day(t, "testdata/today.json")
		s.loader.Fail(trendshiftDaily, errors.New("trendshift/daily: no answer in 25s"))
		page := s.open("/")

		// golang/go is in yesterday's trendshift/daily only: today's did
		// not load, so its ghostty-org/ghostty is known from github/daily.
		assert.Contains(t, page.keys("shown"), "golang/go")
		assert.Equal(t, []string{"github daily #4"}, page.card(t, "ghostty-org/ghostty").badges)
		status := page.list(t, "trendshift/daily")
		assert.Contains(t, status, "is-failed")
		assert.Contains(t, status, "Nov 18, 08:15 · 5 repositories")
		assert.Contains(t, status, "Failed 08:15: no answer in 25s. Showing the snapshot of Nov 18, 2026, 08:15.")
		assert.False(t, page.saysNothingNew())
	})

	t.Run("without any snapshot the page shows the errors, not an empty success", func(t *testing.T) {
		s := start(t, "")
		page := s.open("/")
		assert.Empty(t, page.cards)
		assert.False(t, page.saysNothingNew())
		assert.Contains(t, page.html, `data-empty="failed"`)
		assert.Contains(t, page.html, "No list could be loaded")
		for _, list := range trend.Lists() {
			assert.Contains(t, page.list(t, list.String()), "is-failed", list)
		}
	})

	t.Run("when every list loaded and all is seen the page says nothing new", func(t *testing.T) {
		s := start(t, "")
		s.loader.Day(t, "testdata/yesterday.json")
		require.Equal(t, http.StatusOK, s.seenAll(s.open("/")).Code)
		page := s.open("/")
		assert.Empty(t, page.keys("shown"))
		assert.True(t, page.saysNothingNew())
		assert.Contains(t, page.html, `data-empty="clear"`)
	})
}

// When the outcome of a load cannot be recorded, it still holds for the
// running service: a list that failed is shown as failed, nothing is loaded
// again within the hour, and the page does not say "nothing new" as if the
// state of the lists were kept.
func TestUnrecordedOutcomeOfALoadStillHolds(t *testing.T) {
	s := start(t, "")
	s.loader.Day(t, "testdata/yesterday.json")
	require.Equal(t, http.StatusOK, s.seenAll(s.open("/")).Code)
	recorded := s.open("/")
	require.True(t, recorded.saysNothingNew())
	require.Equal(t, 6, s.loader.Total())

	// A directory in the place of the file: no attempt can be appended.
	attempts := filepath.Join(s.dir, store.AttemptsFile)
	kept := filepath.Join(s.dir, "attempts.kept")
	require.NoError(t, os.Rename(attempts, kept))
	require.NoError(t, os.Mkdir(attempts, 0o700))

	t.Run("a list that failed is shown as failed", func(t *testing.T) {
		s.clock.Advance(61 * time.Minute)
		s.loader.Fail(githubDaily, errors.New("github/daily: the site answered 503 Service Unavailable"))

		page := s.open("/")
		assert.Equal(t, 12, s.loader.Total())
		status := page.list(t, "github/daily")
		assert.Contains(t, status, "is-failed")
		assert.Contains(t, status, "Failed 09:16: the site answered 503 Service Unavailable")
		assert.Contains(t, page.html, `<p class="problem" role="alert">The outcome of the last load could not be recorded`)
		assert.False(t, page.saysNothingNew())
		assert.Contains(t, page.html, `data-empty="partial"`)

		// A second page within the hour asks no site again, and still
		// shows the failed list and the problem.
		s.clock.Advance(10 * time.Minute)
		again := s.open("/")
		assert.Equal(t, 12, s.loader.Total())
		assert.Contains(t, again.list(t, "github/daily"), "Failed 09:16: the site answered 503 Service Unavailable")
		assert.Contains(t, again.html, `<p class="problem" role="alert">`)
		assert.False(t, again.saysNothingNew())
	})

	t.Run("with every list loaded the page still does not say plain nothing new", func(t *testing.T) {
		s.loader.Day(t, "testdata/yesterday.json")
		response := s.post("/api/refresh", struct{}{})
		require.Equal(t, http.StatusOK, response.Code, response.Body.String())
		assert.Contains(t, response.Body.String(), "could not be recorded")
		assert.Contains(t, response.Body.String(), `"failed":0`)
		assert.Equal(t, 18, s.loader.Total())

		page := s.open("/")
		assert.Equal(t, 18, s.loader.Total())
		assert.NotContains(t, page.list(t, "github/daily"), "is-failed")
		assert.Empty(t, page.keys("shown"))
		assert.False(t, page.saysNothingNew())
		assert.Contains(t, page.html, `data-empty="unrecorded"`)
		assert.Contains(t, page.html, "No new repositories, but the state of the lists could not be recorded")
		assert.Contains(t, page.html, `<p class="problem" role="alert">`)

		// What an action answers for the place of the cards says the same.
		response = s.post("/api/seen", map[string][]string{"repos": {}})
		require.Equal(t, http.StatusOK, response.Code)
		assert.Contains(t, response.Body.String(), `data-empty=\"unrecorded\"`)
		assert.NotContains(t, strings.ToLower(response.Body.String()), "nothing new")
	})

	t.Run("once the file can be written the state is recorded", func(t *testing.T) {
		require.NoError(t, os.Remove(attempts))
		require.NoError(t, os.Rename(kept, attempts))

		page := s.open("/")
		assert.Equal(t, 18, s.loader.Total())
		assert.NotContains(t, page.html, `class="problem"`)
		assert.True(t, page.saysNothingNew())
		assert.Contains(t, page.html, `data-empty="clear"`)

		s.restart()
		restarted := s.open("/")
		assert.Equal(t, 18, s.loader.Total(), "the hour holds across the restart")
		assert.True(t, restarted.saysNothingNew())
	})
}

// "Interesting" with a comment creates a bookmark that survives a restart
// and is exported as a URL line and as JSONL with the comment.
func TestInterestingCreatesBookmark(t *testing.T) {
	s := start(t, "")
	s.loader.Day(t, "testdata/today.json")
	require.Contains(t, s.open("/").keys("shown"), "ghostty-org/ghostty")

	response := s.mark("interesting", "ghostty-org/ghostty", "  try for\nthe hub ")
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	var answer struct {
		New, Hidden int
		Card        string
	}
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &answer))
	assert.Equal(t, 9, answer.New)
	assert.Equal(t, 1, answer.Hidden)
	assert.Contains(t, answer.Card, `data-repo="ghostty-org/ghostty" data-state="hidden"`)

	s.clock.Advance(10 * time.Minute)
	require.Equal(t, http.StatusOK, s.mark("interesting", "zed-industries/zed", "").Code)

	export := func() (urls, jsonl string) {
		t.Helper()
		response := s.get("/api/bookmarks")
		require.Equal(t, http.StatusOK, response.Code)
		assert.Equal(t, "text/plain; charset=utf-8", response.Header().Get("Content-Type"))
		urls = response.Body.String()
		response = s.get("/api/bookmarks?format=jsonl")
		require.Equal(t, http.StatusOK, response.Code)
		return urls, response.Body.String()
	}
	wantURLs := "https://github.com/ghostty-org/ghostty\nhttps://github.com/zed-industries/zed\n"
	wantJSONL := `{"url":"https://github.com/ghostty-org/ghostty","repo":"ghostty-org/ghostty","comment":"try for the hub","bookmarked_at":"2026-11-18T08:15:00Z","lists":["trendshift/daily","github/daily"]}` + "\n" +
		`{"url":"https://github.com/zed-industries/zed","repo":"zed-industries/zed","comment":"","bookmarked_at":"2026-11-18T08:25:00Z","lists":["trendshift/weekly","github/weekly"]}` + "\n"

	urls, jsonl := export()
	assert.Equal(t, wantURLs, urls)
	assert.Equal(t, wantJSONL, jsonl)

	s.restart()
	urls, jsonl = export()
	assert.Equal(t, wantURLs, urls, "the bookmarks survive a restart")
	assert.Equal(t, wantJSONL, jsonl)

	page := s.open("/?hidden=1")
	assert.NotContains(t, page.keys("shown"), "ghostty-org/ghostty")
	card := page.card(t, "ghostty-org/ghostty")
	assert.Equal(t, "hidden", card.state)
	assert.Equal(t, "Marked as interesting on Nov 18, 2026, 08:15", card.reasons["seen"])
	assert.Equal(t, "Bookmarked in pulse: try for the hub", card.reasons["bookmark"])
	assert.Contains(t, card.html, `name="comment" value="try for the hub"`)
	assert.Equal(t, "Bookmarked in pulse, without a comment", page.card(t, "zed-industries/zed").reasons["bookmark"])

	t.Run("interesting again replaces the comment and keeps the place", func(t *testing.T) {
		s.clock.Advance(time.Hour)
		require.Equal(t, http.StatusOK, s.mark("interesting", "Ghostty-Org/Ghostty", "the terminal for the hub").Code)
		urls, jsonl := export()
		assert.Equal(t, wantURLs, urls)
		assert.Equal(t, strings.Replace(wantJSONL, "try for the hub", "the terminal for the hub", 1), jsonl)
	})

	t.Run("a skipped repository can be rescued from the hidden view", func(t *testing.T) {
		require.Equal(t, http.StatusOK, s.mark("skip", "ollama/ollama", "ignored").Code)
		urls, _ := export()
		assert.Equal(t, wantURLs, urls, "a skip is not a bookmark")

		card := s.open("/?hidden=1").card(t, "ollama/ollama")
		assert.Equal(t, map[string]string{"seen": "Skipped on Nov 18, 2026, 09:25"}, card.reasons)
		assert.Contains(t, card.html, `data-action="interesting"`)
		assert.NotContains(t, card.html, `data-action="skip"`)

		require.Equal(t, http.StatusOK, s.mark("interesting", "ollama/ollama", "after all").Code)
		urls, _ = export()
		assert.Equal(t, wantURLs+"https://github.com/ollama/ollama\n", urls)
	})

	t.Run("an unknown format is refused", func(t *testing.T) {
		assert.Equal(t, http.StatusBadRequest, s.get("/api/bookmarks?format=csv").Code)
	})
}

// When the record of "Interesting" cannot be written, the action fails, no
// bookmark is exported and the repository is still shown.
func TestInterestingFailsWhenTheRecordCannotBeWritten(t *testing.T) {
	s := start(t, "")
	s.loader.Day(t, "testdata/today.json")
	before := s.open("/")
	require.Contains(t, before.keys("shown"), "ghostty-org/ghostty")

	// A directory in the place of the file: no record can be appended.
	marks := filepath.Join(s.dir, store.MarksFile)
	require.NoError(t, os.Mkdir(marks, 0o700))

	response := s.mark("interesting", "ghostty-org/ghostty", "try for the hub")
	assert.Equal(t, http.StatusInternalServerError, response.Code)
	assert.Contains(t, response.Body.String(), "the mark could not be written")

	assert.Empty(t, s.get("/api/bookmarks").Body.String())
	assert.Empty(t, s.get("/api/bookmarks?format=jsonl").Body.String())
	after := s.open("/")
	assert.Equal(t, before.keys("shown"), after.keys("shown"))
	assert.Equal(t, "0", after.count(t, "hidden"))

	// "Skip" and "Mark all shown as seen" fail the same way.
	assert.Equal(t, http.StatusInternalServerError, s.mark("skip", "ghostty-org/ghostty", "").Code)
	assert.Equal(t, http.StatusInternalServerError, s.seenAll(after).Code)
	assert.Equal(t, before.keys("shown"), s.open("/").keys("shown"))

	// Nothing reached the disk: once the file can be written, a restart
	// finds no mark, and the action works.
	require.NoError(t, os.Remove(marks))
	s.restart()
	assert.Empty(t, s.store.Marks())
	assert.Equal(t, before.keys("shown"), s.open("/").keys("shown"))
	require.Equal(t, http.StatusOK, s.mark("interesting", "ghostty-org/ghostty", "try for the hub").Code)
	assert.Equal(t, "https://github.com/ghostty-org/ghostty\n", s.get("/api/bookmarks").Body.String())
}

// A request for another host, and a request of another site, are rejected.
func TestForeignHostAndCrossSiteOriginAreRejected(t *testing.T) {
	s := start(t, "")
	s.loader.Day(t, "testdata/today.json")
	body := `{"action":"interesting","repo":"ghostty-org/ghostty","comment":""}`

	t.Run("host", func(t *testing.T) {
		for _, host := range []string{
			"pulse.localhost:17842", "pulse.localhost", "PULSE.localhost:8080", "localhost:17842",
			"127.0.0.1:17842", "[::1]:17842", "[::1]",
		} {
			request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", http.NoBody)
			request.Host = host
			assert.Equal(t, http.StatusOK, s.do(request).Code, host)
		}
		for _, host := range []string{
			"evil.example", "evil.example:17842", "pulse.localhost.evil.example:17842", "192.168.1.10:17842",
			"pulse.local:17842", "",
		} {
			for _, path := range []string{"/", "/api/bookmarks", "/assets/app.js"} {
				request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, http.NoBody)
				request.Host = host
				assert.Equal(t, http.StatusMisdirectedRequest, s.do(request).Code, host+path)
			}
			request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/marks", strings.NewReader(body))
			request.Host = host
			request.Header.Set("Content-Type", "application/json")
			assert.Equal(t, http.StatusMisdirectedRequest, s.do(request).Code, host)
		}
		assert.Zero(t, s.loader.Total()-len(trend.Lists()), "a rejected request loads nothing")
	})

	t.Run("origin", func(t *testing.T) {
		for _, from := range []string{
			"https://evil.example", "http://evil.example:17842", "http://pulse.localhost.evil.example:17842",
			"http://localhost:3000", "https://pulse.localhost:17842", "null",
		} {
			for _, path := range []string{"/api/marks", "/api/seen", "/api/refresh"} {
				request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, origin+path, strings.NewReader(body))
				request.Header.Set("Content-Type", "application/json")
				request.Header.Set("Origin", from)
				assert.Equal(t, http.StatusForbidden, s.do(request).Code, from+path)
			}
		}
	})

	t.Run("content type", func(t *testing.T) {
		// What a form of another site can send without asking first.
		for _, media := range []string{"", "text/plain", "application/x-www-form-urlencoded", "multipart/form-data; boundary=x"} {
			request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, origin+"/api/marks", strings.NewReader(body))
			if media != "" {
				request.Header.Set("Content-Type", media)
			}
			assert.Equal(t, http.StatusUnsupportedMediaType, s.do(request).Code, media)
		}
	})

	t.Run("method", func(t *testing.T) {
		for _, method := range []string{http.MethodOptions, http.MethodPut, http.MethodDelete} {
			request := httptest.NewRequestWithContext(t.Context(), method, origin+"/api/marks", http.NoBody)
			response := s.do(request)
			assert.Equal(t, http.StatusMethodNotAllowed, response.Code, method)
			assert.Empty(t, response.Header().Get("Access-Control-Allow-Origin"))
		}
		request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, origin+"/api/marks", http.NoBody)
		assert.Equal(t, http.StatusMethodNotAllowed, s.do(request).Code, "a GET never marks")
	})

	assert.Empty(t, s.store.Marks(), "nothing rejected was marked")

	t.Run("a request without an Origin, as pulsectl sends it, is served", func(t *testing.T) {
		request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, origin+"/api/marks", strings.NewReader(body))
		request.Header.Set("Content-Type", "application/json; charset=utf-8")
		assert.Equal(t, http.StatusOK, s.do(request).Code)
	})
}

func TestMarkRequestsAreChecked(t *testing.T) {
	s := start(t, "")
	s.loader.Day(t, "testdata/today.json")
	s.open("/")

	for name, body := range map[string]any{
		"unknown action":  map[string]string{"action": "seen", "repo": "ollama/ollama"},
		"no action":       map[string]string{"repo": "ollama/ollama"},
		"no repository":   map[string]string{"action": "skip"},
		"an owner":        map[string]string{"action": "skip", "repo": "ollama"},
		"a deeper path":   map[string]string{"action": "skip", "repo": "ollama/ollama/tree"},
		"a line break":    map[string]string{"action": "interesting", "repo": "ollama/ollama\nhttps://evil.example"},
		"a long comment":  map[string]string{"action": "interesting", "repo": "ollama/ollama", "comment": strings.Repeat("x", 2001)},
		"an unknown key":  map[string]string{"action": "skip", "repo": "ollama/ollama", "note": "x"},
		"not an object":   []string{"skip"},
		"a wrong type":    map[string]any{"action": "skip", "repo": 42},
		"seen: bad repo":  nil,
		"seen: not names": nil,
	} {
		t.Run(name, func(t *testing.T) {
			switch name {
			case "seen: bad repo":
				assert.Equal(t, http.StatusBadRequest, s.post("/api/seen", map[string][]string{"repos": {"ollama/ollama", "ollama"}}).Code)
			case "seen: not names":
				assert.Equal(t, http.StatusBadRequest, s.post("/api/seen", map[string]string{"repos": "ollama/ollama"}).Code)
			default:
				assert.Equal(t, http.StatusBadRequest, s.post("/api/marks", body).Code)
			}
		})
	}
	assert.Empty(t, s.store.Marks())
}

func TestPageAndAssets(t *testing.T) {
	s := start(t, "")
	s.loader.Day(t, "testdata/today.json")

	response := s.get("/")
	require.Equal(t, http.StatusOK, response.Code)
	assert.Equal(t, "text/html; charset=utf-8", response.Header().Get("Content-Type"))
	assert.Contains(t, response.Header().Get("Content-Security-Policy"), "default-src 'none'")
	body := response.Body.String()

	// Everything the page needs is served by pulse itself.
	assert.NotRegexp(t, `(src|href)="(https?:)?//`+`[^"]*"[^>]*(stylesheet|script)`, body)
	assert.Contains(t, body, `<link rel="stylesheet" href="/assets/app.css">`)
	assert.Contains(t, body, `<script src="/assets/app.js" defer></script>`)
	assert.NotContains(t, body, "<style")
	assert.NotContains(t, body, "onclick=")

	// The hooks of the controls.
	for _, hook := range []string{
		`data-action="refresh"`, `data-action="toggle-hidden"`, `data-action="seen-all"`,
		`data-action="interesting"`, `data-action="skip"`, `name="comment"`,
	} {
		assert.Contains(t, body, hook)
	}
	assert.Contains(t, body, `aria-checked="false" data-action="toggle-hidden" href="/?hidden=1"`)
	assert.Contains(t, s.get("/?hidden=1").Body.String(), `aria-checked="true" data-action="toggle-hidden" href="/"`)

	for path, media := range map[string]string{
		"/assets/app.css":     "text/css; charset=utf-8",
		"/assets/app.js":      "text/javascript; charset=utf-8",
		"/assets/favicon.svg": "image/svg+xml",
	} {
		response := s.get(path)
		assert.Equal(t, http.StatusOK, response.Code, path)
		assert.Equal(t, media, response.Header().Get("Content-Type"), path)
		assert.NotEmpty(t, response.Body.String(), path)
		assert.NotContains(t, strings.ToLower(response.Body.String()), "nothing new", path)
	}
	for _, path := range []string{"/assets/page.html", "/assets/", "/unknown", "/api/unknown", "/templates/page.html"} {
		assert.Equal(t, http.StatusNotFound, s.get(path).Code, path)
	}
}

// What a list writes is text on the page, never markup.
func TestPageEscapesWhatTheListsSay(t *testing.T) {
	s := start(t, "")
	entries := watchtest.Entries(t, "owner/name")
	entries[0].Description = `<script>alert(1)</script> & "quotes"`
	entries[0].Language = `<b>Go</b>`
	for _, list := range trend.Lists() {
		s.loader.Set(list, entries...)
	}
	s.loader.Fail(githubDaily, errors.New(`<img src=x onerror=alert(1)>`))

	body := s.get("/").Body.String()
	assert.NotContains(t, body, "<script>alert")
	assert.NotContains(t, body, "<img")
	assert.NotContains(t, body, "<b>Go</b>")
	assert.Contains(t, body, "&lt;script&gt;alert(1)&lt;/script&gt;")
}

func TestListenRefusesWhatIsNotLoopback(t *testing.T) {
	for _, address := range []string{"0.0.0.0:0", "[::]:0", ":0", "192.0.2.1:0", "localhost:0"} {
		listeners, err := Listen(t.Context(), []string{"127.0.0.1:0", address}, true, func(string, error) {})
		assert.Error(t, err, address)
		assert.Empty(t, listeners, address)
	}

	listeners, err := Listen(t.Context(), []string{"127.0.0.1:0"}, false, func(string, error) {})
	require.NoError(t, err)
	require.Len(t, listeners, 1)
	assert.Equal(t, "http://pulse.localhost:"+strings.TrimPrefix(listeners[0].Addr().String(), "127.0.0.1:")+"/", URLFor(listeners[0].Addr()))
	require.NoError(t, listeners[0].Close())
}
