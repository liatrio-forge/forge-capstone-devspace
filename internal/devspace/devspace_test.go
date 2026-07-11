package devspace

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"charm.land/fang/v2"
	"filippo.io/age"
)

func TestInitWorkspaceIsIdempotent(t *testing.T) {
	home := t.TempDir()
	workspace := filepath.Join(t.TempDir(), "code")
	t.Setenv(envHome, home)

	first, err := InitWorkspace(workspace)
	if err != nil {
		t.Fatal(err)
	}
	firstIdentityPath, err := resolveAgeIdentityPath(first)
	if err != nil {
		t.Fatal(err)
	}
	identity, err := os.ReadFile(firstIdentityPath)
	if err != nil {
		t.Fatal(err)
	}
	second, err := InitWorkspace(workspace)
	if err != nil {
		t.Fatal(err)
	}
	secondIdentityPath, err := resolveAgeIdentityPath(second)
	if err != nil {
		t.Fatal(err)
	}
	identityAgain, err := os.ReadFile(secondIdentityPath)
	if err != nil {
		t.Fatal(err)
	}

	if first.MachineID != second.MachineID {
		t.Fatalf("machine id rotated: %s != %s", first.MachineID, second.MachineID)
	}
	if !bytes.Equal(identity, identityAgain) {
		t.Fatal("age identity rotated on second init")
	}
	if !exists(filepath.Join(workspace, ".devspace", "manifest.json")) {
		t.Fatal("manifest was not created")
	}
	if !exists(filepath.Join(home, "config.json")) {
		t.Fatal("config was not created in DEVSPACE_HOME")
	}
}

func TestValidateManifestRejectsUnsafeProjects(t *testing.T) {
	base := Manifest{Version: ManifestVersion, WorkspaceRoot: "/tmp/code"}
	cases := []struct {
		name    string
		input   Manifest
		wantErr string
	}{
		{name: "absolute path", input: Manifest{Version: ManifestVersion, WorkspaceRoot: "/tmp/code", Projects: []Project{
			{ID: "a", Name: "one", Path: "/abs", Type: ProjectTypeLocal, HydrateMode: HydrateManual},
		}}, wantErr: "invalid relative path"},
		{name: "duplicate path", input: Manifest{Version: ManifestVersion, WorkspaceRoot: "/tmp/code", Projects: []Project{
			{ID: "a", Name: "one", Path: "one", Type: ProjectTypeLocal, HydrateMode: HydrateManual},
			{ID: "b", Name: "two", Path: "one", Type: ProjectTypeLocal, HydrateMode: HydrateManual},
		}}, wantErr: "duplicate project path"},
		{name: "unsupported type", input: Manifest{Version: ManifestVersion, WorkspaceRoot: "/tmp/code", Projects: []Project{
			{ID: "a", Name: "one", Path: "one", Type: "weird", HydrateMode: HydrateManual},
		}}, wantErr: "unsupported type"},
		{name: "unsupported hydrate mode", input: Manifest{Version: ManifestVersion, WorkspaceRoot: "/tmp/code", Projects: []Project{
			{ID: "a", Name: "one", Path: "one", Type: ProjectTypeLocal, HydrateMode: "sometimes"},
		}}, wantErr: "unsupported hydrateMode"},
		{name: "traversal id", input: Manifest{Version: ManifestVersion, WorkspaceRoot: "/tmp/code", Projects: []Project{
			{ID: "../escape", Name: "one", Path: "one", Type: ProjectTypeLocal, HydrateMode: HydrateManual},
		}}, wantErr: "invalid id"},
		{name: "slash id", input: Manifest{Version: ManifestVersion, WorkspaceRoot: "/tmp/code", Projects: []Project{
			{ID: "team/project", Name: "one", Path: "one", Type: ProjectTypeLocal, HydrateMode: HydrateManual},
		}}, wantErr: "invalid id"},
		{name: "backslash id", input: Manifest{Version: ManifestVersion, WorkspaceRoot: "/tmp/code", Projects: []Project{
			{ID: `team\project`, Name: "one", Path: "one", Type: ProjectTypeLocal, HydrateMode: HydrateManual},
		}}, wantErr: "invalid id"},
		{name: "dot id", input: Manifest{Version: ManifestVersion, WorkspaceRoot: "/tmp/code", Projects: []Project{
			{ID: ".", Name: "one", Path: "one", Type: ProjectTypeLocal, HydrateMode: HydrateManual},
		}}, wantErr: "invalid id"},
		{name: "transport helper remote", input: Manifest{Version: ManifestVersion, WorkspaceRoot: "/tmp/code", Projects: []Project{
			{ID: "a", Name: "one", Path: "one", Type: ProjectTypeGit, Remote: "ext::sh -c id", HydrateMode: HydrateOnDemand},
		}}, wantErr: "transport-helper"},
	}
	if err := ValidateManifest(base); err != nil {
		t.Fatalf("base manifest should validate: %v", err)
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateManifest(tc.input)
			if err == nil {
				t.Fatalf("expected validation failure for %#v", tc.input)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("expected error containing %q, got %v", tc.wantErr, err)
			}
		})
	}
}

func TestValidateProjectRemote(t *testing.T) {
	cases := []struct {
		name    string
		remote  string
		wantErr string
	}{
		{name: "empty", remote: ""},
		{name: "https", remote: "https://github.com/x/y.git"},
		{name: "ssh url", remote: "ssh://git@host/x.git"},
		{name: "https ipv6", remote: "https://[::1]:8080/repo.git"},
		{name: "ssh ipv6", remote: "ssh://git@[2001:db8::1]/repo.git"},
		{name: "scp style", remote: "git@github.com:org/repo.git"},
		{name: "absolute path", remote: "/abs/local/path"},
		{name: "relative path", remote: "../local/repo.git"},
		{name: "http", remote: "http://git.example.test/x/y.git", wantErr: "unsupported scheme"},
		{name: "git url", remote: "git://host/x.git", wantErr: "unsupported scheme"},
		{name: "leading dash", remote: "-c core.sshCommand=sh", wantErr: "must not begin"},
		{name: "leading whitespace dash", remote: " -c core.sshCommand=sh", wantErr: "must not begin"},
		{name: "ext helper", remote: "ext::sh -c id", wantErr: "transport-helper"},
		{name: "fd helper", remote: "fd::17", wantErr: "transport-helper"},
		{name: "uppercase ext helper", remote: "EXT::sh -c id", wantErr: "transport-helper"},
		{name: "mixedcase ext helper", remote: "Ext::sh -c id", wantErr: "transport-helper"},
		{name: "leading whitespace ext helper", remote: " ext::sh -c id", wantErr: "transport-helper"},
		{name: "unsupported scheme", remote: "foo://host/x", wantErr: "unsupported scheme"},
		{name: "ssh opaque", remote: "ssh:opaque-thing", wantErr: "missing host"},
		{name: "https missing host", remote: "https:///path", wantErr: "missing host"},
		{name: "ssh host dash", remote: "ssh://-oProxyCommand=sh/repo", wantErr: "host must not begin"},
		{name: "embedded control", remote: "https://github.com/x\ny.git", wantErr: "control character"},
		{name: "https userinfo", remote: "https://synthetic-user:synthetic-pat@github.test/o/r.git", wantErr: "credentials"},
		{name: "https username-only userinfo", remote: "https://synthetic-pat@github.test/o/r.git", wantErr: "credentials"},
		{name: "ssh url userinfo", remote: "ssh://synthetic-user:synthetic-pat@github.test/o/r.git", wantErr: "credentials"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateProjectRemote(tc.remote)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("validateProjectRemote(%q) returned %v", tc.remote, err)
				}
				return
			}
			if err == nil {
				// tc.name, not tc.remote: credential-shaped rows must never be echoed.
				t.Fatalf("validateProjectRemote returned nil for %s", tc.name)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("expected error containing %q, got %v", tc.wantErr, err)
			}
		})
	}
}

func TestValidateProjectRemoteRejectsCredentialsWithoutEchoingThem(t *testing.T) {
	const credentialedRemote = "https://synthetic-user:synthetic-pat@github.test/o/r.git"
	err := validateProjectRemote(credentialedRemote)
	if err == nil {
		t.Fatal("expected credentialed remote to be rejected")
	}
	if !strings.Contains(err.Error(), "credentials") {
		t.Fatal("expected a credentials error")
	}
	if strings.Contains(err.Error(), "synthetic-pat") || strings.Contains(err.Error(), "synthetic-user") {
		t.Fatal("validation error echoed the credentialed remote")
	}
}

func TestExampleManifestValidates(t *testing.T) {
	var m Manifest
	if err := readJSON(filepath.Join("..", "..", "examples", "manifest.json"), &m); err != nil {
		t.Fatal(err)
	}
	if err := ValidateManifest(m); err != nil {
		t.Fatal(err)
	}
}

func TestWorkspaceScanDetectsGitAndSetup(t *testing.T) {
	home := t.TempDir()
	workspace := filepath.Join(t.TempDir(), "code")
	t.Setenv(envHome, home)
	if _, err := InitWorkspace(workspace); err != nil {
		t.Fatal(err)
	}

	app := filepath.Join(workspace, "work", "app")
	if err := os.MkdirAll(app, 0o755); err != nil {
		t.Fatal(err)
	}
	run(t, app, "git", "init", "-b", "main")
	run(t, app, "git", "config", "user.email", "test@example.com")
	run(t, app, "git", "config", "user.name", "Test User")
	write(t, filepath.Join(app, "package.json"), `{"scripts":{"dev":"vite"}}`)
	write(t, filepath.Join(app, ".env"), "DEV_DROP_ENV_PRESENT=1\n")
	run(t, app, "git", "add", "package.json")
	run(t, app, "git", "commit", "-m", "initial")
	write(t, filepath.Join(app, "README.md"), "dirty\n")

	summary, err := ScanWorkspace()
	if err != nil {
		t.Fatal(err)
	}
	if summary.FoundProjects != 1 || summary.GitRepos != 1 || summary.ProjectsWithEnv != 1 {
		t.Fatalf("unexpected scan summary: %+v", summary)
	}
	m, err := LoadManifest(workspace)
	if err != nil {
		t.Fatal(err)
	}
	p, ok := findProject(m, "app")
	if !ok {
		t.Fatal("scanned project not in manifest")
	}
	if p.Setup.InstallCommand != "npm install" {
		t.Fatalf("setup hint = %q", p.Setup.InstallCommand)
	}
	st, err := LoadState()
	if err != nil {
		t.Fatal(err)
	}
	if !st.Projects[p.ID].Dirty {
		t.Fatal("dirty git repo was not reported dirty")
	}
}

func TestGitInfoStripsRemoteUserinfo(t *testing.T) {
	dir := t.TempDir()
	run(t, dir, "git", "init", "-b", "main")
	run(t, dir, "git", "config", "user.email", "test@example.com")
	run(t, dir, "git", "config", "user.name", "Test User")
	run(t, dir, "git", "remote", "add", "origin", "https://synthetic-user:synthetic-pat@example.invalid/app.git")

	info := gitInfo(dir)
	if !info.IsRepo {
		t.Fatal("expected repo to be detected")
	}
	if strings.Contains(info.Remote, "synthetic-pat") {
		t.Fatal("gitInfo persisted a credential in the remote")
	}
	if info.Remote != "https://example.invalid/app.git" {
		t.Fatalf("gitInfo.Remote = %q, want credential-free URL", info.Remote)
	}
}

func TestStripRemoteUserinfo(t *testing.T) {
	cases := map[string]string{
		"https://synthetic-user:synthetic-pat@github.test/o/r.git": "https://github.test/o/r.git",
		// PAT-as-username shape: username-only userinfo is still a credential on HTTPS.
		"https://synthetic-pat@github.test/o/r.git":   "https://github.test/o/r.git",
		"ssh://git@github.test/o/r.git":               "ssh://git@github.test/o/r.git",
		"ssh://git:synthetic-pat@github.test/o/r.git": "ssh://github.test/o/r.git",
		"git@github.test:o/r.git":                     "git@github.test:o/r.git",
		"/local/bare/repo.git":                        "/local/bare/repo.git",
	}
	for in, want := range cases {
		if got := stripRemoteUserinfo(in); got != want {
			// want only, never the credential-shaped input.
			t.Errorf("stripRemoteUserinfo did not produce %q", want)
		}
	}
}

func TestGitInfoPreservesSSHLoginUserWithoutPassword(t *testing.T) {
	dir := t.TempDir()
	run(t, dir, "git", "init", "-b", "main")
	run(t, dir, "git", "config", "user.email", "test@example.com")
	run(t, dir, "git", "config", "user.name", "Test User")
	const sshRemote = "ssh://git@example.invalid/app.git"
	run(t, dir, "git", "remote", "add", "origin", sshRemote)

	info := gitInfo(dir)
	if info.Remote != sshRemote {
		t.Fatalf("gitInfo.Remote = %q, want unchanged %q (SSH login user is not a credential)", info.Remote, sshRemote)
	}
}

func TestWorkspaceScanNeverPersistsRemoteCredentials(t *testing.T) {
	home := t.TempDir()
	workspace := filepath.Join(t.TempDir(), "code")
	t.Setenv(envHome, home)
	if _, err := InitWorkspace(workspace); err != nil {
		t.Fatal(err)
	}

	app := filepath.Join(workspace, "work", "app")
	if err := os.MkdirAll(app, 0o755); err != nil {
		t.Fatal(err)
	}
	run(t, app, "git", "init", "-b", "main")
	run(t, app, "git", "config", "user.email", "test@example.com")
	run(t, app, "git", "config", "user.name", "Test User")
	write(t, filepath.Join(app, "README.md"), "hello\n")
	run(t, app, "git", "add", "README.md")
	run(t, app, "git", "commit", "-m", "initial")
	run(t, app, "git", "remote", "add", "origin", "https://synthetic-user:synthetic-pat@example.invalid/app.git")

	if _, err := ScanWorkspace(); err != nil {
		t.Fatal(err)
	}

	raw, err := os.ReadFile(manifestPath(workspace))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(raw, []byte("synthetic-pat")) {
		t.Fatal("manifest on disk contains a credential")
	}

	m, err := LoadManifest(workspace)
	if err != nil {
		t.Fatal(err)
	}
	p, ok := findProject(m, "app")
	if !ok {
		t.Fatal("scanned project not in manifest")
	}
	if p.Remote != "https://example.invalid/app.git" {
		t.Fatalf("scanned remote = %q, want credential-free URL", p.Remote)
	}
}

func TestScanTreatsMonorepoAsOneProject(t *testing.T) {
	home := t.TempDir()
	workspace := filepath.Join(t.TempDir(), "code")
	t.Setenv(envHome, home)
	if _, err := InitWorkspace(workspace); err != nil {
		t.Fatal(err)
	}

	write(t, filepath.Join(workspace, "mono", "package.json"), `{"name":"mono"}`)
	write(t, filepath.Join(workspace, "mono", "packages", "a", "package.json"), `{"name":"a"}`)
	write(t, filepath.Join(workspace, "mono", "packages", "b", "package.json"), `{"name":"b"}`)
	write(t, filepath.Join(workspace, "mono", "packages", "a", ".env"), "TOKEN=a\n")

	vendorTool := filepath.Join(workspace, "mono", "vendor-tool")
	if err := os.MkdirAll(vendorTool, 0o755); err != nil {
		t.Fatal(err)
	}
	run(t, vendorTool, "git", "init", "-b", "main")

	write(t, filepath.Join(workspace, "apps", "x", "package.json"), `{"name":"x"}`)
	write(t, filepath.Join(workspace, "apps", "y", "package.json"), `{"name":"y"}`)

	if _, err := ScanWorkspace(); err != nil {
		t.Fatal(err)
	}
	m, err := LoadManifest(workspace)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Projects) != 4 {
		t.Fatalf("expected mono, nested git repo, and two sibling apps; got %+v", m.Projects)
	}

	want := map[string]string{
		"mono":             ProjectTypeLocal,
		"mono/vendor-tool": ProjectTypeGit,
		"apps/x":           ProjectTypeLocal,
		"apps/y":           ProjectTypeLocal,
	}
	for path, typ := range want {
		p, ok := findProject(m, path)
		if !ok {
			t.Fatalf("missing project %q in %+v", path, m.Projects)
		}
		if p.Type != typ {
			t.Fatalf("%s type = %q, want %q", path, p.Type, typ)
		}
	}
	for _, path := range []string{"mono/packages/a", "mono/packages/b"} {
		if _, ok := findProject(m, path); ok {
			t.Fatalf("nested package %q should not be tracked: %+v", path, m.Projects)
		}
	}
}

func TestRemoveProjectUntracksAndCascades(t *testing.T) {
	home := t.TempDir()
	workspace := filepath.Join(t.TempDir(), "code")
	t.Setenv(envHome, home)
	if _, err := InitWorkspace(workspace); err != nil {
		t.Fatal(err)
	}
	appPath := filepath.Join(workspace, "work", "app")
	if err := os.MkdirAll(appPath, 0o755); err != nil {
		t.Fatal(err)
	}
	app, err := AddProject("work/app")
	if err != nil {
		t.Fatal(err)
	}
	otherPath := filepath.Join(workspace, "work", "other")
	if err := os.MkdirAll(otherPath, 0o755); err != nil {
		t.Fatal(err)
	}
	other, err := AddProject("work/other")
	if err != nil {
		t.Fatal(err)
	}
	identity, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatal(err)
	}
	user := User{
		ID:           "user_local",
		Name:         "Local User",
		AgeRecipient: identity.Recipient().String(),
		CreatedAt:    nowRFC3339(),
	}
	m, err := LoadManifest(workspace)
	if err != nil {
		t.Fatal(err)
	}
	m.Users = []User{user}
	m.Access = []ProjectAccess{
		{ProjectID: app.ID, UserID: user.ID, Role: AccessRoleOwner, AddedAt: nowRFC3339()},
		{ProjectID: other.ID, UserID: user.ID, Role: AccessRoleViewer, AddedAt: nowRFC3339()},
	}
	if err := SaveManifest(workspace, m); err != nil {
		t.Fatal(err)
	}

	removed, err := RemoveProject(app.Name)
	if err != nil {
		t.Fatal(err)
	}
	if removed.ID != app.ID {
		t.Fatalf("removed project = %s, want %s", removed.ID, app.ID)
	}
	m, err = LoadManifest(workspace)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := findProject(m, app.ID); ok {
		t.Fatal("removed project is still in manifest")
	}
	if _, ok := findProject(m, other.ID); !ok {
		t.Fatal("unrelated project was removed")
	}
	if len(m.Users) != 1 || m.Users[0].ID != user.ID {
		t.Fatalf("users were not preserved: %+v", m.Users)
	}
	if len(m.Access) != 1 || m.Access[0].ProjectID != other.ID {
		t.Fatalf("access cascade = %+v, want only unrelated project access", m.Access)
	}
	if err := ValidateManifest(m); err != nil {
		t.Fatal(err)
	}
	st, err := LoadState()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := st.Projects[app.ID]; ok {
		t.Fatal("removed project state remains")
	}
	if _, ok := st.Projects[other.ID]; !ok {
		t.Fatal("unrelated project state was removed")
	}
	if !exists(appPath) {
		t.Fatal("project folder was deleted")
	}
}

func TestRemoveProjectByPathAndID(t *testing.T) {
	home := t.TempDir()
	workspace := filepath.Join(t.TempDir(), "code")
	t.Setenv(envHome, home)
	if _, err := InitWorkspace(workspace); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(workspace, "apps", "by-path"), 0o755); err != nil {
		t.Fatal(err)
	}
	byPath, err := AddProject("apps/by-path")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(workspace, "apps", "by-id"), 0o755); err != nil {
		t.Fatal(err)
	}
	byID, err := AddProject("apps/by-id")
	if err != nil {
		t.Fatal(err)
	}

	if _, err := RemoveProject(byPath.Path); err != nil {
		t.Fatal(err)
	}
	if _, err := RemoveProject(byID.ID); err != nil {
		t.Fatal(err)
	}
	m, err := LoadManifest(workspace)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := findProject(m, byPath.ID); ok {
		t.Fatal("path ref removal left project in manifest")
	}
	if _, ok := findProject(m, byID.ID); ok {
		t.Fatal("id ref removal left project in manifest")
	}
}

func TestRemoveProjectNotFoundLeavesFilesUnchanged(t *testing.T) {
	home := t.TempDir()
	workspace := filepath.Join(t.TempDir(), "code")
	t.Setenv(envHome, home)
	if _, err := InitWorkspace(workspace); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(workspace, "work", "app"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := AddProject("work/app"); err != nil {
		t.Fatal(err)
	}
	manifestBefore, err := os.ReadFile(manifestPath(workspace))
	if err != nil {
		t.Fatal(err)
	}
	stateFile, err := statePath()
	if err != nil {
		t.Fatal(err)
	}
	stateBefore, err := os.ReadFile(stateFile)
	if err != nil {
		t.Fatal(err)
	}

	_, err = RemoveProject("missing")
	if err == nil {
		t.Fatal("expected missing project error")
	}
	if !strings.Contains(err.Error(), `project "missing" not found`) {
		t.Fatalf("unexpected error: %v", err)
	}
	manifestAfter, err := os.ReadFile(manifestPath(workspace))
	if err != nil {
		t.Fatal(err)
	}
	stateAfter, err := os.ReadFile(stateFile)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(manifestBefore, manifestAfter) {
		t.Fatal("manifest changed for missing project")
	}
	if !bytes.Equal(stateBefore, stateAfter) {
		t.Fatal("state changed for missing project")
	}
}

func TestRemoveProjectRescanBehavior(t *testing.T) {
	home := t.TempDir()
	workspace := filepath.Join(t.TempDir(), "code")
	t.Setenv(envHome, home)
	if _, err := InitWorkspace(workspace); err != nil {
		t.Fatal(err)
	}
	livePath := filepath.Join(workspace, "work", "live")
	write(t, filepath.Join(livePath, "package.json"), `{"scripts":{"dev":"vite"}}`)
	live, err := AddProject("work/live")
	if err != nil {
		t.Fatal(err)
	}
	deletedPath := filepath.Join(workspace, "work", "deleted")
	write(t, filepath.Join(deletedPath, "package.json"), `{"scripts":{"dev":"vite"}}`)
	deleted, err := AddProject("work/deleted")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := RemoveProject(live.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := RemoveProject(deleted.ID); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(deletedPath); err != nil {
		t.Fatal(err)
	}

	if _, err := ScanWorkspace(); err != nil {
		t.Fatal(err)
	}
	m, err := LoadManifest(workspace)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := findProject(m, live.ID); !ok {
		t.Fatal("rescan did not re-track live project")
	}
	if _, ok := findProject(m, deleted.ID); ok {
		t.Fatal("rescan re-tracked deleted project")
	}
}

func TestProjectUntrackCommandOutputRetainsSecrets(t *testing.T) {
	home := t.TempDir()
	workspace := filepath.Join(t.TempDir(), "code")
	t.Setenv(envHome, home)
	if _, err := InitWorkspace(workspace); err != nil {
		t.Fatal(err)
	}
	projectPath := filepath.Join(workspace, "work", "app")
	if err := os.MkdirAll(projectPath, 0o755); err != nil {
		t.Fatal(err)
	}
	p, err := AddProject("work/app")
	if err != nil {
		t.Fatal(err)
	}
	secretDir := filepath.Join(workspaceDevdrop(workspace), "secrets", p.ID)
	secretFile := filepath.Join(secretDir, "dev.age")
	write(t, secretFile, "ciphertext\n")

	cmd := NewRootCommand("test")
	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs([]string{"project", "untrack", "app"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("command failed: %v\n%s", err, errOut.String())
	}
	got := out.String()
	wantRemoved := "Untracked project app (work/app) from the manifest. Files on disk were not touched.\n"
	if !strings.Contains(got, wantRemoved) {
		t.Fatalf("missing removal output %q in:\n%s", wantRemoved, got)
	}
	wantNote := "Note: encrypted env profiles remain at " + secretDir + "; delete them manually if no longer needed.\n"
	if !strings.Contains(got, wantNote) {
		t.Fatalf("missing secret retention note %q in:\n%s", wantNote, got)
	}
	if !exists(projectPath) {
		t.Fatal("project folder was deleted")
	}
	if !exists(secretFile) {
		t.Fatal("secret file was deleted")
	}
}

func TestSyncCreatesPlaceholderAndHydrateClonesLocalRemote(t *testing.T) {
	home := t.TempDir()
	workspace := filepath.Join(t.TempDir(), "code")
	t.Setenv(envHome, home)
	cfg, err := InitWorkspace(workspace)
	if err != nil {
		t.Fatal(err)
	}
	remote := makeBareRepo(t)
	m := Manifest{
		Version:       ManifestVersion,
		WorkspaceRoot: workspace,
		Machines:      []Machine{machineFromConfig(cfg)},
		Projects: []Project{{
			ID:            projectID("work/app"),
			Name:          "app",
			Path:          "work/app",
			Type:          ProjectTypeGit,
			Remote:        remote,
			DefaultBranch: "main",
			HydrateMode:   HydrateOnDemand,
			Ignore:        DefaultIgnores,
		}},
	}
	if err := SaveManifest(workspace, m); err != nil {
		t.Fatal(err)
	}
	plan, err := BuildPlan()
	if err != nil {
		t.Fatal(err)
	}
	if err := SaveLastPlan(plan); err != nil {
		t.Fatal(err)
	}
	applied, err := ApplyLastPlan()
	if err != nil {
		t.Fatal(err)
	}
	if len(applied.Actions) != 1 || applied.Actions[0].Kind != "placeholder" {
		t.Fatalf("unexpected sync actions: %+v", applied.Actions)
	}
	if !exists(filepath.Join(workspace, "work", "app")) {
		t.Fatal("placeholder not created")
	}
	st, err := LoadState()
	if err != nil {
		t.Fatal(err)
	}
	if !st.Projects[projectID("work/app")].Placeholder {
		t.Fatal("sync did not refresh placeholder state")
	}
	if _, err := HydrateProject("app"); err != nil {
		t.Fatal(err)
	}
	if !exists(filepath.Join(workspace, "work", "app", ".git")) {
		t.Fatal("repo was not cloned")
	}
}

func TestHydrateRejectsUnsupportedRemote(t *testing.T) {
	home := t.TempDir()
	workspace := filepath.Join(t.TempDir(), "code")
	t.Setenv(envHome, home)
	cfg, err := InitWorkspace(workspace)
	if err != nil {
		t.Fatal(err)
	}
	m := Manifest{
		Version:       ManifestVersion,
		WorkspaceRoot: workspace,
		Machines:      []Machine{machineFromConfig(cfg)},
		Projects: []Project{{
			ID:          projectID("work/app"),
			Name:        "app",
			Path:        "work/app",
			Type:        ProjectTypeGit,
			Remote:      "ext::sh -c id",
			HydrateMode: HydrateOnDemand,
			Ignore:      append([]string{}, DefaultIgnores...),
		}},
	}
	if err := writeJSON(manifestPath(workspace), m, 0o600); err != nil {
		t.Fatal(err)
	}

	_, err = HydrateProject("app")
	if err == nil {
		t.Fatal("expected unsupported remote to be rejected")
	}
	if !strings.Contains(err.Error(), "transport-helper") {
		t.Fatalf("expected transport-helper error, got %v", err)
	}
	if exists(filepath.Join(workspace, "work", "app")) {
		t.Fatal("hydrate created destination for unsupported remote")
	}
}

func TestHydrateRejectsCredentialedRemote(t *testing.T) {
	home := t.TempDir()
	workspace := filepath.Join(t.TempDir(), "code")
	t.Setenv(envHome, home)
	cfg, err := InitWorkspace(workspace)
	if err != nil {
		t.Fatal(err)
	}
	const credentialedRemote = "https://synthetic-user:synthetic-pat@example.invalid/app.git"
	m := Manifest{
		Version:       ManifestVersion,
		WorkspaceRoot: workspace,
		Machines:      []Machine{machineFromConfig(cfg)},
		Projects: []Project{{
			ID:          projectID("work/app"),
			Name:        "app",
			Path:        "work/app",
			Type:        ProjectTypeGit,
			Remote:      credentialedRemote,
			HydrateMode: HydrateOnDemand,
			Ignore:      append([]string{}, DefaultIgnores...),
		}},
	}
	if err := writeJSON(manifestPath(workspace), m, 0o600); err != nil {
		t.Fatal(err)
	}

	_, err = HydrateProject("app")
	if err == nil {
		t.Fatal("expected credentialed remote to be rejected")
	}
	if !strings.Contains(err.Error(), "credentials") {
		t.Fatal("expected a credentials error")
	}
	if strings.Contains(err.Error(), "synthetic-pat") {
		t.Fatal("hydrate error leaked the credential")
	}
	if exists(filepath.Join(workspace, "work", "app")) {
		t.Fatal("hydrate created destination for credentialed remote")
	}
}

func TestLoadManifestNamesManifestFileWhenRemoteHasCredentials(t *testing.T) {
	home := t.TempDir()
	workspace := filepath.Join(t.TempDir(), "code")
	t.Setenv(envHome, home)
	if _, err := InitWorkspace(workspace); err != nil {
		t.Fatal(err)
	}
	m := Manifest{
		Version:       ManifestVersion,
		WorkspaceRoot: workspace,
		Projects: []Project{{
			ID:          projectID("work/app"),
			Name:        "app",
			Path:        "work/app",
			Type:        ProjectTypeGit,
			Remote:      "https://synthetic-user:synthetic-pat@example.invalid/app.git",
			HydrateMode: HydrateOnDemand,
			Ignore:      append([]string{}, DefaultIgnores...),
		}},
	}
	if err := writeJSON(manifestPath(workspace), m, 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := LoadManifest(workspace)
	if err == nil {
		t.Fatal("expected credentialed manifest to fail loading")
	}
	// The only remedy is hand-editing the manifest, so the error must say where it is.
	// No %v: the error wraps credential-shaped input and must never be echoed.
	if !strings.Contains(err.Error(), manifestPath(workspace)) {
		t.Fatal("error does not name the manifest file")
	}
	if !strings.Contains(err.Error(), "credentials") {
		t.Fatal("expected a credentials error")
	}
	if strings.Contains(err.Error(), "synthetic-pat") {
		t.Fatal("load error leaked the credential")
	}
}

func TestBuildPlanRedactsRemoteMismatchWarning(t *testing.T) {
	home := t.TempDir()
	workspace := filepath.Join(t.TempDir(), "code")
	t.Setenv(envHome, home)
	if _, err := InitWorkspace(workspace); err != nil {
		t.Fatal(err)
	}

	app := filepath.Join(workspace, "work", "app")
	if err := os.MkdirAll(app, 0o755); err != nil {
		t.Fatal(err)
	}
	run(t, app, "git", "init", "-b", "main")
	run(t, app, "git", "config", "user.email", "test@example.com")
	run(t, app, "git", "config", "user.name", "Test User")
	write(t, filepath.Join(app, "README.md"), "hello\n")
	run(t, app, "git", "add", "README.md")
	run(t, app, "git", "commit", "-m", "initial")
	// Simulate a checkout whose local origin still carries a credential,
	// independent of whatever the synced manifest records.
	run(t, app, "git", "remote", "add", "origin", "https://synthetic-user:synthetic-pat@example.invalid/app.git")

	m := Manifest{
		Version:       ManifestVersion,
		WorkspaceRoot: workspace,
		Projects: []Project{{
			ID:          projectID("work/app"),
			Name:        "app",
			Path:        "work/app",
			Type:        ProjectTypeGit,
			Remote:      "https://example.invalid/app-manifest.git",
			HydrateMode: HydrateOnDemand,
			Ignore:      append([]string{}, DefaultIgnores...),
		}},
	}
	if err := SaveManifest(workspace, m); err != nil {
		t.Fatal(err)
	}

	plan, err := BuildPlan()
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(plan.Warnings, "\n")
	if strings.Contains(joined, "synthetic-pat") {
		t.Fatal("plan warning leaked a credential")
	}
	if !strings.Contains(joined, "different Git remote") {
		t.Fatalf("expected a remote mismatch warning, got %v", plan.Warnings)
	}
}

func TestCloneRepoRedactsCredentialsOnFailure(t *testing.T) {
	const credentialedRemote = "https://synthetic-user:synthetic-pat@127.0.0.1:1/repo.git"
	err := cloneRepo(credentialedRemote, filepath.Join(t.TempDir(), "out"))
	if err == nil {
		t.Fatal("expected clone against a closed port to fail")
	}
	if strings.Contains(err.Error(), "synthetic-pat") {
		t.Fatal("clone error leaked the credential")
	}
	if !strings.Contains(err.Error(), "redacted") {
		t.Fatalf("expected the redacted remote to appear in the clone error, got %v", err)
	}
}

func TestSecretPathRejectsUnsafeProjectID(t *testing.T) {
	home := t.TempDir()
	workspace := filepath.Join(t.TempDir(), "code")
	t.Setenv(envHome, home)
	cfg, err := InitWorkspace(workspace)
	if err != nil {
		t.Fatal(err)
	}

	err = writeSecretProfile(cfg, SecretProfile{
		ProjectID: "../escape",
		Profile:   "dev",
		Values:    map[string]string{"TOKEN": "example"},
	})
	if err == nil {
		t.Fatal("expected unsafe project id to be rejected")
	}
	if !strings.Contains(err.Error(), "project id") {
		t.Fatalf("expected project id error, got %v", err)
	}
	if exists(filepath.Join(workspace, ".devspace", "escape", "dev.age")) {
		t.Fatal("unsafe project id wrote outside the secrets directory")
	}
}

func TestEnvPullReplacesSymlinkedEnvFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink replacement semantics are covered on Linux CI")
	}
	home := t.TempDir()
	workspace := filepath.Join(t.TempDir(), "code")
	t.Setenv(envHome, home)
	if _, err := InitWorkspace(workspace); err != nil {
		t.Fatal(err)
	}
	projectPath := filepath.Join(workspace, "work", "api")
	if err := os.MkdirAll(projectPath, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := AddProject("work/api"); err != nil {
		t.Fatal(err)
	}
	if err := EnvSet("api", "TOKEN", "dev", strings.NewReader("example-token\n")); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "target.txt")
	write(t, target, "unchanged\n")
	envPath := filepath.Join(projectPath, ".env")
	if err := os.Symlink(target, envPath); err != nil {
		t.Fatal(err)
	}

	pulled, err := EnvPull("api", "dev")
	if err != nil {
		t.Fatal(err)
	}
	if pulled != envPath {
		t.Fatalf("unexpected env path: %s", pulled)
	}
	info, err := os.Lstat(envPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		t.Fatal(".env is still a symlink after EnvPull")
	}
	targetData, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(targetData) != "unchanged\n" {
		t.Fatal("EnvPull followed the symlink target")
	}
}

func TestSecretWritesLeaveNoBackupFiles(t *testing.T) {
	home := t.TempDir()
	workspace := filepath.Join(t.TempDir(), "code")
	t.Setenv(envHome, home)
	if _, err := InitWorkspace(workspace); err != nil {
		t.Fatal(err)
	}
	projectPath := filepath.Join(workspace, "work", "api")
	if err := os.MkdirAll(projectPath, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := AddProject("work/api"); err != nil {
		t.Fatal(err)
	}
	if err := EnvSet("api", "TOKEN", "dev", strings.NewReader("example-token\n")); err != nil {
		t.Fatal(err)
	}
	teammate, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatal(err)
	}
	invited, err := EnvInvite("api", "dev", "teammate", teammate.Recipient().String(), "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := EnvRevoke("api", "dev", invited.ID, "offboarding"); err != nil {
		t.Fatal(err)
	}
	secretsDir := filepath.Join(workspaceDevdrop(workspace), "secrets")
	err = filepath.WalkDir(secretsDir, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		name := d.Name()
		if strings.HasSuffix(name, ".bak") || strings.Contains(name, ".tmp-") {
			t.Fatalf("unexpected backup or temp file under secrets dir: %s", path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestSafeWorkspacePathRejectsSymlinkEscape(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink containment is covered on Linux CI")
	}
	workspace := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(workspace, "linked")); err != nil {
		t.Fatal(err)
	}
	if _, _, err := safeWorkspacePath(workspace, "linked/project"); err == nil || !strings.Contains(err.Error(), "escapes workspace via symlink") {
		t.Fatalf("expected symlink escape error, got %v", err)
	}
	if err := os.MkdirAll(filepath.Join(workspace, "real", "project"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, clean, err := safeWorkspacePath(workspace, "real/project"); err != nil || clean != "real/project" {
		t.Fatalf("expected real subdirectory to resolve, clean=%q err=%v", clean, err)
	}
}

func TestScanRejectsSymlinkEscapeProjectPath(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink containment is covered on Linux CI")
	}
	home := t.TempDir()
	workspace := filepath.Join(t.TempDir(), "code")
	t.Setenv(envHome, home)
	cfg, err := InitWorkspace(workspace)
	if err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(workspace, "linked")); err != nil {
		t.Fatal(err)
	}
	m := Manifest{
		Version:       ManifestVersion,
		WorkspaceRoot: workspace,
		Machines:      []Machine{machineFromConfig(cfg)},
		Projects: []Project{{
			ID:          projectID("linked/project"),
			Name:        "escape",
			Path:        "linked/project",
			Type:        ProjectTypeGit,
			Remote:      makeBareRepo(t),
			HydrateMode: HydrateOnDemand,
			Ignore:      append([]string{}, DefaultIgnores...),
		}},
	}
	if err := writeJSON(manifestPath(workspace), m, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := HydrateProject("escape"); err == nil || !strings.Contains(err.Error(), "escapes workspace via symlink") {
		t.Fatalf("expected symlink escape error, got %v", err)
	}
	if exists(filepath.Join(outside, "project")) {
		t.Fatal("hydrate created a project outside the workspace")
	}
}

func TestMergeProjectPreservesUserOverrides(t *testing.T) {
	cases := []struct {
		name string
		old  Project
		next Project
		want Project
	}{
		{
			name: "git custom hydrate mode wins",
			old:  Project{ID: "p", Type: ProjectTypeGit, HydrateMode: HydrateImmediate},
			next: Project{ID: "p", Type: ProjectTypeGit, HydrateMode: HydrateOnDemand},
			want: Project{ID: "p", Type: ProjectTypeGit, HydrateMode: HydrateImmediate},
		},
		{
			name: "custom ignore wins",
			old:  Project{ID: "p", Type: ProjectTypeLocal, HydrateMode: HydrateManual, Ignore: []string{"tmp"}},
			next: Project{ID: "p", Type: ProjectTypeLocal, HydrateMode: HydrateManual, Ignore: append([]string{}, DefaultIgnores...)},
			want: Project{ID: "p", Type: ProjectTypeLocal, HydrateMode: HydrateManual, Ignore: []string{"tmp"}},
		},
		{
			name: "local default upgrades to git default",
			old:  Project{ID: "p", Type: ProjectTypeLocal, HydrateMode: HydrateManual},
			next: Project{ID: "p", Type: ProjectTypeGit, HydrateMode: HydrateOnDemand},
			want: Project{ID: "p", Type: ProjectTypeGit, HydrateMode: HydrateOnDemand},
		},
		{
			name: "local custom hydrate mode survives git upgrade",
			old:  Project{ID: "p", Type: ProjectTypeLocal, HydrateMode: HydrateMetadataOnly},
			next: Project{ID: "p", Type: ProjectTypeGit, HydrateMode: HydrateOnDemand},
			want: Project{ID: "p", Type: ProjectTypeGit, HydrateMode: HydrateMetadataOnly},
		},
		{
			name: "env profiles still preserved",
			old:  Project{ID: "p", Type: ProjectTypeLocal, HydrateMode: HydrateManual, EnvProfiles: []string{"dev"}},
			next: Project{ID: "p", Type: ProjectTypeLocal, HydrateMode: HydrateManual},
			want: Project{ID: "p", Type: ProjectTypeLocal, HydrateMode: HydrateManual, EnvProfiles: []string{"dev"}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := mergeProject(tc.old, tc.next)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("mergeProject() = %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestScanPreservesManualHydrateModeAndIgnore(t *testing.T) {
	home := t.TempDir()
	workspace := filepath.Join(t.TempDir(), "code")
	t.Setenv(envHome, home)
	if _, err := InitWorkspace(workspace); err != nil {
		t.Fatal(err)
	}
	app := filepath.Join(workspace, "work", "app")
	if err := os.MkdirAll(app, 0o755); err != nil {
		t.Fatal(err)
	}
	run(t, app, "git", "init", "-b", "main")
	run(t, app, "git", "config", "user.email", "test@example.com")
	run(t, app, "git", "config", "user.name", "Test User")
	write(t, filepath.Join(app, "README.md"), "hello\n")
	run(t, app, "git", "add", "README.md")
	run(t, app, "git", "commit", "-m", "initial")

	if _, err := ScanWorkspace(); err != nil {
		t.Fatal(err)
	}
	m, err := LoadManifest(workspace)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Projects) != 1 {
		t.Fatalf("expected one project, got %d", len(m.Projects))
	}
	m.Projects[0].HydrateMode = HydrateImmediate
	m.Projects[0].Ignore = []string{"custom"}
	if err := SaveManifest(workspace, m); err != nil {
		t.Fatal(err)
	}
	if _, err := ScanWorkspace(); err != nil {
		t.Fatal(err)
	}
	m, err = LoadManifest(workspace)
	if err != nil {
		t.Fatal(err)
	}
	if got := m.Projects[0].HydrateMode; got != HydrateImmediate {
		t.Fatalf("hydrateMode = %q, want %q", got, HydrateImmediate)
	}
	if len(m.Projects[0].Ignore) != 1 || m.Projects[0].Ignore[0] != "custom" {
		t.Fatalf("ignore = %v, want [custom]", m.Projects[0].Ignore)
	}
}

func TestEncryptedEnvProfilesRoundTripWithoutPlaintextStorage(t *testing.T) {
	home := t.TempDir()
	workspace := filepath.Join(t.TempDir(), "code")
	t.Setenv(envHome, home)
	cfg, err := InitWorkspace(workspace)
	if err != nil {
		t.Fatal(err)
	}
	projectPath := filepath.Join(workspace, "work", "api")
	if err := os.MkdirAll(projectPath, 0o755); err != nil {
		t.Fatal(err)
	}
	p, err := AddProject("work/api")
	if err != nil {
		t.Fatal(err)
	}
	envValue := strings.Repeat("x", 12)
	if err := EnvSet("api", "DATABASE_URL", "dev", strings.NewReader(envValue+"\n")); err != nil {
		t.Fatal(err)
	}
	keys, err := EnvList("api", "dev")
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 1 || keys[0] != "DATABASE_URL" {
		t.Fatalf("unexpected env keys: %v", keys)
	}
	ciphertext, err := os.ReadFile(mustSecretPath(t, cfg, p.ID, "dev"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(ciphertext, []byte(envValue)) {
		t.Fatal("secret stored in plaintext")
	}
	envPath, err := EnvPull("api", "dev")
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(envPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "DATABASE_URL="+envValue+"\n" {
		t.Fatal("unexpected .env content")
	}
	info, err := os.Stat(envPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf(".env permissions = %o", info.Mode().Perm())
	}
	st, err := LoadState()
	if err != nil {
		t.Fatal(err)
	}
	if !st.Projects[p.ID].EnvFilePresent {
		t.Fatal("state was not refreshed after env pull")
	}
}

func TestEncryptedEnvProfilesCanInviteAndRevokeRecipients(t *testing.T) {
	home := t.TempDir()
	workspace := filepath.Join(t.TempDir(), "code")
	t.Setenv(envHome, home)
	cfg, err := InitWorkspace(workspace)
	if err != nil {
		t.Fatal(err)
	}
	projectPath := filepath.Join(workspace, "work", "api")
	if err := os.MkdirAll(projectPath, 0o755); err != nil {
		t.Fatal(err)
	}
	p, err := AddProject("work/api")
	if err != nil {
		t.Fatal(err)
	}
	envValue := strings.Repeat("y", 12)
	if err := EnvSet("api", "TOKEN", "dev", strings.NewReader(envValue+"\n")); err != nil {
		t.Fatal(err)
	}

	teammate, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatal(err)
	}
	invited, err := EnvInvite("api", "dev", "teammate", teammate.Recipient().String(), "platform")
	if err != nil {
		t.Fatal(err)
	}
	if invited.ID == "" {
		t.Fatal("invite did not return a recipient id")
	}
	recipients, err := EnvRecipients("api", "dev")
	if err != nil {
		t.Fatal(err)
	}
	if len(recipients) != 2 {
		t.Fatalf("expected local and teammate recipients, got %d", len(recipients))
	}
	if recipients[0].Name > recipients[1].Name {
		t.Fatalf("recipients are not sorted by name: %+v", recipients)
	}
	for _, recipient := range recipients {
		if recipient.RevokedAt != "" {
			t.Fatalf("recipient should be active before revoke: %+v", recipient)
		}
	}
	profile := decryptSecretProfileForTest(t, mustSecretPath(t, cfg, p.ID, "dev"), teammate)
	if len(profile.Values) != 1 || profile.Values["TOKEN"] == "" {
		t.Fatal("teammate could not decrypt invited profile")
	}
	if len(profile.Recipients) != 2 {
		t.Fatalf("expected local and teammate recipients, got %d", len(profile.Recipients))
	}
	m, err := LoadManifest(workspace)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Users) != 2 || len(m.Teams) != 1 {
		t.Fatalf("manifest access metadata not populated: users=%d teams=%d", len(m.Users), len(m.Teams))
	}
	if len(m.Access) != 2 {
		t.Fatalf("expected owner and team project access, got %d", len(m.Access))
	}

	revoked, err := EnvRevoke("api", "dev", invited.ID, "offboarding")
	if err != nil {
		t.Fatal(err)
	}
	if revoked.ID != invited.ID {
		t.Fatal("revoked the wrong recipient")
	}
	recipients, err = EnvRecipients("api", "dev")
	if err != nil {
		t.Fatal(err)
	}
	var sawRevoked bool
	for _, recipient := range recipients {
		if recipient.ID == invited.ID {
			sawRevoked = true
			if recipient.RevokedAt == "" {
				t.Fatal("revoked recipient did not retain revokedAt in listing")
			}
			continue
		}
		if recipient.RevokedAt != "" {
			t.Fatalf("active recipient was marked revoked: %+v", recipient)
		}
	}
	if !sawRevoked {
		t.Fatal("revoked recipient missing from listing")
	}
	if _, err := decryptSecretProfile(mustSecretPath(t, cfg, p.ID, "dev"), teammate); err == nil {
		t.Fatal("revoked recipient can still decrypt the rewrapped profile")
	}
	if err := EnvRotateRecipients("api", "dev"); err != nil {
		t.Fatal(err)
	}
	identityPath, err := resolveAgeIdentityPath(cfg)
	if err != nil {
		t.Fatal(err)
	}
	local, err := loadIdentity(identityPath)
	if err != nil {
		t.Fatal(err)
	}
	localProfile := decryptSecretProfileForTest(t, mustSecretPath(t, cfg, p.ID, "dev"), local)
	if localProfile.Values["TOKEN"] == "" {
		t.Fatal("local recipient lost access after revocation")
	}
}

func TestEnvRecipientExportReturnsLocalIdentity(t *testing.T) {
	home := t.TempDir()
	workspace := filepath.Join(t.TempDir(), "code")
	t.Setenv(envHome, home)
	if _, err := InitWorkspace(workspace); err != nil {
		t.Fatal(err)
	}
	recipient, err := EnvRecipientExport()
	if err != nil {
		t.Fatal(err)
	}
	if recipient.ID == "" || recipient.Name == "" {
		t.Fatalf("local recipient is missing identity metadata: %+v", recipient)
	}
	if !strings.HasPrefix(recipient.AgeRecipient, "age1") {
		t.Fatalf("age recipient = %q, want age1 prefix", recipient.AgeRecipient)
	}
}

func TestRootCommandHelp(t *testing.T) {
	cmd := NewRootCommand("test")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--help"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "sync") || !strings.Contains(out.String(), "env") || !strings.Contains(out.String(), "setup") {
		t.Fatalf("help did not include expected commands:\n%s", out.String())
	}
}

func TestVersionFlagMatchesConfiguredVersion(t *testing.T) {
	const want = "v1.2.3-test"
	cmd := NewRootCommand(want)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--version"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(out.String()); !strings.Contains(got, want) {
		t.Fatalf("version flag printed %q, want configured version %q", got, want)
	}
}

func TestFangStyledHelpWhenColorForced(t *testing.T) {
	clearColorEnv(t)
	resetStylesAfterTest(t)
	t.Setenv("CLICOLOR_FORCE", "1")
	root := NewRootCommand("test")
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetArgs([]string{"--help"})
	if err := fang.Execute(context.Background(), root, fang.WithVersion("test")); err != nil {
		t.Fatalf("fang.Execute: %v", err)
	}
	if !strings.ContainsRune(buf.String(), 0x1b) {
		t.Fatalf("expected fang's styled help output to contain ANSI escape bytes when CLICOLOR_FORCE=1 is set, got %q", buf.String())
	}
}

func TestFangHelpPlainWhenPiped(t *testing.T) {
	clearColorEnv(t)
	resetStylesAfterTest(t)
	root := NewRootCommand("test")
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetArgs([]string{"--help"})
	if err := fang.Execute(context.Background(), root, fang.WithVersion("test")); err != nil {
		t.Fatalf("fang.Execute: %v", err)
	}
	if strings.ContainsRune(buf.String(), 0x1b) {
		t.Fatalf("expected fang's help output to be plain for a non-terminal writer with no forcing env vars, got %q", buf.String())
	}
}

func makeBareRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	src := filepath.Join(root, "src")
	bare := filepath.Join(root, "remote.git")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	run(t, src, "git", "init", "-b", "main")
	run(t, src, "git", "config", "user.email", "test@example.com")
	run(t, src, "git", "config", "user.name", "Test User")
	write(t, filepath.Join(src, "README.md"), "hello\n")
	run(t, src, "git", "add", "README.md")
	run(t, src, "git", "commit", "-m", "initial")
	run(t, root, "git", "clone", "--bare", src, bare)
	return bare
}

func run(t *testing.T, dir, name string, args ...string) {
	t.Helper()
	if name == "git" {
		args = append([]string{"-c", "commit.gpgsign=false"}, args...)
	}
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %s failed: %v\n%s", name, strings.Join(args, " "), err, out)
	}
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func mustSecretPath(t *testing.T, cfg Config, projectID, profile string) string {
	t.Helper()
	path, err := secretPath(cfg, projectID, profile)
	if err != nil {
		t.Fatal(err)
	}
	return path
}

func decryptSecretProfileForTest(t *testing.T, path string, identity *age.X25519Identity) SecretProfile {
	t.Helper()
	profile, err := decryptSecretProfile(path, identity)
	if err != nil {
		t.Fatal(err)
	}
	return profile
}

func decryptSecretProfile(path string, identity *age.X25519Identity) (SecretProfile, error) {
	ciphertext, err := os.ReadFile(path)
	if err != nil {
		return SecretProfile{}, err
	}
	reader, err := age.Decrypt(bytes.NewReader(ciphertext), identity)
	if err != nil {
		return SecretProfile{}, err
	}
	plaintext, err := io.ReadAll(reader)
	if err != nil {
		return SecretProfile{}, err
	}
	var profile SecretProfile
	if err := json.Unmarshal(plaintext, &profile); err != nil {
		return SecretProfile{}, err
	}
	return profile, nil
}
