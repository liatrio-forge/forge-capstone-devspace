package devdrop

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"filippo.io/age"
)

func EnvSet(projectRef, key, profile string, in io.Reader) error {
	if profile == "" {
		profile = "dev"
	}
	key = strings.TrimSpace(key)
	if key == "" || strings.ContainsAny(key, "=\n\r") {
		return fmt.Errorf("invalid env key %q", key)
	}
	valueBytes, err := io.ReadAll(in)
	if err != nil {
		return err
	}
	value := strings.TrimRight(string(valueBytes), "\r\n")
	if value == "" {
		return fmt.Errorf("empty secret value; pass it on stdin")
	}
	cfg, m, p, err := projectContext(projectRef)
	if err != nil {
		return err
	}
	sp, _ := readSecretProfile(cfg, p.ID, profile)
	if sp.Values == nil {
		sp.Values = map[string]string{}
	}
	sp.ProjectID = p.ID
	sp.Profile = profile
	sp.Values[key] = value
	sp.UpdatedAt = nowRFC3339()
	if err := writeSecretProfile(cfg, sp); err != nil {
		return err
	}
	if !contains(p.EnvProfiles, profile) {
		p.EnvProfiles = append(p.EnvProfiles, profile)
		sort.Strings(p.EnvProfiles)
		for i := range m.Projects {
			if m.Projects[i].ID == p.ID {
				m.Projects[i] = p
				break
			}
		}
		return SaveManifest(cfg.WorkspaceRoot, m)
	}
	return nil
}

func EnvList(projectRef, profile string) ([]string, error) {
	if profile == "" {
		profile = "dev"
	}
	cfg, _, p, err := projectContext(projectRef)
	if err != nil {
		return nil, err
	}
	sp, err := readSecretProfile(cfg, p.ID, profile)
	if err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(sp.Values))
	for k := range sp.Values {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys, nil
}

func EnvPull(projectRef, profile string) (string, error) {
	if profile == "" {
		profile = "dev"
	}
	cfg, _, p, err := projectContext(projectRef)
	if err != nil {
		return "", err
	}
	sp, err := readSecretProfile(cfg, p.ID, profile)
	if err != nil {
		return "", err
	}
	full := filepath.Join(cfg.WorkspaceRoot, filepath.FromSlash(p.Path), ".env")
	var b strings.Builder
	keys := make([]string, 0, len(sp.Values))
	for k := range sp.Values {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(&b, "%s=%s\n", k, sp.Values[k])
	}
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(full, []byte(b.String()), 0o600); err != nil {
		return "", err
	}
	if st, err := LoadState(); err == nil {
		info := gitInfo(filepath.Join(cfg.WorkspaceRoot, filepath.FromSlash(p.Path)))
		st.Projects[p.ID] = stateForProject(filepath.Join(cfg.WorkspaceRoot, filepath.FromSlash(p.Path)), p, info)
		_ = SaveState(st)
	}
	return full, nil
}

func projectContext(projectRef string) (Config, Manifest, Project, error) {
	cfg, err := LoadConfig()
	if err != nil {
		return Config{}, Manifest{}, Project{}, err
	}
	m, err := LoadManifest(cfg.WorkspaceRoot)
	if err != nil {
		return Config{}, Manifest{}, Project{}, err
	}
	p, ok := findProject(m, projectRef)
	if !ok {
		return Config{}, Manifest{}, Project{}, fmt.Errorf("project %q not found", projectRef)
	}
	return cfg, m, p, nil
}

func readSecretProfile(cfg Config, projectID, profile string) (SecretProfile, error) {
	path := secretPath(cfg, projectID, profile)
	var empty SecretProfile
	if !exists(path) {
		return empty, os.ErrNotExist
	}
	identity, err := loadIdentity(cfg.AgeIdentityPath)
	if err != nil {
		return empty, err
	}
	ciphertext, err := os.ReadFile(path)
	if err != nil {
		return empty, err
	}
	r, err := age.Decrypt(bytes.NewReader(ciphertext), identity)
	if err != nil {
		return empty, err
	}
	plaintext, err := io.ReadAll(r)
	if err != nil {
		return empty, err
	}
	var sp SecretProfile
	if err := json.Unmarshal(plaintext, &sp); err != nil {
		return empty, err
	}
	return sp, nil
}

func writeSecretProfile(cfg Config, sp SecretProfile) error {
	identity, err := loadIdentity(cfg.AgeIdentityPath)
	if err != nil {
		return err
	}
	recipient := identity.Recipient()
	var buf bytes.Buffer
	w, err := age.Encrypt(&buf, recipient)
	if err != nil {
		return err
	}
	plain, err := json.MarshalIndent(sp, "", "  ")
	if err != nil {
		return err
	}
	if _, err := w.Write(plain); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	path := secretPath(cfg, sp.ProjectID, sp.Profile)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(path, buf.Bytes(), 0o600)
}

func secretPath(cfg Config, projectID, profile string) string {
	return filepath.Join(cfg.WorkspaceRoot, ".devdrop", "secrets", projectID, profile+".age")
}

func loadIdentity(path string) (*age.X25519Identity, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		return age.ParseX25519Identity(line)
	}
	return nil, fmt.Errorf("no age identity found in %s", path)
}

func contains(values []string, value string) bool {
	for _, v := range values {
		if v == value {
			return true
		}
	}
	return false
}
