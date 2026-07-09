package devspace

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestHardeningInitPreservesExistingManifestAndConfig(t *testing.T) {
	home := t.TempDir()
	workspace := filepath.Join(t.TempDir(), "code")
	t.Setenv(envHome, home)

	identity := filepath.Join(home, "custom", "identity.txt")
	hardeningWriteFile(t, identity, "# existing identity\nAGE-SECRET-KEY-LOCAL\n", 0o600)
	initialConfig := Config{
		MachineID:       "machine_existing",
		MachineName:     "known-host",
		WorkspaceRoot:   filepath.Join(t.TempDir(), "old-code"),
		AgeIdentityPath: identity,
		CreatedAt:       "2025-01-02T03:04:05Z",
		UpdatedAt:       "2025-01-02T03:04:05Z",
	}
	if err := SaveConfig(initialConfig); err != nil {
		t.Fatal(err)
	}
	initialIdentity, err := os.ReadFile(identity)
	if err != nil {
		t.Fatal(err)
	}

	existing := Manifest{
		Version:       ManifestVersion,
		WorkspaceRoot: workspace,
		Machines: []Machine{{
			ID:            "machine_existing",
			Name:          "known-host",
			WorkspaceRoot: workspace,
		}},
		Projects: []Project{hardeningProject("apps/existing", ProjectTypeLocal, "")},
	}
	existing.Projects[0].EnvProfiles = []string{"dev", "test"}
	existing.Projects[0].Ignore = []string{"tmp"}
	if err := SaveManifest(workspace, existing); err != nil {
		t.Fatal(err)
	}

	cfg, err := InitWorkspace(workspace)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MachineID != initialConfig.MachineID {
		t.Fatalf("machine id changed: %q", cfg.MachineID)
	}
	if cfg.MachineName != initialConfig.MachineName {
		t.Fatalf("machine name changed: %q", cfg.MachineName)
	}
	if cfg.AgeIdentityPath != identity {
		t.Fatalf("identity path changed: %q", cfg.AgeIdentityPath)
	}
	if cfg.CreatedAt != initialConfig.CreatedAt {
		t.Fatalf("createdAt changed: %q", cfg.CreatedAt)
	}
	identityAgain, err := os.ReadFile(identity)
	if err != nil {
		t.Fatal(err)
	}
	if string(identityAgain) != string(initialIdentity) {
		t.Fatal("existing age identity was overwritten")
	}

	loaded, err := LoadManifest(workspace)
	if err != nil {
		t.Fatal(err)
	}
	p, ok := findProject(loaded, "apps/existing")
	if !ok {
		t.Fatalf("existing project missing after init: %+v", loaded.Projects)
	}
	if !reflect.DeepEqual(p.EnvProfiles, []string{"dev", "test"}) {
		t.Fatalf("env profiles changed: %#v", p.EnvProfiles)
	}
	if !reflect.DeepEqual(p.Ignore, []string{"tmp"}) {
		t.Fatalf("ignore rules changed: %#v", p.Ignore)
	}
}

func TestHardeningScanIgnoresDependencyFoldersAndNestedRepos(t *testing.T) {
	workspace := hardeningInitWorkspace(t, "code")
	hardeningGitRepo(t, filepath.Join(workspace, "apps", "app"))
	hardeningGitRepo(t, filepath.Join(workspace, "apps", "app", "nested-repo"))
	hardeningGitRepo(t, filepath.Join(workspace, "services", "api"))
	hardeningWriteFile(t, filepath.Join(workspace, "node_modules", "left-pad", "package.json"), `{"name":"left-pad"}`, 0o644)
	hardeningGitRepo(t, filepath.Join(workspace, "vendor", "vendored-repo"))
	hardeningWriteFile(t, filepath.Join(workspace, "dist", "bundle", "package.json"), `{"name":"bundle"}`, 0o644)

	summary, err := ScanWorkspace()
	if err != nil {
		t.Fatal(err)
	}
	if summary.FoundProjects != 2 || summary.GitRepos != 2 {
		t.Fatalf("scan included dependency or nested projects: %+v", summary)
	}
	m, err := LoadManifest(workspace)
	if err != nil {
		t.Fatal(err)
	}
	hardeningAssertProjectPaths(t, m, []string{"apps/app", "services/api"})
}

func TestHardeningScanUsesWorkspaceIgnoreFile(t *testing.T) {
	workspace := hardeningInitWorkspace(t, "code")
	hardeningWriteFile(t, filepath.Join(workspace, ".devspaceignore"), "adobe/\n# comment\n\n", 0o644)
	hardeningGitRepo(t, filepath.Join(workspace, "adobe", "protopack"))
	hardeningGitRepo(t, filepath.Join(workspace, "apps", "api"))

	summary, err := ScanWorkspace()
	if err != nil {
		t.Fatal(err)
	}
	if summary.FoundProjects != 1 || summary.GitRepos != 1 {
		t.Fatalf("ignored workspace path was scanned: %+v", summary)
	}
	m, err := LoadManifest(workspace)
	if err != nil {
		t.Fatal(err)
	}
	hardeningAssertProjectPaths(t, m, []string{"apps/api"})
}

func TestHardeningScanDisambiguatesDuplicateBasenames(t *testing.T) {
	workspace := hardeningInitWorkspace(t, "code")
	hardeningGitRepo(t, filepath.Join(workspace, "client-a", "family-events-backend"))
	hardeningGitRepo(t, filepath.Join(workspace, "client-b", "family-events-backend"))

	if _, err := ScanWorkspace(); err != nil {
		t.Fatal(err)
	}
	m, err := LoadManifest(workspace)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Projects) != 2 {
		t.Fatalf("projects = %+v", m.Projects)
	}
	names := map[string]bool{}
	for _, p := range m.Projects {
		if names[p.Name] {
			t.Fatalf("duplicate project name remained: %+v", m.Projects)
		}
		names[p.Name] = true
	}
	if !names["family-events-backend"] || !names["client-b-family-events-backend"] {
		t.Fatalf("duplicate basename was not disambiguated predictably: %+v", m.Projects)
	}

	if _, err := ScanWorkspace(); err != nil {
		t.Fatalf("rescan failed: %v", err)
	}
	m, err = LoadManifest(workspace)
	if err != nil {
		t.Fatal(err)
	}
	rescanned := map[string]bool{}
	for _, p := range m.Projects {
		rescanned[p.Name] = true
	}
	if !rescanned["family-events-backend"] || !rescanned["client-b-family-events-backend"] {
		t.Fatalf("rescan changed project names: %+v", m.Projects)
	}
}

func TestHardeningRejectsPathTraversal(t *testing.T) {
	workspace := hardeningInitWorkspace(t, "code")
	for _, path := range []string{"../outside", "ok/../../outside", "..", filepath.Join(string(filepath.Separator), "tmp", "abs")} {
		t.Run(path, func(t *testing.T) {
			m := Manifest{
				Version:       ManifestVersion,
				WorkspaceRoot: workspace,
				Projects:      []Project{hardeningProject(path, ProjectTypeLocal, "")},
			}
			if err := ValidateManifest(m); err == nil {
				t.Fatalf("ValidateManifest accepted unsafe path %q", path)
			}
			if _, err := AddProject(path); err == nil {
				t.Fatalf("AddProject accepted unsafe path %q", path)
			}
		})
	}
}

func TestHardeningInvalidManifestJSONReturnsUsefulError(t *testing.T) {
	workspace := hardeningInitWorkspace(t, "code")
	hardeningWriteFile(t, manifestPath(workspace), "{not-json", 0o600)

	_, err := LoadManifest(workspace)
	if err == nil {
		t.Fatal("expected invalid JSON error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "invalid JSON") || !strings.Contains(msg, "manifest.json") {
		t.Fatalf("manifest JSON error lacks useful context: %v", err)
	}
}

func TestHardeningPlanApplyConsistencyAndIdempotentRerun(t *testing.T) {
	workspace := hardeningInitWorkspace(t, "code")
	m := Manifest{
		Version:       ManifestVersion,
		WorkspaceRoot: workspace,
		Projects: []Project{
			hardeningProject("apps/app", ProjectTypeGit, "https://example.invalid/app.git"),
			hardeningProject("tools/tool", ProjectTypeLocal, ""),
		},
	}
	if err := SaveManifest(workspace, m); err != nil {
		t.Fatal(err)
	}

	planned, err := BuildPlan()
	if err != nil {
		t.Fatal(err)
	}
	if err := SaveLastPlan(planned); err != nil {
		t.Fatal(err)
	}
	applied, err := ApplyLastPlan()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(applied.Actions, planned.Actions) {
		t.Fatalf("apply actions differed from plan:\nplan=%+v\napply=%+v", planned, applied)
	}
	for _, action := range planned.Actions {
		if action.Safety != "safe" {
			continue
		}
		if !exists(filepath.Join(workspace, filepath.FromSlash(action.Path))) {
			t.Fatalf("missing path for planned action: %+v", action)
		}
	}

	secondPlan, err := BuildPlan()
	if err != nil {
		t.Fatal(err)
	}
	if got := hardeningSafeActionCount(secondPlan); got != 0 {
		t.Fatalf("plan not idempotent after apply: %+v", secondPlan.Actions)
	}
	if err := SaveLastPlan(secondPlan); err != nil {
		t.Fatal(err)
	}
	secondApply, err := ApplyLastPlan()
	if err != nil {
		t.Fatal(err)
	}
	if got := hardeningSafeActionCount(secondApply); got != 0 {
		t.Fatalf("apply not idempotent after rerun: %+v", secondApply.Actions)
	}
}

func TestHardeningApplyRejectsManifestDriftAfterPlan(t *testing.T) {
	workspace := hardeningInitWorkspace(t, "code")
	m := Manifest{
		Version:       ManifestVersion,
		WorkspaceRoot: workspace,
		Projects:      []Project{hardeningProject("apps/app", ProjectTypeGit, "https://example.invalid/app.git")},
	}
	if err := SaveManifest(workspace, m); err != nil {
		t.Fatal(err)
	}
	planned, err := BuildPlan()
	if err != nil {
		t.Fatal(err)
	}
	if err := SaveLastPlan(planned); err != nil {
		t.Fatal(err)
	}
	m.Projects = append(m.Projects, hardeningProject("apps/other", ProjectTypeLocal, ""))
	if err := SaveManifest(workspace, m); err != nil {
		t.Fatal(err)
	}
	if _, err := ApplyLastPlan(); err == nil || !strings.Contains(err.Error(), "manifest changed") {
		t.Fatalf("apply did not reject manifest drift: %v", err)
	}
}

func TestHardeningPlanJSONIsStructuredAndMachineReadable(t *testing.T) {
	workspace := hardeningInitWorkspace(t, "code")
	m := Manifest{
		Version:       ManifestVersion,
		WorkspaceRoot: workspace,
		Projects:      []Project{hardeningProject("apps/app", ProjectTypeGit, "https://example.invalid/app.git")},
	}
	if err := SaveManifest(workspace, m); err != nil {
		t.Fatal(err)
	}
	cmd := NewRootCommand("test")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"plan", "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var planned Plan
	if err := json.Unmarshal(out.Bytes(), &planned); err != nil {
		t.Fatalf("plan --json did not produce JSON: %v\n%s", err, out.String())
	}
	if planned.ManifestHash == "" || len(planned.Actions) != 1 || planned.Actions[0].Safety != "safe" {
		t.Fatalf("unexpected JSON plan: %+v", planned)
	}
}

func TestHardeningApplyRefusesNonEmptyDestination(t *testing.T) {
	workspace := hardeningInitWorkspace(t, "code")
	hardeningWriteFile(t, filepath.Join(workspace, "apps", "app", "README.md"), "local work\n", 0o644)
	m := Manifest{
		Version:       ManifestVersion,
		WorkspaceRoot: workspace,
		Projects:      []Project{hardeningProject("apps/app", ProjectTypeGit, "https://example.invalid/app.git")},
	}
	if err := SaveManifest(workspace, m); err != nil {
		t.Fatal(err)
	}

	planned, err := BuildPlan()
	if err != nil {
		t.Fatal(err)
	}
	if err := SaveLastPlan(planned); err != nil {
		t.Fatal(err)
	}
	applied, err := ApplyLastPlan()
	if err != nil {
		t.Fatal(err)
	}
	if hardeningSafeActionCount(applied) != 0 {
		t.Fatalf("apply considered non-empty destination safe: %+v", applied.Actions)
	}
	if !exists(filepath.Join(workspace, "apps", "app", "README.md")) {
		t.Fatal("apply removed existing local content")
	}
}

func TestHardeningHydrateRefusesMissingRemoteAndNonEmptyFolder(t *testing.T) {
	workspace := hardeningInitWorkspace(t, "code")
	missingRemote := hardeningProject("apps/no-remote", ProjectTypeGit, "")
	nonEmpty := hardeningProject("apps/non-empty", ProjectTypeGit, "https://example.invalid/non-empty.git")
	m := Manifest{
		Version:       ManifestVersion,
		WorkspaceRoot: workspace,
		Projects:      []Project{missingRemote, nonEmpty},
	}
	if err := SaveManifest(workspace, m); err != nil {
		t.Fatal(err)
	}
	hardeningWriteFile(t, filepath.Join(workspace, "apps", "non-empty", "README.md"), "local work\n", 0o644)

	if _, err := HydrateProject("no-remote"); err == nil || !strings.Contains(err.Error(), "no Git remote") {
		t.Fatalf("hydrate missing remote error = %v", err)
	}
	if _, err := HydrateProject("non-empty"); err == nil || !strings.Contains(err.Error(), "non-empty") {
		t.Fatalf("hydrate non-empty folder error = %v", err)
	}
	if !exists(filepath.Join(workspace, "apps", "non-empty", "README.md")) {
		t.Fatal("hydrate removed existing local content")
	}
}

func TestHardeningWorkspacePathsWithSpaces(t *testing.T) {
	workspace := hardeningInitWorkspace(t, "code with spaces")
	projectPath := filepath.Join(workspace, "client apps", "web app")
	hardeningGitRepo(t, projectPath)

	if _, err := ScanWorkspace(); err != nil {
		t.Fatal(err)
	}
	m, err := LoadManifest(workspace)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := findProject(m, "client apps/web app"); !ok {
		t.Fatalf("project with spaces was not tracked: %+v", m.Projects)
	}

	planned, err := BuildPlan()
	if err != nil {
		t.Fatal(err)
	}
	if got := hardeningSafeActionCount(planned); got != 0 {
		t.Fatalf("existing spaced path should not need safe actions: %+v", planned.Actions)
	}
}

func TestHardeningManifestWritesCreateBackup(t *testing.T) {
	workspace := hardeningInitWorkspace(t, "code")
	first := Manifest{
		Version:       ManifestVersion,
		WorkspaceRoot: workspace,
		Projects:      []Project{hardeningProject("apps/one", ProjectTypeLocal, "")},
	}
	if err := SaveManifest(workspace, first); err != nil {
		t.Fatal(err)
	}
	second := first
	second.Projects = append(second.Projects, hardeningProject("apps/two", ProjectTypeLocal, ""))
	if err := SaveManifest(workspace, second); err != nil {
		t.Fatal(err)
	}
	backup, err := os.ReadFile(manifestPath(workspace) + ".bak")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(backup), "apps/one") || strings.Contains(string(backup), "apps/two") {
		t.Fatalf("manifest backup does not contain previous manifest:\n%s", backup)
	}
}

func TestHardeningMissingGitBinaryHasUsefulHydrateError(t *testing.T) {
	workspace := hardeningInitWorkspace(t, "code")
	m := Manifest{
		Version:       ManifestVersion,
		WorkspaceRoot: workspace,
		Projects:      []Project{hardeningProject("apps/app", ProjectTypeGit, "https://example.invalid/app.git")},
	}
	if err := SaveManifest(workspace, m); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", t.TempDir())
	_, err := HydrateProject("app")
	if err == nil || !strings.Contains(err.Error(), "git executable not found") {
		t.Fatalf("missing git error = %v", err)
	}
}

func TestHardeningTwoMachineSimulationWithLocalBareRemote(t *testing.T) {
	root := t.TempDir()
	homeA := filepath.Join(root, "home-a")
	homeB := filepath.Join(root, "home-b")
	workspaceA := filepath.Join(root, "machine a", "code")
	workspaceB := filepath.Join(root, "machine b", "code")
	remote := hardeningBareRepo(t)

	t.Setenv(envHome, homeA)
	if _, err := InitWorkspace(workspaceA); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(workspaceA, "work"), 0o755); err != nil {
		t.Fatal(err)
	}
	hardeningRun(t, filepath.Join(workspaceA, "work"), "git", "clone", remote, "api")
	hardeningWriteFile(t, filepath.Join(workspaceA, "work", "api", "README.md"), "dirty\n", 0o644)
	if _, err := ScanWorkspace(); err != nil {
		t.Fatal(err)
	}
	manifestBytes, err := os.ReadFile(manifestPath(workspaceA))
	if err != nil {
		t.Fatal(err)
	}

	t.Setenv(envHome, homeB)
	if _, err := InitWorkspace(workspaceB); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifestPath(workspaceB), manifestBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	planned, err := BuildPlan()
	if err != nil {
		t.Fatal(err)
	}
	if hardeningSafeActionCount(planned) != 1 {
		t.Fatalf("machine B expected one safe placeholder action: %+v", planned.Actions)
	}
	if err := SaveLastPlan(planned); err != nil {
		t.Fatal(err)
	}
	if _, err := ApplyLastPlan(); err != nil {
		t.Fatal(err)
	}
	if !exists(filepath.Join(workspaceB, "work", "api")) {
		t.Fatal("machine B did not get matching folder structure")
	}
	if _, err := HydrateProject("api"); err != nil {
		t.Fatal(err)
	}
	if !exists(filepath.Join(workspaceB, "work", "api", ".git")) {
		t.Fatal("machine B did not hydrate local bare remote")
	}
	hardeningWriteFile(t, filepath.Join(workspaceB, "work", "api", "local-change.txt"), "dirty\n", 0o644)

	dirtyPlan, err := BuildPlan()
	if err != nil {
		t.Fatal(err)
	}
	if len(dirtyPlan.Actions) == 0 {
		t.Fatal("expected dirty repo skip after hydrate")
	}
	foundDirtySkip := false
	for _, action := range dirtyPlan.Actions {
		if action.Safety == "skipped" && strings.Contains(action.Reason, "dirty") {
			foundDirtySkip = true
		}
	}
	if !foundDirtySkip {
		t.Fatalf("dirty repo was not detected/skipped: %+v", dirtyPlan.Actions)
	}

	conflictWorkspace := filepath.Join(root, "machine c", "code")
	t.Setenv(envHome, filepath.Join(root, "home-c"))
	if _, err := InitWorkspace(conflictWorkspace); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifestPath(conflictWorkspace), manifestBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	hardeningWriteFile(t, filepath.Join(conflictWorkspace, "work", "api", "local.txt"), "mine\n", 0o644)
	conflictPlan, err := BuildPlan()
	if err != nil {
		t.Fatal(err)
	}
	if hardeningSafeActionCount(conflictPlan) != 0 {
		t.Fatalf("non-empty conflict destination was considered safe: %+v", conflictPlan.Actions)
	}
}

func hardeningInitWorkspace(t *testing.T, leaf string) string {
	t.Helper()
	home := t.TempDir()
	workspace := filepath.Join(t.TempDir(), leaf)
	t.Setenv(envHome, home)
	if _, err := InitWorkspace(workspace); err != nil {
		t.Fatal(err)
	}
	return workspace
}

func hardeningProject(rel, typ, remote string) Project {
	mode := HydrateManual
	if typ == ProjectTypeGit {
		mode = HydrateOnDemand
	}
	return Project{
		ID:            projectID(rel),
		Name:          projectName(rel),
		Path:          filepath.ToSlash(rel),
		Type:          typ,
		Remote:        remote,
		DefaultBranch: "main",
		HydrateMode:   mode,
		Ignore:        append([]string{}, DefaultIgnores...),
	}
}

func hardeningGitRepo(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	hardeningRun(t, dir, "git", "init", "-b", "main")
	hardeningRun(t, dir, "git", "config", "user.email", "test@example.com")
	hardeningRun(t, dir, "git", "config", "user.name", "Test User")
	hardeningWriteFile(t, filepath.Join(dir, "README.md"), "hello\n", 0o644)
	hardeningRun(t, dir, "git", "add", "README.md")
	hardeningRun(t, dir, "git", "commit", "-m", "initial")
}

func hardeningBareRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	src := filepath.Join(root, "src")
	bare := filepath.Join(root, "remote.git")
	hardeningGitRepo(t, src)
	hardeningRun(t, root, "git", "clone", "--bare", src, bare)
	return bare
}

func hardeningRun(t *testing.T, dir, name string, args ...string) {
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

func hardeningWriteFile(t *testing.T, path, content string, perm os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), perm); err != nil {
		t.Fatal(err)
	}
}

func hardeningAssertProjectPaths(t *testing.T, m Manifest, want []string) {
	t.Helper()
	got := make([]string, 0, len(m.Projects))
	for _, p := range m.Projects {
		got = append(got, p.Path)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("project paths = %#v, want %#v", got, want)
	}
}

func hardeningSafeActionCount(plan Plan) int {
	var count int
	for _, action := range plan.Actions {
		if action.Safety == "safe" {
			count++
		}
	}
	return count
}
