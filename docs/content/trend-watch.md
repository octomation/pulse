---
title: Trend watch
description: One local page at http://pulse.localhost:17842/ with the trending GitHub repositories you have not seen or bookmarked, a reader for each of them, a calendar of past days, a flow mode with one card at a time, and pulsectl bookmarks to export the ones you marked as interesting.
---

# Trend watch

`pulse` serves one page at `http://pulse.localhost:17842/` that joins six
lists of trending GitHub repositories, the daily, weekly and monthly trends of
[trendshift.io](https://trendshift.io/) and
[github.com/trending](https://github.com/trending), and shows only the
repositories that are new to you: the ones you have not marked as seen on the
page and do not keep in your bookmark notes. One click bookmarks a repository,
and `pulsectl bookmarks` prints what you bookmarked.

## Install it

On macOS, with [Homebrew](https://brew.sh/):

```sh
brew install --formula octolab/tap/pulse
brew services start octolab/tap/pulse
```

The Formula installs both binaries, the service `pulse` and its client
`pulsectl`, and the second command starts the service in the background, as
your user and without `sudo`. From then on it starts when you log in, so the
page is always at `http://pulse.localhost:17842/`.

On macOS or Linux, on `amd64` or `arm64`, you can also take
`pulse_0.2.1_<os>-<arch>.tar.gz` from the
[v0.2.1 release](https://github.com/octomation/pulse/releases/tag/v0.2.1), or
install with Go 1.27 or later, and [run pulse in a terminal](#in-a-terminal):

```sh
go install go.octolab.org/toolset/pulse/cmd/...@v0.2.1
```

Check what you installed with `pulse version`.

## Point it at your bookmarks

This step is optional. pulse can hide the repositories you already keep as
bookmark notes in your notes vault: a directory of Markdown notes that name
repositories. Name the directory in `~/.config/pulse/config.toml`:

```toml filename="~/.config/pulse/config.toml"
[vault]
bookmarks = "~/Notes/Bookmarks/GitHub"
```

A note names a repository in its frontmatter, by its `url` or by an
`owner/name` entry of its `aliases`; either is enough:

```md filename="cobra.md"
---
url: https://github.com/spf13/cobra
aliases:
  - spf13/cobra
---
```

- Names are compared without case. A URL of an owner or of another site, and
  an alias that is not `owner/name`, name no repository.
- pulse reads the frontmatter of the `.md` files directly in the directory,
  not in its subdirectories, and never writes there.
- The notes are read again every time the page loads, so a note you add hides
  its repository on the next load.
- `~` is your home directory; a relative path is resolved against the
  directory of the config file.
- With an unknown key in the file, pulse reports it and does not start.

The page names the directory it reads, how many repositories its notes name
and when the newest note changed, so a wrong or stale copy of your notes shows
at a glance:

```text
Vault: 2712 repositories in 2647 notes · ~/Notes/Bookmarks/GitHub · newest note Nov 22, 2026, 20:45.
```

When the directory is not inside a vault that [Obsidian](https://obsidian.md/)
knows on this computer, the page adds a warning below that line, as in
`Vault: the directory is not inside any vault Obsidian knows on this computer, so it may be an old copy.`,
and still hides what the notes name. pulse checks the list of vaults Obsidian
keeps, so a vault Obsidian has never opened gives the warning too.

Without the file, the filter is off and the page says so. When the directory
cannot be read, or none of its notes names a repository, the page says that
the list may include what is already in the vault, and it never says
**Nothing new** without that reservation.

pulse reads the file when it starts: after you change it, restart the
service.

## Choose how long a skip lasts

**Skip** and **Mark all shown as seen** hide a repository for three calendar
months. To change that, set the period in the config file, as a whole number
of `days`, `weeks`, `months` or `years`:

```toml filename="~/.config/pulse/config.toml"
[seen]
snooze = "1 month"
```

The period applies to the marks you already have, too. **Interesting** is not
affected: a bookmark hides its repository as long as it is your answer.

## Start it

With Homebrew, the service runs in the background:

| Command | What it does |
| --- | --- |
| `brew services start octolab/tap/pulse` | Starts pulse now and at every login |
| `brew services restart octolab/tap/pulse` | Restarts it: after an upgrade, or after you change the config file |
| `brew services stop octolab/tap/pulse` | Stops it, and it no longer starts at login |

Open `http://pulse.localhost:17842/` in a browser: `.localhost` names resolve
to your own computer, so there is nothing to set up. What pulse says when it
starts goes to its log, `$(brew --prefix)/var/log/pulse.log`:

```text
pulse listens on http://pulse.localhost:17842/
data: /Users/you/.local/share/pulse
vault: /Users/you/Notes/Bookmarks/GitHub (config /Users/you/.config/pulse/config.toml)
seen: Skip and Mark all shown as seen hide a repository for 3 months
github: the reader asks api.github.com without a token, about 30 repositories an hour
```

Without a config file, the third line is
`vault: not configured (no config at /Users/you/.config/pulse/config.toml)`.

### In a terminal

Without Homebrew, or to try another port, run the service yourself; it runs
until `Ctrl-C`:

```sh
pulse
```

Only one pulse can listen on a port: stop the background service first, or
use another port.

To use another port, pass a loopback IP address with a port; the page is then
at `http://pulse.localhost:8080/`:

```sh
pulse --listen 127.0.0.1:8080
```

## The page

The page reads like a morning paper:

- The masthead has the date, which opens the [calendar](#past-days), the
  counts, as in `54 new · 30 hidden`, and **Show hidden**, **Flow**,
  **Queue**, **Refresh** and **Mark all shown as seen**.
- Below it are the six lists, each with the time of its last successful load
  and the number of its repositories, and the state of the vault filter.
- **Front page** holds the repositories that are in most lists at once: the
  first of them is the lead story across two columns, the next two stand
  beside it. **Today**, **This week** and **This month** hold the others,
  each in the section of its shortest period. On a [past day](#past-days)
  they are **That day**, **That week** and **That month**.

The cards stand in three columns from the front page to the last section,
two on a narrow screen and one on a phone. A repository is one card, whatever
number of lists it is in; `owner/name` is compared without case. The card
shows its language, how many lists it is in when there are several, its
description in up to three lines (five on the lead), and one label per list
with its position there: `trendshift daily #8`, `github daily #6`.

A card is for reading: click its name or its description to open the
[reader](#the-reader), which has the answers and the whole description, or
go through the cards one at a time in [flow mode](#flow-mode).

| Action | What it does |
| --- | --- |
| **Interesting** | Bookmarks the repository, with the comment you typed in the field beside it (optional, up to 500 characters), and hides it |
| **Skip** | Marks the repository as seen and hides it for three months |
| **Take back** | Removes the answer the repository has: it is new again |
| **Mark all shown as seen** | Marks every repository the page shows at that moment as seen, for three months; **Undo** beside its message takes those marks back, except for a repository you answered since |

Only these actions mark a repository: opening the page, the reader,
**Show hidden**, **Flow**, **Queue**, **Refresh** and an export mark
nothing. A card leaves the page once its mark is written to disk; if it
cannot be written, the reader stays on the repository and shows the error.

Your last answer counts. **Skip** after **Interesting** removes the bookmark,
**Interesting** after **Skip** ends the snooze, and **Take back** removes
either. Giving the answer a repository already has writes nothing: a second
**Skip** does not move the end of the snooze, and **Interesting** with
another comment replaces the comment.

A seen mark is a snooze. When the three months are over and the repository
trends again, it is back on the page and you decide anew; the reader says
when you skipped it, and a new **Skip** hides it for another three months
from then. A repository stays hidden while **Interesting** is its answer, or
while a note of your vault names it.
[The config file](#choose-how-long-a-skip-lasts) sets another period.

### The reader

Click the name or the description of a repository to read about it without
leaving the page. The reader opens beside the paper and shows:

- the stars, the forks, the open issues, the license, the month the
  repository was created and the day of its last push;
- its topics, its lists, and links to github.com and to its homepage;
- the README as GitHub renders it. Its links open in a new tab.

The reader has the comment field, **Interesting** and **Skip**; after
either it moves to the next repository. A repository that has an answer
shows it as a stamp, such as `★ Interesting` or `Skipped · until Feb 19,
2027`; its answer is pressed, and **Take back** removes it. **Next** and
**Previous**, or `j` and `k`, move between repositories, and `Esc` closes
the reader.

pulse asks `api.github.com` for a repository when you open it and keeps the
answer for five minutes. Without a token GitHub allows about 30 repositories
an hour. To lift that, give pulse a GitHub token in its config file; a token
without any permission is enough for public repositories:

```toml filename="~/.config/pulse/config.toml"
[github]
token = "github_pat_..."
```

The token is kept there as plain text: keep the file readable by you only,
`chmod 600 ~/.config/pulse/config.toml`. pulse sends the token to
`api.github.com` only, never prints it, and says at start where it comes
from:

```text
github: the reader asks api.github.com with the token of the config /Users/you/.config/pulse/config.toml
```

A pulse you run in a terminal also takes the token of `GITHUB_TOKEN` or
`GH_TOKEN`, and that one comes first. The background service does not see
the variables of your shell, so it needs the config file.

When GitHub does not answer, the reader says why and still shows what the
lists know, with the actions.

### Hidden repositories

**Show hidden** adds the hidden repositories below the new ones, grouped by
where they stand, each with a stamp and the reasons it is hidden. It is a
page of its own, `http://pulse.localhost:17842/hidden/`; the switch moves
between the two.

| Group | Stamp on the card | What holds it there |
| --- | --- | --- |
| **Interesting** | `★ Interesting` | Your answer is **Interesting**, and no note names it yet |
| **Skipped** | `Skipped · until Feb 19, 2027`, or `Seen · until Feb 19, 2027` after **Mark all shown as seen** | Its snooze runs |
| **In your notes** | `In your notes` | A note of your bookmarks directory names it, whatever your answer in pulse |

| Reason on the card | Meaning |
| --- | --- |
| `Skipped on Nov 19, 2026, until Feb 19, 2027` | You pressed **Skip**; the repository is hidden until that day |
| `Marked as seen on Nov 19, 2026, until Feb 19, 2027` | You pressed **Mark all shown as seen** |
| `Marked as interesting on Nov 19, 2026, 20:05` | You pressed **Interesting** |
| `Bookmarked in pulse:` and the comment, or `Bookmarked in pulse, without a comment` | It has a pulse bookmark |
| `In the vault: cobra.md` | The named note of your bookmarks directory names it |

In the reader of a hidden repository, **Interesting** replaces a skip,
**Skip** replaces a bookmark, **Save comment** replaces the comment of a
bookmark, and **Take back** makes the repository new again. The card then
moves to its group, and the reader stays on it.

A repository that a note names offers neither **Interesting** nor **Skip**:
it is in your notes already. When pulse holds an answer for it too, **Take
back** removes that answer; the note stays.

### The queue

**Queue**, at `http://pulse.localhost:17842/queue/`, lists the repositories
whose answer is **Interesting** and that no note of your vault names yet,
oldest first, also the ones that left the lists. It is the list
[`pulsectl bookmarks`](#export-the-bookmarks) prints: what is still waiting
for a note. Click a repository to open it in the reader; **Take back** on a
repository removes it from the queue. When the vault cannot be read, the
queue says so and lists every bookmark.

### Past days

pulse keeps every load of the lists, so you can go back to a morning you
skipped. Click the date in the masthead to open the calendar: the days with
snapshots are links, the others are not. The arrows beside the date step to
the previous and the next day that has snapshots.

| Address | What it shows |
| --- | --- |
| `http://pulse.localhost:17842/` | Today |
| `http://pulse.localhost:17842/days/2026-11-19/` | The lists as they stood on November 19, 2026 |
| `http://pulse.localhost:17842/days/2026-11-19/hidden/` | The same day with the hidden repositories |
| `http://pulse.localhost:17842/days/2026-11-19/flow/` | The cards of that day in [flow mode](#flow-mode) |

A day is a calendar day in the time zone of your computer. The page of a past
day shows the last successful load of each list that day; a list that did not
load that day is marked `no snapshot that day`.

A past day is not a restored state: what is hidden is decided by what you
have done since. A repository you skipped or bookmarked today, or added to
your notes, is hidden on yesterday's page too, and a skip whose three months
are over shows it again there. **Interesting**, **Skip** and **Mark all shown
as seen** work on a past day and mark what that page shows; the mark keeps
the lists the repository was in on that day. A past day loads nothing from
the network, so it has no **Refresh**.

A date pulse has no snapshots for, or one that is not written as
`YYYY-MM-DD`, answers `404` with a link to today. The page of today's date is
today's page.

### Flow mode

**Flow** goes through the new repositories one card at a time. Each card is
the whole story of its repository, as the [reader](#the-reader) shows it,
and the answers are always in the same place at the bottom of the screen:

```text
  ← THE PAPER          SUNDAY, NOVEMBER 22, 2026          3 OF 28 · 26 LEFT

  GO · IN 2 LISTS
  Tencent
  LoopForge
  Self-hosted workflows for coding-agent delivery.
  5,109 STARS   708 FORKS   12 OPEN ISSUES   MIT   JULY 2026 SINCE
  agents · workflows · self-hosted
  ○ trendshift daily #1   ■ github weekly #7
  github.com/Tencent/LoopForge
  ┌ README ──────────────────────────────────────────────────────┐

  [ ← SKIP ]       [ ↓ LATER ]       [ ↑ BACK ]       [ INTERESTING → ]
                      COMMENT  C        TAKE BACK  U
```

| Answer | Keys and gestures | What it does |
| --- | --- | --- |
| **Skip** | `←`, or swipe left | Marks the repository as seen, as **Skip** on the page |
| **Interesting** | `→`, or swipe right; `Enter` in the comment field | Bookmarks it with the comment, as **Interesting** on the page |
| **Later** | `↓` | Marks nothing and shows the next card |
| **Back** | `↑` | Shows the previous card of this flow, with the answer it got |
| **Comment** | `c` | Opens the comment field for **Interesting**; `Esc` closes it |
| **Take back** | `u` | Removes the answer of the card on the screen: it is new again and stays |

The flow holds the cards the page shows, in the order of the page; hidden
repositories are not in it. The next card appears once the answer is written
to disk; if it cannot be written, the card stays with the error. A card left
for later stays on the page and is in the next flow. The README scrolls with
the mouse or the trackpad, and a click into it leaves the arrows to the
flow; to copy text of the README, open it on GitHub.

**Back** walks the cards of this flow backwards, also those left for later.
A card you answered shows its stamp, its answer pressed and its comment;
another answer replaces it, the same one writes nothing, and `u` takes it
back. The way back lasts as long as
the flow is open: a reload starts a new flow. When no card is left, the flow
says so, links back to the page, and `↑` returns to the last card.

The flow of today is at `http://pulse.localhost:17842/flow/`, the flow of a
past day at `/days/YYYY-MM-DD/flow/`. It has no **Mark all shown as seen**.

### Loading and errors

Opening the page loads a list from the network only when its last attempt,
successful or not, is more than an hour old; the hour holds across restarts
of pulse. **Refresh** loads all six lists now. The page waits for the lists
it loads, so the first open takes a few seconds.

A list whose request failed, whose answer is for another period or has no
repositories, or that took too long is in error. The error is shown on that
list as `Failed 20:08:` and the reason, and the other lists keep working. The
last successful load of the failed list stays in the view, and the error ends
with its time: `Showing the snapshot of Nov 19, 2026, 20:05.`

A failed list is never shown as "nothing new". When no repository is new, the
page says **Nothing new** only if no list is in error; otherwise it says
**No new repositories in the lists that loaded**, or **No list could be
loaded**.

## Export the bookmarks

`pulsectl bookmarks` asks the running service and prints the repositories
whose answer is **Interesting** and that no note of your vault names yet,
oldest first, one URL per line: the [queue](#the-queue), ready for the tool
that writes your notes. Once a note names a repository, it leaves the list.

```sh
pulsectl bookmarks
```

```text
https://github.com/firecrawl/firecrawl
```

With `--all` it prints every bookmark, also those a note names. When the
vault, or one of its notes, cannot be read, `pulsectl bookmarks` fails and
says why, instead of printing repositories that may have a note already;
`--all` still works.

With `--format jsonl` it prints one JSON object per line, with the comment,
the time of the bookmark in UTC and the lists the repository was in when you
bookmarked it:

```sh
pulsectl bookmarks --format jsonl
```

```json
{"url":"https://github.com/firecrawl/firecrawl","repo":"firecrawl/firecrawl","comment":"try for the docs search","bookmarked_at":"2026-11-19T17:05:35Z","lists":["trendshift/daily","github/daily"]}
```

The export changes nothing. It needs the running service; without it the
command fails:

```text
Error: the pulse service is not running at http://pulse.localhost:17842: start it with `brew services start octolab/tap/pulse`, or run `pulse`
```

For a service on another port, name it:
`pulsectl bookmarks --addr http://pulse.localhost:8080`.

## Where the state lives

| What | Where |
| --- | --- |
| Configuration | The file `--config` names, else the file `PULSE_CONFIG` names, else `$XDG_CONFIG_HOME/pulse/config.toml`, else `~/.config/pulse/config.toml`. The background service uses `~/.config/pulse/config.toml` |
| Log of the background service | `$(brew --prefix)/var/log/pulse.log` |
| Loaded lists, marks and bookmarks | The directory `--data-dir` names, else the directory `PULSE_DATA_DIR` names, else `$XDG_DATA_HOME/pulse`, else `~/.local/share/pulse` |

The data are plain files that survive a restart; pulse creates the directory
when it is not there, and the bookmarks directory is only read.

## Limits

- pulse listens on loopback only, `127.0.0.1` and `::1`. It refuses to start
  on any other address and answers only requests addressed to
  `pulse.localhost`, `localhost` or a loopback address.
- The background service is for macOS with Homebrew. Elsewhere you run
  `pulse` in a terminal, or under a service manager of your own.
- Opening the page loads a list at most once an hour, and a failed attempt
  counts: use **Refresh** to load sooner.
- A snooze cannot be extended: a second **Skip** keeps the end of the first.
  One period applies to every repository.
- The loads of the lists are kept without expiry. The calendar shows whole
  days: not a single load within a day, and no day before the first load.
- **Take back** removes a bookmark from pulse only; a note in your vault
  stays. **Undo** of **Mark all shown as seen** lasts until you leave the
  page or load it again yourself.
- Once an answer is taken back, an older pulse cannot open the data
  directory: it does not know that record.
- The six lists are fixed. A renamed or transferred repository is a new one.
- pulse does not write to your bookmark notes: `pulsectl bookmarks` is the
  way to take the bookmarks out.
