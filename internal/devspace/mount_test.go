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
	if err := SaveManifest(workspace, Manifest{
		Version:       ManifestVersion,
		WorkspaceRoot: workspace,
		Projects: []Project{
			hardeningProject("apps/hydrated", ProjectTypeGit, "https://example.invalid/hydrated.git"),
			hardeningProject("apps/lazy", ProjectTypeGit, "https://example.invalid/lazy.git"),
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
	assertMountEntry(t, entries, "apps/lazy", "lazy", "will hydrate on project lookup")
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
		"apps/lazy\tgit\ton-demand\tlazy",
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

	err := MountWorkspace(context.Background(), mountpoint, WorkspaceMountOptions{HydrateOnLookup: true}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "refusing to hide local files") {
		t.Fatalf("mountpoint error = %v", err)
	}
}

func assertMountEntry(t *testing.T, entries []MountEntry, path, status, reason string) {
	t.Helper()
	for _, entry := range entries {
		if entry.Path != path {
			continue
		}
		if entry.Status != status || entry.Reason != reason {
			t.Fatalf("%s entry = %+v, want status=%q reason=%q", path, entry, status, reason)
		}
		return
	}
	t.Fatalf("entry %s not found in %+v", path, entries)
}
