package watch_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.octolab.org/toolset/pulse/internal/hide"
	"go.octolab.org/toolset/pulse/internal/store"
	"go.octolab.org/toolset/pulse/internal/trend"
	. "go.octolab.org/toolset/pulse/internal/watch"
	"go.octolab.org/toolset/pulse/internal/watch/watchtest"
)

// notes is a source that holds the repositories it names, or fails.
type notes struct {
	names  map[string]bool
	failed bool
	broken []string
}

func (n notes) Check(_ context.Context, items []hide.Item) ([][]hide.Reason, hide.Status) {
	if n.failed {
		return nil, hide.Status{Source: "vault", State: hide.Failed, Error: "the directory is gone"}
	}
	answer := make([][]hide.Reason, len(items))
	for i, item := range items {
		if n.names[item.Repo.Key()] {
			answer[i] = []hide.Reason{{Source: "vault", Label: "In the vault", Ref: item.Repo.Name + ".md"}}
		}
	}
	return answer, hide.Status{Source: "vault", State: hide.OK, Broken: n.broken}
}

func repo(name string) trend.Repo {
	r, err := trend.ParseRepo(name)
	if err != nil {
		panic(err)
	}
	return r
}

// The last answer counts, an answer the repository has writes nothing, and
// Take back makes it new again.
func TestAnswers(t *testing.T) {
	f := start(t)
	f.loader.Set(trendshiftDaily, watchtest.Entries(t, "owner/name", "other/repo")...)
	_, err := f.service.Open(t.Context())
	require.NoError(t, err)
	card := func(key string) Card {
		t.Helper()
		c, found := f.service.View(t.Context()).Card(key)
		require.True(t, found)
		return c
	}
	written := func() int { return len(f.store.Marks()) }

	require.NoError(t, f.service.Mark(store.Interesting, repo("owner/name"), "why", Scope{}))
	require.Equal(t, 1, written())

	t.Run("the same answer writes nothing", func(t *testing.T) {
		require.NoError(t, f.service.Mark(store.Interesting, repo("Owner/Name"), "why", Scope{}))
		assert.Equal(t, 1, written())
	})

	t.Run("another comment is written and keeps the time of the bookmark", func(t *testing.T) {
		f.clock.Advance(time.Hour)
		require.NoError(t, f.service.Mark(store.Interesting, repo("owner/name"), "the hub", Scope{}))
		assert.Equal(t, 2, written())
		assert.Equal(t, &BookmarkReason{At: morning, Comment: "the hub"}, card("owner/name").Bookmark)
	})

	t.Run("a skip after Interesting replaces it", func(t *testing.T) {
		require.NoError(t, f.service.Mark(store.Skip, repo("owner/name"), "", Scope{}))
		c := card("owner/name")
		assert.Nil(t, c.Bookmark)
		require.NotNil(t, c.Seen)
		assert.Equal(t, store.Skip, c.Answer())
		assert.Empty(t, f.service.Bookmarks())
	})

	t.Run("a second skip does not extend the snooze", func(t *testing.T) {
		before := card("owner/name").Seen.Until
		f.clock.Advance(24 * time.Hour)
		require.NoError(t, f.service.Mark(store.Skip, repo("owner/name"), "", Scope{}))
		assert.Equal(t, 3, written())
		assert.Equal(t, before, card("owner/name").Seen.Until)
	})

	t.Run("take back makes it new, and twice writes nothing", func(t *testing.T) {
		require.NoError(t, f.service.Mark(store.Clear, repo("owner/name"), "ignored", Scope{}))
		require.NoError(t, f.service.Mark(store.Clear, repo("owner/name"), "", Scope{}))
		assert.Equal(t, 4, written())
		c := card("owner/name")
		assert.False(t, c.Hidden())
		assert.Empty(t, c.Answer())
		assert.Empty(t, f.store.Marks()[3].Comment, "take back keeps no comment")
	})

	t.Run("a snooze that ended is lapsed: it hides nothing and is remembered", func(t *testing.T) {
		require.NoError(t, f.service.Mark(store.Skip, repo("other/repo"), "", Scope{}))
		f.clock.Advance(100 * 24 * time.Hour)
		c := card("other/repo")
		assert.False(t, c.Hidden())
		require.NotNil(t, c.Lapsed)
		assert.Equal(t, store.Skip, c.Lapsed.Action)
		// A new skip starts a new snooze.
		require.NoError(t, f.service.Mark(store.Skip, repo("other/repo"), "", Scope{}))
		c = card("other/repo")
		assert.True(t, c.Hidden())
		assert.Nil(t, c.Lapsed)
	})
}

// Undo of "Mark all shown as seen" takes back exactly its marks: a
// repository answered since keeps its answer.
func TestUndoSeen(t *testing.T) {
	f := start(t)
	_, err := f.service.Open(t.Context())
	require.NoError(t, err)
	repos := []trend.Repo{repo("owner/trendshift-daily"), repo("owner/github-daily"), repo("owner/github-weekly")}
	require.NoError(t, f.service.Mark(store.Skip, repos[2], "", Scope{}))

	batch, err := f.service.MarkSeen(repos, Scope{})
	require.NoError(t, err)
	assert.Equal(t, repos[:2], batch.Repos, "the skipped one is not marked again")
	assert.Equal(t, 1, batch.From)
	require.NoError(t, f.service.Mark(store.Interesting, repos[1], "after all", Scope{}))

	undone, err := f.service.UndoSeen(batch, Scope{})
	require.NoError(t, err)
	assert.Equal(t, 1, undone)
	view := f.service.View(t.Context())
	first, _ := view.Card(repos[0].Key())
	second, _ := view.Card(repos[1].Key())
	third, _ := view.Card(repos[2].Key())
	assert.False(t, first.Hidden(), "the mark of Mark all is taken back")
	assert.Equal(t, store.Interesting, second.Answer(), "a later answer stays")
	assert.Equal(t, store.Skip, third.Answer(), "an earlier skip stays")

	t.Run("a batch that is not in the marks is refused", func(t *testing.T) {
		_, err := f.service.UndoSeen(Batch{Repos: repos, From: f.store.Len() - 1}, Scope{})
		assert.ErrorIs(t, err, ErrInvalid)
		_, err = f.service.UndoSeen(Batch{Repos: repos[:1], From: -1}, Scope{})
		assert.ErrorIs(t, err, ErrInvalid)
	})

	t.Run("a later Mark all is not taken back by an older Undo", func(t *testing.T) {
		later, err := f.service.MarkSeen(repos[:1], Scope{})
		require.NoError(t, err)
		require.Equal(t, repos[:1], later.Repos)
		undone, err := f.service.UndoSeen(batch, Scope{})
		require.NoError(t, err)
		assert.Zero(t, undone)
		card, _ := f.service.View(t.Context()).Card(repos[0].Key())
		assert.Equal(t, store.Seen, card.Answer())
	})
}

// Pending is the interest without a note; it fails when the notes cannot be
// read. Lookup knows a repository that left the lists.
func TestPendingAndLookup(t *testing.T) {
	dir := t.TempDir()
	state, err := store.Open(dir)
	require.NoError(t, err)
	clock := watchtest.NewClock(morning)
	source := &notes{names: map[string]bool{"owner/noted": true}}
	service := New(Options{Store: state, Loader: watchtest.NewLoader(), Now: clock.Now, Hide: hide.Facade{source}})

	for _, name := range []string{"owner/first", "owner/noted", "owner/second"} {
		require.NoError(t, service.Mark(store.Interesting, repo(name), "", Scope{}))
	}
	pending, err := service.Pending(t.Context())
	require.NoError(t, err)
	names := make([]string, 0, len(pending))
	for _, bookmark := range pending {
		names = append(names, bookmark.Repo.String())
	}
	assert.Equal(t, []string{"owner/first", "owner/second"}, names)
	assert.Len(t, service.Bookmarks(), 3)

	noted := service.Lookup(t.Context(), repo("Owner/Noted"))
	assert.True(t, noted.InNotes())
	assert.Equal(t, store.Interesting, noted.Answer())
	assert.Empty(t, noted.Lists, "the lists do not have it")

	source.broken = []string{"broken.md"}
	_, err = service.Pending(t.Context())
	assert.ErrorContains(t, err, "1 note of the vault could not be read, broken.md")

	source.failed = true
	_, err = service.Pending(t.Context())
	assert.ErrorContains(t, err, "the vault could not be read: the directory is gone")
}
