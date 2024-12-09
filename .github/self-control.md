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

- [ ] **When an advisory gets a fixed version**: `tools` fails on
  GO-2026-6225 and GO-2026-5932, which have none (`Fixed in: N/A`), so the
  workflow is red on `main`. Look at its last run before the next release;
  once one of them names a fixed version, update that module in `tools/`.
- [ ] **When Nextra releases after 4.6.1**: the Dependabot alerts for zod,
  minimatch and katex in `docs/package-lock.json` are dismissed, as in
  those two repositories, because the static build never feeds them
  untrusted input; `docs/README.md` gives the reasons. Once a release ships
  shuding/nextra#4990, move the zod override as `docs/README.md` says,
  check the static export, and reopen any alert the upgrade still leaves.
- [ ] **After the push**: `gh workflow run cd.yml -f reason="snapshot"` on
  `main` builds a snapshot of both binaries and publishes nothing.

### v0.2.0

- [ ] **Before each release, until [PUL-12](issues/PUL-12.md) ships**: no
  workflow runs `make test-browser`, so the grid of the page and flow mode
  are checked by hand on the candidate commit, as AGENTS.md says. Remove
  this item once a workflow runs it.
- [ ] **From February 20, 2027**: the first skips of November 2026 end.
  When a list still has such a repository, it is on the page again, and
  **Show hidden** no longer lists it. If it stays hidden, file a bug
  against the snooze of a seen mark.

### v0.2.1

- [ ] **The morning after `/bookmark-it` turns the queue into notes**:
  `pulsectl bookmarks` prints only the interest without a note, so the
  repositories that got a note leave it, and `/queue/` is shorter by the
  same ones. If a noted repository stays, check that its note names it in
  `url` or `aliases`; if it does, file a bug against the export of pending
  bookmarks.
