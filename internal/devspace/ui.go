package devspace

import (
	"errors"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/spf13/cobra"
)

func newUICommand() *cobra.Command {
	var noWatch bool
	cmd := &cobra.Command{
		Use:   "ui",
		Short: "Open the interactive workspace dashboard",
		Long: strings.Join([]string{
			"Open a full-screen dashboard showing tracked projects, workspace scan counts, and recent filesystem refreshes.",
			"The dashboard exposes only safe actions: scan, plan, apply-safe, and hydrate the selected project.",
		}, "\n\n"),
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			if !isTerminalWriter(out) {
				return errors.New("devspace ui requires an interactive terminal")
			}
			model := newDashboardModel(noWatch)
			program := tea.NewProgram(model, tea.WithOutput(out))
			_, err := program.Run()
			return err
		},
	}
	cmd.Flags().BoolVar(&noWatch, "no-watch", false, "disable the live filesystem watcher; use the r key to refresh manually")
	return cmd
}
