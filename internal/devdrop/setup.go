package devdrop

import (
	"os"
	"path/filepath"
)

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
