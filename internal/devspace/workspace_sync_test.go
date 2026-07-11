package devspace

import (
	"bytes"
	"errors"
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

func TestWorkspaceRemoteCreateLocalInitializesBareRepoAndSetsRemote(t *testing.T) {
	workspace := hardeningInitWorkspace(t, "code")
	remote := filepath.Join(t.TempDir(), "new-manifest.git")

	cfg, err := CreateLocalManifestRemote(remote)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.WorkspaceRoot != workspace {
		t.Fatalf("workspace changed: %s", cfg.WorkspaceRoot)
	}
	if cfg.ManifestRemote != remote {
		t.Fatalf("remote = %q", cfg.ManifestRemote)
	}
	if !isBareGitRepo(remote) {
		t.Fatalf("remote was not created as a bare repo: %s", remote)
	}
}

func TestWorkspaceRemoteCreateGitHubRequiresGh(t *testing.T) {
	hardeningInitWorkspace(t, "code")
	t.Setenv("PATH", t.TempDir())

	_, err := CreateGitHubManifestRemote("your-org/devspace-manifest", true)
	if err == nil || !strings.Contains(err.Error(), "requires GitHub CLI") {
		t.Fatalf("github create error = %v", err)
	}
}

func TestManifestRemoteNotReadyErrorGivesCreateCommands(t *testing.T) {
	err := manifestRemoteNotReadyError(
		"git@github.com:your-org/devspace-manifest.git",
		errors.New("ERROR: Repository not found."),
	)
	if err == nil || !strings.Contains(err.Error(), "manifest remote is not ready") {
		t.Fatalf("remote not ready error = %v", err)
	}
	if !strings.Contains(err.Error(), "devspace sync remote create github your-org/devspace-manifest --private") {
		t.Fatalf("missing github recovery command:\n%v", err)
	}
	if !strings.Contains(err.Error(), "devspace sync remote create local ~/Projects/devspace-manifest.git") {
		t.Fatalf("missing local recovery command:\n%v", err)
	}
	if strings.Contains(err.Error(), "devspace workspace") {
		t.Fatalf("remote recovery still references removed workspace command:\n%v", err)
	}
}

func TestSyncGuidanceUsesCanonicalCommands(t *testing.T) {
	hardeningInitWorkspace(t, "code")
	if _, err := GetManifestRemote(); err == nil || !strings.Contains(err.Error(), "devspace sync remote set <url-or-path>") {
		t.Fatalf("missing remote error = %v", err)
	}
	t.Setenv("PATH", t.TempDir())
	if _, err := CreateGitHubManifestRemote("owner/repo", true); err == nil || !strings.Contains(err.Error(), "devspace sync remote set git@github.com:owner/repo.git") {
		t.Fatalf("github fallback error = %v", err)
	}
}

func TestSyncDivergenceGuidanceUsesCanonicalPull(t *testing.T) {
	remote := workspaceSyncBareRepo(t)
	upstream := filepath.Join(t.TempDir(), "upstream")
	hardeningRun(t, filepath.Dir(upstream), "git", "clone", remote, upstream)
	hardeningRun(t, upstream, "git", "config", "user.email", "test@example.com")
	hardeningRun(t, upstream, "git", "config", "user.name", "Test User")
	hardeningWriteFile(t, filepath.Join(upstream, syncedManifestName), "{}\n", 0o644)
	workspaceSyncCommitAndPush(t, upstream, "initial")

	local := filepath.Join(t.TempDir(), "local")
	hardeningRun(t, filepath.Dir(local), "git", "clone", remote, local)
	hardeningRun(t, local, "git", "config", "user.email", "test@example.com")
	hardeningRun(t, local, "git", "config", "user.name", "Test User")
	hardeningWriteFile(t, filepath.Join(upstream, syncedManifestName), "{\"remote\":1}\n", 0o644)
	workspaceSyncCommitAndPush(t, upstream, "remote update")
	hardeningRun(t, local, "git", "fetch", "origin")
	if err := ensureManifestRepoNotBehind(local); err == nil || err.Error() != "remote manifest is newer; run `devspace sync pull` before pushing" {
		t.Fatalf("behind error = %v", err)
	}

	hardeningWriteFile(t, filepath.Join(local, "local.txt"), "local\n", 0o644)
	hardeningRun(t, local, "git", "add", "local.txt")
	hardeningRun(t, local, "git", "commit", "-m", "local update")
	if err := ensureManifestRepoNotBehind(local); err == nil || err.Error() != "remote branch diverged; run `devspace sync pull` and reconcile before pushing" {
		t.Fatalf("diverged error = %v", err)
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

func TestWorkspacePushIncludesWorkspaceIgnoreFile(t *testing.T) {
	workspace := hardeningInitWorkspace(t, "code")
	remote := workspaceSyncBareRepo(t)
	if _, err := SetManifestRemote(remote); err != nil {
		t.Fatal(err)
	}
	hardeningWriteFile(t, filepath.Join(workspace, workspaceIgnoreFile), "adobe/\n", 0o644)

	if changed, err := PushWorkspaceManifest(); err != nil || !changed {
		t.Fatalf("push changed=%t err=%v", changed, err)
	}
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(cfg.ManifestRepoPath, workspaceIgnoreFile))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "adobe/\n" {
		t.Fatalf("synced ignore = %q", data)
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

func TestWorkspacePushRetriesPendingCommit(t *testing.T) {
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

	// Simulate the state a failed network push leaves behind: the workspace
	// manifest changes, the cache is updated and committed, but the push to
	// origin never happens. Do this directly against the cache (rather than
	// with a flaky network-failure harness) so the cache ends up exactly one
	// commit ahead of origin with content matching the workspace.
	second := Manifest{
		Version:       ManifestVersion,
		WorkspaceRoot: workspace,
		Projects: []Project{
			hardeningProject("apps/app", ProjectTypeLocal, ""),
			hardeningProject("apps/other", ProjectTypeLocal, ""),
		},
	}
	if err := SaveManifest(workspace, second); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := writeSyncedManifest(cfg.ManifestRepoPath, manifestForSync(second)); err != nil {
		t.Fatal(err)
	}
	if err := commitManifestRepo(cfg.ManifestRepoPath, cfg); err != nil {
		t.Fatal(err)
	}
	if ahead := workspaceSyncRun(t, cfg.ManifestRepoPath, "git", "rev-list", "--count", "HEAD", "^origin/main"); strings.TrimSpace(ahead) != "1" {
		t.Fatalf("test setup: cache ahead-of-origin count = %s, want 1", ahead)
	}

	changed, err := PushWorkspaceManifest()
	if err != nil {
		t.Fatalf("retry push failed: %v", err)
	}
	if !changed {
		t.Fatal("retry push should report changed=true when publishing a pending commit")
	}

	localHead := strings.TrimSpace(workspaceSyncRun(t, cfg.ManifestRepoPath, "git", "rev-parse", "HEAD"))
	upstreamHead := strings.TrimSpace(workspaceSyncRun(t, cfg.ManifestRepoPath, "git", "rev-parse", "origin/main"))
	if localHead != upstreamHead {
		t.Fatalf("local HEAD %s != upstream HEAD %s after retry", localHead, upstreamHead)
	}
	if got := workspaceSyncRun(t, cfg.ManifestRepoPath, "git", "rev-list", "--count", "HEAD"); strings.TrimSpace(got) != "2" {
		t.Fatalf("commit count after retry = %s, want 2 (retry must publish, not re-commit)", got)
	}

	clone := filepath.Join(t.TempDir(), "verify-clone")
	hardeningRun(t, filepath.Dir(clone), "git", "clone", remote, clone)
	data, err := os.ReadFile(filepath.Join(clone, syncedManifestName))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(data, []byte("apps/other")) {
		t.Fatalf("published manifest missing pending change:\n%s", data)
	}

	base, hasBase, err := loadBaseManifest()
	if err != nil {
		t.Fatal(err)
	}
	if !hasBase {
		t.Fatal("base manifest was not recorded after retry")
	}
	baseData, err := manifestBytes(base)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(baseData, []byte("apps/other")) {
		t.Fatalf("base manifest does not match published content:\n%s", baseData)
	}
}

func TestWorkspacePushFailureDoesNotRecordBase(t *testing.T) {
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

	// Reject all future pushes at the remote while leaving fetch untouched:
	// a pre-receive hook that exits nonzero fails the push deterministically,
	// exactly like an unreachable network remote would, without flakiness.
	hardeningWriteFile(t, filepath.Join(remote, "hooks", "pre-receive"), "#!/bin/sh\nexit 1\n", 0o755)

	if err := SaveManifest(workspace, Manifest{
		Version:       ManifestVersion,
		WorkspaceRoot: workspace,
		Projects: []Project{
			hardeningProject("apps/app", ProjectTypeLocal, ""),
			hardeningProject("apps/other", ProjectTypeLocal, ""),
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := PushWorkspaceManifest(); err == nil || !strings.Contains(err.Error(), "remote push failed") {
		t.Fatalf("push against rejecting remote should fail at the push step, got err=%v", err)
	}

	base, hasBase, err := loadBaseManifest()
	if err != nil {
		t.Fatal(err)
	}
	if !hasBase {
		t.Fatal("base manifest from the first successful push is missing")
	}
	baseData, err := manifestBytes(base)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(baseData, []byte("apps/other")) {
		t.Fatalf("base manifest recorded unpublished content after failed push:\n%s", baseData)
	}
	if !bytes.Contains(baseData, []byte("apps/app")) {
		t.Fatalf("base manifest no longer matches last published content:\n%s", baseData)
	}
}

func TestWorkspacePushUsesConfiguredCommitIdentity(t *testing.T) {
	workspace := hardeningInitWorkspace(t, "code")
	remote := workspaceSyncBareRepo(t)
	if _, err := SetManifestRemote(remote); err != nil {
		t.Fatal(err)
	}
	// Configure a custom commit identity before pushing.
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	cfg.ManifestCommitEmail = "bot@forge.example"
	cfg.ManifestCommitName = "Forge Bot"
	if err := SaveConfig(cfg); err != nil {
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
		t.Fatalf("push failed: %v", err)
	}
	cfg, err = LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	author := workspaceSyncRun(t, cfg.ManifestRepoPath, "git", "log", "-1", "--format=%ae %an")
	author = strings.TrimSpace(author)
	if author != "bot@forge.example Forge Bot" {
		t.Fatalf("commit author = %q, want %q", author, "bot@forge.example Forge Bot")
	}
}

func TestWorkspacePushFallsBackToDefaultCommitIdentity(t *testing.T) {
	// Isolate git from the developer's real global AND system config so the
	// "no identity configured" fallback path is actually exercised.
	// GIT_CONFIG_GLOBAL alone only skips ~/.gitconfig; a machine-level system
	// gitconfig would still leak through and make this assertion flaky.
	globalCfg := filepath.Join(t.TempDir(), "git-config")
	t.Setenv("GIT_CONFIG_GLOBAL", globalCfg)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")

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
		t.Fatalf("push failed: %v", err)
	}
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	author := workspaceSyncRun(t, cfg.ManifestRepoPath, "git", "log", "-1", "--format=%ae %an")
	author = strings.TrimSpace(author)
	if author != "devspace@example.invalid DevSpace" {
		t.Fatalf("default commit author = %q, want %q", author, "devspace@example.invalid DevSpace")
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

func TestWorkspacePullRestoresWorkspaceIgnoreFile(t *testing.T) {
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
	hardeningWriteFile(t, filepath.Join(workspaceA, workspaceIgnoreFile), "adobe/\n", 0o644)
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
	if changed, err := PullWorkspaceManifest(); err != nil || !changed {
		t.Fatalf("pull changed=%t err=%v", changed, err)
	}
	data, err := os.ReadFile(filepath.Join(workspaceB, workspaceIgnoreFile))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "adobe/\n" {
		t.Fatalf("pulled ignore = %q", data)
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

func TestWorkspacePullSucceedsAfterDiffWhenLocalHasNoUnpushedChanges(t *testing.T) {
	root := t.TempDir()
	remote := workspaceSyncBareRepo(t)
	workspaceA := filepath.Join(root, "a")
	workspaceB := filepath.Join(root, "b")

	// Machine A publishes the initial manifest.
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

	// Machine B pulls, adds a project, and pushes.
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

	// Machine A runs diff first — this fast-forwards the cached manifest
	// clone — then pulls. Machine A has no unpushed local changes, so the
	// pull must fast-forward, not refuse.
	t.Setenv(envHome, filepath.Join(root, "home-a"))
	if _, err := DiffWorkspaceManifest(); err != nil {
		t.Fatal(err)
	}
	changed, err := PullWorkspaceManifest()
	if err != nil {
		t.Fatalf("pull after diff failed: %v", err)
	}
	if !changed {
		t.Fatal("pull after diff reported no change")
	}
	pulled, err := LoadManifest(workspaceA)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := findProject(pulled, "apps/two"); !ok {
		t.Fatalf("pull after diff missing project: %+v", pulled.Projects)
	}
}

func TestWorkspaceDiffReportsRemoteChangesWithoutReplacingLocalManifest(t *testing.T) {
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
	remoteShared := hardeningProject("apps/shared", ProjectTypeGit, projectRemote)
	if err := SaveManifest(workspaceA, Manifest{
		Version:       ManifestVersion,
		WorkspaceRoot: workspaceA,
		Projects: []Project{
			remoteShared,
			hardeningProject("apps/remote-only", ProjectTypeLocal, ""),
		},
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
	if _, err := SetManifestRemote(manifestRemote); err != nil {
		t.Fatal(err)
	}
	localShared := hardeningProject("apps/shared", ProjectTypeLocal, "")
	if err := SaveManifest(workspaceB, Manifest{
		Version:       ManifestVersion,
		WorkspaceRoot: workspaceB,
		Projects: []Project{
			localShared,
			hardeningProject("apps/local-only", ProjectTypeLocal, ""),
		},
	}); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(manifestPath(workspaceB))
	if err != nil {
		t.Fatal(err)
	}

	cmd := newSyncCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"diff"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	output := out.String()
	for _, want := range []string{
		"Added: 1",
		"+ apps/remote-only",
		"Removed: 1",
		"- apps/local-only",
		"Changed: 1",
		"~ apps/shared",
		"type: \"local\" -> \"git\"",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("diff output missing %q:\n%s", want, output)
		}
	}
	after, err := os.ReadFile(manifestPath(workspaceB))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("workspace diff replaced the local manifest")
	}
}

func TestWorkspaceDiffRefusesInvalidRemoteManifestBeforeReplacingLocalManifest(t *testing.T) {
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
	before, err := os.ReadFile(manifestPath(workspace))
	if err != nil {
		t.Fatal(err)
	}
	hardeningWriteFile(t, filepath.Join(repo, syncedManifestName), "{not-json", 0o600)
	workspaceSyncCommitAndPush(t, repo, "bad manifest")

	_, err = DiffWorkspaceManifest()
	if err == nil || !strings.Contains(err.Error(), "invalid JSON") {
		t.Fatalf("diff invalid JSON error = %v", err)
	}
	after, err := os.ReadFile(manifestPath(workspace))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("workspace diff replaced the local manifest after invalid remote manifest")
	}
}

func TestWorkspaceDiffRefusesInvalidRemoteProjectPath(t *testing.T) {
	hardeningInitWorkspace(t, "code")
	remote, repo := workspaceSyncRemoteWithClone(t)
	if _, err := SetManifestRemote(remote); err != nil {
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

	_, err = DiffWorkspaceManifest()
	if err == nil || !strings.Contains(err.Error(), "escapes workspace") {
		t.Fatalf("diff traversal error = %v", err)
	}
}

func TestWorkspaceDiffRefusesCredentialedRemoteProject(t *testing.T) {
	hardeningInitWorkspace(t, "code")
	remote, repo := workspaceSyncRemoteWithClone(t)
	if _, err := SetManifestRemote(remote); err != nil {
		t.Fatal(err)
	}
	const credentialedRemote = "https://synthetic-user:synthetic-pat@example.invalid/app.git"
	unsafe := Manifest{
		Version:       ManifestVersion,
		WorkspaceRoot: ".",
		Projects:      []Project{hardeningProject("apps/app", ProjectTypeGit, credentialedRemote)},
	}
	data, err := manifestBytes(unsafe)
	if err != nil {
		t.Fatal(err)
	}
	hardeningWriteFile(t, filepath.Join(repo, syncedManifestName), string(data), 0o600)
	workspaceSyncCommitAndPush(t, repo, "credentialed manifest")

	_, err = DiffWorkspaceManifest()
	if err == nil {
		t.Fatal("expected credentialed remote to be rejected")
	}
	if !strings.Contains(err.Error(), "credentials") {
		t.Fatal("expected a credentials error")
	}
	if strings.Contains(err.Error(), "synthetic-pat") {
		t.Fatal("diff error leaked the credential")
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

func TestRedactRemoteStripsCredentials(t *testing.T) {
	cases := map[string]string{
		"https://user:token@github.com/o/r.git": "https://redacted@github.com/o/r.git",
		"ssh://git@github.com:22/o/r.git":       "ssh://redacted@github.com:22/o/r.git",
		"https://github.com/o/r.git":            "https://github.com/o/r.git",
		"git@github.com:o/r.git":                "git@github.com:o/r.git",
		"/local/bare/repo.git":                  "/local/bare/repo.git",
	}
	for in, want := range cases {
		if got := redactRemote(in); got != want {
			t.Errorf("redactRemote(%q) = %q, want %q", in, got, want)
		}
	}
}
