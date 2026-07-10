package devspace

import (
	"bufio"
	"fmt"
	"io"
	"sort"
	"strings"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"
)

// This file holds the presentation/formatting helpers used by the cobra
// command RunE handlers. Most take an io.Writer and a domain value and write
// human-readable output with no side effects, which keeps them easy to unit
// test (see commands_test.go); printStatus is the exception -- it loads
// config/manifest/state to assemble the view it prints. Command construction
// and flag wiring remain in commands.go.
//
// Every helper wraps its io.Writer with styledWriter as its first step, so
// callers keep passing plain writers (cmd.OutOrStdout(), a bytes.Buffer in
// tests) and get automatic ANSI downsampling/stripping for free.

func profileOrDefault(profile string) string {
	if profile == "" {
		return "dev"
	}
	return profile
}

// printOK writes a themed confirmation line (rendered green) to out. Used by
// command handlers for simple one-line success confirmations.
func printOK(out io.Writer, format string, args ...any) {
	fmt.Fprintln(styledWriter(out), currentTheme.OK.Render(fmt.Sprintf(format, args...)))
}

// printCaution writes a themed caution line (rendered amber) to out. Used for
// confirmations that carry a caveat worth a second look (irreversible
// actions, remaining state to clean up manually).
func printCaution(out io.Writer, format string, args ...any) {
	fmt.Fprintln(styledWriter(out), currentTheme.Warn.Render(fmt.Sprintf(format, args...)))
}

// printLine writes a plain informational line to out, still routed through
// styledWriter so NO_COLOR/--no-color/piped output stays byte-clean even
// though no color is applied to this particular line.
func printLine(out io.Writer, format string, args ...any) {
	fmt.Fprintln(styledWriter(out), fmt.Sprintf(format, args...))
}

func printProjectUpdateReport(out io.Writer, report ProjectUpdateReport) {
	out = styledWriter(out)
	updated, skipped, failed := 0, 0, 0
	for _, result := range report.Results {
		switch result.Status {
		case "updated":
			updated++
		case "skipped":
			skipped++
		case "failed":
			failed++
		}
		line := fmt.Sprintf("%s %s: %s", result.Action, result.Project.Name, result.Status)
		if result.Reason != "" {
			line = fmt.Sprintf("%s (%s)", line, result.Reason)
		}
		fmt.Fprintln(out, line)
	}
	fmt.Fprintf(out, "Updated projects: %d updated, %d skipped, %d failed\n", updated, skipped, failed)
}

// countStyle colors a count green when it is zero (nothing to worry about)
// and amber otherwise (worth a second look).
func countStyle(n int) func(string) string {
	style := currentTheme.OK
	if n != 0 {
		style = currentTheme.Warn
	}
	return func(s string) string { return style.Render(s) }
}

func printManifestDiff(out io.Writer, diff ManifestDiff) {
	out = styledWriter(out)
	fmt.Fprintln(out, currentTheme.Header.Render("Workspace manifest diff:"))
	fmt.Fprintf(out, "Added: %s\n", countStyle(len(diff.Added))(fmt.Sprint(len(diff.Added))))
	for _, p := range diff.Added {
		fmt.Fprintln(out, currentTheme.OK.Render(fmt.Sprintf("  + %s (%s)", p.Path, p.Name)))
	}
	fmt.Fprintf(out, "Removed: %s\n", countStyle(len(diff.Removed))(fmt.Sprint(len(diff.Removed))))
	for _, p := range diff.Removed {
		fmt.Fprintln(out, currentTheme.Fail.Render(fmt.Sprintf("  - %s (%s)", p.Path, p.Name)))
	}
	fmt.Fprintf(out, "Changed: %s\n", countStyle(len(diff.Changed))(fmt.Sprint(len(diff.Changed))))
	for _, changed := range diff.Changed {
		fmt.Fprintln(out, currentTheme.Warn.Render(fmt.Sprintf("  ~ %s (%s)", changed.Remote.Path, changed.Remote.Name)))
		for _, field := range changed.Changes {
			fmt.Fprintf(out, "    %s: %q -> %q\n", field.Field, field.Local, field.Remote)
		}
	}
	if len(diff.Added) == 0 && len(diff.Removed) == 0 && len(diff.Changed) == 0 {
		fmt.Fprintln(out, currentTheme.OK.Render("No remote manifest differences."))
	}
}

func printReconcilePlan(out io.Writer, plan ReconcilePlan, applied bool) {
	out = styledWriter(out)
	fmt.Fprintln(out, currentTheme.Header.Render("Workspace reconcile:"))
	if plan.TwoWay {
		fmt.Fprintln(out, currentTheme.Warn.Render("Two-way mode: no base manifest snapshot was found."))
	}
	fmt.Fprintf(out, "Auto-merged: %s\n", countStyle(len(plan.Ops))(fmt.Sprint(len(plan.Ops))))
	for _, op := range plan.Ops {
		fmt.Fprintf(out, "  %s %s %s\n", strings.ToUpper(op.Action), op.Kind, op.Key)
	}
	if len(plan.Ops) == 0 {
		fmt.Fprintln(out, currentTheme.OK.Render("No local manifest changes."))
	}
	fmt.Fprintf(out, "Conflicts: %s\n", countStyle(len(plan.Conflicts))(fmt.Sprint(len(plan.Conflicts))))
	for _, conflict := range plan.Conflicts {
		fmt.Fprintf(out, "  %s %s field %s\n", conflict.Entity, conflict.Key, conflict.Field)
		fmt.Fprintf(out, "    local: %q\n", conflict.Ours)
		fmt.Fprintf(out, "    remote: %q\n", conflict.Theirs)
	}
	if len(plan.Conflicts) == 0 {
		fmt.Fprintln(out, currentTheme.OK.Render("No reconcile conflicts."))
	}
	if applied {
		fmt.Fprintln(out, currentTheme.OK.Render("Applied reconciled manifest."))
	} else {
		fmt.Fprintln(out, currentTheme.Muted.Render("Review saved to DEVSPACE_HOME/last-reconcile.json."))
	}
}

func printPlan(out io.Writer, plan Plan) {
	printPlanWithHeading(out, plan, "Planned changes:")
}

func printPlanWithHeading(out io.Writer, plan Plan, heading string) {
	out = styledWriter(out)
	fmt.Fprintln(out, currentTheme.Header.Render(heading))
	fmt.Fprintln(out)
	fmt.Fprintln(out, currentTheme.OK.Render("SAFE:"))
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
	fmt.Fprintln(out, currentTheme.Warn.Render("SKIPPED:"))
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
	fmt.Fprintln(out, currentTheme.Fail.Render("WARNINGS:"))
	if len(plan.Warnings) == 0 {
		fmt.Fprintln(out, "(none)")
	} else {
		for _, w := range plan.Warnings {
			fmt.Fprintf(out, "- %s\n", w)
		}
	}
	fmt.Fprintln(out)
	fmt.Fprintln(out, currentTheme.Muted.Render("No destructive changes will be performed."))
}

func printApply(out io.Writer, plan Plan) {
	out = styledWriter(out)
	fmt.Fprintln(out, currentTheme.OK.Render("Applied safe plan actions."))
	printPlanWithHeading(out, plan, "Apply results:")
}

func printSetupPlan(out io.Writer, plan SetupPlan) {
	out = styledWriter(out)
	fmt.Fprintln(out, currentTheme.Header.Render("Setup commands:"))
	if len(plan.Projects) == 0 {
		fmt.Fprintln(out, "(none)")
		return
	}
	for _, p := range plan.Projects {
		status := currentTheme.OK.Render("runnable")
		if !p.Runnable {
			status = currentTheme.Warn.Render("review required")
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
	fmt.Fprintln(out, currentTheme.Muted.Render("No setup commands were run."))
}

func printSetupResult(out io.Writer, result SetupRunResult) {
	out = styledWriter(out)
	action := "Ran"
	if result.DryRun {
		action = "Would run"
	}
	fmt.Fprintf(out, "%s `%s` in %s\n", currentTheme.Emph.Render(action), result.Command, result.Path)
}

func confirmSetup(in io.Reader, out io.Writer, prompt, expected string) error {
	fmt.Fprint(styledWriter(out), prompt)
	answer, err := bufio.NewReader(in).ReadString('\n')
	if err != nil && err != io.EOF {
		return err
	}
	if strings.TrimSpace(answer) != expected {
		return fmt.Errorf("confirmation did not match; no setup commands were run")
	}
	return nil
}

// WorkspaceStatusReport is the --json shape for the aggregate `status`
// command (no project argument).
type WorkspaceStatusReport struct {
	Machine         string `json:"machine"`
	Workspace       string `json:"workspace"`
	ProjectsTracked int    `json:"projectsTracked"`
	Hydrated        int    `json:"hydrated"`
	Placeholders    int    `json:"placeholders"`
	Dirty           int    `json:"dirty"`
	MissingEnv      int    `json:"missingEnv"`
	Outdated        int    `json:"outdated"`
	LastSyncAt      string `json:"lastSyncAt,omitempty"`
	LastScanAt      string `json:"lastScanAt,omitempty"`
}

// ProjectListRow is the machine-readable row shape for `devspace project list --json`.
type ProjectListRow struct {
	Project Project      `json:"project"`
	State   ProjectState `json:"state"`
}

func buildProjectListRows() ([]ProjectListRow, error) {
	cfg, err := LoadConfig()
	if err != nil {
		return nil, err
	}
	m, err := LoadManifest(cfg.WorkspaceRoot)
	if err != nil {
		return nil, err
	}
	st, err := LoadState()
	if err != nil && !missing(err) {
		return nil, err
	}
	rows := make([]ProjectListRow, 0, len(m.Projects))
	for _, p := range m.Projects {
		rows = append(rows, ProjectListRow{Project: p, State: st.Projects[p.ID]})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Project.Path == rows[j].Project.Path {
			return rows[i].Project.Name < rows[j].Project.Name
		}
		return rows[i].Project.Path < rows[j].Project.Path
	})
	return rows, nil
}

func buildProjectStatusRow(ref string) (ProjectListRow, error) {
	rows, err := buildProjectListRows()
	if err != nil {
		return ProjectListRow{}, err
	}
	for _, row := range rows {
		if row.Project.ID == ref || row.Project.Name == ref || row.Project.Path == ref {
			return row, nil
		}
	}
	return ProjectListRow{}, fmt.Errorf("project %q not found", ref)
}

func printProjectList(out io.Writer, rows []ProjectListRow) {
	out = styledWriter(out)
	if len(rows) == 0 {
		fmt.Fprintln(out, "No tracked projects. Run `devspace project track <relative-path>` or `devspace scan` to add projects.")
		return
	}
	tableRows := make([][]string, 0, len(rows))
	for _, row := range rows {
		tableRows = append(tableRows, []string{
			valueOrDash(row.Project.Name),
			valueOrDash(row.Project.Path),
			valueOrDash(row.Project.Type),
			strings.ToLower(dashboardStatus(row.State)),
			yesNo(row.State.Dirty),
			valueOrDash(row.State.CurrentBranch),
			yesNo(row.State.EnvFilePresent),
		})
	}
	tbl := table.New().
		Headers("NAME", "PATH", "TYPE", "STATUS", "DIRTY", "BRANCH", "ENV").
		Rows(tableRows...).
		BorderStyle(currentTheme.Muted).
		StyleFunc(func(row, col int) lipgloss.Style {
			if row == table.HeaderRow {
				return currentTheme.Header.Padding(0, 1)
			}
			if col == 3 {
				switch tableRows[row][3] {
				case "hydrated":
					return currentTheme.OK.Padding(0, 1)
				case "missing":
					return currentTheme.Fail.Padding(0, 1)
				default:
					return currentTheme.Warn.Padding(0, 1)
				}
			}
			return lipgloss.NewStyle().Padding(0, 1)
		})
	fmt.Fprintln(out, tbl.Render())
}

// buildWorkspaceStatusReport loads config/manifest/state and aggregates
// per-project counts. It is the single source of truth for both the
// aggregate text view (printStatus) and `status --json`.
func buildWorkspaceStatusReport() (WorkspaceStatusReport, error) {
	cfg, err := LoadConfig()
	if err != nil {
		return WorkspaceStatusReport{}, err
	}
	m, err := LoadManifest(cfg.WorkspaceRoot)
	if err != nil {
		return WorkspaceStatusReport{}, err
	}
	st, err := LoadState()
	if err != nil && !missing(err) {
		return WorkspaceStatusReport{}, err
	}
	report := WorkspaceStatusReport{
		Machine:         cfg.MachineName,
		Workspace:       cfg.WorkspaceRoot,
		ProjectsTracked: len(m.Projects),
		LastSyncAt:      st.LastSyncAt,
		LastScanAt:      st.LastScanAt,
	}
	for _, p := range m.Projects {
		ps := st.Projects[p.ID]
		if ps.Hydrated {
			report.Hydrated++
		}
		if ps.Placeholder {
			report.Placeholders++
		}
		if ps.Dirty {
			report.Dirty++
		}
		if !ps.EnvFilePresent {
			report.MissingEnv++
		}
		if ps.Stale || ps.Missing {
			report.Outdated++
		}
	}
	return report, nil
}

func printStatus(out io.Writer, args []string) error {
	out = styledWriter(out)
	if len(args) == 1 {
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
		p, ok := findProject(m, args[0])
		if !ok {
			return fmt.Errorf("project %q not found", args[0])
		}
		ps := st.Projects[p.ID]
		fmt.Fprintf(out, "Project: %s\nPath: %s\nHydrated: %s\nDirty: %s\nMissing env: %s\n",
			p.Name, p.Path,
			boolStyle(ps.Hydrated, true).Render(fmt.Sprint(ps.Hydrated)),
			boolStyle(ps.Dirty, false).Render(fmt.Sprint(ps.Dirty)),
			boolStyle(!ps.EnvFilePresent, false).Render(fmt.Sprint(!ps.EnvFilePresent)),
		)
		return nil
	}
	report, err := buildWorkspaceStatusReport()
	if err != nil {
		return err
	}
	fmt.Fprintln(out, currentTheme.Header.Render("Workspace Status"))
	fmt.Fprintf(out, "Machine: %s\n", report.Machine)
	fmt.Fprintf(out, "Workspace: %s\n\n", report.Workspace)
	fmt.Fprintf(out, "Projects tracked: %d\n", report.ProjectsTracked)
	fmt.Fprintf(out, "Hydrated: %d\n", report.Hydrated)
	fmt.Fprintf(out, "Placeholders: %d\n", report.Placeholders)
	fmt.Fprintf(out, "Dirty repos: %s\n", countStyle(report.Dirty)(fmt.Sprint(report.Dirty)))
	fmt.Fprintf(out, "Missing env files: %s\n", countStyle(report.MissingEnv)(fmt.Sprint(report.MissingEnv)))
	fmt.Fprintf(out, "Outdated repos: %s\n", countStyle(report.Outdated)(fmt.Sprint(report.Outdated)))
	if report.LastSyncAt != "" {
		fmt.Fprintf(out, "Last sync: %s\n", report.LastSyncAt)
	}
	if report.LastScanAt != "" {
		fmt.Fprintf(out, "Last scan: %s\n", report.LastScanAt)
	}
	return nil
}

// boolStyle colors a boolean value: goodWhenTrue controls whether true or
// false is the "OK" state (true is good for Hydrated, false is good for
// Dirty/Missing env).
func boolStyle(v, goodWhenTrue bool) lipgloss.Style {
	if v == goodWhenTrue {
		return currentTheme.OK
	}
	return currentTheme.Warn
}

// printRecipientTable renders a bordered table of encrypted-profile
// recipients, coloring each row's status: active is good, revoked is a
// problem worth noticing.
func printRecipientTable(out io.Writer, recipients []SecretRecipient) {
	out = styledWriter(out)
	if len(recipients) == 0 {
		fmt.Fprintln(out, "(no recipients)")
		return
	}
	rows := make([][]string, 0, len(recipients))
	for _, r := range recipients {
		status := "active"
		if r.RevokedAt != "" {
			status = "revoked"
		}
		rows = append(rows, []string{r.ID, r.Name, status})
	}
	tbl := table.New().
		Headers("ID", "NAME", "STATUS").
		Rows(rows...).
		BorderStyle(currentTheme.Muted).
		StyleFunc(func(row, col int) lipgloss.Style {
			if row == table.HeaderRow {
				return currentTheme.Header.Padding(0, 1)
			}
			if col == 2 {
				style := currentTheme.OK
				if rows[row][2] == "revoked" {
					style = currentTheme.Fail
				}
				return style.Padding(0, 1)
			}
			return lipgloss.NewStyle().Padding(0, 1)
		})
	fmt.Fprintln(out, tbl.Render())
}

func sortedProjectNames(m Manifest) string {
	names := make([]string, 0, len(m.Projects))
	for _, p := range m.Projects {
		names = append(names, p.Name)
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}
