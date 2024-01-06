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

// View is what the page shows: the union of the latest successful snapshot
// of every list, a card per repository, and the state of the lists.
type View struct {
	// Now is the time the view was built.
	Now time.Time
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

// ListStatus is the state of a list: its latest snapshot and its error.
type ListStatus struct {
	List trend.List
	// Loaded says whether the list has a successful snapshot.
	Loaded bool
	// FetchedAt is the time of the latest successful snapshot.
	FetchedAt time.Time
	// Count is the number of repositories of that snapshot.
	Count int
	// Failed says that the last attempt to load the list failed: the list
	// is in error until its next successful attempt.
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

// SeenReason hides a repository that was marked: when and by which action,
// the first time.
type SeenReason struct {
	At     time.Time
	Action store.Action
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
// is seen or held by a source such as the vault.
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

// View builds the view from what is stored, and asks the sources again. It
// loads nothing and marks nothing.
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

	// Any mark makes its repository seen, since the first one; the
	// interesting ones are its bookmark, with the comment of the last.
	seen := map[string]*SeenReason{}
	bookmarks := map[string]*BookmarkReason{}
	for _, mark := range s.store.Marks() {
		key := mark.Repo.Key()
		if seen[key] == nil {
			seen[key] = &SeenReason{At: mark.Time, Action: mark.Action}
		}
		if mark.Action != store.Interesting {
			continue
		}
		if bookmarks[key] == nil {
			bookmarks[key] = &BookmarkReason{At: mark.Time}
		}
		bookmarks[key].Comment = mark.Comment
	}
	items := make([]hide.Item, len(view.Cards))
	for i, card := range view.Cards {
		items[i] = hide.Item{Repo: card.Repo}
	}
	reasons, statuses := s.hide.Check(ctx, items)
	view.Sources = statuses
	for i := range view.Cards {
		card := &view.Cards[i]
		card.Seen = seen[card.Key()]
		card.Bookmark = bookmarks[card.Key()]
		card.Reasons = reasons[i]
	}
}
