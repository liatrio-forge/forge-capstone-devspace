package devdrop

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

type SyncAction struct {
	Kind string
	Path string
	Note string
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
	err = filepath.WalkDir(cfg.WorkspaceRoot, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if !d.IsDir() {
			return nil
		}
		if path == cfg.WorkspaceRoot {
			return nil
		}
		name := d.Name()
		if name == ".git" || name == ".devdrop" || ignoredName(name) {
			return filepath.SkipDir
		}
		rel, err := filepath.Rel(cfg.WorkspaceRoot, path)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		info := gitInfo(path)
		hasMarker := info.IsRepo || hasDependencyMarker(path) || exists(filepath.Join(path, ".env"))
		if !hasMarker {
			return nil
		}
		p := projectFromPath(rel, path, info)
		upsertProject(&m, p)
		st.Projects[p.ID] = stateForProject(path, p, info)
		seen[p.ID] = true
		summary.FoundProjects++
		if info.IsRepo {
			summary.GitRepos++
		} else {
			summary.LocalOnlyProjects++
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
		full := filepath.Join(cfg.WorkspaceRoot, filepath.FromSlash(p.Path))
		info := gitInfo(full)
		st.Projects[p.ID] = stateForProject(full, p, info)
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

func stateForProject(abs string, p Project, info GitInfo) ProjectState {
	_, err := os.Stat(abs)
	existsOnDisk := err == nil
	placeholder := exists(filepath.Join(abs, ".devdrop-placeholder.json"))
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
	clean, err := cleanRel(rel)
	if err != nil {
		return Project{}, err
	}
	full := filepath.Join(cfg.WorkspaceRoot, filepath.FromSlash(clean))
	info := gitInfo(full)
	if !exists(full) {
		if err := os.MkdirAll(full, 0o755); err != nil {
			return Project{}, err
		}
	}
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

func PlanSync() ([]SyncAction, error) {
	cfg, err := LoadConfig()
	if err != nil {
		return nil, err
	}
	m, err := LoadManifest(cfg.WorkspaceRoot)
	if err != nil {
		return nil, err
	}
	var actions []SyncAction
	for _, p := range m.Projects {
		full := filepath.Join(cfg.WorkspaceRoot, filepath.FromSlash(p.Path))
		if !exists(full) {
			actions = append(actions, SyncAction{Kind: "create-placeholder", Path: p.Path, Note: p.Name})
			continue
		}
		if p.Type == ProjectTypeGit {
			info := gitInfo(full)
			if info.IsRepo && p.Remote != "" && info.Remote != "" && info.Remote != p.Remote {
				actions = append(actions, SyncAction{Kind: "conflict", Path: p.Path, Note: fmt.Sprintf("manifest remote %s != local remote %s", p.Remote, info.Remote)})
			}
		}
	}
	return actions, nil
}

func ApplySync() ([]SyncAction, error) {
	actions, err := PlanSync()
	if err != nil {
		return nil, err
	}
	cfg, err := LoadConfig()
	if err != nil {
		return nil, err
	}
	for _, a := range actions {
		if a.Kind != "create-placeholder" {
			continue
		}
		full := filepath.Join(cfg.WorkspaceRoot, filepath.FromSlash(a.Path))
		if err := os.MkdirAll(full, 0o755); err != nil {
			return actions, err
		}
		placeholder := map[string]string{
			"path":      a.Path,
			"createdAt": nowRFC3339(),
			"note":      "DevDrop placeholder. Run `devdrop project hydrate " + a.Note + "` to clone contents.",
		}
		if err := writeJSON(filepath.Join(full, ".devdrop-placeholder.json"), placeholder, 0o644); err != nil {
			return actions, err
		}
	}
	st, err := LoadState()
	if err == nil {
		cfg, cfgErr := LoadConfig()
		if cfgErr == nil {
			if m, mErr := LoadManifest(cfg.WorkspaceRoot); mErr == nil {
				if st.Projects == nil {
					st.Projects = map[string]ProjectState{}
				}
				for _, p := range m.Projects {
					full := filepath.Join(cfg.WorkspaceRoot, filepath.FromSlash(p.Path))
					st.Projects[p.ID] = stateForProject(full, p, gitInfo(full))
				}
			}
		}
		st.LastSyncAt = nowRFC3339()
		_ = SaveState(st)
	}
	return actions, nil
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
		return Project{}, fmt.Errorf("project %s has no Git remote to hydrate", p.Name)
	}
	full := filepath.Join(cfg.WorkspaceRoot, filepath.FromSlash(p.Path))
	placeholder := filepath.Join(full, ".devdrop-placeholder.json")
	if exists(full) && !exists(placeholder) {
		return Project{}, fmt.Errorf("project path already exists and is not a placeholder: %s", p.Path)
	}
	if exists(placeholder) {
		if err := os.RemoveAll(full); err != nil {
			return Project{}, err
		}
	}
	if err := cloneRepo(p.Remote, full); err != nil {
		return Project{}, err
	}
	info := gitInfo(full)
	st, err := LoadState()
	if err == nil {
		st.Projects[p.ID] = stateForProject(full, p, info)
		_ = SaveState(st)
	}
	return p, nil
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
	return strings.HasSuffix(name, ".log")
}

func cleanRel(path string) (string, error) {
	if filepath.IsAbs(path) {
		return "", fmt.Errorf("project path must be relative")
	}
	clean := filepath.ToSlash(filepath.Clean(path))
	if clean == "." || strings.HasPrefix(clean, "../") || clean == ".." {
		return "", fmt.Errorf("project path must stay inside workspace")
	}
	return clean, nil
}
