package devspace

import (
	"bufio"
	"fmt"
	"io"
	"sort"
	"strings"
)

// This file holds the pure presentation/formatting helpers used by the cobra
// command RunE handlers. They take an io.Writer and a domain value and write
// human-readable output; none of them touch the filesystem, the config, or
// global state, which keeps them easy to unit-test (see commands_test.go).
// Command construction and flag wiring remain in commands.go.

func profileOrDefault(profile string) string {
	if profile == "" {
		return "dev"
	}
	return profile
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
