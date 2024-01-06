package watch

import (
	"cmp"
	"context"
	"slices"
	"time"

	"go.octolab.org/toolset/pulse/internal/hide"
	"go.octolab.org/toolset/pulse/internal/store"
	"go.octolab.org/toolset/pulse/internal/trend"
)

// View is what the page shows: the union of a snapshot of every list, a card
// per repository, and the state of the lists. Today's view joins the latest
// successful snapshots; the view of a past day joins the last snapshot of
// each list taken within that day.
type View struct {
	// Now is the time the view was built.
	Now time.Time
	// Day is the start of the past day the view shows; zero for today.
	Day time.Time
	// AsOf is the end of the day of a today's page that is over: the view
	// is that page as it stood, the latest snapshots before AsOf, without
	// the outcome of the loads of that day. Zero otherwise.
	AsOf time.Time
	// Lists are the six lists in the order of trend.Lists.
	Lists []ListStatus
	// Cards are the repositories, shown and hidden, in the order of their
	// first appearance in the lists.
	Cards []Card
	// Sources are the states of the sources that hide cards, such as the
	// vault, in the order of the facade.
	Sources []hide.Status
	// Problem says what the service could not record, if anything: the
	// state of the lists then holds only until the service stops.
	Problem string
}

// Past reports whether the view is of a past day.
func (v View) Past() bool { return !v.Day.IsZero() }

// ListStatus is the state of a list: its snapshot and its error.
type ListStatus struct {
	List trend.List
	// Loaded says whether the list has a successful snapshot: for a past
	// day, one taken that day.
	Loaded bool
	// FetchedAt is the time of that snapshot.
	FetchedAt time.Time
	// Count is the number of repositories of that snapshot.
	Count int
	// Failed says that the last attempt to load the list failed: the list
	// is in error until its next successful attempt. A past day has no
	// errors.
	Failed bool
	// Reason is why the last attempt failed.
	Reason string
	// FailedAt is the time of the failed attempt.
	FailedAt time.Time
}

// Membership is the position of a repository in a list.
type Membership struct {
	List trend.List
	Rank int
}

// SeenReason hides a repository that was marked. A bookmark hides it for
// good, since the first "Interesting"; "Skip" and "Mark all shown as seen"
// snooze it until the period of the last of them ends.
type SeenReason struct {
	At     time.Time
	Action store.Action
	// Until is the end of the snooze; zero for good.
	Until time.Time
}

// BookmarkReason is the pulse bookmark of a hidden repository.
type BookmarkReason struct {
	At      time.Time
	Comment string
}

// Card is a repository of the view.
type Card struct {
	// Repo is the repository as the most recent snapshot writes it.
	Repo        trend.Repo
	Description string
	Language    string
	// Lists are the lists the repository is in, with its position there.
	Lists []Membership

	// Seen, Bookmark and Reasons are the reasons to hide the card; a card
	// carries every reason that applies. Reasons are those of the sources
	// of the facade, such as the vault.
	Seen     *SeenReason
	Bookmark *BookmarkReason
	Reasons  []hide.Reason
}

// Key identifies the card: owner/name without case.
func (c Card) Key() string { return c.Repo.Key() }

// Hidden reports whether the page hides the card by default: the repository
// is bookmarked, snoozed, or held by a source such as the vault.
func (c Card) Hidden() bool { return c.Seen != nil || len(c.Reasons) > 0 }

// Shown returns the cards the page shows by default.
func (v View) Shown() []Card {
	var cards []Card
	for _, card := range v.Cards {
		if !card.Hidden() {
			cards = append(cards, card)
		}
	}
	return cards
}

// HiddenCount returns the number of hidden cards.
func (v View) HiddenCount() int { return len(v.Cards) - len(v.Shown()) }

// Card returns the card of a repository.
func (v View) Card(key string) (Card, bool) {
	for _, card := range v.Cards {
		if card.Key() == key {
			return card, true
		}
	}
	return Card{}, false
}

// FailedLists returns the number of lists in error.
func (v View) FailedLists() int {
	var failed int
	for _, list := range v.Lists {
		if list.Failed {
			failed++
		}
	}
	return failed
}

// LoadedLists returns the number of lists that have a snapshot.
func (v View) LoadedLists() int {
	var loaded int
	for _, list := range v.Lists {
		if list.Loaded {
			loaded++
		}
	}
	return loaded
}

// FailedSources returns the sources that did not answer: the view may
// include what is already in them.
func (v View) FailedSources() []hide.Status {
	var failed []hide.Status
	for _, source := range v.Sources {
		if source.State == hide.Failed {
			failed = append(failed, source)
		}
	}
	return failed
}

// View builds today's view from what is stored, and asks the sources again.
// It loads nothing and marks nothing.
func (s *Service) View(ctx context.Context) View {
	view := View{Now: s.now(), Problem: s.Problem()}
	var snapshots []trend.Snapshot
	for _, list := range trend.Lists() {
		status := ListStatus{List: list}
		if snapshot, found := s.store.Latest(list); found {
			status.Loaded, status.FetchedAt, status.Count = true, snapshot.FetchedAt, len(snapshot.Entries)
			snapshots = append(snapshots, snapshot)
		}
		if last, found := s.lastAttempt(list); found && !last.OK {
			status.Failed, status.Reason, status.FailedAt = true, last.Error, last.Time
		}
		view.Lists = append(view.Lists, status)
	}
	s.fill(ctx, &view, snapshots)
	return view
}

// Day builds the view of a past day, from the last snapshot of each list
// taken at from or later and before to. It loads nothing and marks nothing:
// what is hidden is decided by the marks and the sources as they are now.
func (s *Service) Day(ctx context.Context, from, to time.Time) (View, error) {
	view := View{Now: s.now(), Day: from}
	if from.IsZero() {
		view.AsOf = to
	}
	snapshots, err := s.snapshotsIn(from, to)
	if err != nil {
		return View{}, err
	}
	for _, list := range trend.Lists() {
		status := ListStatus{List: list}
		for _, snapshot := range snapshots {
			if snapshot.List == list {
				status.Loaded, status.FetchedAt, status.Count = true, snapshot.FetchedAt, len(snapshot.Entries)
			}
		}
		view.Lists = append(view.Lists, status)
	}
	s.fill(ctx, &view, snapshots)
	return view, nil
}

// snapshotsIn returns the last snapshot of every list taken at from or later
// and before to. A zero from has no lower bound; a zero span is today: the
// latest snapshot of every list.
func (s *Service) snapshotsIn(from, to time.Time) ([]trend.Snapshot, error) {
	var snapshots []trend.Snapshot
	for _, list := range trend.Lists() {
		var (
			snapshot trend.Snapshot
			found    bool
			err      error
		)
		if from.IsZero() && to.IsZero() {
			snapshot, found = s.store.Latest(list)
		} else if snapshot, found, err = s.store.LatestIn(list, from, to); err != nil {
			return nil, err
		}
		if found {
			snapshots = append(snapshots, snapshot)
		}
	}
	return snapshots, nil
}

// Days returns the start of every day, in loc, that has a snapshot, oldest
// first.
func (s *Service) Days(loc *time.Location) []time.Time {
	seen := map[time.Time]bool{}
	var days []time.Time
	for _, list := range trend.Lists() {
		for _, at := range s.store.Times(list) {
			day := StartOfDay(at, loc)
			if !seen[day] {
				seen[day] = true
				days = append(days, day)
			}
		}
	}
	slices.SortFunc(days, time.Time.Compare)
	return days
}

// StartOfDay returns the start of the calendar day of t in loc.
func StartOfDay(t time.Time, loc *time.Location) time.Time {
	t = t.In(loc)
	return DayStart(t.Year(), t.Month(), t.Day(), loc)
}

// DayStart returns the first instant of the calendar date in loc. Where the
// clocks skip midnight, the day starts when they land: its midnight does
// not exist, and time.Date would give an instant of the day before.
func DayStart(year int, month time.Month, day int, loc *time.Location) time.Time {
	// Normalize the date first, so that day 32 is the next month.
	noon := time.Date(year, month, day, 12, 0, 0, 0, loc)
	year, month, day = noon.Date()
	same := func(t time.Time) bool {
		y, m, d := t.Date()
		return y == year && m == month && d == day
	}
	for hour := 0; hour < 24; hour++ {
		start := time.Date(year, month, day, hour, 0, 0, 0, loc)
		if !same(start) {
			continue
		}
		// The jump may end within the hour.
		for earlier := start.Add(-time.Minute); same(earlier); earlier = earlier.Add(-time.Minute) {
			start = earlier
		}
		return start
	}
	return noon
}

// NextDay returns the start of the calendar day after the one that starts
// at day.
func NextDay(day time.Time, loc *time.Location) time.Time {
	y, m, d := day.In(loc).Date()
	return DayStart(y, m, d+1, loc)
}

// fill joins the snapshots into the cards of the view and gives each card
// its reasons to be hidden.
func (s *Service) fill(ctx context.Context, view *View, snapshots []trend.Snapshot) {
	// One card per key, in the order of the first appearance: the lists in
	// their order, each by rank.
	index := map[string]int{}
	for _, snapshot := range snapshots {
		entries := slices.Clone(snapshot.Entries)
		slices.SortStableFunc(entries, func(a, b trend.Entry) int { return cmp.Compare(a.Rank, b.Rank) })
		for _, entry := range entries {
			key := entry.Repo.Key()
			i, found := index[key]
			if !found {
				i = len(view.Cards)
				index[key] = i
				view.Cards = append(view.Cards, Card{})
			}
			view.Cards[i].Lists = append(view.Cards[i].Lists, Membership{List: snapshot.List, Rank: entry.Rank})
		}
	}
	// The name, the description and the language come from the most recent
	// snapshot that has them.
	slices.SortStableFunc(snapshots, func(a, b trend.Snapshot) int { return b.FetchedAt.Compare(a.FetchedAt) })
	for _, snapshot := range snapshots {
		for _, entry := range snapshot.Entries {
			card := &view.Cards[index[entry.Repo.Key()]]
			if card.Repo == (trend.Repo{}) {
				card.Repo = entry.Repo
			}
			if card.Description == "" {
				card.Description = entry.Description
			}
			if card.Language == "" {
				card.Language = entry.Language
			}
		}
	}

	marks := s.marks(view.Now)
	items := make([]hide.Item, len(view.Cards))
	for i, card := range view.Cards {
		items[i] = hide.Item{Repo: card.Repo}
	}
	reasons, statuses := s.hide.Check(ctx, items)
	view.Sources = statuses
	for i := range view.Cards {
		card := &view.Cards[i]
		card.Seen = marks.seen[card.Key()]
		card.Bookmark = marks.bookmarks[card.Key()]
		card.Reasons = reasons[i]
	}
}

// marked is what the marks say at a moment.
type marked struct {
	// seen holds the repositories the marks hide at that moment.
	seen map[string]*SeenReason
	// bookmarks holds the bookmarked repositories.
	bookmarks map[string]*BookmarkReason
}

// marks reads the marks as they stand at now. "Interesting" bookmarks a
// repository for good, with the comment of its last mark; "Skip" and "Mark
// all shown as seen" snooze it for the period from the last of them, and a
// snooze that ended hides nothing.
func (s *Service) marks(now time.Time) marked {
	m := marked{seen: map[string]*SeenReason{}, bookmarks: map[string]*BookmarkReason{}}
	last := map[string]store.Mark{}
	for _, mark := range s.store.Marks() {
		key := mark.Repo.Key()
		if mark.Action != store.Interesting {
			if previous, found := last[key]; !found || !mark.Time.Before(previous.Time) {
				last[key] = mark
			}
			continue
		}
		if m.bookmarks[key] == nil {
			m.bookmarks[key] = &BookmarkReason{At: mark.Time}
			m.seen[key] = &SeenReason{At: mark.Time, Action: mark.Action}
		}
		m.bookmarks[key].Comment = mark.Comment
	}
	for key, mark := range last {
		if m.seen[key] != nil {
			continue
		}
		if until := s.snooze.After(mark.Time, s.location); now.Before(until) {
			m.seen[key] = &SeenReason{At: mark.Time, Action: mark.Action, Until: until}
		}
	}
	return m
}
