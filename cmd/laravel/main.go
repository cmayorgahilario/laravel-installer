// Command laravel creates a Laravel + Sail project via Docker through an
// interactive wizard. It replicates laravel.build with more control (full
// service catalog, separate app name and folder).
//
// Usage:
//
//	laravel              # interactive wizard
//	laravel --dry-run    # prints the script without running it
//	laravel --debug      # shows Docker's log instead of the spinner
//	laravel --version    # prints the version and exits
package main

import (
	"flag"
	"fmt"
	"os"

	"laravel-installer/internal/installer"
	"laravel-installer/internal/ui"
)

// Build variables injected by GoReleaser via -ldflags -X. In local builds
// (go build/go run) they keep these default values.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	var (
		dryRun      = flag.Bool("dry-run", false, "print the generated script without running it")
		debug       = flag.Bool("debug", false, "show Docker's log instead of the spinner")
		showVersion = flag.Bool("version", false, "print the version and exit")
	)
	flag.Parse()

	if *showVersion {
		fmt.Printf("laravel %s (commit %s, %s)\n", version, commit, date)
		return
	}

	// Check Docker before the wizard so we don't waste the user's time if it's
	// unavailable. In --dry-run we only print the script, so it's not needed.
	if !*dryRun {
		if err := installer.CheckDocker(); err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			os.Exit(1)
		}
	}

	choice, err := ui.Run()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Cancelled:", err)
		os.Exit(1)
	}
	if !choice.Confirmed {
		fmt.Println("Operation cancelled.")
		return
	}

	spec := installer.Spec{
		Name:     choice.ResolvedFolder(),
		AppName:  choice.AppName,
		URL:      choice.URL,
		Template: choice.Template,
		Location: ui.CodeBase(),
		Services: choice.Services(),
	}

	if *dryRun {
		fmt.Println("\n--- generated script ---")
		fmt.Print(installer.BuildScript(spec))
		return
	}

	projectDir := ui.InstallPath(spec.Name)

	// Safety net: the wizard already validates, but we reconfirm the folder
	// doesn't exist before touching Docker.
	if _, err := os.Stat(projectDir); err == nil {
		fmt.Fprintf(os.Stderr, "Error: %s already exists; choose another name/folder\n", projectDir)
		os.Exit(1)
	}

	if err := installer.Run(spec, *debug); err != nil {
		fmt.Fprintln(os.Stderr, "Error running the installer:", err)
		// A partial folder may have been left behind (incl. if you hit Ctrl+C).
		if _, statErr := os.Stat(projectDir); statErr == nil {
			fmt.Fprintf(os.Stderr, "Warning: a partial install may remain at %s; remove it before retrying.\n", projectDir)
		}
		os.Exit(1)
	}

	printNextSteps(spec, projectDir)

	// Offer to bring the project up (only with an interactive terminal).
	if installer.IsInteractive() && ui.ConfirmLaunch() {
		if err := installer.SailUp(projectDir); err != nil {
			fmt.Fprintln(os.Stderr, "Error bringing sail up:", err)
			os.Exit(1)
		}
		url := "http://" + spec.URL
		fmt.Printf("\n✓ Containers running. Open %s\n", url)
		// Best-effort: open the browser (under WSL, the Windows one via wslview).
		if installer.OpenURL(url) == nil {
			fmt.Println("→ Opening in your browser...")
		}
	}
}

// printNextSteps shows the summary and the useful commands after creating the project.
func printNextSteps(spec installer.Spec, projectDir string) {
	fmt.Printf("\n✓ Project created at %s\n\n", projectDir)
	fmt.Println("Next steps:")
	fmt.Printf("  cd %s\n", projectDir)
	fmt.Println("  ./vendor/bin/sail up -d")
	if installer.HasDatabase(spec.Services) {
		fmt.Println("  ./vendor/bin/sail artisan migrate")
	}
	if spec.Template != "" {
		fmt.Println("  ./vendor/bin/sail npm run dev")
	}
}
