# laravel-installer

A command-line tool, written in Go, for creating **Laravel + Sail** projects on
Docker. Through an interactive terminal wizard, the tool collects the project
configuration (name, starter kit and services) and generates it under the
`~/code/<folder>` directory.

It is an extended alternative to Laravel's web installer:

```bash
curl -s "https://laravel.build/example-app?with=mysql,redis" | bash
```

Unlike `laravel.build`, this tool exposes the full Sail service catalog
—including `mongodb`, `rustfs` and `rabbitmq`, which are not available through
the URL—, supports installing starter kits and distinguishes between the
application name and the install directory name.

> [!NOTE]
> **Recommended for WSL (Linux) or native Linux.** This tool is built and
> distributed for Linux only. On macOS, use the native Laravel tooling
> ([Herd](https://herd.laravel.com), Valet or `laravel.build`) instead.

## Features

- Interactive wizard built with [huh](https://github.com/charmbracelet/huh).
- Full catalog of the 15 Sail services, organized by category.
- Support for starter kits (React and Livewire, with their *blank* variants).
- Persistence of the last configuration used.
- Per-phase progress indicator that hides Docker's output.
- Optional project startup via `sail up -d` at the end, opening the URL in the
  browser (the Windows browser under WSL, via `wslview`/`explorer.exe`).
- Docker availability check before the wizard.

## Requirements

- WSL or Linux. The tool is built for this environment; on macOS, use the native
  Laravel tooling (Herd, Valet, `laravel.build`) instead.
- Docker running (Docker Desktop with WSL integration, or Docker Engine).
- Go 1.25 or higher (only to build the binary).
- Projects are always installed under `~/code/`.

## Installation

### Quick method (recommended)

Downloads the binary matching your system from the latest release and installs
it to `/usr/local/bin`:

```bash
curl -fsSL https://raw.githubusercontent.com/cmayorgahilario/laravel-installer/main/install.sh | bash
```

You can pin a version (any published tag, e.g. `v0.2.0`) or change the
destination directory:

```bash
VERSION=vX.Y.Z BIN_DIR=~/.local/bin \
  curl -fsSL https://raw.githubusercontent.com/cmayorgahilario/laravel-installer/main/install.sh | bash
```

Alternatively, download the matching archive from the
[Releases](https://github.com/cmayorgahilario/laravel-installer/releases) page,
extract it and install the binary manually.

### Building from source

Requires Go 1.25 or higher. The resulting binary is named `laravel`:

```bash
go build -o bin/laravel ./cmd/laravel
```

Optional installation into the system `PATH`:

```bash
sudo install -m 755 bin/laravel /usr/local/bin/laravel
# Or, if you have make:
make install
```

## Usage

### Interactive wizard

```bash
laravel
```

The procedure consists of the following steps:

1. **Application name** (`APP_NAME`, allows spaces) and **URL** (`APP_URL`).
2. **Install folder**, proposed automatically from the name (for example,
   `My Store` → `my-store`) and editable.
3. **Starter kit**: none, React, React (blank), Livewire or Livewire (blank).
4. **Service selection**: choosing the defaults skips the manual selection;
   otherwise, you pick database, cache, search engine, storage and additional
   services.
5. **Confirmation** and creation of the project under `~/code/<folder>`.

When finished, the tool shows the next steps and offers to start the environment
with `sail up -d`.

### Options

| Option | Description |
|--------|-------------|
| `--dry-run` | Prints the resulting script without running it. No Docker required. |
| `--debug` | Shows Docker's output in real time instead of the progress indicator. |
| `--list-services` | Prints the full Sail service catalog (grouped) and exits. |
| `--version` | Prints the version and exits. |

```bash
laravel --dry-run
laravel --debug
```

## Available services

| Group | Options |
|-------|---------|
| Database | `mysql`, `pgsql`, `mariadb`, `mongodb` |
| Cache & memory | `redis`, `valkey`, `memcached` |
| Search | `meilisearch`, `typesense` |
| Storage (S3) | `rustfs`, `minio` |
| Additional | `rabbitmq`, `mailpit`, `selenium`, `soketi` |

Defaults: `pgsql`, `valkey` and `mailpit`.

## Starter kits

When a starter kit is selected, the tool performs two additional operations
inside the container, since the `laravelsail/php84-composer:latest` image does
not ship them:

1. Installs **Node.js** (via direct download), required by the `npm install`
   that starter kits run.
2. Updates the **Laravel Installer** to its latest version, as the one bundled
   in the image (5.10.0) doesn't support the `--using` option.

These operations run only when a starter kit is chosen. Creating a base project
uses the version bundled in the image directly.

## Configuration

The last service and starter-kit selection is stored at:

```
~/.config/laravel-installer/config.json
```

On subsequent runs, the wizard presents those values as defaults.

## Development

```bash
go run ./cmd/laravel
go build -o bin/laravel ./cmd/laravel
go test ./...
go vet ./...
```

### Project structure

```
cmd/laravel/         Entry point (flags and orchestration)
internal/ui/         Wizard (huh), validations and path helpers
internal/installer/  Script generation and execution (Docker, phases, progress)
internal/config/     Persistence under ~/.config/laravel-installer
```

The install process runs in three phases (project creation, image pulling and
container building). Each phase is accompanied by a progress indicator that
captures the output and shows it only on error.

### Branching & releasing

Development happens on `develop`; `master` is the stable, released branch.

```
feature branch → develop → PR → master
```

Releases are automated with [GoReleaser](https://goreleaser.com). Commits follow
[Conventional Commits](https://www.conventionalcommits.org) (`feat:`, `fix:`,
`feat!:`/`BREAKING CHANGE:`), which is how the next version is derived. The flow:

1. Open a PR `develop → master` and merge it once approved.
2. On merge (a push to `master`), the `release` workflow computes the next
   version from the commits, tags `vX.Y.Z`, and GoReleaser builds the Linux
   binaries (`amd64`/`arm64`) and publishes the GitHub Release.

There is no intermediate release PR and no manual tagging. The workflow triggers
on push to `master`; keeping `master` **branch-protected** (PR-only, no direct
pushes) ensures releases only happen through a merged PR. A merge with only
`chore:`/`docs:` commits produces no release. The config lives in
`.goreleaser.yaml` and `.github/workflows/release.yml`. To validate the GoReleaser
build locally, without publishing:

```bash
goreleaser release --snapshot --clean
```

CI (formatting, vet, tests and lint) runs on every pull request targeting
`master`.

## Notes

- The application name is interpolated into the `.env` file and the folder name
  into shell commands; both are validated to prevent command injection.
- The `.env` configuration (`APP_NAME`, `APP_URL`) is done inside the container
  to avoid permission conflicts.
- The Docker check pings the daemon's `/_ping` endpoint directly over the API
  socket (a few milliseconds), instead of spawning the docker CLI, so the wizard
  starts without a noticeable delay. It honours `DOCKER_HOST` and falls back to
  `docker version` for non-standard connections (e.g. `ssh://`).
- Under WSL, if `~/code` resolves to a Windows mount (`/mnt/...`), the tool warns
  you: Docker/Sail I/O there is much slower than on the native Linux filesystem.

## License

Distributed under the MIT License. See the [LICENSE](LICENSE) file for details.
