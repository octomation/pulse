---
code: PUL-M1
id: MI_kwDOJvS0hM4BLPJQ
databaseId: 19722832
number: 1
url: https://github.com/octomation/pulse/milestone/1
title: "M1: A morning page of only what is new"
state: CLOSED
openIssueCount: 0
closedIssueCount: 2
dueOn: null
createdAt: 2026-11-20T10:04:50Z
updatedAt: 2026-11-23T10:35:21Z
closedAt: 2026-11-23T09:59:05Z
---

# M1: A morning page of only what is new

Six trending lists, the daily, weekly and monthly trends of Trendshift and GitHub, come together on one page on your computer, and the page holds only the repositories you have not marked yet. You read about a repository without leaving the page, keep what matters with a note on why, and clear the rest in one click.

**What you got**

- `pulse` serves the page at `http://pulse.localhost:17842/`. A repository is one card with its place in every list, and the ones in most lists open the front page.
- A repository you marked does not come back as new when it trends again; **Show hidden** shows each with the reason.
- The reader shows the facts and the README of a repository beside the page, and `j` and `k` move between repositories; reading marks nothing. A GitHub token in the config file lifts the reader's limit of about 30 repositories an hour.
- **Interesting** bookmarks a repository with an optional comment, and `pulsectl bookmarks` exports the bookmarks; **Skip** and **Mark all shown as seen** clear the rest.
- Name the bookmarks directory of your notes vault in the config file, and the repositories its notes name by `url` or `aliases` stay hidden.
- On macOS, run `brew services start octolab/tap/pulse` once: Homebrew then runs pulse in the background and starts it at login, so the page is there every morning without a terminal.

**Done when** one morning page joins the six lists and hides what you marked or keep in your bookmarks directory. Delivered in v0.1.0 and v0.1.1.

## Issues

- [x] [[PUL-3]] — core: rethink tablo
- [x] [[PUL-5]] — core: trend-watch without repeats
