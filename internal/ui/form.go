// Package ui holds the interactive wizard built with huh.
package ui

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/charmbracelet/huh"

	"laravel-installer/internal/config"
)

// Choice is the user's answers after completing the form.
// It embeds config.Config (Database/Cache/... and the Services() method are
// promoted) and adds the per-run fields of the wizard. The project is created in
// ~/code/<Folder>; AppName (the display name) may differ from the folder.
type Choice struct {
	config.Config

	AppName   string // written to .env as APP_NAME (allows spaces)
	Folder    string // folder in ~/code; if empty it's derived from AppName
	URL       string
	Confirmed bool
}

// ResolvedFolder is the effective folder: the typed one, or the slug of AppName.
func (c Choice) ResolvedFolder() string {
	if f := strings.TrimSpace(c.Folder); f != "" {
		return f
	}
	return Slugify(c.AppName)
}

// Catalogs for each single-select group. The "" option means "none".
var (
	databaseOptions = []huh.Option[string]{
		huh.NewOption("MySQL", "mysql"),
		huh.NewOption("PostgreSQL", "pgsql"),
		huh.NewOption("MariaDB", "mariadb"),
		huh.NewOption("MongoDB", "mongodb"),
		huh.NewOption("None", ""),
	}
	cacheOptions = []huh.Option[string]{
		huh.NewOption("Redis", "redis"),
		huh.NewOption("Valkey", "valkey"),
		huh.NewOption("Memcached", "memcached"),
		huh.NewOption("None", ""),
	}
	searchOptions = []huh.Option[string]{
		huh.NewOption("None", ""),
		huh.NewOption("Meilisearch", "meilisearch"),
		huh.NewOption("Typesense", "typesense"),
	}
	storageOptions = []huh.Option[string]{
		huh.NewOption("None", ""),
		huh.NewOption("RustFS (S3)", "rustfs"),
		huh.NewOption("MinIO (S3)", "minio"),
	}
	// addonOptions are independent extras (multiselect). The order matches
	// config.AddonOrder so that --with= is reproducible.
	addonOptions = []huh.Option[string]{
		huh.NewOption("RabbitMQ (queue broker)", "rabbitmq"),
		huh.NewOption("Mailpit (email capture)", "mailpit"),
		huh.NewOption("Selenium (browser tests)", "selenium"),
		huh.NewOption("Soketi (WebSockets)", "soketi"),
	}
	// templateOptions are the laravel new --using starter kits. "" = base.
	templateOptions = []huh.Option[string]{
		huh.NewOption("None (base project)", ""),
		huh.NewOption("React", "laravel/react-starter-kit"),
		huh.NewOption("React (blank)", "laravel/blank-react-starter-kit"),
		huh.NewOption("Livewire", "laravel/livewire-starter-kit"),
		huh.NewOption("Livewire (blank)", "laravel/blank-livewire-starter-kit"),
	}
)

var (
	// nameRe validates the folder: safe for shell and filesystem (no spaces).
	nameRe = regexp.MustCompile(`^[a-zA-Z0-9_.-]+$`)
	// appNameRe validates APP_NAME: allows spaces; excludes quotes, |, &, \, $
	// so it's safe to interpolate into the .env sed.
	appNameRe = regexp.MustCompile(`^[a-zA-Z0-9 ._-]+$`)
)

// ValidateName validates a folder name (required). Used in the direct-flags
// mode, where the positional argument is the folder.
func ValidateName(s string) error {
	s = strings.TrimSpace(s)
	if s == "" {
		return fmt.Errorf("the name can't be empty")
	}
	if !nameRe.MatchString(s) {
		return fmt.Errorf("use only letters, numbers, '.', '-' or '_'")
	}
	return nil
}

// ValidateAppName validates the app's display name (required). It must produce a
// non-empty slug so the folder can be derived from it.
func ValidateAppName(s string) error {
	s = strings.TrimSpace(s)
	if s == "" {
		return fmt.Errorf("the name can't be empty")
	}
	if !appNameRe.MatchString(s) {
		return fmt.Errorf("use letters, numbers, spaces, '.', '-' or '_'")
	}
	if Slugify(s) == "" {
		return fmt.Errorf("the name must have at least one letter or number")
	}
	return nil
}

// ValidateFolderOptional validates the wizard folder: empty is valid (it's
// derived from the name); if typed, it must be a safe slug.
func ValidateFolderOptional(s string) error {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return ValidateName(s)
}

// Run shows the wizard and returns the user's selection. The defaults come from
// the last saved config (or from config.Default()).
func Run() (Choice, error) {
	c := Choice{
		Config:  config.Load(),
		AppName: "Laravel",
		URL:     "localhost",
	}
	// useDefaults: when true, the service groups are skipped and the loaded
	// values (config) are used. Starts at true as a quick shortcut.
	useDefaults := true
	// hideServices hides the service groups when the defaults are used.
	hideServices := func() bool { return useDefaults }
	// defaultsSummary describes the loaded services (the "defaults").
	defaultsSummary := summary(c.Services())

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Application name").
				Description("Used as APP_NAME (may contain spaces)").
				Value(&c.AppName).
				Validate(ValidateAppName),

			huh.NewInput().
				Title("Folder").
				Description("Installed under ~/code/<folder>").
				PlaceholderFunc(func() string { return Slugify(c.AppName) }, &c.AppName).
				Value(&c.Folder).
				Validate(func(s string) error {
					if err := ValidateFolderOptional(s); err != nil {
						return err
					}
					folder := strings.TrimSpace(s)
					if folder == "" {
						folder = Slugify(c.AppName)
					}
					if _, err := os.Stat(InstallPath(folder)); err == nil {
						return fmt.Errorf("~/code/%s already exists; choose another name", folder)
					}
					return nil
				}),

			huh.NewInput().
				Title("Project URL").
				Description("Used as APP_URL (http://...)").
				Value(&c.URL).
				Validate(ValidateURL),
		),

		// Starter kit: goes before the defaults shortcut so it's ALWAYS asked
		// (a select alone in its group keeps huh from compressing it).
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Starter kit").
				Description("Initial project scaffold (laravel new --using)").
				Options(templateOptions...).
				Value(&c.Template),
		),

		huh.NewGroup(
			huh.NewConfirm().
				Title(fmt.Sprintf("Install with the default services? (%s)", defaultsSummary)).
				Description("No = choose services manually").
				Affirmative("Yes, use defaults").
				Negative("No, choose").
				Value(&useDefaults),
		),

		// One select per group keeps huh from compressing the list when several
		// selects don't fit the terminal height (it would show only one option).
		// All of them are hidden if the user chose to use the defaults.
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Database").
				Options(databaseOptions...).
				Value(&c.Database),
		).WithHideFunc(hideServices),

		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Cache / in-memory store").
				Options(cacheOptions...).
				Value(&c.Cache),
		).WithHideFunc(hideServices),

		// These two are short (3 options) and fit together without compressing.
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Search engine").
				Options(searchOptions...).
				Value(&c.Search),

			huh.NewSelect[string]().
				Title("Object storage (S3)").
				Options(storageOptions...).
				Value(&c.Storage),
		).WithHideFunc(hideServices),

		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("Extras").
				Description("Space to toggle, Enter to continue").
				Options(addonOptions...).
				Value(&c.Addons),
		).WithHideFunc(hideServices),

		huh.NewGroup(
			huh.NewConfirm().
				TitleFunc(func() string {
					return fmt.Sprintf("Create %q in %s with: %s?",
						c.AppName, InstallPath(c.ResolvedFolder()), summary(c.Services()))
				}, &c).
				Affirmative("Yes, create").
				Negative("Cancel").
				Value(&c.Confirmed),
		),
	)

	if err := form.Run(); err != nil {
		return Choice{}, err
	}

	// Remember the selection for the next run (best-effort).
	if c.Confirmed {
		_ = config.Save(c.Config)
	}
	return c, nil
}

func summary(services []string) string {
	if len(services) == 0 {
		return "(no services)"
	}
	return strings.Join(services, ", ")
}

// ConfirmLaunch asks whether to bring the project up with sail up. Returns false
// if the user cancels the prompt.
func ConfirmLaunch() bool {
	var yes bool
	err := huh.NewConfirm().
		Title("Bring the project up now with 'sail up'?").
		Affirmative("Yes, start it").
		Negative("No, later").
		Value(&yes).
		Run()
	return err == nil && yes
}
