package web

import (
	"cmp"
	"fmt"
	"html/template"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"go.octolab.org/toolset/pulse/internal/details"
	"go.octolab.org/toolset/pulse/internal/hide"
	"go.octolab.org/toolset/pulse/internal/store"
	"go.octolab.org/toolset/pulse/internal/trend"
	"go.octolab.org/toolset/pulse/internal/watch"
)

// page is what the template renders.
type page struct {
	Now time.Time
	// Date is the day of the page: today, or the past day it shows.
	Date time.Time
	// Today says that the page is today's: it loads the lists and offers
	// Refresh.
	Today bool
	// Day is the date of the page, YYYY-MM-DD, today's included: the marks
	// of the page are made on the page as it stood on that date.
	Day string
	// Base is the path of the day: / for today, /days/YYYY-MM-DD/ for a day
	// of the calendar.
	Base string
	// Prev and Next are the paths of the days with snapshots around this
	// one, in the same mode; empty when there is none.
	Prev, Next string
	// Calendar lays out the days that have snapshots, newest month first.
	Calendar []month
	// ShowHidden says that the hidden cards are on the page too.
	ShowHidden bool
	Shown      []card
	// Sections lay the shown cards out as a newspaper does: each card is in
	// one section.
	Sections []section
	Hidden   []card
	// Groups lay the hidden cards out by their standing: interesting,
	// skipped, and in the notes.
	Groups []section
	// HiddenCount is the number of hidden cards, rendered or not.
	HiddenCount int
	Lists       []watch.ListStatus
	Sources     []hide.Status
	Problem     string
	Empty       empty
	// Flow is the shown cards in the order of the page, for flow mode.
	Flow []card
	// Reservations say why the cards may not be the whole picture: lists
	// that failed and sources that could not be read. Flow mode shows them,
	// since it has no status of the lists.
	Reservations []string
}

// month is a month of the calendar: weeks from Monday, with blanks around
// the days of the month.
type month struct {
	Title string
	Weeks [][]calendarDay
}

// calendarDay is a cell of the calendar.
type calendarDay struct {
	// Number is the day of the month; zero is a blank cell.
	Number int
	// Href is the path of the day when it has snapshots.
	Href string
	// Current says that the page shows this day; Today that it is today.
	Current, Today bool
}

// calendar lays out the months from the first day with snapshots to the
// month of today, newest first, and links the days that have snapshots.
// The days are calendar dates of loc: a day whose midnight the clocks skip
// is a day all the same.
func calendar(days []time.Time, current, today time.Time, loc *time.Location) []month {
	if len(days) == 0 {
		return nil
	}
	date := func(t time.Time) string { return t.In(loc).Format(dateLayout) }
	has := map[string]bool{}
	for _, day := range days {
		has[date(day)] = true
	}
	firstYear, firstMonth, _ := days[0].In(loc).Date()
	lastYear, lastMonth, _ := today.In(loc).Date()
	if end := days[len(days)-1]; end.After(today) {
		lastYear, lastMonth, _ = end.In(loc).Date()
	}
	var months []month
	for at := time.Date(lastYear, lastMonth, 1, 12, 0, 0, 0, time.UTC); !at.Before(time.Date(firstYear, firstMonth, 1, 12, 0, 0, 0, time.UTC)); at = at.AddDate(0, -1, 0) {
		m := month{Title: at.Format("January 2006")}
		week := make([]calendarDay, (int(at.Weekday())+6)%7)
		for d := at; d.Month() == at.Month(); d = d.AddDate(0, 0, 1) {
			key := d.Format(dateLayout)
			cell := calendarDay{Number: d.Day(), Current: key == date(current), Today: key == date(today)}
			if has[key] {
				cell.Href = "/days/" + key + "/"
			}
			week = append(week, cell)
			if len(week) == 7 {
				m.Weeks = append(m.Weeks, week)
				week = nil
			}
		}
		if len(week) > 0 {
			week = append(week, make([]calendarDay, 7-len(week))...)
			m.Weeks = append(m.Weeks, week)
		}
		months = append(months, m)
	}
	return months
}

// dateLayout is how a path writes a day.
const dateLayout = "2006-01-02"

// dayPath is the path of a day of the calendar that starts at day in loc.
func dayPath(day time.Time, loc *time.Location) string {
	return "/days/" + day.In(loc).Format(dateLayout) + "/"
}

// around returns the days with snapshots right before and after current.
func around(days []time.Time, current time.Time) (prev, next time.Time) {
	for _, day := range days {
		switch {
		case day.Before(current):
			prev = day
		case day.After(current) && next.IsZero():
			next = day
		}
	}
	return prev, next
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
// others go to the section of their shortest period. A past day names the
// sections for that day.
func layout(shown []card, past bool) []section {
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
		{Kind: string(trend.Daily), Title: pick(past, "That day", "Today"), Note: "New in the daily lists"},
		{Kind: string(trend.Weekly), Title: pick(past, "That week", "This week"), Note: "New in the weekly lists"},
		{Kind: string(trend.Monthly), Title: pick(past, "That month", "This month"), Note: "New in the monthly lists"},
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

// pick returns past on a page of a past day, else today: the sections of a
// past day are named for that day.
func pick(isPast bool, past, today string) string {
	if isPast {
		return past
	}
	return today
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

// Snoozed reports whether the answer of the repository is a skip or "Mark
// all shown as seen" whose snooze runs.
func (c card) Snoozed() bool { return c.Seen != nil && c.Seen.Action != store.Interesting }

// Comment is the comment the field of the card starts with.
func (c card) Comment() string {
	if c.Bookmark != nil {
		return c.Bookmark.Comment
	}
	return ""
}

func newPage(view watch.View, showHidden bool) page {
	p := page{
		Now: view.Now, Date: view.Now, Today: !view.Past(), Base: "/", ShowHidden: showHidden,
		Lists: view.Lists, Sources: view.Sources, Problem: view.Problem,
	}
	if view.Past() {
		p.Date = view.Day
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
	p.Groups = groups(p.Hidden)
	p.Sections = layout(p.Shown, view.Past())
	for _, part := range p.Sections {
		p.Flow = append(p.Flow, part.Cards...)
	}

	p.Empty = emptyOf(view)
	if failed := view.FailedLists(); failed > 0 {
		p.Reservations = append(p.Reservations, fmt.Sprintf("%s of %d failed to load, so this is not the whole picture.",
			plural(failed, "list", "lists"), len(view.Lists)))
	}
	for _, source := range view.FailedSources() {
		p.Reservations = append(p.Reservations, fmt.Sprintf("The %s could not be read: %s. The cards may include what is already in the %s.",
			source.Source, source.Error, source.Source))
	}
	for _, source := range view.Sources {
		if source.Warning != "" {
			p.Reservations = append(p.Reservations, fmt.Sprintf("%s: %s.", capital(source.Source), source.Warning))
		}
	}
	return p
}

// groups sorts the hidden cards by their standing: what a note holds is in
// the notes whatever its answer in pulse; the rest is interesting or
// skipped.
func groups(hidden []card) []section {
	parts := []section{
		{Kind: "interesting", Title: "Interesting", Note: "Marked as interesting, no note yet"},
		{Kind: "skipped", Title: "Skipped", Note: "Skipped or marked as seen, until the snooze ends"},
		{Kind: "notes", Title: "In your notes", Note: "A note of the vault names them"},
	}
	for _, c := range hidden {
		switch {
		case c.InNotes():
			parts[2].Cards = append(parts[2].Cards, c)
		case c.Bookmark != nil:
			parts[0].Cards = append(parts[0].Cards, c)
		default:
			parts[1].Cards = append(parts[1].Cards, c)
		}
	}
	return slices.DeleteFunc(parts, func(s section) bool { return len(s.Cards) == 0 })
}

// queuePage is the queue: the repositories marked as interesting that no
// note names yet.
type queuePage struct {
	Cards []card
	// Problem says why the notes could not be told apart: the queue then
	// holds every bookmark.
	Problem string
}

func capital(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// emptyOf says what the page shows when no repository is new: "Nothing new"
// without reservations only when no list is in error, every source that
// hides cards answered, and the state of the lists is recorded. A list
// without a snapshot on a past day is not an error: the page names the lists
// that had one.
func emptyOf(view watch.View) empty {
	total := len(view.Lists)
	unchecked := view.FailedSources()
	names := make([]string, 0, len(unchecked))
	for _, source := range unchecked {
		names = append(names, "the "+source.Source)
	}
	// also is what the page says of the sources after another reservation.
	also := ""
	if len(names) > 0 {
		also = " The list may also include what is already in " + strings.Join(names, " and ") + ", which could not be read."
	}
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
				plural(failed, "list", "lists"), total) + also,
		}
	case len(unchecked) > 0:
		return empty{
			Kind:  "unchecked",
			Title: "No new repositories, but " + strings.Join(names, " and ") + " could not be read",
			Text: "Every repository of the lists is seen or bookmarked, but the list may include what is already in " +
				strings.Join(names, " and ") + ". The reason is above.",
		}
	case !view.AsOf.IsZero():
		// The loads of that day are not known: no claim on the whole
		// picture.
		return empty{
			Kind:  "over",
			Title: "No new repositories left on this page",
			Text: "The day of this page is over, so it shows the lists as they stood then, not whether they all loaded. " +
				"Reload it to see today.",
		}
	case view.Past() && view.LoadedLists() < total:
		return empty{
			Kind:  "clear",
			Title: "Nothing new on this day",
			Text: "Every repository of the " + plural(view.LoadedLists(), "list", "lists") +
				" that had a snapshot that day is seen or already in your bookmarks.",
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
		// home writes a path under the home directory with ~.
		"home": func(path string) string {
			if h.Home != "" {
				if rest, found := strings.CutPrefix(path, h.Home+string(filepath.Separator)); found {
					return "~" + string(filepath.Separator) + rest
				}
			}
			return path
		},
		"capital": capital,
		// iso is the date of a datetime attribute, in the zone of the page.
		"iso": func(t time.Time) string { return local(t).Format(dateLayout) },
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
			if n < 2 {
				return ""
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
