package installer

import (
	"strings"
	"testing"
)

func TestBuildScript(t *testing.T) {
	t.Run("includes name, fixed image, location, APP_URL and --with", func(t *testing.T) {
		script := BuildScript(Spec{
			Name:     "store",
			AppName:  "My Store",
			URL:      "store.test",
			Location: "/home/dev/code",
			Services: []string{"mysql", "redis", "rustfs"},
		})

		wants := []string{
			"laravelsail/php84-composer:latest",
			"laravel new store --no-interaction",
			"sail:install --with=mysql,redis,rustfs",
			"./vendor/bin/sail pull mysql redis rustfs",
			"./vendor/bin/sail build",
			"cd store",
			`mkdir -p "/home/dev/code"`,
			`-v "/home/dev/code":/opt`,
			`APP_NAME=\"My Store\"`,
			"APP_URL=http://store.test",
			"cd /home/dev/code/store && ./vendor/bin/sail up",
		}
		for _, w := range wants {
			if !strings.Contains(script, w) {
				t.Errorf("the script does not contain %q\n--- script ---\n%s", w, script)
			}
		}
	})

	t.Run("with a starter kit it installs Node, updates the installer and adds --using", func(t *testing.T) {
		withKit := BuildScript(Spec{Name: "app", AppName: "App", URL: "x", Location: "/c",
			Template: "laravel/react-starter-kit"})
		for _, want := range []string{
			"nodejs.org/dist/v22.14.0",                  // Node via tarball (kit's npm)
			"composer global require laravel/installer", // installer with --using
			"laravel new app --no-interaction --using=laravel/react-starter-kit",
		} {
			if !strings.Contains(withKit, want) {
				t.Errorf("missing %q in the script:\n%s", want, withKit)
			}
		}
	})

	t.Run("without a starter kit it installs neither Node nor updates the installer", func(t *testing.T) {
		noKit := BuildScript(Spec{Name: "app", AppName: "App", URL: "x", Location: "/c"})
		for _, unwanted := range []string{"--using", "composer global require", "nodejs.org"} {
			if strings.Contains(noKit, unwanted) {
				t.Errorf("without a template it should not contain %q:\n%s", unwanted, noKit)
			}
		}
		if !strings.Contains(noKit, "laravel new app --no-interaction &&") {
			t.Errorf("without a template it should use the image's laravel:\n%s", noKit)
		}
	})

	t.Run("without services it uses --with=none and skips sail pull", func(t *testing.T) {
		script := BuildScript(Spec{Name: "api", Services: nil})

		if !strings.Contains(script, "sail:install --with=none") {
			t.Errorf("expected --with=none\n--- script ---\n%s", script)
		}
		if strings.Contains(script, "sail pull") {
			t.Errorf("should not run 'sail pull' without services\n--- script ---\n%s", script)
		}
		if !strings.Contains(script, "./vendor/bin/sail build") {
			t.Errorf("expected 'sail build'\n--- script ---\n%s", script)
		}
	})

	t.Run("the image's PHP version is always fixed", func(t *testing.T) {
		script := BuildScript(Spec{Name: "x", Services: []string{"mysql"}})
		if !strings.Contains(script, composerImage) {
			t.Errorf("expected the fixed image %q", composerImage)
		}
	})
}

func TestHasDatabase(t *testing.T) {
	tests := []struct {
		services []string
		want     bool
	}{
		{[]string{"mysql", "redis"}, true},
		{[]string{"pgsql"}, true},
		{[]string{"mongodb", "mailpit"}, true},
		{[]string{"redis", "mailpit", "rustfs"}, false},
		{nil, false},
	}
	for _, tt := range tests {
		if got := HasDatabase(tt.services); got != tt.want {
			t.Errorf("HasDatabase(%v) = %v, want %v", tt.services, got, tt.want)
		}
	}
}

func TestValidateServices(t *testing.T) {
	tests := []struct {
		name     string
		services []string
		wantErr  bool
	}{
		{"empty list is valid", nil, false},
		{"all catalog services", Services, false},
		{"valid subset", []string{"mysql", "redis", "rustfs"}, false},
		{"new v13 services", []string{"mongodb", "rabbitmq", "rustfs"}, false},
		{"an unknown service", []string{"mysql", "foobar"}, true},
		{"uppercase doesn't match", []string{"MySQL"}, true},
		{"empty service", []string{""}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateServices(tt.services)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateServices(%v) error = %v, wantErr = %v",
					tt.services, err, tt.wantErr)
			}
		})
	}
}
