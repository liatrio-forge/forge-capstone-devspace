package devspace

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
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
	if err := validSecretName(profile); err != nil {
		return err
	}
	key = strings.TrimSpace(key)
	if key == "" || strings.ContainsAny(key, "=\n\r \t") {
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
	if strings.ContainsAny(value, "\n\r") {
		return fmt.Errorf("env value for %q contains newlines, which cannot be written to .env", key)
	}
	cfg, m, p, err := projectContext(projectRef)
	if err != nil {
		return err
	}
	sp, err := readSecretProfile(cfg, p.ID, profile)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		sp = SecretProfile{}
	}
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
	m = ensureManifestLocalUserAccess(cfg, m, p.ID, profile)
	if !contains(p.EnvProfiles, profile) {
		p.EnvProfiles = append(p.EnvProfiles, profile)
		sort.Strings(p.EnvProfiles)
		for i := range m.Projects {
			if m.Projects[i].ID == p.ID {
				m.Projects[i] = p
				break
			}
		}
	}
	return SaveManifest(cfg.WorkspaceRoot, m)
}

func EnvRecipientExport() (SecretRecipient, error) {
	cfg, err := LoadConfig()
	if err != nil {
		return SecretRecipient{}, err
	}
	return localSecretRecipient(cfg, nowRFC3339())
}

func EnvRecipients(projectRef, profile string) ([]SecretRecipient, error) {
	if profile == "" {
		profile = "dev"
	}
	if err := validSecretName(profile); err != nil {
		return nil, err
	}
	cfg, _, p, err := projectContext(projectRef)
	if err != nil {
		return nil, err
	}
	sp, err := readSecretProfile(cfg, p.ID, profile)
	if err != nil {
		return nil, err
	}
	recipients := append([]SecretRecipient(nil), sp.Recipients...)
	sort.Slice(recipients, func(i, j int) bool {
		if recipients[i].Name == recipients[j].Name {
			return recipients[i].ID < recipients[j].ID
		}
		return recipients[i].Name < recipients[j].Name
	})
	return recipients, nil
}

func EnvInvite(projectRef, profile, name, recipientText, teamName string) (SecretRecipient, error) {
	if profile == "" {
		profile = "dev"
	}
	if err := validSecretName(profile); err != nil {
		return SecretRecipient{}, err
	}
	recipient, err := parseAgeRecipient(recipientText)
	if err != nil {
		return SecretRecipient{}, err
	}
	now := nowRFC3339()
	shared := SecretRecipient{
		ID:           recipientID(recipient.String()),
		Name:         defaultRecipientName(name, recipient.String()),
		AgeRecipient: recipient.String(),
		AddedAt:      now,
	}
	cfg, m, p, err := projectContext(projectRef)
	if err != nil {
		return SecretRecipient{}, err
	}
	sp, err := readSecretProfile(cfg, p.ID, profile)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return SecretRecipient{}, err
		}
		sp = SecretProfile{ProjectID: p.ID, Profile: profile, Values: map[string]string{}}
	}
	if sp.Values == nil {
		sp.Values = map[string]string{}
	}
	sp.ProjectID = p.ID
	sp.Profile = profile
	sp.Recipients = upsertSecretRecipient(sp.Recipients, shared)
	sp.UpdatedAt = now
	if err := writeSecretProfile(cfg, sp); err != nil {
		return SecretRecipient{}, err
	}
	m = ensureManifestLocalUserAccess(cfg, m, p.ID, profile)
	m = upsertManifestSharedAccess(m, p.ID, profile, shared, teamName, now)
	if !contains(p.EnvProfiles, profile) {
		p.EnvProfiles = append(p.EnvProfiles, profile)
		sort.Strings(p.EnvProfiles)
		for i := range m.Projects {
			if m.Projects[i].ID == p.ID {
				m.Projects[i] = p
				break
			}
		}
	}
	if err := SaveManifest(cfg.WorkspaceRoot, m); err != nil {
		return SecretRecipient{}, err
	}
	return shared, nil
}

func EnvRevoke(projectRef, profile, recipientRef, reason string) (SecretRecipient, error) {
	if profile == "" {
		profile = "dev"
	}
	if err := validSecretName(profile); err != nil {
		return SecretRecipient{}, err
	}
	cfg, m, p, err := projectContext(projectRef)
	if err != nil {
		return SecretRecipient{}, err
	}
	local, err := localSecretRecipient(cfg, nowRFC3339())
	if err != nil {
		return SecretRecipient{}, err
	}
	sp, err := readSecretProfile(cfg, p.ID, profile)
	if err != nil {
		return SecretRecipient{}, err
	}
	now := nowRFC3339()
	var revoked SecretRecipient
	found := false
	for i := range sp.Recipients {
		if recipientMatches(sp.Recipients[i], recipientRef) {
			if sp.Recipients[i].ID == local.ID {
				return SecretRecipient{}, fmt.Errorf("cannot revoke the local recipient from %s/%s", projectRef, profile)
			}
			sp.Recipients[i].RevokedAt = now
			revoked = sp.Recipients[i]
			found = true
			break
		}
	}
	if !found {
		return SecretRecipient{}, fmt.Errorf("recipient %q is not active on %s/%s", recipientRef, projectRef, profile)
	}
	sp.Revocations = append(sp.Revocations, SecretRevocation{
		RecipientID: revoked.ID,
		Name:        revoked.Name,
		RevokedAt:   now,
		Reason:      reason,
	})
	sp.UpdatedAt = now
	if err := writeSecretProfile(cfg, sp); err != nil {
		return SecretRecipient{}, err
	}
	m = revokeManifestAccess(m, p.ID, profile, revoked.ID, now)
	if err := SaveManifest(cfg.WorkspaceRoot, m); err != nil {
		return SecretRecipient{}, err
	}
	return revoked, nil
}

func EnvRotateRecipients(projectRef, profile string) error {
	if profile == "" {
		profile = "dev"
	}
	if err := validSecretName(profile); err != nil {
		return err
	}
	cfg, _, p, err := projectContext(projectRef)
	if err != nil {
		return err
	}
	sp, err := readSecretProfile(cfg, p.ID, profile)
	if err != nil {
		return err
	}
	sp.UpdatedAt = nowRFC3339()
	return writeSecretProfile(cfg, sp)
}

func EnvList(projectRef, profile string) ([]string, error) {
	if profile == "" {
		profile = "dev"
	}
	if err := validSecretName(profile); err != nil {
		return nil, err
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
	if err := validSecretName(profile); err != nil {
		return "", err
	}
	cfg, _, p, err := projectContext(projectRef)
	if err != nil {
		return "", err
	}
	sp, err := readSecretProfile(cfg, p.ID, profile)
	if err != nil {
		return "", err
	}
	projectPath, _, err := safeWorkspacePath(cfg.WorkspaceRoot, p.Path)
	if err != nil {
		return "", err
	}
	full := filepath.Join(projectPath, ".env")
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
		info := gitInfo(projectPath)
		st.Projects[p.ID] = stateForProject(projectPath, p, info)
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
	identityPath, err := resolveAgeIdentityPath(cfg)
	if err != nil {
		return empty, err
	}
	identity, err := loadIdentity(identityPath)
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
	recipients, normalized, err := activeAgeRecipients(cfg, sp)
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	w, err := age.Encrypt(&buf, recipients...)
	if err != nil {
		return err
	}
	plain, err := json.MarshalIndent(normalized, "", "  ")
	if err != nil {
		return err
	}
	if _, err := w.Write(plain); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	path := secretPath(cfg, normalized.ProjectID, normalized.Profile)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(path, buf.Bytes(), 0o600)
}

func activeAgeRecipients(cfg Config, sp SecretProfile) ([]age.Recipient, SecretProfile, error) {
	now := nowRFC3339()
	local, err := localSecretRecipient(cfg, now)
	if err != nil {
		return nil, SecretProfile{}, err
	}
	normalized := sp
	normalized.Recipients = upsertSecretRecipient(normalized.Recipients, local)
	var recipients []age.Recipient
	seen := map[string]bool{}
	for _, recipient := range normalized.Recipients {
		if recipient.RevokedAt != "" {
			continue
		}
		ageRecipient, err := parseAgeRecipient(recipient.AgeRecipient)
		if err != nil {
			return nil, SecretProfile{}, fmt.Errorf("recipient %s is invalid: %w", recipient.ID, err)
		}
		if seen[recipient.ID] {
			continue
		}
		recipients = append(recipients, ageRecipient)
		seen[recipient.ID] = true
	}
	if len(recipients) == 0 {
		return nil, SecretProfile{}, fmt.Errorf("secret profile has no active recipients")
	}
	return recipients, normalized, nil
}

func localSecretRecipient(cfg Config, addedAt string) (SecretRecipient, error) {
	identityPath, err := resolveAgeIdentityPath(cfg)
	if err != nil {
		return SecretRecipient{}, err
	}
	identity, err := loadIdentity(identityPath)
	if err != nil {
		return SecretRecipient{}, err
	}
	recipient := identity.Recipient().String()
	return SecretRecipient{
		ID:           recipientID(recipient),
		Name:         defaultRecipientName(cfg.MachineName, recipient),
		AgeRecipient: recipient,
		AddedAt:      addedAt,
	}, nil
}

func parseAgeRecipient(value string) (*age.X25519Recipient, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, fmt.Errorf("age recipient is required")
	}
	return age.ParseX25519Recipient(value)
}

func recipientID(ageRecipient string) string {
	sum := sha1.Sum([]byte(ageRecipient))
	return "user_" + hex.EncodeToString(sum[:])[:12]
}

func defaultRecipientName(name, ageRecipient string) string {
	name = strings.TrimSpace(name)
	if name != "" {
		return name
	}
	id := recipientID(ageRecipient)
	return "recipient-" + strings.TrimPrefix(id, "user_")
}

func upsertSecretRecipient(recipients []SecretRecipient, next SecretRecipient) []SecretRecipient {
	for i := range recipients {
		if recipients[i].ID == next.ID || recipients[i].AgeRecipient == next.AgeRecipient {
			if next.Name != "" {
				recipients[i].Name = next.Name
			}
			recipients[i].AgeRecipient = next.AgeRecipient
			if recipients[i].AddedAt == "" {
				recipients[i].AddedAt = next.AddedAt
			}
			recipients[i].RevokedAt = ""
			return recipients
		}
	}
	return append(recipients, next)
}

func recipientMatches(recipient SecretRecipient, ref string) bool {
	ref = strings.TrimSpace(ref)
	return recipient.ID == ref || recipient.Name == ref || recipient.AgeRecipient == ref
}

func ensureManifestLocalUserAccess(cfg Config, m Manifest, projectID, profile string) Manifest {
	local, err := localSecretRecipient(cfg, nowRFC3339())
	if err != nil {
		return m
	}
	now := nowRFC3339()
	m = upsertManifestUser(m, local, now)
	return upsertManifestProjectAccess(m, ProjectAccess{
		ProjectID:   projectID,
		UserID:      local.ID,
		Role:        AccessRoleOwner,
		EnvProfiles: []string{profile},
		AddedAt:     now,
	})
}

func upsertManifestSharedAccess(m Manifest, projectID, profile string, recipient SecretRecipient, teamName string, now string) Manifest {
	m = upsertManifestUser(m, recipient, now)
	teamName = strings.TrimSpace(teamName)
	if teamName == "" {
		return upsertManifestProjectAccess(m, ProjectAccess{
			ProjectID:   projectID,
			UserID:      recipient.ID,
			Role:        AccessRoleDeveloper,
			EnvProfiles: []string{profile},
			AddedAt:     now,
		})
	}
	teamID := stableNameID("team", teamName)
	m = upsertManifestTeamMember(m, teamID, teamName, TeamMember{
		UserID:  recipient.ID,
		Role:    AccessRoleDeveloper,
		AddedAt: now,
	}, now)
	return upsertManifestProjectAccess(m, ProjectAccess{
		ProjectID:   projectID,
		TeamID:      teamID,
		Role:        AccessRoleDeveloper,
		EnvProfiles: []string{profile},
		AddedAt:     now,
	})
}

func upsertManifestUser(m Manifest, recipient SecretRecipient, now string) Manifest {
	user := User{
		ID:           recipient.ID,
		Name:         recipient.Name,
		AgeRecipient: recipient.AgeRecipient,
		Status:       "active",
		CreatedAt:    now,
	}
	for i := range m.Users {
		if m.Users[i].ID == user.ID {
			m.Users[i].Name = user.Name
			m.Users[i].AgeRecipient = user.AgeRecipient
			m.Users[i].Status = "active"
			m.Users[i].RevokedAt = ""
			if m.Users[i].CreatedAt == "" {
				m.Users[i].CreatedAt = now
			}
			return m
		}
	}
	m.Users = append(m.Users, user)
	sort.Slice(m.Users, func(i, j int) bool { return m.Users[i].ID < m.Users[j].ID })
	return m
}

func upsertManifestTeamMember(m Manifest, teamID, teamName string, member TeamMember, now string) Manifest {
	for i := range m.Teams {
		if m.Teams[i].ID != teamID {
			continue
		}
		for j := range m.Teams[i].Members {
			if m.Teams[i].Members[j].UserID == member.UserID {
				m.Teams[i].Members[j].Role = member.Role
				m.Teams[i].Members[j].RevokedAt = ""
				if m.Teams[i].Members[j].AddedAt == "" {
					m.Teams[i].Members[j].AddedAt = now
				}
				return m
			}
		}
		m.Teams[i].Members = append(m.Teams[i].Members, member)
		return m
	}
	m.Teams = append(m.Teams, Team{ID: teamID, Name: teamName, Members: []TeamMember{member}, CreatedAt: now})
	sort.Slice(m.Teams, func(i, j int) bool { return m.Teams[i].ID < m.Teams[j].ID })
	return m
}

func upsertManifestProjectAccess(m Manifest, next ProjectAccess) Manifest {
	for i := range m.Access {
		if m.Access[i].ProjectID == next.ProjectID && m.Access[i].UserID == next.UserID && m.Access[i].TeamID == next.TeamID {
			m.Access[i].Role = next.Role
			m.Access[i].RevokedAt = ""
			if m.Access[i].AddedAt == "" {
				m.Access[i].AddedAt = next.AddedAt
			}
			m.Access[i].EnvProfiles = mergeProfiles(m.Access[i].EnvProfiles, next.EnvProfiles)
			return m
		}
	}
	next.EnvProfiles = mergeProfiles(nil, next.EnvProfiles)
	m.Access = append(m.Access, next)
	sort.Slice(m.Access, func(i, j int) bool {
		if m.Access[i].ProjectID == m.Access[j].ProjectID {
			return m.Access[i].UserID+m.Access[i].TeamID < m.Access[j].UserID+m.Access[j].TeamID
		}
		return m.Access[i].ProjectID < m.Access[j].ProjectID
	})
	return m
}

func revokeManifestAccess(m Manifest, projectID, profile, userID, now string) Manifest {
	for i := range m.Users {
		if m.Users[i].ID == userID {
			m.Users[i].Status = "revoked"
			m.Users[i].RevokedAt = now
		}
	}
	for i := range m.Teams {
		for j := range m.Teams[i].Members {
			if m.Teams[i].Members[j].UserID == userID {
				m.Teams[i].Members[j].RevokedAt = now
			}
		}
	}
	for i := range m.Access {
		if m.Access[i].ProjectID != projectID {
			continue
		}
		if m.Access[i].UserID == userID || accessProfileOnlyThroughTeam(m, m.Access[i], userID) {
			m.Access[i].EnvProfiles = removeProfile(m.Access[i].EnvProfiles, profile)
			if len(m.Access[i].EnvProfiles) == 0 {
				m.Access[i].RevokedAt = now
			}
		}
	}
	return m
}

func accessProfileOnlyThroughTeam(m Manifest, access ProjectAccess, userID string) bool {
	if access.TeamID == "" {
		return false
	}
	for _, team := range m.Teams {
		if team.ID != access.TeamID {
			continue
		}
		userInTeam := false
		for _, member := range team.Members {
			if member.UserID == userID {
				userInTeam = true
				continue
			}
			if member.RevokedAt == "" {
				return false
			}
		}
		return userInTeam
	}
	return false
}

func mergeProfiles(existing, next []string) []string {
	seen := map[string]bool{}
	var merged []string
	for _, profile := range append(existing, next...) {
		if profile == "" || seen[profile] {
			continue
		}
		seen[profile] = true
		merged = append(merged, profile)
	}
	sort.Strings(merged)
	return merged
}

func removeProfile(profiles []string, profile string) []string {
	var kept []string
	for _, value := range profiles {
		if value != profile {
			kept = append(kept, value)
		}
	}
	return kept
}

func stableNameID(prefix, name string) string {
	sum := sha1.Sum([]byte(strings.ToLower(strings.TrimSpace(name))))
	return prefix + "_" + hex.EncodeToString(sum[:])[:12]
}

func secretPath(cfg Config, projectID, profile string) string {
	return filepath.Join(workspaceDevdrop(cfg.WorkspaceRoot), "secrets", projectID, profile+".age")
}

// validSecretName rejects profile names that could escape the per-project
// secrets directory. Only a plain single path segment is allowed.
func validSecretName(name string) error {
	if name == "" || name == "." || name == ".." {
		return fmt.Errorf("invalid profile name %q", name)
	}
	if strings.ContainsAny(name, `/\`) || strings.Contains(name, "..") {
		return fmt.Errorf("invalid profile name %q", name)
	}
	return nil
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
