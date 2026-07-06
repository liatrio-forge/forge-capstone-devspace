package devspace

import (
	"bytes"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"github.com/fsnotify/fsnotify"
)

func TestDashboardInitialModelRendersScan(t *testing.T) {
	workspace := dashboardSeedWorkspace(t)
	summary, err := ScanWorkspace()
	if err != nil {
		t.Fatalf("ScanWorkspace: %v", err)
	}
	rows, err := dashboardRowsFromState()
	if err != nil {
		t.Fatalf("dashboardRowsFromState: %v", err)
	}

	model := newDashboardModel(true)
	updated, cmd := model.Update(scanLoadedMsg{rows: rows, summary: summary})
	if cmd == nil {
		t.Fatal("scanLoadedMsg returned nil command")
	}
	got := updated.(dashboardModel)
	if len(got.rows) != 3 {
		t.Fatalf("rows = %d, want 3", len(got.rows))
	}
	assertDashboardRow(t, got.rows, "api", dashboardStatusHydrated, true, "main", true)
	assertDashboardRow(t, got.rows, "worker", dashboardStatusHydrated, false, "", false)
	assertDashboardRow(t, got.rows, "missing", dashboardStatusMissing, false, "", false)
	if got.summary.FoundProjects != 2 || got.summary.GitRepos != 1 || got.summary.LocalOnlyProjects != 1 || got.summary.ProjectsWithEnv != 1 {
		t.Fatalf("summary = %+v", got.summary)
	}
	if got.workspaceRoot != workspace {
		t.Fatalf("workspaceRoot = %q, want %q", got.workspaceRoot, workspace)
	}
}

func TestDashboardSyncStatusRendersRemoteState(t *testing.T) {
	dashboardSeedWorkspace(t)
	model := newDashboardModel(true)
	model.syncStatus = dashboardSyncStatus{
		Configured:     true,
		LastSyncAt:     "2026-07-06T12:00:00Z",
		LocalDiffers:   true,
		DiffAdded:      1,
		DiffRemoved:    2,
		DiffChanged:    3,
		ConflictCount:  4,
		ReconcileSaved: true,
	}

	content := model.View().Content
	for _, want := range []string{
		"Sync Status",
		"Last sync/base: 2026-07-06T12:00:00Z",
		"Local differs from remote: yes",
		"Remote diff: added=1 removed=2 changed=3",
		"Reconcile conflicts: 4",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("view missing %q:\n%s", want, content)
		}
	}
}

func TestDashboardSyncStatusRendersDegradedStates(t *testing.T) {
	model := dashboardModel{syncStatus: dashboardSyncStatus{UnavailableReason: "no manifest remote configured"}}
	if content := model.renderSyncStatus(); !strings.Contains(content, "remote not configured") {
		t.Fatalf("content = %q, want remote not configured", content)
	}

	model.syncStatus = dashboardSyncStatus{Configured: true, UnavailableReason: "boom"}
	if content := model.renderSyncStatus(); !strings.Contains(content, "status unavailable: boom") {
		t.Fatalf("content = %q, want unavailable reason", content)
	}
}

func TestDashboardSyncStatusMessageUpdatesModel(t *testing.T) {
	model := dashboardModel{}
	status := dashboardSyncStatus{Configured: true, LastSyncAt: "2026-07-06T12:00:00Z"}
	updated, cmd := model.Update(syncStatusLoadedMsg{status: status})
	if cmd != nil {
		t.Fatal("sync status message returned command")
	}
	got := updated.(dashboardModel)
	if got.syncStatus.LastSyncAt != status.LastSyncAt || !got.syncStatus.Configured {
		t.Fatalf("syncStatus = %+v, want %+v", got.syncStatus, status)
	}
}

func TestDashboardSyncStatusCmdUsesDiffAndSavedReconcilePlan(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "home")
	workspace := filepath.Join(root, "code")
	remote := workspaceSyncBareRepo(t)
	t.Setenv(envHome, home)
	if _, err := InitWorkspace(workspace); err != nil {
		t.Fatal(err)
	}
	if _, err := SetManifestRemote(remote); err != nil {
		t.Fatal(err)
	}
	project := hardeningProject("apps/app", ProjectTypeLocal, "")
	manifest := Manifest{Version: ManifestVersion, WorkspaceRoot: workspace, Projects: []Project{project}}
	if err := SaveManifest(workspace, manifest); err != nil {
		t.Fatal(err)
	}
	if _, err := PushWorkspaceManifest(); err != nil {
		t.Fatal(err)
	}
	manifest.Projects[0].Name = "local-app"
	if err := SaveManifest(workspace, manifest); err != nil {
		t.Fatal(err)
	}
	if err := SaveReconcilePlan(ReconcilePlan{
		Version:       1,
		WorkspaceRoot: workspace,
		Conflicts: []MergeConflict{
			{Entity: "project", Key: "apps/app", Field: "*"},
			{Entity: "project", Key: "apps/worker", Field: "*"},
		},
	}); err != nil {
		t.Fatal(err)
	}

	msg := dashboardSyncStatusCmd()()
	loaded, ok := msg.(syncStatusLoadedMsg)
	if !ok {
		t.Fatalf("msg = %T, want syncStatusLoadedMsg", msg)
	}
	status := loaded.status
	if !status.Configured || !status.LocalDiffers || status.DiffChanged != 1 {
		t.Fatalf("status diff = %+v, want configured changed diff", status)
	}
	if status.ConflictCount != 2 || !status.ReconcileSaved {
		t.Fatalf("status conflicts = %+v, want saved conflicts", status)
	}
	if status.LastSyncAt == "" {
		t.Fatalf("status LastSyncAt missing: %+v", status)
	}
}

func TestDashboardSyncStatusCmdUsesHostedStateAndSavedPlan(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "home")
	workspace := filepath.Join(root, "code")
	t.Setenv(envHome, home)
	if _, err := InitWorkspace(workspace); err != nil {
		t.Fatal(err)
	}
	if _, err := SetHostedSync("http://127.0.0.1:8787", "test-token", "team-a"); err != nil {
		t.Fatal(err)
	}
	if err := SaveState(State{
		WorkspaceRoot: workspace,
		LastSyncAt:    "2026-07-06T12:00:00Z",
	}); err != nil {
		t.Fatal(err)
	}
	if err := SaveReconcilePlan(ReconcilePlan{
		Version:       1,
		WorkspaceRoot: workspace,
		Conflicts: []MergeConflict{
			{Entity: "project", Key: "apps/app", Field: "*"},
		},
	}); err != nil {
		t.Fatal(err)
	}

	msg := dashboardSyncStatusCmd()()
	loaded, ok := msg.(syncStatusLoadedMsg)
	if !ok {
		t.Fatalf("msg = %T, want syncStatusLoadedMsg", msg)
	}
	status := loaded.status
	if !status.Configured || status.UnavailableReason != "" {
		t.Fatalf("status availability = %+v, want hosted configured", status)
	}
	if status.LastSyncAt != "2026-07-06T12:00:00Z" || !status.ReconcileSaved || status.ConflictCount != 1 {
		t.Fatalf("status state = %+v, want hosted last sync and saved conflict", status)
	}
	if status.GitDiffUnavailable != "unavailable-for-hosted" {
		t.Fatalf("GitDiffUnavailable = %q, want unavailable-for-hosted", status.GitDiffUnavailable)
	}
	if content := (dashboardModel{syncStatus: status}).renderSyncStatus(); !strings.Contains(content, "Remote diff: unavailable-for-hosted") {
		t.Fatalf("content = %q, want hosted diff unavailable", content)
	}
}

func TestDashboardRefreshesSyncStatusAfterUpdates(t *testing.T) {
	t.Setenv(envHome, t.TempDir())
	if _, err := InitWorkspace(t.TempDir()); err != nil {
		t.Fatal(err)
	}

	model := dashboardModel{}
	updated, cmd := model.Update(scanLoadedMsg{})
	if cmd == nil {
		t.Fatal("scanLoadedMsg returned nil command")
	}
	msg := cmd()
	if _, ok := msg.(syncStatusLoadedMsg); !ok {
		t.Fatalf("scanLoadedMsg command = %T, want syncStatusLoadedMsg", msg)
	}

	_, cmd = updated.(dashboardModel).Update(actionResultMsg{label: "refresh"})
	if cmd == nil {
		t.Fatal("actionResultMsg returned nil command")
	}
	msg = cmd()
	if _, ok := msg.(syncStatusLoadedMsg); !ok {
		t.Fatalf("actionResultMsg command = %T, want syncStatusLoadedMsg", msg)
	}
}

func TestDashboardNavigateAndQuit(t *testing.T) {
	model := dashboardModel{rows: []dashboardRow{{name: "one"}, {name: "two"}}, noWatch: true}

	updated, _ := model.Update(keyMsg("j"))
	got := updated.(dashboardModel)
	if got.selected != 1 {
		t.Fatalf("selected after j = %d, want 1", got.selected)
	}
	updated, _ = got.Update(keyMsg("down"))
	got = updated.(dashboardModel)
	if got.selected != 1 {
		t.Fatalf("selected after clamped down = %d, want 1", got.selected)
	}
	updated, _ = got.Update(keyMsg("k"))
	got = updated.(dashboardModel)
	if got.selected != 0 {
		t.Fatalf("selected after k = %d, want 0", got.selected)
	}
	updated, _ = got.Update(keyMsg("up"))
	got = updated.(dashboardModel)
	if got.selected != 0 {
		t.Fatalf("selected after clamped up = %d, want 0", got.selected)
	}
	if _, cmd := got.Update(keyMsg("q")); cmd == nil {
		t.Fatal("q returned nil command")
	}
	if _, cmd := got.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}); cmd == nil {
		t.Fatal("ctrl+c returned nil command")
	}
}

func TestDashboardActionTransitions(t *testing.T) {
	workspace := hardeningInitWorkspace(t, "code")
	if err := SaveManifest(workspace, Manifest{
		Version:       ManifestVersion,
		WorkspaceRoot: workspace,
		Projects: []Project{
			hardeningProject("services/api", ProjectTypeGit, "https://example.invalid/api.git"),
		},
	}); err != nil {
		t.Fatalf("SaveManifest: %v", err)
	}
	plan, err := BuildPlan()
	if err != nil {
		t.Fatalf("BuildPlan: %v", err)
	}
	if err := SaveLastPlan(plan); err != nil {
		t.Fatalf("SaveLastPlan: %v", err)
	}

	model := newDashboardModel(true)
	updated, cmd := model.Update(keyMsg("a"))
	got := updated.(dashboardModel)
	if !got.busy {
		t.Fatal("action did not set busy")
	}
	if cmd == nil {
		t.Fatal("action returned nil command")
	}
	msg := cmd()
	result, ok := msg.(actionResultMsg)
	if !ok {
		t.Fatalf("cmd msg = %T, want actionResultMsg", msg)
	}
	if result.err != nil {
		t.Fatalf("action error: %v", result.err)
	}
	updated, _ = got.Update(result)
	got = updated.(dashboardModel)
	if got.busy {
		t.Fatal("result did not clear busy")
	}
	if !exists(filepath.Join(workspace, "services", "api")) {
		t.Fatal("apply-safe did not create placeholder")
	}
	if len(result.rows) != 1 || result.rows[0].status != dashboardStatusPlaceholder {
		t.Fatalf("rows after apply = %+v", result.rows)
	}
	if len(result.plan.Actions) != len(plan.Actions) {
		t.Fatalf("applied actions = %d, want %d", len(result.plan.Actions), len(plan.Actions))
	}
}

func TestDashboardSingleFlightRejectsConcurrent(t *testing.T) {
	model := dashboardModel{busy: true, rows: []dashboardRow{{name: "one", ref: "one"}}, noWatch: true}
	updated, cmd := model.Update(keyMsg("s"))
	got := updated.(dashboardModel)
	if cmd != nil {
		t.Fatal("busy scan returned command")
	}
	if got.errText == "" || !strings.Contains(got.errText, "busy") {
		t.Fatalf("errText = %q, want busy hint", got.errText)
	}
}

func TestDashboardActionError(t *testing.T) {
	model := dashboardModel{busy: true}
	updated, cmd := model.Update(errMsg{err: errors.New("boom")})
	got := updated.(dashboardModel)
	if cmd != nil {
		t.Fatal("errMsg returned command")
	}
	if got.busy || got.errText != "boom" {
		t.Fatalf("busy=%v errText=%q", got.busy, got.errText)
	}

	updated, _ = model.Update(actionResultMsg{label: "apply", err: errors.New("failed")})
	got = updated.(dashboardModel)
	if got.busy || got.errText != "failed" {
		t.Fatalf("busy=%v errText=%q", got.busy, got.errText)
	}
}

func TestDashboardWatchRefreshUpdatesModel(t *testing.T) {
	model := dashboardModel{rows: []dashboardRow{{name: "old"}}, summary: ScanSummary{FoundProjects: 1}}
	rows := []dashboardRow{{name: "new", status: dashboardStatusHydrated}}
	refresh := WatchRefresh{
		Summary:          ScanSummary{FoundProjects: 2, GitRepos: 1},
		SyncMode:         WatchSyncOff,
		RefreshStartedAt: "2026-07-04T00:00:00Z",
		FullScan:         true,
		WatchedDirCount:  4,
	}
	updated, cmd := model.Update(watchRefreshMsg{refresh: refresh, rows: rows, summary: refresh.Summary})
	got := updated.(dashboardModel)
	if cmd == nil {
		t.Fatal("watch refresh did not reschedule watcher")
	}
	if len(got.rows) != 1 || got.rows[0].name != "new" {
		t.Fatalf("rows = %+v", got.rows)
	}
	if got.summary.FoundProjects != 2 || len(got.events) != 1 {
		t.Fatalf("summary=%+v events=%+v", got.summary, got.events)
	}
}

func TestDashboardManualRefreshKey(t *testing.T) {
	for _, noWatch := range []bool{false, true} {
		model := dashboardModel{noWatch: noWatch}
		_, cmd := model.Update(keyMsg("r"))
		if cmd == nil {
			t.Fatalf("r with noWatch=%v returned nil command", noWatch)
		}
	}
}

func TestDashboardNoWatchDisablesWatcher(t *testing.T) {
	started := false
	model := newDashboardModel(true)
	model.watchCmdFactory = func(string) tea.Cmd {
		started = true
		return nil
	}
	cmd := model.Init()
	if cmd == nil {
		t.Fatal("Init returned nil command")
	}
	if started {
		t.Fatal("watch command started with noWatch=true")
	}
}

func TestDashboardHeadlessRenderSmoke(t *testing.T) {
	dashboardSeedWorkspace(t)
	model := newDashboardModel(true)
	model.rows = []dashboardRow{{name: "api", typ: ProjectTypeGit, status: dashboardStatusHydrated, branch: "main", env: true}}
	model.summary = ScanSummary{FoundProjects: 1, GitRepos: 1, ProjectsWithEnv: 1}

	// Render path: View() is the real render; assert its content deterministically
	// (raw program output is alt-screen ANSI whose first frame races the quit key).
	content := model.View().Content
	if !strings.Contains(content, "DevSpace UI") || !strings.Contains(content, "api") {
		t.Fatalf("view content = %q", content)
	}

	// Lifecycle path: the program boots and exits cleanly headlessly.
	var out bytes.Buffer
	program := tea.NewProgram(model, tea.WithInput(bytes.NewReader([]byte("q"))), tea.WithOutput(&out))
	if _, err := program.Run(); err != nil {
		t.Fatalf("program.Run: %v", err)
	}
	if out.Len() == 0 {
		t.Fatal("program produced no output")
	}
}

func TestDashboardWatcherEmitsRefreshOnFileChange(t *testing.T) {
	workspace := dashboardSeedWorkspace(t)
	if _, err := fsnotifyNewWatcherForDashboardTest(); err != nil {
		t.Skipf("fsnotify unavailable: %v", err)
	}

	ready := make(chan struct{})
	previousReady := dashboardWatchReadyHook
	dashboardWatchReadyHook = func() { close(ready) }
	t.Cleanup(func() { dashboardWatchReadyHook = previousReady })

	cmd := dashboardWatchCmd(WatchSyncOff)
	results := make(chan tea.Msg, 1)
	go func() {
		results <- cmd()
	}()

	select {
	case <-ready:
	case <-time.After(2 * time.Second):
		t.Fatal("watcher did not become ready")
	}
	// Write directly under the watched workspace root (collectScopedTargets always
	// watches the root); a nested write under an untracked dir would not fire.
	hardeningWriteFile(t, filepath.Join(workspace, "trigger.txt"), "changed\n", 0o644)

	select {
	case msg := <-results:
		refresh, ok := msg.(watchRefreshMsg)
		if !ok {
			t.Fatalf("msg = %T, want watchRefreshMsg", msg)
		}
		if refresh.err != nil {
			t.Fatalf("watch refresh error: %v", refresh.err)
		}
		if refresh.refresh.Summary.FoundProjects == 0 {
			t.Fatalf("refresh = %+v", refresh.refresh)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("watcher did not emit refresh")
	}
}

func TestDashboardTruncateCellRuneSafe(t *testing.T) {
	// Multibyte project names must not be sliced mid-rune (would fail with the
	// old byte-based value[:max] implementation).
	got := truncateCell("日本語プロジェクト名", 5)
	if !utf8.ValidString(got) {
		t.Fatalf("truncateCell produced invalid UTF-8: %q", got)
	}
	if n := utf8.RuneCountInString(got); n != 5 {
		t.Fatalf("rune count = %d, want 5 (got %q)", n, got)
	}
	if !strings.HasSuffix(got, "...") {
		t.Fatalf("expected ellipsis suffix, got %q", got)
	}
	if truncateCell("api", 32) != "api" {
		t.Fatal("value shorter than max should be unchanged")
	}
}

func dashboardSeedWorkspace(t *testing.T) string {
	t.Helper()
	workspace := hardeningInitWorkspace(t, "code")
	hardeningGitRepo(t, filepath.Join(workspace, "apps", "api"))
	hardeningWriteFile(t, filepath.Join(workspace, "apps", "api", ".env"), "TOKEN=x\n", 0o600)
	hardeningWriteFile(t, filepath.Join(workspace, "apps", "api", "dirty.txt"), "dirty\n", 0o644)
	hardeningWriteFile(t, filepath.Join(workspace, "services", "worker", "package.json"), "{}\n", 0o644)
	if err := SaveManifest(workspace, Manifest{
		Version:       ManifestVersion,
		WorkspaceRoot: workspace,
		Projects: []Project{
			hardeningProject("apps/missing", ProjectTypeGit, "https://example.invalid/missing.git"),
		},
	}); err != nil {
		t.Fatalf("SaveManifest: %v", err)
	}
	return workspace
}

func assertDashboardRow(t *testing.T, rows []dashboardRow, name, status string, dirty bool, branch string, env bool) {
	t.Helper()
	for _, row := range rows {
		if row.name != name {
			continue
		}
		if row.status != status || row.dirty != dirty || row.branch != branch || row.env != env {
			t.Fatalf("row %s = %+v", name, row)
		}
		return
	}
	t.Fatalf("missing row %q in %+v", name, rows)
}

func keyMsg(key string) tea.KeyMsg {
	switch key {
	case "up":
		return tea.KeyPressMsg{Code: tea.KeyUp}
	case "down":
		return tea.KeyPressMsg{Code: tea.KeyDown}
	default:
		return tea.KeyPressMsg{Code: []rune(key)[0], Text: key}
	}
}

func fsnotifyNewWatcherForDashboardTest() (interface{ Close() error }, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	return watcher, watcher.Close()
}
