package web_test

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.octolab.org/toolset/pulse/internal/store"
	"go.octolab.org/toolset/pulse/internal/trend"
)

// threeDays loads the lists on three days: twice on the first, the second
// time with another answer; the third day without github/daily. The clock
// stops on the fourth day.
func threeDays(s *service) {
	s.t.Helper()
	s.loadAt(time.Date(2026, time.November, 16, 7, 0, 0, 0, time.UTC), map[trend.List][]string{
		trendshiftDaily: {"early/bird"},
		githubDaily:     {"early/bird"},
	})
	s.loadAt(time.Date(2026, time.November, 16, 19, 0, 0, 0, time.UTC), map[trend.List][]string{
		trendshiftDaily: {"first/one", "both/days"},
		githubDaily:     {"first/two"},
	})
	s.loadAt(time.Date(2026, time.November, 17, 9, 0, 0, 0, time.UTC), map[trend.List][]string{
		trendshiftDaily: {"both/days", "second/one"},
		githubDaily:     {"second/two"},
	})
	s.loadAt(time.Date(2026, time.November, 18, 9, 0, 0, 0, time.UTC), map[trend.List][]string{
		trendshiftDaily: {"third/one"},
		githubWeekly:    {"third/weekly"},
	})
	s.clock.Set(time.Date(2026, time.November, 19, 9, 0, 0, 0, time.UTC))
}

// Each day shows exactly its own last snapshots; a list without a snapshot
// that day is named as missing for the day, not as an error.
func TestEachDayShowsItsOwnLastSnapshots(t *testing.T) {
	s := start(t, "")
	threeDays(s)
	loads := s.loader.Total()

	first := s.open("/days/2026-11-16/")
	assert.ElementsMatch(t, []string{"first/one", "both/days", "first/two"}, first.keys("shown"),
		"the last snapshot of the day, not the first")
	assert.Equal(t, "2026-11-16", first.day())
	assert.Contains(t, first.list(t, "trendshift/daily"), "19:00")
	assert.Contains(t, first.html, "Monday, November 16, 2026")

	second := s.open("/days/2026-11-17/")
	assert.ElementsMatch(t, []string{"both/days", "second/one", "second/two"}, second.keys("shown"))

	third := s.open("/days/2026-11-18/")
	assert.ElementsMatch(t, []string{"third/one", "third/weekly"}, third.keys("shown"))
	assert.Equal(t, "list is-missing | github daily no snapshot that day", third.list(t, "github/daily"))
	assert.NotContains(t, third.html, "is-failed", "a missing list is not an error of the day")
	assert.NotContains(t, third.html, "Failed")

	assert.Equal(t, loads, s.loader.Total(), "a past day loads nothing")
}

// Snapshots taken at 23:30 and at 00:30 local time are on two days, whatever
// the UTC date of their files.
func TestDaysAreLocal(t *testing.T) {
	zone := time.FixedZone("UTC+3", 3*60*60)
	s := startWith(t, "", func(s *service) { s.location = zone })

	// 20:30 and 21:30 UTC on November 18 are 23:30 on November 18 and 00:30
	// on November 19 in UTC+3.
	s.loadAt(time.Date(2026, time.November, 18, 20, 30, 0, 0, time.UTC), map[trend.List][]string{
		trendshiftDaily: {"late/evening"},
	})
	s.loadAt(time.Date(2026, time.November, 18, 21, 30, 0, 0, time.UTC), map[trend.List][]string{
		trendshiftDaily: {"after/midnight"},
	})
	s.clock.Set(time.Date(2026, time.November, 20, 9, 0, 0, 0, time.UTC))

	assert.Equal(t, []string{"late/evening"}, s.open("/days/2026-11-18/").keys("shown"))
	assert.Equal(t, []string{"after/midnight"}, s.open("/days/2026-11-19/").keys("shown"))
	assert.Equal(t, []string{"/days/2026-11-18/", "/days/2026-11-19/"}, s.open("/days/2026-11-18/").calendar())
}

// "Show hidden" is a path for today and for a day; the old query parameter
// is ignored.
func TestHiddenIsAPath(t *testing.T) {
	s := start(t, "")
	threeDays(s)
	require.Equal(t, http.StatusOK, s.markOn("2026-11-17", "skip", "second/one", "").Code)

	// Today is November 19: its page loads the lists, and shows the latest
	// snapshot of each.
	today := s.open("/hidden/")
	assert.ElementsMatch(t, []string{"third/one", "second/two", "third/weekly"}, today.keys("shown"))

	require.Equal(t, http.StatusOK, s.mark("skip", "third/one", "").Code)
	today = s.open("/hidden/")
	assert.ElementsMatch(t, []string{"second/two", "third/weekly"}, today.keys("shown"))
	assert.Equal(t, []string{"third/one"}, today.keys("hidden"))
	assert.Equal(t, "Skipped on Nov 19, 2026, until Feb 19, 2027", today.card(t, "third/one").reasons["seen"])
	assert.Contains(t, today.html, `aria-checked="true" data-action="toggle-hidden" href="/"`)

	day := s.open("/days/2026-11-17/hidden/")
	assert.ElementsMatch(t, []string{"both/days", "second/two"}, day.keys("shown"))
	assert.Equal(t, []string{"second/one"}, day.keys("hidden"))
	assert.Equal(t, "Skipped on Nov 19, 2026, until Feb 19, 2027", day.card(t, "second/one").reasons["seen"])
	assert.Contains(t, day.html, `aria-checked="true" data-action="toggle-hidden" href="/days/2026-11-17/"`)
	assert.Contains(t, s.open("/days/2026-11-17/").html, `aria-checked="false" data-action="toggle-hidden" href="/days/2026-11-17/hidden/"`)

	old := s.open("/?hidden=1")
	assert.ElementsMatch(t, []string{"second/two", "third/weekly"}, old.keys("shown"))
	assert.Empty(t, old.keys("hidden"), "the query parameter is ignored")
	assert.Contains(t, old.html, `data-action="toggle-hidden" href="/hidden/"`)
}

// A past day is judged by what was done since: a repository marked today is
// hidden on yesterday's page and shown with its reason under "Show hidden".
func TestAPastDayFollowsTheMarksOfToday(t *testing.T) {
	s := start(t, "")
	threeDays(s)
	require.Equal(t, http.StatusOK, s.mark("skip", "both/days", "").Code)

	yesterday := s.open("/days/2026-11-17/")
	assert.ElementsMatch(t, []string{"second/one", "second/two"}, yesterday.keys("shown"))
	hidden := s.open("/days/2026-11-17/hidden/")
	assert.Equal(t, map[string]string{"seen": "Skipped on Nov 19, 2026, until Feb 19, 2027"},
		hidden.card(t, "both/days").reasons)
}

// "Mark all shown as seen" on a past day marks exactly what that page
// showed, with the lists of that day, and the day shows nothing new a month
// later.
func TestSeenAllOnAPastDay(t *testing.T) {
	s := start(t, "")
	threeDays(s)
	before := len(s.store.Marks())

	day := s.open("/days/2026-11-16/")
	require.ElementsMatch(t, []string{"first/one", "both/days", "first/two"}, day.keys("shown"))
	response := s.seenAll(day)
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	assert.Contains(t, response.Body.String(), `"new":0`)

	at := time.Date(2026, time.November, 19, 9, 0, 0, 0, time.UTC)
	marks := s.store.Marks()[before:]
	assert.ElementsMatch(t, []store.Mark{
		{Time: at, Repo: trend.Repo{Owner: "first", Name: "one"}, Action: store.Seen, Lists: []string{"trendshift/daily"}},
		{Time: at, Repo: trend.Repo{Owner: "both", Name: "days"}, Action: store.Seen, Lists: []string{"trendshift/daily"}},
		{Time: at, Repo: trend.Repo{Owner: "first", Name: "two"}, Action: store.Seen, Lists: []string{"github/daily"}},
	}, marks, "the lists of November 16, not of today")

	assert.ElementsMatch(t, []string{"second/one", "second/two"}, s.open("/days/2026-11-17/").keys("shown"),
		"both/days is seen on every day")

	s.clock.Advance(31 * 24 * time.Hour)
	s.restart()
	again := s.open("/days/2026-11-16/")
	assert.Empty(t, again.keys("shown"))
	assert.Equal(t, "3", again.count(t, "hidden"))
}

// "Interesting" and "Skip" on a past day record the lists of that day.
func TestMarkOnAPastDayNamesItsLists(t *testing.T) {
	s := start(t, "")
	threeDays(s)
	require.Equal(t, http.StatusOK, s.markOn("2026-11-16", "interesting", "both/days", "from monday").Code)
	marks := s.store.Marks()
	assert.Equal(t, []string{"trendshift/daily"}, marks[len(marks)-1].Lists)
	assert.Equal(t, "from monday", marks[len(marks)-1].Comment)

	response := s.markOn("2026-11-17", "skip", "second/two", "")
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	assert.Contains(t, response.Body.String(), `"new":1`, "the counts are those of that day")
	marks = s.store.Marks()
	assert.Equal(t, []string{"github/daily"}, marks[len(marks)-1].Lists)

	for _, day := range []string{"2026-11-15", "nonsense"} {
		assert.Equal(t, http.StatusBadRequest, s.markOn(day, "skip", "second/one", "").Code, day)
	}
}

// Opening a past day, in any mode, and its reader make no request to the
// sources, and offer no Refresh.
func TestAPastDayLoadsNothing(t *testing.T) {
	s := start(t, "")
	threeDays(s)
	loads := s.loader.Total()
	for _, path := range []string{
		"/days/2026-11-17/", "/days/2026-11-17/hidden/", "/days/2026-11-17/repos/second/one",
	} {
		response := s.get(path)
		require.Equal(t, http.StatusOK, response.Code, path)
		assert.NotContains(t, response.Body.String(), `data-action="refresh"`, path)
	}
	assert.Equal(t, loads, s.loader.Total())
	assert.Contains(t, s.get("/").Body.String(), `data-action="refresh"`, "today offers Refresh")
}

// The calendar offers exactly the days that have snapshots; previous and
// next step between them. A malformed date, or a day without snapshots,
// answers 404 with a link to today; today's date is today's page.
func TestCalendar(t *testing.T) {
	s := start(t, "")
	threeDays(s)

	// Today's page loads the lists: November 19 has snapshots too.
	today := s.open("/")
	assert.Equal(t, []string{"/days/2026-11-16/", "/days/2026-11-17/", "/days/2026-11-18/", "/days/2026-11-19/"}, today.calendar())
	assert.Contains(t, today.html, `data-day-step="prev" href="/days/2026-11-18/"`)
	assert.NotContains(t, today.html, `data-day-step="next"`)

	middle := s.open("/days/2026-11-17/hidden/")
	assert.Contains(t, middle.html, `data-day-step="prev" href="/days/2026-11-16/hidden/"`)
	assert.Contains(t, middle.html, `data-day-step="next" href="/days/2026-11-18/hidden/"`)
	assert.Contains(t, middle.html, `<td class="is-current"><a href="/days/2026-11-17/" data-calendar-day>17</a></td>`)
	assert.Contains(t, middle.html, `<a href="/">Back to today</a>`)
	assert.NotContains(t, s.open("/days/2026-11-16/").html, `data-day-step="prev"`, "the first day has no previous one")

	same := s.open("/days/2026-11-19/")
	assert.Equal(t, today.keys("shown"), same.keys("shown"), "the path of today is today's page")
	assert.Equal(t, "2026-11-19", same.day(), "its marks are made on its day, which is today")
	assert.Contains(t, same.html, `data-action="refresh"`)

	for _, path := range []string{
		"/days/2026-11-15/", "/days/2026-11-20/hidden/", "/days/2026-11-1/", "/days/2026-13-01/",
		"/days/yesterday/", "/days/2026-11-15/repos/second/one",
	} {
		response := s.get(path)
		assert.Equal(t, http.StatusNotFound, response.Code, path)
		assert.Contains(t, response.Body.String(), `href="/">Go to today</a>`, path)
	}
}

// Today without a snapshot yet is still today's page, and its calendar is
// empty.
func TestTodayWithoutSnapshots(t *testing.T) {
	s := start(t, "")
	page := s.open("/")
	assert.Empty(t, page.calendar())
	assert.Contains(t, page.html, "No snapshots yet.")
	assert.Equal(t, http.StatusOK, s.get("/days/2026-11-18/").Code, "today's date")
	assert.Equal(t, http.StatusNotFound, s.get("/days/2026-11-17/").Code)
}

// A day is a calendar date even where the clocks skip its midnight: its page
// opens, and the calendar links every day once.
func TestDaysWhereMidnightIsSkipped(t *testing.T) {
	for zone, gap := range map[string]time.Time{
		// Chile moves its clocks from 00:00 to 01:00 on September 6, 2026.
		"America/Santiago": time.Date(2026, time.September, 6, 0, 0, 0, 0, time.UTC),
		// Lebanon moves them from 00:00 to 01:00 on March 29, 2026.
		"Asia/Beirut": time.Date(2026, time.March, 29, 0, 0, 0, 0, time.UTC),
	} {
		t.Run(zone, func(t *testing.T) {
			loc, err := time.LoadLocation(zone)
			if err != nil {
				t.Skipf("no time zone %s: %v", zone, err)
			}
			s := startWith(t, "", func(s *service) { s.location = loc })
			var want []string
			for i := -1; i <= 1; i++ {
				day := gap.AddDate(0, 0, i)
				noon := time.Date(day.Year(), day.Month(), day.Day(), 12, 0, 0, 0, loc)
				s.loadAt(noon, map[trend.List][]string{trendshiftDaily: {"day/" + day.Format("02")}})
				want = append(want, "/days/"+day.Format("2006-01-02")+"/")
			}
			s.clock.Set(gap.AddDate(0, 0, 3).Add(12 * time.Hour))

			for i, path := range want {
				page := s.open(path)
				assert.Equal(t, []string{"day/" + gap.AddDate(0, 0, i-1).Format("02")}, page.keys("shown"), path)
				assert.Equal(t, want, page.calendar(), path)
			}
		})
	}
}

// A mark made after midnight on a page rendered before it is made on the
// day the page showed, with the lists of that day.
func TestTodaysPageKeepsItsDayAfterMidnight(t *testing.T) {
	s := start(t, "")
	threeDays(s)
	page := s.open("/")
	require.Equal(t, "2026-11-19", page.day())
	require.Contains(t, page.keys("shown"), "third/one")

	// After midnight another tab loads lists without third/one.
	s.loadAt(time.Date(2026, time.November, 20, 0, 30, 0, 0, time.UTC), map[trend.List][]string{
		githubMonthly: {"next/day"},
	})
	response := s.seenAll(page)
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	for _, mark := range s.store.Marks() {
		if mark.Repo.Key() == "third/one" {
			assert.Equal(t, []string{"trendshift/daily"}, mark.Lists, "the lists of November 19")
		}
	}
}

// Today's page shows the latest snapshot of each list, which may be of an
// earlier day. Left open past midnight, its marks name what it showed: the
// lists of those snapshots, and a date without its own snapshots is no
// error.
func TestTodaysPageOfOlderSnapshotsAfterMidnight(t *testing.T) {
	s := start(t, "")
	s.loadAt(time.Date(2026, time.November, 20, 9, 0, 0, 0, time.UTC), map[trend.List][]string{
		trendshiftDaily: {"a/b"},
		githubDaily:     {"e/f"},
	})
	// On November 21 only trendshift/daily loads, with another repository.
	s.loadAt(time.Date(2026, time.November, 21, 9, 0, 0, 0, time.UTC), map[trend.List][]string{
		trendshiftDaily: {"c/d"},
	})
	page := s.open("/")
	require.Equal(t, "2026-11-21", page.day())
	require.ElementsMatch(t, []string{"c/d", "e/f"}, page.keys("shown"))

	s.clock.Set(time.Date(2026, time.November, 22, 0, 10, 0, 0, time.UTC))
	response := s.post("/api/marks", map[string]any{"action": "skip", "repo": "e/f", "day": "2026-11-21", "latest": true})
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	marks := s.store.Marks()
	assert.Equal(t, []string{"github/daily"}, marks[len(marks)-1].Lists, "the list of November 20 the page showed")
	assert.Contains(t, response.Body.String(), `"new":1`, "the counts of the page as it stood")

	// A day on which every list failed: the page showed November 20.
	s.loadAt(time.Date(2026, time.November, 22, 9, 0, 0, 0, time.UTC), map[trend.List][]string{})
	s.clock.Set(time.Date(2026, time.November, 23, 0, 10, 0, 0, time.UTC))
	response = s.post("/api/seen", map[string]any{"repos": []string{"c/d"}, "day": "2026-11-22", "latest": true})
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	var answer struct {
		New   int
		Empty string
	}
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &answer))
	assert.Zero(t, answer.New)
	assert.Contains(t, answer.Empty, `data-empty="over"`, "every list failed that day: no unqualified claim")
	assert.NotContains(t, answer.Empty, "Nothing new")
	marks = s.store.Marks()
	assert.Equal(t, []string{"trendshift/daily"}, marks[len(marks)-1].Lists)

	assert.Equal(t, http.StatusBadRequest,
		s.post("/api/marks", map[string]any{"action": "skip", "repo": "a/b", "day": "2026-11-30", "latest": true}).Code,
		"a date after today")
}
