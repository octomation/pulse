---
code: PUL-6
id: I_kwDOJvS0hM8AAAABg_MSkw
databaseId: 6508712595
number: 90
url: https://github.com/octomation/pulse/issues/90
title: "web: calendar of past snapshots"
issueType: Feature
labels:
  - "type: feature"
  - "scope: code"
  - "impact: medium"
  - "effort: medium"
assignees:
  - kamilsk
milestone: "[[PUL-M2]]"
parent: null
issueFields: {}
state: CLOSED
stateReason: COMPLETED
duplicateOf: null
createdAt: 2026-11-20T09:31:16Z
updatedAt: 2026-11-23T14:14:53Z
lastEditedAt: 2026-11-23T09:35:55Z
closedAt: 2026-11-23T14:14:53Z
---

# web: calendar of past snapshots

## Motivation

The page shows only the latest snapshot of each list. When I skip a morning,
what trended yesterday or the day before is gone from the page, although
pulse keeps every load on disk ([[PUL-5]], "Snapshots"). I want to go back
to any day that has snapshots, see what the lists held then, and catch up on
what I have not seen yet, with "Show hidden" working there as it does today.

## PoC

```bash
$ open http://pulse.localhost:17842/                   # today, as now
$ open http://pulse.localhost:17842/days/2026-11-19/    # the page as the lists stood on that day
$ open http://pulse.localhost:17842/days/2026-11-19/hidden/   # the same day with the hidden ones
$ open http://pulse.localhost:17842/hidden/             # today with the hidden ones
```

The masthead names the day and has a calendar: the days with snapshots can be
opened, the others cannot; "previous" and "next" step between days that have
snapshots.

## Behavior

- A day has its own path, `/days/YYYY-MM-DD/`, not a query parameter; the
  root `/` is today. The path of today's date shows the same page as `/`.
- "Show hidden" is a path too: `/hidden/` for today and
  `/days/YYYY-MM-DD/hidden/` for a day; the switch moves between the two
  paths of the same day. The old `/?hidden=1` is gone: the query parameter
  is ignored and `/` shows today. No page of pulse takes a query parameter.
- A day is a calendar day in the local time of the Mac. Snapshots are stored
  in UTC and are put into days by local time.
- The view of a past day is built like today's, from the last successful
  snapshot of each list taken within that day. A list without a snapshot that
  day is named as missing for the day, not as an error.
- A past day is not a restored state: it is the snapshots of that day passed
  through everything I have done since. Hidden is decided by what I have
  seen at all, the vault and the marks, up to now: a repository seen or
  bookmarked after that day, or added to the vault since, is hidden on that
  day too. "Show hidden" shows it with the reason, as on today's page.
- "Interesting", "Skip" and "Mark all shown as seen" work on a past day, and
  mark what that page showed. A mark records the lists the repository was in
  on that day.
- Opening a past day loads nothing from the network; "Refresh" is offered on
  today only.
- The calendar lists every day that has at least one snapshot, the first day
  included. Today without snapshots yet is still today's page.
- A malformed date, or a day without snapshots, in the path answers 404 with
  a link to today, not
  today's page under another date.

## Limits

- Snapshots are kept without expiry; this task adds no pruning.
- Only whole days: no view of a single load within a day, no range of days.
- A day before the first snapshot cannot be shown: pulse has no data for it.

## Acceptance criteria

- [x] Test: `/hidden/` and `/days/YYYY-MM-DD/hidden/` show the hidden cards
  of their day with the reasons; `/?hidden=1` shows today without the hidden
  ones.
- [x] Test on fixtures: snapshots of three days, one of them without one of
  the lists; each day shows exactly its own last snapshots, and the missing
  list is named for that day.
- [x] Test: snapshots taken at 23:30 and 00:30 local time go to two different
  days, whatever the UTC date of the file.
- [x] Test: a repository marked seen today is hidden on yesterday's page and
  shown with its reason under "Show hidden".
- [x] Test: after "Mark all shown as seen" on a day, opening that day again a
  month later shows nothing new.
- [x] Test: "Mark all shown as seen" on a past day marks exactly what that
  page showed, and its marks name the lists of that day.
- [x] Test: opening a past day makes no request to the sources.
- [x] The calendar offers exactly the days that have snapshots, as links to
  `/days/YYYY-MM-DD/`; a malformed date or a day without snapshots answers
  404 with a link to today.
- [x] The guide in `docs/content/` describes the calendar with an example.

## Out of scope

- Comparing two days or showing what changed between them.
- Pruning or exporting the history.
