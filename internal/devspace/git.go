package devspace

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/url"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

var transportHelperPrefix = regexp.MustCompile(`^[A-Za-z0-9+.-]+::`)

type GitInfo struct {
	IsRepo         bool
	Remote         string
	Remotes        []string
	CurrentBranch  string
	DetachedHead   bool
	LastCommit     string
	Dirty          bool
	DefaultBranch  string
	MissingGit     bool
	InspectWarning string
}

func ensureGitAvailable() error {
	_, err := exec.LookPath("git")
	if err == nil {
		return nil
	}
	return fmt.Errorf("git executable not found in PATH; install Git and retry")
}

func gitInfo(path string) GitInfo {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := ensureGitAvailable(); err != nil {
		return GitInfo{MissingGit: true, InspectWarning: err.Error()}
	}
	inside, err := runGit(ctx, path, "rev-parse", "--is-inside-work-tree")
	if err != nil || inside != "true" {
		return GitInfo{}
	}
	branch, branchErr := runGit(ctx, path, "branch", "--show-current")
	detached := branchErr != nil || branch == ""
	var warnings []string
	// remote/HEAD lookups return non-zero when legitimately absent (no origin
	// configured, repo with no commits yet), so those stay on mustGit and fall
	// back to empty. A failing `status` is genuinely anomalous and would make
	// Dirty silently unreliable, so surface it.
	remoteNames := strings.Fields(mustGit(ctx, path, "remote"))
	remote := stripRemoteUserinfo(mustGit(ctx, path, "config", "--get", "remote.origin.url"))
	if remote == "" && len(remoteNames) == 1 {
		remote = stripRemoteUserinfo(mustGit(ctx, path, "remote", "get-url", remoteNames[0]))
	}
	commit := mustGit(ctx, path, "rev-parse", "--short", "HEAD")
	status, statusErr := runGit(ctx, path, "status", "--porcelain")
	if statusErr != nil {
		warnings = append(warnings, fmt.Sprintf("git status inspection failed; dirty state may be inaccurate: %s", statusErr.Error()))
	}
	def := defaultBranch(ctx, path, branch)
	if len(remoteNames) > 1 {
		warnings = append(warnings, fmt.Sprintf("multiple Git remotes configured: %s; using origin when present", strings.Join(remoteNames, ", ")))
	}
	warning := strings.Join(warnings, "; ")
	return GitInfo{
		IsRepo:         true,
		Remote:         remote,
		Remotes:        remoteNames,
		CurrentBranch:  branch,
		DetachedHead:   detached,
		LastCommit:     commit,
		Dirty:          status != "",
		DefaultBranch:  def,
		InspectWarning: warning,
	}
}

func defaultBranch(ctx context.Context, path, current string) string {
	ref := mustGit(ctx, path, "symbolic-ref", "refs/remotes/origin/HEAD")
	ref = strings.TrimPrefix(ref, "refs/remotes/origin/")
	if ref != "" {
		return ref
	}
	if current != "" {
		return current
	}
	return "main"
}

func mustGit(ctx context.Context, dir string, args ...string) string {
	out, err := runGit(ctx, dir, args...)
	if err != nil {
		return ""
	}
	return out
}

func runGit(ctx context.Context, dir string, args ...string) (string, error) {
	return runCommand(ctx, dir, "git", args...)
}

func pullRepoFastForward(dir string) error {
	if err := ensureGitAvailable(); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	_, err := runGit(ctx, dir, "pull", "--ff-only")
	return err
}

func runCommand(ctx context.Context, dir, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...) //nolint:gosec // name is always the "git" literal from runGit, the only caller
	cmd.Dir = dir
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", errors.New(msg)
	}
	return strings.TrimSpace(out.String()), nil
}

func cloneRepo(remote, dest string) error {
	if err := ensureGitAvailable(); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	// remote is validated by validateProjectRemote (called from ValidateManifest)
	// before any manifest is loaded, and "--" already prevents flag injection.
	cmd := exec.CommandContext(ctx, "git", "clone", "--", remote, dest) //nolint:gosec // validated upstream, see comment
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		// remote may still embed credentials if a caller bypasses
		// validateProjectRemote; redact defensively before it reaches output.
		msg = sanitizeRemoteInText(remote, msg)
		safeRemote := redactRemote(remote)
		//nolint:staticcheck // multi-line message deliberately formatted for direct CLI display, not wrapped
		return fmt.Errorf("git clone failed for %s into %s: %s\n\nNext steps:\n- Confirm you have access to the repository.\n- Confirm your SSH key or local remote path is configured.\n- Try running `git ls-remote %s`.", safeRemote, dest, msg, safeRemote)
	}
	return nil
}

// stripRemoteUserinfo removes an embedded credential from an absolute Git
// remote URL before it is persisted, so a token or password observed via
// `git config --get remote.origin.url` never reaches the manifest. SSH
// scp-style remotes (git@host:owner/repo) and local paths are not valid
// URLs, fail url.Parse, and are returned unchanged; they carry no parseable
// userinfo. ssh://user@host is left alone too: the login user (conventionally
// "git") is required to clone and carries no secret, since SSH auth happens
// out of band via keys. Use this for storage; use redactRemote for display,
// which keeps a placeholder username instead of dropping it.
func stripRemoteUserinfo(remote string) string {
	u, err := url.Parse(remote)
	if err != nil || u.Scheme == "" || u.User == nil {
		return remote
	}
	if _, hasPassword := u.User.Password(); !hasPassword && u.Scheme == "ssh" {
		return remote
	}
	u.User = nil
	return u.String()
}

func validateProjectRemote(remote string) error {
	remote = strings.TrimSpace(remote)
	if remote == "" {
		return nil
	}
	if strings.ContainsFunc(remote, func(r rune) bool {
		return r < 0x20 || r == 0x7f
	}) {
		return fmt.Errorf("git remote contains control character: %q", remote)
	}
	if strings.HasPrefix(remote, "-") {
		return fmt.Errorf("git remote must not begin with '-': %q", remote)
	}
	if transportHelperPrefix.MatchString(remote) {
		return fmt.Errorf("git remote uses unsupported transport-helper syntax: %q", remote)
	}
	if u, err := url.Parse(remote); err == nil && u.Scheme != "" {
		// ssh://user@host is the standard way to name the remote login user
		// (e.g. git@host) and is not a credential on its own, since SSH auth
		// happens out of band via keys. A password component, or any userinfo
		// on non-SSH schemes (e.g. a PAT used as the HTTPS username), is an
		// embedded credential and must be rejected.
		if u.User != nil {
			if _, hasPassword := u.User.Password(); hasPassword || u.Scheme != "ssh" {
				return fmt.Errorf("git remote must not embed credentials in the URL; use a Git credential helper or SSH key instead")
			}
		}
		switch u.Scheme {
		case "https", "ssh":
			if u.Host == "" {
				return fmt.Errorf("git remote %q is missing host", remote)
			}
			if strings.HasPrefix(u.Host, "-") {
				return fmt.Errorf("git remote host must not begin with '-': %q", remote)
			}
			return nil
		case "http", "git":
			if u.Host == "" {
				return fmt.Errorf("git remote %q is missing host", remote)
			}
			if strings.HasPrefix(u.Host, "-") {
				return fmt.Errorf("git remote host must not begin with '-': %q", remote)
			}
			return fmt.Errorf("git remote has unsupported scheme %q", u.Scheme)
		default:
			if u.Host != "" {
				return fmt.Errorf("git remote has unsupported scheme %q", u.Scheme)
			}
		}
	}
	return nil
}
