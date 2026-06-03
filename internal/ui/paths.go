package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// CodeBase is the fixed base install directory: ~/code.
// Projects are always created under CodeBase/<folder>.
func CodeBase() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "code"
	}
	return filepath.Join(home, "code")
}

// InstallPath returns the final project path: ~/code/<folder>.
func InstallPath(folder string) string {
	return filepath.Join(CodeBase(), folder)
}

// Slugify turns an app name into a safe folder name: lowercased, runs of
// non-alphanumeric chars → "-". "My Store" → "my-store".
func Slugify(s string) string {
	var b strings.Builder
	prevDash := false
	for _, r := range strings.ToLower(strings.TrimSpace(s)) {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			b.WriteRune(r)
			prevDash = false
		case !prevDash:
			b.WriteRune('-')
			prevDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}

// ValidateURL checks that the URL is neither empty nor contains spaces.
func ValidateURL(s string) error {
	s = strings.TrimSpace(s)
	if s == "" {
		return fmt.Errorf("the URL can't be empty")
	}
	if strings.ContainsAny(s, " \t") {
		return fmt.Errorf("the URL can't contain spaces")
	}
	return nil
}
