package devspace

import (
	"reflect"
	"strings"
	"testing"
)

func TestMergeManifests(t *testing.T) {
	user := User{
		ID:           "user_1",
		Name:         "Test User",
		AgeRecipient: "age1lydx38xc73yjmwfvqfpd2peulfwftx7tv7x4lw6p2gh594h2wyrqx70a4q",
		CreatedAt:    "2026-01-01T00:00:00Z",
	}
	baseProject := testMergeProject("project_base", "app")
	otherProject := testMergeProject("project_other", "lib")
	baseAccess := ProjectAccess{ProjectID: baseProject.ID, UserID: user.ID, Role: AccessRoleDeveloper, AddedAt: "2026-01-01T00:00:00Z"}

	cases := []struct {
		name          string
		base          Manifest
		ours          Manifest
		theirs        Manifest
		wantProjects  []Project
		wantAccess    []ProjectAccess
		wantConflicts int
		wantConflict  []MergeConflict
		wantErr       string
	}{
		{
			name: "malformed-ours-returns-error",
			base: testMergeManifest(user, nil, nil),
			ours: testMergeManifest(user, []Project{
				baseProject,
				testMergeProject("project_duplicate", baseProject.Path),
			}, nil),
			theirs:  testMergeManifest(user, nil, nil),
			wantErr: "ours manifest failed validation",
		},
		{
			name: "both-added-disjoint",
			base: testMergeManifest(user, nil, nil),
			ours: testMergeManifest(user, []Project{
				baseProject,
			}, []ProjectAccess{
				baseAccess,
			}),
			theirs: testMergeManifest(user, []Project{
				otherProject,
			}, []ProjectAccess{
				{ProjectID: otherProject.ID, UserID: user.ID, Role: AccessRoleViewer, AddedAt: "2026-01-02T00:00:00Z"},
			}),
			wantProjects: []Project{
				baseProject,
				otherProject,
			},
			wantAccess: []ProjectAccess{
				baseAccess,
				{ProjectID: otherProject.ID, UserID: user.ID, Role: AccessRoleViewer, AddedAt: "2026-01-02T00:00:00Z"},
			},
		},
		{
			name: "theirs-modified",
			base: testMergeManifest(user, []Project{
				baseProject,
			}, []ProjectAccess{
				baseAccess,
			}),
			ours: testMergeManifest(user, []Project{
				baseProject,
			}, []ProjectAccess{
				baseAccess,
			}),
			theirs: testMergeManifest(user, []Project{
				withMergeRemote(baseProject, "git@example.com:team/app.git"),
			}, []ProjectAccess{
				{ProjectID: baseProject.ID, UserID: user.ID, Role: AccessRoleMaintainer, AddedAt: baseAccess.AddedAt},
			}),
			wantProjects: []Project{
				withMergeRemote(baseProject, "git@example.com:team/app.git"),
			},
			wantAccess: []ProjectAccess{
				{ProjectID: baseProject.ID, UserID: user.ID, Role: AccessRoleMaintainer, AddedAt: baseAccess.AddedAt},
			},
		},
		{
			name: "ours-modified",
			base: testMergeManifest(user, []Project{
				baseProject,
			}, []ProjectAccess{
				baseAccess,
			}),
			ours: testMergeManifest(user, []Project{
				withMergeIgnore(baseProject, []string{"tmp"}),
			}, []ProjectAccess{
				{ProjectID: baseProject.ID, UserID: user.ID, Role: AccessRoleOwner, AddedAt: baseAccess.AddedAt},
			}),
			theirs: testMergeManifest(user, []Project{
				baseProject,
			}, []ProjectAccess{
				baseAccess,
			}),
			wantProjects: []Project{
				withMergeIgnore(baseProject, []string{"tmp"}),
			},
			wantAccess: []ProjectAccess{
				{ProjectID: baseProject.ID, UserID: user.ID, Role: AccessRoleOwner, AddedAt: baseAccess.AddedAt},
			},
		},
		{
			name: "both-modified-conflict",
			base: testMergeManifest(user, []Project{
				baseProject,
			}, []ProjectAccess{
				baseAccess,
			}),
			ours: testMergeManifest(user, []Project{
				withMergeName(baseProject, "ours-app"),
			}, []ProjectAccess{
				{ProjectID: baseProject.ID, UserID: user.ID, Role: AccessRoleOwner, AddedAt: baseAccess.AddedAt},
			}),
			theirs: testMergeManifest(user, []Project{
				withMergeName(baseProject, "theirs-app"),
			}, []ProjectAccess{
				{ProjectID: baseProject.ID, UserID: user.ID, Role: AccessRoleViewer, AddedAt: baseAccess.AddedAt},
			}),
			wantProjects: []Project{
				withMergeName(baseProject, "ours-app"),
			},
			wantAccess: []ProjectAccess{
				{ProjectID: baseProject.ID, UserID: user.ID, Role: AccessRoleOwner, AddedAt: baseAccess.AddedAt},
			},
			wantConflicts: 2,
			wantConflict: []MergeConflict{
				{Entity: "project", Key: baseProject.Path, Field: "*"},
				{Entity: "access", Key: baseProject.ID + "\x00" + user.ID + "\x00", Field: "*"},
			},
		},
		{
			name: "delete-vs-modify-conflicts",
			base: testMergeManifest(user, []Project{
				baseProject,
			}, []ProjectAccess{
				baseAccess,
			}),
			ours: testMergeManifest(user, nil, nil),
			theirs: testMergeManifest(user, []Project{
				withMergeRemote(baseProject, "git@example.com:team/app.git"),
			}, []ProjectAccess{
				{ProjectID: baseProject.ID, UserID: user.ID, Role: AccessRoleMaintainer, AddedAt: baseAccess.AddedAt},
			}),
			wantProjects: []Project{
				withMergeRemote(baseProject, "git@example.com:team/app.git"),
			},
			wantAccess: []ProjectAccess{
				{ProjectID: baseProject.ID, UserID: user.ID, Role: AccessRoleMaintainer, AddedAt: baseAccess.AddedAt},
			},
			wantConflicts: 2,
			wantConflict: []MergeConflict{
				{Entity: "project", Key: baseProject.Path, Field: "*"},
				{Entity: "access", Key: baseProject.ID + "\x00" + user.ID + "\x00", Field: "*"},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, conflicts, err := mergeManifests(tc.base, tc.ours, tc.theirs)
			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tc.wantErr)
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("expected error containing %q, got %v", tc.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if len(conflicts) != tc.wantConflicts {
				t.Fatalf("conflict count = %d, want %d: %+v", len(conflicts), tc.wantConflicts, conflicts)
			}
			for i, want := range tc.wantConflict {
				if i >= len(conflicts) {
					t.Fatalf("missing conflict %d: want %+v, got %+v", i, want, conflicts)
				}
				gotConflict := conflicts[i]
				if gotConflict.Entity != want.Entity || gotConflict.Key != want.Key || gotConflict.Field != want.Field {
					t.Fatalf("conflict %d = %+v, want entity/key/field %+v", i, gotConflict, want)
				}
				if gotConflict.Ours == "" || gotConflict.Theirs == "" {
					t.Fatalf("conflict %d is missing ours/theirs details: %+v", i, gotConflict)
				}
			}
			if !reflect.DeepEqual(got.Projects, tc.wantProjects) {
				t.Fatalf("projects = %+v, want %+v", got.Projects, tc.wantProjects)
			}
			if !reflect.DeepEqual(got.Access, tc.wantAccess) {
				t.Fatalf("access = %+v, want %+v", got.Access, tc.wantAccess)
			}
			if tc.wantConflicts == 0 {
				if err := ValidateManifest(got); err != nil {
					t.Fatalf("merged manifest failed validation: %v", err)
				}
			}
		})
	}
}

func testMergeManifest(user User, projects []Project, access []ProjectAccess) Manifest {
	return Manifest{
		Version:       ManifestVersion,
		WorkspaceRoot: "/tmp/workspace",
		Projects:      projects,
		Users:         []User{user},
		Access:        access,
	}
}

func testMergeProject(id, path string) Project {
	return Project{
		ID:          id,
		Name:        path,
		Path:        path,
		Type:        ProjectTypeLocal,
		HydrateMode: HydrateManual,
	}
}

func withMergeName(project Project, name string) Project {
	project.Name = name
	return project
}

func withMergeRemote(project Project, remote string) Project {
	project.Type = ProjectTypeGit
	project.Remote = remote
	project.HydrateMode = HydrateOnDemand
	return project
}

func withMergeIgnore(project Project, ignore []string) Project {
	project.Ignore = ignore
	return project
}
