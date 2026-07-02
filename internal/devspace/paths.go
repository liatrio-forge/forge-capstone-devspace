package devspace

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

const (
	envHome       = "DEVSPACE_HOME"
	envHomeLegacy = "DEV_DROP_HOME"
)

const (
	appDirName             = ".devspace"
	legacyAppDirName       = ".devdrop"
	workspaceDirName       = ".devspace"
	legacyWorkspaceDirName = ".devdrop"
)

// appHome is a pure resolver: it never touches the filesystem beyond
// reading environment variables. Migrating a legacy ~/.devdrop directory to
// ~/.devspace is handled separately by migrateLegacyHome.
func appHome() (string, error) {
	if override := os.Getenv(envHome); override != "" {
		return expandPath(override)
	}
	if override := os.Getenv(envHomeLegacy); override != "" {
		return expandPath(override)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, appDirName), nil
}

func configPath() (string, error) {
	home, err := appHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "config.json"), nil
}

func statePath() (string, error) {
	home, err := appHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "state.json"), nil
}

func expandPath(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("path is required")
	}
	if path == "~" || strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		path = filepath.Join(home, strings.TrimPrefix(path, "~/"))
	}
	return filepath.Abs(path)
}

func manifestPath(workspace string) string {
	return filepath.Join(workspaceDevdrop(workspace), "manifest.json")
}

func lastPlanPath(workspace string) string {
	return filepath.Join(workspaceDevdrop(workspace), "last-plan.json")
}

// workspaceDevdrop returns the active devspace metadata directory for a
// workspace. It prefers an existing .devspace directory, falls back to
// reading a legacy .devdrop directory if that is what is present (read-both
// transition support), and otherwise defaults to .devspace for creation.
//
// Reads and writes deliberately resolve to the SAME directory so a workspace
// stays coherent: splitting reads (.devdrop fallback) from writes (.devspace)
// would strand the manifest in one dir while writing updates to the other. An
// existing .devdrop workspace is left in place rather than force-renamed, so a
// machine still on the old binary keeps working during the transition window.
func workspaceDevdrop(workspace string) string {
	current := filepath.Join(workspace, workspaceDirName)
	if exists(current) {
		return current
	}
	legacy := filepath.Join(workspace, legacyWorkspaceDirName)
	if exists(legacy) {
		return legacy
	}
	return current
}

func safeWorkspacePath(workspace, rel string) (string, string, error) {
	if rel == "" {
		return "", "", fmt.Errorf("project path is required")
	}
	if filepath.IsAbs(rel) {
		return "", "", fmt.Errorf("project path must be relative: %s", rel)
	}
	clean := filepath.ToSlash(filepath.Clean(rel))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return "", "", fmt.Errorf("project path escapes workspace: %s", rel)
	}
	root, err := filepath.Abs(workspace)
	if err != nil {
		return "", "", err
	}
	full, err := filepath.Abs(filepath.Join(root, filepath.FromSlash(clean)))
	if err != nil {
		return "", "", err
	}
	back, err := filepath.Rel(root, full)
	if err != nil {
		return "", "", err
	}
	back = filepath.ToSlash(back)
	if back == ".." || strings.HasPrefix(back, "../") || filepath.IsAbs(back) {
		return "", "", fmt.Errorf("project path escapes workspace: %s", rel)
	}
	// Lexical checks above cannot catch symlink-based escapes (a directory
	// inside the workspace that links outside it). Resolve symlinks on the
	// existing portions of both root and candidate and re-check containment.
	realRoot, err := resolveExisting(root)
	if err != nil {
		return "", "", err
	}
	realFull, err := resolveExisting(full)
	if err != nil {
		return "", "", err
	}
	realBack, err := filepath.Rel(realRoot, realFull)
	if err != nil {
		return "", "", err
	}
	realBack = filepath.ToSlash(realBack)
	if realBack == ".." || strings.HasPrefix(realBack, "../") || filepath.IsAbs(realBack) {
		return "", "", fmt.Errorf("project path escapes workspace via symlink: %s", rel)
	}
	return full, clean, nil
}

// resolveExisting evaluates symlinks for the longest existing prefix of path
// and re-appends the not-yet-created remainder. This lets callers detect a
// symlink escape even when the final path does not exist yet.
func resolveExisting(path string) (string, error) {
	p := path
	var tail []string
	for {
		resolved, err := filepath.EvalSymlinks(p)
		if err == nil {
			if len(tail) == 0 {
				return resolved, nil
			}
			slices.Reverse(tail)
			return filepath.Join(append([]string{resolved}, tail...)...), nil
		}
		if !os.IsNotExist(err) {
			return "", err
		}
		parent := filepath.Dir(p)
		if parent == p {
			return path, nil
		}
		tail = append(tail, filepath.Base(p))
		p = parent
	}
}

func randomID(prefix string) (string, error) {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return prefix + "_" + hex.EncodeToString(b[:]), nil
}
