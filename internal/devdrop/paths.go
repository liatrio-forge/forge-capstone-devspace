package devdrop

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const envHome = "DEV_DROP_HOME"

func appHome() (string, error) {
	if override := os.Getenv(envHome); override != "" {
		return filepath.Abs(override)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".devdrop"), nil
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
	return filepath.Join(workspace, ".devdrop", "manifest.json")
}

func workspaceDevdrop(workspace string) string {
	return filepath.Join(workspace, ".devdrop")
}

func randomID(prefix string) (string, error) {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return prefix + "_" + hex.EncodeToString(b[:]), nil
}
