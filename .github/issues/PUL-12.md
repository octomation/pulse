---
code: PUL-12
id: I_kwDOJvS0hM8AAAABhsn-kQ
databaseId: 6556352145
number: 99
url: https://github.com/octomation/pulse/issues/99
title: "ci: run the browser check"
issueType: Task
labels:
  - "type: improvement"
  - "scope: test"
  - "impact: low"
  - "effort: easy"
assignees:
  - kamilsk
milestone: null
parent: null
issueFields: {}
state: OPEN
stateReason: null
duplicateOf: null
createdAt: 2026-11-23T09:35:14Z
updatedAt: 2026-11-23T09:35:14Z
lastEditedAt: null
closedAt: null
---

# ci: run the browser check

## Motivation

Flow mode and the layout of the page are checked in a browser by
`make test-browser` ([[PUL-10]]), and only on the maintainer's computer: no
workflow runs it, and the workflows that test the code do not start on a
change of a template, a stylesheet or a script of the page. A change that
breaks the keys of the flow or the grid of the page can reach `main` green.

## PoC

```bash
$ make test-browser          # the same check, locally
$ gh run list -w ci.yml      # runs it on a change of internal/web/ too
```

## Acceptance criteria

- [ ] A workflow runs `make test-browser` on a pull request and a push to
  `main` that change `internal/web/` or `tests/browser/`.
- [ ] The check installs its browser itself; no secret is needed.
- [ ] AGENTS.md names `make test-browser` among the checks before a release
  until the workflow runs it.

## Out of scope

- Screenshot comparison of the page.
