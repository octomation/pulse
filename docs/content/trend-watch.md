---
title: Trend watch
description: One local page at http://pulse.localhost:17842/ with the trending GitHub repositories you have not seen or bookmarked, a reader for each of them, and pulsectl bookmarks to export the ones you marked as interesting.
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
```

The Formula installs both binaries, the service `pulse` and its client
`pulsectl`. On macOS or Linux, on `amd64` or `arm64`, you can also take
`pulse_0.1.0_<os>-<arch>.tar.gz` from the
[v0.1.0 release](https://github.com/octomation/pulse/releases/tag/v0.1.0), or
install with Go 1.27 or later:

```sh
go install go.octolab.org/toolset/pulse/cmd/...@v0.1.0
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
- `bookmarks` is the only key. With an unknown key in the file, pulse reports
  it and does not start.

Without the file, the filter is off and the page says so. When the directory
cannot be read, or none of its notes names a repository, the page says that
the list may include what is already in the vault.

## Start it

```sh
pulse
```

```text
pulse listens on http://pulse.localhost:17842/
data: /Users/you/.local/share/pulse
vault: /Users/you/Notes/Bookmarks/GitHub (config /Users/you/.config/pulse/config.toml)
github: the reader asks api.github.com without a token, about 30 repositories an hour
```

Open `http://pulse.localhost:17842/` in a browser: `.localhost` names resolve
to your own computer, so there is nothing to set up. pulse runs in the
terminal until `Ctrl-C`. Without a config file, the third line is
`vault: not configured (no config at /Users/you/.config/pulse/config.toml)`.

To use another port, pass a loopback IP address with a port; the page is then
at `http://pulse.localhost:8080/`:

```sh
pulse --listen 127.0.0.1:8080
```

## The page

The page reads like a morning paper:

- The masthead has the date, the counts, as in `54 new · 30 hidden`, and
  **Show hidden**, **Refresh** and **Mark all shown as seen**.
- Below it are the six lists, each with the time of its last successful load
  and the number of its repositories, and the state of the vault filter, as
  in `Vault: 2 repositories in 2 notes.`
- **Front page** holds the repositories that are in most lists at once, the
  first of them as the lead story. **Today**, **This week** and **This
  month** hold the others, each in the section of its shortest period.

A repository is one card, whatever number of lists it is in; `owner/name` is
compared without case. The card shows its language, its description and one
label per list with its position there: `trendshift daily #8`,
`github daily #6`.

| Action | What it does |
| --- | --- |
| **Interesting** | Bookmarks the repository, with the comment you typed in the field above it (optional, up to 500 characters), and marks it as seen |
| **Skip** | Marks the repository as seen |
| **Mark all shown as seen** | Marks every repository the page shows at that moment as seen |

Only these three actions mark a repository as seen: opening the page, the
reader, **Show hidden**, **Refresh** and an export mark nothing. A card
leaves the page once its mark is written to disk; if it cannot be written,
the card stays and shows the error. A seen mark has no expiry: a repository
you saw a month ago stays hidden when it trends again.

### The reader

Click the name or the description of a repository to read about it without
leaving the page. The reader opens beside the paper and shows:

- the stars, the forks, the open issues, the license, the month the
  repository was created and the day of its last push;
- its topics, its lists, and links to github.com and to its homepage;
- the README as GitHub renders it. Its links open in a new tab.

The reader has the same comment field, **Interesting** and **Skip** as the
card; after either it moves to the next repository. **Next** and
**Previous**, or `j` and `k`, move between repositories, and `Esc` closes
the reader.

pulse asks `api.github.com` for a repository when you open it and keeps the
answer for five minutes. Without a token GitHub allows about 30 repositories
an hour. Start pulse with `GITHUB_TOKEN` or `GH_TOKEN` in the environment to
lift that; pulse sends the token to `api.github.com` only and says at start
which variable it uses:

```text
github: the reader asks api.github.com with the token of GITHUB_TOKEN
```

When GitHub does not answer, the reader says why and still shows what the
lists know, with the actions.

### Hidden repositories

**Show hidden** adds the hidden repositories below the new ones, each with
the reasons it is hidden:

| Reason on the card | Meaning |
| --- | --- |
| `Skipped on Nov 19, 2026, 20:05` | You pressed **Skip** |
| `Marked as seen on Nov 19, 2026, 20:05` | You pressed **Mark all shown as seen** |
| `Marked as interesting on Nov 19, 2026, 20:05` | You pressed **Interesting** |
| `Bookmarked in pulse:` and the comment, or `Bookmarked in pulse, without a comment` | It has a pulse bookmark |
| `In the vault: cobra.md` | The named note of your bookmarks directory names it |

A hidden card keeps its comment field. **Interesting** bookmarks a repository
you had only marked as seen or keep in your notes, and **Save comment**
replaces the comment of a bookmark.

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

`pulsectl bookmarks` asks the running service and prints the bookmarks,
oldest first, one URL per line:

```sh
pulsectl bookmarks
```

```text
https://github.com/firecrawl/firecrawl
```

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
Error: the pulse service is not running at http://pulse.localhost:17842: start it with `pulse`
```

For a service on another port, name it:
`pulsectl bookmarks --addr http://pulse.localhost:8080`.

## Where the state lives

| What | Where |
| --- | --- |
| Configuration | The file `--config` names, else the file `PULSE_CONFIG` names, else `$XDG_CONFIG_HOME/pulse/config.toml`, else `~/.config/pulse/config.toml` |
| Loaded lists, marks and bookmarks | The directory `--data-dir` names, else the directory `PULSE_DATA_DIR` names, else `$XDG_DATA_HOME/pulse`, else `~/.local/share/pulse` |

The data are plain files that survive a restart; pulse creates the directory
when it is not there, and the bookmarks directory is only read.

## Limits

- pulse listens on loopback only, `127.0.0.1` and `::1`. It refuses to start
  on any other address and answers only requests addressed to
  `pulse.localhost`, `localhost` or a loopback address.
- You start it by hand, and it runs while its terminal does: no service
  manager keeps it running.
- Opening the page loads a list at most once an hour, and a failed attempt
  counts: use **Refresh** to load sooner.
- A seen mark cannot be taken back.
- A bookmark cannot be deleted from the page; only its comment can be
  replaced.
- The six lists are fixed. A renamed or transferred repository is a new one.
- pulse does not write to your bookmark notes: `pulsectl bookmarks` is the
  way to take the bookmarks out.
