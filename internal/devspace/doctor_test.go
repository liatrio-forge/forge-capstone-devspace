package devspace

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDoctorReportsProjectStatesWithoutMutatingWorkspaceFiles(t *testing.T) {
	workspace := hardeningInitWorkspace(t, "code")
	localPath := filepath.Join(workspace, "apps", "local")
	hardeningWriteFile(t, filepath.Join(localPath, ".env"), "DEV_DROP_ENV_PRESENT=1\n", 0o600)
	placeholderPath := filepath.Join(workspace, "apps", "placeholder")
	if err := os.MkdirAll(placeholderPath, 0o755); err != nil {
		t.Fatal(err)
	}
	dirtyPath := filepath.Join(workspace, "apps", "dirty")
	hardeningGitRepo(t, dirtyPath)
	hardeningWriteFile(t, filepath.Join(dirtyPath, "local.txt"), "dirty\n", 0o644)
	m := Manifest{
		Version:       ManifestVersion,
		WorkspaceRoot: workspace,
		Projects: []Project{
			hardeningProject("apps/local", ProjectTypeLocal, ""),
			hardeningProject("apps/placeholder", ProjectTypeGit, "https://example.invalid/placeholder.git"),
			hardeningProject("apps/missing", ProjectTypeGit, "https://example.invalid/missing.git"),
			hardeningProject("apps/dirty", ProjectTypeGit, "https://example.invalid/dirty.git"),
		},
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
	beforeManifest, err := os.ReadFile(manifestPath(workspace))
	if err != nil {
		t.Fatal(err)
	}
	beforePlan, err := os.ReadFile(lastPlanPath(workspace))
	if err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	if err := RunDoctor(&out); err != nil {
		t.Fatalf("doctor returned hard failure: %v\n%s", err, out.String())
	}
	got := out.String()
	for _, want := range []string{
		"[OK] Config:",
		"[OK] Manifest:",
		"[WARN] Manifest remote: not configured",
		"[OK] Last plan: saved plan hash matches current manifest",
		"[WARN] Projects: 4 tracked; hydrated: 2, placeholders: 1, missing: 1, dirty: 1, missing .env: 3",
		"Project placeholder: placeholder, missing .env",
		"Project missing: missing, missing .env",
		"Project dirty: hydrated, dirty, missing .env",
		"Result: ready",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("doctor output missing %q:\n%s", want, got)
		}
	}
	afterManifest, err := os.ReadFile(manifestPath(workspace))
	if err != nil {
		t.Fatal(err)
	}
	afterPlan, err := os.ReadFile(lastPlanPath(workspace))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(beforeManifest, afterManifest) || !bytes.Equal(beforePlan, afterPlan) {
		t.Fatal("doctor mutated manifest or last plan")
	}
}

func TestDoctorFailsWhenConfigIsMissing(t *testing.T) {
	t.Setenv(envHome, t.TempDir())

	var out bytes.Buffer
	err := RunDoctor(&out)
	if err == nil {
		t.Fatalf("doctor succeeded without config:\n%s", out.String())
	}
	got := out.String()
	if !strings.Contains(got, "[FAIL] Config: missing") || !strings.Contains(got, "devspace init") {
		t.Fatalf("doctor missing config output lacks recovery guidance:\n%s", got)
	}
	if !strings.Contains(got, "Result: 1 hard failure") {
		t.Fatalf("doctor did not summarize hard failure:\n%s", got)
	}
}

func TestDoctorFailsWhenManifestIsInvalid(t *testing.T) {
	workspace := hardeningInitWorkspace(t, "code")
	hardeningWriteFile(t, manifestPath(workspace), "{not-json", 0o600)

	var out bytes.Buffer
	err := RunDoctor(&out)
	if err == nil {
		t.Fatalf("doctor succeeded with invalid manifest:\n%s", out.String())
	}
	got := out.String()
	if !strings.Contains(got, "[FAIL] Manifest: invalid") || !strings.Contains(got, "invalid JSON") {
		t.Fatalf("doctor did not report invalid manifest:\n%s", got)
	}
}

func TestDoctorReportsStalePlanAndDirtyManifestRepoAsWarnings(t *testing.T) {
	workspace := hardeningInitWorkspace(t, "code")
	remote := workspaceSyncBareRepo(t)
	if _, err := SetManifestRemote(remote); err != nil {
		t.Fatal(err)
	}
	m := Manifest{
		Version:       ManifestVersion,
		WorkspaceRoot: workspace,
		Projects:      []Project{hardeningProject("apps/one", ProjectTypeLocal, "")},
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
	if _, err := PushWorkspaceManifest(); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	hardeningWriteFile(t, filepath.Join(cfg.ManifestRepoPath, "scratch.txt"), "dirty\n", 0o644)
	m.Projects = append(m.Projects, hardeningProject("apps/two", ProjectTypeLocal, ""))
	if err := SaveManifest(workspace, m); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	if err := RunDoctor(&out); err != nil {
		t.Fatalf("doctor returned hard failure for warnings: %v\n%s", err, out.String())
	}
	got := out.String()
	if !strings.Contains(got, "[OK] Manifest remote:") {
		t.Fatalf("doctor did not report configured remote:\n%s", got)
	}
	if !strings.Contains(got, "[WARN] Manifest repo cleanliness: manifest repo has uncommitted changes") {
		t.Fatalf("doctor did not report dirty manifest repo:\n%s", got)
	}
	if !strings.Contains(got, "[WARN] Last plan: saved plan hash does not match current manifest") {
		t.Fatalf("doctor did not report stale plan:\n%s", got)
	}
	if !strings.Contains(got, "Result: ready") {
		t.Fatalf("doctor warnings should not fail core readiness:\n%s", got)
	}
}
