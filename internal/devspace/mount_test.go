package devspace

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"
)

func TestMountEntriesRepresentTrackedProjectsFromManifest(t *testing.T) {
	workspace := hardeningInitWorkspace(t, "code")
	hardeningGitRepo(t, filepath.Join(workspace, "apps", "hydrated"))
	hardeningWriteFile(t, filepath.Join(workspace, "apps", "hydrated", ".env"), "TOKEN=local\n", 0o600)
	hardeningWriteFile(t, filepath.Join(workspace, "apps", "hydrated", "local.txt"), "dirty\n", 0o644)
	hydrated := hardeningProject("apps/hydrated", ProjectTypeGit, "https://example.invalid/hydrated.git")
	hydrated.Setup = Setup{
		PackageManager: "npm",
		InstallCommand: "npm install",
		DevCommand:     "npm run dev",
	}
	lazy := hardeningProject("apps/lazy", ProjectTypeGit, "https://example.invalid/lazy.git")
	lazy.Setup = Setup{InstallCommand: "go mod download"}
	if err := SaveManifest(workspace, Manifest{
		Version:       ManifestVersion,
		WorkspaceRoot: workspace,
		Projects: []Project{
			hydrated,
			lazy,
			hardeningProject("notes/local", ProjectTypeLocal, ""),
		},
	}); err != nil {
		t.Fatal(err)
	}

	entries, err := BuildMountEntries()
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 3 {
		t.Fatalf("entries = %+v", entries)
	}
	assertMountEntry(t, entries, "apps/hydrated", "hydrated", "")
	hydratedEntry := mountEntryByPath(t, entries, "apps/hydrated")
	if !hydratedEntry.Dirty || !hydratedEntry.EnvPresent || hydratedEntry.SetupHint != "install: npm install; dev: npm run dev" {
		t.Fatalf("hydrated diagnostics = %+v", hydratedEntry)
	}
	assertMountEntry(t, entries, "apps/lazy", "lazy", "will hydrate on project lookup")
	lazyEntry := mountEntryByPath(t, entries, "apps/lazy")
	if lazyEntry.Dirty || lazyEntry.EnvPresent || lazyEntry.SetupHint != "install: go mod download" {
		t.Fatalf("lazy diagnostics = %+v", lazyEntry)
	}
	assertMountEntry(t, entries, "notes/local", "missing", "no automatic mount hydration is configured")
}

func TestMountPreviewCommandDoesNotRequireFUSE(t *testing.T) {
	workspace := hardeningInitWorkspace(t, "code")
	if err := SaveManifest(workspace, Manifest{
		Version:       ManifestVersion,
		WorkspaceRoot: workspace,
		Projects: []Project{
			hardeningProject("apps/lazy", ProjectTypeGit, "https://example.invalid/lazy.git"),
		},
	}); err != nil {
		t.Fatal(err)
	}

	cmd := NewRootCommand("test")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"mount", filepath.Join(t.TempDir(), "mnt"), "--preview"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	for _, want := range []string{
		"DevSpace lazy mount preview",
		"github.com/hanwen/go-fuse/v2/fs",
		"PATH", "TYPE", "HYDRATE MODE", "STATUS", "DIRTY", "ENV", "REASON",
		"apps/lazy",
		"git",
		"on-demand",
		"lazy",
		"will hydrate on project lookup",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("preview missing %q:\n%s", want, got)
		}
	}
}

func TestMountWorkspaceRefusesNonEmptyMountpointBeforeFUSE(t *testing.T) {
	workspace := hardeningInitWorkspace(t, "code")
	if err := SaveManifest(workspace, Manifest{
		Version:       ManifestVersion,
		WorkspaceRoot: workspace,
		Projects:      []Project{hardeningProject("apps/lazy", ProjectTypeGit, "https://example.invalid/lazy.git")},
	}); err != nil {
		t.Fatal(err)
	}
	mountpoint := filepath.Join(t.TempDir(), "mnt")
	hardeningWriteFile(t, filepath.Join(mountpoint, "keep.txt"), "local\n", 0o644)

	err := MountWorkspace(context.Background(), mountpoint, WorkspaceMountOptions{HydrateOnLookup: true})
	if err == nil || !strings.Contains(err.Error(), "refusing to hide local files") {
		t.Fatalf("mountpoint error = %v", err)
	}
}

func TestStaleMountGuidanceIncludesPlatformCleanupCommands(t *testing.T) {
	guidance := staleMountGuidance("/tmp/devspace-mount")
	for _, want := range []string{
		"previous devspace mount",
		"umount /tmp/devspace-mount",
		"fusermount3 -u /tmp/devspace-mount",
	} {
		if !strings.Contains(guidance, want) {
			t.Fatalf("guidance missing %q: %s", want, guidance)
		}
	}
}

func assertMountEntry(t *testing.T, entries []MountEntry, path, status, reason string) {
	t.Helper()
	entry := mountEntryByPath(t, entries, path)
	if entry.Status != status || entry.Reason != reason {
		t.Fatalf("%s entry = %+v, want status=%q reason=%q", path, entry, status, reason)
	}
}

func mountEntryByPath(t *testing.T, entries []MountEntry, path string) MountEntry {
	t.Helper()
	for _, entry := range entries {
		if entry.Path == path {
			return entry
		}
	}
	t.Fatalf("entry %s not found in %+v", path, entries)
	return MountEntry{}
}
