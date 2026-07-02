package devspace

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
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
	cases := []Manifest{
		{Version: ManifestVersion, WorkspaceRoot: "/tmp/code", Projects: []Project{
			{ID: "a", Name: "one", Path: "/abs", Type: ProjectTypeLocal, HydrateMode: HydrateManual},
		}},
		{Version: ManifestVersion, WorkspaceRoot: "/tmp/code", Projects: []Project{
			{ID: "a", Name: "one", Path: "one", Type: ProjectTypeLocal, HydrateMode: HydrateManual},
			{ID: "b", Name: "two", Path: "one", Type: ProjectTypeLocal, HydrateMode: HydrateManual},
		}},
		{Version: ManifestVersion, WorkspaceRoot: "/tmp/code", Projects: []Project{
			{ID: "a", Name: "one", Path: "one", Type: "weird", HydrateMode: HydrateManual},
		}},
		{Version: ManifestVersion, WorkspaceRoot: "/tmp/code", Projects: []Project{
			{ID: "a", Name: "one", Path: "one", Type: ProjectTypeLocal, HydrateMode: "sometimes"},
		}},
	}
	if err := ValidateManifest(base); err != nil {
		t.Fatalf("base manifest should validate: %v", err)
	}
	for _, tc := range cases {
		if err := ValidateManifest(tc); err == nil {
			t.Fatalf("expected validation failure for %#v", tc)
		}
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
	ciphertext, err := os.ReadFile(secretPath(cfg, p.ID, "dev"))
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
	profile := decryptSecretProfileForTest(t, secretPath(cfg, p.ID, "dev"), teammate)
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
	if _, err := decryptSecretProfile(secretPath(cfg, p.ID, "dev"), teammate); err == nil {
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
	localProfile := decryptSecretProfileForTest(t, secretPath(cfg, p.ID, "dev"), local)
	if localProfile.Values["TOKEN"] == "" {
		t.Fatal("local recipient lost access after revocation")
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
