package devdrop

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
)

func TestWatchEventFilteringPreservesIgnoreRules(t *testing.T) {
	workspace := t.TempDir()
	if watchableDirectory(workspace, filepath.Join(workspace, "node_modules")) {
		t.Fatal("node_modules should not be watched")
	}
	if watchableDirectory(workspace, filepath.Join(workspace, ".devdrop")) {
		t.Fatal(".devdrop should not be watched")
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
