---
code: PUL-M2
id: MI_kwDOJvS0hM4BLi_d
databaseId: 19804125
number: 2
url: https://github.com/octomation/pulse/milestone/2
title: "M2: Catch up at your own pace"
state: OPEN
openIssueCount: 5
closedIssueCount: 0
dueOn: null
createdAt: 2026-11-23T09:57:00Z
updatedAt: 2026-11-23T10:35:23Z
closedAt: null
---

# M2: Catch up at your own pace

A morning you did not finish is not lost. Every day pulse loaded the lists has a page of its own, a skipped repository that trends again gets a second look after a quarter, and the new ones can be gone through one card at a time with the answers always in the same place. The page reads as a newspaper and names the vault directory it reads.

**What you get after upgrading**

- A calendar opens the saved days at `/days/YYYY-MM-DD/`, with arrows between them. A day is saved when the lists load after you open today's page or use **Refresh**; pulse does not collect missed days in the background. A past day applies your current bookmarks, vault notes and unexpired skips; **Skip** and **Interesting** work there too and record the lists of that day. [[PUL-6]]
- **Skip** and **Mark all shown as seen** hide a repository for three calendar months, and `[seen] snooze` sets another period; the skips you already have count from the day you made them. **Interesting** and a note in the vault hide it for good. [[PUL-9]]
- Flow mode at `/flow/`: one card at a time; ← skips, → bookmarks it as **Interesting**, ↓ chooses **Later** and leaves it unmarked for the next flow. On a touch screen, swipe left to skip or right to bookmark. An answer cannot be taken back from the flow. [[PUL-10]]
- The vault line names its directory, how many repositories its notes name and when the newest note changed, and warns when the directory is outside every vault Obsidian knows on this computer. [[PUL-7]]
- One grid from the front page to the last section, with cards that hold text only; open the reader or flow mode to comment, keep or skip. [[PUL-11]]

**Done when** you can open the saved days from the calendar and go through them in flow mode, see which vault directory the page reads and when its newest note changed, and see when a skip ends. Delivery starts with v0.2.0.

## Issues

- [ ] [[PUL-6]] — web: calendar of past snapshots
- [ ] [[PUL-7]] — core: hide through a facade, the vault as its first provider
- [ ] [[PUL-9]] — core: a seen mark snoozes for a quarter
- [ ] [[PUL-10]] — web: flow mode, one card at a time
- [ ] [[PUL-11]] — web: a front page grid without holes
