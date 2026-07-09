package devspace

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"
)

const syncedManifestName = "manifest.json"

type ManifestDiff struct {
	Added   []Project     `json:"added"`
	Removed []Project     `json:"removed"`
	Changed []ProjectDiff `json:"changed"`
}

type ProjectDiff struct {
	Local   Project       `json:"local"`
	Remote  Project       `json:"remote"`
	Changes []FieldChange `json:"changes"`
}

type FieldChange struct {
	Field  string `json:"field"`
	Local  string `json:"local"`
	Remote string `json:"remote"`
}

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

func CreateLocalManifestRemote(path string) (Config, error) {
	if strings.TrimSpace(path) == "" {
		return Config{}, fmt.Errorf("manifest remote path is required")
	}
	if err := ensureGitAvailable(); err != nil {
		return Config{}, err
	}
	remote, err := expandPath(path)
	if err != nil {
		return Config{}, err
	}
	if exists(remote) {
		if !isBareGitRepo(remote) {
			if !isEmptyDir(remote) {
				return Config{}, fmt.Errorf("manifest remote path is non-empty and is not a bare Git repository: %s", remote)
			}
		} else {
			return SetManifestRemote(remote)
		}
	}
	if err := os.MkdirAll(filepath.Dir(remote), 0o700); err != nil {
		return Config{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	if _, err := runCommand(ctx, "", "git", "init", "--bare", "-b", "main", remote); err != nil {
		return Config{}, fmt.Errorf("create local manifest remote failed: %w", err)
	}
	return SetManifestRemote(remote)
}

func CreateGitHubManifestRemote(repo string, private bool) (Config, error) {
	repo = strings.TrimSpace(repo)
	if repo == "" {
		return Config{}, fmt.Errorf("GitHub repository is required, for example your-org/devspace-manifest")
	}
	if strings.Count(repo, "/") != 1 {
		return Config{}, fmt.Errorf("GitHub repository must be owner/name, got %q", repo)
	}
	if _, err := exec.LookPath("gh"); err != nil {
		return Config{}, fmt.Errorf("GitHub remote creation requires GitHub CLI (`gh`); install it or create the repo manually, then run `devspace workspace remote set git@github.com:%s.git`", repo)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	visibility := "--private"
	if !private {
		visibility = "--public"
	}
	if _, err := runCommand(ctx, "", "gh", "repo", "create", repo, visibility); err != nil {
		if !strings.Contains(err.Error(), "already exists") {
			return Config{}, fmt.Errorf("GitHub manifest remote creation failed: %w", err)
		}
	}
	return SetManifestRemote("git@github.com:" + repo + ".git")
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
	if err := fetchManifestRepo(repo, cfg.ManifestRemote); err != nil {
		return false, err
	}
	if err := ensureManifestRepoNotBehind(repo); err != nil {
		return false, err
	}
	changed, err := writeSyncedManifest(repo, normalized)
	if err != nil {
		return false, err
	}
	ignoreChanged, err := writeSyncedWorkspaceIgnore(cfg.WorkspaceRoot, repo)
	if err != nil {
		return false, err
	}
	changed = changed || ignoreChanged
	if !changed {
		recordBaseManifestAfterSync(normalized)
		return false, nil
	}
	if err := commitManifestRepo(repo, cfg); err != nil {
		return false, err
	}
	if err := pushManifestRepo(repo); err != nil {
		return false, err
	}
	recordBaseManifestAfterSync(normalized)
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
	// The clone's pre-pull contents are only a proxy for the last synced
	// state and go stale whenever something else advances the cache (e.g.
	// `workspace diff` fast-forwards the same clone). Prefer the base
	// snapshot recorded on successful sync boundaries when one exists.
	base, hasBase, err := loadBaseManifest()
	if err != nil {
		return false, err
	}
	if hasBase {
		previousRemote, hasPreviousRemote = base, true
	}
	if err := pullManifestRepo(repo, cfg.ManifestRemote); err != nil {
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
	ignoreChanged, err := pullSyncedWorkspaceIgnore(repo, cfg.WorkspaceRoot)
	if err != nil {
		return false, err
	}
	after, err := os.ReadFile(manifestPath(cfg.WorkspaceRoot))
	if err != nil {
		return false, err
	}
	recordBaseManifestAfterSync(manifestForSync(localized))
	return !bytes.Equal(before, after) || ignoreChanged, nil
}

func DiffWorkspaceManifest() (ManifestDiff, error) {
	cfg, err := syncConfig()
	if err != nil {
		return ManifestDiff{}, err
	}
	local, err := LoadManifest(cfg.WorkspaceRoot)
	if err != nil {
		return ManifestDiff{}, err
	}
	localizedRemote, err := fetchLocalizedWorkspaceRemoteManifest(cfg)
	if err != nil {
		return ManifestDiff{}, err
	}
	localForDiff, err := manifestForComparison(local, cfg.WorkspaceRoot)
	if err != nil {
		return ManifestDiff{}, err
	}
	remoteForDiff, err := manifestForComparison(localizedRemote, cfg.WorkspaceRoot)
	if err != nil {
		return ManifestDiff{}, fmt.Errorf("remote manifest failed validation: %w", err)
	}
	return compareManifests(localForDiff, remoteForDiff), nil
}

func fetchLocalizedWorkspaceRemoteManifest(cfg Config) (Manifest, error) {
	repo, err := ensureManifestRepo(cfg)
	if err != nil {
		return Manifest{}, err
	}
	if err := ensureCleanManifestRepo(repo); err != nil {
		return Manifest{}, err
	}
	if err := pullManifestRepo(repo, cfg.ManifestRemote); err != nil {
		return Manifest{}, err
	}
	remote, err := loadSyncedManifest(repo)
	if err != nil {
		return Manifest{}, err
	}
	localized := localizeSyncedManifest(remote, cfg)
	if err := ValidateManifest(localized); err != nil {
		return Manifest{}, fmt.Errorf("remote manifest failed validation: %w", err)
	}
	return localized, nil
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
		return "", manifestRemoteNotReadyError(cfg.ManifestRemote, fmt.Errorf("remote clone failed: %w", err))
	}
	return repo, nil
}

func isBareGitRepo(path string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	out, err := runCommand(ctx, "", "git", "--git-dir", path, "rev-parse", "--is-bare-repository")
	return err == nil && out == "true"
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
		return fmt.Errorf("manifest repo origin is %s, but configured remote is %s; remove %s or reconcile the cache", redactRemote(current), redactRemote(remote), repo)
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

func fetchManifestRepo(repo, remote string) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	if _, err := runGit(ctx, repo, "fetch", "origin"); err != nil {
		return manifestRemoteNotReadyError(remote, fmt.Errorf("remote fetch failed: %w", err))
	}
	return nil
}

func pullManifestRepo(repo, remote string) error {
	if err := fetchManifestRepo(repo, remote); err != nil {
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

func writeSyncedWorkspaceIgnore(workspace, repo string) (bool, error) {
	src := filepath.Join(workspace, workspaceIgnoreFile)
	dst := filepath.Join(repo, workspaceIgnoreFile)
	data, err := os.ReadFile(src)
	if err != nil {
		if !missing(err) {
			return false, err
		}
		if !exists(dst) {
			return false, nil
		}
		return true, os.Remove(dst)
	}
	current, err := os.ReadFile(dst)
	if err == nil && bytes.Equal(current, data) {
		return false, nil
	}
	if err != nil && !missing(err) {
		return false, err
	}
	return true, atomicWriteFile(dst, data, 0o644, false)
}

func pullSyncedWorkspaceIgnore(repo, workspace string) (bool, error) {
	src := filepath.Join(repo, workspaceIgnoreFile)
	dst := filepath.Join(workspace, workspaceIgnoreFile)
	data, err := os.ReadFile(src)
	if err != nil {
		if !missing(err) {
			return false, err
		}
		if !exists(dst) {
			return false, nil
		}
		return true, os.Remove(dst)
	}
	current, err := os.ReadFile(dst)
	if err == nil && bytes.Equal(current, data) {
		return false, nil
	}
	if err != nil && !missing(err) {
		return false, err
	}
	return true, atomicWriteFile(dst, data, 0o644, true)
}

func manifestBytes(m Manifest) ([]byte, error) {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func commitManifestRepo(repo string, cfg Config) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	if err := ensureManifestCommitIdentity(ctx, repo, cfg); err != nil {
		return err
	}
	if _, err := runGit(ctx, repo, "add", "-A"); err != nil {
		return fmt.Errorf("manifest repo add failed: %w", err)
	}
	if _, err := runGit(ctx, repo, "-c", "commit.gpgsign=false", "commit", "-m", "Update workspace manifest"); err != nil {
		return fmt.Errorf("manifest repo commit failed: %w", err)
	}
	return nil
}

// ensureManifestCommitIdentity makes sure the manifest repo has a git author.
// Config values (cfg.ManifestCommitEmail/Name) take precedence and are set
// unconditionally so a team can re-attribute commits by updating config; when
// they are empty the legacy fixed identity (devspace@example.invalid / DevSpace)
// is applied only if the repo has no identity yet.
func ensureManifestCommitIdentity(ctx context.Context, repo string, cfg Config) error {
	email := strings.TrimSpace(cfg.ManifestCommitEmail)
	name := strings.TrimSpace(cfg.ManifestCommitName)
	if email != "" {
		if _, err := runGit(ctx, repo, "config", "user.email", email); err != nil {
			return fmt.Errorf("manifest repo user.email config failed: %w", err)
		}
	} else if mustGit(ctx, repo, "config", "--get", "user.email") == "" {
		if _, err := runGit(ctx, repo, "config", "user.email", "devspace@example.invalid"); err != nil {
			return fmt.Errorf("manifest repo user.email config failed: %w", err)
		}
	}
	if name != "" {
		if _, err := runGit(ctx, repo, "config", "user.name", name); err != nil {
			return fmt.Errorf("manifest repo user.name config failed: %w", err)
		}
	} else if mustGit(ctx, repo, "config", "--get", "user.name") == "" {
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

func manifestRemoteNotReadyError(remote string, err error) error {
	// Remote strings and the underlying git error may embed HTTPS userinfo or a
	// PAT (https://user:token@host/...). Redact before surfacing to logs/output.
	msg := sanitizeRemoteInText(remote, err.Error())
	if !strings.Contains(msg, "Repository not found") &&
		!strings.Contains(msg, "Could not read from remote repository") &&
		!strings.Contains(msg, "does not appear to be a git repository") &&
		!strings.Contains(msg, "not found") {
		return fmt.Errorf("%s", msg)
	}
	return fmt.Errorf("manifest remote is not ready: %s\n\nCreate it first, then rerun sync:\n  devspace workspace remote create github %s --private\n  devspace workspace push\n\nOr use a local bare repo:\n  devspace workspace remote create local ~/Projects/devspace-manifest.git\n  devspace workspace push\n\nOriginal error:\n%s", redactRemote(remote), githubRepoSlug(remote), msg)
}

// redactRemote strips credentials from the userinfo component of a remote URL
// so tokens never reach error messages. SSH scp-style remotes
// (git@host:owner/repo) fail url.Parse and are returned unchanged; they carry
// no secret.
func redactRemote(remote string) string {
	u, err := url.Parse(remote)
	if err != nil || u.User == nil {
		return remote
	}
	u.User = url.User("redacted")
	return u.String()
}

// sanitizeRemoteInText replaces any occurrence of a credentialed remote in free
// text (such as git stderr) with its redacted form.
func sanitizeRemoteInText(remote, text string) string {
	if red := redactRemote(remote); red != remote {
		text = strings.ReplaceAll(text, remote, red)
	}
	return text
}

func githubRepoSlug(remote string) string {
	trimmed := strings.TrimSuffix(remote, ".git")
	if strings.HasPrefix(trimmed, "git@github.com:") {
		return strings.TrimPrefix(trimmed, "git@github.com:")
	}
	if strings.HasPrefix(trimmed, "https://github.com/") {
		return strings.TrimPrefix(trimmed, "https://github.com/")
	}
	return "OWNER/REPO"
}

func localizeSyncedManifest(m Manifest, cfg Config) Manifest {
	m.WorkspaceRoot = cfg.WorkspaceRoot
	m.Machines = upsertMachine(nil, machineFromConfig(cfg))
	return m
}

func manifestForComparison(m Manifest, workspace string) (Manifest, error) {
	m.WorkspaceRoot = workspace
	for i := range m.Projects {
		_, clean, err := safeWorkspacePath(workspace, m.Projects[i].Path)
		if err != nil {
			return Manifest{}, err
		}
		m.Projects[i].Path = clean
	}
	if err := ValidateManifest(m); err != nil {
		return Manifest{}, err
	}
	return m, nil
}

func compareManifests(local, remote Manifest) ManifestDiff {
	localByID := map[string]Project{}
	remoteByID := map[string]Project{}
	for _, p := range local.Projects {
		localByID[p.ID] = p
	}
	for _, p := range remote.Projects {
		remoteByID[p.ID] = p
	}

	diff := ManifestDiff{}
	for id, remoteProject := range remoteByID {
		localProject, ok := localByID[id]
		if !ok {
			diff.Added = append(diff.Added, remoteProject)
			continue
		}
		if changes := projectChanges(localProject, remoteProject); len(changes) > 0 {
			diff.Changed = append(diff.Changed, ProjectDiff{Local: localProject, Remote: remoteProject, Changes: changes})
		}
	}
	for id, localProject := range localByID {
		if _, ok := remoteByID[id]; !ok {
			diff.Removed = append(diff.Removed, localProject)
		}
	}

	sort.Slice(diff.Added, func(i, j int) bool { return diff.Added[i].Path < diff.Added[j].Path })
	sort.Slice(diff.Removed, func(i, j int) bool { return diff.Removed[i].Path < diff.Removed[j].Path })
	sort.Slice(diff.Changed, func(i, j int) bool { return diff.Changed[i].Remote.Path < diff.Changed[j].Remote.Path })
	return diff
}

func projectChanges(local, remote Project) []FieldChange {
	var changes []FieldChange
	addChange := func(field, localValue, remoteValue string) {
		if localValue != remoteValue {
			changes = append(changes, FieldChange{Field: field, Local: localValue, Remote: remoteValue})
		}
	}
	addChange("name", local.Name, remote.Name)
	addChange("path", local.Path, remote.Path)
	addChange("type", local.Type, remote.Type)
	addChange("remote", local.Remote, remote.Remote)
	addChange("defaultBranch", local.DefaultBranch, remote.DefaultBranch)
	addChange("hydrateMode", local.HydrateMode, remote.HydrateMode)
	if !reflect.DeepEqual(local.EnvProfiles, remote.EnvProfiles) {
		changes = append(changes, FieldChange{Field: "envProfiles", Local: strings.Join(local.EnvProfiles, ","), Remote: strings.Join(remote.EnvProfiles, ",")})
	}
	if !reflect.DeepEqual(local.Ignore, remote.Ignore) {
		changes = append(changes, FieldChange{Field: "ignore", Local: strings.Join(local.Ignore, ","), Remote: strings.Join(remote.Ignore, ",")})
	}
	if !reflect.DeepEqual(local.Setup, remote.Setup) {
		changes = append(changes, FieldChange{Field: "setup", Local: setupSummary(local.Setup), Remote: setupSummary(remote.Setup)})
	}
	return changes
}

func setupSummary(setup Setup) string {
	parts := []string{}
	if setup.PackageManager != "" {
		parts = append(parts, "packageManager="+setup.PackageManager)
	}
	if setup.InstallCommand != "" {
		parts = append(parts, "installCommand="+setup.InstallCommand)
	}
	if setup.DevCommand != "" {
		parts = append(parts, "devCommand="+setup.DevCommand)
	}
	return strings.Join(parts, ",")
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
