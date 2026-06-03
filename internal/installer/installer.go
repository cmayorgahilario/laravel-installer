// Package installer generates and runs the script that creates a Laravel + Sail
// project via Docker, replicating what laravel.build does but with control over
// the PHP version and the full service catalog.
package installer

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/charmbracelet/huh/spinner"
	"github.com/mattn/go-isatty"
)

// composerImage is the ephemeral container that runs laravel new + composer +
// sail:install. The project's PHP version does NOT depend on this image: it's
// set later in Sail's compose.yaml (runtimes/8.x). That's why it's fixed, same
// as in laravel.build.
const composerImage = "laravelsail/php84-composer:latest"

// Spec are the parameters to generate the install script.
type Spec struct {
	Name     string // project folder (laravel new arg)
	AppName  string // APP_NAME in the .env (may contain spaces)
	URL      string // used as APP_URL (http://<URL>)
	Template string // starter kit for laravel new --using; "" = base project
	Location string // base directory (absolute path) where the project is created
	Services []string
}

// Services is the valid catalog for sail:install --with= (Sail v13).
var Services = []string{
	"mysql", "pgsql", "mariadb", "mongodb",
	"redis", "valkey", "memcached",
	"meilisearch", "typesense",
	"minio", "rustfs",
	"mailpit", "rabbitmq", "selenium", "soketi",
}

// ValidateServices checks that each service exists in Sail's catalog.
func ValidateServices(services []string) error {
	valid := make(map[string]bool, len(Services))
	for _, s := range Services {
		valid[s] = true
	}
	for _, s := range services {
		if !valid[s] {
			return fmt.Errorf("unknown service: %q (valid: %s)",
				s, strings.Join(Services, ", "))
		}
	}
	return nil
}

// withArg is the value of --with=. Sail accepts "none" when there are no services.
func withArg(services []string) string {
	if len(services) == 0 {
		return "none"
	}
	return strings.Join(services, ",")
}

// nodeVersion is the Node version installed (via tarball) when a starter kit is
// used, because its assets run "npm install" and the composer image ships no Node.
const nodeVersion = "v22.14.0"

// innerInstall is the command that runs INSIDE the container: it creates the
// project, installs Sail and configures the .env. It runs as root, which is why
// the .env is edited here (not on the host, where it would fail on permissions).
// It's shared by Run (real execution) and BuildScript (--dry-run) so they don't
// diverge.
func innerInstall(s Spec) string {
	// No starter kit: the image's laravel (5.10.0) is enough for a base project.
	newProject := fmt.Sprintf("laravel new %s --no-interaction", s.Name)
	if s.Template != "" {
		// With a starter kit, the image falls short in two ways:
		//  - it ships no Node/npm (kits run 'npm install') → tarball to /usr/local (~4s);
		//  - its installer (5.10.0) doesn't support --using → composer global require latest.
		newProject = fmt.Sprintf(
			`ARCH=$(uname -m | sed 's/x86_64/x64/;s/aarch64/arm64/') `+
				`&& curl -fsSL https://nodejs.org/dist/%[1]s/node-%[1]s-linux-$ARCH.tar.xz `+
				`| tar -xJ -C /usr/local --strip-components=1 `+
				`&& composer global require laravel/installer --no-interaction `+
				`&& export PATH="$(composer global config bin-dir --absolute -q):$PATH" `+
				`&& laravel new %[2]s --no-interaction --using=%[3]s`,
			nodeVersion, s.Name, s.Template)
	}
	return fmt.Sprintf(
		`%s && cd %s && composer require laravel/sail --dev `+
			`&& php ./artisan sail:install --with=%s `+
			`&& sed -i 's|^APP_NAME=.*|APP_NAME="%s"|' .env `+
			`&& sed -i 's|^APP_URL=.*|APP_URL=http://%s|' .env`,
		newProject, s.Name, withArg(s.Services), s.AppName, s.URL)
}

// escapeForDoubleQuotes escapes what's needed to embed a command inside
// bash -c "...". It escapes \, ", ` and $ so that $(...) and $VAR reach the
// inner bash (the container's) literally and aren't expanded by the shell that
// runs the script.
func escapeForDoubleQuotes(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, "`", "\\`")
	s = strings.ReplaceAll(s, `$`, `\$`)
	return s
}

var scriptTmpl = template.Must(template.New("install").Parse(`#!/usr/bin/env bash
set -e

# Check that Docker is running...
if ! docker version > /dev/null 2>&1; then
    echo "Docker is not running." >&2
    exit 1
fi

# Base install directory.
mkdir -p "{{.Location}}"
cd "{{.Location}}"

# Create the Laravel project, install Sail and configure the .env. All inside
# the container (as root) to avoid permission issues: root creates the files and
# the final chown hands them back to the user.
docker run --rm \
    --pull=always \
    -v "{{.Location}}":/opt \
    -w /opt \
    {{.Image}} \
    bash -c "{{.Inner}}"

cd {{.Name}}

{{if .HasServices}}./vendor/bin/sail pull {{.ServicesSpaced}}
{{end}}./vendor/bin/sail build

# Fix permissions of the files created by the container (root).
if command -v doas &>/dev/null; then
    SUDO="doas"
elif command -v sudo &>/dev/null; then
    SUDO="sudo"
fi
if [ -n "$SUDO" ]; then
    $SUDO chown -R "$USER": .
fi

echo ""
echo "Done. Start with: cd {{.Location}}/{{.Name}} && ./vendor/bin/sail up"
`))

// BuildScript generates the bash install script from the Spec. Used in
// --dry-run; the inner command's double quotes are escaped so it can sit inside
// bash -c "...".
func BuildScript(s Spec) string {
	var b strings.Builder
	_ = scriptTmpl.Execute(&b, struct {
		Image          string
		Inner          string
		Name           string
		Location       string
		HasServices    bool
		ServicesSpaced string
	}{
		Image:          composerImage,
		Inner:          escapeForDoubleQuotes(innerInstall(s)),
		Name:           s.Name,
		Location:       s.Location,
		HasServices:    len(s.Services) > 0,
		ServicesSpaced: strings.Join(s.Services, " "),
	})
	return b.String()
}

// CheckDocker verifies that the docker command exists and that the daemon is
// running. Called before the wizard to avoid wasting the user's time if Docker
// isn't available.
//
// The daemon check pings the API socket directly (/_ping), which answers in a
// few milliseconds — far cheaper than spawning the docker CLI ("docker version"
// forks a large Go binary that takes ~200-300ms to start and connect). Only if
// the direct ping is inconclusive (a non-default DOCKER_HOST such as ssh:// or a
// TLS endpoint) do we fall back to the CLI for an authoritative answer.
func CheckDocker() error {
	if _, err := exec.LookPath("docker"); err != nil {
		return fmt.Errorf("the 'docker' command was not found; install it and try again")
	}
	if pingDaemon() {
		return nil
	}
	// Fallback: connection types we don't ping directly, or a slow/edge daemon.
	if err := exec.Command("docker", "version").Run(); err != nil {
		return fmt.Errorf("the Docker daemon is not running; start it and try again")
	}
	return nil
}

// pingDaemon hits the Docker Engine's /_ping endpoint over the daemon socket and
// reports whether it answered OK. It honours DOCKER_HOST (unix:// and tcp://);
// for any other scheme it returns false so the caller falls back to the CLI.
func pingDaemon() bool {
	network, addr := "unix", "/var/run/docker.sock"
	if host := os.Getenv("DOCKER_HOST"); host != "" {
		u, err := url.Parse(host)
		if err != nil {
			return false
		}
		switch u.Scheme {
		case "unix":
			network, addr = "unix", u.Path
		case "tcp":
			network, addr = "tcp", u.Host
		default:
			return false // ssh://, npipe://, etc. → let the CLI handle it
		}
	}

	client := http.Client{
		Timeout: 2 * time.Second,
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				return (&net.Dialer{}).DialContext(ctx, network, addr)
			},
		},
	}
	// The host part is ignored by our custom dialer; any value works.
	resp, err := client.Get("http://docker/_ping")
	if err != nil {
		return false
	}
	defer func() { _ = resp.Body.Close() }()
	return resp.StatusCode == http.StatusOK
}

// Run creates the project running the process in phases, each with a spinner
// that hides Docker's log (shown only if the phase fails). With debug=true the
// log is streamed live in each phase, without a spinner.
func Run(s Spec, debug bool) error {
	if err := os.MkdirAll(s.Location, 0o755); err != nil {
		return fmt.Errorf("could not create %s: %w", s.Location, err)
	}
	projectDir := filepath.Join(s.Location, s.Name)

	// Phase 1: create the project and install Sail inside the container.
	if err := runPhase(debug, "Creating Laravel project and installing Sail", s.Location,
		"docker", "run", "--rm", "--pull=always",
		"-v", s.Location+":/opt", "-w", "/opt", composerImage,
		"bash", "-c", innerInstall(s)); err != nil {
		return err
	}

	// Phase 2: pull the images of the chosen services.
	if len(s.Services) > 0 {
		if err := runPhase(debug, "Pulling Docker images", projectDir,
			"./vendor/bin/sail", append([]string{"pull"}, s.Services...)...); err != nil {
			return err
		}
	}

	// Phase 3: build the containers.
	if err := runPhase(debug, "Building containers", projectDir,
		"./vendor/bin/sail", "build"); err != nil {
		return err
	}

	// Give the user back the files created by root (may prompt for a password,
	// so it runs without a spinner).
	fixPermissions(projectDir)
	return nil
}

// IsInteractive reports whether there's a terminal (to decide whether to show
// prompts).
func IsInteractive() bool {
	return isatty.IsTerminal(os.Stdout.Fd())
}

// IsWSL reports whether we're running under WSL (checks /proc/version, where the
// WSL kernel reports "microsoft").
func IsWSL() bool {
	data, err := os.ReadFile("/proc/version")
	if err != nil {
		return false
	}
	v := strings.ToLower(string(data))
	return strings.Contains(v, "microsoft") || strings.Contains(v, "wsl")
}

// OpenURL opens a URL in the default browser. Under WSL it prefers wslview or
// explorer.exe to reach the Windows browser; otherwise it falls back to
// xdg-open. Best-effort and non-blocking: it returns an error only if no opener
// is found, not if the browser itself fails to launch.
func OpenURL(url string) error {
	candidates := []string{"xdg-open"}
	if IsWSL() {
		candidates = []string{"wslview", "explorer.exe", "xdg-open"}
	}
	for _, name := range candidates {
		if _, err := exec.LookPath(name); err == nil {
			// Start (not Run): fire-and-forget. explorer.exe in particular
			// returns a non-zero exit code even on success, which we don't want
			// to surface.
			return exec.Command(name, url).Start()
		}
	}
	return fmt.Errorf("no browser opener found")
}

// databaseServices are the services that require 'artisan migrate'.
var databaseServices = map[string]bool{
	"mysql": true, "pgsql": true, "mariadb": true, "mongodb": true,
}

// HasDatabase reports whether the selection includes a database.
func HasDatabase(services []string) bool {
	for _, s := range services {
		if databaseServices[s] {
			return true
		}
	}
	return false
}

// SailUp runs 'sail up -d' from the project directory (detached: the containers
// keep running in the background and the command returns).
func SailUp(projectDir string) error {
	fmt.Println("\n→ Bringing up with 'sail up -d'...")
	cmd := exec.Command("./vendor/bin/sail", "up", "-d")
	cmd.Dir = projectDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

// runPhase runs a command hiding its output behind a spinner. On failure it
// dumps the captured log. With debug, or without a TTY (CI, pipes), it runs
// streaming the output live.
func runPhase(debug bool, title, dir, name string, args ...string) error {
	if debug || !isatty.IsTerminal(os.Stdout.Fd()) {
		fmt.Println("→", title)
		cmd := exec.Command(name, args...)
		cmd.Dir = dir
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}

	var out bytes.Buffer
	action := func(ctx context.Context) error {
		cmd := exec.CommandContext(ctx, name, args...)
		cmd.Dir = dir
		cmd.Stdout = &out
		cmd.Stderr = &out
		return cmd.Run()
	}
	if err := spinner.New().Title(title).ActionWithErr(action).Run(); err != nil {
		fmt.Fprintf(os.Stderr, "\n✗ %s\n--- log ---\n%s\n", title, out.String())
		return fmt.Errorf("%s: %w", title, err)
	}
	fmt.Printf("✓ %s\n", title)
	return nil
}

// fixPermissions chowns the project to the user (the files were created by root
// inside the container). Uses doas or sudo if available.
func fixPermissions(dir string) {
	var sudo string
	if _, err := exec.LookPath("doas"); err == nil {
		sudo = "doas"
	} else if _, err := exec.LookPath("sudo"); err == nil {
		sudo = "sudo"
	}
	user := os.Getenv("USER")
	if sudo == "" || user == "" {
		return
	}
	fmt.Println("→ Fixing permissions (may ask for your password)...")
	cmd := exec.Command(sudo, "chown", "-R", user+":", ".")
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	_ = cmd.Run()
}
