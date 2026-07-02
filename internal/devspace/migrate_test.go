package devspace

import (
	"os"
	"path/filepath"
	"testing"
)

// withFakeHome points os.UserHomeDir() at a fresh temp directory and clears
// any DEVSPACE_HOME/DEV_DROP_HOME override so migrateLegacyHome exercises
// its default (non-override) resolution path.
func withFakeHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv(envHome, "")
	t.Setenv(envHomeLegacy, "")
	return home
}

func TestMigrateLegacyHomeFreshInstallNoOp(t *testing.T) {
	home := withFakeHome(t)

	if err := migrateLegacyHome(); err != nil {
		t.Fatalf("migrateLegacyHome returned error: %v", err)
	}

	newPath := filepath.Join(home, appDirName)
	if exists(newPath) {
		t.Fatal("fresh install should not create the app home directory")
	}
	resolved, err := appHome()
	if err != nil {
		t.Fatal(err)
	}
	if resolved != newPath {
		t.Fatalf("appHome() = %q, want %q", resolved, newPath)
	}
}

func TestMigrateLegacyHomeRepairsUnrewrittenConfig(t *testing.T) {
	home := withFakeHome(t)
	newPath := filepath.Join(home, appDirName)
	oldPath := filepath.Join(home, legacyAppDirName)
	oldIdentity := filepath.Join(oldPath, "identity.txt")
	// Simulate a prior run that renamed the home to .devspace but failed
	// before rewriting config.json: the new dir exists, the old dir is gone,
	// but the stored identity path still points inside the old directory.
	if err := SaveConfig(Config{MachineID: "machine_1", AgeIdentityPath: oldIdentity}); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}
	if !exists(newPath) {
		t.Fatal("precondition: new home should exist after SaveConfig")
	}

	if err := migrateLegacyHome(); err != nil {
		t.Fatalf("migrateLegacyHome returned error: %v", err)
	}

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	want := filepath.Join(newPath, "identity.txt")
	if cfg.AgeIdentityPath != want {
		t.Fatalf("AgeIdentityPath = %q, want %q (config should self-heal)", cfg.AgeIdentityPath, want)
	}
}

func TestMigrateLegacyHomeMovesLegacyDirectory(t *testing.T) {
	home := withFakeHome(t)
	oldPath := filepath.Join(home, legacyAppDirName)
	writeMigrateFile(t, filepath.Join(oldPath, "config.json"), `{"machineId":"machine_1"}`, 0o600)
	writeMigrateFile(t, filepath.Join(oldPath, "identity.txt"), "# identity\nAGE-SECRET-KEY-1\n", 0o600)

	if err := migrateLegacyHome(); err != nil {
		t.Fatalf("migrateLegacyHome returned error: %v", err)
	}

	newPath := filepath.Join(home, appDirName)
	if !exists(newPath) {
		t.Fatal("new home directory was not created")
	}
	if exists(oldPath) {
		t.Fatal("legacy home directory should be gone after migration")
	}
	data, err := os.ReadFile(filepath.Join(newPath, "identity.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "# identity\nAGE-SECRET-KEY-1\n" {
		t.Fatalf("identity contents changed: %q", data)
	}
}

func TestMigrateLegacyHomeDoesNotClobberExistingNewHome(t *testing.T) {
	home := withFakeHome(t)
	oldPath := filepath.Join(home, legacyAppDirName)
	newPath := filepath.Join(home, appDirName)
	writeMigrateFile(t, filepath.Join(oldPath, "marker.txt"), "old", 0o600)
	writeMigrateFile(t, filepath.Join(newPath, "marker.txt"), "new", 0o600)

	if err := migrateLegacyHome(); err != nil {
		t.Fatalf("migrateLegacyHome returned error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(newPath, "marker.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "new" {
		t.Fatal("existing new home directory was clobbered by migration")
	}
	if !exists(oldPath) {
		t.Fatal("legacy home directory should be left alone when the new home already exists")
	}
}

func TestMigrateLegacyHomeSkipsWhenEnvOverrideSet(t *testing.T) {
	home := withFakeHome(t)
	oldPath := filepath.Join(home, legacyAppDirName)
	writeMigrateFile(t, filepath.Join(oldPath, "marker.txt"), "old", 0o600)

	t.Setenv(envHome, t.TempDir())

	if err := migrateLegacyHome(); err != nil {
		t.Fatalf("migrateLegacyHome returned error: %v", err)
	}
	if !exists(oldPath) {
		t.Fatal("migration should not run when DEVSPACE_HOME is set")
	}
	newPath := filepath.Join(home, appDirName)
	if exists(newPath) {
		t.Fatal("migration should not create ~/.devspace when an env override is set")
	}
}

func TestMigrateLegacyHomeIsIdempotent(t *testing.T) {
	home := withFakeHome(t)
	oldPath := filepath.Join(home, legacyAppDirName)
	writeMigrateFile(t, filepath.Join(oldPath, "marker.txt"), "old", 0o600)

	if err := migrateLegacyHome(); err != nil {
		t.Fatalf("first migrateLegacyHome returned error: %v", err)
	}
	if err := migrateLegacyHome(); err != nil {
		t.Fatalf("second migrateLegacyHome returned error: %v", err)
	}

	newPath := filepath.Join(home, appDirName)
	if exists(oldPath) {
		t.Fatal("legacy home directory reappeared after a second migration run")
	}
	data, err := os.ReadFile(filepath.Join(newPath, "marker.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "old" {
		t.Fatal("second migration run altered already-migrated contents")
	}
}

func TestMigrateLegacyHomeRewritesAgeIdentityPath(t *testing.T) {
	home := withFakeHome(t)
	oldPath := filepath.Join(home, legacyAppDirName)
	newPath := filepath.Join(home, appDirName)
	cfg := Config{
		MachineID:       "machine_1",
		AgeIdentityPath: filepath.Join(oldPath, "identity.txt"),
		CreatedAt:       "2025-01-01T00:00:00Z",
		UpdatedAt:       "2025-01-01T00:00:00Z",
	}
	writeMigrateJSON(t, filepath.Join(oldPath, "config.json"), cfg)
	writeMigrateFile(t, filepath.Join(oldPath, "identity.txt"), "# identity\nAGE-SECRET-KEY-1\n", 0o600)

	if err := migrateLegacyHome(); err != nil {
		t.Fatalf("migrateLegacyHome returned error: %v", err)
	}

	loaded, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(newPath, "identity.txt")
	if loaded.AgeIdentityPath != want {
		t.Fatalf("AgeIdentityPath = %q, want %q", loaded.AgeIdentityPath, want)
	}
}

func TestMigrateLegacyHomePreservesFilePermissions(t *testing.T) {
	home := withFakeHome(t)
	oldPath := filepath.Join(home, legacyAppDirName)
	writeMigrateFile(t, filepath.Join(oldPath, "identity.txt"), "# identity\nAGE-SECRET-KEY-1\n", 0o600)
	writeMigrateFile(t, filepath.Join(oldPath, "secrets", "proj1", "dev.age"), "ciphertext", 0o600)

	if err := migrateLegacyHome(); err != nil {
		t.Fatalf("migrateLegacyHome returned error: %v", err)
	}

	newPath := filepath.Join(home, appDirName)
	for _, rel := range []string{"identity.txt", filepath.Join("secrets", "proj1", "dev.age")} {
		info, err := os.Stat(filepath.Join(newPath, rel))
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0o600 {
			t.Fatalf("%s permissions = %o, want 0600", rel, info.Mode().Perm())
		}
	}
}

func TestWorkspaceDevdropReadsBothLegacyAndCurrent(t *testing.T) {
	legacyWorkspace := t.TempDir()
	if err := os.MkdirAll(filepath.Join(legacyWorkspace, ".devdrop"), 0o700); err != nil {
		t.Fatal(err)
	}
	if got, want := workspaceDevdrop(legacyWorkspace), filepath.Join(legacyWorkspace, ".devdrop"); got != want {
		t.Fatalf("workspaceDevdrop(legacy-only workspace) = %q, want %q", got, want)
	}

	currentWorkspace := t.TempDir()
	if err := os.MkdirAll(filepath.Join(currentWorkspace, ".devspace"), 0o700); err != nil {
		t.Fatal(err)
	}
	if got, want := workspaceDevdrop(currentWorkspace), filepath.Join(currentWorkspace, ".devspace"); got != want {
		t.Fatalf("workspaceDevdrop(current workspace) = %q, want %q", got, want)
	}
}

func writeMigrateFile(t *testing.T, path, content string, perm os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), perm); err != nil {
		t.Fatal(err)
	}
}

func writeMigrateJSON(t *testing.T, path string, value any) {
	t.Helper()
	if err := writeJSON(path, value, 0o600); err != nil {
		t.Fatal(err)
	}
}
