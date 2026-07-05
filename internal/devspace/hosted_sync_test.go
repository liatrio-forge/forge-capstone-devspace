package devspace

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
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

func TestHostedServerRateLimiterMapIsBounded(t *testing.T) {
	handler, err := NewHostedSyncServer(HostedSyncServerOptions{StoreDir: t.TempDir(), Token: "test-token", MaxLimiters: 8})
	if err != nil {
		t.Fatal(err)
	}
	s := handler.(*hostedSyncServer)
	for i := 0; i < 100; i++ {
		r := httptest.NewRequest(http.MethodGet, "/v1/workspaces/x/manifest", nil)
		r.RemoteAddr = "203.0.113." + strconv.Itoa(i) + ":9999"
		s.allowRequest(r)
	}
	s.limiterMu.Lock()
	n := len(s.limiters)
	s.limiterMu.Unlock()
	if n > 8 {
		t.Fatalf("per-IP limiter map grew past cap: %d entries (max 8)", n)
	}
}

func TestHostedServerWorkspaceLocksAreBounded(t *testing.T) {
	handler, err := NewHostedSyncServer(HostedSyncServerOptions{StoreDir: t.TempDir(), Token: "test-token"})
	if err != nil {
		t.Fatal(err)
	}
	s := handler.(*hostedSyncServer)

	seen := map[*sync.Mutex]bool{}
	first := s.workspaceMutex("workspace-0")
	if again := s.workspaceMutex("workspace-0"); again != first {
		t.Fatalf("workspaceMutex returned different pointers for the same workspace ID: %p vs %p", first, again)
	}
	for i := 0; i < 10000; i++ {
		seen[s.workspaceMutex("workspace-"+strconv.Itoa(i))] = true
	}
	if n := len(seen); n > 256 {
		t.Fatalf("workspace mutex stripes grew past the fixed array size: %d distinct pointers (max 256)", n)
	}
}

func TestHostedServerClientIPRespectsTrustedXFF(t *testing.T) {
	_, trusted, err := net.ParseCIDR("10.0.0.0/8")
	if err != nil {
		t.Fatal(err)
	}
	handler, err := NewHostedSyncServer(HostedSyncServerOptions{
		StoreDir:       t.TempDir(),
		Token:          "test-token",
		TrustedProxies: []*net.IPNet{trusted},
	})
	if err != nil {
		t.Fatal(err)
	}
	s := handler.(*hostedSyncServer)

	// Proxy peer (10.x) carries XFF naming three distinct clients; the
	// rightmost untrusted entry is the real client and should key the limiter.
	for _, client := range []string{"198.51.100.1", "198.51.100.2", "198.51.100.3"} {
		r := httptest.NewRequest(http.MethodGet, "/v1/workspaces/x/manifest", nil)
		r.RemoteAddr = "10.1.2.3:9999"
		r.Header.Set("X-Forwarded-For", client)
		s.allowRequest(r)
	}
	s.limiterMu.Lock()
	n := len(s.limiters)
	s.limiterMu.Unlock()
	if n != 3 {
		t.Fatalf("expected 3 distinct client-IP limiters via trusted XFF, got %d", n)
	}
}

func TestHostedServerClientIPIgnoresUntrustedXFF(t *testing.T) {
	// No trusted proxies configured: XFF must be ignored entirely and every
	// request keyed on the peer IP, so spoofed XFF values cannot inflate the
	// limiter map.
	handler, err := NewHostedSyncServer(HostedSyncServerOptions{StoreDir: t.TempDir(), Token: "test-token"})
	if err != nil {
		t.Fatal(err)
	}
	s := handler.(*hostedSyncServer)
	for i := 0; i < 20; i++ {
		r := httptest.NewRequest(http.MethodGet, "/v1/workspaces/x/manifest", nil)
		r.RemoteAddr = "203.0.113.7:9999"
		r.Header.Set("X-Forwarded-For", "198.51.100."+strconv.Itoa(i))
		s.allowRequest(r)
	}
	s.limiterMu.Lock()
	n := len(s.limiters)
	s.limiterMu.Unlock()
	if n != 1 {
		t.Fatalf("untrusted XFF must not create distinct limiters: got %d, want 1", n)
	}
}

func TestHostedServerClientIPWalksPastTrustedHops(t *testing.T) {
	// Chain: client 198.51.100.9 -> trusted proxy 10.0.0.1 -> trusted proxy
	// 10.0.0.2 -> this server. XFF lists both proxies; the server must walk
	// past the trusted hops to the untrusted originating client.
	_, trusted, err := net.ParseCIDR("10.0.0.0/8")
	if err != nil {
		t.Fatal(err)
	}
	handler, err := NewHostedSyncServer(HostedSyncServerOptions{
		StoreDir:       t.TempDir(),
		Token:          "test-token",
		TrustedProxies: []*net.IPNet{trusted},
	})
	if err != nil {
		t.Fatal(err)
	}
	s := handler.(*hostedSyncServer)
	r := httptest.NewRequest(http.MethodGet, "/v1/workspaces/x/manifest", nil)
	r.RemoteAddr = "10.0.0.2:9999"
	r.Header.Set("X-Forwarded-For", "198.51.100.9, 10.0.0.1")
	s.allowRequest(r)
	s.limiterMu.Lock()
	defer s.limiterMu.Unlock()
	_, ok := s.limiters["198.51.100.9"]
	if !ok {
		t.Fatalf("expected limiter keyed on originating client 198.51.100.9; have keys %v", limiterKeysLocked(s))
	}
}

func limiterKeysLocked(s *hostedSyncServer) []string {
	keys := make([]string, 0, len(s.limiters))
	for k := range s.limiters {
		keys = append(keys, k)
	}
	return keys
}

func TestParseTrustedProxyCIDRs(t *testing.T) {
	cidrs, err := parseTrustedProxyCIDRs([]string{"10.0.0.0/8", "  192.168.1.5 ", "", "fe80::/64"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cidrs) != 3 {
		t.Fatalf("expected 3 CIDRs (blank ignored), got %d", len(cidrs))
	}
	// A bare IP becomes a single-host network.
	bare, err := parseTrustedProxyCIDRs([]string{"203.0.113.42"})
	if err != nil {
		t.Fatalf("bare IP parse error: %v", err)
	}
	if !bare[0].IP.Equal(net.ParseIP("203.0.113.42").To4()) {
		t.Fatalf("bare IP not normalized: %v", bare[0].IP)
	}
	if _, err := parseTrustedProxyCIDRs([]string{"not-a-cidr"}); err == nil {
		t.Fatal("expected error for invalid CIDR/IP")
	}
}

func TestSetHostedSyncRejectsPlainHTTPForNonLoopbackHost(t *testing.T) {
	hardeningInitWorkspace(t, "code")

	_, err := SetHostedSync("http://evil.example.com", "test-token", "team-a")
	if err == nil || !strings.Contains(err.Error(), "must use https") {
		t.Fatalf("plain http to public host error = %v", err)
	}
}

func TestSetHostedSyncAllowsHTTPSForNonLoopbackHost(t *testing.T) {
	hardeningInitWorkspace(t, "code")

	cfg, err := SetHostedSync("https://evil.example.com", "test-token", "team-a")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.HostedSyncEndpoint != "https://evil.example.com" {
		t.Fatalf("endpoint = %q", cfg.HostedSyncEndpoint)
	}
}

func TestSetHostedSyncAllowsPlainHTTPForLoopbackHost(t *testing.T) {
	hardeningInitWorkspace(t, "code")

	cfg, err := SetHostedSync("http://127.0.0.1:8787", "test-token", "team-a")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.HostedSyncEndpoint != "http://127.0.0.1:8787" {
		t.Fatalf("endpoint = %q", cfg.HostedSyncEndpoint)
	}
}

func TestGetHostedSyncRejectsPlainHTTPEndpoint(t *testing.T) {
	workspace := hardeningInitWorkspace(t, "code")
	if err := SaveConfig(Config{
		WorkspaceRoot:       workspace,
		HostedSyncEndpoint:  "http://example.com",
		HostedSyncToken:     "test-token",
		HostedSyncWorkspace: "team-a",
	}); err != nil {
		t.Fatal(err)
	}

	_, err := GetHostedSync()
	if err == nil || !strings.Contains(err.Error(), "must use https") {
		t.Fatalf("plain http configured endpoint error = %v", err)
	}
}

func TestGetHostedSyncAllowsLoopbackHTTP(t *testing.T) {
	workspace := hardeningInitWorkspace(t, "code")
	if err := SaveConfig(Config{
		WorkspaceRoot:       workspace,
		HostedSyncEndpoint:  "http://127.0.0.1:8787",
		HostedSyncToken:     "test-token",
		HostedSyncWorkspace: "team-a",
	}); err != nil {
		t.Fatal(err)
	}

	cfg, err := GetHostedSync()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.HostedSyncEndpoint != "http://127.0.0.1:8787" {
		t.Fatalf("endpoint = %q", cfg.HostedSyncEndpoint)
	}
}

func TestHostedConfigSetReadsTokenFromEnv(t *testing.T) {
	hardeningInitWorkspace(t, "code")
	t.Setenv("DEVSPACE_HOSTED_TOKEN", "env-token")

	if _, _, err := executeCommand(t, "test", "hosted", "config", "set", "https://example.com"); err != nil {
		t.Fatal(err)
	}

	cfg, err := GetHostedSync()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.HostedSyncToken != "env-token" {
		t.Fatal("hosted sync token was not read from DEVSPACE_HOSTED_TOKEN")
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

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL+"/healthz", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
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

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL+"/v1/workspaces/team-a/manifest", nil)
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

	reqBadAuth, err := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL+"/v1/workspaces/team-a/manifest", nil)
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

func TestHostedServerAuthConstantTimeCompareRejectsInvalidTokens(t *testing.T) {
	server := hostedSyncTestServer(t)

	cases := []struct {
		name   string
		header string
	}{
		{"missing", ""},
		{"wrong token", "Bearer wrong-token"},
		{"wrong scheme", "Basic dGVzdC10b2tlbg=="},
		{"empty bearer", "Bearer "},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL+"/v1/workspaces/team-a/manifest", nil)
			if err != nil {
				t.Fatal(err)
			}
			if tc.header != "" {
				req.Header.Set("Authorization", tc.header)
			}
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusUnauthorized {
				t.Fatalf("status = %d", resp.StatusCode)
			}
		})
	}
}

func TestHostedServerPutSerializesConcurrentRequestsPerWorkspace(t *testing.T) {
	server := hostedSyncTestServer(t)
	manifest := Manifest{
		Version:       ManifestVersion,
		WorkspaceRoot: ".",
		Projects:      []Project{hardeningProject("apps/app", ProjectTypeLocal, "")},
	}
	body, err := json.Marshal(hostedManifestPutRequest{ExpectedVersion: 0, Manifest: manifest})
	if err != nil {
		t.Fatal(err)
	}

	const attempts = 2
	statuses := make([]int, attempts)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < attempts; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			<-start
			req, err := http.NewRequestWithContext(context.Background(), http.MethodPut, server.URL+"/v1/workspaces/concurrent/manifest", bytes.NewReader(body))
			if err != nil {
				t.Error(err)
				return
			}
			req.Header.Set("Authorization", "Bearer test-token")
			req.Header.Set("Content-Type", "application/json")
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Error(err)
				return
			}
			defer resp.Body.Close()
			statuses[idx] = resp.StatusCode
		}(i)
	}
	close(start)
	wg.Wait()

	var ok, conflict int
	for _, code := range statuses {
		switch code {
		case http.StatusOK:
			ok++
		case http.StatusConflict:
			conflict++
		default:
			t.Fatalf("unexpected status %d in %v", code, statuses)
		}
	}
	if ok != 1 {
		t.Fatalf("expected exactly one 200, got %d (statuses=%v)", ok, statuses)
	}
	if conflict != 1 {
		t.Fatalf("expected exactly one 409, got %d (statuses=%v)", conflict, statuses)
	}

	final := hostedSyncGet(t, server.URL, "concurrent")
	if final.Version != 1 {
		t.Fatalf("final version = %d, want 1 (no lost update)", final.Version)
	}
}

func TestHostedServerRateLimitReturns429ButExemptsHealthz(t *testing.T) {
	server := hostedSyncTestServerWithOptions(t, HostedSyncServerOptions{RateLimit: 1, RateBurst: 1})

	got429 := false
	for i := 0; i < 10; i++ {
		req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL+"/v1/workspaces/team-a/manifest", nil)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Authorization", "Bearer test-token")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()
		if resp.StatusCode == http.StatusTooManyRequests {
			got429 = true
			break
		}
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("unexpected status %d before rate limit tripped", resp.StatusCode)
		}
	}
	if !got429 {
		t.Fatal("expected at least one 429 response under a tiny rate limit")
	}

	for i := 0; i < 20; i++ {
		req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL+"/healthz", nil)
		if err != nil {
			t.Fatal(err)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("healthz status = %d, want 200 (must never be rate limited)", resp.StatusCode)
		}
	}
}

func TestRunHostedSyncServerShutsDownCleanlyWhenContextIsCanceled(t *testing.T) {
	handler, err := NewHostedSyncServer(HostedSyncServerOptions{StoreDir: t.TempDir(), Token: "test-token"})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errCh := make(chan error, 1)
	ready := make(chan string, 1)
	go func() {
		errCh <- RunHostedSyncServer(ctx, HostedSyncServeOptions{
			Addr:              "127.0.0.1:0",
			Handler:           handler,
			DiagnosticsWriter: io.Discard,
			ready:             ready,
		})
	}()

	var addr string
	select {
	case addr = <-ready:
	case err := <-errCh:
		t.Fatalf("server exited before listening: %v", err)
	case <-time.After(2 * time.Second):
		t.Fatal("server did not start listening")
	}

	deadline := time.Now().Add(2 * time.Second)
	probeCtx, cancelProbe := context.WithDeadline(context.Background(), deadline)
	defer cancelProbe()
	for {
		attemptCtx, cancelAttempt := context.WithTimeout(probeCtx, 100*time.Millisecond)
		req, err := http.NewRequestWithContext(attemptCtx, http.MethodGet, "http://"+addr+"/healthz", nil)
		if err != nil {
			cancelAttempt()
			t.Fatal(err)
		}
		resp, err := http.DefaultClient.Do(req)
		cancelAttempt()
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				break
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("server did not become ready: %v", err)
		}
		time.Sleep(10 * time.Millisecond)
	}

	timer := time.AfterFunc(100*time.Millisecond, cancel)
	defer timer.Stop()
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("server did not shut down")
	}

	deadline = time.Now().Add(2 * time.Second)
	for {
		conn, err := net.DialTimeout("tcp", addr, 50*time.Millisecond)
		if err != nil {
			return
		}
		_ = conn.Close()
		if time.Now().After(deadline) {
			t.Fatal("server port remained open after shutdown")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func hostedSyncTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	return hostedSyncTestServerWithOptions(t, HostedSyncServerOptions{})
}

func hostedSyncTestServerWithOptions(t *testing.T, opts HostedSyncServerOptions) *httptest.Server {
	t.Helper()
	if opts.StoreDir == "" {
		opts.StoreDir = t.TempDir()
	}
	if opts.Token == "" {
		opts.Token = "test-token"
	}
	handler, err := NewHostedSyncServer(opts)
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
