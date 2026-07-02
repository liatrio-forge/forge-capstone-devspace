package devspace

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type doctorSeverity string

const (
	doctorOK   doctorSeverity = "OK"
	doctorInfo doctorSeverity = "INFO"
	doctorWarn doctorSeverity = "WARN"
	doctorFail doctorSeverity = "FAIL"
)

type doctorCheck struct {
	Severity doctorSeverity
	Subject  string
	Detail   string
	Hard     bool
}

type doctorReport struct {
	Checks       []doctorCheck
	HardFailures int
}

type projectDoctorState struct {
	Project Project
	State   ProjectState
	Warning string
}

func RunDoctor(out io.Writer) error {
	report := buildDoctorReport()
	printDoctorReport(out, report)
	if report.HardFailures > 0 {
		return fmt.Errorf("doctor found %d hard failure(s)", report.HardFailures)
	}
	return nil
}

func buildDoctorReport() doctorReport {
	var report doctorReport
	gitAvailable := checkGit(&report)

	cfgPath, cfgPathErr := configPath()
	if cfgPathErr != nil {
		report.add(doctorFail, "Config", fmt.Sprintf("cannot resolve config path: %s", cfgPathErr), true)
		return report
	}
	if !exists(cfgPath) {
		report.add(doctorFail, "Config", fmt.Sprintf("missing %s; run `devspace init` first", cfgPath), true)
		return report
	}
	cfg, err := LoadConfig()
	if err != nil {
		report.add(doctorFail, "Config", fmt.Sprintf("cannot read %s: %s", cfgPath, err), true)
		return report
	}
	report.add(doctorOK, "Config", cfgPath, false)

	workspaceReady := checkWorkspace(&report, cfg)
	checkAgeIdentity(&report, cfg)
	m, manifestReady := checkManifest(&report, cfg, workspaceReady)
	checkManifestRemote(&report, cfg, gitAvailable)
	checkLastPlan(&report, cfg, manifestReady)
	if manifestReady {
		checkProjectStates(&report, cfg, m)
	}
	return report
}

func checkGit(report *doctorReport) bool {
	if err := ensureGitAvailable(); err != nil {
		report.add(doctorFail, "Git", err.Error(), true)
		return false
	}
	report.add(doctorOK, "Git", "git executable is available", false)
	return true
}

func checkWorkspace(report *doctorReport, cfg Config) bool {
	if strings.TrimSpace(cfg.WorkspaceRoot) == "" {
		report.add(doctorFail, "Workspace", "workspaceRoot is empty in config", true)
		return false
	}
	workspace, err := expandPath(cfg.WorkspaceRoot)
	if err != nil {
		report.add(doctorFail, "Workspace", fmt.Sprintf("cannot resolve workspaceRoot %q: %s", cfg.WorkspaceRoot, err), true)
		return false
	}
	info, err := os.Stat(workspace)
	if err != nil {
		report.add(doctorFail, "Workspace", fmt.Sprintf("cannot access %s: %s", workspace, err), true)
		return false
	}
	if !info.IsDir() {
		report.add(doctorFail, "Workspace", fmt.Sprintf("%s is not a directory", workspace), true)
		return false
	}
	report.add(doctorOK, "Workspace", workspace, false)
	return true
}

func checkAgeIdentity(report *doctorReport, cfg Config) {
	identityPath, err := resolveAgeIdentityPath(cfg)
	if err != nil {
		report.add(doctorFail, "Age identity", fmt.Sprintf("cannot resolve age identity path: %s", err), true)
		return
	}
	if _, err := loadIdentity(identityPath); err != nil {
		report.add(doctorFail, "Age identity", fmt.Sprintf("cannot read %s: %s", identityPath, err), true)
		return
	}
	report.add(doctorOK, "Age identity", identityPath, false)
}

func checkManifest(report *doctorReport, cfg Config, workspaceReady bool) (Manifest, bool) {
	if !workspaceReady {
		report.add(doctorFail, "Manifest", "workspace is not usable, so manifest cannot be checked", true)
		return Manifest{}, false
	}
	path := manifestPath(cfg.WorkspaceRoot)
	if !exists(path) {
		report.add(doctorFail, "Manifest", fmt.Sprintf("missing %s; run `devspace init` or `devspace workspace pull`", path), true)
		return Manifest{}, false
	}
	m, err := LoadManifest(cfg.WorkspaceRoot)
	if err != nil {
		report.add(doctorFail, "Manifest", fmt.Sprintf("invalid %s: %s", path, err), true)
		return Manifest{}, false
	}
	if m.WorkspaceRoot != cfg.WorkspaceRoot {
		report.add(doctorFail, "Manifest", fmt.Sprintf("workspaceRoot mismatch: manifest has %s, config has %s", m.WorkspaceRoot, cfg.WorkspaceRoot), true)
		return m, false
	}
	report.add(doctorOK, "Manifest", fmt.Sprintf("%s is valid with %d tracked project(s)", path, len(m.Projects)), false)
	report.add(doctorOK, "Workspace path safety", "all tracked project paths stay inside the workspace", false)
	return m, true
}

func checkManifestRemote(report *doctorReport, cfg Config, gitAvailable bool) {
	if strings.TrimSpace(cfg.ManifestRemote) == "" {
		report.add(doctorWarn, "Manifest remote", "not configured; workspace push/pull are unavailable until `devspace workspace remote set <url-or-path>`", false)
		return
	}
	report.add(doctorOK, "Manifest remote", redactRemote(cfg.ManifestRemote), false)
	if strings.TrimSpace(cfg.ManifestRepoPath) == "" {
		report.add(doctorWarn, "Manifest repo", "cache path is not set; the next workspace sync command will initialize it", false)
		return
	}
	if !gitAvailable {
		report.add(doctorFail, "Manifest repo", "cannot inspect manifest repo because Git is unavailable", true)
		return
	}
	repo, err := expandPath(cfg.ManifestRepoPath)
	if err != nil {
		report.add(doctorFail, "Manifest repo", fmt.Sprintf("cannot resolve %q: %s", cfg.ManifestRepoPath, err), true)
		return
	}
	if !exists(repo) {
		report.add(doctorWarn, "Manifest repo", fmt.Sprintf("%s does not exist yet; workspace push/pull will clone or initialize it", repo), false)
		return
	}
	info := gitInfo(repo)
	if !info.IsRepo {
		if isEmptyDir(repo) {
			report.add(doctorWarn, "Manifest repo", fmt.Sprintf("%s is empty but not initialized yet", repo), false)
			return
		}
		report.add(doctorFail, "Manifest repo", fmt.Sprintf("%s is non-empty and is not a Git repository", repo), true)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	origin := mustGit(ctx, repo, "remote", "get-url", "origin")
	if origin == "" {
		report.add(doctorWarn, "Manifest repo", fmt.Sprintf("%s has no origin remote configured", repo), false)
	} else if origin != cfg.ManifestRemote {
		report.add(doctorFail, "Manifest repo", fmt.Sprintf("origin is %s, but config remote is %s", redactRemote(origin), redactRemote(cfg.ManifestRemote)), true)
	}
	if err := ensureCleanManifestRepo(repo); err != nil {
		report.add(doctorWarn, "Manifest repo cleanliness", err.Error(), false)
		return
	}
	report.add(doctorOK, "Manifest repo cleanliness", fmt.Sprintf("%s is clean", repo), false)
}

func checkLastPlan(report *doctorReport, cfg Config, manifestReady bool) {
	path := lastPlanPath(cfg.WorkspaceRoot)
	if !exists(path) {
		report.add(doctorInfo, "Last plan", "no saved plan found; run `devspace plan` before `devspace apply`", false)
		return
	}
	plan, err := LoadLastPlan(cfg.WorkspaceRoot)
	if err != nil {
		report.add(doctorWarn, "Last plan", fmt.Sprintf("cannot read %s: %s", path, err), false)
		return
	}
	if plan.WorkspaceRoot != cfg.WorkspaceRoot {
		report.add(doctorWarn, "Last plan", fmt.Sprintf("workspaceRoot mismatch: plan has %s, config has %s", plan.WorkspaceRoot, cfg.WorkspaceRoot), false)
		return
	}
	if !manifestReady {
		report.add(doctorWarn, "Last plan", "manifest is not valid, so saved plan hash cannot be checked", false)
		return
	}
	currentHash, err := ManifestHash(cfg.WorkspaceRoot)
	if err != nil {
		report.add(doctorWarn, "Last plan", fmt.Sprintf("cannot hash current manifest: %s", err), false)
		return
	}
	if plan.ManifestHash == "" {
		report.add(doctorWarn, "Last plan", "saved plan has no manifest hash; run `devspace plan` again", false)
		return
	}
	if plan.ManifestHash != currentHash {
		report.add(doctorWarn, "Last plan", "saved plan hash does not match current manifest; run `devspace plan` again before apply", false)
		return
	}
	report.add(doctorOK, "Last plan", "saved plan hash matches current manifest", false)
}

func checkProjectStates(report *doctorReport, cfg Config, m Manifest) {
	states := make([]projectDoctorState, 0, len(m.Projects))
	counts := map[string]int{
		"hydrated":    0,
		"placeholder": 0,
		"missing":     0,
		"dirty":       0,
		"missingEnv":  0,
	}
	for _, p := range m.Projects {
		full, _, err := safeWorkspacePath(cfg.WorkspaceRoot, p.Path)
		if err != nil {
			report.add(doctorFail, "Project path", fmt.Sprintf("%s: %s", p.Path, err), true)
			continue
		}
		info := gitInfo(full)
		ps := stateForProject(full, p, info)
		if ps.Hydrated {
			counts["hydrated"]++
		}
		if ps.Placeholder {
			counts["placeholder"]++
		}
		if ps.Missing {
			counts["missing"]++
		}
		if ps.Dirty {
			counts["dirty"]++
		}
		if !ps.EnvFilePresent {
			counts["missingEnv"]++
		}
		states = append(states, projectDoctorState{
			Project: p,
			State:   ps,
			Warning: info.InspectWarning,
		})
	}
	sort.Slice(states, func(i, j int) bool {
		return states[i].Project.Path < states[j].Project.Path
	})
	severity := doctorOK
	if counts["placeholder"] > 0 || counts["missing"] > 0 || counts["dirty"] > 0 || counts["missingEnv"] > 0 {
		severity = doctorWarn
	}
	report.add(severity, "Projects", fmt.Sprintf(
		"%d tracked; hydrated: %d, placeholders: %d, missing: %d, dirty: %d, missing .env: %d",
		len(m.Projects),
		counts["hydrated"],
		counts["placeholder"],
		counts["missing"],
		counts["dirty"],
		counts["missingEnv"],
	), false)
	for _, state := range states {
		report.add(doctorInfo, "Project "+state.Project.Name, projectStateDetail(state), false)
		if state.Warning != "" {
			report.add(doctorWarn, "Project "+state.Project.Name, state.Warning, false)
		}
	}
}

func projectStateDetail(p projectDoctorState) string {
	var labels []string
	switch {
	case p.State.Missing:
		labels = append(labels, "missing")
	case p.State.Placeholder:
		labels = append(labels, "placeholder")
	case p.State.Hydrated:
		labels = append(labels, "hydrated")
	default:
		labels = append(labels, "present")
	}
	if p.State.Dirty {
		labels = append(labels, "dirty")
	}
	if !p.State.EnvFilePresent {
		labels = append(labels, "missing .env")
	}
	if p.State.CurrentBranch != "" {
		labels = append(labels, "branch "+p.State.CurrentBranch)
	}
	if p.State.LastCommit != "" {
		labels = append(labels, "commit "+p.State.LastCommit)
	}
	return fmt.Sprintf("%s (%s)", strings.Join(labels, ", "), filepath.ToSlash(p.Project.Path))
}

func printDoctorReport(out io.Writer, report doctorReport) {
	fmt.Fprintln(out, "DevSpace doctor")
	fmt.Fprintln(out)
	for _, check := range report.Checks {
		fmt.Fprintf(out, "[%s] %s: %s\n", check.Severity, check.Subject, check.Detail)
	}
	fmt.Fprintln(out)
	if report.HardFailures == 0 {
		fmt.Fprintln(out, "Result: ready; warnings above do not block core commands.")
		return
	}
	fmt.Fprintf(out, "Result: %d hard failure(s); fix these before running core commands.\n", report.HardFailures)
}

func (r *doctorReport) add(severity doctorSeverity, subject, detail string, hard bool) {
	r.Checks = append(r.Checks, doctorCheck{
		Severity: severity,
		Subject:  subject,
		Detail:   detail,
		Hard:     hard,
	})
	if hard {
		r.HardFailures++
	}
}
