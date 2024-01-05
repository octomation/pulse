# Agent guide

## Features

A larger feature goes from its issue to the code to the user guide.

- **The issue comes first.** Before you implement it, its issue in
  `.github/issues/` states the expected behavior, the limits and the
  acceptance criteria. When the requirements change, update the issue before
  the code. The issue sets the requirements: don't rewrite it to fit a defect
  or a limit of the implementation.
- **The guide finishes it.** A feature is done when `docs/content/` describes
  it for users: what it does, how to use it, and an example of the result.
  Update the command's page or add one; mention only the flags and limits
  needed to use it, and check the examples against the code and `--help`. The
  guide changes in the product commit ([ground rules](#ground-rules)); what a
  guide may claim is in
  [docs/README.md](docs/README.md#keep-the-release-accurate).
- A guide stands on its own: no development history, implementation plans,
  test reports, internals or promises of unreleased behavior.

## Binaries

pulse ships two binaries from one module, `go.octolab.org/toolset/pulse`:
the service `pulse` (`cmd/pulse`) and its client `pulsectl`
(`cmd/pulsectl`). The catalog name in octolab/vanity is `pulse`. Both
binaries go into
every archive and the one Formula; `make install` and `make go-build` build
both into `bin/<os>/<arch>/`.

## Local services

`pulse` is a local service, started by hand for now. It follows the
convention of octopot/indexit and octomation/maintainer: it listens on
loopback only, on `127.0.0.1` and `::1`, and answers at
`http://pulse.localhost:17842/`. `brew services` does not run it yet; when it
does, it runs it as the user, without root.

| Port | Service |
| --- | --- |
| 17840 | indexit |
| 17841 | maintainer |
| 17842 | pulse |

The port of pulse is recorded here only: the AGENTS.md of the other services
stay as they are.

## Issues and the board

The issues are mirrored in `.github/issues/` and laid out on the board in
`.github/roadmap/` by the `projects:maintainer` skill; `maintainer.py lint`
checks the files. The mirror changes with the commit that changes the issue's
state, or with the next `chore(github): sync pm entities`.

## Releasing

A release is a curated note plus a signed tag. A stable release also
publishes the module to [go.octolab.org](https://go.octolab.org) through the
[octolab/vanity](https://github.com/octolab/vanity) catalog. A prerelease
(`vX.Y.Z-rcN`) is a release too: it has its own note and tag and is frozen
once published. GoReleaser publishes it as a prerelease that never becomes
*Latest* and leaves the tap as it is, and the catalog keeps stable releases
only. Pipeline mechanics, secrets and hooks are in
[.github/workflows/README.md](.github/workflows/README.md); docs conventions
are in [docs/README.md](docs/README.md#keep-the-release-accurate).

### Ground rules

- The agent brings the shells, the maintainer fires: the agent prepares each
  step in full and hands over one command; the maintainer reviews it, commits,
  tags and pushes. Nothing else waits for him.
- A change is a series of commits planned up front, each bound to its issue
  (`, issue #N` while it is in progress, `, closes #N` on the commit that
  finishes it). The agent stages one self-contained commit at a time, writes
  its message to a file outside the repository and gives
  `git contrib < <file>`: `type(scope): summary`, then, when it helps, a body
  wrapped at 72 columns. An intermediate state the working tree no longer has
  is staged from blobs (`git hash-object -w` and `git update-index
  --cacheinfo`), leaving the tree as it is. Tests and linters run on the end
  state, not on each commit.
- What can be verified mechanically the agent does without asking: the checks
  before the release, the release commit, the note, the image and the tag
  annotation, the checks after the push with the upgrade of this Mac, the
  issue mirror and the board, the announcement row, `.github/self-control.md`
  and the cleanup. It asks only for decisions: scope, wording the owner cares
  about, a criterion to change.
- Product changes first, release commit last: `chore(release): prepare vX.Y.Z`.
  A guide that explains new behavior changes with the product commit.
- The binaries are the source of truth for what the docs and the notes claim.
- A published release is frozen: its tag, annotation, image and catalog entry
  stay as tagged. Its note may still get pictures or wording that keep it true
  to the tag. A missed fix ships in the next patch, or in the next candidate
  before the final tag.
- A release that changes stored data, configuration or URLs ships with the
  way to move to it, and the agent moves this Mac with it: it backs up the
  data and the configuration, migrates them, checks that the binaries work on
  the result, and removes the backup once the owner confirms.

### Before you start

`git fetch` and reconcile `main` with `origin/main`, then:

```sh
for w in ci cd.docs tools doctor; do gh run list -w "$w.yml" -b main -L 1; done
make doctor
(cd docs && npm ci)
```

- The latest `ci` and `cd.docs` runs on `main` are green.
- `tools` is green, or fails only on govulncheck advisories in `tools/` that
  have no fixed version (`Fixed in: N/A`): `cd` installs the tools but doesn't
  scan them.
- `doctor` is green: it mints the tap and catalog tokens from the OctoLab
  Releaser App, which proves the App can publish to both. If the App, its
  installations, the `OCTOLAB_RELEASER_*` secrets or `doctor.yml` changed
  since its last run, run it again:
  `make workflow-doctor REASON="before vX.Y.Z"` dispatches it on `main` and
  waits for the result; `REASON` is optional.
- `make doctor` has no `fail`.

On the candidate commit:

```sh
make verify
make go-dist-check
make release-config-check
```

`make release-config-check` accepts exactly the GoReleaser `brews`
deprecation and nothing else: `brews` ships both binaries as one Formula.

### 1. Changelog

The note covers everything that changed since the last stable release, not
only the issues of the milestone, and a final note after candidates also
counts from that stable tag. Read the whole range before writing; for the
first release, the range starts at the root commit:

```sh
base="$(git describe --tags --abbrev=0 --exclude='*-rc*' 2>/dev/null || git rev-list --max-parents=0 HEAD)"
git log --oneline "$base"..HEAD
git diff --stat "$base"..HEAD -- . ':!docs' ':!tools' ':!.github' ':!AGENTS.md'
```

Go through every product change the second command lists: the code, the
release channels, and each dependency the binaries ship with (`go.mod`),
whose own release notes say whether users see a difference. Only what users
get goes into the note; tooling, CI, docs and the mirror stay out, however
large.

Write `docs/content/changelog/vX.Y.Z.md`; it becomes the GitHub release:

```md
---
title: vX.Y.Z — Slogan.
description: "One sentence on what changed for the user."
---

# vX.Y.Z — Slogan.

Lead paragraph: what the release does, in terms of commands and results.

## A section per user-facing change

- Short, concrete bullets.

**Upgrade:** on macOS, run `brew update` followed by `brew upgrade --formula octolab/tap/pulse`. With Go, run `go install go.octolab.org/toolset/pulse/cmd/...@vX.Y.Z`. For a manual installation, use a [vX.Y.Z release archive](https://github.com/octomation/pulse/releases/tag/vX.Y.Z). Run `pulse version` and `pulsectl version` to check the installed version.
```

- English; a short slogan ending with a period.
- Only what users get: behavior and compatibility, no docs, tools or CI/CD.
  Explain jargon in user terms; no comparisons to release candidates.
- Links are site-relative with a trailing slash. Markdown and HTML, no MDX.

A prerelease note, `vX.Y.Z-rcN.md`, is short and self-contained: what to try,
what to report, and links only to pages that are already on the site, not to
the final note. Add it to `changelog/_meta.js` with `display: 'hidden'` and
give it no card; the current version elsewhere stays the stable one.

Then, in the same release commit:

- `changelog/_meta.js`: the new entry on top, `'vX.Y.Z': 'vX.Y.Z — Slogan.'`.
- `changelog/index.mdx`: a new card `vX.Y.Z · LATEST`, as octopot/indexit
  lays them out; the previous one becomes `· STABLE`. The first release
  replaces the "no release yet" paragraph.
- `docs/content/index.mdx`: description, release pill, release band, scope note.
- The current version in `README.md` and `docs/README.md`. In the content
  table of `docs/README.md`, the new note gets its own row.

```sh
grep -rn 'X\.Y\.W' README.md docs/README.md docs/content docs/public  # only old notes, cards and "since" facts may remain
node .github/scripts/release.mjs render vX.Y.Z --site-url https://pulse.octolab.org/ --out "$(mktemp -d)/notes.md"
(cd docs && TARGET=static SITE_URL=https://pulse.octolab.org/ npx next build)
```

Check the sidebar: All releases first, then version groups with the new note;
no prerelease note in it.

### 2. Social preview image

`docs/public/og.png` is rendered from `docs/scripts/og.html` by
`docs/scripts/og.mjs`, as [docs/README.md](docs/README.md#social-preview-image)
describes; the geometry of its version label is recorded there.

A release changes only the text of the version label in `og.html` and renders
it again. Then diff the result against the previous image on an RGB copy at
native resolution: only the box of the label may change. If anything else
moved, a font or the browser rendered differently: keep the previous image
and patch the changed characters of its label instead, as step 2 of
octopot/indexit's AGENTS.md describes. The site is the only user of this
file.

A prerelease doesn't change the image. Change the rest of `og.html` only when
the wordmark, the slogan, the monospaced line, the desk or the brand colors
change, then update `width`, `height` and `alt` in
`docs/app/[[...mdxPath]]/page.jsx` if they changed.

### 3. Tag annotation

Follow the previous tag, `git tag -l --format='%(contents)' vX.Y.W`:

```text
vX.Y.Z — Slogan.

One or two sentences on what the release does, wrapped
at 72 columns.

- lowercase imperative bullets without trailing periods

Release notes: docs/content/changelog/vX.Y.Z.md
```

Same scope as the note. Write it to `.agents/release/vX.Y.Z/annotation.txt`:
`.agents/` is not tracked, so it never reaches a commit.

### 4. Tag, check, push

For a prerelease, record the stable channels first:

```sh
gh release view --json tagName -q .tagName
gh api repos/octolab/homebrew-tap/contents/Formula/pulse.rb -q .sha
```

Then the maintainer runs one line, which the agent hands over with the
release commit:

```sh
git tag -s vX.Y.Z -F .agents/release/vX.Y.Z/annotation.txt && git tag -v vX.Y.Z && make release-check TAG=vX.Y.Z && git push --atomic origin main vX.Y.Z
```

Run it on `main` with the release commit checked out: `make release-check`
checks the tag against the branch you are about to push, as the `pre-push`
hook does, together with tidy modules and the GoReleaser config.

`git tag -v` must report a good signature from a key GitHub knows for your
email: `cd` checks that GitHub verifies the tag before anything is published,
and the module catalog publishes only from a verified tag.

If the push lock is set (`remote.origin.pushurl` is `no_push`), lift it for
this push only and set it again afterwards.

Never push the tag separately. A tag pushed by mistake can be deleted only
before the GitHub release exists; after that, ship the next patch.

### 5. After the push

The agent runs this section by itself once the push is done, the upgrade of
this Mac included, and reports what it saw.

```sh
gh run watch --exit-status "$(gh run list -w cd.yml -c "$(git rev-parse 'vX.Y.Z^{commit}')" -L 1 --json databaseId -q '.[0].databaseId')"
gh release view vX.Y.Z --json name,isPrerelease,assets -q '.name, .isPrerelease, .assets[].name'
gh api 'repos/octolab/homebrew-tap/commits?per_page=1' -q '.[] | .commit.verification.verified, .commit.message'
gh api repos/octolab/vanity/commits/pulse--vX.Y.Z -q '.commit.verification.verified, .commit.message'
```

- `cd` is green; for a prerelease, its vanity job is skipped. If
  `gh run watch` says the run is not found, `cd` hasn't started yet: run the
  line again.
- The release has the note's title and body, four archives with both
  binaries and `checksums.txt`.
- A stable release: the Formula commit for vX.Y.Z in the tap is verified,
  signed as the App.
- A prerelease: `isPrerelease` is `true`,
  `gh release view --json tagName -q .tagName` still names the previous stable
  release, and the Formula has the recorded `sha`.
- A stable release only: the catalog tag `pulse--vX.Y.Z` in `octolab/vanity`
  points at a verified commit, and
  `go install go.octolab.org/toolset/pulse/cmd/...@vX.Y.Z` works outside a
  checkout.
- Every channel delivers vX.Y.Z, checked by `pulse version` and
  `pulsectl version`, not by a successful download alone: the archives,
  `bin/install` with the tag, `go install`, and Homebrew:
  `brew update && brew upgrade --formula octolab/tap/pulse`.
- The site shows the new changelog page and release band once `cd.docs`
  finishes for the release commit.
- Criteria ticked by the release commit go to GitHub; the server fields it
  brings back and the board moves are the next
  `chore(github): sync pm entities` commit.
- What the owner has to see himself goes to
  [.github/self-control.md](.github/self-control.md).

### When `cd` fails

- **Before the GitHub release exists** (the tag checks, tests or the build;
  `gh release view vX.Y.Z` finds no release): nothing is published. Delete
  the tag with `git push --delete origin vX.Y.Z && git tag -d vX.Y.Z`, fix the
  cause on `main` or in your signing setup, and go back to step 4.
- **A prerelease reached a stable channel** (*Latest* or the tap changed):
  restore the previous stable release as *Latest* and the Formula, then
  file an issue before the next candidate.
- **Only the vanity job failed**: the release and the Formula are out and
  stay. Fix the cause, for example the App's installation on
  `octolab/vanity`, and run the failed job again:
  `gh run rerun <run-id> --failed`.
- **Anything else after the release exists**: it is frozen; ship the fix in
  the next patch.

### 6. Announcement

Add a row for "Latest contributions" in the
[kamilsk/kamilsk](https://github.com/kamilsk/kamilsk) profile README, in the
checkout `~/Development/public/kamilsk/kamilsk`, stage it with the message
`chore: add new contribution log entry` and hand over its `git contrib`, as
octopot/indexit's AGENTS.md shows. Newest first, one row per release day; a
prerelease gets no row. The reference label is `o` + project initials +
version digits, `op010` for pulse v0.1.0. Notes is a short phrase, no longer
than the existing ones, saying what the user gets.

### 7. Self-control and cleanup

[.github/self-control.md](.github/self-control.md) holds the checks that
outlive a session: each item says when to look, what proves it and what to do
if it fails. The agent adds the checks a release leaves open, removes an item
once it is proven or filed as an issue, and keeps the follow-ups that still
need one. The file is public: no private paths, notes, secrets or names of
agent sessions. Its changes go with the commit they belong to, or with the
next `chore(github): sync pm entities` commit.

The drafts of a release live in `.agents/release/vX.Y.Z/` (plan, note,
annotation, migration). After the checks of step 5 the agent removes them,
its scratch services, builds and test folders, and any backup the owner has
confirmed.
