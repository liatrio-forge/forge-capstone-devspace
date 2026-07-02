package devspace

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHostedConfigSetGetStoresEndpointTokenAndWorkspace(t *testing.T) {
	hardeningInitWorkspace(t, "code")

	cfg, err := SetHostedSync("http://127.0.0.1:8787/", "test-token", "team-a")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.HostedSyncEndpoint != "http://127.0.0.1:8787" {
		t.Fatalf("endpoint = %q", cfg.HostedSyncEndpoint)
	}
	if cfg.HostedSyncToken != "test-token" {
		t.Fatal("hosted token was not stored")
	}
	if cfg.HostedSyncWorkspace != "team-a" {
		t.Fatalf("workspace = %q", cfg.HostedSyncWorkspace)
	}
	got, err := GetHostedSync()
	if err != nil {
		t.Fatal(err)
	}
	if got.HostedSyncEndpoint != cfg.HostedSyncEndpoint || got.HostedSyncToken != cfg.HostedSyncToken || got.HostedSyncWorkspace != cfg.HostedSyncWorkspace {
		t.Fatalf("get hosted config = %+v", got)
	}
}

func TestHostedConfigRejectsUnsafeWorkspaceID(t *testing.T) {
	hardeningInitWorkspace(t, "code")

	_, err := SetHostedSync("http://127.0.0.1:8787", "test-token", "../bad")
	if err == nil || !strings.Contains(err.Error(), "unsupported character") {
		t.Fatalf("unsafe workspace error = %v", err)
	}
}

func TestHostedPushStoresOnlyNormalizedManifestAndRecordsVersion(t *testing.T) {
	workspace := hardeningInitWorkspace(t, "code")
	server := hostedSyncTestServer(t)
	if _, err := SetHostedSync(server.URL, "test-token", "team-a"); err != nil {
		t.Fatal(err)
	}
	if err := SaveManifest(workspace, Manifest{
		Version:       ManifestVersion,
		WorkspaceRoot: workspace,
		Projects:      []Project{hardeningProject("apps/app", ProjectTypeLocal, "")},
	}); err != nil {
		t.Fatal(err)
	}

	result, err := PushHostedManifest()
	if err != nil {
		t.Fatal(err)
	}
	if !result.Changed || result.Version != 1 || result.ManifestHash == "" {
		t.Fatalf("push result = %+v", result)
	}
	envelope := hostedSyncGet(t, server.URL, "team-a")
	if envelope.Manifest.WorkspaceRoot != "." {
		t.Fatalf("hosted manifest was not normalized: %q", envelope.Manifest.WorkspaceRoot)
	}
	data, err := json.Marshal(envelope)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(data, []byte(workspace)) {
		t.Fatalf("hosted envelope leaked local workspace path:\n%s", data)
	}
	st, err := LoadState()
	if err != nil {
		t.Fatal(err)
	}
	if st.HostedSyncVersion != 1 || st.HostedSyncManifestHash != result.ManifestHash {
		t.Fatalf("hosted sync baseline not recorded: %+v", st)
	}
}

func TestHostedServerRejectsUnsafeProjectPaths(t *testing.T) {
	server := hostedSyncTestServer(t)
	unsafe := Manifest{
		Version:       ManifestVersion,
		WorkspaceRoot: ".",
		Projects:      []Project{hardeningProject("node_modules/left-pad", ProjectTypeLocal, "")},
	}
	resp := hostedSyncPutRaw(t, server.URL, "team-a", hostedManifestPutRequest{ExpectedVersion: 0, Manifest: unsafe})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d", resp.StatusCode)
	}
}

func TestHostedPushDetectsRemoteVersionConflict(t *testing.T) {
	workspace := hardeningInitWorkspace(t, "code")
	server := hostedSyncTestServer(t)
	if _, err := SetHostedSync(server.URL, "test-token", "team-a"); err != nil {
		t.Fatal(err)
	}
	if err := SaveManifest(workspace, Manifest{
		Version:       ManifestVersion,
		WorkspaceRoot: workspace,
		Projects:      []Project{hardeningProject("apps/app", ProjectTypeLocal, "")},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := PushHostedManifest(); err != nil {
		t.Fatal(err)
	}
	remoteChange := Manifest{
		Version:       ManifestVersion,
		WorkspaceRoot: ".",
		Projects:      []Project{hardeningProject("apps/remote-change", ProjectTypeLocal, "")},
	}
	resp := hostedSyncPutRaw(t, server.URL, "team-a", hostedManifestPutRequest{ExpectedVersion: 1, Manifest: remoteChange})
	if resp.Body != nil {
		_ = resp.Body.Close()
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("remote change status = %d", resp.StatusCode)
	}
	localChange := Manifest{
		Version:       ManifestVersion,
		WorkspaceRoot: workspace,
		Projects:      []Project{hardeningProject("apps/local-change", ProjectTypeLocal, "")},
	}
	if err := SaveManifest(workspace, localChange); err != nil {
		t.Fatal(err)
	}

	_, err := PushHostedManifest()
	if err == nil || !strings.Contains(err.Error(), "changed since last sync") {
		t.Fatalf("push conflict error = %v", err)
	}
}

func TestHostedPullRefusesToOverwriteLocalChanges(t *testing.T) {
	workspace := hardeningInitWorkspace(t, "code")
	server := hostedSyncTestServer(t)
	if _, err := SetHostedSync(server.URL, "test-token", "team-a"); err != nil {
		t.Fatal(err)
	}
	initial := Manifest{
		Version:       ManifestVersion,
		WorkspaceRoot: workspace,
		Projects:      []Project{hardeningProject("apps/app", ProjectTypeLocal, "")},
	}
	if err := SaveManifest(workspace, initial); err != nil {
		t.Fatal(err)
	}
	if _, err := PushHostedManifest(); err != nil {
		t.Fatal(err)
	}
	remoteChange := Manifest{
		Version:       ManifestVersion,
		WorkspaceRoot: ".",
		Projects:      []Project{hardeningProject("apps/remote-change", ProjectTypeLocal, "")},
	}
	resp := hostedSyncPutRaw(t, server.URL, "team-a", hostedManifestPutRequest{ExpectedVersion: 1, Manifest: remoteChange})
	if resp.Body != nil {
		_ = resp.Body.Close()
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("remote change status = %d", resp.StatusCode)
	}
	localChange := Manifest{
		Version:       ManifestVersion,
		WorkspaceRoot: workspace,
		Projects:      []Project{hardeningProject("apps/local-change", ProjectTypeLocal, "")},
	}
	if err := SaveManifest(workspace, localChange); err != nil {
		t.Fatal(err)
	}

	_, err := PullHostedManifest()
	if err == nil || !strings.Contains(err.Error(), "local manifest changed") {
		t.Fatalf("pull conflict error = %v", err)
	}
	after, err := LoadManifest(workspace)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := findProject(after, "apps/local-change"); !ok {
		t.Fatalf("local changes were overwritten: %+v", after.Projects)
	}
}

func TestHostedPullLocalizesManifestForSecondWorkspace(t *testing.T) {
	root := t.TempDir()
	server := hostedSyncTestServer(t)
	workspaceA := filepath.Join(root, "machine-a", "code")
	workspaceB := filepath.Join(root, "machine-b", "code")

	t.Setenv(envHome, filepath.Join(root, "home-a"))
	if _, err := InitWorkspace(workspaceA); err != nil {
		t.Fatal(err)
	}
	if _, err := SetHostedSync(server.URL, "test-token", "team-a"); err != nil {
		t.Fatal(err)
	}
	if err := SaveManifest(workspaceA, Manifest{
		Version:       ManifestVersion,
		WorkspaceRoot: workspaceA,
		Projects:      []Project{hardeningProject("apps/app", ProjectTypeLocal, "")},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := PushHostedManifest(); err != nil {
		t.Fatal(err)
	}

	t.Setenv(envHome, filepath.Join(root, "home-b"))
	if _, err := InitWorkspace(workspaceB); err != nil {
		t.Fatal(err)
	}
	if _, err := SetHostedSync(server.URL, "test-token", "team-a"); err != nil {
		t.Fatal(err)
	}
	if _, err := PullHostedManifest(); err != nil {
		t.Fatal(err)
	}
	pulled, err := LoadManifest(workspaceB)
	if err != nil {
		t.Fatal(err)
	}
	if pulled.WorkspaceRoot != workspaceB {
		t.Fatalf("workspace root was not localized: %s", pulled.WorkspaceRoot)
	}
	if _, ok := findProject(pulled, "apps/app"); !ok {
		t.Fatalf("project missing after pull: %+v", pulled.Projects)
	}
	if _, err := os.ReadFile(manifestPath(workspaceB) + ".bak"); err != nil {
		t.Fatalf("pull did not back up previous manifest: %v", err)
	}
}

func TestHostedServerHealthzOkWithoutAuth(t *testing.T) {
	server := hostedSyncTestServer(t)

	resp, err := http.Get(server.URL + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "ok" {
		t.Fatalf("body = %q", body)
	}
}

func TestHostedServerHealthzDoesNotOpenAuthHole(t *testing.T) {
	server := hostedSyncTestServer(t)

	req, err := http.NewRequest(http.MethodGet, server.URL+"/v1/workspaces/team-a/manifest", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d", resp.StatusCode)
	}

	reqBadAuth, err := http.NewRequest(http.MethodGet, server.URL+"/v1/workspaces/team-a/manifest", nil)
	if err != nil {
		t.Fatal(err)
	}
	reqBadAuth.Header.Set("Authorization", "Bearer wrong-token")
	respBadAuth, err := http.DefaultClient.Do(reqBadAuth)
	if err != nil {
		t.Fatal(err)
	}
	defer respBadAuth.Body.Close()
	if respBadAuth.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d", respBadAuth.StatusCode)
	}
}

func hostedSyncTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	handler, err := NewHostedSyncServer(HostedSyncServerOptions{StoreDir: t.TempDir(), Token: "test-token"})
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return server
}

func hostedSyncGet(t *testing.T, baseURL, workspace string) hostedManifestEnvelope {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, baseURL+"/v1/workspaces/"+workspace+"/manifest", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer test-token")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("get status = %d", resp.StatusCode)
	}
	var envelope hostedManifestEnvelope
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		t.Fatal(err)
	}
	return envelope
}

func hostedSyncPutRaw(t *testing.T, baseURL, workspace string, reqBody hostedManifestPutRequest) *http.Response {
	t.Helper()
	data, err := json.Marshal(reqBody)
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest(http.MethodPut, baseURL+"/v1/workspaces/"+workspace+"/manifest", bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer test-token")
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}
