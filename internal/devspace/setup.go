package devspace

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type SetupPlan struct {
	GeneratedAt   string             `json:"generatedAt"`
	WorkspaceRoot string             `json:"workspaceRoot"`
	Projects      []SetupPlanProject `json:"projects"`
}

type SetupPlanProject struct {
	Project        string `json:"project"`
	Path           string `json:"path"`
	PackageManager string `json:"packageManager,omitempty"`
	InstallCommand string `json:"installCommand,omitempty"`
	DevCommand     string `json:"devCommand,omitempty"`
	Runnable       bool   `json:"runnable"`
	Reason         string `json:"reason,omitempty"`
}

type SetupRunOptions struct {
	CommandKind  string
	DryRun       bool
	AllowUnknown bool
	AllowGlobal  bool
	Stdout       io.Writer
	Stderr       io.Writer
}

type SetupRunResult struct {
	Project string
	Path    string
	Command string
	DryRun  bool
}

type setupCommandSpec struct {
	Display string
	Args    []string
	Unknown bool
	Global  bool
}

func detectSetup(path string) Setup {
	files := map[string]bool{}
	for _, name := range []string{
		"pnpm-lock.yaml", "yarn.lock", "package-lock.json", "bun.lockb",
		"package.json", "Cargo.toml", "go.mod", "requirements.txt",
		"pyproject.toml", "Gemfile",
	} {
		if exists(filepath.Join(path, name)) {
			files[name] = true
		}
	}
	switch {
	case files["pnpm-lock.yaml"]:
		return Setup{PackageManager: "pnpm", InstallCommand: "pnpm install", DevCommand: "pnpm dev"}
	case files["yarn.lock"]:
		return Setup{PackageManager: "yarn", InstallCommand: "yarn install", DevCommand: "yarn dev"}
	case files["bun.lockb"]:
		return Setup{PackageManager: "bun", InstallCommand: "bun install", DevCommand: "bun dev"}
	case files["package.json"]:
		return Setup{PackageManager: "npm", InstallCommand: "npm install", DevCommand: "npm run dev"}
	case files["Cargo.toml"]:
		return Setup{PackageManager: "cargo", InstallCommand: "cargo build"}
	case files["go.mod"]:
		return Setup{PackageManager: "go", InstallCommand: "go mod download"}
	case files["requirements.txt"]:
		return Setup{PackageManager: "pip", InstallCommand: "pip install -r requirements.txt"}
	case files["pyproject.toml"]:
		return Setup{PackageManager: "python", InstallCommand: "pip install -e ."}
	case files["Gemfile"]:
		return Setup{PackageManager: "bundler", InstallCommand: "bundle install"}
	default:
		return Setup{}
	}
}

func hasDependencyMarker(path string) bool {
	for _, name := range []string{"package.json", "Cargo.toml", "go.mod", "requirements.txt", "pyproject.toml", "Gemfile"} {
		if _, err := os.Stat(filepath.Join(path, name)); err == nil {
			return true
		}
	}
	return false
}

func BuildSetupPlan() (SetupPlan, error) {
	cfg, err := LoadConfig()
	if err != nil {
		return SetupPlan{}, err
	}
	m, err := LoadManifest(cfg.WorkspaceRoot)
	if err != nil {
		return SetupPlan{}, err
	}
	plan := SetupPlan{
		GeneratedAt:   nowRFC3339(),
		WorkspaceRoot: cfg.WorkspaceRoot,
	}
	for _, p := range m.Projects {
		if p.Setup.InstallCommand == "" && p.Setup.DevCommand == "" && p.Setup.PackageManager == "" {
			continue
		}
		entry := SetupPlanProject{
			Project:        p.Name,
			Path:           p.Path,
			PackageManager: p.Setup.PackageManager,
			InstallCommand: p.Setup.InstallCommand,
			DevCommand:     p.Setup.DevCommand,
		}
		if full, _, err := safeWorkspacePath(cfg.WorkspaceRoot, p.Path); err != nil {
			entry.Reason = err.Error()
		} else if info, err := os.Stat(full); err != nil {
			if os.IsNotExist(err) {
				entry.Reason = "project path does not exist locally"
			} else {
				entry.Reason = err.Error()
			}
		} else if !info.IsDir() {
			entry.Reason = "project path is not a directory"
		} else if _, ok := knownSetupCommand(p.Setup, "install"); !ok {
			entry.Reason = "install command is not a known safe command"
		} else {
			entry.Runnable = true
		}
		plan.Projects = append(plan.Projects, entry)
	}
	return plan, nil
}

func RunProjectSetup(ref string, opts SetupRunOptions) (SetupRunResult, error) {
	if opts.CommandKind == "" {
		opts.CommandKind = "install"
	}
	cfg, err := LoadConfig()
	if err != nil {
		return SetupRunResult{}, err
	}
	m, err := LoadManifest(cfg.WorkspaceRoot)
	if err != nil {
		return SetupRunResult{}, err
	}
	p, ok := findProject(m, ref)
	if !ok {
		return SetupRunResult{}, fmt.Errorf("project %q not found", ref)
	}
	full, _, err := safeWorkspacePath(cfg.WorkspaceRoot, p.Path)
	if err != nil {
		return SetupRunResult{}, err
	}
	info, err := os.Stat(full)
	if err != nil {
		if os.IsNotExist(err) {
			return SetupRunResult{}, fmt.Errorf("cannot run setup for %s: project path does not exist locally", p.Path)
		}
		return SetupRunResult{}, err
	}
	if !info.IsDir() {
		return SetupRunResult{}, fmt.Errorf("cannot run setup for %s: project path is not a directory", p.Path)
	}
	spec, err := setupCommandFor(p.Setup, opts.CommandKind, opts.AllowUnknown)
	if err != nil {
		return SetupRunResult{}, err
	}
	if spec.Global && !opts.AllowGlobal {
		return SetupRunResult{}, fmt.Errorf("refusing global setup command %q; pass --allow-global after review", spec.Display)
	}
	result := SetupRunResult{Project: p.Name, Path: p.Path, Command: spec.Display, DryRun: opts.DryRun}
	if opts.DryRun {
		return result, nil
	}
	if err := runSetupCommand(full, spec.Args, opts.Stdout, opts.Stderr); err != nil {
		return result, fmt.Errorf("setup failed for %s: %w", p.Path, err)
	}
	return result, nil
}

func RunAllProjectSetups(opts SetupRunOptions) ([]SetupRunResult, error) {
	cfg, err := LoadConfig()
	if err != nil {
		return nil, err
	}
	m, err := LoadManifest(cfg.WorkspaceRoot)
	if err != nil {
		return nil, err
	}
	var results []SetupRunResult
	for _, p := range m.Projects {
		if p.Setup.InstallCommand == "" {
			continue
		}
		result, err := RunProjectSetup(p.Name, opts)
		if err != nil {
			return results, err
		}
		results = append(results, result)
	}
	return results, nil
}

func setupCommandFor(setup Setup, kind string, allowUnknown bool) (setupCommandSpec, error) {
	if kind != "install" && kind != "dev" {
		return setupCommandSpec{}, fmt.Errorf("unsupported setup command %q; use install or dev", kind)
	}
	command := setup.InstallCommand
	if kind == "dev" {
		command = setup.DevCommand
	}
	if strings.TrimSpace(command) == "" {
		return setupCommandSpec{}, fmt.Errorf("no %s command detected for package manager %q", kind, setup.PackageManager)
	}
	if args, ok := knownSetupCommand(setup, kind); ok {
		return setupCommandSpec{Display: command, Args: args, Global: looksGlobalInstall(command)}, nil
	}
	if !allowUnknown {
		return setupCommandSpec{}, fmt.Errorf("refusing unknown setup command %q; review it and pass --allow-unknown to run it", command)
	}
	return setupCommandSpec{
		Display: command,
		Args:    []string{"sh", "-c", command},
		Unknown: true,
		Global:  looksGlobalInstall(command),
	}, nil
}

func knownSetupCommand(setup Setup, kind string) ([]string, bool) {
	command := setup.InstallCommand
	if kind == "dev" {
		command = setup.DevCommand
	}
	switch setup.PackageManager + "\x00" + kind + "\x00" + command {
	case "pnpm\x00install\x00pnpm install":
		return []string{"pnpm", "install"}, true
	case "pnpm\x00dev\x00pnpm dev":
		return []string{"pnpm", "dev"}, true
	case "yarn\x00install\x00yarn install":
		return []string{"yarn", "install"}, true
	case "yarn\x00dev\x00yarn dev":
		return []string{"yarn", "dev"}, true
	case "bun\x00install\x00bun install":
		return []string{"bun", "install"}, true
	case "bun\x00dev\x00bun dev":
		return []string{"bun", "dev"}, true
	case "npm\x00install\x00npm install":
		return []string{"npm", "install"}, true
	case "npm\x00dev\x00npm run dev":
		return []string{"npm", "run", "dev"}, true
	case "cargo\x00install\x00cargo build":
		return []string{"cargo", "build"}, true
	case "go\x00install\x00go mod download":
		return []string{"go", "mod", "download"}, true
	case "pip\x00install\x00pip install -r requirements.txt":
		return []string{"pip", "install", "-r", "requirements.txt"}, true
	case "python\x00install\x00pip install -e .":
		return []string{"pip", "install", "-e", "."}, true
	case "bundler\x00install\x00bundle install":
		return []string{"bundle", "install"}, true
	default:
		return nil, false
	}
}

func looksGlobalInstall(command string) bool {
	fields := strings.Fields(command)
	for i, field := range fields {
		switch field {
		case "-g", "--global":
			return true
		case "global":
			if i > 0 && (fields[i-1] == "yarn" || fields[i-1] == "pnpm") {
				return true
			}
		}
	}
	return false
}

func runSetupCommand(dir string, args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("setup command is empty")
	}
	// args come from a project's own declared setup config, only reachable via
	// the explicit `devspace setup` command (never auto-executed), behind
	// safeWorkspacePath + the --allow-global gate for global commands.
	cmd := exec.Command(args[0], args[1:]...) //nolint:gosec // gated, see comment
	cmd.Dir = dir
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd.Run()
}
