package web_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.octolab.org/toolset/pulse/internal/hide"
)

// fakeSource answers as the test says: it hides the repositories it holds
// with its own reasons, or fails.
type fakeSource struct {
	holds map[string]string
	fail  string
}

func (f fakeSource) Check(_ context.Context, items []hide.Item) ([][]hide.Reason, hide.Status) {
	status := hide.Status{Source: "stars", Provider: "fake", Origin: "https://stars.example/", Entries: len(f.holds)}
	if f.fail != "" {
		status.State, status.Error = hide.Failed, f.fail
		return nil, status
	}
	status.State = hide.OK
	answer := make([][]hide.Reason, len(items))
	for i, item := range items {
		if ref, found := f.holds[item.Repo.Key()]; found {
			answer[i] = []hide.Reason{{Source: "stars", Label: "Starred", Ref: ref}}
		}
	}
	return answer, status
}

// A source swapped in behind the facade hides its repositories with its own
// reasons; the page needs nothing else.
func TestAnotherSourceHidesWithItsReasons(t *testing.T) {
	s := startWith(t, "", func(s *service) {
		s.hide = hide.Facade{fakeSource{holds: map[string]string{"golang/go": "starred in 2019"}}}
	})
	s.loader.Day(t, "testdata/yesterday.json")

	page := s.open("/hidden/")
	assert.Equal(t, []string{"golang/go"}, page.keys("hidden"))
	assert.Equal(t, map[string]string{"stars": "Starred: starred in 2019"}, page.card(t, "golang/go").reasons)
	assert.Contains(t, page.html, `data-source="stars" data-vault="ok">Stars: 1 repository`)
	assert.Contains(t, page.html, "https://stars.example/")
}

// A source that fails leaves its repositories shown; the page names the
// source and the error, and never says "nothing new" without that
// reservation.
func TestAFailedSourceFailsOpen(t *testing.T) {
	s := startWith(t, "", func(s *service) {
		s.hide = hide.Facade{fakeSource{fail: "connection refused", holds: map[string]string{"golang/go": "x"}}}
	})
	s.loader.Day(t, "testdata/yesterday.json")

	page := s.open("/")
	assert.Len(t, page.keys("shown"), 10)
	assert.Contains(t, page.html, "The stars could not be read from")
	assert.Contains(t, page.html, "connection refused. The list may include what is already in the stars.")

	response := s.seenAll(page)
	require.Equal(t, http.StatusOK, response.Code)
	var answer struct{ Empty string }
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &answer))
	assert.Contains(t, answer.Empty, `data-empty="unchecked"`)
	assert.NotContains(t, answer.Empty, "Nothing new")

	empty := s.open("/")
	assert.False(t, empty.saysNothingNew())
	assert.Contains(t, empty.html, "No new repositories, but the stars could not be read")
}

// The page names the vault with its origin, its count and its newest note,
// so a wrong or stale path shows at a glance.
func TestTheVaultLineNamesOriginCountAndNewestNote(t *testing.T) {
	notes := t.TempDir()
	write := func(name, content string, at time.Time) {
		path := filepath.Join(notes, name)
		require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
		require.NoError(t, os.Chtimes(path, at, at))
	}
	write("go.md", "---\nurl: https://github.com/golang/go\n---\n", time.Date(2026, time.November, 1, 9, 0, 0, 0, time.UTC))
	write("uv.md", "---\naliases: [astral-sh/uv]\n---\n", time.Date(2026, time.November, 17, 12, 5, 0, 0, time.UTC))

	s := start(t, notes)
	s.loader.Day(t, "testdata/yesterday.json")
	page := s.open("/hidden/")
	_, line, found := strings.Cut(page.html, `data-vault="ok"`)
	require.True(t, found)
	line = text(line)
	assert.Contains(t, line, "Vault: 2 repositories in 2 notes · "+notes+" · newest note Nov 17, 2026, 12:05.")
	assert.Equal(t, map[string]string{"vault": "In the vault: go.md"}, page.card(t, "golang/go").reasons,
		"the reasons of a hidden card are those of before")

	// Under the home directory, the path starts with ~.
	s.home = filepath.Dir(notes)
	s.restart()
	assert.Contains(t, s.open("/").html, `data-origin>~`+string(filepath.Separator)+filepath.Base(notes)+`</span>`)
}

// The page warns, without failing, when the vault directory is not inside a
// vault Obsidian knows.
func TestObsidianWarning(t *testing.T) {
	root := t.TempDir()
	vault := filepath.Join(root, "Mindset")
	bookmarks := filepath.Join(vault, "Bookmarks")
	copied := filepath.Join(root, "Mindset copy", "Bookmarks")
	for _, dir := range []string{bookmarks, copied} {
		require.NoError(t, os.MkdirAll(dir, 0o700))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "go.md"), []byte("---\nurl: https://github.com/golang/go\n---\n"), 0o600))
	}
	list := filepath.Join(root, "obsidian.json")
	known, err := json.Marshal(map[string]any{"vaults": map[string]any{"a1": map[string]any{"path": vault, "ts": 1, "open": true}}})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(list, known, 0o600))

	inside := startWith(t, bookmarks, func(s *service) { s.obsidian = list })
	inside.loader.Day(t, "testdata/yesterday.json")
	page := inside.open("/")
	assert.NotContains(t, page.html, "data-warning")
	assert.Contains(t, page.html, `data-vault="ok"`)

	outside := startWith(t, copied, func(s *service) { s.obsidian = list })
	outside.loader.Day(t, "testdata/yesterday.json")
	page = outside.open("/hidden/")
	assert.Contains(t, page.html, "data-warning>Vault: the directory is not inside any vault Obsidian knows on this computer")
	assert.Contains(t, page.html, `data-vault="ok"`, "a warning is not a failure")
	assert.Contains(t, page.keys("hidden"), "golang/go")
}

// A failed list and a failed source both stay in the reservation of the
// empty page.
func TestEmptyPageKeepsEveryReservation(t *testing.T) {
	s := startWith(t, "", func(s *service) { s.hide = hide.Facade{fakeSource{fail: "connection refused"}} })
	s.loader.Day(t, "testdata/yesterday.json")
	require.Equal(t, http.StatusOK, s.seenAll(s.open("/")).Code)
	s.clock.Advance(2 * time.Hour)
	s.loader.Fail(githubDaily, errors.New("the site is down"))

	page := s.open("/")
	require.Empty(t, page.keys("shown"))
	assert.Contains(t, page.html, `data-empty="partial"`)
	assert.Contains(t, page.html, "1 list of 6 failed to load")
	assert.Contains(t, page.html, "The list may also include what is already in the stars, which could not be read.")
}

// Flow mode has no status of the lists, so it says what may be missing: a
// failed source, a failed list, a warning; and a flow without cards says
// why, as the page does.
func TestFlowKeepsTheReservations(t *testing.T) {
	s := startWith(t, "", func(s *service) { s.hide = hide.Facade{fakeSource{fail: "connection refused"}} })
	s.loader.Day(t, "testdata/yesterday.json")
	s.loader.Fail(githubDaily, errors.New("the site is down"))
	body := s.getFlow("/flow/")
	assert.Contains(t, body, "The stars could not be read: connection refused. The cards may include what is already in the stars.")
	assert.Contains(t, body, "1 list of 6 failed to load, so this is not the whole picture.")

	failed := start(t, "")
	body = failed.getFlow("/flow/")
	assert.Empty(t, flow(body))
	assert.Contains(t, body, `data-empty="failed"><h2>No list could be loaded</h2>`)
	assert.NotContains(t, body, "Nothing new")
}
