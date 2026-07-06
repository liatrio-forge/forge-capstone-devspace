package devspace

import (
	"os"
	"sort"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/fsnotify/fsnotify"
)

const dashboardWatchDebounce = 150 * time.Millisecond

var dashboardWatchReadyHook func()

// runLocked is the only dashboard lock boundary. withAppLock is non-reentrant:
// domain functions called here must not acquire it themselves.
func runLocked(op func() error) error {
	return withAppLock(op)
}

func dashboardScanCmd() tea.Cmd {
	return func() tea.Msg {
		var summary ScanSummary
		err := runLocked(func() error {
			var scanErr error
			summary, scanErr = ScanWorkspace()
			return scanErr
		})
		if err != nil {
			return scanLoadedMsg{err: err}
		}
		rows, err := dashboardRowsFromState()
		if err != nil {
			return scanLoadedMsg{err: err}
		}
		return scanLoadedMsg{rows: rows, summary: summary}
	}
}

func dashboardPlanCmd() tea.Cmd {
	return func() tea.Msg {
		var plan Plan
		err := runLocked(func() error {
			var planErr error
			plan, planErr = BuildPlan()
			if planErr != nil {
				return planErr
			}
			return SaveLastPlan(plan)
		})
		if err != nil {
			return actionResultMsg{label: "plan", err: err}
		}
		rows, summary, snapshotErr := dashboardSnapshotFromState()
		if snapshotErr != nil {
			return actionResultMsg{label: "plan", err: snapshotErr}
		}
		return actionResultMsg{label: "plan", rows: rows, summary: summary, plan: plan}
	}
}

func dashboardApplyCmd() tea.Cmd {
	return func() tea.Msg {
		var plan Plan
		err := runLocked(func() error {
			var applyErr error
			plan, applyErr = ApplyLastPlan()
			return applyErr
		})
		if err != nil {
			return actionResultMsg{label: "apply-safe", err: err}
		}
		rows, summary, snapshotErr := dashboardSnapshotFromState()
		if snapshotErr != nil {
			return actionResultMsg{label: "apply-safe", err: snapshotErr}
		}
		return actionResultMsg{label: "apply-safe", rows: rows, summary: summary, plan: plan}
	}
}

func dashboardHydrateCmd(ref string) tea.Cmd {
	return func() tea.Msg {
		var project Project
		err := runLocked(func() error {
			cfg, err := LoadConfig()
			if err != nil {
				return err
			}
			if _, _, err := safeWorkspacePath(cfg.WorkspaceRoot, ref); err != nil {
				return err
			}
			var hydrateErr error
			project, hydrateErr = HydrateProject(ref)
			return hydrateErr
		})
		if err != nil {
			return actionResultMsg{label: "hydrate", err: err}
		}
		rows, summary, snapshotErr := dashboardSnapshotFromState()
		if snapshotErr != nil {
			return actionResultMsg{label: "hydrate", err: snapshotErr}
		}
		return actionResultMsg{label: "hydrate", rows: rows, summary: summary, project: project}
	}
}

func dashboardRefreshCmd(syncMode string) tea.Cmd {
	return func() tea.Msg {
		var refresh WatchRefresh
		err := runLocked(func() error {
			var refreshErr error
			refresh, refreshErr = RefreshWorkspaceForWatch(syncMode)
			return refreshErr
		})
		if err != nil {
			return actionResultMsg{label: "refresh", err: err}
		}
		rows, err := dashboardRowsFromState()
		if err != nil {
			return actionResultMsg{label: "refresh", err: err}
		}
		return actionResultMsg{label: "refresh", rows: rows, summary: refresh.Summary, refresh: refresh}
	}
}

func dashboardSyncStatusCmd() tea.Cmd {
	return func() tea.Msg {
		var status dashboardSyncStatus
		err := runLocked(func() error {
			cfg, err := LoadConfig()
			if err != nil {
				return err
			}
			status.Configured = cfg.ManifestRemote != "" || hostedSyncConfigured(cfg)
			st, err := LoadState()
			if err != nil && !missing(err) {
				return err
			}
			status.LastSyncAt = st.LastSyncAt
			plan, err := LoadReconcilePlan()
			if err == nil && plan.WorkspaceRoot == cfg.WorkspaceRoot {
				status.ReconcileSaved = true
				status.ConflictCount = len(plan.Conflicts)
			} else if err != nil && !missing(err) {
				return err
			}
			if !status.Configured {
				status.UnavailableReason = "remote not configured"
				return nil
			}
			if status.LastSyncAt == "" {
				status.LastSyncAt = baseManifestTimestamp()
			}
			if cfg.ManifestRemote == "" {
				status.GitDiffUnavailable = "unavailable-for-hosted"
				return nil
			}
			// DiffWorkspaceManifest can hold the dashboard lock for up to about a minute against a slow remote.
			diff, err := DiffWorkspaceManifest()
			if err != nil {
				return err
			}
			status.DiffAdded = len(diff.Added)
			status.DiffRemoved = len(diff.Removed)
			status.DiffChanged = len(diff.Changed)
			status.LocalDiffers = status.DiffAdded+status.DiffRemoved+status.DiffChanged > 0
			return nil
		})
		if err != nil {
			status.Configured = true
			status.UnavailableReason = err.Error()
		}
		return syncStatusLoadedMsg{status: status}
	}
}

func hostedSyncConfigured(cfg Config) bool {
	return strings.TrimSpace(cfg.HostedSyncEndpoint) != ""
}

func baseManifestTimestamp() string {
	path, err := baseManifestPath()
	if err != nil {
		return ""
	}
	info, err := os.Stat(path)
	if err != nil {
		return ""
	}
	return info.ModTime().UTC().Format(time.RFC3339)
}

func dashboardWatchCmd(syncMode string) tea.Cmd {
	return func() tea.Msg {
		mode, err := normalizeWatchSyncMode(syncMode)
		if err != nil {
			return watchRefreshMsg{err: err}
		}
		cfg, err := LoadConfig()
		if err != nil {
			return watchRefreshMsg{err: err}
		}
		watcher, err := fsnotify.NewWatcher()
		if err != nil {
			return watchRefreshMsg{err: err}
		}
		defer func() { _ = watcher.Close() }()

		projectPaths, err := watchProjectPaths(cfg.WorkspaceRoot)
		if err != nil {
			return watchRefreshMsg{err: err}
		}
		registry := newWatchRegistry(watcher, cfg.WorkspaceRoot)
		watched, err := registry.sync(projectPaths)
		if err != nil {
			return watchRefreshMsg{err: err}
		}
		if dashboardWatchReadyHook != nil {
			dashboardWatchReadyHook()
		}

		timer := time.NewTimer(24 * time.Hour)
		if !timer.Stop() {
			<-timer.C
		}
		timerC := (<-chan time.Time)(nil)
		pending := map[string]bool{}
		fullScan := false
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return nil
				}
				if !watchEventRelevant(cfg.WorkspaceRoot, event) {
					continue
				}
				if projectPath, ok := watchProjectPathForEvent(cfg.WorkspaceRoot, event.Name, projectPaths); ok {
					if event.Op&fsnotify.Create != 0 {
						var addErr error
						watched, addErr = registry.addCreatedDir(event.Name)
						if addErr != nil {
							return watchRefreshMsg{err: addErr}
						}
					}
					pending[projectPath] = true
				} else {
					fullScan = true
				}
				if !timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}
				timer.Reset(dashboardWatchDebounce)
				timerC = timer.C
			case err, ok := <-watcher.Errors:
				if !ok {
					return nil
				}
				return watchRefreshMsg{err: err}
			case <-timerC:
				changed := make([]string, 0, len(pending))
				for projectPath := range pending {
					changed = append(changed, projectPath)
					delete(pending, projectPath)
				}
				sort.Strings(changed)
				return dashboardWatchRefresh(mode, fullScan, changed, watched)
			}
		}
	}
}

func dashboardWatchRefresh(syncMode string, fullScan bool, changed []string, watched int) tea.Msg {
	var refresh WatchRefresh
	err := runLocked(func() error {
		var refreshErr error
		if fullScan || len(changed) == 0 {
			refresh, refreshErr = RefreshWorkspaceForWatch(syncMode)
		} else {
			refresh, refreshErr = RefreshProjectsForWatch(syncMode, changed)
		}
		if refreshErr != nil {
			return refreshErr
		}
		refresh.WatchedDirCount = watched
		return nil
	})
	if err != nil {
		return watchRefreshMsg{err: err}
	}
	rows, err := dashboardRowsFromState()
	if err != nil {
		return watchRefreshMsg{err: err}
	}
	return watchRefreshMsg{refresh: refresh, rows: rows, summary: refresh.Summary}
}

func dashboardSnapshotFromState() ([]dashboardRow, ScanSummary, error) {
	rows, err := dashboardRowsFromState()
	if err != nil {
		return nil, ScanSummary{}, err
	}
	return rows, summaryFromRows(rows), nil
}
