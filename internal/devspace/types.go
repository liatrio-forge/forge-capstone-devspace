package devspace

import "time"

const (
	ManifestVersion = 1

	ProjectTypeGit      = "git"
	ProjectTypeLocal    = "local"
	ProjectTypeExternal = "external"

	HydrateImmediate    = "immediate"
	HydrateOnDemand     = "on-demand"
	HydrateMetadataOnly = "metadata-only"
	HydrateManual       = "manual"

	AccessRoleOwner      = "owner"
	AccessRoleMaintainer = "maintainer"
	AccessRoleDeveloper  = "developer"
	AccessRoleViewer     = "viewer"
)

var DefaultIgnores = []string{
	"node_modules",
	"dist",
	"build",
	".next",
	"turbo",
	"target",
	"vendor",
	"coverage",
	".cache",
	".DS_Store",
	"*.log",
}

type Config struct {
	MachineID           string `json:"machineId"`
	MachineName         string `json:"machineName"`
	WorkspaceRoot       string `json:"workspaceRoot"`
	AgeIdentityPath     string `json:"ageIdentityPath"`
	ManifestRemote      string `json:"manifestRemote,omitempty"`
	ManifestRepoPath    string `json:"manifestRepoPath,omitempty"`
	HostedSyncEndpoint  string `json:"hostedSyncEndpoint,omitempty"`
	HostedSyncToken     string `json:"hostedSyncToken,omitempty"`
	HostedSyncWorkspace string `json:"hostedSyncWorkspace,omitempty"`
	CreatedAt           string `json:"createdAt"`
	UpdatedAt           string `json:"updatedAt"`
}

type State struct {
	MachineID              string                  `json:"machineId"`
	WorkspaceRoot          string                  `json:"workspaceRoot"`
	Projects               map[string]ProjectState `json:"projects"`
	LastScanAt             string                  `json:"lastScanAt,omitempty"`
	LastSyncAt             string                  `json:"lastSyncAt,omitempty"`
	HostedSyncVersion      int                     `json:"hostedSyncVersion,omitempty"`
	HostedSyncManifestHash string                  `json:"hostedSyncManifestHash,omitempty"`
}

type Machine struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	OS            string `json:"os"`
	Arch          string `json:"arch"`
	WorkspaceRoot string `json:"workspaceRoot"`
	LastSeenAt    string `json:"lastSeenAt"`
}

type Manifest struct {
	Version       int             `json:"version"`
	WorkspaceRoot string          `json:"workspaceRoot"`
	Machines      []Machine       `json:"machines"`
	Projects      []Project       `json:"projects"`
	Users         []User          `json:"users,omitempty"`
	Teams         []Team          `json:"teams,omitempty"`
	Access        []ProjectAccess `json:"access,omitempty"`
}

type User struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	AgeRecipient string `json:"ageRecipient"`
	Status       string `json:"status,omitempty"`
	CreatedAt    string `json:"createdAt"`
	RevokedAt    string `json:"revokedAt,omitempty"`
}

type Team struct {
	ID        string       `json:"id"`
	Name      string       `json:"name"`
	Members   []TeamMember `json:"members,omitempty"`
	CreatedAt string       `json:"createdAt"`
}

type TeamMember struct {
	UserID    string `json:"userId"`
	Role      string `json:"role"`
	AddedAt   string `json:"addedAt"`
	RevokedAt string `json:"revokedAt,omitempty"`
}

type Project struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	Path          string   `json:"path"`
	Type          string   `json:"type"`
	Remote        string   `json:"remote,omitempty"`
	DefaultBranch string   `json:"defaultBranch,omitempty"`
	HydrateMode   string   `json:"hydrateMode"`
	EnvProfiles   []string `json:"envProfiles,omitempty"`
	Ignore        []string `json:"ignore,omitempty"`
	Setup         Setup    `json:"setup,omitempty"`
}

type ProjectAccess struct {
	ProjectID   string   `json:"projectId"`
	UserID      string   `json:"userId,omitempty"`
	TeamID      string   `json:"teamId,omitempty"`
	Role        string   `json:"role"`
	EnvProfiles []string `json:"envProfiles,omitempty"`
	AddedAt     string   `json:"addedAt"`
	RevokedAt   string   `json:"revokedAt,omitempty"`
}

type Setup struct {
	PackageManager string `json:"packageManager,omitempty"`
	InstallCommand string `json:"installCommand,omitempty"`
	DevCommand     string `json:"devCommand,omitempty"`
}

type ProjectState struct {
	Hydrated       bool   `json:"hydrated"`
	Exists         bool   `json:"exists"`
	Dirty          bool   `json:"dirty"`
	CurrentBranch  string `json:"currentBranch,omitempty"`
	LastCommit     string `json:"lastCommit,omitempty"`
	EnvFilePresent bool   `json:"envFilePresent"`
	LastCheckedAt  string `json:"lastCheckedAt"`
	Placeholder    bool   `json:"placeholder"`
	Stale          bool   `json:"stale"`
	Missing        bool   `json:"missing"`
}

type Plan struct {
	Version       int          `json:"version"`
	WorkspaceRoot string       `json:"workspaceRoot"`
	ManifestHash  string       `json:"manifestHash"`
	GeneratedAt   string       `json:"generatedAt"`
	Actions       []PlanAction `json:"actions"`
	Warnings      []string     `json:"warnings"`
}

type PlanAction struct {
	Safety  string `json:"safety"`
	Kind    string `json:"kind"`
	Path    string `json:"path"`
	Reason  string `json:"reason,omitempty"`
	Project string `json:"project,omitempty"`
}

type SecretProfile struct {
	ProjectID   string             `json:"projectId"`
	Profile     string             `json:"profile"`
	Values      map[string]string  `json:"values"`
	Recipients  []SecretRecipient  `json:"recipients,omitempty"`
	Revocations []SecretRevocation `json:"revocations,omitempty"`
	UpdatedAt   string             `json:"updatedAt"`
}

type SecretRecipient struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	AgeRecipient string `json:"ageRecipient"`
	AddedAt      string `json:"addedAt"`
	RevokedAt    string `json:"revokedAt,omitempty"`
}

type SecretRevocation struct {
	RecipientID string `json:"recipientId"`
	Name        string `json:"name,omitempty"`
	RevokedAt   string `json:"revokedAt"`
	Reason      string `json:"reason,omitempty"`
}

func nowRFC3339() string {
	return time.Now().UTC().Format(time.RFC3339)
}
