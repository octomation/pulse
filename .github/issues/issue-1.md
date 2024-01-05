---
code: null
id: I_kwDOJvS0hM5oslkT
databaseId: 1756518675
number: 1
url: https://github.com/octomation/pulse/issues/1
title: "core: gitboard port"
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
state: OPEN
stateReason: null
duplicateOf: null
createdAt: 2023-06-14T10:02:53Z
updatedAt: 2026-11-20T10:13:16Z
lastEditedAt: 2026-11-20T10:13:16Z
closedAt: null
---

# core: gitboard port

## Motivation

I maintain dozens of repositories across several owners, and a broken build
in one of them goes unnoticed until I open that repository. GitBoard
(https://gitboard.io, "Builds made visible") solved this for its users with
one board of build statuses; I have the same problem with my ecosystem.

pulse is my one point of view to external activity: the state of the builds
of my repositories belongs on the same morning page.

## PoC

```toml
# ~/.config/pulse/config.toml
[builds]
owners = ["octomation", "octopot", "octolab", "kamilsk"]
```

```bash
$ open http://pulse.localhost:17842/        # a "Builds" section: what is red on the default branches
```

## Behavior

- pulse watches the GitHub Actions runs of the default branch of every
  repository of the configured owners, archived and forked ones excluded.
- The page shows the repositories whose latest run of a workflow failed,
  with the workflow, the time and a link to the run; green repositories are
  only counted.
- A build that turned red since my last visit is marked as new; one that
  turned green again leaves the section.
- The statuses are loaded with the GitHub token of the configuration, as
  the reader does, and at most once an hour, as the lists are.

## Limits

- GitHub Actions only; other CI services are not read.
- Default branches only: pull requests and other branches are not watched.

## Acceptance criteria

- [ ] Test on fixtures: of three repositories, one with a failed latest run
  is shown with its workflow and link, two green ones are only counted.
- [ ] Test: archived and forked repositories are not watched.
- [ ] Test: a build red since the last visit is marked new; a build that
  became green is gone.
- [ ] Test: without a token the section says why it is empty, and the rest
  of the page works.
- [ ] The guide in `docs/content/` describes the section and its
  configuration.

## Out of scope

- Re-running or cancelling workflows from the page.
- Notifications outside the page.
