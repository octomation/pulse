---
code: PUL-9
id: I_kwDOJvS0hM8AAAABg_rpGA
databaseId: 6509226264
number: 93
url: https://github.com/octomation/pulse/issues/93
title: "core: a seen mark snoozes for a quarter"
issueType: Feature
labels:
  - "type: feature"
  - "scope: code"
  - "impact: medium"
  - "effort: easy"
assignees:
  - kamilsk
milestone: "[[PUL-M1]]"
parent: null
issueFields: {}
state: OPEN
stateReason: null
duplicateOf: null
createdAt: 2026-11-20T10:04:29Z
updatedAt: 2026-11-22T16:31:15Z
lastEditedAt: 2026-11-22T16:31:15Z
closedAt: null
---

# core: a seen mark snoozes for a quarter

## Motivation

A repository I skip today may be skipped only because its direction does not
interest me now. My interests change, and a repository that trends again in a
few months deserves another look. Today a seen mark has no expiry
([[PUL-5]], "The page"), so a skipped repository is hidden for good.

"Seen" should work as a snooze: it hides the repository for a quarter, and
when the repository is back on the lists after that, I see it again and
decide anew. What I have kept, a note in the vault or a pulse bookmark, stays
hidden for good.

## PoC

```bash
$ open http://pulse.localhost:17842/        # 2026-11-20: Skip owner/name
$ open http://pulse.localhost:17842/        # 2027-01-15: owner/name trends again, still hidden
                                            #   Show hidden: "Skipped on Nov 20, 2026, until Feb 20, 2027"
$ open http://pulse.localhost:17842/        # 2027-02-21: owner/name trends again and is shown
```

## Behavior

- "Skip" and "Mark all shown as seen" hide the repository for three calendar
  months from the mark, by default; the configuration may set another
  period.
- After the period the repository is shown again if a list has it. A new
  mark snoozes it for a new period from that mark; the earlier marks stay in
  the log.
- "Interesting" never expires: it creates a pulse bookmark, which hides for
  good, as a note in the vault does.
- A repository in the vault stays hidden whatever its marks.
- Under "Show hidden", a snoozed repository says when it was seen, how, and
  until when it is hidden.
- The same rule holds for every day of the calendar ([[PUL-6]]): a past
  day is judged by the marks as they stand now, so an expired snooze shows the
  repository on that day too.
- The marks already on disk follow the new rule: a mark older than the period
  no longer hides.

## Limits

- One period for all repositories: no per-repository or per-topic period.
- A snooze cannot be extended or taken back from the page; a new mark after
  it expires starts a new one.

## Acceptance criteria

- [ ] Test: a repository skipped three months less a day ago is hidden; one skipped three
  months and a day ago is shown again when a list has it.
- [ ] Test: "Mark all shown as seen" expires the same way as "Skip".
- [ ] Test: a repository marked "Interesting" a year ago stays hidden.
- [ ] Test: a repository in the vault with an expired skip stays hidden, with
  the vault as the reason.
- [ ] Test: a repository skipped again after its snooze expired is hidden for
  a new period from the second mark.
- [ ] Test: a configured period of one month replaces the default.
- [ ] The hidden view names the end of the snooze.
- [ ] The guide in `docs/content/` describes the snooze and its period.

## Out of scope

- Snoozing from the page for a chosen period.
- Expiry of pulse bookmarks or of vault notes.
