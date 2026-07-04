package devspace

import (
	"bytes"
	"strings"
	"testing"

	"github.com/charmbracelet/colorprofile"
)

func clearColorEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{"NO_COLOR", "CLICOLOR", "CLICOLOR_FORCE", "TTY_FORCE"} {
		t.Setenv(key, "")
	}
}

// renderThemed always returns the theme's full-color rendering (the theme
// itself is never plain; stripping happens in styledWriter).
func renderThemed() string {
	return currentTheme.OK.Render("ok")
}

func TestStyledWriterStripsAnsiForNonTTYByDefault(t *testing.T) {
	clearColorEnv(t)
	var buf bytes.Buffer
	configureStyles(&buf, false)

	var out bytes.Buffer
	if _, err := styledWriter(&out).Write([]byte(renderThemed())); err != nil {
		t.Fatalf("write: %v", err)
	}
	if strings.ContainsRune(out.String(), 0x1b) {
		t.Fatalf("expected no ANSI escape bytes for a non-terminal writer with no forcing env vars, got %q", out.String())
	}
	if out.String() != "ok" {
		t.Fatalf("expected stripped output to equal the plain label, got %q", out.String())
	}
}

func TestStyledWriterPreservesAnsiWhenCliColorForced(t *testing.T) {
	clearColorEnv(t)
	t.Setenv("CLICOLOR_FORCE", "1")
	var buf bytes.Buffer
	configureStyles(&buf, false)

	var out bytes.Buffer
	if _, err := styledWriter(&out).Write([]byte(renderThemed())); err != nil {
		t.Fatalf("write: %v", err)
	}
	if !strings.ContainsRune(out.String(), 0x1b) {
		t.Fatalf("expected ANSI escape bytes when CLICOLOR_FORCE=1 is set, got %q", out.String())
	}
}

func TestStyledWriterNoColorFlagOverridesForcing(t *testing.T) {
	clearColorEnv(t)
	t.Setenv("CLICOLOR_FORCE", "1")
	var buf bytes.Buffer
	configureStyles(&buf, true) // --no-color

	var out bytes.Buffer
	if _, err := styledWriter(&out).Write([]byte(renderThemed())); err != nil {
		t.Fatalf("write: %v", err)
	}
	if strings.ContainsRune(out.String(), 0x1b) {
		t.Fatalf("expected --no-color to force plain output even when CLICOLOR_FORCE=1 is set, got %q", out.String())
	}
	if out.String() != "ok" {
		t.Fatalf("expected stripped output to equal the plain label, got %q", out.String())
	}
}

func TestStyledWriterHonorsNoColorEnvOnRealTTYProfile(t *testing.T) {
	clearColorEnv(t)
	// Simulate an application that has already detected a colored profile
	// (as if running on a real TTY) and confirm NO_COLOR-driven ASCII
	// downgrade still strips color (decorations like bold may remain per the
	// NO_COLOR spec, but no color escape should survive).
	currentProfile = colorprofile.ASCII
	defer func() { currentProfile = colorprofile.NoTTY }()

	var out bytes.Buffer
	if _, err := styledWriter(&out).Write([]byte(renderThemed())); err != nil {
		t.Fatalf("write: %v", err)
	}
	if strings.Contains(out.String(), "38;2;") {
		t.Fatalf("expected no truecolor SGR sequence at ASCII profile, got %q", out.String())
	}
}
