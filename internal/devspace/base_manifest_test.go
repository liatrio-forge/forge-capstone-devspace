package devspace

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestSyncRecordsBaseManifest(t *testing.T) {
	root := t.TempDir()
	remote := workspaceSyncBareRepo(t)
	workspaceA := filepath.Join(root, "machine-a", "code")
	workspaceB := filepath.Join(root, "machine-b", "code")
	homeA := t.TempDir()
	homeB := t.TempDir()

	t.Setenv(envHome, homeA)
	if _, err := InitWorkspace(workspaceA); err != nil {
		t.Fatal(err)
	}
	if _, err := SetManifestRemote(remote); err != nil {
		t.Fatal(err)
	}
	local := Manifest{
		Version:       ManifestVersion,
		WorkspaceRoot: workspaceA,
		Projects:      []Project{hardeningProject("apps/app", ProjectTypeLocal, "")},
	}
	if err := SaveManifest(workspaceA, local); err != nil {
		t.Fatal(err)
	}

	if changed, err := PushWorkspaceManifest(); err != nil || !changed {
		t.Fatalf("push changed=%t err=%v", changed, err)
	}
	assertBaseManifest(t, manifestForSync(local), 0o600)

	t.Setenv(envHome, homeB)
	if _, err := InitWorkspace(workspaceB); err != nil {
		t.Fatal(err)
	}
	if _, err := SetManifestRemote(remote); err != nil {
		t.Fatal(err)
	}
	if changed, err := PullWorkspaceManifest(); err != nil || !changed {
		t.Fatalf("pull changed=%t err=%v", changed, err)
	}
	expected, err := LoadManifest(workspaceB)
	if err != nil {
		t.Fatal(err)
	}
	assertBaseManifest(t, expected, 0o600)
}

func TestHostedSyncRecordsBaseManifest(t *testing.T) {
	root := t.TempDir()
	server := hostedSyncTestServer(t)
	workspaceA := filepath.Join(root, "machine-a", "code")
	workspaceB := filepath.Join(root, "machine-b", "code")
	homeA := t.TempDir()
	homeB := t.TempDir()

	t.Setenv(envHome, homeA)
	if _, err := InitWorkspace(workspaceA); err != nil {
		t.Fatal(err)
	}
	if _, err := SetHostedSync(server.URL, "test-token", "team-a"); err != nil {
		t.Fatal(err)
	}
	local := Manifest{
		Version:       ManifestVersion,
		WorkspaceRoot: workspaceA,
		Projects:      []Project{hardeningProject("apps/app", ProjectTypeLocal, "")},
	}
	if err := SaveManifest(workspaceA, local); err != nil {
		t.Fatal(err)
	}

	if result, err := PushHostedManifest(); err != nil || !result.Changed {
		t.Fatalf("push result=%+v err=%v", result, err)
	}
	assertBaseManifest(t, manifestForSync(local), 0o600)

	t.Setenv(envHome, homeB)
	if _, err := InitWorkspace(workspaceB); err != nil {
		t.Fatal(err)
	}
	if _, err := SetHostedSync(server.URL, "test-token", "team-a"); err != nil {
		t.Fatal(err)
	}
	if result, err := PullHostedManifest(); err != nil || !result.Changed {
		t.Fatalf("pull result=%+v err=%v", result, err)
	}
	expected, err := LoadManifest(workspaceB)
	if err != nil {
		t.Fatal(err)
	}
	assertBaseManifest(t, expected, 0o600)
}

func TestBaseManifestAbsentIsDetectable(t *testing.T) {
	t.Setenv(envHome, t.TempDir())

	got, ok, err := loadBaseManifest()
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatalf("ok = true, manifest = %+v", got)
	}
}

func assertBaseManifest(t *testing.T, want Manifest, wantPerm os.FileMode) {
	t.Helper()
	got, ok, err := loadBaseManifest()
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("base manifest was not recorded")
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("base manifest = %+v, want %+v", got, want)
	}
	path, err := baseManifestPath()
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if gotPerm := info.Mode().Perm(); gotPerm != wantPerm {
		t.Fatalf("base manifest permissions = %o, want %o", gotPerm, wantPerm)
	}
}
