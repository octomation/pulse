// Package watch is the logic of trend-watch: it loads the lists, keeps the
// hour between loads, and joins the snapshots with the marks and the
// sources of the hide facade, such as the vault, into the view the page
// shows.
package watch

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"go.octolab.org/toolset/pulse/internal/config"
	"go.octolab.org/toolset/pulse/internal/hide"
	"go.octolab.org/toolset/pulse/internal/store"
	"go.octolab.org/toolset/pulse/internal/trend"
)

const (
	// Interval is how old the last attempt to load a list has to be before
	// opening the page loads the list again.
	Interval = time.Hour
	// Timeout is how long a list may take to load.
	Timeout = 30 * time.Second

	// MaxComment is the longest comment of a bookmark, in bytes.
	MaxComment = 2000
	// maxReason bounds the reason of a failed load in attempts.jsonl.
	maxReason = 500
)

// Loader loads a list from its source.
type Loader interface {
	Load(ctx context.Context, list trend.List) (trend.Snapshot, error)
}

// Options wire a Service.
type Options struct {
	// Store keeps the snapshots, the marks and the attempts.
	Store *store.Store
	// Loader loads the lists.
	Loader Loader
	// Hide asks the sources that may hide a card, such as the vault; none
	// hides nothing.
	Hide hide.Facade
	// Snooze is how long "Skip" and "Mark all shown as seen" hide a
	// repository; zero is config.DefaultSnooze.
	Snooze config.Period
	// Location is the time zone of the calendar the snooze is counted in;
	// nil is local.
	Location *time.Location
	// Now is the clock; nil is time.Now.
	Now func() time.Time
	// Timeout bounds the load of a list; zero is Timeout.
	Timeout time.Duration
}

// Service is trend-watch.
type Service struct {
	store    *store.Store
	loader   Loader
	hide     hide.Facade
	snooze   config.Period
	location *time.Location
	now      func() time.Time
	timeout  time.Duration

	// loading serializes the loads: a second page waits for the first and
	// then finds the lists fresh.
	loading sync.Mutex

	// state guards unrecorded and problem.
	state sync.Mutex
	// unrecorded holds the attempts attempts.jsonl could not take, the last
	// of each list: they hold for the running process, and go to the file
	// with the next write.
	unrecorded []store.Attempt
	// problem says why they are not in the file.
	problem string
}

// New returns the service.
func New(o Options) *Service {
	s := &Service{store: o.Store, loader: o.Loader, hide: o.Hide, snooze: o.Snooze, now: o.Now, timeout: o.Timeout}
	if s.snooze == (config.Period{}) {
		s.snooze = config.DefaultSnooze
	}
	if s.location = o.Location; s.location == nil {
		s.location = time.Local
	}
	if s.now == nil {
		s.now = time.Now
	}
	if s.timeout <= 0 {
		s.timeout = Timeout
	}
	return s
}

// clock returns the time of a record: UTC, to the second.
func (s *Service) clock() time.Time { return s.now().UTC().Truncate(time.Second) }

// Snooze returns how long "Skip" and "Mark all shown as seen" hide a
// repository.
func (s *Service) Snooze() config.Period { return s.snooze }

// Open loads the lists whose last attempt, successful or not, is more than
// an hour old or was never made, waits for them, and returns the view. It
// marks nothing.
func (s *Service) Open(ctx context.Context) (View, error) {
	if _, err := s.load(ctx, s.stale); err != nil {
		return View{}, err
	}
	return s.View(ctx), nil
}

// Refresh loads all six lists now. The outcome of each list is in the view;
// the string it returns says what could not be recorded, as View.Problem
// does.
func (s *Service) Refresh(ctx context.Context) (string, error) {
	return s.load(ctx, func(trend.List, time.Time) bool { return true })
}

// stale reports whether the list is due for a load.
func (s *Service) stale(list trend.List, now time.Time) bool {
	last, found := s.lastAttempt(list)
	return !found || now.Sub(last.Time) > Interval || last.Time.After(now)
}

// lastAttempt returns the last attempt to load the list: the one the file
// could not take, when there is one, else the last recorded one.
func (s *Service) lastAttempt(list trend.List) (store.Attempt, bool) {
	s.state.Lock()
	for _, attempt := range s.unrecorded {
		if attempt.List == list.String() {
			s.state.Unlock()
			return attempt, true
		}
	}
	s.state.Unlock()
	return s.store.LastAttempt(list)
}

// Problem says what the service could not record, if anything: the outcome
// of the loads then holds only until the service stops.
func (s *Service) Problem() string {
	s.state.Lock()
	defer s.state.Unlock()
	return s.problem
}

// record writes the attempts to attempts.jsonl, after the ones an earlier
// write left behind. When the file cannot be written, the attempts still
// hold for the running process: the page shows a list that failed as
// failed, and the hour between loads is kept.
func (s *Service) record(attempts []store.Attempt) string {
	s.state.Lock()
	defer s.state.Unlock()

	attempts = append(slices.Clone(s.unrecorded), attempts...)
	if err := s.store.AppendAttempts(attempts...); err != nil {
		// The last attempt of each list is all the service needs.
		var last []store.Attempt
		for i, attempt := range attempts {
			later := slices.ContainsFunc(attempts[i+1:], func(a store.Attempt) bool { return a.List == attempt.List })
			if !later {
				last = append(last, attempt)
			}
		}
		s.unrecorded = last
		s.problem = fmt.Sprintf("The outcome of the last load could not be recorded: %v. "+
			"It holds until pulse stops; after a restart the lists are loaded again.", err)
		return s.problem
	}
	s.unrecorded, s.problem = nil, ""
	return ""
}

// load loads, in parallel, the lists that due selects, and records a
// snapshot for every success and an attempt for every list that answered or
// failed. A list whose load failed because the context ended gets no
// record: the list did not fail, the wait was given up. The loads that
// succeeded by then are kept all the same, and the error of the context is
// returned after them.
func (s *Service) load(ctx context.Context, due func(trend.List, time.Time) bool) (problem string, err error) {
	s.loading.Lock()
	defer s.loading.Unlock()

	var lists []trend.List
	now := s.clock()
	for _, list := range trend.Lists() {
		if due(list, now) {
			lists = append(lists, list)
		}
	}
	if len(lists) == 0 {
		// Nothing to load, but an earlier outcome may still wait for the
		// file.
		if problem := s.Problem(); problem != "" {
			return s.record(nil), nil
		}
		return "", nil
	}

	type result struct {
		snapshot trend.Snapshot
		err      error
		// givenUp says that the context had ended when the load returned.
		givenUp bool
	}
	results := make([]result, len(lists))
	var group sync.WaitGroup
	for i, list := range lists {
		group.Go(func() {
			snapshot, err := s.loadList(ctx, list)
			results[i] = result{snapshot: snapshot, err: err, givenUp: ctx.Err() != nil}
		})
	}
	group.Wait()

	now = s.clock()
	attempts := make([]store.Attempt, 0, len(lists))
	for i, list := range lists {
		attempt := store.Attempt{Time: now, List: list.String(), OK: true}
		snapshot, err := results[i].snapshot, results[i].err
		if err != nil && results[i].givenUp {
			continue
		}
		if err == nil {
			if snapshot.FetchedAt.IsZero() {
				snapshot.FetchedAt = s.now().UTC()
			}
			if err = s.store.SaveSnapshot(snapshot); err != nil {
				err = fmt.Errorf("the snapshot could not be stored: %w", err)
			}
		}
		if err != nil {
			attempt.OK, attempt.Error = false, reason(list, err)
		}
		attempts = append(attempts, attempt)
	}
	problem = s.record(attempts)
	if err := ctx.Err(); err != nil {
		return "", err
	}
	return problem, nil
}

// loadList loads one list within the timeout and checks the answer: it has
// to be that list and hold a repository.
func (s *Service) loadList(ctx context.Context, list trend.List) (trend.Snapshot, error) {
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	snapshot, err := s.loader.Load(ctx, list)
	switch {
	case err != nil && errors.Is(ctx.Err(), context.DeadlineExceeded):
		return snapshot, fmt.Errorf("no answer in %s: %w", s.timeout, err)
	case err != nil:
		return snapshot, err
	case snapshot.List != list:
		return snapshot, fmt.Errorf("the answer is the list %s", snapshot.List)
	case len(snapshot.Entries) == 0:
		return snapshot, errors.New("the list has no repositories")
	}
	for _, entry := range snapshot.Entries {
		if entry.Repo.Owner == "" || entry.Repo.Name == "" {
			return snapshot, fmt.Errorf("the entry at position %d has no repository", entry.Rank)
		}
	}
	return snapshot, nil
}

// reason is why a load failed, as the page shows it next to the list: a
// loader that names the list first says it twice there.
func reason(list trend.List, err error) string {
	text := strings.TrimPrefix(err.Error(), list.String()+": ")
	if len(text) > maxReason {
		cut := maxReason
		for cut > 0 && text[cut]&0xC0 == 0x80 {
			cut--
		}
		text = text[:cut] + "…"
	}
	return text
}

// Mark records "Interesting" or "Skip" for a repository: "Interesting"
// bookmarks it with the comment, "Skip" snoozes it. The record is on disk
// when Mark returns nil; when it cannot be written, Mark fails and the
// repository stays as it was. It loads nothing.
func (s *Service) Mark(action store.Action, repo trend.Repo, comment string) error {
	switch {
	case action != store.Interesting && action != store.Skip:
		return fmt.Errorf("%w: the action is %q, want interesting or skip", ErrInvalid, action)
	case len(comment) > MaxComment:
		return fmt.Errorf("%w: the comment is longer than %d bytes", ErrInvalid, MaxComment)
	}
	if action == store.Skip {
		comment = ""
	}
	current := s.memberships()
	return s.store.AppendMarks(newMark(s.clock(), action, repo, comment, current))
}

// MarkSeen records "Mark all shown as seen" for exactly the repositories the
// page showed: one write for all of them. A repository that the marks hide
// already gets no second mark. It loads nothing and returns the number it
// marked.
func (s *Service) MarkSeen(repos []trend.Repo) (int, error) {
	now := s.clock()
	seen := map[string]bool{}
	for key := range s.marks(now).seen {
		seen[key] = true
	}
	current := s.memberships()
	var marks []store.Mark
	for _, repo := range repos {
		if seen[repo.Key()] {
			continue
		}
		seen[repo.Key()] = true
		marks = append(marks, newMark(now, store.Seen, repo, "", current))
	}
	if err := s.store.AppendMarks(marks...); err != nil {
		return 0, err
	}
	return len(marks), nil
}

// ErrInvalid marks a request the service cannot take as it is.
var ErrInvalid = errors.New("invalid request")

// Bookmarks returns the bookmarks, oldest first.
func (s *Service) Bookmarks() []store.Bookmark { return s.store.Bookmarks() }

// membership is where a repository is in the latest snapshots.
type membership struct {
	repo  trend.Repo
	lists []Membership
}

// memberships maps the key of every repository of the view to the lists it
// is in, in the order of trend.Lists.
func (s *Service) memberships() map[string]*membership {
	index := map[string]*membership{}
	for _, list := range trend.Lists() {
		snapshot, found := s.store.Latest(list)
		if !found {
			continue
		}
		for _, entry := range snapshot.Entries {
			key := entry.Repo.Key()
			if index[key] == nil {
				index[key] = &membership{repo: entry.Repo}
			}
			index[key].lists = append(index[key].lists, Membership{List: list, Rank: entry.Rank})
		}
	}
	return index
}

// newMark builds the record of a mark: the repository as the lists write it,
// when they have it, and the lists it is in.
func newMark(now time.Time, action store.Action, repo trend.Repo, comment string, current map[string]*membership) store.Mark {
	mark := store.Mark{Time: now, Repo: repo, Action: action, Comment: comment, Lists: []string{}}
	if known := current[repo.Key()]; known != nil {
		for _, m := range known.lists {
			mark.Lists = append(mark.Lists, m.List.String())
		}
	}
	return mark
}
