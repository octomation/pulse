> # 📰 pulse
>
> Your morning, with a hand on the pulse.

**Only what is new. One calm morning paper.**

pulse folds the daily, weekly and monthly GitHub trends of [trendshift.io](https://trendshift.io/) and [github.com/trending](https://github.com/trending) into one page on your computer that shows only the repositories you have not seen yet. Read a repository without leaving the page, keep what matters with a comment, and export your bookmarks with `pulsectl`.

```sh
brew install --formula octolab/tap/pulse
pulse                      # then open http://pulse.localhost:17842/
pulsectl bookmarks         # the repositories you kept, one URL per line
```

The current release is [v0.1.0](https://pulse.octolab.org/changelog/v0.1.0/), for macOS and Linux on `amd64` and `arm64`; archives are on the [release page](https://github.com/octomation/pulse/releases/tag/v0.1.0). The [guide](https://pulse.octolab.org/trend-watch/) shows how to connect the bookmarks of your notes vault and a GitHub token.

## Development

<details>
<summary>Build from source</summary>

To work on pulse itself, clone the repository and build it with Go 1.27 or newer:

```sh
git clone https://github.com/octomation/pulse.git
cd pulse
make setup
make install
source bin/activate
pulse version
```

`make setup` wires the git hooks in `.github/hooks` and installs the development tools into `bin/<os>/<arch>`, where `make install` puts both binaries and `source bin/activate` adds them to `PATH`; `make help` lists the other targets. The module path is `go.octolab.org/toolset/pulse`.

</details>

How the repository is checked, released and kept healthy: [AGENTS.md](AGENTS.md), [.github/workflows/README.md](.github/workflows/README.md) and [tools/README.md](tools/README.md).

## Documentation site

```sh
./Taskfile docs npm ci
./Taskfile docs dev
```

See [docs/README.md](docs/README.md) for the site structure, production build, and static export.

---

<p align="center">
  <sub>pulse 📰</sub><br>
  <sub>MIT · Built by <a href="https://github.com/octolab">OctoLab.</a></sub>
</p>
