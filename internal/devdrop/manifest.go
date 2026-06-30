package devdrop

import (
	"crypto/sha1"
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

func ValidateManifest(m Manifest) error {
	if m.Version != ManifestVersion {
		return fmt.Errorf("unsupported manifest version %d", m.Version)
	}
	if strings.TrimSpace(m.WorkspaceRoot) == "" {
		return fmt.Errorf("workspaceRoot is required")
	}
	paths := map[string]bool{}
	names := map[string]bool{}
	for _, p := range m.Projects {
		if p.ID == "" || p.Name == "" || p.Path == "" {
			return fmt.Errorf("project id, name, and path are required")
		}
		cleanPath := filepath.ToSlash(filepath.Clean(p.Path))
		if filepath.IsAbs(p.Path) || cleanPath == ".." || strings.HasPrefix(cleanPath, "../") {
			return fmt.Errorf("project %s has invalid relative path %q", p.Name, p.Path)
		}
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
	return nil
}

func validHydrateMode(mode string) bool {
	return mode == HydrateImmediate ||
		mode == HydrateOnDemand ||
		mode == HydrateMetadataOnly ||
		mode == HydrateManual
}

func projectID(rel string) string {
	h := sha1.Sum([]byte(filepath.ToSlash(rel)))
	return "project_" + hex.EncodeToString(h[:])[:12]
}

func projectName(rel string) string {
	return filepath.Base(filepath.Clean(rel))
}

func upsertProject(m *Manifest, p Project) {
	for i := range m.Projects {
		if m.Projects[i].ID == p.ID || m.Projects[i].Path == p.Path {
			m.Projects[i] = mergeProject(m.Projects[i], p)
			return
		}
	}
	m.Projects = append(m.Projects, p)
	slices.SortFunc(m.Projects, func(a, b Project) int {
		return strings.Compare(a.Path, b.Path)
	})
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
