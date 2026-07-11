package devspace

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

type ScanSummary struct {
	FoundProjects     int
	GitRepos          int
	UntrackedFolders  int
	LocalOnlyProjects int
	ProjectsWithEnv   int
}

const workspaceIgnoreFile = ".devspaceignore"

type ProjectUpdateReport struct {
	Results []ProjectUpdateResult
}

type ProjectUpdateResult struct {
	Project Project
	Action  string
	Status  string
	Reason  string
}

func ScanWorkspace() (ScanSummary, error) {
	cfg, err := LoadConfig()
	if err != nil {
		return ScanSummary{}, err
	}
	m, err := LoadManifest(cfg.WorkspaceRoot)
	if err != nil {
		return ScanSummary{}, err
	}
	st, err := LoadState()
	if err != nil && !missing(err) {
		return ScanSummary{}, err
	}
	if st.Projects == nil {
		st.Projects = map[string]ProjectState{}
	}

	seen := map[string]bool{}
	summary := ScanSummary{}
	ignore := loadWorkspaceIgnore(cfg.WorkspaceRoot)
	var localRoot string
	err = filepath.WalkDir(cfg.WorkspaceRoot, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !d.IsDir() {
			return nil
		}
		if path == cfg.WorkspaceRoot {
			return nil
		}
		name := d.Name()
		if name == ".git" || name == workspaceDirName || name == legacyWorkspaceDirName || ignoredName(name) {
			return filepath.SkipDir
		}
		rel, err := filepath.Rel(cfg.WorkspaceRoot, path)
		if err != nil {
			return err
		}
		_, clean, err := safeWorkspacePath(cfg.WorkspaceRoot, rel)
		if err != nil {
			return filepath.SkipDir
		}
		if ignore.match(clean) {
			return filepath.SkipDir
		}
		info := gitInfo(path)
		underLocal := localRoot != "" && strings.HasPrefix(path+string(filepath.Separator), localRoot+string(filepath.Separator))
		if !underLocal {
			localRoot = ""
		}
		hasMarker := info.IsRepo || (!underLocal && (hasDependencyMarker(path) || exists(filepath.Join(path, ".env"))))
		if !hasMarker {
			summary.UntrackedFolders++
			return nil
		}
		p := projectFromPath(clean, path, info)
		upsertProject(&m, p)
		st.Projects[p.ID] = stateForProject(path, p, info)
		seen[p.ID] = true
		summary.FoundProjects++
		if info.IsRepo {
			summary.GitRepos++
		} else {
			summary.LocalOnlyProjects++
			if localRoot == "" {
				localRoot = path
			}
		}
		if exists(filepath.Join(path, ".env")) {
			summary.ProjectsWithEnv++
		}
		if info.IsRepo {
			return filepath.SkipDir
		}
		return nil
	})
	if err != nil {
		return ScanSummary{}, err
	}
	for _, p := range m.Projects {
		if seen[p.ID] {
			continue
		}
		full, _, err := safeWorkspacePath(cfg.WorkspaceRoot, p.Path)
		if err != nil {
			return ScanSummary{}, err
		}
		st.Projects[p.ID] = stateForProject(full, p, gitInfo(full))
	}
	st.LastScanAt = nowRFC3339()
	if err := SaveManifest(cfg.WorkspaceRoot, m); err != nil {
		return ScanSummary{}, err
	}
	return summary, SaveState(st)
}

func projectFromPath(rel, abs string, info GitInfo) Project {
	p := Project{
		ID:          projectID(rel),
		Name:        projectName(rel),
		Path:        rel,
		Type:        ProjectTypeLocal,
		HydrateMode: HydrateManual,
		Ignore:      append([]string{}, DefaultIgnores...),
		Setup:       detectSetup(abs),
	}
	if info.IsRepo {
		p.Type = ProjectTypeGit
		p.Remote = info.Remote
		p.DefaultBranch = info.DefaultBranch
		p.HydrateMode = HydrateOnDemand
	}
	return p
}

type workspaceIgnore []string

func loadWorkspaceIgnore(workspace string) workspaceIgnore {
	data, err := os.ReadFile(filepath.Join(workspace, workspaceIgnoreFile))
	if err != nil {
		return nil
	}
	var rules workspaceIgnore
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(strings.TrimSuffix(line, "/"), "/")
		line = filepath.ToSlash(filepath.Clean(line))
		if line == "." || line == ".." || strings.HasPrefix(line, "../") {
			continue
		}
		rules = append(rules, line)
	}
	return rules
}

func (rules workspaceIgnore) match(rel string) bool {
	rel = filepath.ToSlash(filepath.Clean(rel))
	for _, rule := range rules {
		if rel == rule || strings.HasPrefix(rel+"/", rule+"/") {
			return true
		}
	}
	return false
}

func stateForProject(abs string, p Project, info GitInfo) ProjectState {
	_, err := os.Stat(abs)
	existsOnDisk := err == nil
	placeholder := existsOnDisk && isEmptyDir(abs) && p.Type == ProjectTypeGit && !info.IsRepo
	return ProjectState{
		Hydrated:       existsOnDisk && !placeholder && (p.Type != ProjectTypeGit || info.IsRepo),
		Exists:         existsOnDisk,
		Dirty:          info.Dirty,
		CurrentBranch:  info.CurrentBranch,
		LastCommit:     info.LastCommit,
		EnvFilePresent: exists(filepath.Join(abs, ".env")),
		LastCheckedAt:  nowRFC3339(),
		Placeholder:    placeholder,
		Stale:          false,
		Missing:        !existsOnDisk,
	}
}

func AddProject(rel string) (Project, error) {
	cfg, err := LoadConfig()
	if err != nil {
		return Project{}, err
	}
	full, clean, err := safeWorkspacePath(cfg.WorkspaceRoot, rel)
	if err != nil {
		return Project{}, err
	}
	stat, statErr := os.Stat(full)
	if statErr != nil {
		if !os.IsNotExist(statErr) {
			return Project{}, statErr
		}
		if err := os.MkdirAll(full, 0o750); err != nil {
			return Project{}, err
		}
	} else if !stat.IsDir() {
		return Project{}, fmt.Errorf("cannot add %s: path exists and is not a directory", clean)
	}
	info := gitInfo(full)
	p := projectFromPath(clean, full, info)
	m, err := LoadManifest(cfg.WorkspaceRoot)
	if err != nil {
		return Project{}, err
	}
	upsertProject(&m, p)
	if err := SaveManifest(cfg.WorkspaceRoot, m); err != nil {
		return Project{}, err
	}
	st, err := LoadState()
	if err != nil && !missing(err) {
		return Project{}, err
	}
	if st.Projects == nil {
		st.Projects = map[string]ProjectState{}
	}
	st.Projects[p.ID] = stateForProject(full, p, info)
	return p, SaveState(st)
}

func RefreshProjectsForWatch(syncMode string, changed []string) (WatchRefresh, error) {
	mode, err := normalizeWatchSyncMode(syncMode)
	if err != nil {
		return WatchRefresh{}, err
	}
	cfg, err := LoadConfig()
	if err != nil {
		return WatchRefresh{}, err
	}
	m, err := LoadManifest(cfg.WorkspaceRoot)
	if err != nil {
		return WatchRefresh{}, err
	}
	st, err := LoadState()
	if err != nil && !missing(err) {
		return WatchRefresh{}, err
	}
	if st.Projects == nil {
		st.Projects = map[string]ProjectState{}
	}
	result := WatchRefresh{SyncMode: mode, RefreshStartedAt: nowRFC3339()}
	seen := map[string]bool{}
	for _, rel := range changed {
		full, clean, err := safeWorkspacePath(cfg.WorkspaceRoot, rel)
		if err != nil {
			return WatchRefresh{}, err
		}
		if seen[clean] {
			continue
		}
		seen[clean] = true
		existing, hasExisting := findProject(m, clean)
		info := gitInfo(full)
		if stat, err := os.Stat(full); err != nil {
			if os.IsNotExist(err) && hasExisting {
				st.Projects[existing.ID] = stateForProject(full, existing, info)
				continue
			}
			return WatchRefresh{}, err
		} else if !stat.IsDir() {
			if hasExisting {
				st.Projects[existing.ID] = stateForProject(full, existing, info)
			}
			continue
		}
		if !info.IsRepo && !hasDependencyMarker(full) && !exists(filepath.Join(full, ".env")) {
			if hasExisting {
				st.Projects[existing.ID] = stateForProject(full, existing, info)
			}
			continue
		}
		p := projectFromPath(clean, full, info)
		upsertProject(&m, p)
		st.Projects[p.ID] = stateForProject(full, p, info)
		result.Summary.FoundProjects++
		if info.IsRepo {
			result.Summary.GitRepos++
		} else {
			result.Summary.LocalOnlyProjects++
		}
		if exists(filepath.Join(full, ".env")) {
			result.Summary.ProjectsWithEnv++
		}
	}
	st.LastScanAt = nowRFC3339()
	if err := SaveManifest(cfg.WorkspaceRoot, m); err != nil {
		return WatchRefresh{}, err
	}
	if err := SaveState(st); err != nil {
		return WatchRefresh{}, err
	}
	return result, syncWatchManifest(&result, mode)
}

func RemoveProject(ref string) (Project, error) {
	cfg, err := LoadConfig()
	if err != nil {
		return Project{}, err
	}
	m, err := LoadManifest(cfg.WorkspaceRoot)
	if err != nil {
		return Project{}, err
	}
	p, ok := findProject(m, ref)
	if !ok {
		return Project{}, fmt.Errorf("project %q not found", ref)
	}
	projects := make([]Project, 0, len(m.Projects)-1)
	for _, project := range m.Projects {
		if project.ID != p.ID {
			projects = append(projects, project)
		}
	}
	access := make([]ProjectAccess, 0, len(m.Access))
	for _, projectAccess := range m.Access {
		if projectAccess.ProjectID != p.ID {
			access = append(access, projectAccess)
		}
	}
	m.Projects = projects
	m.Access = access
	if err := ValidateManifest(m); err != nil {
		return Project{}, err
	}
	if err := SaveManifest(cfg.WorkspaceRoot, m); err != nil {
		return Project{}, err
	}
	st, err := LoadState()
	if err != nil && !missing(err) {
		return Project{}, err
	}
	if st.Projects != nil {
		delete(st.Projects, p.ID)
	}
	return p, SaveState(st)
}

func BuildPlan() (Plan, error) {
	cfg, err := LoadConfig()
	if err != nil {
		return Plan{}, err
	}
	m, err := LoadManifest(cfg.WorkspaceRoot)
	if err != nil {
		return Plan{}, err
	}
	hash, err := ManifestHash(cfg.WorkspaceRoot)
	if err != nil {
		return Plan{}, err
	}
	plan := Plan{
		Version:       1,
		WorkspaceRoot: cfg.WorkspaceRoot,
		ManifestHash:  hash,
		GeneratedAt:   nowRFC3339(),
	}
	for _, p := range m.Projects {
		full, _, err := safeWorkspacePath(cfg.WorkspaceRoot, p.Path)
		if err != nil {
			plan.Actions = append(plan.Actions, PlanAction{Safety: "skipped", Kind: "skip", Path: p.Path, Project: p.Name, Reason: err.Error()})
			continue
		}
		info := gitInfo(full)
		if info.MissingGit {
			plan.Warnings = append(plan.Warnings, info.InspectWarning)
		}
		if !exists(full) {
			kind := "create-folder"
			if p.Type == ProjectTypeGit {
				kind = "placeholder"
			}
			plan.Actions = append(plan.Actions, PlanAction{Safety: "safe", Kind: kind, Path: p.Path, Project: p.Name})
			continue
		}
		if p.Type == ProjectTypeGit && info.IsRepo {
			if info.Dirty {
				plan.Actions = append(plan.Actions, PlanAction{Safety: "skipped", Kind: "skip", Path: p.Path, Project: p.Name, Reason: "repo is dirty; DevSpace will not pull or modify it"})
			}
			if p.Remote != "" && info.Remote != "" && info.Remote != p.Remote {
				plan.Warnings = append(plan.Warnings, fmt.Sprintf("%s has a different Git remote than manifest: %s != %s", p.Path, redactRemote(info.Remote), redactRemote(p.Remote)))
				plan.Actions = append(plan.Actions, PlanAction{Safety: "skipped", Kind: "skip", Path: p.Path, Project: p.Name, Reason: "Git remote differs from manifest"})
			}
			if info.InspectWarning != "" {
				plan.Warnings = append(plan.Warnings, fmt.Sprintf("%s: %s", p.Path, info.InspectWarning))
			}
			continue
		}
		if p.Type == ProjectTypeGit && !isEmptyDir(full) {
			plan.Actions = append(plan.Actions, PlanAction{Safety: "skipped", Kind: "skip", Path: p.Path, Project: p.Name, Reason: "folder exists and is non-empty but is not a Git repo"})
			continue
		}
		plan.Actions = append(plan.Actions, PlanAction{Safety: "skipped", Kind: "skip", Path: p.Path, Project: p.Name, Reason: "project already exists"})
	}
	return plan, nil
}

func SaveLastPlan(plan Plan) error {
	return writeJSON(lastPlanPath(plan.WorkspaceRoot), plan, 0o600)
}

func LoadLastPlan(workspace string) (Plan, error) {
	var plan Plan
	err := readJSON(lastPlanPath(workspace), &plan)
	return plan, err
}

func ApplyLastPlan() (Plan, error) {
	cfg, err := LoadConfig()
	if err != nil {
		return Plan{}, err
	}
	plan, err := LoadLastPlan(cfg.WorkspaceRoot)
	if err != nil {
		return Plan{}, fmt.Errorf("no saved plan found; run `devspace plan` first: %w", err)
	}
	currentHash, err := ManifestHash(cfg.WorkspaceRoot)
	if err != nil {
		return Plan{}, err
	}
	if currentHash != plan.ManifestHash {
		return Plan{}, fmt.Errorf("manifest changed since plan was generated; run `devspace plan` again before apply")
	}
	applied := plan
	applied.Actions = append([]PlanAction(nil), plan.Actions...)
	for i, action := range applied.Actions {
		if action.Safety != "safe" {
			continue
		}
		if action.Kind != "create-folder" && action.Kind != "placeholder" {
			continue
		}
		full, _, err := safeWorkspacePath(cfg.WorkspaceRoot, action.Path)
		if err != nil {
			applied.Actions[i].Safety = "skipped"
			applied.Actions[i].Kind = "skip"
			applied.Actions[i].Reason = err.Error()
			continue
		}
		if exists(full) {
			if !isEmptyDir(full) {
				applied.Actions[i].Safety = "skipped"
				applied.Actions[i].Kind = "skip"
				applied.Actions[i].Reason = "destination became non-empty after plan; skipped"
				continue
			}
			applied.Actions[i].Safety = "skipped"
			applied.Actions[i].Kind = "skip"
			applied.Actions[i].Reason = "destination already exists"
			continue
		}
		if err := os.MkdirAll(full, 0o750); err != nil {
			return applied, err
		}
	}
	if err := refreshAllProjectState(cfg.WorkspaceRoot); err != nil {
		return applied, err
	}
	return applied, nil
}

func HydrateProject(ref string) (Project, error) {
	cfg, err := LoadConfig()
	if err != nil {
		return Project{}, err
	}
	m, err := LoadManifest(cfg.WorkspaceRoot)
	if err != nil {
		return Project{}, err
	}
	p, ok := findProject(m, ref)
	if !ok {
		return Project{}, fmt.Errorf("project %q not found", ref)
	}
	if p.Type != ProjectTypeGit || p.Remote == "" {
		return Project{}, fmt.Errorf("cannot hydrate %s: project has no Git remote", p.Path)
	}
	if err := validateProjectRemote(p.Remote); err != nil {
		return Project{}, err
	}
	full, _, err := safeWorkspacePath(cfg.WorkspaceRoot, p.Path)
	if err != nil {
		return Project{}, err
	}
	if exists(full) && !isEmptyDir(full) {
		return Project{}, fmt.Errorf("cannot hydrate %s: destination folder is non-empty; no files were changed", p.Path)
	}
	parent := filepath.Dir(full)
	if err := os.MkdirAll(parent, 0o750); err != nil {
		return Project{}, err
	}
	// Clone into a sibling temp dir first so a failed or interrupted clone
	// cannot leave partial contents at the destination and block a retry.
	tmp, err := os.MkdirTemp(parent, ".devspace-hydrate-*")
	if err != nil {
		return Project{}, err
	}
	defer os.RemoveAll(tmp)
	if err := cloneRepo(p.Remote, tmp); err != nil {
		return Project{}, fmt.Errorf("cannot hydrate %s.\n\nReason:\n%w\n\nRemote:\n%s", p.Path, err, redactRemote(p.Remote))
	}
	if exists(full) {
		// Verified empty above; remove the empty placeholder so Rename succeeds.
		if err := os.Remove(full); err != nil {
			return Project{}, err
		}
	}
	if err := os.Rename(tmp, full); err != nil {
		return Project{}, err
	}
	if err := refreshAllProjectState(cfg.WorkspaceRoot); err != nil {
		return Project{}, err
	}
	return p, nil
}

func UpdateProjects(ref string, all bool) (ProjectUpdateReport, error) {
	ref = strings.TrimSpace(ref)
	if all && ref != "" {
		return ProjectUpdateReport{}, fmt.Errorf("use either --all or <project>, not both")
	}
	if !all && ref == "" {
		return ProjectUpdateReport{}, fmt.Errorf("update requires <project> or --all")
	}
	cfg, err := LoadConfig()
	if err != nil {
		return ProjectUpdateReport{}, err
	}
	m, err := LoadManifest(cfg.WorkspaceRoot)
	if err != nil {
		return ProjectUpdateReport{}, err
	}
	projects := m.Projects
	if !all {
		p, ok := findProject(m, ref)
		if !ok {
			return ProjectUpdateReport{}, fmt.Errorf("project %q not found", ref)
		}
		projects = []Project{p}
	}

	report := ProjectUpdateReport{}
	failures := 0
	for _, p := range projects {
		result := updateProject(cfg.WorkspaceRoot, p)
		if result.Status == "failed" {
			failures++
		}
		report.Results = append(report.Results, result)
	}
	if _, err := ScanWorkspace(); err != nil {
		return report, err
	}
	if failures > 0 {
		return report, fmt.Errorf("project update failed for %d project(s)", failures)
	}
	return report, nil
}

func updateProject(workspace string, p Project) ProjectUpdateResult {
	if p.Type != ProjectTypeGit {
		return ProjectUpdateResult{Project: p, Action: "skip", Status: "skipped", Reason: "not a Git project"}
	}
	if p.Remote == "" {
		return ProjectUpdateResult{Project: p, Action: "skip", Status: "skipped", Reason: "project has no Git remote"}
	}
	if err := validateProjectRemote(p.Remote); err != nil {
		return ProjectUpdateResult{Project: p, Action: "skip", Status: "skipped", Reason: err.Error()}
	}
	full, _, err := safeWorkspacePath(workspace, p.Path)
	if err != nil {
		return ProjectUpdateResult{Project: p, Action: "skip", Status: "skipped", Reason: err.Error()}
	}
	if !exists(full) || isEmptyDir(full) {
		if _, err := HydrateProject(p.ID); err != nil {
			return ProjectUpdateResult{Project: p, Action: "hydrate", Status: "failed", Reason: err.Error()}
		}
		return ProjectUpdateResult{Project: p, Action: "hydrate", Status: "updated"}
	}
	info := gitInfo(full)
	if !info.IsRepo {
		return ProjectUpdateResult{Project: p, Action: "skip", Status: "skipped", Reason: "destination is non-empty and is not a Git checkout"}
	}
	if info.Dirty {
		return ProjectUpdateResult{Project: p, Action: "skip", Status: "skipped", Reason: "dirty working tree"}
	}
	if info.DetachedHead {
		return ProjectUpdateResult{Project: p, Action: "skip", Status: "skipped", Reason: "detached HEAD"}
	}
	if len(info.Remotes) == 0 {
		return ProjectUpdateResult{Project: p, Action: "skip", Status: "skipped", Reason: "checkout has no Git remote"}
	}
	if err := pullRepoFastForward(full); err != nil {
		return ProjectUpdateResult{Project: p, Action: "pull", Status: "failed", Reason: err.Error()}
	}
	return ProjectUpdateResult{Project: p, Action: "pull", Status: "updated"}
}

func refreshAllProjectState(workspace string) error {
	m, err := LoadManifest(workspace)
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
	for _, p := range m.Projects {
		full, _, err := safeWorkspacePath(workspace, p.Path)
		if err != nil {
			return err
		}
		st.Projects[p.ID] = stateForProject(full, p, gitInfo(full))
	}
	st.LastSyncAt = nowRFC3339()
	return SaveState(st)
}

func findProject(m Manifest, ref string) (Project, bool) {
	for _, p := range m.Projects {
		if p.ID == ref || p.Name == ref || p.Path == ref {
			return p, true
		}
	}
	return Project{}, false
}

func ignoredName(name string) bool {
	if slices.Contains(DefaultIgnores, name) {
		return true
	}
	// Leftover hydration temp dirs (see HydrateProject) must never be scanned
	// as real projects if a hydrate was interrupted mid-clone.
	if strings.HasPrefix(name, ".devspace-hydrate-") || strings.HasPrefix(name, ".devdrop-hydrate-") {
		return true
	}
	return strings.HasSuffix(name, ".log")
}

func isEmptyDir(path string) bool {
	entries, err := os.ReadDir(path)
	return err == nil && len(entries) == 0
}
