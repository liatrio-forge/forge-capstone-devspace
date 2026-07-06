package devspace

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
)

func TestHostedReconcileResolvesVersionConflict(t *testing.T) {
	root := t.TempDir()
	server := hostedSyncTestServer(t)
	workspaceA := filepath.Join(root, "machine-a", "code")
	workspaceB := filepath.Join(root, "machine-b", "code")
	homeA := filepath.Join(root, "home-a")
	homeB := filepath.Join(root, "home-b")

	t.Setenv(envHome, homeA)
	if _, err := InitWorkspace(workspaceA); err != nil {
		t.Fatal(err)
	}
	if _, err := SetHostedSync(server.URL, "test-token", "team-a"); err != nil {
		t.Fatal(err)
	}
	initial := Manifest{
		Version:       ManifestVersion,
		WorkspaceRoot: workspaceA,
		Projects:      []Project{hardeningProject("apps/base", ProjectTypeLocal, "")},
	}
	if err := SaveManifest(workspaceA, initial); err != nil {
		t.Fatal(err)
	}
	if result, err := PushHostedManifest(); err != nil {
		t.Fatal(err)
	} else if result.Version != 1 {
		t.Fatalf("initial hosted version = %d, want 1", result.Version)
	}

	t.Setenv(envHome, homeB)
	if _, err := InitWorkspace(workspaceB); err != nil {
		t.Fatal(err)
	}
	if _, err := SetHostedSync(server.URL, "test-token", "team-a"); err != nil {
		t.Fatal(err)
	}
	if result, err := PullHostedManifest(); err != nil {
		t.Fatal(err)
	} else if result.Version != 1 {
		t.Fatalf("pulled hosted version = %d, want 1", result.Version)
	}

	t.Setenv(envHome, homeA)
	localA, err := LoadManifest(workspaceA)
	if err != nil {
		t.Fatal(err)
	}
	localA.Projects = append(localA.Projects, hardeningProject("apps/remote", ProjectTypeLocal, ""))
	if err := SaveManifest(workspaceA, localA); err != nil {
		t.Fatal(err)
	}
	if result, err := PushHostedManifest(); err != nil {
		t.Fatal(err)
	} else if result.Version != 2 {
		t.Fatalf("remote update version = %d, want 2", result.Version)
	}

	t.Setenv(envHome, homeB)
	localB, err := LoadManifest(workspaceB)
	if err != nil {
		t.Fatal(err)
	}
	localB.Projects = append(localB.Projects, hardeningProject("apps/local", ProjectTypeLocal, ""))
	if err := SaveManifest(workspaceB, localB); err != nil {
		t.Fatal(err)
	}
	if _, err := PushHostedManifest(); err == nil || !strings.Contains(err.Error(), "changed since last sync") {
		t.Fatalf("push conflict error = %v", err)
	}

	plan, err := ReconcileHostedManifest("", true)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Backend != "hosted" || plan.RemoteSource.Version != 2 || len(plan.Conflicts) != 0 {
		t.Fatalf("plan = %+v", plan)
	}
	merged, err := LoadManifest(workspaceB)
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"apps/base", "apps/local", "apps/remote"} {
		if _, ok := findProject(merged, path); !ok {
			t.Fatalf("missing project %s after reconcile: %+v", path, merged.Projects)
		}
	}
	envelope := hostedSyncGet(t, server.URL, "team-a")
	if envelope.Version != 3 {
		t.Fatalf("hosted version = %d, want 3", envelope.Version)
	}
	for _, path := range []string{"apps/base", "apps/local", "apps/remote"} {
		if _, ok := findProject(envelope.Manifest, path); !ok {
			t.Fatalf("server missing project %s after reconcile: %+v", path, envelope.Manifest.Projects)
		}
	}
	st, err := LoadState()
	if err != nil {
		t.Fatal(err)
	}
	if st.HostedSyncVersion != envelope.Version || st.HostedSyncManifestHash != envelope.ManifestHash {
		t.Fatalf("hosted sync state = %+v, want version/hash %d/%s", st, envelope.Version, envelope.ManifestHash)
	}

	second, err := ReconcileHostedManifest("", true)
	if err != nil {
		t.Fatal(err)
	}
	if len(second.Ops) != 0 || len(second.Conflicts) != 0 {
		t.Fatalf("second reconcile ops=%+v conflicts=%+v", second.Ops, second.Conflicts)
	}
}

func TestHostedReconcileConflictBlocksPush(t *testing.T) {
	root := t.TempDir()
	server := hostedSyncTestServer(t)
	workspaceA := filepath.Join(root, "machine-a", "code")
	workspaceB := filepath.Join(root, "machine-b", "code")
	homeA := filepath.Join(root, "home-a")
	homeB := filepath.Join(root, "home-b")

	t.Setenv(envHome, homeA)
	if _, err := InitWorkspace(workspaceA); err != nil {
		t.Fatal(err)
	}
	if _, err := SetHostedSync(server.URL, "test-token", "team-a"); err != nil {
		t.Fatal(err)
	}
	project := hardeningProject("apps/app", ProjectTypeLocal, "")
	if err := SaveManifest(workspaceA, Manifest{
		Version:       ManifestVersion,
		WorkspaceRoot: workspaceA,
		Projects:      []Project{project},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := PushHostedManifest(); err != nil {
		t.Fatal(err)
	}

	t.Setenv(envHome, homeB)
	if _, err := InitWorkspace(workspaceB); err != nil {
		t.Fatal(err)
	}
	if _, err := SetHostedSync(server.URL, "test-token", "team-a"); err != nil {
		t.Fatal(err)
	}
	if _, err := PullHostedManifest(); err != nil {
		t.Fatal(err)
	}

	t.Setenv(envHome, homeA)
	remoteManifest, err := LoadManifest(workspaceA)
	if err != nil {
		t.Fatal(err)
	}
	remoteManifest.Projects[0].Name = "remote-app"
	if err := SaveManifest(workspaceA, remoteManifest); err != nil {
		t.Fatal(err)
	}
	if result, err := PushHostedManifest(); err != nil {
		t.Fatal(err)
	} else if result.Version != 2 {
		t.Fatalf("remote update version = %d, want 2", result.Version)
	}

	t.Setenv(envHome, homeB)
	localManifest, err := LoadManifest(workspaceB)
	if err != nil {
		t.Fatal(err)
	}
	localManifest.Projects[0].Name = "local-app"
	if err := SaveManifest(workspaceB, localManifest); err != nil {
		t.Fatal(err)
	}
	beforeLocal, err := os.ReadFile(manifestPath(workspaceB))
	if err != nil {
		t.Fatal(err)
	}
	beforeRemote := hostedSyncGet(t, server.URL, "team-a")

	plan, err := ReconcileHostedManifest("", true)
	if err == nil || !strings.Contains(err.Error(), "unresolved reconcile conflicts") {
		t.Fatalf("apply conflict error = %v", err)
	}
	if plan.Version == 0 || len(plan.Conflicts) == 0 {
		t.Fatalf("conflict plan = %+v", plan)
	}
	afterLocal, err := os.ReadFile(manifestPath(workspaceB))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(afterLocal, beforeLocal) {
		t.Fatal("conflicted hosted reconcile changed local manifest")
	}
	afterRemote := hostedSyncGet(t, server.URL, "team-a")
	if !reflect.DeepEqual(afterRemote, beforeRemote) {
		t.Fatalf("conflicted hosted reconcile changed server: before=%+v after=%+v", beforeRemote, afterRemote)
	}

	plan, err = ReconcileHostedManifest("remote", true)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Conflicts) != 0 {
		t.Fatalf("forced plan still has conflicts: %+v", plan.Conflicts)
	}
	appliedLocal, err := LoadManifest(workspaceB)
	if err != nil {
		t.Fatal(err)
	}
	if appliedLocal.Projects[0].Name != "remote-app" {
		t.Fatalf("force remote local project name = %q", appliedLocal.Projects[0].Name)
	}
	appliedRemote := hostedSyncGet(t, server.URL, "team-a")
	if appliedRemote.Version != 3 {
		t.Fatalf("forced hosted version = %d, want 3", appliedRemote.Version)
	}
	if appliedRemote.Manifest.Projects[0].Name != "remote-app" {
		t.Fatalf("force remote server project name = %q", appliedRemote.Manifest.Projects[0].Name)
	}
}

func TestHostedReconcileUpdatesBaseOnSuccess(t *testing.T) {
	root := t.TempDir()
	server := hostedSyncTestServer(t)
	workspaceA := filepath.Join(root, "machine-a", "code")
	workspaceB := filepath.Join(root, "machine-b", "code")
	homeA := filepath.Join(root, "home-a")
	homeB := filepath.Join(root, "home-b")

	t.Setenv(envHome, homeA)
	if _, err := InitWorkspace(workspaceA); err != nil {
		t.Fatal(err)
	}
	if _, err := SetHostedSync(server.URL, "test-token", "team-a"); err != nil {
		t.Fatal(err)
	}
	if err := SaveManifest(workspaceA, Manifest{
		Version:       ManifestVersion,
		WorkspaceRoot: workspaceA,
		Projects:      []Project{hardeningProject("apps/base", ProjectTypeLocal, "")},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := PushHostedManifest(); err != nil {
		t.Fatal(err)
	}

	t.Setenv(envHome, homeB)
	if _, err := InitWorkspace(workspaceB); err != nil {
		t.Fatal(err)
	}
	if _, err := SetHostedSync(server.URL, "test-token", "team-a"); err != nil {
		t.Fatal(err)
	}
	if _, err := PullHostedManifest(); err != nil {
		t.Fatal(err)
	}

	t.Setenv(envHome, homeA)
	remote, err := LoadManifest(workspaceA)
	if err != nil {
		t.Fatal(err)
	}
	remote.Projects = append(remote.Projects, hardeningProject("apps/remote", ProjectTypeLocal, ""))
	if err := SaveManifest(workspaceA, remote); err != nil {
		t.Fatal(err)
	}
	if _, err := PushHostedManifest(); err != nil {
		t.Fatal(err)
	}

	t.Setenv(envHome, homeB)
	local, err := LoadManifest(workspaceB)
	if err != nil {
		t.Fatal(err)
	}
	local.Projects = append(local.Projects, hardeningProject("apps/local", ProjectTypeLocal, ""))
	if err := SaveManifest(workspaceB, local); err != nil {
		t.Fatal(err)
	}
	plan, err := ReconcileHostedManifest("", true)
	if err != nil {
		t.Fatal(err)
	}
	base, ok, err := loadBaseManifest()
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("base manifest was not recorded")
	}
	want := manifestForSync(plan.Merged)
	if !reflect.DeepEqual(base, want) {
		t.Fatalf("base manifest = %+v, want %+v", base, want)
	}
}

func TestHostedReconcileUserConflictDoesNotClobberServer(t *testing.T) {
	root := t.TempDir()
	server := hostedSyncTestServer(t)
	workspaceA := filepath.Join(root, "machine-a", "code")
	workspaceB := filepath.Join(root, "machine-b", "code")
	homeA := filepath.Join(root, "home-a")
	homeB := filepath.Join(root, "home-b")
	user := reconcileUser()

	t.Setenv(envHome, homeA)
	if _, err := InitWorkspace(workspaceA); err != nil {
		t.Fatal(err)
	}
	if _, err := SetHostedSync(server.URL, "test-token", "team-a"); err != nil {
		t.Fatal(err)
	}
	if err := SaveManifest(workspaceA, Manifest{
		Version:       ManifestVersion,
		WorkspaceRoot: workspaceA,
		Projects:      []Project{hardeningProject("apps/app", ProjectTypeLocal, "")},
		Users:         []User{user},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := PushHostedManifest(); err != nil {
		t.Fatal(err)
	}

	t.Setenv(envHome, homeB)
	if _, err := InitWorkspace(workspaceB); err != nil {
		t.Fatal(err)
	}
	if _, err := SetHostedSync(server.URL, "test-token", "team-a"); err != nil {
		t.Fatal(err)
	}
	if _, err := PullHostedManifest(); err != nil {
		t.Fatal(err)
	}

	t.Setenv(envHome, homeA)
	remoteManifest, err := LoadManifest(workspaceA)
	if err != nil {
		t.Fatal(err)
	}
	remoteManifest.Users[0].AgeRecipient = reconcileRotatedRecipient
	if err := SaveManifest(workspaceA, remoteManifest); err != nil {
		t.Fatal(err)
	}
	if _, err := PushHostedManifest(); err != nil {
		t.Fatal(err)
	}

	t.Setenv(envHome, homeB)
	localManifest, err := LoadManifest(workspaceB)
	if err != nil {
		t.Fatal(err)
	}
	localManifest.Users[0].Name = "Renamed Locally"
	if err := SaveManifest(workspaceB, localManifest); err != nil {
		t.Fatal(err)
	}
	beforeRemote := hostedSyncGet(t, server.URL, "team-a")

	plan, err := ReconcileHostedManifest("", true)
	if err == nil || !strings.Contains(err.Error(), "unresolved reconcile conflicts") {
		t.Fatalf("apply conflict error = %v", err)
	}
	if len(plan.Conflicts) != 1 || plan.Conflicts[0].Entity != "user" || plan.Conflicts[0].Key != user.ID {
		t.Fatalf("conflicts = %+v, want one user conflict", plan.Conflicts)
	}
	afterRemote := hostedSyncGet(t, server.URL, "team-a")
	if !reflect.DeepEqual(afterRemote, beforeRemote) {
		t.Fatalf("blocked reconcile changed server users: before=%+v after=%+v", beforeRemote, afterRemote)
	}

	if _, err := ReconcileHostedManifest("local", true); err != nil {
		t.Fatal(err)
	}
	forced := hostedSyncGet(t, server.URL, "team-a")
	if len(forced.Manifest.Users) != 1 {
		t.Fatalf("server users = %+v", forced.Manifest.Users)
	}
	got := forced.Manifest.Users[0]
	if got.Name != "Renamed Locally" || got.AgeRecipient != user.AgeRecipient {
		t.Fatalf("force local server user = %+v, want local record", got)
	}
	applied, err := LoadManifest(workspaceB)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(applied.Users, forced.Manifest.Users) {
		t.Fatalf("local users = %+v, server users = %+v", applied.Users, forced.Manifest.Users)
	}
}

func TestHostedReconcileHashGuardRefusesApply(t *testing.T) {
	root := t.TempDir()
	workspace := filepath.Join(root, "code")
	inner, err := NewHostedSyncServer(HostedSyncServerOptions{StoreDir: t.TempDir(), Token: "test-token"})
	if err != nil {
		t.Fatal(err)
	}
	// While enabled, every GET mutates the workspace manifest before responding,
	// simulating a concurrent local edit between plan generation and apply.
	var mutate atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && mutate.Load() {
			m, loadErr := LoadManifest(workspace)
			if loadErr != nil {
				t.Error(loadErr)
			} else {
				m.Projects = append(m.Projects, hardeningProject("apps/raced", ProjectTypeLocal, ""))
				if saveErr := SaveManifest(workspace, m); saveErr != nil {
					t.Error(saveErr)
				}
			}
		}
		inner.ServeHTTP(w, r)
	}))
	t.Cleanup(server.Close)

	t.Setenv(envHome, t.TempDir())
	if _, err := InitWorkspace(workspace); err != nil {
		t.Fatal(err)
	}
	if _, err := SetHostedSync(server.URL, "test-token", "team-a"); err != nil {
		t.Fatal(err)
	}
	if err := SaveManifest(workspace, Manifest{
		Version:       ManifestVersion,
		WorkspaceRoot: workspace,
		Projects:      []Project{hardeningProject("apps/base", ProjectTypeLocal, "")},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := PushHostedManifest(); err != nil {
		t.Fatal(err)
	}

	mutate.Store(true)
	if _, err := ReconcileHostedManifest("", true); err == nil || !strings.Contains(err.Error(), "manifest changed since reconcile was generated") {
		t.Fatalf("hash guard error = %v", err)
	}
	mutate.Store(false)
	envelope := hostedSyncGet(t, server.URL, "team-a")
	if envelope.Version != 1 {
		t.Fatalf("hosted version = %d, want 1 (guard must block the push)", envelope.Version)
	}
}
