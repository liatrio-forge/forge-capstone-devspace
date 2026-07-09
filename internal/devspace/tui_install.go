package devspace

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

const (
	tuiReleaseRepo = "liatrio-forge/forge-capstone-devspace"
	tuiAPIBase     = "https://api.github.com"
)

type tuiRelease struct {
	Assets []tuiReleaseAsset `json:"assets"`
}

type tuiReleaseAsset struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

func newTUICommand(version string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tui",
		Short: "Manage the devspace-tui companion",
	}
	cmd.AddCommand(newTUIInstallCommand(version))
	return cmd
}

func newTUIInstallCommand(version string) *cobra.Command {
	repo := tuiReleaseRepo
	tag := defaultTUITag(version)
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install the matching devspace-tui companion",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if version == "dev" && !cmd.Flags().Changed("version") {
				return errors.New("running a dev build; pass --version vX.Y.Z")
			}
			return tuiInstall(cmd.OutOrStdout(), repo, tag)
		},
	}
	cmd.Flags().StringVar(&tag, "version", tag, "release tag to install, e.g. v0.2.0")
	cmd.Flags().StringVar(&repo, "repo", repo, "GitHub repository owner/name")
	return cmd
}

func defaultTUITag(version string) string {
	if strings.HasPrefix(version, "v") {
		return version
	}
	return "v" + version
}

func tuiInstall(out io.Writer, repo, tag string) error {
	return tuiInstallFrom(out, tuiAPIBase, repo, tag)
}

func tuiInstallFrom(out io.Writer, apiBase, repo, tag string) error {
	assetName, err := tuiAssetName(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return err
	}
	token := githubToken()
	ctx := context.Background()
	releaseClient := &http.Client{Timeout: 30 * time.Second}
	release, err := tuiFetchRelease(ctx, releaseClient, apiBase, repo, tag, token)
	if err != nil {
		return err
	}
	asset, ok := tuiFindAsset(release.Assets, assetName)
	if !ok {
		return fmt.Errorf("release asset %s for %s/%s not found", assetName, runtime.GOOS, runtime.GOARCH)
	}
	home, err := appHome()
	if err != nil {
		return err
	}
	dest := filepath.Join(home, "bin", tuiBinaryName)
	destDir := filepath.Dir(dest)
	if err := os.MkdirAll(destDir, 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(destDir, "devspace-tui.tmp*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()

	downloadClient := &http.Client{Timeout: 5 * time.Minute}
	if err := tuiDownloadTo(ctx, downloadClient, asset.URL, token, tmp); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Chmod(0o755); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}

	checksums, ok := tuiFindAsset(release.Assets, "checksums.txt")
	if !ok {
		fmt.Fprintf(out, "note: release does not publish a checksum for %s; skipping verification\n", assetName)
	} else if err := tuiVerifyChecksum(ctx, downloadClient, checksums.URL, token, tmpName, assetName); err != nil {
		if errors.Is(err, errTUIChecksumMissing) {
			fmt.Fprintf(out, "note: release does not publish a checksum for %s; skipping verification\n", assetName)
		} else {
			return err
		}
	}
	if err := os.Rename(tmpName, dest); err != nil {
		return err
	}
	fmt.Fprintf(out, "installed devspace-tui %s to %s\n", tag, dest)
	if other := findTUIBinary(); other != "" && filepath.Clean(other) != filepath.Clean(dest) {
		fmt.Fprintf(out, "note: devspace ui will prefer %s\n", other)
	}
	return nil
}

func tuiAssetName(goos, goarch string) (string, error) {
	switch goos + "/" + goarch {
	case "linux/amd64", "linux/arm64", "darwin/amd64", "darwin/arm64":
		return fmt.Sprintf("devspace-tui_%s_%s", goos, goarch), nil
	default:
		return "", fmt.Errorf("no devspace-tui build for %s/%s", goos, goarch)
	}
}

func githubToken() string {
	if token := strings.TrimSpace(os.Getenv("GITHUB_TOKEN")); token != "" {
		return token
	}
	if token := strings.TrimSpace(os.Getenv("GH_TOKEN")); token != "" {
		return token
	}
	path, err := exec.LookPath("gh")
	if err != nil {
		return ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, path, "auth", "token").Output() //nolint:gosec // path comes from exec.LookPath("gh"); args are fixed
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func tuiFetchRelease(ctx context.Context, client *http.Client, apiBase, repo, tag, token string) (tuiRelease, error) {
	url := strings.TrimRight(apiBase, "/") + "/repos/" + repo + "/releases/tags/" + tag
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return tuiRelease{}, err
	}
	tuiSetGitHubHeaders(req, "application/vnd.github+json", token)
	resp, err := client.Do(req)
	if err != nil {
		return tuiRelease{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return tuiRelease{}, errors.New("release or asset not found (private repo requires GITHUB_TOKEN or gh auth login)")
	}
	if resp.StatusCode != http.StatusOK {
		return tuiRelease{}, fmt.Errorf("release lookup failed: GitHub API returned %s", resp.Status)
	}
	var release tuiRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return tuiRelease{}, err
	}
	return release, nil
}

func tuiFindAsset(assets []tuiReleaseAsset, name string) (tuiReleaseAsset, bool) {
	for _, asset := range assets {
		if asset.Name == name {
			return asset, true
		}
	}
	return tuiReleaseAsset{}, false
}

func tuiDownloadTo(ctx context.Context, client *http.Client, url, token string, out io.Writer) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	tuiSetGitHubHeaders(req, "application/octet-stream", token)
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("asset download failed: GitHub API returned %s", resp.Status)
	}
	_, err = io.Copy(out, resp.Body)
	return err
}

func tuiSetGitHubHeaders(req *http.Request, accept, token string) {
	req.Header.Set("Accept", accept)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
}

var errTUIChecksumMissing = errors.New("checksum missing")

func tuiVerifyChecksum(ctx context.Context, client *http.Client, url, token, path, assetName string) error {
	var data strings.Builder
	if err := tuiDownloadTo(ctx, client, url, token, &data); err != nil {
		return err
	}
	want, ok := tuiChecksumForAsset(data.String(), assetName)
	if !ok {
		return errTUIChecksumMissing
	}
	got, err := tuiSHA256File(path)
	if err != nil {
		return err
	}
	if !strings.EqualFold(got, want) {
		return fmt.Errorf("checksum mismatch for %s", assetName)
	}
	return nil
}

func tuiChecksumForAsset(checksums, assetName string) (string, bool) {
	for _, line := range strings.Split(checksums, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		name := fields[len(fields)-1]
		if name == assetName || filepath.Base(name) == assetName {
			return fields[0], true
		}
	}
	return "", false
}

func tuiSHA256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()
	hash := sha256.New()
	if _, err := io.Copy(hash, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}
