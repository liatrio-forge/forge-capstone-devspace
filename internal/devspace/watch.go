package devspace

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
)

const (
	WatchSyncOff    = "off"
	WatchSyncGit    = "git"
	WatchSyncHosted = "hosted"
)

type WatchOptions struct {
	Debounce   time.Duration
	SyncMode   string
	RunInitial bool
	Once       bool
	OnRefresh  func(WatchRefresh)
}

type WatchRefresh struct {
	Summary          ScanSummary
	SyncMode         string
	SyncChanged      bool
	HostedVersion    int
	HostedHash       string
	WatchedDirCount  int
	RefreshStartedAt string
	FullScan         bool
}

var (
	watchFullScanEvery       = 10
	watchFullScanMaxInterval = 5 * time.Minute
)

func RefreshWorkspaceForWatch(syncMode string) (WatchRefresh, error) {
	mode, err := normalizeWatchSyncMode(syncMode)
	if err != nil {
		return WatchRefresh{}, err
	}
	result := WatchRefresh{SyncMode: mode, RefreshStartedAt: nowRFC3339(), FullScan: true}
	summary, err := ScanWorkspace()
	if err != nil {
		return WatchRefresh{}, err
	}
	result.Summary = summary
	return result, syncWatchManifest(&result, mode)
}

func syncWatchManifest(result *WatchRefresh, mode string) error {
	switch mode {
	case WatchSyncOff:
		return nil
	case WatchSyncGit:
		changed, err := PushWorkspaceManifest()
		if err != nil {
			return err
		}
		result.SyncChanged = changed
		return nil
	case WatchSyncHosted:
		hosted, err := PushHostedManifest()
		if err != nil {
			return err
		}
		result.SyncChanged = hosted.Changed
		result.HostedVersion = hosted.Version
		result.HostedHash = hosted.ManifestHash
		return nil
	default:
		return fmt.Errorf("unsupported watch sync mode %q", mode)
	}
}

func WatchWorkspace(ctx context.Context, opts WatchOptions, out io.Writer) error {
	if out == nil {
		out = io.Discard
	}
	mode, err := normalizeWatchSyncMode(opts.SyncMode)
	if err != nil {
		return err
	}
	if opts.Debounce <= 0 {
		opts.Debounce = 2 * time.Second
	}
	cfg, err := LoadConfig()
	if err != nil {
		return err
	}
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer watcher.Close()

	watched, err := addWorkspaceWatches(watcher, cfg.WorkspaceRoot)
	if err != nil {
		return err
	}
	trackedProjects, _ := watchProjectPaths(cfg.WorkspaceRoot)
	fmt.Fprintf(out, "Watching %s (%d directories)\n", cfg.WorkspaceRoot, watched)
	fmt.Fprintf(out, "Debounce: %s\n", opts.Debounce)
	fmt.Fprintf(out, "Sync: %s\n", watchSyncDescription(mode))
	fmt.Fprintln(out, "Watch refreshes manifest/state only; it never pulls, applies plans, hydrates repositories, runs setup commands, or uploads secrets.")

	refreshCount := 0
	lastFullScan := time.Time{}
	runRefresh := func(forceFull bool, changed []string) error {
		return withAppLock(func() error {
			refreshCount++
			needsPeriodicCount := watchFullScanEvery > 0 && refreshCount%watchFullScanEvery == 0
			needsPeriodicTime := !lastFullScan.IsZero() && watchFullScanMaxInterval > 0 && time.Since(lastFullScan) >= watchFullScanMaxInterval
			full := forceFull || len(changed) == 0 || needsPeriodicCount || needsPeriodicTime
			var result WatchRefresh
			var err error
			if full {
				result, err = RefreshWorkspaceForWatch(mode)
				lastFullScan = time.Now()
			} else {
				result, err = RefreshProjectsForWatch(mode, changed)
			}
			if err != nil {
				return err
			}
			result.WatchedDirCount = watched
			if paths, err := watchProjectPaths(cfg.WorkspaceRoot); err == nil {
				trackedProjects = paths
			}
			if opts.OnRefresh != nil {
				opts.OnRefresh(result)
			}
			printWatchRefresh(out, result)
			return nil
		})
	}
	if opts.RunInitial || opts.Once {
		if err := runRefresh(true, nil); err != nil {
			return err
		}
		if opts.Once {
			return nil
		}
	}

	var timer *time.Timer
	timerC := (<-chan time.Time)(nil)
	defer func() {
		if timer != nil {
			timer.Stop()
		}
	}()
	pending := map[string]bool{}
	fullScan := false
	schedule := func() {
		if timer == nil {
			timer = time.NewTimer(opts.Debounce)
			timerC = timer.C
			return
		}
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
		timer.Reset(opts.Debounce)
		timerC = timer.C
	}

	for {
		select {
		case <-ctx.Done():
			if errors.Is(ctx.Err(), context.Canceled) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
				return ctx.Err()
			}
			return nil
		case event, ok := <-watcher.Events:
			if !ok {
				return nil
			}
			if event.Op&fsnotify.Create != 0 {
				added, err := addCreatedDirWatches(watcher, cfg.WorkspaceRoot, event.Name)
				if err != nil {
					return err
				}
				watched += added
			}
			if watchEventRelevant(cfg.WorkspaceRoot, event) {
				if projectPath, ok := watchProjectPathForEvent(cfg.WorkspaceRoot, event.Name, trackedProjects); ok {
					pending[projectPath] = true
				} else {
					fullScan = true
				}
				schedule()
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return nil
			}
			return err
		case <-timerC:
			timerC = nil
			changed := make([]string, 0, len(pending))
			for projectPath := range pending {
				changed = append(changed, projectPath)
			}
			sort.Strings(changed)
			for projectPath := range pending {
				delete(pending, projectPath)
			}
			forceFull := fullScan
			fullScan = false
			if err := runRefresh(forceFull, changed); err != nil {
				return err
			}
		}
	}
}

func watchProjectPaths(workspace string) ([]string, error) {
	m, err := LoadManifest(workspace)
	if err != nil {
		return nil, err
	}
	paths := make([]string, 0, len(m.Projects))
	for _, p := range m.Projects {
		paths = append(paths, filepath.ToSlash(filepath.Clean(p.Path)))
	}
	sort.Slice(paths, func(i, j int) bool {
		if len(paths[i]) == len(paths[j]) {
			return paths[i] < paths[j]
		}
		return len(paths[i]) > len(paths[j])
	})
	return paths, nil
}

func watchProjectPathForEvent(workspace, path string, projectPaths []string) (string, bool) {
	components, ok := workspaceRelativeComponents(workspace, path)
	if !ok || len(components) == 0 {
		return "", false
	}
	rel := filepath.ToSlash(filepath.Join(components...))
	for _, projectPath := range projectPaths {
		if rel == projectPath || strings.HasPrefix(rel+"/", projectPath+"/") {
			return projectPath, true
		}
	}
	return "", false
}

func addWorkspaceWatches(watcher *fsnotify.Watcher, workspace string) (int, error) {
	count := 0
	err := filepath.WalkDir(workspace, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !d.IsDir() {
			return nil
		}
		if !watchableDirectory(workspace, path) {
			return filepath.SkipDir
		}
		if err := watcher.Add(path); err != nil {
			return err
		}
		count++
		return nil
	})
	return count, err
}

func addCreatedDirWatches(watcher *fsnotify.Watcher, workspace, path string) (int, error) {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	if !info.IsDir() || !watchableDirectory(workspace, path) {
		return 0, nil
	}
	return addWorkspaceWatches(watcher, path)
}

func watchEventRelevant(workspace string, event fsnotify.Event) bool {
	if event.Op&(fsnotify.Create|fsnotify.Write|fsnotify.Remove|fsnotify.Rename) == 0 {
		return false
	}
	rel, ok := workspaceRelativeComponents(workspace, event.Name)
	if !ok || len(rel) == 0 {
		return false
	}
	if hasIgnoredWatchComponent(rel) {
		return false
	}
	for i, component := range rel {
		if component != ".git" {
			continue
		}
		gitRel := rel[i+1:]
		if len(gitRel) == 0 {
			return true
		}
		if len(gitRel) == 1 && (gitRel[0] == "HEAD" || gitRel[0] == "index" || gitRel[0] == "packed-refs" || gitRel[0] == "config") {
			return true
		}
		return len(gitRel) >= 3 && gitRel[0] == "refs" && (gitRel[1] == "heads" || gitRel[1] == "remotes")
	}
	return true
}

func watchableDirectory(workspace, path string) bool {
	rel, ok := workspaceRelativeComponents(workspace, path)
	if !ok {
		return false
	}
	if len(rel) == 0 {
		return true
	}
	if hasIgnoredWatchComponent(rel) {
		return false
	}
	for i, component := range rel {
		if component != ".git" {
			continue
		}
		gitRel := rel[i+1:]
		if len(gitRel) == 0 {
			return true
		}
		return len(gitRel) <= 2 && gitRel[0] == "refs" && (len(gitRel) == 1 || gitRel[1] == "heads" || gitRel[1] == "remotes")
	}
	return true
}

func hasIgnoredWatchComponent(components []string) bool {
	for _, component := range components {
		if component == workspaceDirName || component == legacyWorkspaceDirName || ignoredName(component) {
			return true
		}
	}
	return false
}

func workspaceRelativeComponents(workspace, path string) ([]string, bool) {
	rel, err := filepath.Rel(workspace, path)
	if err != nil {
		return nil, false
	}
	rel = filepath.ToSlash(filepath.Clean(rel))
	if rel == "." {
		return nil, true
	}
	if rel == ".." || strings.HasPrefix(rel, "../") {
		return nil, false
	}
	return strings.Split(rel, "/"), true
}

func normalizeWatchSyncMode(mode string) (string, error) {
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode == "" {
		mode = WatchSyncOff
	}
	switch mode {
	case WatchSyncOff, WatchSyncGit, WatchSyncHosted:
		return mode, nil
	default:
		return "", fmt.Errorf("unsupported watch sync mode %q; expected off, git, or hosted", mode)
	}
}

func watchSyncDescription(mode string) string {
	switch mode {
	case WatchSyncGit:
		return "push manifest to configured Git remote after each refresh"
	case WatchSyncHosted:
		return "push normalized manifest to configured hosted sync after each refresh"
	default:
		return "local manifest/state only"
	}
}

func printWatchRefresh(out io.Writer, result WatchRefresh) {
	scope := "scoped"
	if result.FullScan {
		scope = "full"
	}
	fmt.Fprintf(out, "Refreshed at %s (%s): found %d projects, %d Git repos, %d untracked folders, %d local-only projects, %d projects with env files.\n",
		result.RefreshStartedAt,
		scope,
		result.Summary.FoundProjects,
		result.Summary.GitRepos,
		result.Summary.UntrackedFolders,
		result.Summary.LocalOnlyProjects,
		result.Summary.ProjectsWithEnv,
	)
	switch result.SyncMode {
	case WatchSyncGit:
		if result.SyncChanged {
			fmt.Fprintln(out, "Git manifest sync: pushed changes.")
		} else {
			fmt.Fprintln(out, "Git manifest sync: already up to date.")
		}
	case WatchSyncHosted:
		if result.SyncChanged {
			fmt.Fprintf(out, "Hosted manifest sync: pushed version %d.\n", result.HostedVersion)
		} else {
			fmt.Fprintf(out, "Hosted manifest sync: already up to date at version %d.\n", result.HostedVersion)
		}
	}
}
