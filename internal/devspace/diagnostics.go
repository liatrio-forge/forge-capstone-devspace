package devspace

// diagnostics.go provides the shared charm.land/log/v2 logger used by
// long-running surfaces (watch, experimental hosted serve, experimental mount) to report leveled,
// timestamped diagnostics to stderr. The logger always renders through its
// own lipgloss-based styles; newDiagnosticsLogger decides, independently of
// stdout's profile, whether stderr gets the human-styled TextFormatter or
// the machine-parsable LogfmtFormatter based on stderr's own color profile.

import (
	"io"

	"charm.land/log/v2"
	"github.com/charmbracelet/colorprofile"
)

// newDiagnosticsLogger builds a leveled logger writing to w (typically
// cmd.ErrOrStderr()). When w is not a color-capable TTY (piped, redirected,
// NO_COLOR, or --no-color), diagnostics are emitted as logfmt instead of
// styled text, so log aggregation and grep-based tooling see stable
// key=value output rather than ANSI-decorated lines.
func newDiagnosticsLogger(w io.Writer) *log.Logger {
	profile := detectProfile(w)
	logger := log.NewWithOptions(&colorprofile.Writer{Forward: w, Profile: profile}, log.Options{
		ReportTimestamp: true,
	})
	if profile <= colorprofile.NoTTY {
		logger.SetFormatter(log.LogfmtFormatter)
	}
	return logger
}
