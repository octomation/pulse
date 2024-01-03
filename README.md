> # 📰 pulse
>
> The one point of view to all external activities.

**All your activity. In one view.**

pulse aims to bring what you do on external services, starting with GitHub, into one place. It comes as a service, `pulse`, and its client, `pulsectl`; neither collects any activity yet.

## Status

pulse is in early development and has no release yet: there is nothing to install. The plans are in the [issues](https://github.com/octomation/pulse/issues): a port of [gitboard](https://gitboard.io) and of [watchub](https://github.com/octomation/watchub), and a rethink of [tablo](https://github.com/octopot/tablo).

Once released, pulse installs with [Homebrew](https://brew.sh/) as `brew install --formula octolab/tap/pulse`, or from a release archive for macOS and Linux on `amd64` and `arm64`.

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

`make setup` wires the git hooks in `.github/hooks` and installs the development tools into `bin/<os>/<arch>`, where `make install` puts both binaries and `source bin/activate` adds them to `PATH`; `make help` lists the other targets. The module path is `go.octolab.org/toolset/pulse`, the project's former name.

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
