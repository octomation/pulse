---
code: null
id: I_kwDOJvS0hM5ospRm
databaseId: 1756533862
number: 4
url: https://github.com/octomation/pulse/issues/4
title: "core: workflow flags"
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
createdAt: 2023-06-14T10:10:38Z
updatedAt: 2026-11-20T10:13:24Z
lastEditedAt: 2026-11-20T10:13:24Z
closedAt: null
---

# core: workflow flags

## Motivation

A repository created from a template, such as octomation/go-module, starts
with workflows that fail from scratch (octomation/go-module#130), and some
workflows I want off for a while, for example to save the minutes of the free
tier. Today switching a workflow means editing the repository or clicking
through its settings, one repository at a time.

I want one place to see and switch the workflows of my repositories.

## PoC

```bash
$ open https://github.com/owner/name/issues/1     # the flags issue of a repository
- [x] ci
- [x] cd
- [ ] docs
- [x] tools
```

Unchecking `docs` disables that workflow; checking it again enables it.

## Behavior

- A flags issue in a repository lists its workflows as a task list, one
  item per workflow, checked when it is enabled.
- pulse watches the flags issues of the configured repositories and, when an
  item changes, enables or disables that workflow with the GitHub API for
  disabling and enabling a workflow.
- A workflow switched in the settings of GitHub is reflected back in the
  issue, so the issue never lies.
- Closing the flags issue stops pulse from watching it; the workflows stay
  as they are.

## Limits

- GitHub Actions only.
- A workflow added to the repository appears in the issue on the next
  check; its state is what GitHub says.

## Acceptance criteria

- [ ] Test with a fake GitHub: unchecking an item disables its workflow,
  checking it enables it.
- [ ] Test: a workflow disabled in the settings is unchecked in the issue.
- [ ] Test: a closed flags issue is not watched, and nothing is switched.
- [ ] The guide in `docs/content/` describes the flags issue.

## Out of scope

- Secrets or variables as switches: the API for disabling a workflow is
  enough (see the comments).
- Other CI services.

<!-- 2023-06-14T10:12Z https://github.com/octomation/pulse/issues/4#issuecomment-1590903932
Research:
- [ ] https://docs.github.com/en/actions/deployment/targeting-different-environments/using-environments-for-deployment
-->

<!-- 2023-06-18T19:46Z https://github.com/octomation/pulse/issues/4#issuecomment-1596248384
another example

<img width="424" alt="image" src="https://github.com/octomation/tablo/assets/1165416/ac4290ca-2465-4d8f-89ea-49ac954ac424">

<img width="1083" alt="image" src="https://github.com/octomation/tablo/assets/1165416/a165cf42-c6c8-4ab1-8e80-5371a76f3313">

```
Error: This version of the CodeQL Action was deprecated on January 18th, 2023, and is no longer updated or supported. For better performance, improved security, and new features, upgrade to v2. For more information, see https://github.blog/changelog/2023-01-18-code-scanning-codeql-action-v1-is-now-deprecated/
RequestError [HttpError]: Advanced Security must be enabled for this repository to use code scanning.
    at /home/runner/work/_actions/github/codeql-action/v1/node_modules/@octokit/request/dist-node/index.js:66:23
    at processTicksAndRejections (internal/process/task_queues.js:97:5)
    at async Job.doExecute (/home/runner/work/_actions/github/codeql-action/v1/node_modules/bottleneck/light.js:405:18) {
  status: 403,
```

- https://github.com/octomation/settings-action/actions/runs/5264343717
- https://github.com/octomation/goimports-action/actions/runs/5264329882
-->

<!-- 2023-06-25T11:42Z https://github.com/octomation/pulse/issues/4#issuecomment-1606052318
e.g., GitLab

<img width="1064" alt="image" src="https://github.com/octomation/tablo/assets/1165416/265f4fe0-3dcc-465a-ae11-53f756f09aa8">
-->

<!-- 2023-07-03T17:53Z https://github.com/octomation/pulse/issues/4#issuecomment-1618946757
The solution is https://docs.github.com/en/actions/using-workflows/disabling-and-enabling-a-workflow.
-->
