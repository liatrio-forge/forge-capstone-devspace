package devspace

import (
	"context"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"time"
)

type ReconcileOp struct {
	Action string `json:"action"`
	Kind   string `json:"kind"`
	Key    string `json:"key"`
}

type ReconcileResult struct {
	Merged    Manifest        `json:"merged"`
	Ops       []ReconcileOp   `json:"ops"`
	Conflicts []MergeConflict `json:"conflicts"`
	TwoWay    bool            `json:"twoWay"`
}

type ReconcileRemoteSource struct {
	GitRemote string `json:"gitRemote,omitempty"`
	Remote    string `json:"remote,omitempty"`
	Commit    string `json:"commit,omitempty"`
	Endpoint  string `json:"endpoint,omitempty"`
	Version   int    `json:"version,omitempty"`
}

type ReconcilePlan struct {
	Version       int                   `json:"version"`
	CreatedAt     string                `json:"createdAt"`
	Backend       string                `json:"backend"`
	ManifestHash  string                `json:"manifestHash"`
	RemoteSource  ReconcileRemoteSource `json:"remoteSource"`
	Ops           []ReconcileOp         `json:"ops"`
	Conflicts     []MergeConflict       `json:"conflicts"`
	TwoWay        bool                  `json:"twoWay"`
	Merged        Manifest              `json:"merged"`
	WorkspaceRoot string                `json:"workspaceRoot"`
}

func reconcileManifests(base *Manifest, local, remote Manifest) (ReconcileResult, error) {
	var result ReconcileResult
	var err error
	if base != nil {
		result.Merged, result.Conflicts, err = mergeManifests(*base, local, remote)
		if err != nil {
			return ReconcileResult{}, err
		}
	} else {
		result, err = reconcileTwoWay(local, remote)
		if err != nil {
			return ReconcileResult{}, err
		}
	}
	result.Ops = diffReconcileOps(local, result.Merged)
	return result, nil
}

func reconcileTwoWay(local, remote Manifest) (ReconcileResult, error) {
	if err := ValidateManifest(local); err != nil {
		return ReconcileResult{}, fmt.Errorf("local manifest failed validation: %w", err)
	}
	if err := ValidateManifest(remote); err != nil {
		return ReconcileResult{}, fmt.Errorf("remote manifest failed validation: %w", err)
	}
	merged := local
	var conflicts []MergeConflict

	projects, projectConflicts := twoWayProjectRecords(local.Projects, remote.Projects)
	conflicts = append(conflicts, projectConflicts...)
	merged.Projects = projects

	access, accessConflicts := twoWayRecordSection("access", local.Access, remote.Access, accessKey)
	conflicts = append(conflicts, accessConflicts...)
	merged.Access = access

	// ponytail: users/teams merge at whole-record granularity, same shortcut as
	// the three-way engine; upgrade to per-field merging if needed.
	users, userConflicts := twoWayRecordSection("user", local.Users, remote.Users, userID)
	conflicts = append(conflicts, userConflicts...)
	merged.Users = users

	teams, teamConflicts := twoWayRecordSection("team", local.Teams, remote.Teams, teamID)
	conflicts = append(conflicts, teamConflicts...)
	merged.Teams = teams

	if len(conflicts) == 0 {
		if err := ValidateManifest(merged); err != nil {
			return ReconcileResult{}, fmt.Errorf("merged manifest failed validation: %w", err)
		}
	}
	return ReconcileResult{Merged: merged, Conflicts: conflicts, TwoWay: true}, nil
}

// twoWayProjectRecords keys by Path (see mergeProjectRecords) and flags a
// same-path/different-ID pair as an "id" conflict.
func twoWayProjectRecords(local, remote []Project) ([]Project, []MergeConflict) {
	merged, conflicts := twoWayRecordSection("project", local, remote, projectPath)
	localByPath := projectByPath(local)
	remoteByPath := projectByPath(remote)
	for i, conflict := range conflicts {
		if localByPath[conflict.Key].ID != remoteByPath[conflict.Key].ID {
			conflicts[i].Field = "id"
		}
	}
	return merged, conflicts
}

func twoWayRecordSection[T any](entity string, local, remote []T, key func(T) string) ([]T, []MergeConflict) {
	localByKey := recordsByKey(local, key)
	remoteByKey := recordsByKey(remote, key)
	keys := map[string]bool{}
	for k := range localByKey {
		keys[k] = true
	}
	for k := range remoteByKey {
		keys[k] = true
	}
	var merged []T
	var conflicts []MergeConflict
	for _, k := range sortedKeys(keys) {
		localRecord, inLocal := localByKey[k]
		remoteRecord, inRemote := remoteByKey[k]
		switch {
		case inLocal && inRemote:
			merged = append(merged, localRecord)
			if !reflect.DeepEqual(localRecord, remoteRecord) {
				conflicts = append(conflicts, MergeConflict{Entity: entity, Key: k, Field: "*", Ours: fmt.Sprintf("%+v", localRecord), Theirs: fmt.Sprintf("%+v", remoteRecord)})
			}
		case inLocal:
			merged = append(merged, localRecord)
		case inRemote:
			merged = append(merged, remoteRecord)
		}
	}
	slices.SortFunc(merged, func(a, b T) int {
		return strings.Compare(key(a), key(b))
	})
	return merged, conflicts
}

func diffReconcileOps(local, merged Manifest) []ReconcileOp {
	var ops []ReconcileOp
	ops = append(ops, diffRecordOps("project", local.Projects, merged.Projects, projectPath)...)
	ops = append(ops, diffRecordOps("access", local.Access, merged.Access, accessKey)...)
	ops = append(ops, diffRecordOps("user", local.Users, merged.Users, userID)...)
	ops = append(ops, diffRecordOps("team", local.Teams, merged.Teams, teamID)...)
	return ops
}

func diffRecordOps[T any](kind string, local, merged []T, key func(T) string) []ReconcileOp {
	localByKey := recordsByKey(local, key)
	mergedByKey := recordsByKey(merged, key)
	keys := map[string]bool{}
	for k := range localByKey {
		keys[k] = true
	}
	for k := range mergedByKey {
		keys[k] = true
	}
	var ops []ReconcileOp
	for _, k := range sortedKeys(keys) {
		localRecord, inLocal := localByKey[k]
		mergedRecord, inMerged := mergedByKey[k]
		switch {
		case !inLocal && inMerged:
			ops = append(ops, ReconcileOp{Action: "added", Kind: kind, Key: k})
		case inLocal && !inMerged:
			ops = append(ops, ReconcileOp{Action: "removed", Kind: kind, Key: k})
		case inLocal && inMerged && !reflect.DeepEqual(localRecord, mergedRecord):
			ops = append(ops, ReconcileOp{Action: "changed", Kind: kind, Key: k})
		}
	}
	return ops
}

func ReconcileWorkspaceManifest(force string, apply bool, forceProjects ...map[string]string) (ReconcilePlan, error) {
	if force != "" && force != "local" && force != "remote" {
		return ReconcilePlan{}, fmt.Errorf("force must be one of: local, remote")
	}
	projectForces := mergeProjectForces(forceProjects...)
	if err := validateForceDirections(projectForces); err != nil {
		return ReconcilePlan{}, err
	}
	cfg, err := syncConfig()
	if err != nil {
		return ReconcilePlan{}, err
	}
	local, err := LoadManifest(cfg.WorkspaceRoot)
	if err != nil {
		return ReconcilePlan{}, err
	}
	hash, err := ManifestHash(cfg.WorkspaceRoot)
	if err != nil {
		return ReconcilePlan{}, err
	}
	remote, err := fetchLocalizedWorkspaceRemoteManifest(cfg)
	if err != nil {
		return ReconcilePlan{}, err
	}
	source := reconcileGitRemoteSource(cfg)
	if force == "" && len(projectForces) == 0 {
		if previous, ok := reusableReconcilePlan(local, "git", source); ok {
			plan := ReconcilePlan{
				Version:       1,
				CreatedAt:     nowRFC3339(),
				Backend:       "git",
				ManifestHash:  hash,
				RemoteSource:  source,
				TwoWay:        previous.TwoWay,
				Merged:        local,
				WorkspaceRoot: cfg.WorkspaceRoot,
			}
			if err := SaveReconcilePlan(plan); err != nil {
				return ReconcilePlan{}, err
			}
			return plan, nil
		}
	}
	baseManifest, hasBase, err := loadBaseManifest()
	if err != nil {
		return ReconcilePlan{}, err
	}
	var base *Manifest
	if hasBase {
		base = &baseManifest
	}
	result, err := reconcileManifests(base, local, remote)
	if err != nil {
		return ReconcilePlan{}, err
	}
	if err := validateProjectForces(result.Conflicts, local, remote, projectForces); err != nil {
		return ReconcilePlan{}, err
	}
	if (force != "" || len(projectForces) > 0) && len(result.Conflicts) > 0 {
		result.Merged = forceReconcileConflicts(result.Merged, result.Conflicts, local, remote, force, projectForces)
		result.Conflicts = unresolvedReconcileConflicts(result.Conflicts, local, remote, force, projectForces)
		if err := ValidateManifest(result.Merged); err != nil {
			return ReconcilePlan{}, fmt.Errorf("forced merged manifest failed validation: %w", err)
		}
		result.Ops = diffReconcileOps(local, result.Merged)
	}
	plan := ReconcilePlan{
		Version:       1,
		CreatedAt:     nowRFC3339(),
		Backend:       "git",
		ManifestHash:  hash,
		RemoteSource:  source,
		Ops:           result.Ops,
		Conflicts:     result.Conflicts,
		TwoWay:        result.TwoWay,
		Merged:        result.Merged,
		WorkspaceRoot: cfg.WorkspaceRoot,
	}
	if err := SaveReconcilePlan(plan); err != nil {
		return ReconcilePlan{}, err
	}
	if !apply {
		return plan, nil
	}
	if len(plan.Conflicts) > 0 {
		return plan, fmt.Errorf("unresolved reconcile conflicts:\n%s", formatReconcileConflictErrors(plan.Conflicts))
	}
	current, err := LoadManifest(cfg.WorkspaceRoot)
	if err != nil {
		return ReconcilePlan{}, err
	}
	currentHash, err := ManifestHash(cfg.WorkspaceRoot)
	if err != nil {
		return ReconcilePlan{}, err
	}
	if currentHash != plan.ManifestHash {
		return ReconcilePlan{}, fmt.Errorf("manifest changed since reconcile was generated; run `devspace workspace reconcile` again before apply")
	}
	backup, err := manifestBackupPath()
	if err != nil {
		return ReconcilePlan{}, err
	}
	if err := writeJSON(backup, current, 0o600); err != nil {
		return ReconcilePlan{}, err
	}
	if err := SaveManifest(cfg.WorkspaceRoot, plan.Merged); err != nil {
		return ReconcilePlan{}, err
	}
	return plan, nil
}

func ReconcileHostedManifest(force string, apply bool, forceProjects ...map[string]string) (ReconcilePlan, error) {
	if force != "" && force != "local" && force != "remote" {
		return ReconcilePlan{}, fmt.Errorf("force must be one of: local, remote")
	}
	projectForces := mergeProjectForces(forceProjects...)
	if err := validateForceDirections(projectForces); err != nil {
		return ReconcilePlan{}, err
	}
	cfg, err := GetHostedSync()
	if err != nil {
		return ReconcilePlan{}, err
	}
	local, err := LoadManifest(cfg.WorkspaceRoot)
	if err != nil {
		return ReconcilePlan{}, err
	}
	hash, err := ManifestHash(cfg.WorkspaceRoot)
	if err != nil {
		return ReconcilePlan{}, err
	}
	client := newHostedClient(cfg)
	envelope, hasRemote, err := client.get(context.Background())
	if err != nil {
		return ReconcilePlan{}, err
	}
	if !hasRemote {
		return ReconcilePlan{}, fmt.Errorf("nothing to reconcile; push first")
	}
	remote := localizeSyncedManifest(envelope.Manifest, cfg)
	if err := validateHostedManifest(remote); err != nil {
		return ReconcilePlan{}, fmt.Errorf("hosted manifest failed validation: %w", err)
	}
	source := ReconcileRemoteSource{
		Endpoint: redactRemote(cfg.HostedSyncEndpoint),
		Version:  envelope.Version,
	}
	if force == "" && len(projectForces) == 0 {
		if previous, ok := reusableReconcilePlan(local, "hosted", source); ok {
			plan := ReconcilePlan{
				Version:       1,
				CreatedAt:     nowRFC3339(),
				Backend:       "hosted",
				ManifestHash:  hash,
				RemoteSource:  source,
				TwoWay:        previous.TwoWay,
				Merged:        local,
				WorkspaceRoot: cfg.WorkspaceRoot,
			}
			if err := SaveReconcilePlan(plan); err != nil {
				return ReconcilePlan{}, err
			}
			return plan, nil
		}
	}
	baseManifest, hasBase, err := loadBaseManifest()
	if err != nil {
		return ReconcilePlan{}, err
	}
	var base *Manifest
	if hasBase {
		base = &baseManifest
	}
	result, err := reconcileManifests(base, local, remote)
	if err != nil {
		return ReconcilePlan{}, err
	}
	if err := validateProjectForces(result.Conflicts, local, remote, projectForces); err != nil {
		return ReconcilePlan{}, err
	}
	if (force != "" || len(projectForces) > 0) && len(result.Conflicts) > 0 {
		result.Merged = forceReconcileConflicts(result.Merged, result.Conflicts, local, remote, force, projectForces)
		result.Conflicts = unresolvedReconcileConflicts(result.Conflicts, local, remote, force, projectForces)
		if err := ValidateManifest(result.Merged); err != nil {
			return ReconcilePlan{}, fmt.Errorf("forced merged manifest failed validation: %w", err)
		}
		result.Ops = diffReconcileOps(local, result.Merged)
	}
	plan := ReconcilePlan{
		Version:       1,
		CreatedAt:     nowRFC3339(),
		Backend:       "hosted",
		ManifestHash:  hash,
		RemoteSource:  source,
		Ops:           result.Ops,
		Conflicts:     result.Conflicts,
		TwoWay:        result.TwoWay,
		Merged:        result.Merged,
		WorkspaceRoot: cfg.WorkspaceRoot,
	}
	if err := SaveReconcilePlan(plan); err != nil {
		return ReconcilePlan{}, err
	}
	if !apply {
		return plan, nil
	}
	if len(plan.Conflicts) > 0 {
		return plan, fmt.Errorf("unresolved reconcile conflicts:\n%s", formatReconcileConflictErrors(plan.Conflicts))
	}
	current, err := LoadManifest(cfg.WorkspaceRoot)
	if err != nil {
		return ReconcilePlan{}, err
	}
	currentHash, err := ManifestHash(cfg.WorkspaceRoot)
	if err != nil {
		return ReconcilePlan{}, err
	}
	if currentHash != plan.ManifestHash {
		return ReconcilePlan{}, fmt.Errorf("manifest changed since reconcile was generated; run `devspace hosted reconcile` again before apply")
	}
	backup, err := manifestBackupPath()
	if err != nil {
		return ReconcilePlan{}, err
	}
	if err := writeJSON(backup, current, 0o600); err != nil {
		return ReconcilePlan{}, err
	}
	normalized := manifestForSync(plan.Merged)
	if err := validateHostedManifest(normalized); err != nil {
		return ReconcilePlan{}, fmt.Errorf("merged hosted manifest failed validation: %w", err)
	}
	updated, err := client.put(context.Background(), envelope.Version, normalized)
	if err != nil {
		return ReconcilePlan{}, err
	}
	if err := SaveManifest(cfg.WorkspaceRoot, plan.Merged); err != nil {
		return ReconcilePlan{}, err
	}
	if err := recordHostedSync(updated.Version, updated.ManifestHash); err != nil {
		return ReconcilePlan{}, err
	}
	recordBaseManifestAfterSync(normalized)
	return plan, nil
}

func reusableReconcilePlan(local Manifest, backend string, source ReconcileRemoteSource) (ReconcilePlan, bool) {
	previous, err := LoadReconcilePlan()
	if err != nil {
		return ReconcilePlan{}, false
	}
	if previous.Version == 0 || previous.Backend != backend || len(previous.Conflicts) > 0 {
		return ReconcilePlan{}, false
	}
	switch backend {
	case "git":
		if previous.RemoteSource.Commit == "" || previous.RemoteSource.Commit != source.Commit {
			return ReconcilePlan{}, false
		}
	case "hosted":
		if previous.RemoteSource.Version == 0 || previous.RemoteSource.Version != source.Version {
			return ReconcilePlan{}, false
		}
	default:
		return ReconcilePlan{}, false
	}
	if !reflect.DeepEqual(local, previous.Merged) {
		return ReconcilePlan{}, false
	}
	return previous, true
}

func reconcileGitRemoteSource(cfg Config) ReconcileRemoteSource {
	source := ReconcileRemoteSource{
		GitRemote: "origin",
		Remote:    redactRemote(cfg.ManifestRemote),
	}
	repo, err := expandPath(cfg.ManifestRepoPath)
	if err != nil {
		return source
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	source.Commit = mustGit(ctx, repo, "rev-parse", "HEAD")
	return source
}

func forceReconcileConflicts(merged Manifest, conflicts []MergeConflict, local, remote Manifest, force string, forceProjects ...map[string]string) Manifest {
	projectForces := mergeProjectForces(forceProjects...)
	projects := projectByPath(merged.Projects)
	access := accessByKey(merged.Access)
	users := recordsByKey(merged.Users, userID)
	teams := recordsByKey(merged.Teams, teamID)
	for _, conflict := range conflicts {
		conflictForce := forceForConflict(conflict, local, remote, force, projectForces)
		if conflictForce == "" {
			continue
		}
		switch conflict.Entity {
		case "project":
			forceResolveRecord(projects, projectByPath(local.Projects), projectByPath(remote.Projects), conflict.Key, conflictForce)
		case "access":
			forceResolveRecord(access, accessByKey(local.Access), accessByKey(remote.Access), conflict.Key, conflictForce)
		case "user":
			forceResolveRecord(users, recordsByKey(local.Users, userID), recordsByKey(remote.Users, userID), conflict.Key, conflictForce)
		case "team":
			forceResolveRecord(teams, recordsByKey(local.Teams, teamID), recordsByKey(remote.Teams, teamID), conflict.Key, conflictForce)
		}
	}
	merged.Projects = sortedRecordSlice(projects, projectPath)
	merged.Access = sortedRecordSlice(access, accessKey)
	merged.Users = sortedRecordSlice(users, userID)
	merged.Teams = sortedRecordSlice(teams, teamID)
	return merged
}

func mergeProjectForces(forces ...map[string]string) map[string]string {
	merged := map[string]string{}
	for _, force := range forces {
		for projectID, direction := range force {
			merged[projectID] = direction
		}
	}
	return merged
}

func validateForceDirections(projectForces map[string]string) error {
	for projectID, direction := range projectForces {
		if direction != "local" && direction != "remote" {
			return fmt.Errorf("--force-project %s must resolve to local or remote", projectID)
		}
	}
	return nil
}

func validateProjectForces(conflicts []MergeConflict, local, remote Manifest, projectForces map[string]string) error {
	if len(projectForces) == 0 {
		return nil
	}
	conflicted := map[string]bool{}
	projectIDs := allProjectIDs(local, remote)
	localProjects := projectByPath(local.Projects)
	remoteProjects := projectByPath(remote.Projects)
	for _, conflict := range conflicts {
		if conflict.Entity != "project" {
			continue
		}
		localProject, hasLocal := localProjects[conflict.Key]
		remoteProject, hasRemote := remoteProjects[conflict.Key]
		if hasLocal {
			conflicted[localProject.ID] = true
		}
		if hasRemote {
			conflicted[remoteProject.ID] = true
		}
		if hasLocal && hasRemote && localProject.ID != remoteProject.ID {
			localDirection, hasLocalForce := projectForces[localProject.ID]
			remoteDirection, hasRemoteForce := projectForces[remoteProject.ID]
			if hasLocalForce && hasRemoteForce && localDirection != remoteDirection {
				return fmt.Errorf("conflicting --force-project directives for %s: %s=%s vs %s=%s", conflict.Key, localProject.ID, localDirection, remoteProject.ID, remoteDirection)
			}
		}
	}
	for projectID := range projectForces {
		if !conflicted[projectID] {
			if !projectIDs[projectID] {
				return fmt.Errorf("--force-project %s: unknown project", projectID)
			}
			return fmt.Errorf("--force-project %s has no reconcile conflict", projectID)
		}
	}
	return nil
}

func allProjectIDs(local, remote Manifest) map[string]bool {
	ids := map[string]bool{}
	for _, project := range local.Projects {
		ids[project.ID] = true
	}
	for _, project := range remote.Projects {
		ids[project.ID] = true
	}
	return ids
}

func unresolvedReconcileConflicts(conflicts []MergeConflict, local, remote Manifest, force string, projectForces map[string]string) []MergeConflict {
	var unresolved []MergeConflict
	for _, conflict := range conflicts {
		if forceForConflict(conflict, local, remote, force, projectForces) == "" {
			unresolved = append(unresolved, conflict)
		}
	}
	return unresolved
}

func forceForConflict(conflict MergeConflict, local, remote Manifest, force string, projectForces map[string]string) string {
	if conflict.Entity == "project" {
		localProjects := projectByPath(local.Projects)
		remoteProjects := projectByPath(remote.Projects)
		if project, ok := localProjects[conflict.Key]; ok {
			if direction, ok := projectForces[project.ID]; ok {
				return direction
			}
		}
		if project, ok := remoteProjects[conflict.Key]; ok {
			if direction, ok := projectForces[project.ID]; ok {
				return direction
			}
		}
	}
	return force
}

func forceResolveRecord[T any](records, localRecords, remoteRecords map[string]T, key, force string) {
	source := localRecords
	if force == "remote" {
		source = remoteRecords
	}
	if record, ok := source[key]; ok {
		records[key] = record
	} else {
		delete(records, key)
	}
}

func sortedRecordSlice[T any](records map[string]T, key func(T) string) []T {
	var out []T
	for _, record := range records {
		out = append(out, record)
	}
	slices.SortFunc(out, func(a, b T) int {
		return strings.Compare(key(a), key(b))
	})
	return out
}

func formatReconcileConflictErrors(conflicts []MergeConflict) string {
	lines := make([]string, 0, len(conflicts))
	for _, conflict := range conflicts {
		lines = append(lines, fmt.Sprintf("- %s %s %s: local=%q remote=%q", conflict.Entity, conflict.Key, conflict.Field, conflict.Ours, conflict.Theirs))
	}
	return strings.Join(lines, "\n")
}

func SaveReconcilePlan(plan ReconcilePlan) error {
	path, err := reconcilePlanPath()
	if err != nil {
		return err
	}
	return writeJSON(path, plan, 0o600)
}

func LoadReconcilePlan() (ReconcilePlan, error) {
	var plan ReconcilePlan
	path, err := reconcilePlanPath()
	if err != nil {
		return plan, err
	}
	if err := readJSON(path, &plan); err != nil {
		return plan, err
	}
	return plan, nil
}
