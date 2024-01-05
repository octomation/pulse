---
code: null
id: I_kwDOJvS0hM8AAAABg7ewAQ
databaseId: 6504820737
number: 85
url: https://github.com/octomation/pulse/issues/85
title: "core: trend-watch without repeats"
issueType: Feature
labels:
  - "type: feature"
  - "scope: code"
  - "impact: high"
  - "effort: hard"
assignees:
  - kamilsk
milestone: null
parent: null
issueFields: {}
state: CLOSED
stateReason: COMPLETED
duplicateOf: null
createdAt: 2026-11-20T04:31:06Z
updatedAt: 2026-11-20T07:43:24Z
lastEditedAt: 2026-11-20T07:12:53Z
closedAt: 2026-11-20T07:43:24Z
---

# core: trend-watch without repeats

## Motivation

Every morning I compare the GitHub trends of several sources. The lists repeat
each other and themselves: most of today's repositories were there yesterday,
and many are already in my bookmarks. I keep what is interesting in Safari tabs
and turn the tab group into bookmarks later.

I need one window that shows only what I have not seen, and a bookmark in one
click. This is the prototype of pulse v0.1.0.

## PoC

```bash
$ pulse
pulse listens on http://pulse.localhost:17842/
$ open http://pulse.localhost:17842/        # only the repositories that are new to me
$ pulsectl bookmarks                        # URLs, one per line, as copied from a Safari tab group
https://github.com/owner/name
$ pulsectl bookmarks --format jsonl         # the same bookmarks with their comments
{"url":"https://github.com/owner/name","repo":"owner/name","comment":"try for the hub","bookmarked_at":"2026-11-19T08:15:00Z","lists":["trendshift/daily","github/weekly"]}
```

## Behavior

### Lists

pulse watches six lists, each a source and a period:

| List | Request | Period marker in the answer |
| --- | --- | --- |
| trendshift/daily | `https://trendshift.io/` | canonical link `https://trendshift.io`, current navigation item `Daily` |
| trendshift/weekly | `https://trendshift.io/weekly` | canonical link `…/weekly`, current navigation item `Weekly` |
| trendshift/monthly | `https://trendshift.io/monthly` | canonical link `…/monthly`, current navigation item `Monthly` |
| github/daily | `https://github.com/trending?since=daily` | `Date range: Today`, rows say `stars today` |
| github/weekly | `https://github.com/trending?since=weekly` | `Date range: This week`, rows say `stars this week` |
| github/monthly | `https://github.com/trending?since=monthly` | `Date range: This month`, rows say `stars this month` |

All six were available on 2026-11-19, and the three periods of each source
returned different rankings.

- A list is the ranking of the page only: sponsored and featured repositories
  of trendshift.io are not part of it.
- pulse reads the period marker from the answer and compares it with the
  period it asked for. An answer whose marker is missing or names another
  period is an error of that list: the same output under another label is not
  another period.

### Snapshots

- Every successful load of a list is stored as a snapshot: the source, the
  period, the request, the period marker the source returned, the time, and
  the repositories in the order of the list.
- Opening the page loads a list from the network only when its last attempt,
  successful or not, is more than an hour old. "Refresh" on the page loads all
  six lists now. The hour is counted across restarts.
- A failed request, a period mismatch or a page with no repositories is an
  error of that list, shown with its reason and time. It is never shown as
  "nothing new". The other lists keep working. The last successful snapshot of
  the failed list stays in the view, labelled with its time.
- The page says "nothing new" without reservations only when no list is in
  error.

### The page

- `pulse` serves one page at `http://pulse.localhost:17842/`. It listens on
  loopback only, `127.0.0.1` and `::1`, and refuses any other address and any
  request for another host.
- The view is the union of the latest successful snapshot of every list. A
  repository is shown once, whatever number of lists it is in. Its key is
  `owner/name`, compared without case. Its card names every list it is in,
  with its position there.
- By default the page shows only the repositories that are neither seen nor
  in the vault bookmarks. A switch shows the hidden ones too, each with the
  reason: seen (when and how), bookmarked in pulse, or in the vault.
- The page reads like a newspaper: a masthead with the date and the counts,
  a front page with the repositories that are in most lists, the first of
  them as the lead story, and then the sections "Today", "This week" and
  "This month". A repository is in one section only.
- Each card has "Interesting", with an optional comment, and "Skip". The page
  has "Mark all shown as seen", which marks exactly the repositories the page
  showed when it was rendered, not what a later load would add.
- Only these three actions mark a repository as seen. Opening the page, the
  hidden view, a refresh or an export mark nothing.
- A seen mark has no expiry: a repository seen a month ago that trends again
  stays hidden.

### The reader

- A click on a repository opens it in the reader, a panel of the same page:
  no new tab and no visit to github.com are needed to decide.
- The reader shows what the lists know (the description, the language, the
  lists with the positions) and what GitHub knows: the stars, the forks, the
  open issues, the license, the topics, the homepage, the dates of creation
  and of the last push, and the README as GitHub renders it.
- pulse asks the GitHub API for these on demand, when a repository is opened,
  and keeps the answer for five minutes. With `GITHUB_TOKEN` or `GH_TOKEN` in
  the environment it sends the token to `api.github.com` only; without one
  GitHub allows about 30 repositories an hour.
- The README is shown in a frame without scripts; its links open outside the
  page.
- When GitHub does not answer, the reader says why and still shows what the
  lists know, with the actions.
- The reader has the same "Interesting", comment and "Skip" as the card, and
  moves to the next repository after either. Opening the reader marks
  nothing.

### Bookmarks

- "Interesting" marks the repository as seen and creates a pulse bookmark:
  the repository, its URL, the comment, the time, and the lists it was in.
  The bookmark and the mark are one record on disk: when it cannot be
  written, the action fails and the repository stays in the view.
  "Interesting" on a repository that already has a bookmark replaces the
  comment.
- `pulsectl bookmarks` asks the running service and prints the bookmarks,
  oldest first: by default `https://github.com/owner/name`, one per line;
  with `--format jsonl`, one JSON object per line with the comment. Without a
  running service it fails with a message that says so.

### Vault

- The path of the vault bookmarks directory is in the configuration file of
  pulse, outside the repository. pulse only reads that directory.
- A repository is in the vault when a note of the directory names it in the
  `url` of its frontmatter (`https://github.com/owner/name`) or in one of its
  `aliases` of the form `owner/name`. Other aliases and URLs of owners are
  ignored. The comparison is without case.
- The notes are read again for every page, so a bookmark added to the vault
  hides its repository on the next load.
- A configured directory that cannot be read, or has no note with a
  repository, is an error on the page: the page says that the list may include
  what is already in the vault. Without a configured path the page says that
  the vault filter is off.

### State

- The configuration is `$XDG_CONFIG_HOME/pulse/config.toml`
  (`~/.config/pulse/config.toml`), or the file `PULSE_CONFIG` names.
- Snapshots, marks and bookmarks live in `$XDG_DATA_HOME/pulse`
  (`~/.local/share/pulse`), or the directory `PULSE_DATA_DIR` names: plain
  files that survive a restart. Nothing is stored in the repository.
- A mark is written to disk before the page reports it as done.

## Limits

- The six lists are fixed. A renamed or transferred repository is a new one.
- A seen mark cannot be taken back, and a bookmark cannot be deleted from the
  page.
- The page waits for the lists it loads; a load that takes longer than its
  timeout is an error of that list.
- The service is started by hand: no `brew services` yet.

## Acceptance criteria

- [x] Each of the six lists is loaded with its own request, and a stored
  fixture of each keeps the request and the period marker of the source; a
  test fails when two periods of one source hold the same ranking or a marker
  does not match its label.
- [x] The parsers of both sources pass on the saved HTML of all six lists:
  each fixture keeps the complete ordered list of the repositories of its
  ranking, checked independently of the parser, and the parser returns
  exactly that list, without the sponsored and featured repositories of
  trendshift.io.
- [x] Test on fixtures: yesterday's list has 10 repositories, all marked as
  seen; today's has 8 of them and 2 new ones; the page shows exactly those 2.
- [x] Test: a repository from the vault bookmarks and a repository seen a
  month ago, both trending again, are not shown, and the hidden view names
  the reason for each.
- [x] Test: a repository in several lists, under different letter case, is
  one card that names all its lists.
- [x] Test: opening the page marks nothing and loads a list at most once an
  hour; "Refresh" loads it now; the hour holds across a restart.
- [x] Test: a failed or empty list is an error of that list, the others are
  shown, and the page does not say "nothing new".
- [x] Test: "Interesting" with a comment creates a bookmark that survives a
  restart and comes out of `pulsectl bookmarks` as a URL line and as JSONL
  with the comment.
- [x] Test: when the record of "Interesting" cannot be written, the action
  fails, no bookmark is exported and the repository is still shown.
- [x] `pulse` refuses to listen on an address that is not loopback.
- [x] Live run on a separate temporary data directory with the configured
  vault directory: all six lists come from the network; in the browser, one
  repository is marked as interesting with a comment and the rest as seen;
  after a restart on the same snapshots nothing is new, and the bookmark with
  its comment is exported. The working state is left untouched.
- [x] Test: opening a repository in the reader marks nothing and shows the
  facts and the README of GitHub; a README cannot run scripts, and its
  relative links and images point to github.com.
- [x] Test: when GitHub does not answer, the reader shows what the lists
  know, the reason and the actions.
- [x] Test: the front page and the sections hold every new repository
  exactly once.
- [x] `make verify` is green.
- [x] `docs/content/` describes the prototype as it is, without presenting
  it as released.

## Out of scope

Snapshots on a schedule and a digest (v2), comments generated from my
interests, other sources, GitHub stars, writing to the vault, `brew services`,
the release.
