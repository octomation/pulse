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
- [ ] **Before the next release**: Dependabot has open alerts for
  `docs/package-lock.json` (zod, minimatch, katex) and `tools/go.mod`
  (go.opentelemetry.io/otel). None is in the binaries. Update the
  packages, then check that `make go-tools-check` and the site build pass.
- [ ] **After the push**: `gh workflow run cd.yml -f reason="snapshot"` on
  `main` builds a snapshot of both binaries and publishes nothing.
