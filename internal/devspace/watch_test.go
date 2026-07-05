package devspace

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
)

func TestWatchEventFilteringPreservesIgnoreRules(t *testing.T) {
	workspace := t.TempDir()
	if watchableDirectory(workspace, filepath.Join(workspace, "node_modules")) {
		t.Fatal("node_modules should not be watched")
	}
	if watchableDirectory(workspace, filepath.Join(workspace, ".devspace")) {
		t.Fatal(".devspace should not be watched")
	}
	if !watchableDirectory(workspace, filepath.Join(workspace, "apps", "api")) {
		t.Fatal("regular project directory should be watched")
	}
	if watchEventRelevant(workspace, fsnotify.Event{Name: filepath.Join(workspace, "apps", "api", "node_modules", "left-pad", "package.json"), Op: fsnotify.Write}) {
		t.Fatal("dependency-folder event should not refresh metadata")
	}
	if !watchEventRelevant(workspace, fsnotify.Event{Name: filepath.Join(workspace, "apps", "api", "package.json"), Op: fsnotify.Write}) {
		t.Fatal("package marker change should refresh metadata")
	}
	if !watchEventRelevant(workspace, fsnotify.Event{Name: filepath.Join(workspace, "apps", "api", ".git", "HEAD"), Op: fsnotify.Write}) {
		t.Fatal("Git HEAD change should refresh metadata")
	}
}

func TestWatchRegistryScopesToTrackedProjects(t *testing.T) {
	workspace := hardeningInitWorkspace(t, "code")
	tracked := filepath.Join(workspace, "apps", "api")
	if err := os.MkdirAll(filepath.Join(tracked, "pkg", "service"), 0o755); err != nil {
		t.Fatal(err)
	}
	untracked := filepath.Join(workspace, "family-events", "family-events-mobile", "ios", "SourcePackages", "checkouts", "deep")
	if err := os.MkdirAll(untracked, 0o755); err != nil {
		t.Fatal(err)
	}
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = watcher.Close()
	}()
	registry := newWatchRegistry(watcher, workspace)

	count, err := registry.sync([]string{"apps/api"})
	if err != nil {
		t.Fatal(err)
	}

	if count != len(registry.watched) {
		t.Fatalf("count = %d, watched entries = %d", count, len(registry.watched))
	}
	for _, path := range []string{
		workspace,
		filepath.Join(workspace, "apps"),
		tracked,
		filepath.Join(tracked, "pkg"),
		filepath.Join(tracked, "pkg", "service"),
	} {
		if !registry.watched[path] {
			t.Fatalf("expected %s to be watched", path)
		}
	}
	for _, path := range []string{
		filepath.Join(workspace, "family-events"),
		filepath.Join(workspace, "family-events", "family-events-mobile"),
		untracked,
	} {
		if registry.watched[path] {
			t.Fatalf("untracked sibling was watched: %s", path)
		}
	}
}

func TestWatchRegistryIgnoresGeneratedProjectTrees(t *testing.T) {
	workspace := hardeningInitWorkspace(t, "code")
	project := filepath.Join(workspace, "apps", "api")
	for _, dir := range []string{
		filepath.Join(project, "Sources", "App"),
		filepath.Join(project, ".build", "checkouts", "dependency"),
		filepath.Join(project, ".swiftpm", "configuration"),
		filepath.Join(project, "DerivedData", "Index"),
		filepath.Join(project, "Index.noindex", "DataStore"),
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = watcher.Close()
	}()
	registry := newWatchRegistry(watcher, workspace)

	if _, err := registry.sync([]string{"apps/api"}); err != nil {
		t.Fatal(err)
	}

	if !registry.watched[filepath.Join(project, "Sources", "App")] {
		t.Fatal("regular project source directory was not watched")
	}
	for _, path := range []string{
		filepath.Join(project, ".build"),
		filepath.Join(project, ".swiftpm"),
		filepath.Join(project, "DerivedData"),
		filepath.Join(project, "Index.noindex"),
	} {
		if registry.watched[path] {
			t.Fatalf("generated directory was watched: %s", path)
		}
	}
}

func TestRefreshWorkspaceForWatchDefaultsToLocalOnly(t *testing.T) {
	workspace := hardeningInitWorkspace(t, "code")
	if err := os.MkdirAll(filepath.Join(workspace, "apps", "api"), 0o755); err != nil {
		t.Fatal(err)
	}
	hardeningWriteFile(t, filepath.Join(workspace, "apps", "api", "package.json"), `{"scripts":{"dev":"vite"}}`, 0o644)

	result, err := RefreshWorkspaceForWatch("")
	if err != nil {
		t.Fatal(err)
	}
	if result.SyncMode != WatchSyncOff || result.SyncChanged {
		t.Fatalf("unexpected sync result: %+v", result)
	}
	if result.Summary.FoundProjects != 1 || result.Summary.LocalOnlyProjects != 1 {
		t.Fatalf("unexpected refresh summary: %+v", result.Summary)
	}
	if _, err := GetManifestRemote(); err == nil {
		t.Fatal("local-only watch refresh unexpectedly configured a Git remote")
	}
	if !result.FullScan {
		t.Fatal("RefreshWorkspaceForWatch should report a full scan")
	}
}

func TestRefreshProjectsForWatchOnlyTouchesChangedProject(t *testing.T) {
	workspace := hardeningInitWorkspace(t, "code")
	api := filepath.Join(workspace, "apps", "api")
	web := filepath.Join(workspace, "apps", "web")
	if err := os.MkdirAll(api, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(web, 0o755); err != nil {
		t.Fatal(err)
	}
	hardeningWriteFile(t, filepath.Join(api, "package.json"), `{"scripts":{"dev":"vite"}}`, 0o644)
	hardeningWriteFile(t, filepath.Join(web, "package.json"), `{"scripts":{"dev":"vite"}}`, 0o644)
	if _, err := ScanWorkspace(); err != nil {
		t.Fatal(err)
	}
	hardeningWriteFile(t, filepath.Join(api, ".env"), "TOKEN=api\n", 0o600)
	hardeningWriteFile(t, filepath.Join(web, ".env"), "TOKEN=web\n", 0o600)

	result, err := RefreshProjectsForWatch(WatchSyncOff, []string{"apps/api"})
	if err != nil {
		t.Fatal(err)
	}
	if result.FullScan {
		t.Fatal("RefreshProjectsForWatch should report a scoped refresh")
	}
	if result.Summary.FoundProjects != 1 || result.Summary.ProjectsWithEnv != 1 {
		t.Fatalf("unexpected scoped summary: %+v", result.Summary)
	}
	st, err := LoadState()
	if err != nil {
		t.Fatal(err)
	}
	m, err := LoadManifest(workspace)
	if err != nil {
		t.Fatal(err)
	}
	apiProject, ok := findProject(m, "apps/api")
	if !ok {
		t.Fatal("api project missing")
	}
	webProject, ok := findProject(m, "apps/web")
	if !ok {
		t.Fatal("web project missing")
	}
	if !st.Projects[apiProject.ID].EnvFilePresent {
		t.Fatal("changed project state was not refreshed")
	}
	if st.Projects[webProject.ID].EnvFilePresent {
		t.Fatal("unchanged project state should not be refreshed by scoped refresh")
	}
}

func TestWatchDiagnosticsAreLogfmtWhenPiped(t *testing.T) {
	clearColorEnv(t)
	resetStylesAfterTest(t)
	hardeningInitWorkspace(t, "code")
	var diagnostics bytes.Buffer

	err := WatchWorkspace(context.Background(), WatchOptions{
		SyncMode:   WatchSyncOff,
		RunInitial: true,
		Once:       true,
	}, &diagnostics)
	if err != nil {
		t.Fatalf("WatchWorkspace: %v", err)
	}

	out := diagnostics.String()
	if strings.ContainsRune(out, 0x1b) {
		t.Fatalf("expected logfmt diagnostics with no ANSI escape bytes for a non-terminal writer, got %q", out)
	}
	if !strings.Contains(out, "msg=\"watching workspace\"") {
		t.Fatalf("expected logfmt-formatted startup message, got %q", out)
	}
	if !strings.Contains(out, "msg=\"refreshed workspace metadata\"") {
		t.Fatalf("expected logfmt-formatted refresh message, got %q", out)
	}
	if !strings.Contains(out, "directories=") {
		t.Fatalf("expected watched directory count in diagnostics, got %q", out)
	}
}

func TestWatchProjectPathForEventMapsNestedPaths(t *testing.T) {
	workspace := t.TempDir()
	paths := []string{"apps/api", "apps/api-tools"}
	got, ok := watchProjectPathForEvent(workspace, filepath.Join(workspace, "apps", "api", "package.json"), paths)
	if !ok || got != "apps/api" {
		t.Fatalf("event mapped to %q/%v, want apps/api/true", got, ok)
	}
	if _, ok := watchProjectPathForEvent(workspace, filepath.Join(workspace, "apps", "new", "package.json"), paths); ok {
		t.Fatal("untracked project event should force a full scan")
	}
}

func TestWatchWorkspaceDebouncesFilesystemEvents(t *testing.T) {
	workspace := hardeningInitWorkspace(t, "code")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	refreshed := make(chan WatchRefresh, 4)
	errs := make(chan error, 1)
	go func() {
		errs <- WatchWorkspace(ctx, WatchOptions{
			Debounce: 40 * time.Millisecond,
			SyncMode: WatchSyncOff,
			OnRefresh: func(result WatchRefresh) {
				refreshed <- result
			},
		}, io.Discard)
	}()
	time.Sleep(100 * time.Millisecond)

	app := filepath.Join(workspace, "apps", "api")
	if err := os.MkdirAll(app, 0o755); err != nil {
		t.Fatal(err)
	}
	hardeningWriteFile(t, filepath.Join(app, "package.json"), `{"scripts":{"dev":"vite"}}`, 0o644)
	hardeningWriteFile(t, filepath.Join(app, "README.md"), "changed\n", 0o644)

	deadline := time.After(2 * time.Second)
	for {
		select {
		case result := <-refreshed:
			if result.Summary.FoundProjects == 1 {
				hardeningWriteFile(t, filepath.Join(app, ".env"), "TOKEN=local\n", 0o600)
				goto waitForSecondRefresh
			}
		case err := <-errs:
			t.Fatalf("watcher exited early: %v", err)
		case <-deadline:
			t.Fatal("timed out waiting for debounced watch refresh")
		}
	}

waitForSecondRefresh:
	deadline = time.After(2 * time.Second)
	for {
		select {
		case result := <-refreshed:
			if result.Summary.ProjectsWithEnv == 1 {
				cancel()
				select {
				case err := <-errs:
					if err != nil && err != context.Canceled {
						t.Fatal(err)
					}
				case <-time.After(time.Second):
					t.Fatal("watcher did not stop after cancellation")
				}
				return
			}
		case err := <-errs:
			t.Fatalf("watcher exited early: %v", err)
		case <-deadline:
			t.Fatal("timed out waiting for second debounced watch refresh")
		}
	}
}
