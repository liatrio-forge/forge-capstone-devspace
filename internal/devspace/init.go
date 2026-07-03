package devspace

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
	// The default identity path is intentionally left unset in cfg so it is
	// not persisted as an absolute path; resolveAgeIdentityPath fills it in
	// at read time. Only a custom, user-set AgeIdentityPath is persisted.
	identityPath, err := resolveAgeIdentityPath(cfg)
	if err != nil {
		return Config{}, err
	}
	if err := ensureAgeIdentity(identityPath); err != nil {
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

// resolveAgeIdentityPath returns the effective age identity path for cfg. A
// non-empty cfg.AgeIdentityPath is a user-configured override and is
// returned as-is. Otherwise the default location under the app home
// directory is resolved on demand rather than persisted, so a rename of the
// app home directory (e.g. .devdrop -> .devspace) does not require
// rewriting every config.json that relied on the default path.
func resolveAgeIdentityPath(cfg Config) (string, error) {
	if cfg.AgeIdentityPath != "" {
		return cfg.AgeIdentityPath, nil
	}
	home, err := appHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "identity.txt"), nil
}

func ensureAgeIdentity(path string) error {
	if info, err := os.Lstat(path); err == nil {
		if !info.Mode().IsRegular() {
			return fmt.Errorf("age identity path %q is not a regular file", path)
		}
		if info.Mode().Perm() != 0o600 {
			if err := os.Chmod(path, 0o600); err != nil {
				return err
			}
		}
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	identity, err := age.GenerateX25519Identity()
	if err != nil {
		return err
	}
	content := fmt.Sprintf("# devspace age identity\n%s\n", identity.String())
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return atomicWriteFile(path, []byte(content), 0o600, false)
}
