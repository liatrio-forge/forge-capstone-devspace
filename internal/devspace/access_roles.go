package devspace

import (
	"fmt"
	"io"
	"strings"
)

var rolePrivilege = map[string]int{
	AccessRoleViewer:     1,
	AccessRoleDeveloper:  2,
	AccessRoleMaintainer: 3,
	AccessRoleOwner:      4,
}

type effectiveRoleResult struct {
	UserID   string
	Role     string
	Warnings []string
}

type roleCandidate struct {
	projectID string
	role      string
	source    string
}

func effectiveRole(m Manifest, projectID, localAgeRecipient string) effectiveRoleResult {
	return resolveEffectiveRole(m, projectID, strings.TrimSpace(localAgeRecipient))
}

func effectiveWorkspaceRole(m Manifest, localAgeRecipient string) effectiveRoleResult {
	return resolveEffectiveRole(m, "", strings.TrimSpace(localAgeRecipient))
}

func resolveEffectiveRole(m Manifest, projectID, localAgeRecipient string) effectiveRoleResult {
	var result effectiveRoleResult
	user, ok := activeManifestUser(m, localAgeRecipient)
	if !ok {
		result.Warnings = append(result.Warnings, "Access role advisory: no active manifest user matches the local age recipient.")
		return result
	}
	result.UserID = user.ID

	candidates, warnings := activeRoleCandidates(m, projectID, user.ID)
	result.Warnings = append(result.Warnings, warnings...)
	if len(candidates) == 0 {
		result.Warnings = append(result.Warnings, "Access role advisory: no active project role was found for the local manifest user.")
		return result
	}

	result.Role = mostPrivilegedRole(candidates)
	if projectID != "" && directTeamDisagreement(candidates) {
		result.Warnings = append(result.Warnings, "Access role advisory: direct and team grants disagree; using the most privileged active grant.")
	}
	return result
}

func activeManifestUser(m Manifest, localAgeRecipient string) (User, bool) {
	for _, user := range m.Users {
		if user.AgeRecipient != localAgeRecipient {
			continue
		}
		if strings.EqualFold(user.Status, "revoked") || user.RevokedAt != "" {
			continue
		}
		return user, true
	}
	return User{}, false
}

func activeRoleCandidates(m Manifest, projectID, userID string) ([]roleCandidate, []string) {
	teamsByID := map[string]Team{}
	for _, team := range m.Teams {
		teamsByID[team.ID] = team
	}

	var candidates []roleCandidate
	var warnings []string
	for _, access := range m.Access {
		if access.RevokedAt != "" {
			continue
		}
		if projectID != "" && access.ProjectID != projectID {
			continue
		}
		accessRank, ok := roleRank(access.Role)
		if !ok {
			warnings = append(warnings, fmt.Sprintf("Access role advisory: unknown project access role %q on project %q.", access.Role, access.ProjectID))
			continue
		}
		if access.UserID == userID {
			candidates = append(candidates, roleCandidate{projectID: access.ProjectID, role: access.Role, source: "direct"})
		}
		if access.TeamID == "" {
			continue
		}
		team, ok := teamsByID[access.TeamID]
		if !ok {
			continue
		}
		member, ok := activeTeamMember(team, userID)
		if !ok {
			continue
		}
		memberRank, ok := roleRank(member.Role)
		if !ok {
			warnings = append(warnings, fmt.Sprintf("Access role advisory: unknown team member role %q on team %q.", member.Role, team.ID))
			continue
		}
		cappedRole := access.Role
		if memberRank < accessRank {
			cappedRole = member.Role
		}
		candidates = append(candidates, roleCandidate{projectID: access.ProjectID, role: cappedRole, source: "team"})
	}
	return candidates, warnings
}

func activeTeamMember(team Team, userID string) (TeamMember, bool) {
	for _, member := range team.Members {
		if member.UserID == userID && member.RevokedAt == "" {
			return member, true
		}
	}
	return TeamMember{}, false
}

func roleRank(role string) (int, bool) {
	rank, ok := rolePrivilege[role]
	return rank, ok
}

func mostPrivilegedRole(candidates []roleCandidate) string {
	best := ""
	bestRank := 0
	for _, candidate := range candidates {
		rank, ok := roleRank(candidate.role)
		if !ok {
			continue
		}
		if rank > bestRank {
			best = candidate.role
			bestRank = rank
		}
	}
	return best
}

func directTeamDisagreement(candidates []roleCandidate) bool {
	type projectRoles struct {
		direct map[string]bool
		team   map[string]bool
	}
	rolesByProject := map[string]projectRoles{}
	for _, candidate := range candidates {
		roles := rolesByProject[candidate.projectID]
		if roles.direct == nil {
			roles.direct = map[string]bool{}
			roles.team = map[string]bool{}
		}
		switch candidate.source {
		case "direct":
			roles.direct[candidate.role] = true
		case "team":
			roles.team[candidate.role] = true
		}
		rolesByProject[candidate.projectID] = roles
	}
	for _, roles := range rolesByProject {
		for direct := range roles.direct {
			for team := range roles.team {
				if direct != team {
					return true
				}
			}
		}
	}
	return false
}

func accessRoleAdvisoryWarnings(surface, projectRef string, allowedRoles ...string) []string {
	cfg, err := LoadConfig()
	if err != nil {
		return nil
	}
	m, err := LoadManifest(cfg.WorkspaceRoot)
	if err != nil {
		return nil
	}
	if len(m.Users) == 0 && len(m.Access) == 0 {
		return nil
	}
	identityPath, err := resolveAgeIdentityPath(cfg)
	if err != nil {
		return nil
	}
	identity, err := loadIdentity(identityPath)
	if err != nil {
		return nil
	}

	var result effectiveRoleResult
	if projectRef == "" {
		result = effectiveWorkspaceRole(m, identity.Recipient().String())
	} else {
		project, ok := findProject(m, projectRef)
		if !ok {
			return nil
		}
		result = effectiveRole(m, project.ID, identity.Recipient().String())
	}

	warnings := append([]string(nil), result.Warnings...)
	if !roleIn(result.Role, allowedRoles) {
		role := result.Role
		if role == "" {
			role = "none"
		}
		warnings = append(warnings, fmt.Sprintf(
			"Access role advisory: %s is recommended for %s, but the local effective role is %s. Command will continue because roles are advisory.",
			surface,
			humanRoleList(allowedRoles),
			role,
		))
	}
	return warnings
}

func printAccessRoleAdvisories(out io.Writer, warnings []string) {
	for _, warning := range warnings {
		printCaution(out, "%s", warning)
	}
}

func roleIn(role string, allowed []string) bool {
	for _, allowedRole := range allowed {
		if role == allowedRole {
			return true
		}
	}
	return false
}

func humanRoleList(roles []string) string {
	switch len(roles) {
	case 0:
		return "no manifest roles"
	case 1:
		return roles[0]
	case 2:
		return roles[0] + " or " + roles[1]
	default:
		return strings.Join(roles[:len(roles)-1], ", ") + ", or " + roles[len(roles)-1]
	}
}
