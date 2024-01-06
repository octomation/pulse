---
code: PUL-2
id: I_kwDOJvS0hM5osmId
databaseId: 1756520989
number: 2
url: https://github.com/octomation/pulse/issues/2
title: "core: watchub port"
issueType: Feature
labels:
  - "type: feature"
  - "scope: code"
  - "impact: high"
  - "effort: medium"
assignees:
  - kamilsk
milestone: null
parent: null
issueFields: {}
state: OPEN
stateReason: null
duplicateOf: null
createdAt: 2023-06-14T10:03:56Z
updatedAt: 2026-11-20T10:13:19Z
lastEditedAt: 2026-11-20T10:13:19Z
closedAt: null
---

# core: watchub port

## Motivation

Watchub (https://github.com/octomation/watchub) told me once a day who
followed or unfollowed me and who starred or unstarred my repositories. The
service has been down and unmaintained, and nothing replaced it.

pulse is my one point of view to external activity: these changes belong on
the same morning page, without email.

## PoC

```bash
$ open http://pulse.localhost:17842/        # a "People" section since my last visit
+ alice followed you
- bob unfollowed you
+ carol starred octomation/pulse
- dave unstarred octopot/indexit
```

## Behavior

- pulse keeps the list of my followers and the stargazers of my
  repositories, and shows what changed since my last visit: followed,
  unfollowed, starred and unstarred, each with the person and the time pulse
  noticed it.
- GitHub reports no unfollow or unstar event, so pulse finds them by
  comparing the lists it stored with the current ones.
- The first load only records the lists: it reports nothing.
- The lists are loaded with the GitHub token of the configuration, at most
  once a day.

## Limits

- Only the repositories I own, forks excluded.
- The time of a change is when pulse noticed it, not when it happened.

## Acceptance criteria

- [ ] Test on fixtures: two stored lists and two current ones give exactly
  the followed, unfollowed, starred and unstarred entries.
- [ ] Test: the first load reports nothing and stores the lists.
- [ ] Test: a failed load keeps the stored lists and reports the error, not
  "nothing changed".
- [ ] The guide in `docs/content/` describes the section.

## Out of scope

- Email or any notification outside the page.
- Followers and stargazers of repositories I do not own.
