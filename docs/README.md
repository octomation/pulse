# pulse documentation

An English site for pulse v0.1.0: a landing page with an isometric morning desk that plays as the page scrolls, the trend-watch guide and the changelog. Nextra 4 on Next.js, the stack shared with octopot/indexit and octomation/maintainer.

## Run locally

Use Node 24 (the CI version). From the repository root:

```sh
./Taskfile docs npm ci
./Taskfile docs dev
```

Open the local URL printed by Next.js, normally <http://localhost:3000>.

Or, from `docs/`, run `npm ci` and `npm run dev`.

## Build

For a production server:

```sh
./Taskfile docs build
./Taskfile docs start
```

For GitHub Pages:

```sh
TARGET=static SITE_URL=https://pulse.octolab.org/ ./Taskfile docs build
```

Static output is written to `docs/dist/`. `SITE_URL` is required; add `BASE_PATH=/pulse` only when hosting under a path, as on the default `octomation.github.io/pulse/`. The [Pages workflow](../.github/workflows/cd.docs.yml) supplies both: from the Pages configuration on main, from `.github/settings.json` for PR previews. Local development remains available while the static build runs.

## Edit the content

| Location | Purpose |
| --- | --- |
| `content/index.mdx` | The landing page |
| `components/morning.jsx` | The first screen of the landing page: the isometric desk in CSS 3D and the five steps of a morning. The script writes the scroll and the pointer into custom properties; the motion is CSS in `app/landing.css` |
| `components/reveal.jsx` | Fades a block in when it scrolls into view |
| `content/trend-watch.md` | Trend watch: install, the page, the reader, the export and the limits |
| `content/changelog/index.mdx` | Release overview |
| `content/changelog/v0.1.0.md` | The latest release notes, also the GitHub release body |
| `content/**/_meta.js` | Navigation order and labels |
| `navigation.mjs` | Release groups in the sidebar; keeps published release URLs unchanged |
| `app/globals.css` | Shared layouts, kept identical to octopot/indexit |
| `app/brand.css` | The tokens of pulse: paper, coffee ink, terracotta accent |
| `app/landing.css` | The landing page and the desk: its boxes, the objects and what each step moves |
| `app/layout.jsx` | Site identity and Nextra theme |
| `app/[[...mdxPath]]/page.jsx` | Page-specific titles and social metadata |
| `public/` | Favicon, social preview (see [below](#social-preview-image)) and the screenshots of the page in `screens/` |
| `scripts/og.html`, `scripts/og.mjs` | The source of `og.png` and its renderer |

The public origin in metadata comes from `SITE_URL` (see `site.mjs`); the Pages workflow sets it from the Pages configuration. Internal links use Next.js/Nextra and keep the configured base path.

Changelog is a section of the sidebar, with All releases first. `navigation.mjs` groups release pages into expandable `v0.1.x`, `v0.2.x`, and later branches, newest first. Add each release note and its label to `content/changelog/_meta.js`; the version group is created automatically. A prerelease note, such as `v0.1.0-rc1`, is not grouped: add it with `display: 'hidden'`, so its page exists for the GitHub release but the sidebar shows stable releases only. Keep the notes at `content/changelog/{tag}.md` so published links and the release workflow stay valid. Top-level navbar links use separate `type: 'page'` aliases; content folders use `display: 'children'`.

## Social preview image

`public/og.png`, 1733×908 RGB, is the social preview of the home page; other pages share a link without an image (`app/[[...mdxPath]]/page.jsx`). On warm paper it shows the `📰 pulse` wordmark, the slogan "Your morning, with a hand on the pulse." with "the pulse." in terracotta italics, the monospaced line "Only what is new. One calm morning paper." and the version label at the bottom left. On the right is the desk of the landing page, held still at 07:35: the paper, a story risen above it, the coffee, the clock and the glasses.

`scripts/og.html` holds the `.iso-stage` element as the build renders it, inside a `.morning` element whose custom properties (`--p1`…`--p5`, `--day`, `--s`) fix the moment, and uses the styles of `app/brand.css` and `app/landing.css`. Render it with headless Chrome, with a copy of `playwright-core` the script can import:

```sh
PLAYWRIGHT_CORE=/path/to/playwright-core/index.js node scripts/og.mjs
```

The version label `v0.1.0` is set in `ui-monospace` (Menlo on macOS), weight 600, 26 px, colour `#b0513a`; its box is x 96, y 818, 94×26 px. A release changes only the text of `.version` in `og.html` and renders again; the script prints the box of the label. Compare the result with the previous image: only the pixels inside that box may change, so a longer version that widens the box needs the same check. When the desk changes, copy `.iso-stage` from `dist/index.html` again and set the time on the clock to the step shown.

## Change the domain

The site is served from the custom domain `pulse.octolab.org`; `octomation.github.io/pulse/` redirects to it, keeping the path. The build bakes the domain into asset paths and metadata, and changing it in Settings → Pages triggers no rebuild. To change it:

1. Update `pages.cname` in [`.github/settings.json`](../.github/settings.json) (remove it for the default domain) and push.
2. Change Settings → Pages → Custom domain to match.
3. Rebuild: `gh workflow run cd.docs.yml -f reason="domain change"`.

The docs build stops while the two disagree, the deployment is smoke-tested, and the daily [doctor](../.github/workflows/doctor.yml) reports a stale site.

## Keep the release accurate

The implementation is the source of truth: check examples against command help and tests. Closed issues provide context, not a substitute for checking the current code.

Lead installation instructions with Homebrew and release binaries; keep source builds as an optional contributor workflow. Archive names and supported platforms must match `.goreleaser.yml`. Do not describe planned features as shipped. The current release is v0.1.0.

Release notes describe changes to pulse's behavior. Keep documentation, development tools, CI/CD, and repository inventory changes out of the product changelog. A release note describes its release as published, with examples that reproduce on the built binaries. Preserve historical release notes when updating the current installation instructions. How to write a note and cut a release: [AGENTS.md](../AGENTS.md#releasing).

`package.json` pins Zod to `4.1.12` under `nextra-theme-docs` to avoid [a validation regression](https://github.com/shuding/nextra/issues/5036). In the current dependency tree, this also covers the theme's Nextra peer dependency. Keep the exact Zod version: a caret range allows incompatible releases. When upgrading Nextra and its theme, check whether the site builds without this override. Zod before 4.6.6 has a memory exhaustion advisory (GHSA-mrhx-c7xc-m3w5); its Dependabot alert is dismissed as a tolerable risk in octopot/indexit and octomation/maintainer, and gets the same answer here, since only the static build validates the repository's own files, and Zod 4.6.6 still fails the build with that regression. Once a Nextra release after 4.6.1 ships [its fix](https://github.com/shuding/nextra/pull/4990), move the override to `4.6.6` or later, or drop it, and check the static export. See [Nextra’s docs theme guide](https://nextra.site/docs/docs-theme/start) for the underlying structure.

Nextra reaches minimatch 10 through `ts-morph` for TSDoc, which the site does not use. Its advisory GHSA-g3hv-75wg-m249 needs `optimizationLevel` 2 or more, while `@ts-morph/common` calls minimatch without options, at level 1, so the alert is dismissed as not used, here as in octopot/indexit, and there is no override: minimatch 11 is outside the range `ts-morph` declares. Look again when `ts-morph` or its glob options change.

For `speech-rule-engine@4.1.4`, an override replaces its pinned xmldom dependency with `@xmldom/xmldom@^0.9.12` to include the security fixes while allowing later `0.9.x` patches. The lockfile records the resolved version. This override stops applying when speech-rule-engine changes version; run `npm audit` after that update and remove or revise the rule as needed.

Search is disabled: static search indexing is not configured.
