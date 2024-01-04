package web

import (
	"cmp"
	"fmt"
	"html/template"
	"slices"
	"strconv"
	"strings"
	"time"

	"go.octolab.org/toolset/pulse/internal/details"
	"go.octolab.org/toolset/pulse/internal/store"
	"go.octolab.org/toolset/pulse/internal/trend"
	"go.octolab.org/toolset/pulse/internal/watch"
)

// page is what the template renders.
type page struct {
	Now time.Time
	// ShowHidden says that the hidden cards are on the page too.
	ShowHidden bool
	Shown      []card
	// Sections lay the shown cards out as a newspaper does: each card is in
	// one section.
	Sections []section
	Hidden   []card
	// HiddenCount is the number of hidden cards, rendered or not.
	HiddenCount int
	Lists       []watch.ListStatus
	Vault       watch.VaultStatus
	Problem     string
	Empty       empty
}

// empty is what the page says in place of the new repositories when there
// are none.
type empty struct {
	// Kind is clear when nothing is new and every list loaded, partial when
	// some list is in error, failed when no list has a snapshot, unrecorded
	// when the state of the lists could not be written.
	Kind  string
	Title string
	Text  string
}

// section is a part of the newspaper.
type section struct {
	// Kind is front, daily, weekly or monthly.
	Kind  string
	Title string
	// Note says what the section holds.
	Note  string
	Cards []card
}

// frontPage is the most the front page holds.
const frontPage = 7

// layout sorts the shown cards into the sections. The front page takes the
// repositories that are in most lists, the first of them as the lead; the
// others go to the section of their shortest period.
func layout(shown []card) []section {
	best := func(c card, period trend.Period) int {
		rank := 0
		for _, m := range c.Lists {
			if (period == "" || m.List.Period == period) && (rank == 0 || m.Rank < rank) {
				rank = m.Rank
			}
		}
		return rank
	}
	cards := slices.Clone(shown)
	slices.SortStableFunc(cards, func(a, b card) int {
		return cmp.Or(cmp.Compare(len(b.Lists), len(a.Lists)), cmp.Compare(best(a, ""), best(b, "")))
	})
	front := section{Kind: "front", Title: "Front page", Note: "In most lists at once"}
	for len(cards) > 0 && len(front.Cards) < frontPage && (len(cards[0].Lists) > 1 || len(front.Cards) == 0) {
		front.Cards = append(front.Cards, cards[0])
		cards = cards[1:]
	}
	if len(front.Cards) > 0 {
		front.Cards[0].Lead = true
	}
	sections := []section{front}
	for _, part := range []section{
		{Kind: string(trend.Daily), Title: "Today", Note: "New in the daily lists"},
		{Kind: string(trend.Weekly), Title: "This week", Note: "New in the weekly lists"},
		{Kind: string(trend.Monthly), Title: "This month", Note: "New in the monthly lists"},
	} {
		period := trend.Period(part.Kind)
		var rest []card
		for _, c := range cards {
			if best(c, period) > 0 {
				part.Cards = append(part.Cards, c)
			} else {
				rest = append(rest, c)
			}
		}
		cards = rest
		slices.SortStableFunc(part.Cards, func(a, b card) int { return cmp.Compare(best(a, period), best(b, period)) })
		sections = append(sections, part)
	}
	return slices.DeleteFunc(sections, func(s section) bool { return len(s.Cards) == 0 })
}

// card is a repository on the page.
type card struct {
	watch.Card
	// State is shown or hidden.
	State string
	// Lead says that the card is the lead story of the front page.
	Lead bool
}

// story is a repository in the reader: its card, and what GitHub knows.
type story struct {
	card
	// Known says whether the lists have the repository now.
	Known   bool
	Details *details.Details
	// Error says why GitHub is not in the story.
	Error string
}

func newCard(c watch.Card) card {
	state := "shown"
	if c.Hidden() {
		state = "hidden"
	}
	return card{Card: c, State: state}
}

// Comment is the comment the field of the card starts with.
func (c card) Comment() string {
	if c.Bookmark != nil {
		return c.Bookmark.Comment
	}
	return ""
}

func newPage(view watch.View, showHidden bool) page {
	p := page{
		Now: view.Now, ShowHidden: showHidden,
		Lists: view.Lists, Vault: view.Vault, Problem: view.Problem,
	}
	for _, c := range view.Cards {
		switch {
		case !c.Hidden():
			p.Shown = append(p.Shown, newCard(c))
		case showHidden:
			p.Hidden = append(p.Hidden, newCard(c))
		}
	}
	p.HiddenCount = view.HiddenCount()
	p.Sections = layout(p.Shown)

	p.Empty = emptyOf(view)
	return p
}

// emptyOf says what the page shows when no repository is new: "Nothing new"
// without reservations only when no list is in error and their state is
// recorded.
func emptyOf(view watch.View) empty {
	total := len(view.Lists)
	switch failed := view.FailedLists(); {
	case view.LoadedLists() == 0:
		return empty{
			Kind:  "failed",
			Title: "No list could be loaded",
			Text:  "None of the lists has a snapshot yet, so there is nothing to compare. The reasons are above; Refresh tries again.",
		}
	case failed > 0:
		return empty{
			Kind:  "partial",
			Title: "No new repositories in the lists that loaded",
			Text: fmt.Sprintf("%s of %d failed to load, so this is not the whole picture. The reasons are above; Refresh tries again.",
				plural(failed, "list", "lists"), total),
		}
	case view.Problem != "":
		return empty{
			Kind:  "unrecorded",
			Title: "No new repositories, but the state of the lists could not be recorded",
			Text: "Every repository of the " + plural(total, "list", "lists") + " is seen or already in your bookmarks. " +
				"pulse could not write the outcome of the last load, so it holds only until pulse stops. The reason is above.",
		}
	default:
		return empty{
			Kind:  "clear",
			Title: "Nothing new",
			Text:  "Every repository of the " + plural(total, "list", "lists") + " is seen or already in your bookmarks.",
		}
	}
}

func plural(n int, one, many string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, one)
	}
	return fmt.Sprintf("%d %s", n, many)
}

func (h *handler) funcs() template.FuncMap {
	local := func(t time.Time) time.Time { return t.In(h.Location) }
	return template.FuncMap{
		// stamp is the machine-readable time.
		"stamp": func(t time.Time) string { return t.UTC().Format(time.RFC3339) },
		// full is the time with its date.
		"full": func(t time.Time) string { return local(t).Format("Jan 2, 2006, 15:04") },
		// short is the time of day, with the date when it is not the day of
		// the page.
		"short": func(t, now time.Time) string {
			t, now = local(t), local(now)
			switch {
			case t.Year() == now.Year() && t.YearDay() == now.YearDay():
				return t.Format("15:04")
			case t.Year() == now.Year():
				return t.Format("Jan 2, 15:04")
			default:
				return t.Format("Jan 2, 2006, 15:04")
			}
		},
		"plural": plural,
		// day is the date of the masthead.
		"day": func(t time.Time) string { return local(t).Format("Monday, January 2, 2006") },
		// month is a date to the month.
		"month": func(t time.Time) string { return local(t).Format("January 2006") },
		// date is a date without the time.
		"date": func(t time.Time) string { return local(t).Format("Jan 2, 2006") },
		// num writes a number with thousands apart.
		"num": func(n int) string {
			digits := strconv.Itoa(n)
			for i := len(digits) - 3; i > 0; i -= 3 {
				digits = digits[:i] + "," + digits[i:]
			}
			return digits
		},
		"among": func(n int) string {
			if n == 1 {
				return "in one list"
			}
			return fmt.Sprintf("in %d lists", n)
		},
		"title": func(l trend.List) string { return string(l.Source) + " " + string(l.Period) },
		"action": func(a store.Action) string {
			switch a {
			case store.Interesting:
				return "Marked as interesting"
			case store.Skip:
				return "Skipped"
			case store.Seen:
				return "Marked as seen"
			default:
				return "Marked"
			}
		},
		"names": func(names []string) string {
			const most = 20
			if len(names) > most {
				return strings.Join(names[:most], ", ") + fmt.Sprintf(" and %d more", len(names)-most)
			}
			return strings.Join(names, ", ")
		},
	}
}
