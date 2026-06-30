package devdrop

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const syncedManifestName = "manifest.json"

func SetManifestRemote(remote string) (Config, error) {
	if strings.TrimSpace(remote) == "" {
		return Config{}, fmt.Errorf("manifest remote is required")
	}
	cfg, err := LoadConfig()
	if err != nil {
		return Config{}, err
	}
	cfg.ManifestRemote = remote
	if cfg.ManifestRepoPath == "" {
		cfg.ManifestRepoPath, err = defaultManifestRepoPath()
		if err != nil {
			return Config{}, err
		}
	}
	cfg.UpdatedAt = nowRFC3339()
	if err := SaveConfig(cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func GetManifestRemote() (Config, error) {
	cfg, err := LoadConfig()
	if err != nil {
		return Config{}, err
	}
	if cfg.ManifestRemote == "" {
		return Config{}, fmt.Errorf("no manifest remote configured; run `devspace workspace remote set <url-or-path>`")
	}
	return cfg, nil
}

func PushWorkspaceManifest() (bool, error) {
	cfg, err := syncConfig()
	if err != nil {
		return false, err
	}
	local, err := LoadManifest(cfg.WorkspaceRoot)
	if err != nil {
		return false, err
	}
	normalized := manifestForSync(local)
	if err := ValidateManifest(normalized); err != nil {
		return false, err
	}
	repo, err := ensureManifestRepo(cfg)
	if err != nil {
		return false, err
	}
	if err := ensureCleanManifestRepo(repo); err != nil {
		return false, err
	}
	if err := fetchManifestRepo(repo); err != nil {
		return false, err
	}
	if err := ensureManifestRepoNotBehind(repo); err != nil {
		return false, err
	}
	changed, err := writeSyncedManifest(repo, normalized)
	if err != nil {
		return false, err
	}
	if !changed {
		return false, nil
	}
	if err := commitManifestRepo(repo); err != nil {
		return false, err
	}
	if err := pushManifestRepo(repo); err != nil {
		return false, err
	}
	return true, nil
}

func PullWorkspaceManifest() (bool, error) {
	cfg, err := syncConfig()
	if err != nil {
		return false, err
	}
	repo, err := ensureManifestRepo(cfg)
	if err != nil {
		return false, err
	}
	if err := ensureCleanManifestRepo(repo); err != nil {
		return false, err
	}
	previousRemote, hasPreviousRemote, err := loadSyncedManifestIfExists(repo)
	if err != nil {
		return false, err
	}
	if err := pullManifestRepo(repo); err != nil {
		return false, err
	}
	remote, err := loadSyncedManifest(repo)
	if err != nil {
		return false, err
	}
	localized := localizeSyncedManifest(remote, cfg)
	if err := ValidateManifest(localized); err != nil {
		return false, fmt.Errorf("pulled manifest failed validation: %w", err)
	}
	current, err := LoadManifest(cfg.WorkspaceRoot)
	if err != nil && !missing(err) {
		return false, err
	}
	if err == nil && localHasUnpushedManifestChanges(current, previousRemote, hasPreviousRemote, remote) {
		return false, fmt.Errorf("local manifest differs from remote manifest; push or reconcile local changes before pulling")
	}
	before, err := os.ReadFile(manifestPath(cfg.WorkspaceRoot))
	if err != nil && !missing(err) {
		return false, err
	}
	if err := SaveManifest(cfg.WorkspaceRoot, localized); err != nil {
		return false, err
	}
	after, err := os.ReadFile(manifestPath(cfg.WorkspaceRoot))
	if err != nil {
		return false, err
	}
	return !bytes.Equal(before, after), nil
}

func syncConfig() (Config, error) {
	cfg, err := GetManifestRemote()
	if err != nil {
		return Config{}, err
	}
	if cfg.ManifestRepoPath == "" {
		cfg.ManifestRepoPath, err = defaultManifestRepoPath()
		if err != nil {
			return Config{}, err
		}
		if saveErr := SaveConfig(cfg); saveErr != nil {
			return Config{}, saveErr
		}
	}
	return cfg, nil
}

func defaultManifestRepoPath() (string, error) {
	home, err := appHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "remotes", "default"), nil
}

func ensureManifestRepo(cfg Config) (string, error) {
	if err := ensureGitAvailable(); err != nil {
		return "", err
	}
	repo, err := expandPath(cfg.ManifestRepoPath)
	if err != nil {
		return "", err
	}
	if gitInfo(repo).IsRepo {
		if err := ensureManifestRepoRemote(repo, cfg.ManifestRemote); err != nil {
			return "", err
		}
		return repo, nil
	}
	if exists(repo) && !isEmptyDir(repo) {
		return "", fmt.Errorf("manifest repo path is non-empty and is not a Git repository: %s", repo)
	}
	if err := os.MkdirAll(filepath.Dir(repo), 0o700); err != nil {
		return "", err
	}
	if err := cloneRepo(cfg.ManifestRemote, repo); err != nil {
		return "", fmt.Errorf("remote clone failed: %w", err)
	}
	return repo, nil
}

func ensureManifestRepoRemote(repo, remote string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	current := mustGit(ctx, repo, "remote", "get-url", "origin")
	if current == "" {
		if _, err := runGit(ctx, repo, "remote", "add", "origin", remote); err != nil {
			return fmt.Errorf("manifest repo origin setup failed: %w", err)
		}
		return nil
	}
	if current != remote {
		return fmt.Errorf("manifest repo origin is %s, but configured remote is %s; remove %s or reconcile the cache", current, remote, repo)
	}
	return nil
}

func ensureCleanManifestRepo(repo string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	status, err := runGit(ctx, repo, "status", "--porcelain")
	if err != nil {
		return fmt.Errorf("manifest repo status failed: %w", err)
	}
	if status != "" {
		return fmt.Errorf("manifest repo has uncommitted changes; reconcile %s before syncing", repo)
	}
	return nil
}

func fetchManifestRepo(repo string) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	if _, err := runGit(ctx, repo, "fetch", "origin"); err != nil {
		return fmt.Errorf("remote fetch failed: %w", err)
	}
	return nil
}

func pullManifestRepo(repo string) error {
	if err := fetchManifestRepo(repo); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	if !manifestRepoHasCommit(ctx, repo) {
		return nil
	}
	if upstreamRef(ctx, repo) == "" {
		return nil
	}
	if _, err := runGit(ctx, repo, "pull", "--ff-only"); err != nil {
		return fmt.Errorf("remote branch diverged or cannot be fast-forwarded; pull/reconcile manually: %w", err)
	}
	if err := ensureManifestRepoNoUnpushedCommits(ctx, repo); err != nil {
		return err
	}
	return nil
}

func ensureManifestRepoNoUnpushedCommits(ctx context.Context, repo string) error {
	if !manifestRepoHasCommit(ctx, repo) || upstreamRef(ctx, repo) == "" {
		return nil
	}
	ahead, _, err := aheadBehind(ctx, repo)
	if err != nil {
		return err
	}
	if ahead > 0 {
		return fmt.Errorf("manifest repo has unpushed commits; push or reconcile %s before pulling", repo)
	}
	return nil
}

func ensureManifestRepoNotBehind(repo string) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	if !manifestRepoHasCommit(ctx, repo) || upstreamRef(ctx, repo) == "" {
		return nil
	}
	ahead, behind, err := aheadBehind(ctx, repo)
	if err != nil {
		return err
	}
	if behind > 0 {
		if ahead > 0 {
			return fmt.Errorf("remote branch diverged; run `devspace workspace pull` and reconcile before pushing")
		}
		return fmt.Errorf("remote manifest is newer; run `devspace workspace pull` before pushing")
	}
	return nil
}

func manifestRepoHasCommit(ctx context.Context, repo string) bool {
	_, err := runGit(ctx, repo, "rev-parse", "--verify", "HEAD")
	return err == nil
}

func upstreamRef(ctx context.Context, repo string) string {
	out, err := runGit(ctx, repo, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}")
	if err != nil {
		return ""
	}
	return out
}

func aheadBehind(ctx context.Context, repo string) (int, int, error) {
	out, err := runGit(ctx, repo, "rev-list", "--left-right", "--count", "HEAD...@{u}")
	if err != nil {
		return 0, 0, fmt.Errorf("remote branch comparison failed: %w", err)
	}
	fields := strings.Fields(out)
	if len(fields) != 2 {
		return 0, 0, fmt.Errorf("remote branch comparison returned unexpected output: %q", out)
	}
	ahead, err := strconv.Atoi(fields[0])
	if err != nil {
		return 0, 0, err
	}
	behind, err := strconv.Atoi(fields[1])
	if err != nil {
		return 0, 0, err
	}
	return ahead, behind, nil
}

func writeSyncedManifest(repo string, m Manifest) (bool, error) {
	data, err := manifestBytes(m)
	if err != nil {
		return false, err
	}
	path := filepath.Join(repo, syncedManifestName)
	current, err := os.ReadFile(path)
	if err == nil && bytes.Equal(current, data) {
		return false, nil
	}
	if err != nil && !missing(err) {
		return false, err
	}
	if err := atomicWriteFile(path, data, 0o600, false); err != nil {
		return false, err
	}
	return true, nil
}

func manifestBytes(m Manifest) ([]byte, error) {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func commitManifestRepo(repo string) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	if err := ensureManifestCommitIdentity(ctx, repo); err != nil {
		return err
	}
	if _, err := runGit(ctx, repo, "add", syncedManifestName); err != nil {
		return fmt.Errorf("manifest repo add failed: %w", err)
	}
	if _, err := runGit(ctx, repo, "-c", "commit.gpgsign=false", "commit", "-m", "Update workspace manifest"); err != nil {
		return fmt.Errorf("manifest repo commit failed: %w", err)
	}
	return nil
}

func ensureManifestCommitIdentity(ctx context.Context, repo string) error {
	if mustGit(ctx, repo, "config", "--get", "user.email") == "" {
		if _, err := runGit(ctx, repo, "config", "user.email", "devspace@example.invalid"); err != nil {
			return fmt.Errorf("manifest repo user.email config failed: %w", err)
		}
	}
	if mustGit(ctx, repo, "config", "--get", "user.name") == "" {
		if _, err := runGit(ctx, repo, "config", "user.name", "DevSpace"); err != nil {
			return fmt.Errorf("manifest repo user.name config failed: %w", err)
		}
	}
	return nil
}

func pushManifestRepo(repo string) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	if _, err := runGit(ctx, repo, "push", "-u", "origin", "HEAD"); err != nil {
		return fmt.Errorf("remote push failed; pull/reconcile first if the remote changed: %w", err)
	}
	return nil
}

func loadSyncedManifest(repo string) (Manifest, error) {
	var m Manifest
	path := filepath.Join(repo, syncedManifestName)
	if err := readJSON(path, &m); err != nil {
		return m, err
	}
	if err := ValidateManifest(m); err != nil {
		return m, err
	}
	return m, nil
}

func loadSyncedManifestIfExists(repo string) (Manifest, bool, error) {
	path := filepath.Join(repo, syncedManifestName)
	if !exists(path) {
		return Manifest{}, false, nil
	}
	m, err := loadSyncedManifest(repo)
	return m, err == nil, err
}

func manifestForSync(m Manifest) Manifest {
	m.WorkspaceRoot = "."
	m.Machines = nil
	return m
}

func localizeSyncedManifest(m Manifest, cfg Config) Manifest {
	m.WorkspaceRoot = cfg.WorkspaceRoot
	m.Machines = upsertMachine(nil, machineFromConfig(cfg))
	return m
}

func localHasUnpushedManifestChanges(local, previousRemote Manifest, hasPreviousRemote bool, remote Manifest) bool {
	if len(local.Projects) == 0 {
		return false
	}
	localBytes, err := manifestBytes(manifestForSync(local))
	if err != nil {
		return true
	}
	remoteBytes, err := manifestBytes(manifestForSync(remote))
	if err != nil {
		return true
	}
	if bytes.Equal(localBytes, remoteBytes) {
		return false
	}
	if hasPreviousRemote {
		previousBytes, err := manifestBytes(manifestForSync(previousRemote))
		if err != nil {
			return true
		}
		return !bytes.Equal(localBytes, previousBytes)
	}
	return true
}
