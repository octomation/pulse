---
code: PUL-11
id: I_kwDOJvS0hM8AAAABhsn5-Q
databaseId: 6556350969
number: 98
url: https://github.com/octomation/pulse/issues/98
title: "web: a front page grid without holes"
issueType: Feature
labels:
  - "type: improvement"
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
createdAt: 2026-11-23T09:35:10Z
updatedAt: 2026-11-23T14:15:02Z
lastEditedAt: 2026-11-23T10:43:55Z
closedAt: 2026-11-23T14:15:02Z
---

# web: a front page grid without holes

## Motivation

The page leaves holes and jumps. The lead of the front page takes two
columns and two rows of a three-column grid and leaves a third of the row
empty; each section is a grid of its own, so a section of four cards ends
with two empty boxes. Every card repeats a comment field and two buttons,
so the page is at once sparse and busy, and the boxes and buttons sit at a
different height in every row.

I want the page to read as a newspaper page: one grid from top to bottom,
an even vertical rhythm, nothing to look for. The decisions belong to the
reader and to flow mode ([[PUL-10]]), where the answers are always in one
place; the page is for reading. Masonry is not the answer: it packs
pictures of very different heights, the cards here differ by a line or two,
and it would scramble the order of the ranks.

## PoC

```text
FRONT PAGE ────────────────────────────────────────────────────────
 PYTHON · IN 3 LISTS                          │ TYPESCRIPT · IN 3 LISTS
 Claude-BugHunter                             │ stitch-skills
 A Claude Code skill bundle for bug hunting…  │ A library of Agent Skills…
 ○ trendshift daily #17 ■ github daily #1     │ ■ github daily #11
                                              │ plannotator …
 t3code          │ minimind        │ last30days-skill
TODAY ─────────────────────────────────────────────────────────────
 RUST            │ SHELL           │ PYTHON          ← kickers on one line
 nectarlink      │ superpowers     │ mobile-agent-…  ← names on one line
 Your Android…   │ An agentic…     │ 20 AI testers…  ← three lines at most
 ○ trendshift #2 │ ■ github #2     │ ○ trendshift #3 ← lists on one line
```

## Behavior

- The page is one three-column grid from the front page to the last
  section: two columns on a narrow screen, one on a phone. No section
  changes it.
- A card is text only: the kicker, the name, the description and the lists
  (and the reasons on a hidden card). Its parts line up with the parts of
  the other cards of its row, on a step of 8 pixels.
- A card has no answers and no comment field: a click on its name or
  description opens the reader, which has them, and flow mode has them.
  "Mark all shown as seen" and "Flow" stay in the masthead.
- No boxes around the cards: a hairline between the rows. A row that is not
  full ends the section.
- A card shows at most three lines of the description, the lead five; the
  reader shows it whole.
- The front page: the lead takes two columns, the next two stories stand in
  the third beside it, the rest go on in rows.
- "In one list" is not written: the kicker names the lists only when there
  are several.
- On a past day the sections are named for that day: "That day", "That
  week", "That month" instead of "Today", "This week", "This month".
- The order of the cards, the reader with its keys and flow mode do not
  change.

## Acceptance criteria

- [x] Checked in a browser at 1200, 900 and 390 pixels: every section on
  the same columns, the kickers, names, descriptions and lists of a row on
  one line, with sections of 1, 2, 4 and 50 cards and the front page with
  the lead alone and with one to six cards more.
- [x] Test: the cards are in the page once each, in the order of the ranks,
  without answers, and the reader walks them in that order and answers
  them.
- [x] Test: a past day names its sections for that day.
- [x] The guide in `docs/content/` shows the page as it is and says where
  the answers are.

## Out of scope

- Flow mode ([[PUL-10]]) and the reader, beyond being the place of the
  answers.
- A setting for the number of columns or for the density of the page.
