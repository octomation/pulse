package watch

import (
	"cmp"
	"slices"
	"time"

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
	// Vault is the state of the vault filter.
	Vault VaultStatus
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

// VaultReason hides a repository that a vault note names.
type VaultReason struct {
	Note string
}

// Card is a repository of the view.
type Card struct {
	// Repo is the repository as the most recent snapshot writes it.
	Repo        trend.Repo
	Description string
	Language    string
	// Lists are the lists the repository is in, with its position there.
	Lists []Membership

	// Seen, Bookmark and Vault are the reasons to hide the card; a card
	// carries every reason that applies.
	Seen     *SeenReason
	Bookmark *BookmarkReason
	Vault    *VaultReason
}

// Key identifies the card: owner/name without case.
func (c Card) Key() string { return c.Repo.Key() }

// Hidden reports whether the page hides the card by default: the repository
// is seen or is in the vault.
func (c Card) Hidden() bool { return c.Seen != nil || c.Vault != nil }

// VaultStatus is the state of the vault filter.
type VaultStatus struct {
	// Configured says whether the configuration names a directory.
	Configured bool
	Dir        string
	// Repos is the number of repositories the notes name.
	Repos int
	// Notes is the number of notes read.
	Notes int
	// Broken names the notes whose frontmatter could not be read.
	Broken []string
	// Error says why the filter does not work: the view may include what is
	// already in the vault.
	Error string
}

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

// View builds the view from what is stored, and reads the vault notes
// again. It loads nothing and marks nothing.
func (s *Service) View() View {
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
	status, notes := s.readVault()
	view.Vault = status
	for i := range view.Cards {
		card := &view.Cards[i]
		card.Seen = seen[card.Key()]
		card.Bookmark = bookmarks[card.Key()]
		if note, found := notes.Note(card.Repo); found {
			card.Vault = &VaultReason{Note: note}
		}
	}
	return view
}
