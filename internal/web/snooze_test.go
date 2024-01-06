package web_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.octolab.org/toolset/pulse/internal/config"
)

// at moves the clock of the service and opens today's page with the hidden
// cards: the lists answer as before, so the repository trends again.
func (s *service) at(when time.Time) page {
	s.t.Helper()
	s.clock.Set(when)
	return s.open("/hidden/")
}

const day = 24 * time.Hour

// A skip hides a repository for three calendar months: the day before the
// end it is hidden, the day after it is shown again.
func TestSkipSnoozesForAQuarter(t *testing.T) {
	s := start(t, "")
	s.loader.Day(t, "testdata/yesterday.json")
	require.Contains(t, s.open("/").keys("shown"), "tailscale/tailscale")
	require.Equal(t, http.StatusOK, s.mark("skip", "tailscale/tailscale", "").Code)

	end := time.Date(2027, time.February, 18, 8, 15, 0, 0, time.UTC)
	before := s.at(end.Add(-day))
	assert.Contains(t, before.keys("hidden"), "tailscale/tailscale")
	assert.Equal(t, "Skipped on Nov 18, 2026, until Feb 18, 2027", before.card(t, "tailscale/tailscale").reasons["seen"])

	after := s.at(end.Add(day))
	assert.Contains(t, after.keys("shown"), "tailscale/tailscale")
	assert.Empty(t, after.card(t, "tailscale/tailscale").reasons, "an ended snooze is no reason")
}

// "Mark all shown as seen" snoozes as "Skip" does.
func TestSeenAllSnoozesForAQuarter(t *testing.T) {
	s := start(t, "")
	s.loader.Day(t, "testdata/yesterday.json")
	shown := s.open("/")
	require.Len(t, shown.keys("shown"), 10)
	require.Equal(t, http.StatusOK, s.seenAll(shown).Code)

	end := time.Date(2027, time.February, 18, 8, 15, 0, 0, time.UTC)
	before := s.at(end.Add(-day))
	assert.Empty(t, before.keys("shown"))
	assert.Equal(t, "Marked as seen on Nov 18, 2026, until Feb 18, 2027", before.card(t, "golang/go").reasons["seen"])

	after := s.at(end.Add(day))
	assert.Len(t, after.keys("shown"), 10)

	// The marks of the snoozes that ended do not keep a new one from being
	// written.
	require.Equal(t, http.StatusOK, s.seenAll(s.open("/")).Code)
	assert.Empty(t, s.open("/").keys("shown"))
}

// "Interesting" is a bookmark, and a bookmark hides for good.
func TestInterestingNeverExpires(t *testing.T) {
	s := start(t, "")
	s.loader.Day(t, "testdata/yesterday.json")
	require.Equal(t, http.StatusOK, s.mark("interesting", "golang/go", "keep").Code)

	later := s.at(morning.AddDate(1, 0, 1))
	assert.Contains(t, later.keys("hidden"), "golang/go")
	assert.Equal(t, map[string]string{
		"seen":     "Marked as interesting on Nov 18, 2026, 08:15",
		"bookmark": "Bookmarked in pulse: keep",
	}, later.card(t, "golang/go").reasons)
}

// A repository in the vault stays hidden after its skip ended, with the
// vault as the reason.
func TestVaultOutlastsASnooze(t *testing.T) {
	s := start(t, "testdata/vault")
	s.loader.Day(t, "testdata/trending-again.json")
	s.open("/")
	require.Equal(t, http.StatusOK, s.mark("skip", "astral-sh/uv", "").Code)

	page := s.at(morning.AddDate(0, 3, 1))
	assert.Contains(t, page.keys("hidden"), "astral-sh/uv")
	assert.Equal(t, map[string]string{"vault": "In the vault: uv.md"}, page.card(t, "astral-sh/uv").reasons)
}

// A skip after the snooze ended snoozes the repository for a new period from
// that mark; the earlier mark stays in the log.
func TestSkipAgainStartsANewPeriod(t *testing.T) {
	s := start(t, "")
	s.loader.Day(t, "testdata/yesterday.json")
	s.open("/")
	require.Equal(t, http.StatusOK, s.mark("skip", "tailscale/tailscale", "").Code)

	second := time.Date(2027, time.February, 19, 10, 0, 0, 0, time.UTC)
	require.Contains(t, s.at(second).keys("shown"), "tailscale/tailscale")
	require.Equal(t, http.StatusOK, s.mark("skip", "tailscale/tailscale", "").Code)
	assert.Len(t, s.store.Marks(), 2)

	before := s.at(time.Date(2027, time.May, 18, 10, 0, 0, 0, time.UTC))
	assert.Equal(t, "Skipped on Feb 19, 2027, until May 19, 2027", before.card(t, "tailscale/tailscale").reasons["seen"])
	assert.Contains(t, s.at(time.Date(2027, time.May, 20, 10, 0, 0, 0, time.UTC)).keys("shown"), "tailscale/tailscale")
}

// A period of the configuration replaces the quarter.
func TestConfiguredSnooze(t *testing.T) {
	s := startWith(t, "", func(s *service) { s.snooze = config.Period{Months: 1} })
	s.loader.Day(t, "testdata/yesterday.json")
	s.open("/")
	require.Equal(t, http.StatusOK, s.mark("skip", "tailscale/tailscale", "").Code)

	before := s.at(time.Date(2026, time.December, 17, 8, 15, 0, 0, time.UTC))
	assert.Equal(t, "Skipped on Nov 18, 2026, until Dec 18, 2026", before.card(t, "tailscale/tailscale").reasons["seen"])
	assert.Contains(t, s.at(time.Date(2026, time.December, 19, 8, 15, 0, 0, time.UTC)).keys("shown"), "tailscale/tailscale")
}

// A past day is judged by the marks as they stand now: a snooze that ended
// shows the repository on that day too.
func TestAnEndedSnoozeShowsOnAPastDay(t *testing.T) {
	s := start(t, "")
	s.loader.Day(t, "testdata/yesterday.json")
	s.open("/")
	require.Equal(t, http.StatusOK, s.mark("skip", "tailscale/tailscale", "").Code)
	s.clock.Set(time.Date(2026, time.November, 25, 8, 15, 0, 0, time.UTC))
	assert.NotContains(t, s.open("/days/2026-11-18/").keys("shown"), "tailscale/tailscale")

	s.clock.Set(time.Date(2027, time.March, 1, 8, 15, 0, 0, time.UTC))
	assert.Contains(t, s.open("/days/2026-11-18/").keys("shown"), "tailscale/tailscale")
}

// The snooze is counted on the calendar of the computer, and a month without
// the day of the mark ends on its last day.
func TestSnoozeOnTheLocalCalendar(t *testing.T) {
	zone := time.FixedZone("UTC+3", 3*60*60)
	s := startWith(t, "", func(s *service) { s.location = zone })
	s.loader.Day(t, "testdata/yesterday.json")
	// 00:30 on December 1 in UTC+3 is still November 30 in UTC.
	s.clock.Set(time.Date(2026, time.November, 30, 21, 30, 0, 0, time.UTC))
	s.open("/")
	require.Equal(t, http.StatusOK, s.mark("skip", "tailscale/tailscale", "").Code)
	assert.Equal(t, "Skipped on Dec 1, 2026, until Mar 1, 2027",
		s.open("/hidden/").card(t, "tailscale/tailscale").reasons["seen"])

	clamp := startWith(t, "", nil)
	clamp.loader.Day(t, "testdata/yesterday.json")
	clamp.clock.Set(time.Date(2026, time.November, 30, 9, 0, 0, 0, time.UTC))
	clamp.open("/")
	require.Equal(t, http.StatusOK, clamp.mark("skip", "golang/go", "").Code)
	assert.Equal(t, "Skipped on Nov 30, 2026, until Feb 28, 2027",
		clamp.open("/hidden/").card(t, "golang/go").reasons["seen"])
}
