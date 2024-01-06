package web_test

import (
	"bytes"
	"encoding/json"
	"html"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"go.octolab.org/toolset/pulse/internal/config"
	"go.octolab.org/toolset/pulse/internal/hide"
	"go.octolab.org/toolset/pulse/internal/store"
	"go.octolab.org/toolset/pulse/internal/vault"
	"go.octolab.org/toolset/pulse/internal/watch"
	"go.octolab.org/toolset/pulse/internal/watch/watchtest"
	. "go.octolab.org/toolset/pulse/internal/web"
)

// origin is where the browser opens the page.
const origin = "http://pulse.localhost:17842"

// morning is the time the tests start at.
var morning = time.Date(2026, time.November, 18, 8, 15, 0, 0, time.UTC)

// service is pulse on a temporary data directory, with a loader and a clock
// the test controls.
type service struct {
	t       *testing.T
	dir     string
	vault   string
	loader  *watchtest.Loader
	clock   *watchtest.Clock
	store   *store.Store
	handler http.Handler
	// github answers the reader; nil starts the service without it.
	github Details
	// obsidian is the list of Obsidian vaults; empty skips the check.
	obsidian string
	// snooze is how long a skip hides; zero is the default.
	snooze config.Period
	// location is the time zone of the calendar the snooze is counted in;
	// nil is UTC.
	location *time.Location
	// hide replaces the vault of the directory; nil keeps it.
	hide hide.Facade
	// home is the home directory the page writes as ~.
	home string
}

// start runs the service on an empty data directory. The vault directory
// may be empty: the filter is off then.
func start(t *testing.T, vault string) *service {
	t.Helper()
	return startWith(t, vault, nil)
}

// startWith runs the service as start does, after configure has set what
// the test needs.
func startWith(t *testing.T, vault string, configure func(*service)) *service {
	t.Helper()
	s := &service{
		t: t, dir: filepath.Join(t.TempDir(), "data"), vault: vault,
		loader: watchtest.NewLoader(), clock: watchtest.NewClock(morning),
	}
	if configure != nil {
		configure(s)
	}
	s.restart()
	return s
}

// restart drops everything the service holds in memory and opens the data
// directory again, as a new process would.
func (s *service) restart() {
	s.t.Helper()
	state, err := store.Open(s.dir)
	require.NoError(s.t, err)
	facade := s.hide
	if facade == nil {
		facade = hide.Facade{vault.Dir{Settings: vault.Settings{Bookmarks: s.vault, Obsidian: s.obsidian}}}
	}
	handler, err := NewHandler(Options{
		Service: watch.New(watch.Options{
			Store: state, Loader: s.loader, Now: s.clock.Now, Snooze: s.snooze, Hide: facade, Location: s.zone(),
		}),
		Details:  s.github,
		Location: s.zone(),
		Home:     s.home,
	})
	require.NoError(s.t, err)
	s.store, s.handler = state, handler
}

// zone is the time zone of the calendar the snooze is counted in.
func (s *service) zone() *time.Location {
	if s.location == nil {
		return time.UTC
	}
	return s.location
}

func (s *service) do(request *http.Request) *httptest.ResponseRecorder {
	s.t.Helper()
	recorder := httptest.NewRecorder()
	s.handler.ServeHTTP(recorder, request)
	return recorder
}

func (s *service) get(path string) *httptest.ResponseRecorder {
	s.t.Helper()
	return s.do(httptest.NewRequestWithContext(s.t.Context(), http.MethodGet, origin+path, http.NoBody))
}

// post sends JSON as the page does.
func (s *service) post(path string, body any) *httptest.ResponseRecorder {
	s.t.Helper()
	raw, err := json.Marshal(body)
	require.NoError(s.t, err)
	request := httptest.NewRequestWithContext(s.t.Context(), http.MethodPost, origin+path, bytes.NewReader(raw))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Origin", origin)
	return s.do(request)
}

// open renders the page and reads its cards.
func (s *service) open(path string) page {
	s.t.Helper()
	response := s.get(path)
	require.Equal(s.t, http.StatusOK, response.Code, response.Body.String())
	return parse(s.t, response.Body.String())
}

// mark clicks "Interesting" or "Skip" on a card.
func (s *service) mark(action, repo, comment string) *httptest.ResponseRecorder {
	s.t.Helper()
	return s.post("/api/marks", map[string]string{"action": action, "repo": repo, "comment": comment})
}

// seenAll clicks "Mark all shown as seen" on a rendered page: it sends
// exactly the cards the page shows.
func (s *service) seenAll(p page) *httptest.ResponseRecorder {
	s.t.Helper()
	return s.post("/api/seen", map[string][]string{"repos": p.names("shown")})
}

// page is what a rendered page shows.
type page struct {
	html  string
	cards []card
}

// card is a card as rendered.
type card struct {
	key, state, name string
	badges           []string
	reasons          map[string]string
	html             string
}

var (
	cardPattern   = regexp.MustCompile(`(?s)<article class="card[^"]*" data-repo="([^"]+)" data-state="([^"]+)" data-name="([^"]+)">(.*?)</article>`)
	badgePattern  = regexp.MustCompile(`(?s)<li class="badge[^"]*" data-list="[^"]+">(.*?)</li>`)
	reasonPattern = regexp.MustCompile(`(?s)<li class="reason" data-reason="([^"]+)">(.*?)</li>`)
	tagPattern    = regexp.MustCompile(`<[^>]+>`)
	countPattern  = regexp.MustCompile(`<b data-count="(new|hidden)">(\d+)</b>`)
)

func text(fragment string) string {
	return strings.Join(strings.Fields(html.UnescapeString(tagPattern.ReplaceAllString(fragment, ""))), " ")
}

func parse(t *testing.T, body string) page {
	t.Helper()
	p := page{html: body}
	for _, match := range cardPattern.FindAllStringSubmatch(body, -1) {
		c := card{
			key: html.UnescapeString(match[1]), state: match[2], name: html.UnescapeString(match[3]),
			reasons: map[string]string{}, html: match[4],
		}
		for _, badge := range badgePattern.FindAllStringSubmatch(match[4], -1) {
			c.badges = append(c.badges, text(badge[1]))
		}
		for _, reason := range reasonPattern.FindAllStringSubmatch(match[4], -1) {
			c.reasons[reason[1]] = text(reason[2])
		}
		p.cards = append(p.cards, c)
	}
	require.Equal(t, strings.Count(body, "<article"), len(p.cards), "every card of the page is read")
	return p
}

// keys returns the keys of the cards in a state, in the order of the page.
func (p page) keys(state string) []string {
	var keys []string
	for _, c := range p.cards {
		if c.state == state {
			keys = append(keys, c.key)
		}
	}
	return keys
}

// names returns the repositories of the cards in a state as the page writes
// them.
func (p page) names(state string) []string {
	var names []string
	for _, c := range p.cards {
		if c.state == state {
			names = append(names, c.name)
		}
	}
	return names
}

// card returns the card of a repository.
func (p page) card(t *testing.T, key string) card {
	t.Helper()
	for _, c := range p.cards {
		if c.key == key {
			return c
		}
	}
	t.Fatalf("the page has no card of %s", key)
	return card{}
}

// count returns a counter of the header: new or hidden.
func (p page) count(t *testing.T, name string) string {
	t.Helper()
	for _, match := range countPattern.FindAllStringSubmatch(p.html, -1) {
		if match[1] == name {
			return match[2]
		}
	}
	t.Fatalf("the page has no counter %q", name)
	return ""
}

// saysNothingNew reports whether the page says "Nothing new" anywhere.
func (p page) saysNothingNew() bool {
	return strings.Contains(strings.ToLower(p.html), "nothing new")
}

// list returns the status of a list as the header shows it.
func (p page) list(t *testing.T, name string) string {
	t.Helper()
	pattern := regexp.MustCompile(`(?s)<li class="([^"]*)" data-list="` + regexp.QuoteMeta(name) + `">(.*?)</li>`)
	match := pattern.FindStringSubmatch(p.html)
	require.NotNil(t, match, "the page has no list %s", name)
	return match[1] + " | " + text(match[2])
}
