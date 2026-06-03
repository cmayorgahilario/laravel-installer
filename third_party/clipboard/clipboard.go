// Package clipboard is a no-op stand-in for github.com/atotto/clipboard, wired
// into this project via a `replace` directive in the root go.mod.
//
// Why: the upstream package's init() probes the PATH with exec.LookPath for
// xclip/xsel/wl-copy/etc. Under WSL the PATH includes dozens of Windows
// directories (/mnt/c/...) whose lookups are slow, so that probe adds ~0.4-0.6s
// to EVERY startup — before main() even runs, affecting even `--version`. This
// tool only collects a few short wizard inputs (app name, folder, URL), which
// don't need clipboard paste, so we trade it away for an instant startup. There
// is intentionally NO init() here.
//
// The exported surface mirrors what huh/bubbles use: ReadAll, WriteAll and
// Unsupported.
package clipboard

// Unsupported reports that no system clipboard is available. It is always true
// in this build so callers that check it skip clipboard operations entirely.
var Unsupported = true

// ReadAll returns an empty clipboard. It never errors, so callers that paste
// simply insert nothing.
func ReadAll() (string, error) {
	return "", nil
}

// WriteAll is a no-op; copying to the clipboard silently succeeds.
func WriteAll(text string) error {
	return nil
}
