---
code: null
id: I_kwDOJvS0hM5ospRm
databaseId: 1756533862
number: 4
url: https://github.com/octomation/pulse/issues/4
title: "core: workflow flags"
issueType: null
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
updatedAt: 2023-07-03T17:53:19Z
lastEditedAt: null
closedAt: null
---

# core: workflow flags

**Motivation:** provide a mechanism to manage actions, e.g., to reduce time on the free tier.

See octomation/go-module/issues/130.

**Idea**

- create an issue with flags (task list) == workflows of a target repository
- use it to enable/disable some workflows
- unsubscribe if issue will be closed

**Alternative**

- run workflow to set some secret or env on a target repository
- provide on/off value and specified workflow name

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
