---
code: PUL-13
id: I_kwDOJvS0hM8AAAABh22gVw
databaseId: 6567075927
number: 100
url: https://github.com/octomation/pulse/issues/100
title: "web: decide, see and take back without leaving the flow"
issueType: Feature
labels:
  - "effort: medium"
  - "impact: high"
  - "scope: code"
  - "type: improvement"
assignees:
  - kamilsk
milestone: "[[PUL-M3]]"
parent: null
issueFields: {}
state: OPEN
stateReason: null
duplicateOf: null
createdAt: 2026-11-23T19:24:34Z
updatedAt: 2026-11-23T19:24:34Z
lastEditedAt: null
closedAt: null
---

# web: decide, see and take back without leaving the flow

## Motivation

pulse is one window onto sources that change every day, and a bridge into my
notes: I go through what is new, mark what interests me, and `/bookmark-it`
turns the marks into notes of my vault. v0.2.0 makes me leave the context for
every decision that needs more than a description:

- A flow card shows only the surface of a repository, and it is not obvious
  that it can be clicked. The facts and the README are in the reader, and the
  reader that opens over the flow has no answers: I close it before I answer.
- On the hidden page I cannot tell a repository I marked as interesting from
  one I skipped, and every hidden card still offers **Interesting**, even one
  that a note of my vault already names.
- An answer cannot be taken back, not even a slip of the key a second ago, and
  "Mark all shown as seen" has no undo.
- `pulsectl bookmarks` prints every bookmark forever, also the ones that
  already have a note, so the list I hand to `/bookmark-it` never gets shorter.

I want to decide in one move with the details in front of me, see what I
decided before, and change my mind in one move.

## PoC

```text
                    3 of 28 · 26 left
  GO · yetone
  magpie
  Every agent's model. One place.
  40,418 STARS  3,709 FORKS  MIT  SEP 2026 SINCE  NOV 23, 2026 LAST PUSH
  claude-code · codex · deepseek · llm        github monthly #10
  github.com/yetone/magpie · usemagpie.ai
  ┌ README ─────────────────────────────────┐
  │ …                                       │
   ← Skip     ↓ Later     ↑ Back     Interesting →     c comment
```

```bash
$ open http://pulse.localhost:17842/hidden/  # Interesting · Skipped · In your notes
$ open http://pulse.localhost:17842/queue/   # interesting, no note yet
$ pulsectl bookmarks                         # only what has no note yet
$ pulsectl bookmarks --all                   # every bookmark, as before
```

## Behavior

### The flow card is the whole story

- A flow card shows what the reader shows, without a click: the facts, the
  topics, the lists, the links and the README below them. The reader does not
  open over the flow any more.
- The answers stay in the bar and answer at any time: ← Skip, → Interesting,
  ↓ Later, ↑ Back, and `u` Take back for a card that has an answer. A click
  into the README does not take the arrows away.
- The comment opens on demand: `c` or **Comment**; Enter answers Interesting
  with it, Escape closes it.
- ↑ goes back to the previous card of this flow. The card shows the answer it
  got, with its comment, and another answer replaces it; the same answer
  writes nothing. Take back makes the card new and keeps it on the screen.

### A card says where it stands

- The last answer counts: a later answer replaces an earlier one, also a skip
  after Interesting. **Take back** removes the answer: the repository is new
  again unless a note names it.
- Giving the answer a repository already has writes nothing: a second Skip
  does not extend the snooze. Interesting with another comment saves the
  comment.
- The hidden page, the reader and the flow show a stamp: **Interesting**,
  **Skipped until …**, **Seen until …** for "Mark all shown as seen", or
  **In your notes**. The hidden page groups its cards under these three.
- A repository that a note of the vault names offers no **Interesting** and
  no **Skip**; it offers **Take back** when pulse holds an answer for it.
  Taking it back changes nothing in the vault.
- A repository whose snooze ended says in the reader when it was skipped.
- After "Mark all shown as seen", the page offers **Undo**, which takes back
  exactly the marks it wrote, also on the hidden page; a repository answered
  since, even by a later "Mark all shown as seen", keeps its answer.

### The queue drains into notes

- `pulsectl bookmarks` prints the repositories marked as interesting that no
  note names yet, oldest first; `--all` prints every bookmark, as before.
  When the vault, or one of its notes, cannot be read, it fails and says so
  instead of printing what may have a note.
- `/queue/` lists the same repositories, also those that left the lists, with
  their comment and **Take back** on each; the reader opens from there.

## Limits

- pulse still only reads the vault: it neither writes nor moves notes.
- Later is not stored, as before.
- A data directory with a taken-back answer cannot be opened by an older
  pulse: it refuses the unknown record.
- The trail of ↑ lives in the open flow; a reload starts a new one.
- Undo lasts until the page is left or loaded again by hand.
- Text of the README is copied on GitHub: a click into it gives the keys
  back to the flow.

## Acceptance criteria

- [ ] Test: the flow card holds the facts and the README of the repository,
  and the flow page has no reader panel.
- [ ] Test: the last answer wins, Take back makes a repository new, and a
  repeated answer writes nothing.
- [ ] Test: the stamps of Interesting, Skipped, Seen and In your notes, and
  the hidden page grouped under them.
- [ ] Test: a repository a note names has no Interesting and no Skip in the
  reader.
- [ ] Test: Undo after "Mark all shown as seen" takes back exactly its marks.
- [ ] Test: `pulsectl bookmarks` leaves out a bookmark with a note, `--all`
  keeps it, and an unreadable vault fails the command.
- [ ] Test: `/queue/` lists a bookmark that left the lists, and Take back
  removes it.
- [ ] Checked in a browser: ← → ↓ ↑ answer after a click into the README; ↑
  shows the previous answer and its comment, another key replaces it, and
  `u` takes it back.
- [ ] The guide in `docs/content/` describes the stamps, Take back, Undo, ↑,
  the queue and the new default of `pulsectl bookmarks`.

## Out of scope

- What happens to a note when its interest is taken back.
- A flow over the hidden repositories.
- Sources other than GitHub.
