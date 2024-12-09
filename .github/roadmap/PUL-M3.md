---
code: PUL-M3
id: MI_kwDOJvS0hM4BLnwF
databaseId: 19823621
number: 3
url: https://github.com/octomation/pulse/milestone/3
title: "M3: Change your mind in one move"
state: OPEN
openIssueCount: 1
closedIssueCount: 0
dueOn: null
createdAt: 2026-11-23T19:24:25Z
updatedAt: 2026-11-23T19:25:01Z
closedAt: null
---

# M3: Change your mind in one move

Every decision about a repository takes one move, with its details in front of you, and so does changing it. You see what you decided before, what waits for a note and what is in your notes already, and you hand only the interest without a note to the tool that writes your notes.

**What you get after upgrading**

- A flow card is the whole story of its repository: the facts, the topics, the links and the README, with the answers always in the bar; ↑ goes back to the previous card, `u` takes an answer back, `c` adds a comment. [[PUL-13]]
- Stamps say where a repository stands: Interesting, Skipped, Seen or In your notes; the hidden page is grouped under them, and a repository in your notes is not offered Interesting again. [[PUL-13]]
- The last answer counts, Take back makes a repository new again, and Undo follows "Mark all shown as seen". [[PUL-13]]
- `pulsectl bookmarks` and the new Queue list only the interest that has no note yet; `--all` lists every bookmark. [[PUL-13]]

**Done when** you can go through the flow without leaving it, see on every card what you decided, take any answer back in one move, and hand only the repositories without a note to your notes. Delivery starts with v0.2.1.

## Issues

- [ ] [[PUL-13]] — web: decide, see and take back without leaving the flow
