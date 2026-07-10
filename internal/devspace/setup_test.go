package devspace

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSetupPlanReportsDetectedInstallAndDevCommands(t *testing.T) {
	workspace := hardeningInitWorkspace(t, "code")
	hardeningWriteFile(t, filepath.Join(workspace, "apps", "web", "package.json"), `{"scripts":{"dev":"vite"}}`, 0o644)
	hardeningWriteFile(t, filepath.Join(workspace, "services", "api", "go.mod"), "module example.com/api\n", 0o644)

	if _, err := ScanWorkspace(); err != nil {
		t.Fatal(err)
	}
	plan, err := BuildSetupPlan()
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Projects) != 2 {
		t.Fatalf("setup projects = %+v", plan.Projects)
	}
	web := setupTestProject(t, plan, "web")
	if web.PackageManager != "npm" || web.InstallCommand != "npm install" || web.DevCommand != "npm run dev" || !web.Runnable {
		t.Fatalf("unexpected web setup: %+v", web)
	}
	api := setupTestProject(t, plan, "api")
	if api.PackageManager != "go" || api.InstallCommand != "go mod download" || api.DevCommand != "" || !api.Runnable {
		t.Fatalf("unexpected api setup: %+v", api)
	}
}

func TestSetupRunExecutesKnownCommandInsideProjectPath(t *testing.T) {
	workspace := hardeningInitWorkspace(t, "code")
	projectDir := filepath.Join(workspace, "apps", "web")
	hardeningWriteFile(t, filepath.Join(projectDir, "package.json"), `{"scripts":{"dev":"vite"}}`, 0o644)
	if _, err := ScanWorkspace(); err != nil {
		t.Fatal(err)
	}
	logPath := setupFakeExecutable(t, "npm")

	result, err := RunProjectSetup("web", SetupRunOptions{CommandKind: "install"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Command != "npm install" || result.Path != "apps/web" {
		t.Fatalf("unexpected result: %+v", result)
	}
	logged, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	want := projectDir + "|install\n"
	if string(logged) != want {
		t.Fatalf("setup command ran in wrong location or with wrong args:\n%s\nwant %s", logged, want)
	}
}

func TestSetupRunRejectsUnknownCommandWithoutExplicitOverride(t *testing.T) {
	workspace := hardeningInitWorkspace(t, "code")
	hardeningWriteFile(t, filepath.Join(workspace, "apps", "custom", "setup.sh"), "#!/bin/sh\n", 0o755)
	m := Manifest{
		Version:       ManifestVersion,
		WorkspaceRoot: workspace,
		Projects: []Project{{
			ID:          projectID("apps/custom"),
			Name:        "custom",
			Path:        "apps/custom",
			Type:        ProjectTypeLocal,
			HydrateMode: HydrateManual,
			Setup: Setup{
				PackageManager: "custom",
				InstallCommand: "./setup.sh --with-flags",
			},
		}},
	}
	if err := SaveManifest(workspace, m); err != nil {
		t.Fatal(err)
	}

	_, err := RunProjectSetup("custom", SetupRunOptions{CommandKind: "install"})
	if err == nil || !strings.Contains(err.Error(), "refusing unknown setup command") {
		t.Fatalf("unknown setup error = %v", err)
	}
}

func TestSetupRunRejectsGlobalCommandWithoutExplicitOverride(t *testing.T) {
	workspace := hardeningInitWorkspace(t, "code")
	hardeningWriteFile(t, filepath.Join(workspace, "apps", "custom", "setup.sh"), "#!/bin/sh\n", 0o755)
	m := Manifest{
		Version:       ManifestVersion,
		WorkspaceRoot: workspace,
		Projects: []Project{{
			ID:          projectID("apps/custom"),
			Name:        "custom",
			Path:        "apps/custom",
			Type:        ProjectTypeLocal,
			HydrateMode: HydrateManual,
			Setup: Setup{
				PackageManager: "custom",
				InstallCommand: "npm install -g local-tool",
			},
		}},
	}
	if err := SaveManifest(workspace, m); err != nil {
		t.Fatal(err)
	}

	_, err := RunProjectSetup("custom", SetupRunOptions{CommandKind: "install", AllowUnknown: true})
	if err == nil || !strings.Contains(err.Error(), "refusing global setup command") {
		t.Fatalf("global setup error = %v", err)
	}
}

func TestSetupRunCommandRequiresConfirmation(t *testing.T) {
	workspace := hardeningInitWorkspace(t, "code")
	hardeningWriteFile(t, filepath.Join(workspace, "apps", "web", "package.json"), `{"scripts":{"dev":"vite"}}`, 0o644)
	if _, err := ScanWorkspace(); err != nil {
		t.Fatal(err)
	}
	logPath := setupFakeExecutable(t, "npm")

	cmd := NewRootCommand("test")
	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetIn(strings.NewReader("wrong\n"))
	cmd.SetArgs([]string{"setup", "run", "web"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "confirmation did not match") {
		t.Fatalf("confirmation error = %v", err)
	}
	if data, err := os.ReadFile(logPath); err == nil && len(data) > 0 {
		t.Fatalf("setup command ran without confirmation:\n%s", data)
	}
}

func TestSetupRunAllCommandRequiresConfirmation(t *testing.T) {
	workspace := hardeningInitWorkspace(t, "code")
	hardeningWriteFile(t, filepath.Join(workspace, "apps", "web", "package.json"), `{"scripts":{"dev":"vite"}}`, 0o644)
	if _, err := ScanWorkspace(); err != nil {
		t.Fatal(err)
	}
	logPath := setupFakeExecutable(t, "npm")

	cmd := NewRootCommand("test")
	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetIn(strings.NewReader("wrong\n"))
	cmd.SetArgs([]string{"setup", "run", "--all"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "confirmation did not match") {
		t.Fatalf("confirmation error = %v", err)
	}
	if data, err := os.ReadFile(logPath); err == nil && len(data) > 0 {
		t.Fatalf("setup commands ran without confirmation:\n%s", data)
	}
}

func TestRunAllProjectSetupsSelectsCommandsByKind(t *testing.T) {
	workspace := hardeningInitWorkspace(t, "code")
	projects := []Project{
		{
			ID: projectID("services/install-only"), Name: "install-only", Path: "services/install-only",
			Type: ProjectTypeLocal, HydrateMode: HydrateManual,
			Setup: Setup{PackageManager: "go", InstallCommand: "go mod download"},
		},
		{
			ID: projectID("apps/dev-only"), Name: "dev-only", Path: "apps/dev-only",
			Type: ProjectTypeLocal, HydrateMode: HydrateManual,
			Setup: Setup{PackageManager: "npm", DevCommand: "npm run dev"},
		},
		{
			ID: projectID("apps/both"), Name: "both", Path: "apps/both",
			Type: ProjectTypeLocal, HydrateMode: HydrateManual,
			Setup: Setup{PackageManager: "npm", InstallCommand: "npm install", DevCommand: "npm run dev"},
		},
	}
	for _, project := range projects {
		if err := os.MkdirAll(filepath.Join(workspace, project.Path), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := SaveManifest(workspace, Manifest{Version: ManifestVersion, WorkspaceRoot: workspace, Projects: projects}); err != nil {
		t.Fatal(err)
	}

	install, err := RunAllProjectSetups(SetupRunOptions{CommandKind: "install", DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	assertSetupRunProjects(t, install, "install-only", "both")
	dev, err := RunAllProjectSetups(SetupRunOptions{CommandKind: "dev", DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	assertSetupRunProjects(t, dev, "dev-only", "both")
}

func TestRunAllProjectSetupsPreflightsBeforeExecuting(t *testing.T) {
	workspace := hardeningInitWorkspace(t, "code")
	projects := []Project{
		{
			ID: projectID("apps/first"), Name: "first", Path: "apps/first",
			Type: ProjectTypeLocal, HydrateMode: HydrateManual,
			Setup: Setup{PackageManager: "npm", InstallCommand: "npm install"},
		},
		{
			ID: projectID("apps/invalid"), Name: "invalid", Path: "apps/invalid",
			Type: ProjectTypeLocal, HydrateMode: HydrateManual,
			Setup: Setup{PackageManager: "custom", InstallCommand: "./setup.sh"},
		},
	}
	for _, project := range projects {
		if err := os.MkdirAll(filepath.Join(workspace, project.Path), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := SaveManifest(workspace, Manifest{Version: ManifestVersion, WorkspaceRoot: workspace, Projects: projects}); err != nil {
		t.Fatal(err)
	}
	logPath := setupFakeExecutable(t, "npm")

	results, err := RunAllProjectSetups(SetupRunOptions{CommandKind: "install"})
	if err == nil || !strings.Contains(err.Error(), "refusing unknown setup command") {
		t.Fatalf("preflight error = %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("preflight returned partial results: %+v", results)
	}
	if data, err := os.ReadFile(logPath); err == nil && len(data) > 0 {
		t.Fatalf("setup ran before all projects passed preflight:\n%s", data)
	}
}

func assertSetupRunProjects(t *testing.T, results []SetupRunResult, want ...string) {
	t.Helper()
	if len(results) != len(want) {
		t.Fatalf("setup results = %+v, want projects %v", results, want)
	}
	for i, project := range want {
		if results[i].Project != project {
			t.Fatalf("setup result %d = %+v, want project %q", i, results[i], project)
		}
		if !results[i].DryRun {
			t.Fatalf("setup result %d is not a dry run: %+v", i, results[i])
		}
	}
}

func setupTestProject(t *testing.T, plan SetupPlan, name string) SetupPlanProject {
	t.Helper()
	for _, p := range plan.Projects {
		if p.Project == name {
			return p
		}
	}
	t.Fatalf("setup project %q not found in %+v", name, plan.Projects)
	return SetupPlanProject{}
}

func setupFakeExecutable(t *testing.T, name string) string {
	t.Helper()
	dir := t.TempDir()
	logPath := filepath.Join(dir, "commands.log")
	script := filepath.Join(dir, name)
	content := "#!/bin/sh\nprintf '%s|%s\\n' \"$PWD\" \"$*\" >> " + shellQuote(logPath) + "\n"
	if err := os.WriteFile(script, []byte(content), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return logPath
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}
