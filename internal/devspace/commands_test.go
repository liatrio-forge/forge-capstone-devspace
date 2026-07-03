package devspace

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestResolveServeAddrRejectsExplicitPublicWithoutOptIn(t *testing.T) {
	_, err := resolveServeAddr("0.0.0.0:8787", true, "", false)
	if err == nil || !strings.Contains(err.Error(), "refusing to bind public address") {
		t.Fatalf("expected public-bind guard error, got %v", err)
	}
}

func TestResolveServeAddrAllowsPublicWithFlag(t *testing.T) {
	got, err := resolveServeAddr("0.0.0.0:8787", true, "", true)
	if err != nil {
		t.Fatalf("unexpected error with --allow-public-http: %v", err)
	}
	if got.addr != "0.0.0.0:8787" {
		t.Fatalf("addr = %q", got.addr)
	}
	if !got.public {
		t.Fatal("expected public=true for 0.0.0.0 bind")
	}
}

func TestResolveServeAddrAllowsPublicWithPortEnv(t *testing.T) {
	// PORT env implies a PaaS/proxy that terminates TLS upstream, so a public
	// bind is sanctioned without --allow-public-http.
	got, err := resolveServeAddr("127.0.0.1:8787", false, "8080", false)
	if err != nil {
		t.Fatalf("unexpected error with PORT env: %v", err)
	}
	if got.addr != "0.0.0.0:8080" {
		t.Fatalf("addr = %q, want 0.0.0.0:8080", got.addr)
	}
	if !got.public {
		t.Fatal("expected public=true for PORT-driven 0.0.0.0 bind")
	}
}

func TestResolveServeAddrAllowsLoopback(t *testing.T) {
	got, err := resolveServeAddr("127.0.0.1:8787", true, "", false)
	if err != nil {
		t.Fatalf("loopback bind should not require opt-in: %v", err)
	}
	if got.public {
		t.Fatal("expected public=false for loopback bind")
	}
	if got.addr != "127.0.0.1:8787" {
		t.Fatalf("addr = %q", got.addr)
	}
}

func TestResolveServeAddrRejectsInvalidAddr(t *testing.T) {
	if _, err := resolveServeAddr("not-a-valid-addr", true, "", false); err == nil {
		t.Fatal("expected error for invalid listen address")
	}
}

// executeCommand wires a fresh root command to buffer-backed stdout/stderr, sets
// the given args, executes it, and returns the captured output and the error.
// It does NOT touch the real DEVSPACE_HOME; callers set that up first.
func executeCommand(t *testing.T, version string, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	cmd := NewRootCommand(version)
	var out, errBuf bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)
	cmd.SetArgs(args)
	err = cmd.Execute()
	return out.String(), errBuf.String(), err
}

func TestVersionCommandPrintsVersion(t *testing.T) {
	stdout, _, err := executeCommand(t, "v1.2.3-test", "version")
	if err != nil {
		t.Fatalf("version error: %v", err)
	}
	if strings.TrimSpace(stdout) != "v1.2.3-test" {
		t.Fatalf("version output = %q, want %q", stdout, "v1.2.3-test")
	}
}

func TestRootHelpListsSubcommands(t *testing.T) {
	// --help on root: SilenceUsage is true, but help still prints to stdout.
	stdout, _, err := executeCommand(t, "test", "--help")
	if err != nil {
		t.Fatalf("root --help error: %v", err)
	}
	for _, want := range []string{"init", "scan", "watch", "plan", "apply", "hosted", "env", "doctor"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("root --help missing subcommand %q in output:\n%s", want, stdout)
		}
	}
}

func TestHostedServeHelpDocumentsFlags(t *testing.T) {
	stdout, _, err := executeCommand(t, "test", "hosted", "serve", "--help")
	if err != nil {
		t.Fatalf("hosted serve --help error: %v", err)
	}
	for _, want := range []string{"--addr", "--token", "--trusted-proxy", "--allow-public-http"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("hosted serve --help missing flag %q in output:\n%s", want, stdout)
		}
	}
}

func TestWorkspaceRemoteSetHelpDocumentsCommitFlags(t *testing.T) {
	stdout, _, err := executeCommand(t, "test", "workspace", "remote", "set", "--help")
	if err != nil {
		t.Fatalf("workspace remote set --help error: %v", err)
	}
	for _, want := range []string{"--commit-email", "--commit-name"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("workspace remote set --help missing flag %q in output:\n%s", want, stdout)
		}
	}
}

// initCommandWorkspace prepares an isolated DEVSPACE_HOME + initialized
// workspace and returns the workspace root. Used by the command tests below.
func initCommandWorkspace(t *testing.T) string {
	t.Helper()
	t.Setenv(envHome, t.TempDir())
	workspace := filepath.Join(t.TempDir(), "code")
	if _, err := InitWorkspace(workspace); err != nil {
		t.Fatalf("InitWorkspace: %v", err)
	}
	return workspace
}

func TestScanCommandReportsEmptyWorkspace(t *testing.T) {
	initCommandWorkspace(t)
	stdout, _, err := executeCommand(t, "test", "scan")
	if err != nil {
		t.Fatalf("scan error: %v", err)
	}
	if !strings.Contains(stdout, "Found 0 projects") {
		t.Fatalf("scan output = %q, want 0 projects", stdout)
	}
}

func TestStatusCommandReportsWorkspaceHealth(t *testing.T) {
	initCommandWorkspace(t)
	stdout, _, err := executeCommand(t, "test", "status")
	if err != nil {
		t.Fatalf("status error: %v", err)
	}
	for _, want := range []string{"Projects tracked: 0", "Machine:", "Workspace:"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("status output missing %q:\n%s", want, stdout)
		}
	}
}

func TestDoctorCommandRunsAndReports(t *testing.T) {
	initCommandWorkspace(t)
	stdout, _, err := executeCommand(t, "test", "doctor")
	if err != nil {
		t.Fatalf("doctor error: %v", err)
	}
	// doctor always prints something; just confirm it produced output.
	if strings.TrimSpace(stdout) == "" {
		t.Fatal("doctor produced no output")
	}
}

func TestProjectAddAndStatusCommands(t *testing.T) {
	initCommandWorkspace(t)

	// Track a project via the command surface.
	stdout, _, err := executeCommand(t, "test", "project", "add", "apps/app")
	if err != nil {
		t.Fatalf("project add error: %v", err)
	}
	if !strings.Contains(stdout, "Added project app at apps/app") {
		t.Fatalf("project add output = %q", stdout)
	}

	// Status for a single tracked project.
	stdout, _, err = executeCommand(t, "test", "project", "status", "app")
	if err != nil {
		t.Fatalf("project status error: %v", err)
	}
	for _, want := range []string{"Project: app", "Path: apps/app", "Hydrated:"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("project status output missing %q:\n%s", want, stdout)
		}
	}
}

func TestPlanAndApplyCommandsRoundTrip(t *testing.T) {
	initCommandWorkspace(t)
	if _, _, err := executeCommand(t, "test", "project", "add", "services/api"); err != nil {
		t.Fatalf("project add error: %v", err)
	}

	planOut, _, err := executeCommand(t, "test", "plan")
	if err != nil {
		t.Fatalf("plan error: %v", err)
	}
	if !strings.Contains(planOut, "Planned changes:") {
		t.Fatalf("plan output = %q", planOut)
	}

	applyOut, _, err := executeCommand(t, "test", "apply")
	if err != nil {
		t.Fatalf("apply error: %v", err)
	}
	if !strings.Contains(applyOut, "Applied safe plan actions.") {
		t.Fatalf("apply output = %q", applyOut)
	}
}

func TestSetupPlanCommandReportsNoSetupCommands(t *testing.T) {
	// An empty workspace has no detected setup commands; the plan prints the
	// "(none)" branch of printSetupPlan and exits cleanly.
	initCommandWorkspace(t)
	stdout, _, err := executeCommand(t, "test", "setup", "plan")
	if err != nil {
		t.Fatalf("setup plan error: %v", err)
	}
	if !strings.Contains(stdout, "Setup commands:") || !strings.Contains(stdout, "(none)") {
		t.Fatalf("setup plan output = %q", stdout)
	}
}

func TestProfileOrDefaultAndSortedProjectNames(t *testing.T) {
	// Directly exercise the two trivial helpers that the command scenarios
	// above don't reach.
	if profileOrDefault("") != "dev" {
		t.Fatal("profileOrDefault('') should be 'dev'")
	}
	if profileOrDefault("prod") != "prod" {
		t.Fatal("profileOrDefault('prod') should be 'prod'")
	}
	m := Manifest{Projects: []Project{{Name: "zeta"}, {Name: "alpha"}}}
	if got := sortedProjectNames(m); got != "alpha, zeta" {
		t.Fatalf("sortedProjectNames = %q, want %q", got, "alpha, zeta")
	}
}

// Silence prevents unused-import warnings if cobra is only referenced in
// helpers above; keep the import explicit.
var _ = cobra.Command{}
