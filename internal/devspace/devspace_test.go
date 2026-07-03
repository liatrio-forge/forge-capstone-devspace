package devspace

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

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
	actions, err := ApplySync()
	if err != nil {
		t.Fatal(err)
	}
	if len(actions) != 1 || actions[0].Kind != "placeholder" {
		t.Fatalf("unexpected sync actions: %+v", actions)
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
	if !strings.Contains(out.String(), "workspace") || !strings.Contains(out.String(), "env") || !strings.Contains(out.String(), "setup") {
		t.Fatalf("help did not include expected commands:\n%s", out.String())
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
