package devdrop

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestWorkspaceRemoteSetGet(t *testing.T) {
	workspace := hardeningInitWorkspace(t, "code")
	remote := workspaceSyncBareRepo(t)

	cfg, err := SetManifestRemote(remote)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.WorkspaceRoot != workspace {
		t.Fatalf("workspace changed: %s", cfg.WorkspaceRoot)
	}
	if cfg.ManifestRemote != remote {
		t.Fatalf("remote = %q", cfg.ManifestRemote)
	}
	if cfg.ManifestRepoPath == "" {
		t.Fatal("manifest repo path was not stored")
	}
	got, err := GetManifestRemote()
	if err != nil {
		t.Fatal(err)
	}
	if got.ManifestRemote != remote {
		t.Fatalf("get remote = %q", got.ManifestRemote)
	}
}

func TestWorkspacePushInitializesClonedManifestRepoAndCommitsChanges(t *testing.T) {
	workspace := hardeningInitWorkspace(t, "code")
	remote := workspaceSyncBareRepo(t)
	if _, err := SetManifestRemote(remote); err != nil {
		t.Fatal(err)
	}
	if err := SaveManifest(workspace, Manifest{
		Version:       ManifestVersion,
		WorkspaceRoot: workspace,
		Projects:      []Project{hardeningProject("apps/app", ProjectTypeLocal, "")},
	}); err != nil {
		t.Fatal(err)
	}

	changed, err := PushWorkspaceManifest()
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("initial push reported no change")
	}
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if !exists(filepath.Join(cfg.ManifestRepoPath, ".git")) {
		t.Fatal("manifest repo was not cloned")
	}
	data, err := os.ReadFile(filepath.Join(cfg.ManifestRepoPath, syncedManifestName))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(data, []byte("apps/app")) || bytes.Contains(data, []byte(workspace)) {
		t.Fatalf("unexpected synced manifest:\n%s", data)
	}
	if got := workspaceSyncRun(t, cfg.ManifestRepoPath, "git", "rev-list", "--count", "HEAD"); strings.TrimSpace(got) != "1" {
		t.Fatalf("commit count = %s", got)
	}
}

func TestWorkspacePushIdempotentWhenNothingChanged(t *testing.T) {
	workspace := hardeningInitWorkspace(t, "code")
	remote := workspaceSyncBareRepo(t)
	if _, err := SetManifestRemote(remote); err != nil {
		t.Fatal(err)
	}
	if err := SaveManifest(workspace, Manifest{
		Version:       ManifestVersion,
		WorkspaceRoot: workspace,
		Projects:      []Project{hardeningProject("apps/app", ProjectTypeLocal, "")},
	}); err != nil {
		t.Fatal(err)
	}
	if changed, err := PushWorkspaceManifest(); err != nil || !changed {
		t.Fatalf("first push changed=%t err=%v", changed, err)
	}
	if changed, err := PushWorkspaceManifest(); err != nil || changed {
		t.Fatalf("second push changed=%t err=%v", changed, err)
	}
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if got := workspaceSyncRun(t, cfg.ManifestRepoPath, "git", "rev-list", "--count", "HEAD"); strings.TrimSpace(got) != "1" {
		t.Fatalf("commit count = %s", got)
	}
}

func TestWorkspacePullCopiesManifestToSecondWorkspaceAndCreatesBackup(t *testing.T) {
	root := t.TempDir()
	remote := workspaceSyncBareRepo(t)
	workspaceA := filepath.Join(root, "machine a", "code")
	workspaceB := filepath.Join(root, "machine b", "code")

	t.Setenv(envHome, filepath.Join(root, "home-a"))
	if _, err := InitWorkspace(workspaceA); err != nil {
		t.Fatal(err)
	}
	if _, err := SetManifestRemote(remote); err != nil {
		t.Fatal(err)
	}
	if err := SaveManifest(workspaceA, Manifest{
		Version:       ManifestVersion,
		WorkspaceRoot: workspaceA,
		Projects:      []Project{hardeningProject("apps/app", ProjectTypeLocal, "")},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := PushWorkspaceManifest(); err != nil {
		t.Fatal(err)
	}

	t.Setenv(envHome, filepath.Join(root, "home-b"))
	if _, err := InitWorkspace(workspaceB); err != nil {
		t.Fatal(err)
	}
	if _, err := SetManifestRemote(remote); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(manifestPath(workspaceB))
	if err != nil {
		t.Fatal(err)
	}
	if changed, err := PullWorkspaceManifest(); err != nil || !changed {
		t.Fatalf("pull changed=%t err=%v", changed, err)
	}
	pulled, err := LoadManifest(workspaceB)
	if err != nil {
		t.Fatal(err)
	}
	if pulled.WorkspaceRoot != workspaceB {
		t.Fatalf("workspace root was not localized: %s", pulled.WorkspaceRoot)
	}
	if _, ok := findProject(pulled, "apps/app"); !ok {
		t.Fatalf("project missing after pull: %+v", pulled.Projects)
	}
	backup, err := os.ReadFile(manifestPath(workspaceB) + ".bak")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(backup, before) {
		t.Fatal("pull backup does not match previous manifest")
	}
}

func TestWorkspacePullRefusesInvalidJSONBeforeReplacingManifest(t *testing.T) {
	workspace := hardeningInitWorkspace(t, "code")
	remote, repo := workspaceSyncRemoteWithClone(t)
	if _, err := SetManifestRemote(remote); err != nil {
		t.Fatal(err)
	}
	original := Manifest{
		Version:       ManifestVersion,
		WorkspaceRoot: workspace,
		Projects:      []Project{hardeningProject("apps/original", ProjectTypeLocal, "")},
	}
	if err := SaveManifest(workspace, original); err != nil {
		t.Fatal(err)
	}
	hardeningWriteFile(t, filepath.Join(repo, syncedManifestName), "{not-json", 0o600)
	workspaceSyncCommitAndPush(t, repo, "bad manifest")

	_, err := PullWorkspaceManifest()
	if err == nil || !strings.Contains(err.Error(), "invalid JSON") {
		t.Fatalf("pull invalid JSON error = %v", err)
	}
	after, err := LoadManifest(workspace)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := findProject(after, "apps/original"); !ok {
		t.Fatalf("local manifest was replaced: %+v", after.Projects)
	}
}

func TestWorkspacePullRefusesPathTraversalBeforeReplacingManifest(t *testing.T) {
	workspace := hardeningInitWorkspace(t, "code")
	remote, repo := workspaceSyncRemoteWithClone(t)
	if _, err := SetManifestRemote(remote); err != nil {
		t.Fatal(err)
	}
	original := Manifest{
		Version:       ManifestVersion,
		WorkspaceRoot: workspace,
		Projects:      []Project{hardeningProject("apps/original", ProjectTypeLocal, "")},
	}
	if err := SaveManifest(workspace, original); err != nil {
		t.Fatal(err)
	}
	unsafe := Manifest{
		Version:       ManifestVersion,
		WorkspaceRoot: ".",
		Projects: []Project{{
			ID:          "bad",
			Name:        "bad",
			Path:        "../bad",
			Type:        ProjectTypeLocal,
			HydrateMode: HydrateManual,
		}},
	}
	data, err := manifestBytes(unsafe)
	if err != nil {
		t.Fatal(err)
	}
	hardeningWriteFile(t, filepath.Join(repo, syncedManifestName), string(data), 0o600)
	workspaceSyncCommitAndPush(t, repo, "unsafe manifest")

	_, err = PullWorkspaceManifest()
	if err == nil || !strings.Contains(err.Error(), "escapes workspace") {
		t.Fatalf("pull traversal error = %v", err)
	}
	after, err := LoadManifest(workspace)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := findProject(after, "apps/original"); !ok {
		t.Fatalf("local manifest was replaced: %+v", after.Projects)
	}
}

func TestWorkspacePullRefusesToOverwriteLocalUnpushedChanges(t *testing.T) {
	root := t.TempDir()
	remote := workspaceSyncBareRepo(t)
	workspaceA := filepath.Join(root, "a")
	workspaceB := filepath.Join(root, "b")

	t.Setenv(envHome, filepath.Join(root, "home-a"))
	if _, err := InitWorkspace(workspaceA); err != nil {
		t.Fatal(err)
	}
	if _, err := SetManifestRemote(remote); err != nil {
		t.Fatal(err)
	}
	if err := SaveManifest(workspaceA, Manifest{
		Version:       ManifestVersion,
		WorkspaceRoot: workspaceA,
		Projects:      []Project{hardeningProject("apps/remote", ProjectTypeLocal, "")},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := PushWorkspaceManifest(); err != nil {
		t.Fatal(err)
	}

	t.Setenv(envHome, filepath.Join(root, "home-b"))
	if _, err := InitWorkspace(workspaceB); err != nil {
		t.Fatal(err)
	}
	if _, err := SetManifestRemote(remote); err != nil {
		t.Fatal(err)
	}
	if err := SaveManifest(workspaceB, Manifest{
		Version:       ManifestVersion,
		WorkspaceRoot: workspaceB,
		Projects:      []Project{hardeningProject("apps/local", ProjectTypeLocal, "")},
	}); err != nil {
		t.Fatal(err)
	}
	_, err := PullWorkspaceManifest()
	if err == nil || !strings.Contains(err.Error(), "local manifest differs") {
		t.Fatalf("pull local overwrite error = %v", err)
	}
	after, err := LoadManifest(workspaceB)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := findProject(after, "apps/local"); !ok {
		t.Fatalf("local changes were overwritten: %+v", after.Projects)
	}
}

func TestWorkspacePullAllowsFastForwardWhenLocalMatchesPreviousRemoteManifest(t *testing.T) {
	root := t.TempDir()
	remote := workspaceSyncBareRepo(t)
	workspaceA := filepath.Join(root, "a")
	workspaceB := filepath.Join(root, "b")

	t.Setenv(envHome, filepath.Join(root, "home-a"))
	if _, err := InitWorkspace(workspaceA); err != nil {
		t.Fatal(err)
	}
	if _, err := SetManifestRemote(remote); err != nil {
		t.Fatal(err)
	}
	if err := SaveManifest(workspaceA, Manifest{
		Version:       ManifestVersion,
		WorkspaceRoot: workspaceA,
		Projects:      []Project{hardeningProject("apps/one", ProjectTypeLocal, "")},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := PushWorkspaceManifest(); err != nil {
		t.Fatal(err)
	}

	t.Setenv(envHome, filepath.Join(root, "home-b"))
	if _, err := InitWorkspace(workspaceB); err != nil {
		t.Fatal(err)
	}
	if _, err := SetManifestRemote(remote); err != nil {
		t.Fatal(err)
	}
	if _, err := PullWorkspaceManifest(); err != nil {
		t.Fatal(err)
	}
	updated, err := LoadManifest(workspaceB)
	if err != nil {
		t.Fatal(err)
	}
	updated.Projects = append(updated.Projects, hardeningProject("apps/two", ProjectTypeLocal, ""))
	if err := SaveManifest(workspaceB, updated); err != nil {
		t.Fatal(err)
	}
	if _, err := PushWorkspaceManifest(); err != nil {
		t.Fatal(err)
	}

	t.Setenv(envHome, filepath.Join(root, "home-a"))
	if changed, err := PullWorkspaceManifest(); err != nil || !changed {
		t.Fatalf("pull changed=%t err=%v", changed, err)
	}
	pulled, err := LoadManifest(workspaceA)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := findProject(pulled, "apps/two"); !ok {
		t.Fatalf("fast-forward pull missing project: %+v", pulled.Projects)
	}
}

func TestWorkspacePushRefusesDirtyManifestRepoState(t *testing.T) {
	workspace := hardeningInitWorkspace(t, "code")
	remote := workspaceSyncBareRepo(t)
	if _, err := SetManifestRemote(remote); err != nil {
		t.Fatal(err)
	}
	if err := SaveManifest(workspace, Manifest{
		Version:       ManifestVersion,
		WorkspaceRoot: workspace,
		Projects:      []Project{hardeningProject("apps/app", ProjectTypeLocal, "")},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := PushWorkspaceManifest(); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	hardeningWriteFile(t, filepath.Join(cfg.ManifestRepoPath, "scratch.txt"), "dirty\n", 0o644)

	_, err = PushWorkspaceManifest()
	if err == nil || !strings.Contains(err.Error(), "uncommitted changes") {
		t.Fatalf("push dirty repo error = %v", err)
	}
}

func TestWorkspacePushPullWorkWithWorkspacePathsContainingSpaces(t *testing.T) {
	root := t.TempDir()
	remote := workspaceSyncBareRepo(t)
	workspaceA := filepath.Join(root, "machine a", "code with spaces")
	workspaceB := filepath.Join(root, "machine b", "code with spaces")

	t.Setenv(envHome, filepath.Join(root, "home a"))
	if _, err := InitWorkspace(workspaceA); err != nil {
		t.Fatal(err)
	}
	if _, err := SetManifestRemote(remote); err != nil {
		t.Fatal(err)
	}
	if err := SaveManifest(workspaceA, Manifest{
		Version:       ManifestVersion,
		WorkspaceRoot: workspaceA,
		Projects:      []Project{hardeningProject("client apps/web app", ProjectTypeLocal, "")},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := PushWorkspaceManifest(); err != nil {
		t.Fatal(err)
	}

	t.Setenv(envHome, filepath.Join(root, "home b"))
	if _, err := InitWorkspace(workspaceB); err != nil {
		t.Fatal(err)
	}
	if _, err := SetManifestRemote(remote); err != nil {
		t.Fatal(err)
	}
	if _, err := PullWorkspaceManifest(); err != nil {
		t.Fatal(err)
	}
	m, err := LoadManifest(workspaceB)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := findProject(m, "client apps/web app"); !ok {
		t.Fatalf("spaced path missing: %+v", m.Projects)
	}
}

func TestWorkspaceGitBackedTwoMachinePlanApplyAndHydrate(t *testing.T) {
	root := t.TempDir()
	manifestRemote := workspaceSyncBareRepo(t)
	projectRemote := hardeningBareRepo(t)
	workspaceA := filepath.Join(root, "machine-a", "code")
	workspaceB := filepath.Join(root, "machine-b", "code")

	t.Setenv(envHome, filepath.Join(root, "home-a"))
	if _, err := InitWorkspace(workspaceA); err != nil {
		t.Fatal(err)
	}
	if _, err := SetManifestRemote(manifestRemote); err != nil {
		t.Fatal(err)
	}
	m := Manifest{
		Version:       ManifestVersion,
		WorkspaceRoot: workspaceA,
		Projects:      []Project{hardeningProject("work/api", ProjectTypeGit, projectRemote)},
	}
	if err := SaveManifest(workspaceA, m); err != nil {
		t.Fatal(err)
	}
	if _, err := PushWorkspaceManifest(); err != nil {
		t.Fatal(err)
	}

	t.Setenv(envHome, filepath.Join(root, "home-b"))
	if _, err := InitWorkspace(workspaceB); err != nil {
		t.Fatal(err)
	}
	if _, err := SetManifestRemote(manifestRemote); err != nil {
		t.Fatal(err)
	}
	if _, err := PullWorkspaceManifest(); err != nil {
		t.Fatal(err)
	}
	planned, err := BuildPlan()
	if err != nil {
		t.Fatal(err)
	}
	if hardeningSafeActionCount(planned) != 1 {
		t.Fatalf("expected one safe action: %+v", planned.Actions)
	}
	if err := SaveLastPlan(planned); err != nil {
		t.Fatal(err)
	}
	if _, err := ApplyLastPlan(); err != nil {
		t.Fatal(err)
	}
	if !exists(filepath.Join(workspaceB, "work", "api")) {
		t.Fatal("plan/apply did not create placeholder")
	}
	if _, err := HydrateProject("api"); err != nil {
		t.Fatal(err)
	}
	if !exists(filepath.Join(workspaceB, "work", "api", ".git")) {
		t.Fatal("hydrate did not clone project")
	}
}

func workspaceSyncBareRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	remote := filepath.Join(root, "manifest.git")
	hardeningRun(t, root, "git", "init", "--bare", "-b", "main", remote)
	return remote
}

func workspaceSyncRemoteWithClone(t *testing.T) (string, string) {
	t.Helper()
	remote := workspaceSyncBareRepo(t)
	clone := filepath.Join(t.TempDir(), "manifest-clone")
	hardeningRun(t, filepath.Dir(clone), "git", "clone", remote, clone)
	hardeningRun(t, clone, "git", "config", "user.email", "test@example.com")
	hardeningRun(t, clone, "git", "config", "user.name", "Test User")
	return remote, clone
}

func workspaceSyncCommitAndPush(t *testing.T, repo, msg string) {
	t.Helper()
	hardeningRun(t, repo, "git", "add", syncedManifestName)
	hardeningRun(t, repo, "git", "commit", "-m", msg)
	hardeningRun(t, repo, "git", "push", "-u", "origin", "HEAD")
}

func workspaceSyncRun(t *testing.T, dir, name string, args ...string) string {
	t.Helper()
	out := bytes.Buffer{}
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		t.Fatalf("%s %s failed: %v\n%s", name, strings.Join(args, " "), err, out.String())
	}
	return out.String()
}
