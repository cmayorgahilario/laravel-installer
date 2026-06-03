# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

A Go CLI that creates a Laravel + Sail project via Docker. It does NOT call
`laravel.build` — it generates its own equivalent bash script and runs it. This
unlocks the full 15-service Sail catalog (incl. `mongodb`, `rustfs`, `rabbitmq`)
instead of only the 12 the `?with=` URL exposes.

How to run:
- **Interactive wizard** (the only mode): app name/folder/url → starter kit select
  (`laravel new --using`, "" = base) → "use default services?" confirm. YES (default)
  skips straight to the final confirm using the loaded config; NO reveals the
  single-select service groups + add-ons multiselect. The starter kit is asked
  BEFORE the defaults confirm so it's never hidden by the shortcut.
- Flags: `--dry-run` prints the script instead of executing (no Docker needed);
  `--debug` streams Docker's log live instead of hiding it behind the spinner;
  `--list-services` prints the grouped Sail catalog (`installer.FormatCatalog`)
  and exits; `--version` prints build vars. Under WSL, `main` warns if
  `ui.CodeBase()` is on a Windows mount (`installer.IsWindowsMount`, `/mnt/...`)
  since Docker I/O there is slow.

The emitted script runs
`docker run … laravelsail/php84-composer:latest … laravel new … sail:install --with=…`.

The composer image is **fixed at php84** on purpose: it is only the ephemeral
bootstrap container. The project's actual PHP version is set later in Sail's
`compose.yaml` (runtimes/8.x), independent of this image — so there is no PHP
selector.

The compiled binary is named `laravel` (the binary name comes from the `cmd/laravel/`
directory name).

## Commands

```bash
# Iterate without compiling
go run ./cmd/laravel

# Build to bin/laravel
go build -o bin/laravel ./cmd/laravel

# Tests (none exist yet; this is the pattern when added)
go test ./...
go test ./internal/installer -run TestBuildURL   # single test

go vet ./...
go fmt ./...
```

A `Makefile` mirrors these (`build`, `run`, `clean`, `fmt`, `install`, `release`),
but **`make` is not installed in this environment** — invoke the `go` commands
directly. `make install` copies the binary to `/usr/local/bin/laravel`;
`make release` cross-compiles for linux/amd64 and darwin amd64+arm64 into `bin/`.

## Architecture

`main` resolves a `*installer.Spec` from either the wizard or flags, then
executes (or prints, with `--dry-run`).

- `cmd/laravel/main.go` — thin entrypoint. `CheckDocker()` BEFORE the wizard
  (skipped in `--dry-run`), runs `ui.Run()`, builds a `Spec`, then `BuildScript`
  (`--dry-run`) or `Run(spec, debug)`. Guards that `~/code/<folder>` doesn't already
  exist (wizard validates too; this is the backstop). On error, warns a partial
  install may remain. On success, `printNextSteps` (cd + `sail up -d` + migrate if
  DB + npm run dev if kit) then offers `installer.SailUp` (`sail up -d`, detached)
  via `ui.ConfirmLaunch` (only if `installer.IsInteractive()`).

- `internal/ui/form.go` — the huh wizard. `Choice` embeds `config.Config` (service
  fields + `Services()`) plus `AppName` (→ APP_NAME, allows spaces), `Folder`
  (the `~/code/<folder>` dir), `URL`, `Confirmed`. App name and folder are SEPARATE:
  `Folder` may be blank, in which case `ResolvedFolder()` derives it as
  `Slugify(AppName)`. The folder input shows the slug as a live `PlaceholderFunc`
  bound to `&c.AppName`. A `useDefaults` confirm (default true) gates the service
  groups via `.WithHideFunc` — true hides them all (use loaded config), false shows
  them. There is NO location field — base is always `~/code`. Each
  heavy `huh.Select` (DB, cache) is in its OWN group (stacking selects makes huh
  compress them). The 4 add-ons are ONE `huh.MultiSelect` bound to `Config.Addons`
  (stacked Confirms don't align their Yes/No buttons). Persists via `config.Save`.

- `internal/ui/paths.go` — `CodeBase()` = fixed `~/code`; `InstallPath(folder)` =
  `~/code/<folder>`; `Slugify` (name → folder slug); `ValidateURL`,
  `ValidateAppName` (allows spaces, must slug non-empty, no `"|&\$`),
  `ValidateName`/`ValidateFolderOptional` (folder slug; optional = blank ok).

- `internal/config/config.go` — persistence at `~/.config/laravel-installer/config.json`
  (via `os.UserConfigDir`). `Load` falls back to `Default()` on any error; `Save`
  is best-effort. Add-ons are `Addons []string` (not 4 bools); `AddonOrder` fixes
  their order in `--with=`. `Config.Services()` is the single flatten-to-`--with=`
  helper, reused by ui and main. Defaults: pgsql, valkey, addons=[mailpit].

- `internal/installer/installer.go` — `BuildScript(Spec)` renders a bash script
  from a `text/template`. `composerImage` is the fixed `laravelsail/php84-composer`.
  The script `mkdir -p`s `Spec.Location`, mounts it as the Docker volume, creates
  the project (`laravel new <Spec.Name>`), then `sed`s `APP_NAME="<Spec.AppName>"`
  and `APP_URL=http://<Spec.URL>` into `.env`. Empty
  service list → `--with=none` and a build-only path (no `sail pull`). `Run(Spec)`
  `innerInstall`: no kit → uses the image's `laravel` (Installer **5.10.0**). With
  a kit, the image is short in TWO ways, both worked around inline before
  `laravel new`: (1) no Node/npm — kits run `npm install`, so we install Node via
  tarball into `/usr/local` (~4s, `uname -m`→x64/arm64; apt takes >2min, too slow);
  (2) Installer 5.10.0 lacks `--using` — `composer global require laravel/installer`
  (→5.28+) then prepend the global bin-dir to PATH. Then `laravel new …
  --using=<Spec.Template>`. The in-container command uses `$(...)`/`$VAR`, so
  `escapeForDoubleQuotes` (not just `"`-escaping) escapes `\ " ` + `$` so they reach
  the container's bash literally in `--dry-run`. (check installer version:
  `docker run --rm laravelsail/php84-composer:latest laravel --version`).
  `Run(spec, debug)` executes in 3 phases (`runPhase`), each behind a
  `huh/spinner` that hides Docker's log (dumped only on failure). `debug=true` or
  no TTY → stream the log live instead. `innerInstall` (the in-container command,
  incl. the `.env` seds) is shared by `Run` and `BuildScript` so they don't
  diverge. `fixPermissions` does the post-build `chown` (no spinner — may prompt
  for a password). `Services` (15-name catalog) + `ValidateServices` live here.
  `Spec.Location` is always `ui.CodeBase()` (~/code), set by `main`.

`internal/` is used (not `pkg/`) deliberately — nothing here is meant to be
imported by other modules.

## Branching & release flow

- **Branches:** feature branches → `develop` (integration) → PR `develop` →
  `master` (stable/released). `master` is protected: PR-only, CI must pass.
- **Conventional Commits are REQUIRED** on what lands in `master` — the next
  version is derived from them. `feat:` → minor, `fix:` → patch, `feat!:`/`fix!:`
  or `BREAKING CHANGE:` → major. `chore:`/`docs:`/`test:` don't trigger a release.
- **Releasing is automatic on merge:** merge `develop` → `master` and the
  `release` workflow tags `vX.Y.Z` and publishes the GitHub Release with binaries
  — NO intermediate release PR, NO manual `git tag`. Trigger is `push: branches:
  [master]`; "only via a merged PR" is enforced by **branch protection** (master
  is PR-only, no direct pushes). A merge with only `chore:`/`docs:` commits → no
  version bump → no tag.

## Distribution & CI

**Linux-only on purpose.** The tool targets WSL/Linux; macOS already has its own
Laravel tooling (Herd/Valet/laravel.build). GoReleaser builds and install.sh only
support `linux` — don't re-add a `darwin` target unless the scope changes.

- `.goreleaser.yaml` — GoReleaser v2 config. Cross-compiles `cmd/laravel` for
  linux / amd64+arm64 (`CGO_ENABLED=0`, `-s -w` + `-X main.{version,commit,date}`)
  and uploads `.tar.gz` archives named `laravel-installer_Linux_{x86_64|arm64}.tar.gz`
  + `checksums.txt`. GoReleaser creates the GitHub Release and its notes (no
  external changelog tool).
- `.github/workflows/release.yml` — on `push: branches: [master]`: computes the
  next version with `svu` (`github.com/caarlos0/svu`; `svu current` vs `svu next`
  from the Conventional Commits — handles the no-prior-tag bootstrap, which
  `github-tag-action` did NOT). If it bumps, it creates+pushes `vX.Y.Z` and runs
  `goreleaser-action@v7`; otherwise the tag/release steps skip. Default
  `GITHUB_TOKEN` (no PAT). No release-please. Rely on branch protection so only
  merged PRs reach `master`.
- `.github/workflows/ci.yml` — only on `pull_request: branches: [master]`: gofmt
  check, `go vet`, `go test -race`, and `golangci-lint` (action v8 / v2.5.0).
- `install.sh` — `curl … | bash` installer. Linux-only; detects arch (`uname`,
  mapped to the SAME x86_64/arm64 format GoReleaser emits), resolves the latest
  release via the GitHub API (or `VERSION=`), downloads+extracts the matching
  archive, installs to `BIN_DIR` (default `/usr/local/bin`, sudo/doas fallback).
  **install.sh's asset name MUST stay in sync with `.goreleaser.yaml`'s
  `archives.name_template`.**
- `main.version/commit/date` are build vars (`go build` → "dev"); `laravel
  --version` prints them.
- Repo slug assumed `cmayorgahilario/laravel-installer` (in install.sh URL +
  README).

`installer.IsWSL()` (reads `/proc/version` for "microsoft"/"wsl") and
`installer.OpenURL(url)` (prefers `wslview`/`explorer.exe` under WSL, else
`xdg-open`) power the post-`sail up` browser open in `main`.

## Key constraints

- **huh requires a real TTY.** Running the binary without an interactive terminal
  (e.g. piping input, CI, this agent's sandbox) fails with
  `could not open a new TTY` — that's expected, not a bug. Test the wizard in a
  real terminal.
- **The generated script interpolates the project name into bash unquoted**, so
  the name is validated in `ui` against `^[a-zA-Z0-9_.-]+$` to prevent shell
  injection. Keep that validation if you change the input flow. Use `--dry-run`
  to print the script instead of executing it.
- **Service names must match Sail's `$services` list** (in laravel/sail's
  `InteractsWithDockerComposeServices` trait), NOT the docs prose. The 15 valid
  values: mysql, pgsql, mariadb, mongodb, redis, valkey, memcached, meilisearch,
  typesense, minio, rustfs, mailpit, rabbitmq, selenium, soketi.
- **The composer image is fixed at php84** (`installer.composerImage`). Do not add
  a PHP-version selector tied to this image — it does not affect the project's PHP
  version (that's Sail's `compose.yaml`). Also, `laravelsail/php85-composer` does
  not exist on Docker Hub (only php80–php84).
- `go.mod` declares `go 1.25.0`; the installed toolchain is 1.26. `bubbletea` and
  `lipgloss` are present as transitive deps of `huh` — not used directly yet, but
  available if a live progress UI is added around the install phase.
