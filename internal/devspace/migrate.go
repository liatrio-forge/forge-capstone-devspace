package devspace

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// migrateLegacyHome moves a legacy ~/.devdrop application home directory to
// the new ~/.devspace location the first time devspace runs after the
// rename. It respects explicit DEVSPACE_HOME/DEV_DROP_HOME overrides (no
// migration happens when either is set, since the user has already pointed
// devspace somewhere specific) and is cheap to call on every invocation: it
// is a fast no-op once the migration has happened or was never needed.
func migrateLegacyHome() error {
	if os.Getenv(envHome) != "" || os.Getenv(envHomeLegacy) != "" {
		return nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	newPath := filepath.Join(home, appDirName)
	oldPath := filepath.Join(home, legacyAppDirName)
	if exists(newPath) || !exists(oldPath) {
		return nil
	}
	if err := os.Rename(oldPath, newPath); err != nil {
		return fmt.Errorf("migrate %s to %s: %w", oldPath, newPath, err)
	}
	if err := rewriteMigratedConfigPaths(oldPath, newPath); err != nil {
		return fmt.Errorf("rewrite config paths after migrating %s to %s: %w", oldPath, newPath, err)
	}
	fmt.Fprintln(os.Stderr, "devspace: migrated ~/.devdrop -> ~/.devspace")
	return nil
}

// rewriteMigratedConfigPaths rewrites absolute paths stored in config.json
// that pointed inside the legacy home directory so they point at the new
// home directory after the rename. Paths outside the legacy home (e.g. a
// custom AgeIdentityPath the user set explicitly) are left untouched.
func rewriteMigratedConfigPaths(oldHome, newHome string) error {
	cfg, err := LoadConfig()
	if err != nil {
		if missing(err) {
			return nil
		}
		return err
	}
	rewritten := cfg
	rewritten.AgeIdentityPath = rewriteLegacyHomePrefix(cfg.AgeIdentityPath, oldHome, newHome)
	rewritten.ManifestRepoPath = rewriteLegacyHomePrefix(cfg.ManifestRepoPath, oldHome, newHome)
	rewritten.ManifestRemote = rewriteLegacyHomePrefix(cfg.ManifestRemote, oldHome, newHome)
	if rewritten == cfg {
		return nil
	}
	return SaveConfig(rewritten)
}

// rewriteLegacyHomePrefix replaces an oldHome prefix in value with newHome,
// matching only whole path components so it does not corrupt an unrelated
// path that merely shares oldHome as a string prefix (e.g. ~/.devdrop-other).
func rewriteLegacyHomePrefix(value, oldHome, newHome string) string {
	if value == "" {
		return value
	}
	if value == oldHome {
		return newHome
	}
	prefix := oldHome + string(filepath.Separator)
	if strings.HasPrefix(value, prefix) {
		return filepath.Join(newHome, strings.TrimPrefix(value, prefix))
	}
	return value
}
