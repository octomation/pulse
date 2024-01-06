---
code: PUL-8
id: I_kwDOJvS0hM8AAAABg_MaAQ
databaseId: 6508714497
number: 92
url: https://github.com/octomation/pulse/issues/92
title: "core: indexit as the vault provider"
issueType: Feature
labels:
  - "type: feature"
  - "scope: code"
  - "impact: medium"
  - "effort: medium"
assignees:
  - kamilsk
milestone: null
parent: null
issueFields: {}
state: OPEN
stateReason: null
duplicateOf: null
createdAt: 2026-11-20T09:31:24Z
updatedAt: 2026-11-20T09:59:02Z
lastEditedAt: 2026-11-20T09:59:02Z
closedAt: null
---

# core: indexit as the vault provider

## Motivation

pulse reads the vault notes itself, and so does every other local tool that
needs to know "is this already in my notes"; each of them can be pointed at a
wrong copy without noticing. indexit is to keep the indexes of the vault
(octopot/indexit#147), and
pulse should ask it through the facade of
[[PUL-7]] instead of reading
the files.

## PoC

```toml
# ~/.config/pulse/config.toml
[vault]
provider = "indexit"                 # or "dir", the reader inside pulse
vault    = "Mindset"                 # a vault indexit serves
```

```bash
$ open http://pulse.localhost:17842/
Vault: 2845 repositories · Mindset through indexit · indexed 2026-11-21 08:02
```

## Behavior

- With the indexit provider, pulse asks the local indexit service for every
  note of the vault that names a GitHub repository, wherever the note is, and
  hides those repositories with the note as the reason. A repository named
  outside the bookmarks folder is hidden too: the whole vault is what I have
  seen.
- pulse asks again for every page, conditionally: an unchanged index costs no
  transfer.
- When indexit does not answer, pulse uses the last index it received and
  says how old it is; with none received, the provider fails open, as the
  facade requires.
- The reader inside pulse stays and is chosen with `provider = "dir"`;
  switching between the two needs only the configuration and a restart.

## Limits

- pulse only reads the index: what goes into it is decided in indexit.
- One vault per pulse.

## Acceptance criteria

- [ ] Test with a fake indexit: the repositories of the index are hidden
  with their notes as reasons.
- [ ] Test: an unchanged index answered as not modified keeps the cards
  hidden without a new transfer.
- [ ] Test: indexit going down after a successful answer keeps the last index,
  and the page shows its age; indexit down from the start leaves the cards
  shown with the warning.
- [ ] Test: the notes of the bookmarks folder give the same hidden cards
  through "dir" and through "indexit"; a note outside the folder hides its
  repository through "indexit" only.
- [ ] The guide in `docs/content/` describes both providers and how to switch.

## Out of scope

- Seen marks and bookmarks through indexit.
