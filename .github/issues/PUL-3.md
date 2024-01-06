---
code: PUL-3
id: I_kwDOJvS0hM5osnIw
databaseId: 1756525104
number: 3
url: https://github.com/octomation/pulse/issues/3
title: "core: rethink tablo"
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
createdAt: 2023-06-14T10:05:54Z
updatedAt: 2026-11-20T10:13:38Z
lastEditedAt: 2026-11-20T10:13:21Z
closedAt: 2026-11-20T10:13:38Z
---

# core: rethink tablo

## Motivation

Tablo (https://github.com/octopot/tablo) was "the one point of view to all
your task boards", and https://github.com/tact-app/github a GitHub inbox. I
wanted to extend that concept beyond task boards: one point of view to all
the external activity I follow, starting with GitHub.

## Outcome

The rethinking is done: this repository, first named tablo, became pulse,
"your morning, with a hand on the pulse". It is one local page that shows
only what is new since I last looked, with an action on each item, and it is
the place for the other kinds of external activity of this backlog:
[[PUL-1]], [[PUL-2]], [[PUL-4]].

The first version, [[PUL-5]], released as v0.1.0, does this for the
GitHub trends.

## Acceptance criteria

- [x] The concept has a name and a shape: pulse, one page of what is new,
  with marks that hide what was seen.
- [x] A first release implements it: v0.1.0, [[PUL-5]].

## Out of scope

- The task boards of tablo: they are not part of pulse.

<!-- 2026-11-20T12:00Z https://github.com/octomation/pulse/issues/3#issuecomment-7036902951
The rethinking is done: tablo became pulse, this repository. pulse is the one point of view to external activity that tablo meant to be: one local page that shows only what is new since I last looked, with an action on each item to hide what was seen.

The first version, #85, released as v0.1.0 and v0.1.1, does this for the GitHub trends of trendshift.io and github.com/trending. The other activities of the backlog follow on the same page: build statuses (#1), followers and stars (#2), workflow flags (#4).

The task boards of the original tablo are not part of pulse. Closing as completed.
-->
