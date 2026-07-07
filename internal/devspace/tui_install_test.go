package devspace

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
)

func TestTUIInstallHappyPathWithChecksum(t *testing.T) {
	home, assetName := tuiInstallTestEnv(t)
	body := []byte("fake-binary")
	sum := sha256.Sum256(body)
	server := tuiInstallTestServer(t, tuiInstallTestOptions{
		AssetName:       assetName,
		AssetBody:       body,
		IncludeAsset:    true,
		IncludeChecksum: true,
		ChecksumBody:    fmt.Sprintf("%x  %s\n", sum, assetName),
	})
	defer server.Close()

	var out bytes.Buffer
	if err := tuiInstallFrom(&out, server.URL, "o/r", "v1.2.3"); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(home, "bin", tuiBinaryName)
	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(body) {
		t.Fatalf("installed content = %q", got)
	}
	info, err := os.Stat(dest)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0o111 == 0 {
		t.Fatalf("installed mode = %v, want executable", info.Mode().Perm())
	}
}

func TestTUIInstallChecksumMismatchCleansUp(t *testing.T) {
	home, assetName := tuiInstallTestEnv(t)
	server := tuiInstallTestServer(t, tuiInstallTestOptions{
		AssetName:       assetName,
		AssetBody:       []byte("fake-binary"),
		IncludeAsset:    true,
		IncludeChecksum: true,
		ChecksumBody:    strings.Repeat("0", 64) + "  " + assetName + "\n",
	})
	defer server.Close()

	var out bytes.Buffer
	err := tuiInstallFrom(&out, server.URL, "o/r", "v1.2.3")
	if err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("error = %v, want checksum mismatch", err)
	}
	if strings.Contains(err.Error()+out.String(), "test-token") {
		t.Fatalf("token leaked in error output: %q %q", err, out.String())
	}
	destDir := filepath.Join(home, "bin")
	if _, err := os.Stat(filepath.Join(destDir, tuiBinaryName)); !os.IsNotExist(err) {
		t.Fatalf("destination exists after mismatch: %v", err)
	}
	entries, err := os.ReadDir(destDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("leftover files after mismatch: %v", entries)
	}
}

func TestTUIInstallNoChecksumCoveragePrintsSkippingVerification(t *testing.T) {
	_, assetName := tuiInstallTestEnv(t)
	server := tuiInstallTestServer(t, tuiInstallTestOptions{
		AssetName:    assetName,
		AssetBody:    []byte("fake-binary"),
		IncludeAsset: true,
	})
	defer server.Close()

	var out bytes.Buffer
	if err := tuiInstallFrom(&out, server.URL, "o/r", "v1.2.3"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "skipping verification") {
		t.Fatalf("output = %q, want skipping verification", out.String())
	}
}

func TestTUIInstallMissingAssetNamesAssetAndPlatform(t *testing.T) {
	_, assetName := tuiInstallTestEnv(t)
	server := tuiInstallTestServer(t, tuiInstallTestOptions{})
	defer server.Close()

	var out bytes.Buffer
	err := tuiInstallFrom(&out, server.URL, "o/r", "v1.2.3")
	if err == nil {
		t.Fatal("expected missing asset error")
	}
	if !strings.Contains(err.Error(), assetName) || !strings.Contains(err.Error(), runtime.GOOS+"/"+runtime.GOARCH) {
		t.Fatalf("error = %q", err)
	}
}

func TestTUIInstallSendsAuthHeader(t *testing.T) {
	_, assetName := tuiInstallTestEnv(t)
	body := []byte("fake-binary")
	sum := sha256.Sum256(body)
	var sawAuth atomic.Bool
	server := tuiInstallTestServer(t, tuiInstallTestOptions{
		AssetName:       assetName,
		AssetBody:       body,
		IncludeAsset:    true,
		IncludeChecksum: true,
		ChecksumBody:    fmt.Sprintf("%x  %s\n", sum, assetName),
		RequireAuth:     true,
		SawAuth:         &sawAuth,
	})
	defer server.Close()

	var out bytes.Buffer
	if err := tuiInstallFrom(&out, server.URL, "o/r", "v1.2.3"); err != nil {
		t.Fatal(err)
	}
	if !sawAuth.Load() {
		t.Fatal("authorization header was not received")
	}
}

func TestTUIInstallUnsupportedPlatform(t *testing.T) {
	if _, err := tuiAssetName("windows", "amd64"); err == nil || !strings.Contains(err.Error(), "no devspace-tui build for windows/amd64") {
		t.Fatalf("error = %v", err)
	}
}

func TestTUIInstallDevVersionRequiresVersionFlag(t *testing.T) {
	t.Setenv("DEVSPACE_HOME", t.TempDir())
	cmd := newTUIInstallCommand("dev")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "--version") {
		t.Fatalf("error = %v, want --version", err)
	}
}

type tuiInstallTestOptions struct {
	AssetName       string
	AssetBody       []byte
	IncludeAsset    bool
	IncludeChecksum bool
	ChecksumBody    string
	RequireAuth     bool
	SawAuth         *atomic.Bool
}

func tuiInstallTestEnv(t *testing.T) (string, string) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("DEVSPACE_HOME", home)
	t.Setenv("GITHUB_TOKEN", "test-token")
	t.Setenv("GH_TOKEN", "")
	assetName, err := tuiAssetName(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		t.Skip(err)
	}
	return home, assetName
}

func tuiInstallTestServer(t *testing.T, opts tuiInstallTestOptions) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	mux.HandleFunc("/repos/o/r/releases/tags/v1.2.3", func(w http.ResponseWriter, r *http.Request) {
		tuiInstallAssertAuth(t, r, opts)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"assets":[`)
		wrote := false
		if opts.IncludeAsset {
			fmt.Fprintf(w, `{"name":%q,"url":%q}`, opts.AssetName, server.URL+"/asset")
			wrote = true
		}
		if opts.IncludeChecksum {
			if wrote {
				fmt.Fprint(w, `,`)
			}
			fmt.Fprintf(w, `{"name":"checksums.txt","url":%q}`, server.URL+"/checksums")
		}
		fmt.Fprint(w, `]}`)
	})
	mux.HandleFunc("/asset", func(w http.ResponseWriter, r *http.Request) {
		tuiInstallAssertAuth(t, r, opts)
		if r.Header.Get("Accept") != "application/octet-stream" {
			t.Errorf("asset Accept = %q", r.Header.Get("Accept"))
		}
		_, _ = w.Write(opts.AssetBody)
	})
	mux.HandleFunc("/checksums", func(w http.ResponseWriter, r *http.Request) {
		tuiInstallAssertAuth(t, r, opts)
		if r.Header.Get("Accept") != "application/octet-stream" {
			t.Errorf("checksums Accept = %q", r.Header.Get("Accept"))
		}
		fmt.Fprint(w, opts.ChecksumBody)
	})
	return server
}

func tuiInstallAssertAuth(t *testing.T, r *http.Request, opts tuiInstallTestOptions) {
	t.Helper()
	if !opts.RequireAuth {
		return
	}
	if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
		t.Errorf("Authorization = %q", got)
		return
	}
	opts.SawAuth.Store(true)
}
