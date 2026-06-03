package ui

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInstallPath(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no HOME available")
	}
	want := filepath.Join(home, "code", "my-app")
	if got := InstallPath("my-app"); got != want {
		t.Errorf("InstallPath(\"my-app\") = %q, want %q", got, want)
	}
}

func TestSlugify(t *testing.T) {
	tests := map[string]string{
		"Laravel":    "laravel",
		"My Store":   "my-store",
		"  Foo  Bar": "foo-bar",
		"App_2024":   "app-2024",
		"---x---":    "x",
		"!!!":        "",
	}
	for in, want := range tests {
		if got := Slugify(in); got != want {
			t.Errorf("Slugify(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestValidateAppName(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{"Laravel", false},
		{"My Store", false},
		{"", true},
		{"!!!", true}, // empty slug
		{`Bad"quote`, true},
		{"with|pipe", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ValidateAppName(tt.name); (err != nil) != tt.wantErr {
				t.Errorf("ValidateAppName(%q) err=%v wantErr=%v", tt.name, err, tt.wantErr)
			}
		})
	}
}

func TestValidateURL(t *testing.T) {
	tests := []struct {
		url     string
		wantErr bool
	}{
		{"localhost", false},
		{"miapp.test", false},
		{"", true},
		{"with space", true},
	}
	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			if err := ValidateURL(tt.url); (err != nil) != tt.wantErr {
				t.Errorf("ValidateURL(%q) error = %v, wantErr = %v",
					tt.url, err, tt.wantErr)
			}
		})
	}
}
