---
code: PUL-M2
id: MI_kwDOJvS0hM4BLi_d
databaseId: 19804125
number: 2
url: https://github.com/octomation/pulse/milestone/2
title: "M2: Catch up at your own pace"
state: OPEN
openIssueCount: 0
closedIssueCount: 6
dueOn: null
createdAt: 2026-11-23T09:57:00Z
updatedAt: 2026-11-23T20:00:33Z
closedAt: null
---

# M2: Catch up at your own pace

A morning you did not finish is not lost, and an answer you gave in a hurry is not final. Every day pulse loaded the lists has a page of its own, a skipped repository that trends again gets a second look after three months, and the new ones can be gone through one card at a time, each with its whole story and the answers always in the same place. A card shows the answer it has, and one move changes it or takes it back. The page reads as a newspaper and names the vault directory it reads.

**What you get after upgrading**

- A calendar opens the saved days at `/days/YYYY-MM-DD/`, with arrows between them. A day is saved when the lists load after you open today's page or use **Refresh**; pulse does not collect missed days in the background. A past day applies your current bookmarks, vault notes and unexpired skips; **Skip** and **Interesting** work there too and record the lists of that day. [[PUL-6]]
- **Skip** and **Mark all shown as seen** hide a repository for three calendar months, and `[seen] snooze` sets another period; the skips you already have count from the day you made them. **Interesting** hides it while it stays your answer, and a note in the vault hides it while it names the repository. [[PUL-9]]
- Flow mode at `/flow/`: one card at a time, with the facts, the topics, the links and the README of the repository on the card. ← **Skip**, → **Interesting**, ↓ **Later**, which marks nothing, ↑ **Back** to the previous card and its answer, `u` **Take back**, and `c` opens the comment for **Interesting**; on a touch screen, swipe left to skip or right to mark as **Interesting**. [[PUL-10]] [[PUL-13]]
- A card with an answer or a note shows a stamp: **Interesting**, **Skipped** or **Seen** with the day it ends, or **In your notes**; **Show hidden** groups them under **Interesting**, **Skipped** and **In your notes**. The last answer counts, **Take back** makes a repository new again, **Mark all shown as seen** has **Undo**, and a repository a note names is offered neither **Interesting** nor **Skip**. [[PUL-13]]
- `pulsectl bookmarks` and **Queue**, at `/queue/`, list the repositories marked as **Interesting** that no note names yet; `pulsectl bookmarks --all` prints every bookmark. [[PUL-13]]
- The vault line names its directory, how many repositories its notes name and when the newest note changed, and warns when the directory is outside every vault Obsidian knows on this computer. [[PUL-7]]
- One grid from the front page to the last section, with cards that hold text only; open the reader or flow mode to answer. [[PUL-11]]

**Done when** you can open a saved day from the calendar and answer its cards in flow mode with the README in front of you, find under **Show hidden** what you answered and when a skip ends, change or take back any answer in one move, and read on the vault line which directory the page reads and when its newest note changed. Delivered in v0.2.0 and v0.2.1.

## Issues

- [x] [[PUL-6]] — web: calendar of past snapshots
- [x] [[PUL-7]] — core: hide through a facade, the vault as its first provider
- [x] [[PUL-9]] — core: a seen mark snoozes for a quarter
- [x] [[PUL-10]] — web: flow mode, one card at a time
- [x] [[PUL-11]] — web: a front page grid without holes
- [x] [[PUL-13]] — web: decide, see and take back without leaving the flow
