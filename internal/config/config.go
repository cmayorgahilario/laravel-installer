// Package config persists the last wizard selection so it can be reused as the
// defaults on the next run.
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Config holds the service preferences (not the project name, which differs on
// every run).
type Config struct {
	Template string `json:"template,omitempty"` // starter kit (laravel new --using); "" = base

	Database string   `json:"database"`
	Cache    string   `json:"cache"`
	Search   string   `json:"search"`
	Storage  string   `json:"storage"`
	Addons   []string `json:"addons"` // independent extras: rabbitmq, mailpit, selenium, soketi
}

// AddonOrder fixes the canonical order of the add-ons in --with=.
var AddonOrder = []string{"rabbitmq", "mailpit", "selenium", "soketi"}

// Default is the initial config when nothing has been saved yet.
func Default() Config {
	return Config{
		Database: "pgsql",
		Cache:    "valkey",
		Addons:   []string{"mailpit"},
	}
}

// Services flattens the selection into the stable order that sail:install
// --with= expects.
func (c Config) Services() []string {
	var s []string
	for _, v := range []string{c.Database, c.Cache, c.Search, c.Storage} {
		if v != "" {
			s = append(s, v)
		}
	}
	for _, addon := range AddonOrder {
		if contains(c.Addons, addon) {
			s = append(s, addon)
		}
	}
	return s
}

func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

// path returns the config file path (~/.config/laravel-installer/config.json).
func path() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "laravel-installer", "config.json"), nil
}

// Load returns the saved config, or Default() if it doesn't exist or can't be
// read. Fields missing from the JSON keep their Default() value.
func Load() Config {
	cfg := Default()

	p, err := path()
	if err != nil {
		return cfg
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return cfg
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Default()
	}
	return cfg
}

// Save persists the config. It's best-effort: the caller may ignore the error.
func Save(c Config) error {
	p, err := path()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0o644)
}
