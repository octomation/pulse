# Self-control

Open checks that outlive a working session: things to look at later, decisions
that are waiting, and follow-ups that still need an issue. Each item says when
to look, what proves it and what to do if it fails. An item is removed once it
is proven or moved into an issue; a release adds the checks it leaves open
([AGENTS.md](../AGENTS.md#7-self-control-and-cleanup)).

## Watch

### The infrastructure of octopot/indexit and octomation/maintainer

The workflows, hooks, release helpers, tools module, Makefile and docs site
come from those two repositories, adapted to two binaries and no agent skill.

- [ ] **After the push to `main`**: `ci`, `tools` and `cd.docs` are green,
  and the site at <https://pulse.octolab.org/> passes the smoke test of
  `cd.docs`. If one fails, compare the job with the same job of
  octopot/indexit before changing it here.
- [ ] **After the push**: Settings → Pages → Enforce HTTPS is on, as
  `.github/settings.json` declares; it was off before. Then
  `make workflow-doctor REASON="after the alignment"` is green: it also
  proves that the OctoLab Releaser App is installed on `octolab/homebrew-tap`
  and `octolab/vanity`. If the doctor still lists `caches.yml`, `docs.yml`,
  `runs.yml`, `stale.yml` or `warmup.yml`, the files the alignment renamed,
  look at whether GitHub has dropped them from the workflow list.
- [ ] **After the push**: `gh workflow run cd.yml -f reason="snapshot"` on
  `main` builds a snapshot of both binaries and publishes nothing.

## Before the first release

- [ ] `docs/public/og.png` is a placeholder labelled `unreleased`: regenerate
  it as [docs/README.md](../docs/README.md#social-preview-image) describes,
  and record the geometry of its version label there.
- [ ] The issue mirror in `.github/issues/` and `.github/roadmap/` predates
  the format of the `projects:maintainer` skill that octopot/indexit and
  octomation/maintainer use: sync it with the skill, as its own
  `chore(github): sync pm entities` commit.
- [ ] octolab/vanity has no entry of `go.octolab.org/toolset/tablo`, the
  path the module had before it moved to `go.octolab.org/toolset/pulse`, and
  none of the new path yet: the first stable tag writes it. Check both on
  octolab/vanity's `main` before that tag; an entry of the new path that
  names another repository fails the `vanity` job of `cd`.
