---
code: PUL-10
id: I_kwDOJvS0hM8AAAABg_8ldw
databaseId: 6509503863
number: 94
url: https://github.com/octomation/pulse/issues/94
title: "web: flow mode, one card at a time"
issueType: Feature
labels:
  - "type: feature"
  - "scope: code"
  - "impact: high"
  - "effort: medium"
assignees:
  - kamilsk
milestone: "[[PUL-M1]]"
parent: null
issueFields: {}
state: OPEN
stateReason: null
duplicateOf: null
createdAt: 2026-11-20T10:22:46Z
updatedAt: 2026-11-20T10:28:08Z
lastEditedAt: 2026-11-20T10:28:08Z
closedAt: null
---

# web: flow mode, one card at a time

## Motivation

I go through the page card by card, marking each one. Every mark removes a
card, the grid moves, and its rows are not aligned, so "Interesting" and
"Skip" land in a different place each time: I have to look for the button
before every decision, and the rest of the page distracts me.

I want a focused mode that shows one card at a time with the actions always
in the same place, so I can answer "interesting or not" quickly, card after
card, without the noise around it.

## PoC

```bash
$ open http://pulse.localhost:17842/flow/                   # today's cards
$ open http://pulse.localhost:17842/days/2026-11-19/flow/   # the cards of that day
```

```text
                      3 of 28
  ┌───────────────────────────────────────────┐
  │ GO · IN 2 LISTS                           │
  │ Tencent                                   │
  │ LoopForge                                 │
  │ Self-hosted workflows for coding-agent…   │
  │ trendshift daily #1 · trendshift weekly #7│
  └───────────────────────────────────────────┘
   ← Skip        ↓ Later        Interesting →
```

## Behavior

- Flow mode shows the cards of the page one at a time, in the order of the
  page: the front page first, then "Today", "This week" and "This month".
  It shows only what the page shows by default: hidden repositories are not
  in the flow.
- Each card has three answers, always in the same place on the screen:
  - **Skip**, swipe or arrow left: marks the repository as seen, as "Skip"
    on the page.
  - **Interesting**, swipe or arrow right: marks it and bookmarks it, as
    "Interesting" on the page, with an optional comment.
  - **Later**, arrow down: marks nothing and goes to the next card, for when
    I do not want to decide now. The card stays on the page and is in the
    next flow.
- The next card appears as soon as the answer is written; an answer that
  cannot be written keeps the card on the screen with the error.
- The mode shows how many cards are left. When none is left, it says so and
  links back to the page.
- The reader of the repository opens from the card and returns to the same
  card.
- The mode has its own path, not a query parameter: `/flow/` for today and
  `/days/YYYY-MM-DD/flow/` for a day of the calendar ([[PUL-6]]), over the
  cards of that day.
- The keys work without the mouse, and swipes work on a touch screen.

## Limits

- An answer cannot be taken back from the flow, as on the page.
- "Mark all shown as seen" is not offered in the flow.
- Later is not stored: a card left for later is in the flow again the next
  time, in its place, with the other cards without a mark.

## Acceptance criteria

- [ ] Test: the flow holds exactly the cards the page shows by default, in
  the order of the page.
- [ ] Test: left marks the repository as skipped, right marks it as
  interesting with the comment, down marks nothing and shows the next card;
  a new flow holds the card again.
- [ ] Test: a failed write keeps the card and shows the error.
- [ ] Test: the last answer shows the end of the flow with a link to the
  page.
- [ ] The answers stay in the same place on the screen for cards with a
  short and a long description, checked in a browser.
- [ ] The guide in `docs/content/` describes flow mode and its keys.

## Out of scope

- Undoing an answer.
- Snoozing for a chosen period from the flow ([[PUL-9]] sets the period).
