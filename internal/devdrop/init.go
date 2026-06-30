package devdrop

import (
	"fmt"
	"os"
	"path/filepath"

	"filippo.io/age"
)

func InitWorkspace(workspaceArg string) (Config, error) {
	workspace, err := expandPath(workspaceArg)
	if err != nil {
		return Config{}, err
	}
	if err := os.MkdirAll(workspaceDevdrop(workspace), 0o700); err != nil {
		return Config{}, err
	}
	home, err := appHome()
	if err != nil {
		return Config{}, err
	}
	if err := os.MkdirAll(filepath.Join(home, "secrets"), 0o700); err != nil {
		return Config{}, err
	}

	now := nowRFC3339()
	cfg, err := LoadConfig()
	if err != nil && !missing(err) {
		return Config{}, err
	}
	if cfg.MachineID == "" {
		cfg.MachineID, err = randomID("machine")
		if err != nil {
			return Config{}, err
		}
		cfg.CreatedAt = now
	}
	if cfg.MachineName == "" {
		cfg.MachineName, _ = os.Hostname()
	}
	cfg.WorkspaceRoot = workspace
	cfg.UpdatedAt = now
	if cfg.AgeIdentityPath == "" {
		cfg.AgeIdentityPath = filepath.Join(home, "identity.txt")
	}
	if err := ensureAgeIdentity(cfg.AgeIdentityPath); err != nil {
		return Config{}, err
	}
	if err := SaveConfig(cfg); err != nil {
		return Config{}, err
	}

	st, err := LoadState()
	if err != nil && !missing(err) {
		return Config{}, err
	}
	if st.Projects == nil {
		st.Projects = map[string]ProjectState{}
	}
	st.MachineID = cfg.MachineID
	st.WorkspaceRoot = workspace
	if err := SaveState(st); err != nil {
		return Config{}, err
	}

	m, err := ensureWorkspaceManifest(workspace, cfg)
	if err != nil {
		return Config{}, err
	}
	if err := SaveManifest(workspace, m); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func ensureAgeIdentity(path string) error {
	if exists(path) {
		return nil
	}
	identity, err := age.GenerateX25519Identity()
	if err != nil {
		return err
	}
	content := fmt.Sprintf("# devdrop age identity\n%s\n", identity.String())
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o600)
}
