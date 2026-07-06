package devspace

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestReconcileMergeCases(t *testing.T) {
	user := reconcileUser()
	baseProject := testMergeProject("project_app", "apps/app")
	localProject := testMergeProject("project_local", "apps/local")
	remoteProject := testMergeProject("project_remote", "apps/remote")
	baseAccess := ProjectAccess{ProjectID: baseProject.ID, UserID: user.ID, Role: AccessRoleDeveloper, AddedAt: "2026-01-01T00:00:00Z"}

	base := testMergeManifest(user, []Project{baseProject}, []ProjectAccess{baseAccess})
	emptyBase := testMergeManifest(user, nil, nil)
	local := base
	remote := base

	cases := []struct {
		name          string
		base          *Manifest
		local         Manifest
		remote        Manifest
		wantProjects  []Project
		wantOps       []ReconcileOp
		wantConflicts int
		wantTwoWay    bool
	}{
		{
			name:         "add-one-side",
			base:         &base,
			local:        local,
			remote:       testMergeManifest(user, []Project{baseProject, remoteProject}, []ProjectAccess{baseAccess}),
			wantProjects: []Project{baseProject, remoteProject},
			wantOps:      []ReconcileOp{{Action: "added", Kind: "project", Key: remoteProject.Path}},
		},
		{
			name:         "remove-one-side",
			base:         &base,
			local:        local,
			remote:       testMergeManifest(user, nil, nil),
			wantProjects: nil,
			wantOps:      []ReconcileOp{{Action: "removed", Kind: "project", Key: baseProject.Path}, {Action: "removed", Kind: "access", Key: accessKey(baseAccess)}},
		},
		{
			name:         "change-one-side",
			base:         &base,
			local:        local,
			remote:       testMergeManifest(user, []Project{withMergeRemote(baseProject, "git@example.com:team/app.git")}, []ProjectAccess{baseAccess}),
			wantProjects: []Project{withMergeRemote(baseProject, "git@example.com:team/app.git")},
			wantOps:      []ReconcileOp{{Action: "changed", Kind: "project", Key: baseProject.Path}},
		},
		{
			name:          "change-both-different",
			base:          &base,
			local:         testMergeManifest(user, []Project{withMergeName(baseProject, "ours")}, []ProjectAccess{baseAccess}),
			remote:        testMergeManifest(user, []Project{withMergeName(baseProject, "theirs")}, []ProjectAccess{baseAccess}),
			wantProjects:  []Project{withMergeName(baseProject, "ours")},
			wantConflicts: 1,
		},
		{
			name:          "add-add-different",
			base:          &emptyBase,
			local:         testMergeManifest(user, []Project{withMergeName(baseProject, "ours")}, nil),
			remote:        testMergeManifest(user, []Project{withMergeName(baseProject, "theirs")}, nil),
			wantProjects:  []Project{withMergeName(baseProject, "ours")},
			wantConflicts: 1,
		},
		{
			name:          "change-vs-remove",
			base:          &base,
			local:         testMergeManifest(user, []Project{withMergeName(baseProject, "ours")}, []ProjectAccess{baseAccess}),
			remote:        testMergeManifest(user, nil, nil),
			wantProjects:  []Project{withMergeName(baseProject, "ours")},
			wantOps:       []ReconcileOp{{Action: "removed", Kind: "access", Key: accessKey(baseAccess)}},
			wantConflicts: 1,
		},
		{
			name:         "nil-base-two-way-additions",
			local:        testMergeManifest(user, []Project{localProject}, nil),
			remote:       testMergeManifest(user, []Project{remoteProject}, nil),
			wantProjects: []Project{localProject, remoteProject},
			wantOps:      []ReconcileOp{{Action: "added", Kind: "project", Key: remoteProject.Path}},
			wantTwoWay:   true,
		},
		{
			name:          "nil-base-two-way-differences-conflict",
			local:         testMergeManifest(user, []Project{withMergeName(baseProject, "ours")}, nil),
			remote:        testMergeManifest(user, []Project{withMergeName(baseProject, "theirs")}, nil),
			wantProjects:  []Project{withMergeName(baseProject, "ours")},
			wantConflicts: 1,
			wantTwoWay:    true,
		},
		{
			name:         "idempotent",
			base:         &base,
			local:        local,
			remote:       remote,
			wantProjects: []Project{baseProject},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := reconcileManifests(tc.base, tc.local, tc.remote)
			if err != nil {
				t.Fatal(err)
			}
			if got.TwoWay != tc.wantTwoWay {
				t.Fatalf("TwoWay = %t, want %t", got.TwoWay, tc.wantTwoWay)
			}
			if len(got.Conflicts) != tc.wantConflicts {
				t.Fatalf("conflicts = %d, want %d: %+v", len(got.Conflicts), tc.wantConflicts, got.Conflicts)
			}
			if !reflect.DeepEqual(got.Merged.Projects, tc.wantProjects) {
				t.Fatalf("projects = %+v, want %+v", got.Merged.Projects, tc.wantProjects)
			}
			if !reflect.DeepEqual(got.Ops, tc.wantOps) {
				t.Fatalf("ops = %+v, want %+v", got.Ops, tc.wantOps)
			}
		})
	}
}

func TestWorkspaceReconcileNonConflicting(t *testing.T) {
	root := t.TempDir()
	remote := workspaceSyncBareRepo(t)
	workspaceA := filepath.Join(root, "machine-a", "code")
	workspaceB := filepath.Join(root, "machine-b", "code")
	homeA := filepath.Join(root, "home-a")
	homeB := filepath.Join(root, "home-b")

	t.Setenv(envHome, homeA)
	if _, err := InitWorkspace(workspaceA); err != nil {
		t.Fatal(err)
	}
	if _, err := SetManifestRemote(remote); err != nil {
		t.Fatal(err)
	}
	base := Manifest{
		Version:       ManifestVersion,
		WorkspaceRoot: workspaceA,
		Projects:      []Project{hardeningProject("apps/base", ProjectTypeLocal, "")},
	}
	if err := SaveManifest(workspaceA, base); err != nil {
		t.Fatal(err)
	}
	if _, err := PushWorkspaceManifest(); err != nil {
		t.Fatal(err)
	}

	t.Setenv(envHome, homeB)
	if _, err := InitWorkspace(workspaceB); err != nil {
		t.Fatal(err)
	}
	if _, err := SetManifestRemote(remote); err != nil {
		t.Fatal(err)
	}
	if _, err := PullWorkspaceManifest(); err != nil {
		t.Fatal(err)
	}
	localB, err := LoadManifest(workspaceB)
	if err != nil {
		t.Fatal(err)
	}
	localB.Projects = append(localB.Projects, hardeningProject("apps/remote", ProjectTypeLocal, ""))
	if err := SaveManifest(workspaceB, localB); err != nil {
		t.Fatal(err)
	}
	if _, err := PushWorkspaceManifest(); err != nil {
		t.Fatal(err)
	}

	t.Setenv(envHome, homeA)
	localA, err := LoadManifest(workspaceA)
	if err != nil {
		t.Fatal(err)
	}
	localA.Projects = append(localA.Projects, hardeningProject("apps/local", ProjectTypeLocal, ""))
	if err := SaveManifest(workspaceA, localA); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(manifestPath(workspaceA))
	if err != nil {
		t.Fatal(err)
	}

	plan, err := ReconcileWorkspaceManifest("", true)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Conflicts) != 0 || len(plan.Ops) != 1 {
		t.Fatalf("plan conflicts=%+v ops=%+v", plan.Conflicts, plan.Ops)
	}
	merged, err := LoadManifest(workspaceA)
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"apps/base", "apps/local", "apps/remote"} {
		if _, ok := findProject(merged, path); !ok {
			t.Fatalf("missing project %s after reconcile: %+v", path, merged.Projects)
		}
	}
	backup, err := os.ReadFile(filepath.Join(homeA, "manifest-backup.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(backup, before) {
		t.Fatal("backup does not match pre-reconcile manifest")
	}
	baseSnapshot, ok, err := loadBaseManifest()
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("base snapshot missing")
	}
	if _, ok := findProject(baseSnapshot, "apps/local"); ok {
		t.Fatalf("base snapshot recorded unpushed local project: %+v", baseSnapshot.Projects)
	}
	if _, ok := findProject(baseSnapshot, "apps/remote"); ok {
		t.Fatalf("base snapshot recorded remote project before local push: %+v", baseSnapshot.Projects)
	}

	t.Setenv(envHome, homeB)
	nextRemote, err := LoadManifest(workspaceB)
	if err != nil {
		t.Fatal(err)
	}
	nextRemote.Projects = append(nextRemote.Projects, hardeningProject("apps/another-remote", ProjectTypeLocal, ""))
	if err := SaveManifest(workspaceB, nextRemote); err != nil {
		t.Fatal(err)
	}
	if _, err := PushWorkspaceManifest(); err != nil {
		t.Fatal(err)
	}

	t.Setenv(envHome, homeA)
	nextPlan, err := ReconcileWorkspaceManifest("", true)
	if err != nil {
		t.Fatal(err)
	}
	if len(nextPlan.Conflicts) != 0 {
		t.Fatalf("next reconcile conflicts=%+v", nextPlan.Conflicts)
	}
	merged, err = LoadManifest(workspaceA)
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"apps/base", "apps/local", "apps/remote", "apps/another-remote"} {
		if _, ok := findProject(merged, path); !ok {
			t.Fatalf("missing project %s after second reconcile: %+v", path, merged.Projects)
		}
	}
	if changed, err := PushWorkspaceManifest(); err != nil || !changed {
		t.Fatalf("push after reconcile changed=%t err=%v", changed, err)
	}
	baseSnapshot, ok, err = loadBaseManifest()
	if err != nil {
		t.Fatal(err)
	}
	if !ok || !reflect.DeepEqual(baseSnapshot.Projects, manifestForSync(merged).Projects) {
		t.Fatalf("base snapshot = %+v ok=%t, want pushed merged %+v", baseSnapshot.Projects, ok, merged.Projects)
	}

	second, err := ReconcileWorkspaceManifest("", true)
	if err != nil {
		t.Fatal(err)
	}
	if len(second.Ops) != 0 || len(second.Conflicts) != 0 {
		t.Fatalf("second reconcile ops=%+v conflicts=%+v", second.Ops, second.Conflicts)
	}
}

func TestWorkspaceReconcileConflictBlocksApply(t *testing.T) {
	root := t.TempDir()
	remote := workspaceSyncBareRepo(t)
	workspaceA := filepath.Join(root, "machine-a", "code")
	workspaceB := filepath.Join(root, "machine-b", "code")
	homeA := filepath.Join(root, "home-a")
	homeB := filepath.Join(root, "home-b")

	t.Setenv(envHome, homeA)
	if _, err := InitWorkspace(workspaceA); err != nil {
		t.Fatal(err)
	}
	if _, err := SetManifestRemote(remote); err != nil {
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
	if _, err := PushWorkspaceManifest(); err != nil {
		t.Fatal(err)
	}

	t.Setenv(envHome, homeB)
	if _, err := InitWorkspace(workspaceB); err != nil {
		t.Fatal(err)
	}
	if _, err := SetManifestRemote(remote); err != nil {
		t.Fatal(err)
	}
	if _, err := PullWorkspaceManifest(); err != nil {
		t.Fatal(err)
	}
	remoteManifest, err := LoadManifest(workspaceB)
	if err != nil {
		t.Fatal(err)
	}
	remoteManifest.Projects[0].Name = "remote-app"
	if err := SaveManifest(workspaceB, remoteManifest); err != nil {
		t.Fatal(err)
	}
	if _, err := PushWorkspaceManifest(); err != nil {
		t.Fatal(err)
	}

	t.Setenv(envHome, homeA)
	localManifest, err := LoadManifest(workspaceA)
	if err != nil {
		t.Fatal(err)
	}
	localManifest.Projects[0].Name = "local-app"
	if err := SaveManifest(workspaceA, localManifest); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(manifestPath(workspaceA))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ReconcileWorkspaceManifest("", true); err == nil || !strings.Contains(err.Error(), "unresolved reconcile conflicts") {
		t.Fatalf("apply conflict error = %v", err)
	}
	after, err := os.ReadFile(manifestPath(workspaceA))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(after, before) {
		t.Fatal("conflicted apply changed local manifest")
	}

	if _, err := ReconcileWorkspaceManifest("remote", true); err != nil {
		t.Fatal(err)
	}
	appliedRemote, err := LoadManifest(workspaceA)
	if err != nil {
		t.Fatal(err)
	}
	if appliedRemote.Projects[0].Name != "remote-app" {
		t.Fatalf("force remote project name = %q", appliedRemote.Projects[0].Name)
	}

	appliedRemote.Projects[0].Name = "local-app"
	if err := SaveManifest(workspaceA, appliedRemote); err != nil {
		t.Fatal(err)
	}
	if err := recordBaseManifest(Manifest{
		Version:       ManifestVersion,
		WorkspaceRoot: workspaceA,
		Projects:      []Project{project},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := ReconcileWorkspaceManifest("local", true); err != nil {
		t.Fatal(err)
	}
	appliedLocal, err := LoadManifest(workspaceA)
	if err != nil {
		t.Fatal(err)
	}
	if appliedLocal.Projects[0].Name != "local-app" {
		t.Fatalf("force local project name = %q", appliedLocal.Projects[0].Name)
	}
}

func TestReconcileSamePathDifferentIDConflict(t *testing.T) {
	user := reconcileUser()
	localProject := testMergeProject("project_local", "apps/app")
	remoteProject := testMergeProject("project_remote", "apps/app")
	local := testMergeManifest(user, []Project{localProject}, nil)
	remote := testMergeManifest(user, []Project{remoteProject}, nil)
	emptyBase := testMergeManifest(user, nil, nil)

	cases := []struct {
		name string
		base *Manifest
	}{
		{name: "three-way", base: &emptyBase},
		{name: "two-way", base: nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := reconcileManifests(tc.base, local, remote)
			if err != nil {
				t.Fatal(err)
			}
			if len(result.Conflicts) != 1 {
				t.Fatalf("conflicts = %+v, want 1", result.Conflicts)
			}
			conflict := result.Conflicts[0]
			if conflict.Entity != "project" || conflict.Key != "apps/app" || conflict.Field != "id" {
				t.Fatalf("conflict = %+v, want project apps/app id", conflict)
			}
			forcedLocal := forceReconcileConflicts(result.Merged, result.Conflicts, local, remote, "local")
			if err := ValidateManifest(forcedLocal); err != nil {
				t.Fatalf("force local failed validation: %v", err)
			}
			if len(forcedLocal.Projects) != 1 || forcedLocal.Projects[0].ID != localProject.ID {
				t.Fatalf("force local projects = %+v", forcedLocal.Projects)
			}
			forcedRemote := forceReconcileConflicts(result.Merged, result.Conflicts, local, remote, "remote")
			if err := ValidateManifest(forcedRemote); err != nil {
				t.Fatalf("force remote failed validation: %v", err)
			}
			if len(forcedRemote.Projects) != 1 || forcedRemote.Projects[0].ID != remoteProject.ID {
				t.Fatalf("force remote projects = %+v", forcedRemote.Projects)
			}
		})
	}
}

func TestReconcileUserTeamRemoteOnlyChangeMerges(t *testing.T) {
	user := reconcileUser()
	rotated := user
	rotated.AgeRecipient = reconcileRotatedRecipient
	team := Team{ID: "team_1", Name: "Core", CreatedAt: "2026-01-01T00:00:00Z"}
	renamedTeam := team
	renamedTeam.Name = "Platform"

	base := testMergeManifest(user, nil, nil)
	base.Teams = []Team{team}
	local := base
	remote := testMergeManifest(rotated, nil, nil)
	remote.Teams = []Team{renamedTeam}

	result, err := reconcileManifests(&base, local, remote)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Conflicts) != 0 {
		t.Fatalf("conflicts = %+v, want none", result.Conflicts)
	}
	if len(result.Merged.Users) != 1 || result.Merged.Users[0].AgeRecipient != reconcileRotatedRecipient {
		t.Fatalf("merged users = %+v, want rotated recipient", result.Merged.Users)
	}
	if len(result.Merged.Teams) != 1 || result.Merged.Teams[0].Name != "Platform" {
		t.Fatalf("merged teams = %+v, want renamed team", result.Merged.Teams)
	}
	wantOps := []ReconcileOp{
		{Action: "changed", Kind: "user", Key: user.ID},
		{Action: "changed", Kind: "team", Key: team.ID},
	}
	if !reflect.DeepEqual(result.Ops, wantOps) {
		t.Fatalf("ops = %+v, want %+v", result.Ops, wantOps)
	}
}

func TestReconcileUserBothChangedConflict(t *testing.T) {
	user := reconcileUser()
	localUser := user
	localUser.Name = "Local Name"
	remoteUser := user
	remoteUser.AgeRecipient = reconcileRotatedRecipient

	base := testMergeManifest(user, nil, nil)
	local := testMergeManifest(localUser, nil, nil)
	remote := testMergeManifest(remoteUser, nil, nil)

	result, err := reconcileManifests(&base, local, remote)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Conflicts) != 1 {
		t.Fatalf("conflicts = %+v, want 1", result.Conflicts)
	}
	conflict := result.Conflicts[0]
	if conflict.Entity != "user" || conflict.Key != user.ID {
		t.Fatalf("conflict = %+v, want user %s", conflict, user.ID)
	}
	forced := forceReconcileConflicts(result.Merged, result.Conflicts, local, remote, "remote")
	if err := ValidateManifest(forced); err != nil {
		t.Fatalf("force remote failed validation: %v", err)
	}
	if !reflect.DeepEqual(forced.Users, []User{remoteUser}) {
		t.Fatalf("forced users = %+v, want %+v", forced.Users, remoteUser)
	}
}

func TestReconcileTwoWayUserConflict(t *testing.T) {
	user := reconcileUser()
	remoteUser := user
	remoteUser.Name = "Remote Name"

	result, err := reconcileManifests(nil, testMergeManifest(user, nil, nil), testMergeManifest(remoteUser, nil, nil))
	if err != nil {
		t.Fatal(err)
	}
	if !result.TwoWay {
		t.Fatal("expected two-way mode")
	}
	if len(result.Conflicts) != 1 {
		t.Fatalf("conflicts = %+v, want 1", result.Conflicts)
	}
	conflict := result.Conflicts[0]
	if conflict.Entity != "user" || conflict.Key != user.ID || conflict.Field != "*" {
		t.Fatalf("conflict = %+v, want user %s *", conflict, user.ID)
	}
}

func TestWorkspaceReconcileJSONOutput(t *testing.T) {
	root := t.TempDir()
	remote := workspaceSyncBareRepo(t)
	workspaceA := filepath.Join(root, "machine-a", "code")
	workspaceB := filepath.Join(root, "machine-b", "code")
	homeA := filepath.Join(root, "home-a")
	homeB := filepath.Join(root, "home-b")

	t.Setenv(envHome, homeA)
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

	t.Setenv(envHome, homeB)
	if _, err := InitWorkspace(workspaceB); err != nil {
		t.Fatal(err)
	}
	if _, err := SetManifestRemote(remote); err != nil {
		t.Fatal(err)
	}
	if _, err := PullWorkspaceManifest(); err != nil {
		t.Fatal(err)
	}
	remoteManifest, err := LoadManifest(workspaceB)
	if err != nil {
		t.Fatal(err)
	}
	remoteManifest.Projects[0].Name = "remote-app"
	if err := SaveManifest(workspaceB, remoteManifest); err != nil {
		t.Fatal(err)
	}
	if _, err := PushWorkspaceManifest(); err != nil {
		t.Fatal(err)
	}

	t.Setenv(envHome, homeA)
	localManifest, err := LoadManifest(workspaceA)
	if err != nil {
		t.Fatal(err)
	}
	localManifest.Projects[0].Name = "local-app"
	if err := SaveManifest(workspaceA, localManifest); err != nil {
		t.Fatal(err)
	}

	stdout, _, err := executeCommand(t, "test", "workspace", "reconcile", "--json")
	if err != nil {
		t.Fatalf("reconcile --json error: %v", err)
	}
	var plan map[string]any
	if err := json.Unmarshal([]byte(stdout), &plan); err != nil {
		t.Fatalf("unmarshal reconcile --json output: %v\n%s", err, stdout)
	}
	if plan["backend"] != "git" {
		t.Fatalf("backend = %v, want git", plan["backend"])
	}
	hash, _ := plan["manifestHash"].(string)
	if hash == "" {
		t.Fatalf("manifestHash missing in %s", stdout)
	}
	if _, ok := plan["merged"].(map[string]any); !ok {
		t.Fatalf("merged missing in %s", stdout)
	}
	conflicts, _ := plan["conflicts"].([]any)
	if len(conflicts) != 1 {
		t.Fatalf("conflicts = %v, want 1", plan["conflicts"])
	}
	conflict, _ := conflicts[0].(map[string]any)
	if conflict["entity"] != "project" || conflict["key"] != "apps/app" || conflict["field"] != "*" {
		t.Fatalf("conflict = %v, want camelCase project/apps/app/*", conflict)
	}
	for _, want := range []string{"ours", "theirs"} {
		if value, _ := conflict[want].(string); value == "" {
			t.Fatalf("conflict %q missing in %v", want, conflict)
		}
	}
	for _, stale := range []string{"Entity", "Key", "Field", "Ours", "Theirs"} {
		if _, ok := conflict[stale]; ok {
			t.Fatalf("conflict still serializes PascalCase key %q: %v", stale, conflict)
		}
	}
}

func TestReconcileForceFlagsMutuallyExclusive(t *testing.T) {
	t.Setenv(envHome, t.TempDir())
	for _, args := range [][]string{
		{"workspace", "reconcile", "--force-local", "--force-remote"},
		{"hosted", "reconcile", "--force-local", "--force-remote"},
	} {
		if _, _, err := executeCommand(t, "test", args...); err == nil || !strings.Contains(err.Error(), "mutually exclusive") {
			t.Fatalf("%v error = %v, want mutual exclusion error", args, err)
		}
	}
}

// reconcileRotatedRecipient is a second valid age X25519 recipient used to
// simulate key rotation in merge tests.
const reconcileRotatedRecipient = "age1neneutt8fuj4hsm8fwj6943g3c2hg790tlf2j5k4wz2vydcfhcvsslmu07"

func reconcileUser() User {
	return User{
		ID:           "user_1",
		Name:         "Test User",
		AgeRecipient: "age1lydx38xc73yjmwfvqfpd2peulfwftx7tv7x4lw6p2gh594h2wyrqx70a4q",
		CreatedAt:    "2026-01-01T00:00:00Z",
	}
}
