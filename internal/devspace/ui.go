package devspace

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/spf13/cobra"
)

const tuiBinaryName = "devspace-tui"

func newUICommand() *cobra.Command {
	var noWatch bool
	var legacy bool
	cmd := &cobra.Command{
		Use:   "ui",
		Short: "Open the interactive workspace dashboard",
		Long: strings.Join([]string{
			"Open a full-screen dashboard showing tracked projects, workspace scan counts, and recent filesystem refreshes.",
			"The dashboard exposes only safe actions: scan, plan, apply-safe, and hydrate the selected project.",
			"When the devspace-tui companion binary is installed (next to devspace, in $DEVSPACE_HOME/bin, or on PATH) it is launched instead of the built-in dashboard; --legacy forces the built-in one.",
		}, "\n\n"),
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			if !isTerminalWriter(out) {
				return errors.New("devspace ui requires an interactive terminal")
			}
			if !legacy {
				if tui := findTUIBinary(); tui != "" {
					return runExternalTUI(tui, noWatch)
				}
				fmt.Fprintln(cmd.ErrOrStderr(), currentTheme.Muted.Render(
					"devspace-tui not found; using the built-in dashboard (run 'devspace tui install' to get the full experience)"))
			}
			model := newDashboardModel(noWatch)
			program := tea.NewProgram(model, tea.WithOutput(out))
			_, err := program.Run()
			return err
		},
	}
	cmd.Flags().BoolVar(&noWatch, "no-watch", false, "disable the live filesystem watcher; use the r key to refresh manually")
	cmd.Flags().BoolVar(&legacy, "legacy", false, "use the built-in dashboard even when devspace-tui is installed")
	return cmd
}

// findTUIBinary locates the devspace-tui companion binary: next to the
// devspace executable first, then $DEVSPACE_HOME/bin, then PATH. The PATH
// lookup is a deliberate last resort with the same trust model any CLI uses
// when resolving a plugin binary via PATH; the adjacent-binary and
// $DEVSPACE_HOME/bin checks take precedence and are both within the user's
// own control.
func findTUIBinary() string {
	if exe, err := os.Executable(); err == nil {
		if candidate := filepath.Join(filepath.Dir(exe), tuiBinaryName); isExecutableFile(candidate) {
			return candidate
		}
	}
	if home, err := appHome(); err == nil {
		if candidate := filepath.Join(home, "bin", tuiBinaryName); isExecutableFile(candidate) {
			return candidate
		}
	}
	if path, err := exec.LookPath(tuiBinaryName); err == nil {
		return path
	}
	return ""
}

func isExecutableFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular() && info.Mode().Perm()&0o111 != 0
}

// runExternalTUI hands the terminal to devspace-tui, telling it which devspace
// binary to spawn as its ui-server backend.
func runExternalTUI(path string, noWatch bool) error {
	var args []string
	if noWatch {
		args = append(args, "--no-watch")
	}
	c := exec.Command(path, args...) //nolint:gosec // path is resolved by findTUIBinary (adjacent binary, $DEVSPACE_HOME/bin, or PATH as a last resort); launching the resolved companion binary is the feature, not attacker-controlled input
	c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stdout, os.Stderr
	c.Env = os.Environ()
	if exe, err := os.Executable(); err == nil {
		c.Env = append(c.Env, "DEVSPACE_BIN="+exe)
	}
	return c.Run()
}
