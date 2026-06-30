package devdrop

import (
	"bytes"
	"context"
	"os/exec"
	"strings"
	"time"
)

type GitInfo struct {
	IsRepo        bool
	Remote        string
	CurrentBranch string
	LastCommit    string
	Dirty         bool
	DefaultBranch string
}

func gitInfo(path string) GitInfo {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if runGit(ctx, path, "rev-parse", "--is-inside-work-tree") != "true" {
		return GitInfo{}
	}
	branch := runGit(ctx, path, "branch", "--show-current")
	remote := runGit(ctx, path, "config", "--get", "remote.origin.url")
	commit := runGit(ctx, path, "rev-parse", "--short", "HEAD")
	status := runGit(ctx, path, "status", "--porcelain")
	def := defaultBranch(ctx, path)
	return GitInfo{
		IsRepo:        true,
		Remote:        remote,
		CurrentBranch: branch,
		LastCommit:    commit,
		Dirty:         status != "",
		DefaultBranch: def,
	}
}

func defaultBranch(ctx context.Context, path string) string {
	ref := runGit(ctx, path, "symbolic-ref", "refs/remotes/origin/HEAD")
	ref = strings.TrimPrefix(ref, "refs/remotes/origin/")
	if ref != "" {
		return ref
	}
	branch := runGit(ctx, path, "branch", "--show-current")
	if branch != "" {
		return branch
	}
	return "main"
}

func runGit(ctx context.Context, dir string, args ...string) string {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return ""
	}
	return strings.TrimSpace(out.String())
}

func cloneRepo(remote, dest string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "clone", remote, dest)
	return cmd.Run()
}
