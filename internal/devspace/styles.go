package devspace

// styles.go defines the shared Liatrio-brand terminal theme and the
// color-capability plumbing used by the presentation helpers in output.go.
//
// The theme always renders full-color styles; plain output is produced by
// wrapping every writer with styledWriter, which downsamples or strips ANSI
// according to the detected colorprofile. This keeps helpers simple (always
// call theme.OK.Render(...)) while guaranteeing piped/NO_COLOR/--no-color
// output never contains escape sequences, since the profile for those cases
// is colorprofile.NoTTY and the writer strips unconditionally.
import (
	"image/color"
	"io"
	"os"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"
	"golang.org/x/term"
)

// Liatrio brand palette, verified from liatrio.ai stylesheets and logo
// assets. Severity colors are adaptive light/dark pairs so they read on both
// terminal themes.
const (
	liatrioGreenOnDark  = "#89DF00" // primary brand green (dark backgrounds)
	liatrioGreenOnLight = "#24AE1D" // primary brand green (light backgrounds)
	liatrioRed          = "#EF4343" // brand --destructive token
	amberOnDark         = "#FBBF24" // warn; Liatrio defines no warning token
	amberOnLight        = "#D97706"
	slateOnDark         = "#9CA3AF" // muted/info neutrals from the brand slates
	slateOnLight        = "#6B7280"
)

// theme holds the named lipgloss styles used by every output helper.
type theme struct {
	OK     lipgloss.Style
	Warn   lipgloss.Style
	Fail   lipgloss.Style
	Info   lipgloss.Style
	Header lipgloss.Style
	Muted  lipgloss.Style
}

// currentTheme and currentProfile are resolved once per CLI invocation by
// configureStyles (called from the root PersistentPreRunE). They default to
// NoTTY so helpers invoked directly from tests render stable plain text.
var (
	currentTheme   = colorTheme(true)
	currentProfile = colorprofile.NoTTY
)

func colorTheme(darkBackground bool) theme {
	pick := func(onLight, onDark string) color.Color {
		if darkBackground {
			return lipgloss.Color(onDark)
		}
		return lipgloss.Color(onLight)
	}
	accent := pick(liatrioGreenOnLight, liatrioGreenOnDark)
	return theme{
		OK:     lipgloss.NewStyle().Foreground(accent),
		Warn:   lipgloss.NewStyle().Foreground(pick(amberOnLight, amberOnDark)),
		Fail:   lipgloss.NewStyle().Foreground(lipgloss.Color(liatrioRed)),
		Info:   lipgloss.NewStyle().Foreground(pick(slateOnLight, slateOnDark)),
		Header: lipgloss.NewStyle().Bold(true).Foreground(accent),
		Muted:  lipgloss.NewStyle().Foreground(pick(slateOnLight, slateOnDark)),
	}
}

// configureStyles resolves terminal color capability for this invocation and
// installs the process-wide theme and profile. NO_COLOR, CLICOLOR, and
// CLICOLOR_FORCE are honored via colorprofile detection; noColor (the root
// --no-color flag) forces the NoTTY profile regardless of detected capability.
func configureStyles(out io.Writer, noColor bool) {
	profile := colorprofile.Detect(out, os.Environ())
	if noColor {
		profile = colorprofile.NoTTY
	}
	currentProfile = profile
	currentTheme = colorTheme(hasDarkBackground(out))
}

// hasDarkBackground queries the terminal background only when both stdin and
// the output are real TTYs (the query needs a round-trip); otherwise it
// assumes dark, the common developer terminal default.
func hasDarkBackground(out io.Writer) bool {
	f, ok := out.(*os.File)
	if !ok || !term.IsTerminal(int(f.Fd())) || !term.IsTerminal(int(os.Stdin.Fd())) {
		return true
	}
	return lipgloss.HasDarkBackground(os.Stdin, f)
}

// styledWriter wraps w with the detected colorprofile so themed output is
// automatically downsampled (TrueColor -> ANSI256 -> ANSI) or, for NoTTY
// profiles (piped output, NO_COLOR, --no-color), stripped entirely.
func styledWriter(w io.Writer) io.Writer {
	return &colorprofile.Writer{Forward: w, Profile: currentProfile}
}
