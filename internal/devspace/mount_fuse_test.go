//go:build linux && fusetest

package devspace

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestFuseMountListsManifestPathsWithoutHydration(t *testing.T) {
	if _, err := os.Stat("/dev/fuse"); err != nil {
		t.Skipf("FUSE device unavailable: %v", err)
	}
	workspace := hardeningInitWorkspace(t, "code")
	goodRemote := hardeningBareRepo(t)
	badRemote := filepath.Join(t.TempDir(), "missing.git")
	projects := []Project{
		hardeningProject("apps/lazy", ProjectTypeGit, goodRemote),
		hardeningProject("apps/broken", ProjectTypeGit, badRemote),
	}
	if err := SaveManifest(workspace, Manifest{
		Version:       ManifestVersion,
		WorkspaceRoot: workspace,
		Projects:      projects,
	}); err != nil {
		t.Fatal(err)
	}

	mountpoint, unmount, _ := startFuseMount(t, false)
	defer unmount()
	waitForMountEntry(t, mountpoint, "apps")
	waitForMountEntry(t, filepath.Join(mountpoint, "apps"), "lazy")
	waitForMountEntry(t, filepath.Join(mountpoint, "apps"), "broken")
	apps := mountNames(t, filepath.Join(mountpoint, "apps"))
	if !containsMountName(apps, "lazy") || !containsMountName(apps, "broken") {
		t.Fatalf("apps entries = %v, want lazy and broken", apps)
	}
	if exists(filepath.Join(workspace, "apps", "lazy")) {
		t.Fatal("lazy project hydrated while hydrate-on-lookup was disabled")
	}
}

func TestFuseMountHydratesOnLookupAndPropagatesFailure(t *testing.T) {
	if _, err := os.Stat("/dev/fuse"); err != nil {
		t.Skipf("FUSE device unavailable: %v", err)
	}
	workspace := hardeningInitWorkspace(t, "code")
	goodRemote := hardeningBareRepo(t)
	badRemote := filepath.Join(t.TempDir(), "missing.git")
	projects := []Project{
		hardeningProject("apps/lazy", ProjectTypeGit, goodRemote),
		hardeningProject("apps/broken", ProjectTypeGit, badRemote),
	}
	if err := SaveManifest(workspace, Manifest{
		Version:       ManifestVersion,
		WorkspaceRoot: workspace,
		Projects:      projects,
	}); err != nil {
		t.Fatal(err)
	}

	mountpoint, unmount, errOut := startFuseMount(t, true)
	defer unmount()
	waitForMountEntry(t, mountpoint, "apps")

	readme, err := os.ReadFile(filepath.Join(mountpoint, "apps", "lazy", "README.md"))
	if err != nil {
		t.Fatalf("read hydrated project through mount: %v\nstderr:\n%s", err, errOut.String())
	}
	if string(readme) != "hello\n" {
		t.Fatalf("README = %q, want hello", readme)
	}
	if !exists(filepath.Join(workspace, "apps", "lazy", ".git")) {
		t.Fatal("lookup did not hydrate project into workspace")
	}

	if _, err := os.ReadDir(filepath.Join(mountpoint, "apps", "broken")); err == nil {
		t.Fatal("broken project lookup succeeded; want hydration error")
	}
	if !strings.Contains(errOut.String(), "hydrate apps/broken failed") {
		t.Fatalf("stderr missing hydration failure:\n%s", errOut.String())
	}
}

func startFuseMount(t *testing.T, hydrateOnLookup bool) (string, func(), *bytes.Buffer) {
	t.Helper()
	mountpoint := filepath.Join(t.TempDir(), "mnt")
	ctx, cancel := context.WithCancel(context.Background())
	var out bytes.Buffer
	var errOut bytes.Buffer
	done := make(chan error, 1)
	go func() {
		done <- MountWorkspace(ctx, mountpoint, WorkspaceMountOptions{
			HydrateOnLookup: hydrateOnLookup,
			ErrOut:          &errOut,
		}, &out)
	}()

	cleanup := func() {
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Fatalf("mount returned error: %v", err)
			}
		case <-time.After(10 * time.Second):
			t.Fatal("mount did not unmount after cancellation")
		}
	}
	return mountpoint, cleanup, &errOut
}

func waitForMountEntry(t *testing.T, dir, name string) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		names, err := os.ReadDir(dir)
		if err == nil {
			for _, entry := range names {
				if entry.Name() == name {
					return
				}
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("mount entry %q did not appear under %s", name, dir)
}

func mountNames(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	return names
}

func containsMountName(names []string, want string) bool {
	for _, name := range names {
		if name == want {
			return true
		}
	}
	return false
}
