package devspace

import (
	"os"
	"runtime"
)

func LoadConfig() (Config, error) {
	var cfg Config
	p, err := configPath()
	if err != nil {
		return cfg, err
	}
	err = readJSON(p, &cfg)
	return cfg, err
}

func SaveConfig(cfg Config) error {
	p, err := configPath()
	if err != nil {
		return err
	}
	return writeJSON(p, cfg, 0o600)
}

func LoadState() (State, error) {
	var st State
	p, err := statePath()
	if err != nil {
		return st, err
	}
	err = readJSON(p, &st)
	if st.Projects == nil {
		st.Projects = map[string]ProjectState{}
	}
	return st, err
}

func SaveState(st State) error {
	p, err := statePath()
	if err != nil {
		return err
	}
	if st.Projects == nil {
		st.Projects = map[string]ProjectState{}
	}
	return writeJSON(p, st, 0o600)
}

func machineFromConfig(cfg Config) Machine {
	name := cfg.MachineName
	if name == "" {
		name, _ = os.Hostname()
	}
	return Machine{
		ID:            cfg.MachineID,
		Name:          name,
		OS:            runtime.GOOS,
		Arch:          runtime.GOARCH,
		WorkspaceRoot: cfg.WorkspaceRoot,
		LastSeenAt:    nowRFC3339(),
	}
}
