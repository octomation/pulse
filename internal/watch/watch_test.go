package watch_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.octolab.org/toolset/pulse/internal/store"
	"go.octolab.org/toolset/pulse/internal/trend"
	. "go.octolab.org/toolset/pulse/internal/watch"
	"go.octolab.org/toolset/pulse/internal/watch/watchtest"
)

var (
	morning = time.Date(2026, time.November, 19, 8, 15, 0, 0, time.UTC)

	trendshiftDaily  = trend.List{Source: trend.Trendshift, Period: trend.Daily}
	trendshiftWeekly = trend.List{Source: trend.Trendshift, Period: trend.Weekly}
	githubDaily      = trend.List{Source: trend.GitHub, Period: trend.Daily}
)

type fixture struct {
	dir     string
	store   *store.Store
	loader  *watchtest.Loader
	clock   *watchtest.Clock
	service *Service
}

func start(t *testing.T) *fixture {
	t.Helper()
	f := &fixture{dir: t.TempDir(), loader: watchtest.NewLoader(), clock: watchtest.NewClock(morning)}
	state, err := store.Open(f.dir)
	require.NoError(t, err)
	f.store = state
	f.service = New(Options{Store: state, Loader: f.loader, Now: f.clock.Now})
	for _, list := range trend.Lists() {
		f.loader.Set(list, watchtest.Entries(t, "owner/"+string(list.Source)+"-"+string(list.Period))...)
	}
	return f
}

func keys(cards []Card) []string {
	names := make([]string, 0, len(cards))
	for _, card := range cards {
		names = append(names, card.Key())
	}
	return names
}

func TestViewOrderNamesAndMemberships(t *testing.T) {
	f := start(t)
	f.loader.Set(trendshiftDaily, watchtest.Entries(t, "b/two", "a/one")...)
	f.loader.Set(trendshiftWeekly, watchtest.Entries(t, "c/three", "A/One")...)
	described := watchtest.Entries(t, "C/Three", "d/four", "B/TWO")
	described[0].Description, described[0].Language = "the third", "Go"
	// A list need not come sorted: the rank orders it.
	described[1].Rank, described[2].Rank = 3, 2
	f.loader.Set(githubDaily, described...)

	view, err := f.service.Open(t.Context())
	require.NoError(t, err)
	assert.Empty(t, view.Problem)
	assert.Equal(t, morning, view.Now)

	cards := view.Shown()
	assert.Equal(t, []string{
		"b/two", "a/one", "c/three", "owner/trendshift-monthly", "d/four", "owner/github-weekly", "owner/github-monthly",
	}, keys(cards), "the lists in their order, each by rank, a repository once")
	assert.Zero(t, view.HiddenCount())

	two, found := view.Card("b/two")
	require.True(t, found)
	assert.Equal(t, []Membership{{List: trendshiftDaily, Rank: 1}, {List: githubDaily, Rank: 2}}, two.Lists)

	three, _ := view.Card("c/three")
	assert.Equal(t, []Membership{{List: trendshiftWeekly, Rank: 1}, {List: githubDaily, Rank: 1}}, three.Lists)
	assert.Equal(t, "the third", three.Description, "what one list does not say comes from another")
	assert.Equal(t, "Go", three.Language)

	assert.Equal(t, 6, view.LoadedLists())
	assert.Zero(t, view.FailedLists())
	for _, list := range view.Lists {
		assert.True(t, list.Loaded)
		assert.Equal(t, morning, list.FetchedAt)
		assert.False(t, list.Failed)
	}

	t.Run("the name is the one of the most recent snapshot", func(t *testing.T) {
		assert.Equal(t, "b/two", two.Repo.String(), "of the first list, when the snapshots are of one time")

		f.clock.Advance(2 * time.Hour)
		f.loader.Fail(trendshiftDaily, errors.New("down"))
		view, err := f.service.Open(t.Context())
		require.NoError(t, err)
		two, _ := view.Card("b/two")
		assert.Equal(t, "B/TWO", two.Repo.String(), "github/daily is newer than the last good trendshift/daily")
		assert.Equal(t, []Membership{{List: trendshiftDaily, Rank: 1}, {List: githubDaily, Rank: 2}}, two.Lists)
		assert.Equal(t, keys(cards), keys(view.Shown()), "the order does not depend on the age of the snapshots")
	})
}

func TestMarks(t *testing.T) {
	f := start(t)
	f.loader.Set(trendshiftDaily, watchtest.Entries(t, "Owner/Name", "other/repo")...)
	f.loader.Set(githubDaily, watchtest.Entries(t, "owner/name")...)
	_, err := f.service.Open(t.Context())
	require.NoError(t, err)
	loads := f.loader.Total()

	t.Run("a mark takes the name and the lists of the view", func(t *testing.T) {
		require.NoError(t, f.service.Mark(store.Interesting, trend.Repo{Owner: "OWNER", Name: "NAME"}, "why", Scope{}))
		assert.Equal(t, []store.Mark{{
			Time: morning, Repo: trend.Repo{Owner: "OWNER", Name: "NAME"}, Action: store.Interesting, Comment: "why",
			Lists: []string{"trendshift/daily", "github/daily"},
		}}, f.store.Marks())

		card, found := f.service.View(t.Context()).Card("owner/name")
		require.True(t, found)
		assert.True(t, card.Hidden())
		assert.Equal(t, &SeenReason{At: morning, Action: store.Interesting}, card.Seen)
		assert.Equal(t, &BookmarkReason{At: morning, Comment: "why"}, card.Bookmark)
		assert.Empty(t, card.Reasons)
	})

	t.Run("a skip keeps no comment", func(t *testing.T) {
		require.NoError(t, f.service.Mark(store.Skip, trend.Repo{Owner: "other", Name: "repo"}, "not for me", Scope{}))
		marks := f.store.Marks()
		assert.Empty(t, marks[len(marks)-1].Comment)
	})

	t.Run("a repository that left the lists can still be marked", func(t *testing.T) {
		require.NoError(t, f.service.Mark(store.Interesting, trend.Repo{Owner: "gone", Name: "away"}, "", Scope{}))
		marks := f.store.Marks()
		assert.Equal(t, []string{}, marks[len(marks)-1].Lists)
	})

	t.Run("what is not a mark of a card is refused", func(t *testing.T) {
		before := len(f.store.Marks())
		assert.ErrorIs(t, f.service.Mark(store.Seen, trend.Repo{Owner: "a", Name: "b"}, "", Scope{}), ErrInvalid)
		assert.ErrorIs(t, f.service.Mark("star", trend.Repo{Owner: "a", Name: "b"}, "", Scope{}), ErrInvalid)
		assert.ErrorIs(t, f.service.Mark(store.Interesting, trend.Repo{Owner: "a", Name: "b"}, strings.Repeat("x", MaxComment+1), Scope{}), ErrInvalid)
		assert.Error(t, f.service.Mark(store.Skip, trend.Repo{Owner: "a"}, "", Scope{}))
		assert.Len(t, f.store.Marks(), before)
	})

	t.Run("mark all shown marks each repository once and the seen ones not again", func(t *testing.T) {
		before := len(f.store.Marks())
		marked, err := f.service.MarkSeen([]trend.Repo{
			{Owner: "owner", Name: "name"},              // interesting already
			{Owner: "owner", Name: "trendshift-weekly"}, // new
			{Owner: "Owner", Name: "Trendshift-Weekly"}, // the same one
			{Owner: "owner", Name: "github-weekly"},     // new
		}, Scope{})
		require.NoError(t, err)
		assert.Equal(t, 2, marked)
		marks := f.store.Marks()
		require.Len(t, marks, before+2)
		assert.Equal(t, store.Mark{
			Time: morning, Repo: trend.Repo{Owner: "owner", Name: "trendshift-weekly"}, Action: store.Seen,
			Lists: []string{"trendshift/weekly"},
		}, marks[before])

		marked, err = f.service.MarkSeen(nil, Scope{})
		require.NoError(t, err)
		assert.Zero(t, marked)
	})

	assert.Equal(t, loads, f.loader.Total(), "a mark loads nothing")
}

// slowLoader waits until the test lets it answer, or its context ends.
type slowLoader struct {
	inner   *watchtest.Loader
	started chan trend.List
	release chan struct{}
}

func (l *slowLoader) Load(ctx context.Context, list trend.List) (trend.Snapshot, error) {
	l.started <- list
	select {
	case <-l.release:
		return l.inner.Load(ctx, list)
	case <-ctx.Done():
		return trend.Snapshot{}, ctx.Err()
	}
}

func TestLoads(t *testing.T) {
	t.Run("a list that takes longer than its timeout is in error, the others are not", func(t *testing.T) {
		f := start(t)
		slow := &slowLoader{inner: f.loader, started: make(chan trend.List, 6), release: make(chan struct{})}
		service := New(Options{Store: f.store, Loader: loaderFunc(func(ctx context.Context, list trend.List) (trend.Snapshot, error) {
			if list == githubDaily {
				return slow.Load(ctx, list)
			}
			return f.loader.Load(ctx, list)
		}), Now: f.clock.Now, Timeout: 20 * time.Millisecond})

		view, err := service.Open(t.Context())
		require.NoError(t, err)
		assert.Equal(t, 1, view.FailedLists())
		assert.Equal(t, 5, view.LoadedLists())
		for _, list := range view.Lists {
			if list.List == githubDaily {
				assert.True(t, list.Failed)
				assert.Contains(t, list.Reason, "no answer in 20ms")
				assert.Equal(t, morning, list.FailedAt)
			}
		}
	})

	t.Run("a wait that is given up leaves no record", func(t *testing.T) {
		f := start(t)
		slow := &slowLoader{inner: f.loader, started: make(chan trend.List, 6), release: make(chan struct{})}
		service := New(Options{Store: f.store, Loader: slow, Now: f.clock.Now})

		ctx, cancel := context.WithCancel(t.Context())
		done := make(chan error, 1)
		go func() {
			_, err := service.Open(ctx)
			done <- err
		}()
		for range trend.Lists() {
			<-slow.started
		}
		cancel()
		require.ErrorIs(t, <-done, context.Canceled)

		for _, list := range trend.Lists() {
			_, found := f.store.LastAttempt(list)
			assert.False(t, found, "the list did not fail: the service stopped")
		}
		assert.NoFileExists(t, filepath.Join(f.dir, store.AttemptsFile))
	})

	t.Run("a wait that is given up keeps the loads that succeeded", func(t *testing.T) {
		f := start(t)
		trendshiftMonthly := trend.List{Source: trend.Trendshift, Period: trend.Monthly}
		f.loader.Fail(trendshiftMonthly, errors.New("trendshift/monthly: the site answered 503"))
		blocked := make(chan struct{})
		loader := loaderFunc(func(ctx context.Context, list trend.List) (trend.Snapshot, error) {
			if list != githubDaily {
				return f.loader.Load(ctx, list)
			}
			// github/daily does not answer before the service stops.
			close(blocked)
			<-ctx.Done()
			return trend.Snapshot{}, ctx.Err()
		})

		// In the bubble the test knows when every other load has returned
		// and only the blocked one is left.
		synctest.Test(t, func(t *testing.T) {
			service := New(Options{Store: f.store, Loader: loader, Now: f.clock.Now})
			ctx, cancel := context.WithCancel(t.Context())
			done := make(chan error, 1)
			go func() {
				_, err := service.Open(ctx)
				done <- err
			}()
			<-blocked
			synctest.Wait()
			cancel()
			require.ErrorIs(t, <-done, context.Canceled)
		})
		assert.Equal(t, 5, f.loader.Total())

		// A restart on the same directory.
		reopened, err := store.Open(f.dir)
		require.NoError(t, err)
		for _, list := range trend.Lists() {
			_, loaded := reopened.Latest(list)
			last, attempted := reopened.LastAttempt(list)
			switch list {
			case githubDaily:
				assert.False(t, loaded, list)
				assert.False(t, attempted, "the list did not fail: the wait was given up")
			case trendshiftMonthly:
				assert.False(t, loaded, list)
				require.True(t, attempted, list)
				assert.Equal(t, store.Attempt{Time: morning, List: list.String(), Error: "the site answered 503"}, last,
					"a list that failed for its own reason is in error")
			default:
				assert.True(t, loaded, list)
				require.True(t, attempted, list)
				assert.Equal(t, store.Attempt{Time: morning, List: list.String(), OK: true}, last)
			}
		}

		// Within the hour only the list without an attempt is loaded.
		f.clock.Advance(30 * time.Minute)
		restarted := New(Options{Store: reopened, Loader: f.loader, Now: f.clock.Now})
		view, err := restarted.Open(t.Context())
		require.NoError(t, err)
		assert.Equal(t, 1, f.loader.Calls(githubDaily))
		assert.Equal(t, 6, f.loader.Total(), "the hour of the lists that answered holds")
		assert.Equal(t, 5, view.LoadedLists())
		assert.Equal(t, 1, view.FailedLists())
		assert.Len(t, view.Shown(), 5)
	})

	t.Run("pages opened at once load a list once", func(t *testing.T) {
		f := start(t)
		slow := &slowLoader{inner: f.loader, started: make(chan trend.List, 60), release: make(chan struct{})}
		service := New(Options{Store: f.store, Loader: slow, Now: f.clock.Now})

		var group sync.WaitGroup
		for range 5 {
			group.Go(func() {
				view, err := service.Open(t.Context())
				assert.NoError(t, err)
				assert.Len(t, view.Shown(), 6)
			})
		}
		for range trend.Lists() {
			<-slow.started
		}
		close(slow.release)
		group.Wait()
		assert.Equal(t, len(trend.Lists()), f.loader.Total())
	})

	t.Run("an answer that is another list, or has no repository, is an error", func(t *testing.T) {
		f := start(t)
		service := New(Options{Store: f.store, Now: f.clock.Now, Loader: loaderFunc(func(ctx context.Context, list trend.List) (trend.Snapshot, error) {
			snapshot, err := f.loader.Load(ctx, list)
			switch list {
			case githubDaily:
				snapshot.List = trendshiftDaily
			case trendshiftWeekly:
				snapshot.Entries[0].Repo.Name = ""
			case trendshiftDaily:
				snapshot.Entries = nil
			default:
			}
			return snapshot, err
		})})
		view, err := service.Open(t.Context())
		require.NoError(t, err)
		reasons := map[trend.List]string{}
		for _, list := range view.Lists {
			if list.Failed {
				reasons[list.List] = list.Reason
				assert.False(t, list.Loaded)
			}
		}
		assert.Equal(t, map[trend.List]string{
			githubDaily:      "the answer is the list trendshift/daily",
			trendshiftWeekly: "the entry at position 1 has no repository",
			trendshiftDaily:  "the list has no repositories",
		}, reasons)
		assert.Len(t, view.Shown(), 3)
	})

	t.Run("the loader's time of the snapshot is kept", func(t *testing.T) {
		f := start(t)
		fetched := morning.Add(-3 * time.Second)
		service := New(Options{Store: f.store, Now: f.clock.Now, Loader: loaderFunc(func(ctx context.Context, list trend.List) (trend.Snapshot, error) {
			snapshot, err := f.loader.Load(ctx, list)
			snapshot.FetchedAt = fetched
			return snapshot, err
		})})
		view, err := service.Open(t.Context())
		require.NoError(t, err)
		assert.Equal(t, fetched, view.Lists[0].FetchedAt)
	})

	t.Run("an outcome that cannot be recorded holds until the service stops", func(t *testing.T) {
		f := start(t)
		attempts := filepath.Join(f.dir, store.AttemptsFile)
		require.NoError(t, os.Mkdir(attempts, 0o700))
		f.loader.Fail(githubDaily, errors.New("github/daily: the site answered 503"))

		view, err := f.service.Open(t.Context())
		require.NoError(t, err)
		assert.Contains(t, view.Problem, "could not be recorded")
		assert.Len(t, view.Shown(), 5, "the snapshots are there")
		assert.Equal(t, 1, view.FailedLists(), "the list that failed is in error all the same")
		assert.Equal(t, 6, f.loader.Total())

		// The hour holds for the running service, and so does the problem.
		f.clock.Advance(30 * time.Minute)
		view, err = f.service.Open(t.Context())
		require.NoError(t, err)
		assert.Equal(t, 6, f.loader.Total(), "nothing is loaded again within the hour")
		assert.Contains(t, view.Problem, "could not be recorded")
		assert.Equal(t, 1, view.FailedLists())
		assert.Contains(t, f.service.View(t.Context()).Problem, "could not be recorded")

		problem, err := f.service.Refresh(t.Context())
		require.NoError(t, err)
		assert.Contains(t, problem, "could not be recorded")
		assert.Equal(t, 12, f.loader.Total())

		// Once the file can be written, the next page records what was
		// left, without a load.
		require.NoError(t, os.Remove(attempts))
		f.clock.Advance(30 * time.Minute)
		view, err = f.service.Open(t.Context())
		require.NoError(t, err)
		assert.Empty(t, view.Problem)
		assert.Equal(t, 12, f.loader.Total())

		reopened, err := store.Open(f.dir)
		require.NoError(t, err)
		last, found := reopened.LastAttempt(githubDaily)
		require.True(t, found)
		assert.Equal(t, store.Attempt{
			Time: morning.Add(30 * time.Minute), List: "github/daily", Error: "the site answered 503",
		}, last, "the last attempt of the list, as the service held it")
		restarted := New(Options{Store: reopened, Loader: f.loader, Now: f.clock.Now})
		view, err = restarted.Open(t.Context())
		require.NoError(t, err)
		assert.Equal(t, 12, f.loader.Total(), "the hour holds across the restart")
		assert.Equal(t, 1, view.FailedLists())
	})

	t.Run("a snapshot that cannot be stored is an error of its list", func(t *testing.T) {
		f := start(t)
		require.NoError(t, os.WriteFile(filepath.Join(f.dir, store.SnapshotsDir), nil, 0o600))

		view, err := f.service.Open(t.Context())
		require.NoError(t, err)
		assert.Equal(t, 6, view.FailedLists())
		assert.Zero(t, view.LoadedLists())
		assert.Contains(t, view.Lists[0].Reason, "the snapshot could not be stored")
	})
}

// loaderFunc is a function as a Loader.
type loaderFunc func(ctx context.Context, list trend.List) (trend.Snapshot, error)

func (f loaderFunc) Load(ctx context.Context, list trend.List) (trend.Snapshot, error) {
	return f(ctx, list)
}

func TestLongReasonIsCut(t *testing.T) {
	f := start(t)
	f.loader.Fail(githubDaily, errors.New("github/daily: "+strings.Repeat("я", 400)))
	view, err := f.service.Open(t.Context())
	require.NoError(t, err)
	for _, list := range view.Lists {
		if list.List != githubDaily {
			continue
		}
		assert.True(t, strings.HasPrefix(list.Reason, "яяя"), "the name of the list is said once, by the page")
		assert.True(t, strings.HasSuffix(list.Reason, "я…"))
		assert.LessOrEqual(t, len(list.Reason), 504)
		assert.NotContains(t, list.Reason, "�")
	}
}
