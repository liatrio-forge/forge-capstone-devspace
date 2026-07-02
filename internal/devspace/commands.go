package devspace

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/term"
)

func NewRootCommand(version string) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "devspace",
		Short:        "Synchronize local developer workspace metadata",
		SilenceUsage: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if err := migrateLegacyHome(); err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "devspace: legacy home migration failed: %v\n", err)
			}
			return nil
		},
	}
	cmd.AddCommand(newVersionCommand(version))
	cmd.AddCommand(newInitCommand())
	cmd.AddCommand(newScanCommand())
	cmd.AddCommand(newWatchCommand())
	cmd.AddCommand(newPlanCommand())
	cmd.AddCommand(newApplyCommand())
	cmd.AddCommand(newWorkspaceCommand())
	cmd.AddCommand(newHostedCommand())
	cmd.AddCommand(newProjectCommand())
	cmd.AddCommand(newEnvCommand())
	cmd.AddCommand(newStatusCommand())
	cmd.AddCommand(newDoctorCommand())
	cmd.AddCommand(newSetupCommand())
	cmd.AddCommand(newMountCommand())
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
		Short: "Initialize a DevSpace workspace",
		RunE: func(cmd *cobra.Command, args []string) error {
			if workspace == "" {
				workspace = "~/code"
			}
			cfg, err := InitWorkspace(workspace)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Initialized DevSpace workspace: %s\n", cfg.WorkspaceRoot)
			fmt.Fprintf(cmd.OutOrStdout(), "Machine: %s (%s)\n", cfg.MachineName, cfg.MachineID)
			return nil
		},
	}
	cmd.Flags().StringVar(&workspace, "workspace", "", "workspace root")
	return cmd
}

func newWatchCommand() *cobra.Command {
	var debounce time.Duration
	var syncMode string
	var noInitial bool
	var once bool
	cmd := &cobra.Command{
		Use:   "watch",
		Short: "Watch the workspace and refresh project metadata",
		Long: strings.Join([]string{
			"Watch the configured workspace for project additions, removals, and manifest-relevant file changes.",
			"By default watch mode refreshes only local manifest/state metadata.",
			"Use --sync git or --sync hosted to explicitly push the refreshed manifest; watch mode never pulls, applies plans, hydrates repositories, installs dependencies, or uploads secrets.",
		}, "\n\n"),
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			err := WatchWorkspace(ctx, WatchOptions{
				Debounce:   debounce,
				SyncMode:   syncMode,
				RunInitial: !noInitial,
				Once:       once,
			}, cmd.OutOrStdout())
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return nil
			}
			return err
		},
	}
	cmd.Flags().DurationVar(&debounce, "debounce", 2*time.Second, "delay after filesystem events before refreshing metadata")
	cmd.Flags().StringVar(&syncMode, "sync", WatchSyncOff, "post-refresh manifest sync mode: off, git, or hosted")
	cmd.Flags().BoolVar(&noInitial, "no-initial", false, "wait for filesystem events before the first refresh")
	cmd.Flags().BoolVar(&once, "once", false, "perform one refresh and exit")
	return cmd
}

func newHostedCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "hosted", Short: "Manage opt-in hosted manifest sync"}
	cmd.AddCommand(newHostedConfigCommand())
	cmd.AddCommand(&cobra.Command{
		Use:   "push",
		Short: "Push workspace manifest to hosted sync",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := PushHostedManifest()
			if err != nil {
				return err
			}
			if result.Changed {
				fmt.Fprintf(cmd.OutOrStdout(), "Pushed hosted manifest version %d.\n", result.Version)
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Hosted manifest already up to date at version %d.\n", result.Version)
			return nil
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "pull",
		Short: "Pull workspace manifest from hosted sync",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := PullHostedManifest()
			if err != nil {
				return err
			}
			if result.Changed {
				fmt.Fprintf(cmd.OutOrStdout(), "Pulled hosted manifest version %d.\n", result.Version)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "Local manifest already matches hosted version %d.\n", result.Version)
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Next: devspace plan && devspace apply")
			return nil
		},
	})
	cmd.AddCommand(newHostedServeCommand())
	return cmd
}

func newHostedConfigCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "config", Short: "Configure hosted manifest sync"}
	var token string
	var workspace string
	set := &cobra.Command{
		Use:   "set <endpoint>",
		Short: "Set hosted sync endpoint and auth token",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := SetHostedSync(args[0], token, workspace)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Endpoint: %s\n", cfg.HostedSyncEndpoint)
			fmt.Fprintf(cmd.OutOrStdout(), "Workspace: %s\n", cfg.HostedSyncWorkspace)
			return nil
		},
	}
	set.Flags().StringVar(&token, "token", "", "hosted sync bearer token")
	set.Flags().StringVar(&workspace, "workspace", "default", "hosted workspace id")
	cmd.AddCommand(set)
	cmd.AddCommand(&cobra.Command{
		Use:   "get",
		Short: "Print hosted sync configuration without the token",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := GetHostedSync()
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Endpoint: %s\n", cfg.HostedSyncEndpoint)
			fmt.Fprintf(cmd.OutOrStdout(), "Workspace: %s\n", cfg.HostedSyncWorkspace)
			fmt.Fprintln(cmd.OutOrStdout(), "Token: configured")
			return nil
		},
	})
	return cmd
}

func newHostedServeCommand() *cobra.Command {
	var addr string
	var store string
	var token string
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Run a local hosted manifest sync prototype server",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if store == "" {
				home, err := appHome()
				if err != nil {
					return err
				}
				store = filepath.Join(home, "hosted-control-plane")
			}
			handler, err := NewHostedSyncServer(HostedSyncServerOptions{StoreDir: store, Token: token})
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Hosted manifest sync listening on http://%s\n", addr)
			fmt.Fprintf(cmd.OutOrStdout(), "Storage: %s\n", store)
			fmt.Fprintln(cmd.OutOrStdout(), "API: GET/PUT /v1/workspaces/{workspace}/manifest")
			server := &http.Server{Addr: addr, Handler: handler}
			return server.ListenAndServe()
		},
	}
	cmd.Flags().StringVar(&addr, "addr", "127.0.0.1:8787", "listen address")
	cmd.Flags().StringVar(&store, "store", "", "directory for hosted manifest storage")
	cmd.Flags().StringVar(&token, "token", "", "required bearer token")
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
	cmd.AddCommand(&cobra.Command{
		Use:   "diff",
		Short: "Preview differences from the configured workspace manifest remote",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			diff, err := DiffWorkspaceManifest()
			if err != nil {
				return err
			}
			printManifestDiff(cmd.OutOrStdout(), diff)
			return nil
		},
	})
	var dryRun bool
	syncCmd := &cobra.Command{
		Use:        "sync",
		Short:      "Compatibility alias for plan/apply",
		Deprecated: "use `devspace plan` and `devspace apply` instead",
		RunE: func(cmd *cobra.Command, args []string) error {
			plan, err := BuildPlan()
			if err != nil {
				return err
			}
			if err := SaveLastPlan(plan); err != nil {
				return err
			}
			if dryRun {
				printPlan(cmd.OutOrStdout(), plan)
				return nil
			}
			applied, err := ApplyLastPlan()
			if err != nil {
				return err
			}
			printApply(cmd.OutOrStdout(), applied)
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
	create := &cobra.Command{Use: "create", Short: "Create and set a workspace manifest remote"}
	create.AddCommand(&cobra.Command{
		Use:   "local <path>",
		Short: "Create a local bare Git manifest remote",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := CreateLocalManifestRemote(args[0])
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s\n", cfg.ManifestRemote)
			return nil
		},
	})
	private := true
	var public bool
	github := &cobra.Command{
		Use:   "github <owner/repo>",
		Short: "Create a GitHub manifest remote with gh",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if public {
				private = false
			}
			cfg, err := CreateGitHubManifestRemote(args[0], private)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s\n", cfg.ManifestRemote)
			return nil
		},
	}
	github.Flags().BoolVar(&private, "private", true, "create a private GitHub repo")
	github.Flags().BoolVar(&public, "public", false, "create a public GitHub repo")
	create.AddCommand(github)
	cmd.AddCommand(create)
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

func newMountCommand() *cobra.Command {
	var preview bool
	var hydrateOnLookup bool
	var debug bool
	cmd := &cobra.Command{
		Use:   "mount <mountpoint>",
		Short: "Mount a prototype lazy workspace view",
		Long: strings.Join([]string{
			"Mount a read-only FUSE-backed prototype view of tracked workspace projects.",
			"Tracked manifest paths appear as directories before they are hydrated.",
			"Looking up an on-demand Git project through the mount runs the same safe hydration checks as `devspace project hydrate`.",
			"Use --preview to inspect the projected mount entries without requiring FUSE.",
		}, "\n\n"),
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if preview {
				entries, err := BuildMountEntries()
				if err != nil {
					return err
				}
				PrintMountPreview(cmd.OutOrStdout(), entries)
				return nil
			}
			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			return MountWorkspace(ctx, args[0], WorkspaceMountOptions{
				HydrateOnLookup: hydrateOnLookup,
				Debug:           debug,
				ErrOut:          cmd.ErrOrStderr(),
			}, cmd.OutOrStdout())
		},
	}
	cmd.Flags().BoolVar(&preview, "preview", false, "print manifest-backed mount entries without mounting FUSE")
	cmd.Flags().BoolVar(&hydrateOnLookup, "hydrate-on-lookup", true, "hydrate on-demand Git projects when their mount entry is accessed")
	cmd.Flags().BoolVar(&debug, "debug", false, "enable go-fuse debug logging")
	return cmd
}

func printManifestDiff(out io.Writer, diff ManifestDiff) {
	fmt.Fprintln(out, "Workspace manifest diff:")
	fmt.Fprintf(out, "Added: %d\n", len(diff.Added))
	for _, p := range diff.Added {
		fmt.Fprintf(out, "  + %s (%s)\n", p.Path, p.Name)
	}
	fmt.Fprintf(out, "Removed: %d\n", len(diff.Removed))
	for _, p := range diff.Removed {
		fmt.Fprintf(out, "  - %s (%s)\n", p.Path, p.Name)
	}
	fmt.Fprintf(out, "Changed: %d\n", len(diff.Changed))
	for _, changed := range diff.Changed {
		fmt.Fprintf(out, "  ~ %s (%s)\n", changed.Remote.Path, changed.Remote.Name)
		for _, field := range changed.Changes {
			fmt.Fprintf(out, "    %s: %q -> %q\n", field.Field, field.Local, field.Remote)
		}
	}
	if len(diff.Added) == 0 && len(diff.Removed) == 0 && len(diff.Changed) == 0 {
		fmt.Fprintln(out, "No remote manifest differences.")
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
	cmd.AddCommand(newEnvRecipientCommand())
	return cmd
}

func newEnvRecipientCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "recipient", Short: "Manage encrypted env profile recipients"}
	cmd.AddCommand(&cobra.Command{
		Use:   "export",
		Short: "Print this machine's public age recipient",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			recipient, err := EnvRecipientExport()
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Name: %s\n", recipient.Name)
			fmt.Fprintf(cmd.OutOrStdout(), "ID: %s\n", recipient.ID)
			fmt.Fprintf(cmd.OutOrStdout(), "Age recipient: %s\n", recipient.AgeRecipient)
			return nil
		},
	})
	var listProfile string
	list := &cobra.Command{
		Use:   "list <project>",
		Short: "List recipients for an encrypted env profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			recipients, err := EnvRecipients(args[0], listProfile)
			if err != nil {
				return err
			}
			for _, recipient := range recipients {
				status := "active"
				if recipient.RevokedAt != "" {
					status = "revoked"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\n", recipient.ID, recipient.Name, status)
			}
			return nil
		},
	}
	list.Flags().StringVar(&listProfile, "profile", "dev", "env profile")
	cmd.AddCommand(list)

	var inviteProfile string
	var inviteTeam string
	invite := &cobra.Command{
		Use:   "invite <project> <name> <age-recipient>",
		Short: "Encrypt an env profile for another recipient",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			recipient, err := EnvInvite(args[0], inviteProfile, args[1], args[2], inviteTeam)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Invited %s (%s) to %s/%s.\n", recipient.Name, recipient.ID, args[0], profileOrDefault(inviteProfile))
			return nil
		},
	}
	invite.Flags().StringVar(&inviteProfile, "profile", "dev", "env profile")
	invite.Flags().StringVar(&inviteTeam, "team", "", "team name for manifest access metadata")
	cmd.AddCommand(invite)

	var revokeProfile string
	var revokeReason string
	revoke := &cobra.Command{
		Use:   "revoke <project> <recipient-id-or-name>",
		Short: "Remove a recipient from future encrypted profile writes",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			recipient, err := EnvRevoke(args[0], revokeProfile, args[1], revokeReason)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Revoked %s (%s) from %s/%s.\n", recipient.Name, recipient.ID, args[0], profileOrDefault(revokeProfile))
			fmt.Fprintln(cmd.OutOrStdout(), "Already copied ciphertext, pulled .env files, and previously decrypted values cannot be clawed back.")
			return nil
		},
	}
	revoke.Flags().StringVar(&revokeProfile, "profile", "dev", "env profile")
	revoke.Flags().StringVar(&revokeReason, "reason", "", "revocation note")
	cmd.AddCommand(revoke)

	var rotateProfile string
	rotate := &cobra.Command{
		Use:   "rotate <project>",
		Short: "Rewrap an env profile for current active recipients",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := EnvRotateRecipients(args[0], rotateProfile); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Rewrapped %s/%s for current active recipients.\n", args[0], profileOrDefault(rotateProfile))
			return nil
		},
	}
	rotate.Flags().StringVar(&rotateProfile, "profile", "dev", "env profile")
	cmd.AddCommand(rotate)
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

func newDoctorCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Diagnose local DevSpace readiness",
		RunE: func(cmd *cobra.Command, args []string) error {
			return RunDoctor(cmd.OutOrStdout())
		},
	}
}

func newSetupCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "setup", Short: "Review and run project setup commands"}
	cmd.AddCommand(newSetupPlanCommand())
	cmd.AddCommand(newSetupRunCommand())
	cmd.AddCommand(newSetupApplyCommand())
	return cmd
}

func newSetupPlanCommand() *cobra.Command {
	var jsonOut bool
	cmd := &cobra.Command{
		Use:   "plan",
		Short: "Show detected setup commands without running them",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			plan, err := BuildSetupPlan()
			if err != nil {
				return err
			}
			if jsonOut {
				return writePrettyJSON(cmd.OutOrStdout(), plan)
			}
			printSetupPlan(cmd.OutOrStdout(), plan)
			return nil
		},
	}
	cmd.Flags().BoolVar(&jsonOut, "json", false, "print machine-readable setup plan")
	return cmd
}

func newSetupRunCommand() *cobra.Command {
	var kind string
	var yes bool
	var dryRun bool
	var allowUnknown bool
	var allowGlobal bool
	cmd := &cobra.Command{
		Use:   "run <project>",
		Short: "Run a reviewed setup command for one project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := RunProjectSetup(args[0], SetupRunOptions{
				CommandKind:  kind,
				DryRun:       true,
				AllowUnknown: allowUnknown,
				AllowGlobal:  allowGlobal,
				Stdout:       cmd.OutOrStdout(),
				Stderr:       cmd.ErrOrStderr(),
			})
			if err != nil {
				return err
			}
			if !dryRun && !yes {
				if err := confirmSetup(cmd.InOrStdin(), cmd.ErrOrStderr(), fmt.Sprintf("Type %s to run `%s` in %s: ", result.Project, result.Command, result.Path), result.Project); err != nil {
					return err
				}
			}
			result, err = RunProjectSetup(args[0], SetupRunOptions{
				CommandKind:  kind,
				DryRun:       dryRun,
				AllowUnknown: allowUnknown,
				AllowGlobal:  allowGlobal,
				Stdout:       cmd.OutOrStdout(),
				Stderr:       cmd.ErrOrStderr(),
			})
			if err != nil {
				return err
			}
			printSetupResult(cmd.OutOrStdout(), result)
			return nil
		},
	}
	cmd.Flags().StringVar(&kind, "command", "install", "setup command to run: install or dev")
	cmd.Flags().BoolVar(&yes, "yes", false, "run without interactive confirmation")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "print the command without running it")
	cmd.Flags().BoolVar(&allowUnknown, "allow-unknown", false, "allow an unknown setup command after review")
	cmd.Flags().BoolVar(&allowGlobal, "allow-global", false, "allow a setup command that appears to install globally")
	return cmd
}

func newSetupApplyCommand() *cobra.Command {
	var yes bool
	var dryRun bool
	var allowUnknown bool
	var allowGlobal bool
	cmd := &cobra.Command{
		Use:   "apply",
		Short: "Run reviewed install commands for all detected projects",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			plan, err := BuildSetupPlan()
			if err != nil {
				return err
			}
			if len(plan.Projects) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No setup commands detected.")
				return nil
			}
			if !dryRun && !yes {
				if err := confirmSetup(cmd.InOrStdin(), cmd.ErrOrStderr(), "Type run all to run install commands for every runnable project: ", "run all"); err != nil {
					return err
				}
			}
			results, err := RunAllProjectSetups(SetupRunOptions{
				CommandKind:  "install",
				DryRun:       dryRun,
				AllowUnknown: allowUnknown,
				AllowGlobal:  allowGlobal,
				Stdout:       cmd.OutOrStdout(),
				Stderr:       cmd.ErrOrStderr(),
			})
			if err != nil {
				return err
			}
			if len(results) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No runnable install commands detected.")
				return nil
			}
			for _, result := range results {
				printSetupResult(cmd.OutOrStdout(), result)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&yes, "yes", false, "run without interactive confirmation")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "print commands without running them")
	cmd.Flags().BoolVar(&allowUnknown, "allow-unknown", false, "allow unknown setup commands after review")
	cmd.Flags().BoolVar(&allowGlobal, "allow-global", false, "allow setup commands that appear to install globally")
	return cmd
}

func printSetupPlan(out io.Writer, plan SetupPlan) {
	fmt.Fprintln(out, "Setup commands:")
	if len(plan.Projects) == 0 {
		fmt.Fprintln(out, "(none)")
		return
	}
	for _, p := range plan.Projects {
		status := "runnable"
		if !p.Runnable {
			status = "review required"
		}
		fmt.Fprintf(out, "- %s (%s): %s\n", p.Project, p.Path, status)
		if p.PackageManager != "" {
			fmt.Fprintf(out, "  package manager: %s\n", p.PackageManager)
		}
		if p.InstallCommand != "" {
			fmt.Fprintf(out, "  install: %s\n", p.InstallCommand)
		}
		if p.DevCommand != "" {
			fmt.Fprintf(out, "  dev: %s\n", p.DevCommand)
		}
		if p.Reason != "" {
			fmt.Fprintf(out, "  reason: %s\n", p.Reason)
		}
	}
	fmt.Fprintln(out)
	fmt.Fprintln(out, "No setup commands were run.")
}

func printSetupResult(out io.Writer, result SetupRunResult) {
	action := "Ran"
	if result.DryRun {
		action = "Would run"
	}
	fmt.Fprintf(out, "%s `%s` in %s\n", action, result.Command, result.Path)
}

func confirmSetup(in io.Reader, out io.Writer, prompt, expected string) error {
	fmt.Fprint(out, prompt)
	answer, err := bufio.NewReader(in).ReadString('\n')
	if err != nil && err != io.EOF {
		return err
	}
	if strings.TrimSpace(answer) != expected {
		return fmt.Errorf("confirmation did not match; no setup commands were run")
	}
	return nil
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
