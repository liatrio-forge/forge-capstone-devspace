package devdrop

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"
)

func NewRootCommand(version string) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "devspace",
		Short:        "Synchronize local developer workspace metadata",
		SilenceUsage: true,
	}
	cmd.AddCommand(newVersionCommand(version))
	cmd.AddCommand(newInitCommand())
	cmd.AddCommand(newScanCommand())
	cmd.AddCommand(newPlanCommand())
	cmd.AddCommand(newApplyCommand())
	cmd.AddCommand(newWorkspaceCommand())
	cmd.AddCommand(newProjectCommand())
	cmd.AddCommand(newEnvCommand())
	cmd.AddCommand(newStatusCommand())
	return cmd
}

func newVersionCommand(version string) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Fprintln(cmd.OutOrStdout(), version)
		},
	}
}

func newInitCommand() *cobra.Command {
	var workspace string
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize a DevDrop workspace",
		RunE: func(cmd *cobra.Command, args []string) error {
			if workspace == "" {
				workspace = "~/code"
			}
			cfg, err := InitWorkspace(workspace)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Initialized DevDrop workspace: %s\n", cfg.WorkspaceRoot)
			fmt.Fprintf(cmd.OutOrStdout(), "Machine: %s (%s)\n", cfg.MachineName, cfg.MachineID)
			return nil
		},
	}
	cmd.Flags().StringVar(&workspace, "workspace", "", "workspace root")
	return cmd
}

func newWorkspaceCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "workspace", Short: "Manage workspace"}
	cmd.AddCommand(newScanCommand())
	cmd.AddCommand(newWorkspaceRemoteCommand())
	cmd.AddCommand(&cobra.Command{
		Use:   "push",
		Short: "Push workspace manifest to the configured Git remote",
		RunE: func(cmd *cobra.Command, args []string) error {
			changed, err := PushWorkspaceManifest()
			if err != nil {
				return err
			}
			if changed {
				fmt.Fprintln(cmd.OutOrStdout(), "Pushed workspace manifest.")
				return nil
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Workspace manifest already up to date.")
			return nil
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "pull",
		Short: "Pull workspace manifest from the configured Git remote",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, err := PullWorkspaceManifest()
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Pulled workspace manifest.")
			fmt.Fprintln(cmd.OutOrStdout(), "Next: devspace plan && devspace apply")
			return nil
		},
	})
	var dryRun bool
	syncCmd := &cobra.Command{
		Use:        "sync",
		Short:      "Compatibility alias for plan/apply",
		Deprecated: "use `devspace plan` and `devspace apply` instead",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRun {
				plan, err := BuildPlan()
				if err != nil {
					return err
				}
				if err := SaveLastPlan(plan); err != nil {
					return err
				}
				printPlan(cmd.OutOrStdout(), plan)
				return nil
			}
			plan, err := ApplyLastPlan()
			if err != nil {
				return err
			}
			printApply(cmd.OutOrStdout(), plan)
			return nil
		},
	}
	syncCmd.Flags().BoolVar(&dryRun, "dry-run", false, "show planned actions without changing files")
	cmd.AddCommand(syncCmd)
	return cmd
}

func newWorkspaceRemoteCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "remote", Short: "Manage workspace manifest remote"}
	cmd.AddCommand(&cobra.Command{
		Use:   "set <url-or-path>",
		Short: "Set workspace manifest Git remote",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := SetManifestRemote(args[0])
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s\n", cfg.ManifestRemote)
			return nil
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "get",
		Short: "Print workspace manifest Git remote",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := GetManifestRemote()
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s\n", cfg.ManifestRemote)
			return nil
		},
	})
	return cmd
}

func newScanCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "scan",
		Short: "Scan workspace projects",
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := ScanWorkspace()
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "Found %d projects\n", s.FoundProjects)
			fmt.Fprintf(out, "%d Git repos\n", s.GitRepos)
			fmt.Fprintf(out, "%d untracked folders\n", s.UntrackedFolders)
			fmt.Fprintf(out, "%d local-only projects\n", s.LocalOnlyProjects)
			fmt.Fprintf(out, "%d projects with env files\n", s.ProjectsWithEnv)
			return nil
		},
	}
}

func newPlanCommand() *cobra.Command {
	var jsonOut bool
	cmd := &cobra.Command{
		Use:   "plan",
		Short: "Plan safe workspace changes",
		RunE: func(cmd *cobra.Command, args []string) error {
			plan, err := BuildPlan()
			if err != nil {
				return err
			}
			if err := SaveLastPlan(plan); err != nil {
				return err
			}
			if jsonOut {
				return writePrettyJSON(cmd.OutOrStdout(), plan)
			}
			printPlan(cmd.OutOrStdout(), plan)
			return nil
		},
	}
	cmd.Flags().BoolVar(&jsonOut, "json", false, "print machine-readable plan")
	return cmd
}

func newApplyCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "apply",
		Short: "Apply the last saved safe plan",
		RunE: func(cmd *cobra.Command, args []string) error {
			plan, err := ApplyLastPlan()
			if err != nil {
				return err
			}
			printApply(cmd.OutOrStdout(), plan)
			return nil
		},
	}
}

func printPlan(out io.Writer, plan Plan) {
	fmt.Fprintln(out, "Planned changes:")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "SAFE:")
	hasSafe := false
	for _, a := range plan.Actions {
		if a.Safety != "safe" {
			continue
		}
		hasSafe = true
		fmt.Fprintf(out, "%s %s\n", strings.ToUpper(a.Kind), a.Path)
	}
	if !hasSafe {
		fmt.Fprintln(out, "(none)")
	}
	fmt.Fprintln(out)
	fmt.Fprintln(out, "SKIPPED:")
	hasSkipped := false
	for _, a := range plan.Actions {
		if a.Safety != "skipped" {
			continue
		}
		hasSkipped = true
		fmt.Fprintf(out, "SKIP %s because %s\n", a.Path, a.Reason)
	}
	if !hasSkipped {
		fmt.Fprintln(out, "(none)")
	}
	fmt.Fprintln(out)
	fmt.Fprintln(out, "WARNINGS:")
	if len(plan.Warnings) == 0 {
		fmt.Fprintln(out, "(none)")
	} else {
		for _, w := range plan.Warnings {
			fmt.Fprintf(out, "- %s\n", w)
		}
	}
	fmt.Fprintln(out)
	fmt.Fprintln(out, "No destructive changes will be performed.")
}

func printApply(out io.Writer, plan Plan) {
	fmt.Fprintln(out, "Applied safe plan actions.")
	printPlan(out, plan)
}

func newProjectCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "project", Short: "Manage projects"}
	cmd.AddCommand(&cobra.Command{
		Use:   "add <relative-path>",
		Short: "Track a project path",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			p, err := AddProject(args[0])
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Added project %s at %s\n", p.Name, p.Path)
			return nil
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "hydrate <project>",
		Short: "Clone a placeholder Git project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			p, err := HydrateProject(args[0])
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Hydrated %s\n", p.Name)
			if p.Setup.InstallCommand != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "Suggested setup: %s\n", p.Setup.InstallCommand)
			}
			return nil
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "status [project]",
		Short: "Show project status",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return printStatus(cmd.OutOrStdout(), args)
		},
	})
	return cmd
}

func newEnvCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "env", Short: "Manage encrypted env profiles"}
	var profile string
	set := &cobra.Command{
		Use:   "set <project> <key>",
		Short: "Set an encrypted env value from stdin or a hidden prompt",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			secret, err := secretInput(cmd.ErrOrStderr(), args[1])
			if err != nil {
				return err
			}
			if err := EnvSet(args[0], args[1], profile, strings.NewReader(secret)); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Stored %s for %s/%s\n", args[1], args[0], profileOrDefault(profile))
			return nil
		},
	}
	set.Flags().StringVar(&profile, "profile", "dev", "env profile")
	cmd.AddCommand(set)

	list := &cobra.Command{
		Use:   "list <project>",
		Short: "List encrypted env keys",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			keys, err := EnvList(args[0], profile)
			if err != nil {
				return err
			}
			for _, key := range keys {
				fmt.Fprintf(cmd.OutOrStdout(), "%s=****\n", key)
			}
			return nil
		},
	}
	list.Flags().StringVar(&profile, "profile", "dev", "env profile")
	cmd.AddCommand(list)

	pull := &cobra.Command{
		Use:   "pull <project>",
		Short: "Generate local .env from encrypted profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := EnvPull(args[0], profile)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Wrote %s\n", path)
			return nil
		},
	}
	pull.Flags().StringVar(&profile, "profile", "dev", "env profile")
	cmd.AddCommand(pull)
	return cmd
}

func profileOrDefault(profile string) string {
	if profile == "" {
		return "dev"
	}
	return profile
}

func secretInput(errOut io.Writer, key string) (string, error) {
	stat, err := os.Stdin.Stat()
	if err != nil {
		return "", err
	}
	if stat.Mode()&os.ModeCharDevice == 0 {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return "", err
		}
		return string(data), nil
	}
	fmt.Fprintf(errOut, "Value for %s: ", key)
	data, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(errOut)
	if err != nil {
		return "", err
	}
	return string(bytes.TrimRight(data, "\r\n")), nil
}

func newStatusCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show workspace health",
		RunE: func(cmd *cobra.Command, args []string) error {
			return printStatus(cmd.OutOrStdout(), nil)
		},
	}
}

func printStatus(out io.Writer, args []string) error {
	cfg, err := LoadConfig()
	if err != nil {
		return err
	}
	m, err := LoadManifest(cfg.WorkspaceRoot)
	if err != nil {
		return err
	}
	st, err := LoadState()
	if err != nil && !missing(err) {
		return err
	}
	if st.Projects == nil {
		st.Projects = map[string]ProjectState{}
	}
	if len(args) == 1 {
		p, ok := findProject(m, args[0])
		if !ok {
			return fmt.Errorf("project %q not found", args[0])
		}
		ps := st.Projects[p.ID]
		fmt.Fprintf(out, "Project: %s\nPath: %s\nHydrated: %t\nDirty: %t\nMissing env: %t\n", p.Name, p.Path, ps.Hydrated, ps.Dirty, !ps.EnvFilePresent)
		return nil
	}
	var hydrated, placeholders, dirty, missingEnv, stale int
	for _, p := range m.Projects {
		ps := st.Projects[p.ID]
		if ps.Hydrated {
			hydrated++
		}
		if ps.Placeholder {
			placeholders++
		}
		if ps.Dirty {
			dirty++
		}
		if !ps.EnvFilePresent {
			missingEnv++
		}
		if ps.Stale || ps.Missing {
			stale++
		}
	}
	fmt.Fprintf(out, "Machine: %s\n", cfg.MachineName)
	fmt.Fprintf(out, "Workspace: %s\n\n", cfg.WorkspaceRoot)
	fmt.Fprintf(out, "Projects tracked: %d\n", len(m.Projects))
	fmt.Fprintf(out, "Hydrated: %d\n", hydrated)
	fmt.Fprintf(out, "Placeholders: %d\n", placeholders)
	fmt.Fprintf(out, "Dirty repos: %d\n", dirty)
	fmt.Fprintf(out, "Missing env files: %d\n", missingEnv)
	fmt.Fprintf(out, "Outdated repos: %d\n", stale)
	if st.LastSyncAt != "" {
		fmt.Fprintf(out, "Last sync: %s\n", st.LastSyncAt)
	}
	if st.LastScanAt != "" {
		fmt.Fprintf(out, "Last scan: %s\n", st.LastScanAt)
	}
	return nil
}

func sortedProjectNames(m Manifest) string {
	names := make([]string, 0, len(m.Projects))
	for _, p := range m.Projects {
		names = append(names, p.Name)
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}
