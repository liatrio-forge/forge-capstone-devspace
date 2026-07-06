package devspace

import (
	"path/filepath"
	"strings"
	"testing"

	"filippo.io/age"
)

func TestEffectiveRoleResolution(t *testing.T) {
	const localRecipient = "age-local"
	baseUser := User{ID: "user-local", Name: "Local", AgeRecipient: localRecipient, Status: "active", CreatedAt: "now"}
	baseProject := Project{ID: "project-api", Name: "api", Path: "apps/api", Type: ProjectTypeLocal, HydrateMode: HydrateManual}

	cases := []struct {
		name          string
		manifest      Manifest
		recipient     string
		wantRole      string
		wantWarning   string
		absentWarning string
		workspace     bool
	}{
		{
			name:      "most privileged direct and team grant wins",
			recipient: localRecipient,
			manifest: Manifest{
				Version:       ManifestVersion,
				WorkspaceRoot: "/tmp/workspace",
				Projects:      []Project{baseProject},
				Users:         []User{baseUser},
				Teams: []Team{{
					ID:        "team-platform",
					Name:      "Platform",
					Members:   []TeamMember{{UserID: baseUser.ID, Role: AccessRoleMaintainer, AddedAt: "now"}},
					CreatedAt: "now",
				}},
				Access: []ProjectAccess{
					{ProjectID: baseProject.ID, UserID: baseUser.ID, Role: AccessRoleViewer, AddedAt: "now"},
					{ProjectID: baseProject.ID, TeamID: "team-platform", Role: AccessRoleOwner, AddedAt: "now"},
				},
			},
			wantRole:    AccessRoleMaintainer,
			wantWarning: "direct and team grants disagree",
		},
		{
			name:      "team member role caps project team access",
			recipient: localRecipient,
			manifest: Manifest{
				Version:       ManifestVersion,
				WorkspaceRoot: "/tmp/workspace",
				Projects:      []Project{baseProject},
				Users:         []User{baseUser},
				Teams: []Team{{
					ID:        "team-platform",
					Name:      "Platform",
					Members:   []TeamMember{{UserID: baseUser.ID, Role: AccessRoleViewer, AddedAt: "now"}},
					CreatedAt: "now",
				}},
				Access: []ProjectAccess{
					{ProjectID: baseProject.ID, TeamID: "team-platform", Role: AccessRoleMaintainer, AddedAt: "now"},
				},
			},
			wantRole: AccessRoleViewer,
		},
		{
			name:      "revoked grants and team memberships are ignored",
			recipient: localRecipient,
			manifest: Manifest{
				Version:       ManifestVersion,
				WorkspaceRoot: "/tmp/workspace",
				Projects:      []Project{baseProject},
				Users:         []User{baseUser},
				Teams: []Team{{
					ID:        "team-platform",
					Name:      "Platform",
					Members:   []TeamMember{{UserID: baseUser.ID, Role: AccessRoleOwner, AddedAt: "now", RevokedAt: "later"}},
					CreatedAt: "now",
				}},
				Access: []ProjectAccess{
					{ProjectID: baseProject.ID, UserID: baseUser.ID, Role: AccessRoleMaintainer, AddedAt: "now", RevokedAt: "later"},
					{ProjectID: baseProject.ID, UserID: baseUser.ID, Role: AccessRoleDeveloper, AddedAt: "now"},
					{ProjectID: baseProject.ID, TeamID: "team-platform", Role: AccessRoleOwner, AddedAt: "now"},
				},
			},
			wantRole: AccessRoleDeveloper,
		},
		{
			name:      "revoked user is treated as unknown",
			recipient: localRecipient,
			manifest: Manifest{
				Version:       ManifestVersion,
				WorkspaceRoot: "/tmp/workspace",
				Projects:      []Project{baseProject},
				Users:         []User{{ID: baseUser.ID, Name: baseUser.Name, AgeRecipient: localRecipient, Status: "revoked", CreatedAt: "now"}},
				Access:        []ProjectAccess{{ProjectID: baseProject.ID, UserID: baseUser.ID, Role: AccessRoleOwner, AddedAt: "now"}},
			},
			wantWarning: "no active manifest user",
		},
		{
			name:      "unknown user continues with warning",
			recipient: "age-other",
			manifest: Manifest{
				Version:       ManifestVersion,
				WorkspaceRoot: "/tmp/workspace",
				Projects:      []Project{baseProject},
				Users:         []User{baseUser},
				Access:        []ProjectAccess{{ProjectID: baseProject.ID, UserID: baseUser.ID, Role: AccessRoleOwner, AddedAt: "now"}},
			},
			wantWarning: "no active manifest user",
		},
		{
			name:      "unknown role continues with warning",
			recipient: localRecipient,
			manifest: Manifest{
				Version:       ManifestVersion,
				WorkspaceRoot: "/tmp/workspace",
				Projects:      []Project{baseProject},
				Users:         []User{baseUser},
				Access:        []ProjectAccess{{ProjectID: baseProject.ID, UserID: baseUser.ID, Role: "admin", AddedAt: "now"}},
			},
			wantWarning: "unknown project access role",
		},
		{
			name:      "workspace grants on different projects do not disagree",
			recipient: localRecipient,
			manifest: Manifest{
				Version:       ManifestVersion,
				WorkspaceRoot: "/tmp/workspace",
				Projects: []Project{
					baseProject,
					{ID: "project-web", Name: "web", Path: "apps/web", Type: ProjectTypeLocal, HydrateMode: HydrateManual},
				},
				Users: []User{baseUser},
				Teams: []Team{{
					ID:        "team-platform",
					Name:      "Platform",
					Members:   []TeamMember{{UserID: baseUser.ID, Role: AccessRoleMaintainer, AddedAt: "now"}},
					CreatedAt: "now",
				}},
				Access: []ProjectAccess{
					{ProjectID: baseProject.ID, UserID: baseUser.ID, Role: AccessRoleViewer, AddedAt: "now"},
					{ProjectID: "project-web", TeamID: "team-platform", Role: AccessRoleMaintainer, AddedAt: "now"},
				},
			},
			wantRole:      AccessRoleMaintainer,
			absentWarning: "direct and team grants disagree",
			workspace:     true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var got effectiveRoleResult
			if tc.workspace {
				got = effectiveWorkspaceRole(tc.manifest, tc.recipient)
			} else {
				got = effectiveRole(tc.manifest, baseProject.ID, tc.recipient)
			}
			if got.Role != tc.wantRole {
				t.Fatalf("role = %q, want %q; warnings=%v", got.Role, tc.wantRole, got.Warnings)
			}
			if tc.wantWarning == "" {
				if tc.absentWarning != "" && strings.Contains(strings.Join(got.Warnings, "\n"), tc.absentWarning) {
					t.Fatalf("warnings = %v, did not want substring %q", got.Warnings, tc.absentWarning)
				}
				return
			}
			if !strings.Contains(strings.Join(got.Warnings, "\n"), tc.wantWarning) {
				t.Fatalf("warnings = %v, want substring %q", got.Warnings, tc.wantWarning)
			}
		})
	}
}

func TestWorkspacePushWithoutRoleMetadataEmitsNoAdvisory(t *testing.T) {
	commandWorkspaceWithoutRoleMetadata(t)
	remote := filepath.Join(t.TempDir(), "manifest.git")
	if _, err := CreateLocalManifestRemote(remote); err != nil {
		t.Fatal(err)
	}

	stdout, stderr, err := executeCommand(t, "test", "workspace", "push")
	if err != nil {
		t.Fatalf("workspace push error: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}
	assertNoAccessWarning(t, stderr)
}

func TestWorkspacePushEmitsAccessRoleAdvisoryWarning(t *testing.T) {
	commandWorkspaceWithProjectRole(t, AccessRoleDeveloper)
	remote := filepath.Join(t.TempDir(), "manifest.git")
	if _, err := CreateLocalManifestRemote(remote); err != nil {
		t.Fatal(err)
	}

	stdout, stderr, err := executeCommand(t, "test", "workspace", "push")
	if err != nil {
		t.Fatalf("workspace push error: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}
	assertAccessWarning(t, stderr, "devspace workspace push")
}

func TestHostedPushEmitsAccessRoleAdvisoryWarningBeforeTransportError(t *testing.T) {
	commandWorkspaceWithProjectRole(t, AccessRoleDeveloper)
	if _, err := SetHostedSync("http://127.0.0.1:1", "test-token", "team-a"); err != nil {
		t.Fatal(err)
	}

	stdout, stderr, err := executeCommand(t, "test", "hosted", "push")
	if err == nil {
		t.Fatalf("hosted push unexpectedly succeeded\nstdout:\n%s\nstderr:\n%s", stdout, stderr)
	}
	assertAccessWarning(t, stderr, "devspace hosted push")
}

func TestProjectRemoveWithoutRoleMetadataEmitsNoAdvisory(t *testing.T) {
	_, project := commandWorkspaceWithoutRoleMetadata(t)

	stdout, stderr, err := executeCommand(t, "test", "project", "remove", project.Name)
	if err != nil {
		t.Fatalf("project remove error: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}
	assertNoAccessWarning(t, stderr)
}

func TestProjectRemoveEmitsAccessRoleAdvisoryWarning(t *testing.T) {
	_, project := commandWorkspaceWithProjectRole(t, AccessRoleDeveloper)

	stdout, stderr, err := executeCommand(t, "test", "project", "remove", project.Name)
	if err != nil {
		t.Fatalf("project remove error: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}
	assertAccessWarning(t, stderr, "devspace project remove")
}

func TestEnvRecipientChangesEmitAccessRoleAdvisoryWarnings(t *testing.T) {
	t.Run("invite", func(t *testing.T) {
		_, project := commandWorkspaceWithProjectRole(t, AccessRoleDeveloper)
		teammate, err := age.GenerateX25519Identity()
		if err != nil {
			t.Fatal(err)
		}

		stdout, stderr, err := executeCommand(t, "test", "env", "recipient", "invite", project.Name, "teammate", teammate.Recipient().String())
		if err != nil {
			t.Fatalf("env recipient invite error: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
		}
		assertAccessWarning(t, stderr, "devspace env recipient invite")
	})

	t.Run("revoke", func(t *testing.T) {
		workspace, project := commandWorkspaceWithProjectRole(t, AccessRoleDeveloper)
		teammate, err := age.GenerateX25519Identity()
		if err != nil {
			t.Fatal(err)
		}
		recipient, err := EnvInvite(project.Name, "dev", "teammate", teammate.Recipient().String(), "")
		if err != nil {
			t.Fatal(err)
		}
		setLocalProjectRole(t, workspace, project.ID, AccessRoleDeveloper)

		stdout, stderr, err := executeCommand(t, "test", "env", "recipient", "revoke", project.Name, recipient.ID)
		if err != nil {
			t.Fatalf("env recipient revoke error: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
		}
		assertAccessWarning(t, stderr, "devspace env recipient revoke")
	})

	t.Run("rotate", func(t *testing.T) {
		workspace, project := commandWorkspaceWithProjectRole(t, AccessRoleDeveloper)
		teammate, err := age.GenerateX25519Identity()
		if err != nil {
			t.Fatal(err)
		}
		if _, err := EnvInvite(project.Name, "dev", "teammate", teammate.Recipient().String(), ""); err != nil {
			t.Fatal(err)
		}
		setLocalProjectRole(t, workspace, project.ID, AccessRoleDeveloper)

		stdout, stderr, err := executeCommand(t, "test", "env", "recipient", "rotate", project.Name)
		if err != nil {
			t.Fatalf("env recipient rotate error: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
		}
		assertAccessWarning(t, stderr, "devspace env recipient rotate")
	})
}

func commandWorkspaceWithoutRoleMetadata(t *testing.T) (string, Project) {
	t.Helper()
	workspace := initCommandWorkspace(t)
	project := hardeningProject("apps/api", ProjectTypeLocal, "")
	m, err := LoadManifest(workspace)
	if err != nil {
		t.Fatal(err)
	}
	m.Projects = []Project{project}
	m.Users = nil
	m.Teams = nil
	m.Access = nil
	if err := SaveManifest(workspace, m); err != nil {
		t.Fatal(err)
	}
	return workspace, project
}

func commandWorkspaceWithProjectRole(t *testing.T, role string) (string, Project) {
	t.Helper()
	workspace := initCommandWorkspace(t)
	project := hardeningProject("apps/api", ProjectTypeLocal, "")
	m, err := LoadManifest(workspace)
	if err != nil {
		t.Fatal(err)
	}
	m.Projects = []Project{project}
	if err := SaveManifest(workspace, m); err != nil {
		t.Fatal(err)
	}
	setLocalProjectRole(t, workspace, project.ID, role)
	return workspace, project
}

func setLocalProjectRole(t *testing.T, workspace, projectID, role string) {
	t.Helper()
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	local, err := localSecretRecipient(cfg, nowRFC3339())
	if err != nil {
		t.Fatal(err)
	}
	m, err := LoadManifest(workspace)
	if err != nil {
		t.Fatal(err)
	}
	m = upsertManifestUser(m, local, nowRFC3339())
	m = upsertManifestProjectAccess(m, ProjectAccess{
		ProjectID: projectID,
		UserID:    local.ID,
		Role:      role,
		AddedAt:   nowRFC3339(),
	})
	if err := SaveManifest(workspace, m); err != nil {
		t.Fatal(err)
	}
}

func assertNoAccessWarning(t *testing.T, stderr string) {
	t.Helper()
	if stderr != "" {
		t.Fatalf("stderr = %q, want no advisories", stderr)
	}
}

func assertAccessWarning(t *testing.T, stderr, surface string) {
	t.Helper()
	for _, want := range []string{
		"Access role advisory",
		surface,
		"recommended for owner or maintainer",
		"local effective role is developer",
		"Command will continue",
	} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("stderr missing %q:\n%s", want, stderr)
		}
	}
}
