package devspace

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"
)

var updateUIFixtures = flag.Bool("update-ui-fixtures", false, "rewrite tui/test/fixtures")

func TestUIProtocolFixtures(t *testing.T) {
	fixtures := map[string]any{
		"hello.json": uiHello{
			Protocol:      uiProtocolVersion,
			Version:       "9.9.9",
			WorkspaceRoot: "/ws",
			MachineID:     "m-1",
			MachineName:   "mach",
			SyncMode:      "git",
			Watch:         true,
		},
		"snapshot.json": uiSnapshot{
			Rows: []uiProjectRow{
				{Ref: "apps/api", Name: "api", Path: "apps/api", Type: "git", Status: dashboardStatusHydrated, Dirty: true, Branch: "main", Env: true},
				{Ref: "tools/cli", Name: "cli", Path: "tools/cli", Type: "local", Status: dashboardStatusPlaceholder, Dirty: false, Env: false},
			},
			Summary: uiScanSummary{FoundProjects: 2, GitRepos: 1, UntrackedFolders: 1, LocalOnlyProjects: 1, ProjectsWithEnv: 1},
			Plan: &Plan{
				Version:       1,
				WorkspaceRoot: "/ws",
				ManifestHash:  "abc123",
				GeneratedAt:   "2026-07-07T12:00:00Z",
				Actions: []PlanAction{
					{Safety: "safe", Kind: "hydrate", Path: "apps/api", Reason: "missing checkout", Project: "api"},
					{Safety: "skipped", Kind: "delete", Path: "tmp/old", Reason: "dirty worktree", Project: "old"},
				},
				Warnings: []string{"untracked folder ignored"},
			},
			Project: &Project{
				ID:            "api",
				Name:          "api",
				Path:          "apps/api",
				Type:          "git",
				Remote:        "git@example.com:org/api.git",
				DefaultBranch: "main",
				HydrateMode:   "clone",
				EnvProfiles:   []string{"dev", "prod"},
			},
		},
		"sync-status.json": uiSyncStatus{
			Configured:         true,
			LastSyncAt:         "2026-07-07T12:00:00Z",
			LocalDiffers:       true,
			DiffAdded:          1,
			DiffRemoved:        2,
			DiffChanged:        3,
			ReconcileSaved:     true,
			ConflictCount:      4,
			GitDiffUnavailable: "git unavailable",
			UnavailableReason:  "network",
		},
		"event-watch-refresh.json": uiServerEvent{
			Method: "event",
			Params: map[string]any{
				"type": "watch-refresh",
				"rows": []uiProjectRow{
					{Ref: "apps/api", Name: "api", Path: "apps/api", Type: "git", Status: dashboardStatusHydrated, Dirty: true, Branch: "main", Env: true},
				},
				"summary": uiScanSummary{FoundProjects: 1, GitRepos: 1, UntrackedFolders: 0, LocalOnlyProjects: 0, ProjectsWithEnv: 1},
				"refresh": uiWatchRefresh{FullScan: true, RefreshStartedAt: "2026-07-07T12:00:00Z", WatchedDirCount: 5, SyncChanged: true, SyncMode: "git"},
			},
		},
		"event-watch-error.json": uiServerEvent{
			Method: "event",
			Params: map[string]any{
				"type":    "watch-error",
				"message": "boom",
			},
		},
		"response-error.json": uiServerResponse{
			ID:    7,
			Error: &uiServerError{Message: "nope"},
		},
	}

	fixturesDir := filepath.Join("..", "..", "tui", "test", "fixtures")
	if *updateUIFixtures {
		if err := os.MkdirAll(fixturesDir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for name, value := range fixtures {
		got, err := json.MarshalIndent(value, "", "  ")
		if err != nil {
			t.Fatalf("marshal %s: %v", name, err)
		}
		got = append(got, '\n')
		path := filepath.Join(fixturesDir, name)
		if *updateUIFixtures {
			if err := os.WriteFile(path, got, 0o644); err != nil {
				t.Fatalf("write %s: %v", name, err)
			}
			continue
		}
		want, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		if !bytes.Equal(want, got) {
			t.Fatalf("fixture %s is stale; run: go test ./internal/devspace -run TestUIProtocolFixtures -update-ui-fixtures", name)
		}
	}
}
