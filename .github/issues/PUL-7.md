---
code: PUL-7
id: I_kwDOJvS0hM8AAAABg_MWBQ
databaseId: 6508713477
number: 91
url: https://github.com/octomation/pulse/issues/91
title: "core: hide through a facade, the vault as its first provider"
issueType: Feature
labels:
  - "type: feature"
  - "scope: code"
  - "impact: high"
  - "effort: medium"
assignees:
  - kamilsk
milestone: "[[PUL-M2]]"
parent: null
issueFields: {}
state: OPEN
stateReason: null
duplicateOf: null
createdAt: 2026-11-20T09:31:20Z
updatedAt: 2026-11-23T09:58:08Z
lastEditedAt: 2026-11-23T09:35:57Z
closedAt: null
---

# core: hide through a facade, the vault as its first provider

## Motivation

On 2026-11-20 pulse kept showing 21 of 50 repositories that were already in
the vault: the configuration pointed at an old clone of the vault, and the page
said only "Vault: 2643 repositories in 2576 notes", without the path. Nothing
told me that the notes I had just restored were not the ones pulse read.

The answer to "may this repository be hidden, and why" should come from one
place with one contract, so that the vault can later be answered by indexit
([[PUL-8]]) instead of pulse reading the
files itself, and other reasons can follow.

## PoC

```bash
$ open http://pulse.localhost:17842/
Vault: 2712 repositories in 2645 notes · ~/Reflection/Mindset/4. Resources/Collections/Bookmarks/GitHub · newest note 2026-11-20 12:05
```

```go
// A provider takes a slice of repositories and answers, for each, the
// reasons it may be hidden; the answer has the length and order of items.
type Provider interface {
	Check(ctx context.Context, items []Item) ([][]Reason, Status)
}
```

## Behavior

- What may hide a card goes through one facade. In this task the vault is its
  only provider: the current reader of the bookmarks directory, unchanged in
  what it finds.
- A provider is given all the repositories of the view at once and answers,
  for each one, zero or more reasons with the source, a reference (the note)
  and a time; it also answers its own status: off, ok or failed, with the
  origin it read (a path or a URL), the number of entries, the time of its
  newest entry and the error.
- Which implementation answers for the vault is chosen by the configuration;
  the page and the cards do not depend on it.
- The page names every source with its origin, its count and its newest
  entry, so a wrong or stale path is visible at a glance.
- The page warns, without failing, when the configured directory is not
  inside a vault that Obsidian knows on this Mac.
- A failed provider fails open: the cards stay shown, the page says that the
  list may include what is already in that source, and it never says
  "nothing new" without that reservation.
- The reasons of a hidden card and the counts in the masthead are the same as
  before this change.

## Limits

- Only the vault goes behind the facade now; seen marks and bookmarks keep
  their current code.
- The vault directory is read flat, as today: notes in subdirectories are
  not read.
- The Obsidian check reads the list of vaults Obsidian keeps; a vault Obsidian
  never opened produces the warning.

## Acceptance criteria

- [x] The tests of the vault filter from [[PUL-5]] pass through the
  facade unchanged.
- [x] Test: a provider that fails leaves its repositories shown, and the page
  names the source and the error.
- [x] Test: the page shows the origin path, the count and the newest note of
  the vault.
- [x] Test: a configured directory outside every vault Obsidian knows gives
  the warning; one inside a known vault does not.
- [x] Test: a fake provider swapped in through the configuration hides its
  repositories with its own reasons, with no change to the page code.
- [x] Live check on this Mac: with the path fixed, the 21 repositories of the
  restored notes are hidden with "In the vault".
- [x] The guide in `docs/content/` describes the vault line and the warning.

## Out of scope

- Reading the vault through indexit: [[PUL-8]].
- Seen marks and bookmarks as providers.
