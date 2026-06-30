package devdrop

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
	MachineID       string `json:"machineId"`
	MachineName     string `json:"machineName"`
	WorkspaceRoot   string `json:"workspaceRoot"`
	AgeIdentityPath string `json:"ageIdentityPath"`
	CreatedAt       string `json:"createdAt"`
	UpdatedAt       string `json:"updatedAt"`
}

type State struct {
	MachineID     string                  `json:"machineId"`
	WorkspaceRoot string                  `json:"workspaceRoot"`
	Projects      map[string]ProjectState `json:"projects"`
	LastScanAt    string                  `json:"lastScanAt,omitempty"`
	LastSyncAt    string                  `json:"lastSyncAt,omitempty"`
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
	Version       int       `json:"version"`
	WorkspaceRoot string    `json:"workspaceRoot"`
	Machines      []Machine `json:"machines"`
	Projects      []Project `json:"projects"`
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

type SecretProfile struct {
	ProjectID string            `json:"projectId"`
	Profile   string            `json:"profile"`
	Values    map[string]string `json:"values"`
	UpdatedAt string            `json:"updatedAt"`
}

func nowRFC3339() string {
	return time.Now().UTC().Format(time.RFC3339)
}
