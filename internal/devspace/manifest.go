package devspace

import (
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

func LoadManifest(workspace string) (Manifest, error) {
	var m Manifest
	err := readJSON(manifestPath(workspace), &m)
	if err != nil {
		return m, err
	}
	return m, ValidateManifest(m)
}

func SaveManifest(workspace string, m Manifest) error {
	if err := ValidateManifest(m); err != nil {
		return err
	}
	return writeJSON(manifestPath(workspace), m, 0o600)
}

func ManifestHash(workspace string) (string, error) {
	data, err := os.ReadFile(manifestPath(workspace))
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func ValidateManifest(m Manifest) error {
	if m.Version != ManifestVersion {
		return fmt.Errorf("unsupported manifest version %d", m.Version)
	}
	if strings.TrimSpace(m.WorkspaceRoot) == "" {
		return fmt.Errorf("workspaceRoot is required")
	}
	ids := map[string]bool{}
	paths := map[string]bool{}
	names := map[string]bool{}
	projectIDs := map[string]bool{}
	for _, p := range m.Projects {
		if p.ID == "" || p.Name == "" || p.Path == "" {
			return fmt.Errorf("project id, name, and path are required")
		}
		if _, _, err := safeWorkspacePath(m.WorkspaceRoot, p.Path); err != nil {
			return fmt.Errorf("project %s has invalid relative path %q: %w", p.Name, p.Path, err)
		}
		if ids[p.ID] {
			return fmt.Errorf("duplicate project id %q", p.ID)
		}
		ids[p.ID] = true
		projectIDs[p.ID] = true
		if paths[p.Path] {
			return fmt.Errorf("duplicate project path %q", p.Path)
		}
		paths[p.Path] = true
		if names[p.Name] {
			return fmt.Errorf("duplicate project name %q", p.Name)
		}
		names[p.Name] = true
		if p.Type != ProjectTypeGit && p.Type != ProjectTypeLocal && p.Type != ProjectTypeExternal {
			return fmt.Errorf("project %s has unsupported type %q", p.Name, p.Type)
		}
		if !validHydrateMode(p.HydrateMode) {
			return fmt.Errorf("project %s has unsupported hydrateMode %q", p.Name, p.HydrateMode)
		}
	}
	userIDs := map[string]bool{}
	for _, u := range m.Users {
		if strings.TrimSpace(u.ID) == "" || strings.TrimSpace(u.Name) == "" || strings.TrimSpace(u.AgeRecipient) == "" {
			return fmt.Errorf("user id, name, and ageRecipient are required")
		}
		if _, err := parseAgeRecipient(u.AgeRecipient); err != nil {
			return fmt.Errorf("user %s has invalid ageRecipient: %w", u.ID, err)
		}
		if userIDs[u.ID] {
			return fmt.Errorf("duplicate user id %q", u.ID)
		}
		userIDs[u.ID] = true
	}
	teamIDs := map[string]bool{}
	for _, team := range m.Teams {
		if strings.TrimSpace(team.ID) == "" || strings.TrimSpace(team.Name) == "" {
			return fmt.Errorf("team id and name are required")
		}
		if teamIDs[team.ID] {
			return fmt.Errorf("duplicate team id %q", team.ID)
		}
		teamIDs[team.ID] = true
		memberIDs := map[string]bool{}
		for _, member := range team.Members {
			if !userIDs[member.UserID] {
				return fmt.Errorf("team %s references unknown user %q", team.Name, member.UserID)
			}
			if memberIDs[member.UserID] {
				return fmt.Errorf("team %s has duplicate member %q", team.Name, member.UserID)
			}
			memberIDs[member.UserID] = true
			if !validAccessRole(member.Role) {
				return fmt.Errorf("team %s has unsupported role %q", team.Name, member.Role)
			}
		}
	}
	for _, access := range m.Access {
		if !projectIDs[access.ProjectID] {
			return fmt.Errorf("access references unknown project %q", access.ProjectID)
		}
		if access.UserID == "" && access.TeamID == "" {
			return fmt.Errorf("access for project %q requires userId or teamId", access.ProjectID)
		}
		if access.UserID != "" && !userIDs[access.UserID] {
			return fmt.Errorf("access references unknown user %q", access.UserID)
		}
		if access.TeamID != "" && !teamIDs[access.TeamID] {
			return fmt.Errorf("access references unknown team %q", access.TeamID)
		}
		if !validAccessRole(access.Role) {
			return fmt.Errorf("access for project %q has unsupported role %q", access.ProjectID, access.Role)
		}
	}
	return nil
}

func validHydrateMode(mode string) bool {
	return mode == HydrateImmediate ||
		mode == HydrateOnDemand ||
		mode == HydrateMetadataOnly ||
		mode == HydrateManual
}

func validAccessRole(role string) bool {
	return role == AccessRoleOwner ||
		role == AccessRoleMaintainer ||
		role == AccessRoleDeveloper ||
		role == AccessRoleViewer
}

func projectID(rel string) string {
	h := sha1.Sum([]byte(filepath.ToSlash(rel)))
	return "project_" + hex.EncodeToString(h[:])[:12]
}

func projectName(rel string) string {
	parts := strings.Split(strings.Trim(filepath.ToSlash(rel), "/"), "/")
	return parts[len(parts)-1]
}

func upsertProject(m *Manifest, p Project) {
	for i := range m.Projects {
		if m.Projects[i].ID == p.ID || m.Projects[i].Path == p.Path {
			m.Projects[i] = mergeProject(m.Projects[i], p)
			return
		}
	}
	p.Name = uniqueProjectName(m.Projects, p)
	m.Projects = append(m.Projects, p)
	slices.SortFunc(m.Projects, func(a, b Project) int {
		return strings.Compare(a.Path, b.Path)
	})
}

func uniqueProjectName(projects []Project, p Project) string {
	if !projectNameExists(projects, p.Name) {
		return p.Name
	}
	pathName := strings.ReplaceAll(strings.Trim(filepath.ToSlash(p.Path), "/"), "/", "-")
	if pathName != "" && !projectNameExists(projects, pathName) {
		return pathName
	}
	idSuffix := strings.TrimPrefix(p.ID, "project_")
	if len(idSuffix) > 8 {
		idSuffix = idSuffix[:8]
	}
	if pathName == "" {
		pathName = p.Name
	}
	return pathName + "-" + idSuffix
}

func projectNameExists(projects []Project, name string) bool {
	for _, p := range projects {
		if p.Name == name {
			return true
		}
	}
	return false
}

func mergeProject(old, next Project) Project {
	if old.EnvProfiles != nil && next.EnvProfiles == nil {
		next.EnvProfiles = old.EnvProfiles
	}
	if len(next.Ignore) == 0 {
		next.Ignore = old.Ignore
	}
	if next.HydrateMode == "" {
		next.HydrateMode = old.HydrateMode
	}
	return next
}

func ensureWorkspaceManifest(workspace string, cfg Config) (Manifest, error) {
	m := Manifest{
		Version:       ManifestVersion,
		WorkspaceRoot: workspace,
		Machines:      []Machine{machineFromConfig(cfg)},
		Projects:      []Project{},
	}
	if exists(manifestPath(workspace)) {
		loaded, err := LoadManifest(workspace)
		if err != nil {
			return m, err
		}
		loaded.WorkspaceRoot = workspace
		loaded.Machines = upsertMachine(loaded.Machines, machineFromConfig(cfg))
		return loaded, nil
	}
	if err := os.MkdirAll(workspaceDevdrop(workspace), 0o700); err != nil {
		return m, err
	}
	return m, nil
}

func upsertMachine(ms []Machine, m Machine) []Machine {
	for i := range ms {
		if ms[i].ID == m.ID {
			ms[i] = m
			return ms
		}
	}
	return append(ms, m)
}
