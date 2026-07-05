package devspace

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

const hostedManifestAPIVersion = "v1"

var hostedUnsafePathComponents = map[string]bool{
	".env":         true,
	".git":         true,
	"node_modules": true,
	"dist":         true,
	"build":        true,
	".next":        true,
	"turbo":        true,
	"target":       true,
	"vendor":       true,
	"coverage":     true,
	".cache":       true,
}

type HostedSyncResult struct {
	Changed      bool
	Version      int
	ManifestHash string
}

type hostedManifestEnvelope struct {
	APIVersion   string   `json:"apiVersion"`
	Workspace    string   `json:"workspace"`
	Version      int      `json:"version"`
	ManifestHash string   `json:"manifestHash"`
	UpdatedAt    string   `json:"updatedAt"`
	Manifest     Manifest `json:"manifest"`
}

type hostedManifestPutRequest struct {
	ExpectedVersion int      `json:"expectedVersion"`
	Manifest        Manifest `json:"manifest"`
}

func SetHostedSync(endpoint, token, workspace string) (Config, error) {
	endpoint = strings.TrimSpace(endpoint)
	token = strings.TrimSpace(token)
	workspace = strings.TrimSpace(workspace)
	if endpoint == "" {
		return Config{}, fmt.Errorf("hosted sync endpoint is required")
	}
	if token == "" {
		return Config{}, fmt.Errorf("hosted sync auth token is required")
	}
	if workspace == "" {
		workspace = "default"
	}
	if err := validateHostedWorkspaceID(workspace); err != nil {
		return Config{}, err
	}
	if err := validateHostedEndpoint(endpoint); err != nil {
		return Config{}, err
	}
	cfg, err := LoadConfig()
	if err != nil {
		return Config{}, err
	}
	cfg.HostedSyncEndpoint = strings.TrimRight(endpoint, "/")
	cfg.HostedSyncToken = token
	cfg.HostedSyncWorkspace = workspace
	cfg.UpdatedAt = nowRFC3339()
	if err := SaveConfig(cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// validateHostedEndpoint enforces the HTTPS-only rule (plain http is only
// allowed for loopback endpoints) so it can be checked both at config-write
// time (SetHostedSync) and at point of use (GetHostedSync) — anything that
// puts a non-loopback http:// endpoint into config.json outside the CLI's
// own setter must still be caught before a sync sends the bearer token and
// manifest over plaintext.
func validateHostedEndpoint(endpoint string) error {
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return fmt.Errorf("hosted sync endpoint must be an absolute http(s) URL")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("hosted sync endpoint must use http or https")
	}
	if parsed.Scheme == "http" && !isLoopbackHost(parsed.Hostname()) {
		return fmt.Errorf("hosted sync endpoint must use https (plain http is only allowed for localhost)")
	}
	return nil
}

func GetHostedSync() (Config, error) {
	cfg, err := LoadConfig()
	if err != nil {
		return Config{}, err
	}
	if strings.TrimSpace(cfg.HostedSyncEndpoint) == "" {
		return Config{}, fmt.Errorf("no hosted sync endpoint configured; run `devspace hosted config set <endpoint> --token <token>` or use Git-backed `devspace workspace push/pull`")
	}
	if err := validateHostedEndpoint(cfg.HostedSyncEndpoint); err != nil {
		return Config{}, fmt.Errorf("configured hosted sync endpoint is invalid: %w; re-run `devspace hosted config set`", err)
	}
	if strings.TrimSpace(cfg.HostedSyncToken) == "" {
		return Config{}, fmt.Errorf("no hosted sync auth token configured; run `devspace hosted config set <endpoint> --token <token>`")
	}
	if strings.TrimSpace(cfg.HostedSyncWorkspace) == "" {
		cfg.HostedSyncWorkspace = "default"
		if err := SaveConfig(cfg); err != nil {
			return Config{}, err
		}
	}
	if err := validateHostedWorkspaceID(cfg.HostedSyncWorkspace); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func PushHostedManifest() (HostedSyncResult, error) {
	cfg, err := GetHostedSync()
	if err != nil {
		return HostedSyncResult{}, err
	}
	local, err := LoadManifest(cfg.WorkspaceRoot)
	if err != nil {
		return HostedSyncResult{}, err
	}
	normalized := manifestForSync(local)
	if err := validateHostedManifest(normalized); err != nil {
		return HostedSyncResult{}, err
	}
	localHash, err := hostedManifestHash(normalized)
	if err != nil {
		return HostedSyncResult{}, err
	}
	client := newHostedClient(cfg)
	remote, hasRemote, err := client.get(context.Background())
	if err != nil {
		return HostedSyncResult{}, err
	}
	st, err := LoadState()
	if err != nil && !missing(err) {
		return HostedSyncResult{}, err
	}
	if hasRemote {
		if remote.ManifestHash == localHash {
			if err := recordHostedSync(remote.Version, remote.ManifestHash); err != nil {
				return HostedSyncResult{}, err
			}
			return HostedSyncResult{Changed: false, Version: remote.Version, ManifestHash: remote.ManifestHash}, nil
		}
		if st.HostedSyncVersion == 0 {
			return HostedSyncResult{}, fmt.Errorf("hosted manifest already exists and differs from local manifest; run `devspace hosted pull` and reconcile before pushing")
		}
		if remote.Version != st.HostedSyncVersion {
			return HostedSyncResult{}, fmt.Errorf("hosted manifest changed since last sync; run `devspace hosted pull` and reconcile before pushing")
		}
	} else if st.HostedSyncVersion != 0 {
		return HostedSyncResult{}, fmt.Errorf("hosted manifest is missing but local state expected version %d; reconcile hosted sync config before pushing", st.HostedSyncVersion)
	}

	expected := 0
	if hasRemote {
		expected = remote.Version
	}
	updated, err := client.put(context.Background(), expected, normalized)
	if err != nil {
		return HostedSyncResult{}, err
	}
	if err := recordHostedSync(updated.Version, updated.ManifestHash); err != nil {
		return HostedSyncResult{}, err
	}
	return HostedSyncResult{Changed: true, Version: updated.Version, ManifestHash: updated.ManifestHash}, nil
}

func PullHostedManifest() (HostedSyncResult, error) {
	cfg, err := GetHostedSync()
	if err != nil {
		return HostedSyncResult{}, err
	}
	client := newHostedClient(cfg)
	remote, hasRemote, err := client.get(context.Background())
	if err != nil {
		return HostedSyncResult{}, err
	}
	if !hasRemote {
		return HostedSyncResult{}, fmt.Errorf("hosted manifest not found for workspace %q; push one first or use Git-backed `devspace workspace pull`", cfg.HostedSyncWorkspace)
	}
	localized := localizeSyncedManifest(remote.Manifest, cfg)
	if err := validateHostedManifest(localized); err != nil {
		return HostedSyncResult{}, fmt.Errorf("hosted manifest failed validation: %w", err)
	}
	current, err := LoadManifest(cfg.WorkspaceRoot)
	if err != nil && !missing(err) {
		return HostedSyncResult{}, err
	}
	before, err := os.ReadFile(manifestPath(cfg.WorkspaceRoot))
	if err != nil && !missing(err) {
		return HostedSyncResult{}, err
	}
	if err == nil {
		localHash, hashErr := hostedManifestHash(manifestForSync(current))
		if hashErr != nil {
			return HostedSyncResult{}, hashErr
		}
		st, stErr := LoadState()
		if stErr != nil && !missing(stErr) {
			return HostedSyncResult{}, stErr
		}
		if localHash != remote.ManifestHash {
			if st.HostedSyncManifestHash == "" {
				if len(current.Projects) > 0 {
					return HostedSyncResult{}, fmt.Errorf("local manifest differs from hosted manifest and no hosted sync baseline exists; reconcile before pulling")
				}
			} else if localHash != st.HostedSyncManifestHash {
				return HostedSyncResult{}, fmt.Errorf("local manifest changed since last hosted sync; push or reconcile local changes before pulling")
			}
		}
	}
	if err := SaveManifest(cfg.WorkspaceRoot, localized); err != nil {
		return HostedSyncResult{}, err
	}
	after, err := os.ReadFile(manifestPath(cfg.WorkspaceRoot))
	if err != nil {
		return HostedSyncResult{}, err
	}
	if err := recordHostedSync(remote.Version, remote.ManifestHash); err != nil {
		return HostedSyncResult{}, err
	}
	return HostedSyncResult{Changed: !bytes.Equal(before, after), Version: remote.Version, ManifestHash: remote.ManifestHash}, nil
}

type hostedClient struct {
	httpClient *http.Client
	cfg        Config
}

func newHostedClient(cfg Config) hostedClient {
	return hostedClient{httpClient: &http.Client{Timeout: 30 * time.Second}, cfg: cfg}
}

func (c hostedClient) get(ctx context.Context) (hostedManifestEnvelope, bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.manifestURL(), nil)
	if err != nil {
		return hostedManifestEnvelope{}, false, err
	}
	c.authorize(req)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return hostedManifestEnvelope{}, false, fmt.Errorf("hosted manifest get failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return hostedManifestEnvelope{}, false, nil
	}
	if resp.StatusCode != http.StatusOK {
		return hostedManifestEnvelope{}, false, hostedHTTPError("hosted manifest get failed", resp)
	}
	var envelope hostedManifestEnvelope
	if err := json.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(&envelope); err != nil {
		return hostedManifestEnvelope{}, false, fmt.Errorf("hosted manifest response was invalid JSON: %w", err)
	}
	if err := validateHostedEnvelope(envelope, c.cfg.HostedSyncWorkspace); err != nil {
		return hostedManifestEnvelope{}, false, err
	}
	return envelope, true, nil
}

func (c hostedClient) put(ctx context.Context, expectedVersion int, m Manifest) (hostedManifestEnvelope, error) {
	body, err := json.Marshal(hostedManifestPutRequest{ExpectedVersion: expectedVersion, Manifest: m})
	if err != nil {
		return hostedManifestEnvelope{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, c.manifestURL(), bytes.NewReader(body))
	if err != nil {
		return hostedManifestEnvelope{}, err
	}
	c.authorize(req)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return hostedManifestEnvelope{}, fmt.Errorf("hosted manifest put failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusConflict {
		return hostedManifestEnvelope{}, hostedHTTPError("hosted manifest version conflict", resp)
	}
	if resp.StatusCode != http.StatusOK {
		return hostedManifestEnvelope{}, hostedHTTPError("hosted manifest put failed", resp)
	}
	var envelope hostedManifestEnvelope
	if err := json.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(&envelope); err != nil {
		return hostedManifestEnvelope{}, fmt.Errorf("hosted manifest response was invalid JSON: %w", err)
	}
	if err := validateHostedEnvelope(envelope, c.cfg.HostedSyncWorkspace); err != nil {
		return hostedManifestEnvelope{}, err
	}
	return envelope, nil
}

func (c hostedClient) authorize(req *http.Request) {
	req.Header.Set("Authorization", "Bearer "+c.cfg.HostedSyncToken)
	req.Header.Set("Accept", "application/json")
}

func (c hostedClient) manifestURL() string {
	return c.cfg.HostedSyncEndpoint + "/v1/workspaces/" + url.PathEscape(c.cfg.HostedSyncWorkspace) + "/manifest"
}

func hostedHTTPError(prefix string, resp *http.Response) error {
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	msg := strings.TrimSpace(string(data))
	if msg == "" {
		msg = resp.Status
	}
	return fmt.Errorf("%s: %s", prefix, msg)
}

func recordHostedSync(version int, hash string) error {
	st, err := LoadState()
	if err != nil && !missing(err) {
		return err
	}
	if st.Projects == nil {
		st.Projects = map[string]ProjectState{}
	}
	st.HostedSyncVersion = version
	st.HostedSyncManifestHash = hash
	st.LastSyncAt = nowRFC3339()
	return SaveState(st)
}

func validateHostedEnvelope(envelope hostedManifestEnvelope, workspace string) error {
	if envelope.APIVersion != hostedManifestAPIVersion {
		return fmt.Errorf("hosted manifest API version %q is unsupported", envelope.APIVersion)
	}
	if envelope.Workspace != workspace {
		return fmt.Errorf("hosted manifest workspace mismatch: got %q, want %q", envelope.Workspace, workspace)
	}
	if envelope.Version <= 0 {
		return fmt.Errorf("hosted manifest version is required")
	}
	if err := validateHostedManifest(envelope.Manifest); err != nil {
		return err
	}
	hash, err := hostedManifestHash(envelope.Manifest)
	if err != nil {
		return err
	}
	if envelope.ManifestHash != hash {
		return fmt.Errorf("hosted manifest hash mismatch")
	}
	return nil
}

func validateHostedManifest(m Manifest) error {
	if err := ValidateManifest(m); err != nil {
		return err
	}
	for _, p := range m.Projects {
		parts := strings.Split(filepath.ToSlash(p.Path), "/")
		for _, part := range parts {
			if hostedUnsafePathComponents[part] {
				return fmt.Errorf("project %s has unsafe hosted sync path component %q", p.Name, part)
			}
		}
	}
	return nil
}

func hostedManifestHash(m Manifest) (string, error) {
	normalized := manifestForSync(m)
	data, err := manifestBytes(normalized)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

// isLoopbackHost reports whether host (as returned by url.URL.Hostname, which
// strips any surrounding brackets from IPv6 literals) refers to localhost.
func isLoopbackHost(host string) bool {
	if host == "" {
		return false
	}
	if strings.EqualFold(host, "localhost") {
		return true
	}
	trimmed := strings.TrimSuffix(strings.TrimPrefix(host, "["), "]")
	ip := net.ParseIP(trimmed)
	return ip != nil && ip.IsLoopback()
}

func validateHostedWorkspaceID(workspace string) error {
	if workspace == "" {
		return fmt.Errorf("hosted workspace id is required")
	}
	if len(workspace) > 120 {
		return fmt.Errorf("hosted workspace id is too long")
	}
	for _, r := range workspace {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' || r == '.' {
			continue
		}
		return fmt.Errorf("hosted workspace id contains unsupported character %q", r)
	}
	if workspace == "." || workspace == ".." || strings.Contains(workspace, "..") {
		return fmt.Errorf("hosted workspace id is unsafe")
	}
	return nil
}

const (
	defaultHostedSyncRateLimit = rate.Limit(10)
	defaultHostedSyncRateBurst = 20
	// defaultHostedSyncMaxLimiters bounds the per-IP limiter map so a stream of
	// distinct client IPs cannot exhaust memory. Idle entries are evicted
	// first; the oldest is dropped if the cap is still hit.
	defaultHostedSyncMaxLimiters = 4096
	hostedSyncLimiterIdleTTL     = 10 * time.Minute
)

type HostedSyncServerOptions struct {
	StoreDir string
	Token    string

	// RateLimit and RateBurst configure the per-client-IP token-bucket rate
	// limiter. Zero values fall back to sensible defaults.
	RateLimit rate.Limit
	RateBurst int
	// MaxLimiters bounds the number of tracked per-IP limiters (0 = default).
	MaxLimiters int
	// TrustedProxies is the set of CIDR ranges the server is willing to trust
	// for X-Forwarded-For client identification. When the immediate TCP peer
	// matches one of these, the rightmost (leftmost-in-chain) untrusted XFF
	// entry is used as the rate-limit key; otherwise the peer IP is used.
	// Empty (the default) means no proxy is trusted and the peer IP is always
	// used — the safe choice when the server is reached directly.
	TrustedProxies []*net.IPNet
}

type HostedSyncServeOptions struct {
	Addr              string
	Handler           http.Handler
	DiagnosticsWriter io.Writer
	ready             chan<- string
}

func RunHostedSyncServer(ctx context.Context, opts HostedSyncServeOptions) error {
	addr := opts.Addr
	if addr == "" {
		addr = ":http"
	}
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	if opts.ready != nil {
		opts.ready <- listener.Addr().String()
	}
	var errorLog *log.Logger
	if opts.DiagnosticsWriter != nil {
		errorLog = log.New(opts.DiagnosticsWriter, "", 0)
	}
	server := &http.Server{
		Addr:              addr,
		Handler:           opts.Handler,
		ErrorLog:          errorLog,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       120 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- server.Serve(listener)
	}()

	select {
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			return err
		}
		return nil
	}
}

func NewHostedSyncServer(opts HostedSyncServerOptions) (http.Handler, error) {
	storeDir := strings.TrimSpace(opts.StoreDir)
	if storeDir == "" {
		return nil, fmt.Errorf("hosted sync store directory is required")
	}
	token := strings.TrimSpace(opts.Token)
	if token == "" {
		return nil, fmt.Errorf("hosted sync auth token is required")
	}
	storeDir, err := expandPath(storeDir)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(storeDir, 0o700); err != nil {
		return nil, err
	}
	rateLimit := opts.RateLimit
	if rateLimit == 0 {
		rateLimit = defaultHostedSyncRateLimit
	}
	rateBurst := opts.RateBurst
	if rateBurst == 0 {
		rateBurst = defaultHostedSyncRateBurst
	}
	maxLimiters := opts.MaxLimiters
	if maxLimiters <= 0 {
		maxLimiters = defaultHostedSyncMaxLimiters
	}
	return &hostedSyncServer{
		storeDir:       storeDir,
		token:          token,
		limiters:       map[string]*hostedIPLimiter{},
		rateLimit:      rateLimit,
		rateBurst:      rateBurst,
		maxLimiters:    maxLimiters,
		trustedProxies: opts.TrustedProxies,
	}, nil
}

type hostedSyncServer struct {
	storeDir string
	token    string

	workspaceMus [256]sync.Mutex

	limiterMu      sync.Mutex
	limiters       map[string]*hostedIPLimiter
	rateLimit      rate.Limit
	rateBurst      int
	maxLimiters    int
	trustedProxies []*net.IPNet
}

// hostedIPLimiter is a per-client rate limiter plus the last time it was used,
// so idle entries can be evicted to keep the limiter map bounded.
type hostedIPLimiter struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// workspaceMutex returns the stripe lock that serializes read-check-write
// access to a workspace's stored manifest. Stripes are a fixed array so
// client-chosen workspace IDs cannot grow server memory; distinct workspaces
// that hash to the same stripe merely serialize, which is safe.
func (s *hostedSyncServer) workspaceMutex(workspace string) *sync.Mutex {
	h := fnv.New32a()
	_, _ = h.Write([]byte(workspace))
	return &s.workspaceMus[h.Sum32()%uint32(len(s.workspaceMus))]
}

// allowRequest reports whether the identified client is within its rate
// limit, lazily creating a per-client token-bucket limiter. Client identity
// comes from clientIP, which honors X-Forwarded-For when the immediate peer
// is a configured trusted proxy.
func (s *hostedSyncServer) allowRequest(r *http.Request) bool {
	ip := s.clientIP(r)
	now := time.Now()
	s.limiterMu.Lock()
	entry, ok := s.limiters[ip]
	if !ok {
		if len(s.limiters) >= s.maxLimiters {
			s.evictLimitersLocked(now)
		}
		entry = &hostedIPLimiter{limiter: rate.NewLimiter(s.rateLimit, s.rateBurst)}
		s.limiters[ip] = entry
	}
	entry.lastSeen = now
	s.limiterMu.Unlock()
	return entry.limiter.Allow()
}

// evictLimitersLocked drops limiters idle beyond the TTL and, if the map is
// still at capacity, the single oldest entry. The caller must hold limiterMu.
func (s *hostedSyncServer) evictLimitersLocked(now time.Time) {
	var oldestKey string
	var oldest time.Time
	for k, e := range s.limiters {
		if now.Sub(e.lastSeen) > hostedSyncLimiterIdleTTL {
			delete(s.limiters, k)
			continue
		}
		if oldestKey == "" || e.lastSeen.Before(oldest) {
			oldestKey, oldest = k, e.lastSeen
		}
	}
	if len(s.limiters) >= s.maxLimiters && oldestKey != "" {
		delete(s.limiters, oldestKey)
	}
}

func hostedClientIP(remoteAddr string) string {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return remoteAddr
	}
	return host
}

// clientIP resolves the rate-limit key for r. When the immediate TCP peer is
// a configured trusted proxy, the rightmost (leftmost-in-chain) untrusted
// entry of X-Forwarded-For is used; otherwise the peer IP is used directly.
// This keeps rate limiting meaningful behind a proxy (Railway, Fly, nginx),
// where every connection's RemoteAddr is the proxy and an untrusted XFF
// header would otherwise let a single client spoof an unbounded number of
// distinct limiter keys.
func (s *hostedSyncServer) clientIP(r *http.Request) string {
	peer := hostedClientIP(r.RemoteAddr)
	if len(s.trustedProxies) == 0 {
		return peer
	}
	if !ipInAnyCIDR(peer, s.trustedProxies) {
		return peer
	}
	// Peer is trusted: walk X-Forwarded-For right-to-left, skipping any hop
	// that is itself a trusted proxy. The first untrusted address is the
	// originating client as far back as this server can verify. If every hop
	// is trusted, fall back to the leftmost entry.
	xff := r.Header.Get("X-Forwarded-For")
	hops := splitForwarded(xff)
	for i := len(hops) - 1; i >= 0; i-- {
		hop := hops[i]
		if hop == "" {
			continue
		}
		if !ipInAnyCIDR(hop, s.trustedProxies) {
			return hop
		}
	}
	if len(hops) > 0 {
		// All listed hops are trusted proxies; use the original (leftmost).
		for _, hop := range hops {
			if hop != "" {
				return hop
			}
		}
	}
	return peer
}

// splitForwarded parses an X-Forwarded-For header into its comma-separated
// client entries, trimmed and de-blanked, in the order they appear (leftmost
// client first). It performs no validation beyond trimming.
func splitForwarded(header string) []string {
	if header == "" {
		return nil
	}
	parts := strings.Split(header, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

// ipInAnyCIDR reports whether addr (an IP literal) is contained in any of the
// given CIDR ranges. Non-IP literals return false.
func ipInAnyCIDR(addr string, cidrs []*net.IPNet) bool {
	ip := net.ParseIP(addr)
	if ip == nil {
		return false
	}
	for _, c := range cidrs {
		if c.Contains(ip) {
			return true
		}
	}
	return false
}

// parseTrustedProxyCIDRs parses a list of CIDR strings (e.g. "10.0.0.0/8",
// "::1/128") into net.IPNet values. A bare IP is treated as a /32 or /128.
func parseTrustedProxyCIDRs(values []string) ([]*net.IPNet, error) {
	var out []*net.IPNet
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		_, ipnet, err := net.ParseCIDR(v)
		if err != nil {
			// Allow a bare IP: treat as a single-host network.
			ip := net.ParseIP(v)
			if ip == nil {
				return nil, fmt.Errorf("invalid trusted-proxy CIDR or IP %q", v)
			}
			if ip.To4() != nil {
				ipnet = &net.IPNet{IP: ip.To4(), Mask: net.CIDRMask(32, 32)}
			} else {
				ipnet = &net.IPNet{IP: ip, Mask: net.CIDRMask(128, 128)}
			}
		}
		out = append(out, ipnet)
	}
	return out, nil
}

func (s *hostedSyncServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/healthz" {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
		return
	}
	if !s.allowRequest(r) {
		http.Error(w, "too many requests\n", http.StatusTooManyRequests)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	expected := "Bearer " + s.token
	if subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), []byte(expected)) != 1 {
		http.Error(w, "unauthorized\n", http.StatusUnauthorized)
		return
	}
	workspace, ok := hostedWorkspaceFromPath(r.URL.Path)
	if !ok {
		http.NotFound(w, r)
		return
	}
	if err := validateHostedWorkspaceID(workspace); err != nil {
		http.Error(w, err.Error()+"\n", http.StatusBadRequest)
		return
	}
	switch r.Method {
	case http.MethodGet:
		s.handleGet(w, workspace)
	case http.MethodPut:
		s.handlePut(w, r, workspace)
	default:
		w.Header().Set("Allow", "GET, PUT")
		http.Error(w, "method not allowed\n", http.StatusMethodNotAllowed)
	}
}

func (s *hostedSyncServer) handleGet(w http.ResponseWriter, workspace string) {
	envelope, err := s.load(workspace)
	if missing(err) {
		http.Error(w, "not found\n", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, err.Error()+"\n", http.StatusInternalServerError)
		return
	}
	_ = json.NewEncoder(w).Encode(envelope)
}

func (s *hostedSyncServer) handlePut(w http.ResponseWriter, r *http.Request, workspace string) {
	var req hostedManifestPutRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 2<<20)).Decode(&req); err != nil {
		http.Error(w, "invalid JSON\n", http.StatusBadRequest)
		return
	}
	if err := validateHostedManifest(req.Manifest); err != nil {
		http.Error(w, err.Error()+"\n", http.StatusBadRequest)
		return
	}

	wsMutex := s.workspaceMutex(workspace)
	wsMutex.Lock()
	defer wsMutex.Unlock()

	current, err := s.load(workspace)
	if err != nil && !missing(err) {
		http.Error(w, err.Error()+"\n", http.StatusInternalServerError)
		return
	}
	currentVersion := 0
	if err == nil {
		currentVersion = current.Version
	}
	if req.ExpectedVersion != currentVersion {
		http.Error(w, "version conflict: current version is "+strconv.Itoa(currentVersion)+"\n", http.StatusConflict)
		return
	}
	hash, err := hostedManifestHash(req.Manifest)
	if err != nil {
		http.Error(w, err.Error()+"\n", http.StatusInternalServerError)
		return
	}
	envelope := hostedManifestEnvelope{
		APIVersion:   hostedManifestAPIVersion,
		Workspace:    workspace,
		Version:      currentVersion + 1,
		ManifestHash: hash,
		UpdatedAt:    nowRFC3339(),
		Manifest:     manifestForSync(req.Manifest),
	}
	if err := s.save(envelope); err != nil {
		http.Error(w, err.Error()+"\n", http.StatusInternalServerError)
		return
	}
	_ = json.NewEncoder(w).Encode(envelope)
}

func (s *hostedSyncServer) load(workspace string) (hostedManifestEnvelope, error) {
	var envelope hostedManifestEnvelope
	if err := readJSON(s.path(workspace), &envelope); err != nil {
		return envelope, err
	}
	return envelope, validateHostedEnvelope(envelope, workspace)
}

func (s *hostedSyncServer) save(envelope hostedManifestEnvelope) error {
	return writeJSON(s.path(envelope.Workspace), envelope, 0o600)
}

func (s *hostedSyncServer) path(workspace string) string {
	return filepath.Join(s.storeDir, workspace+".json")
}

func hostedWorkspaceFromPath(path string) (string, bool) {
	const prefix = "/v1/workspaces/"
	const suffix = "/manifest"
	if !strings.HasPrefix(path, prefix) || !strings.HasSuffix(path, suffix) {
		return "", false
	}
	workspace := strings.TrimSuffix(strings.TrimPrefix(path, prefix), suffix)
	if workspace == "" || strings.Contains(workspace, "/") {
		return "", false
	}
	unescaped, err := url.PathUnescape(workspace)
	if err != nil {
		return "", false
	}
	return unescaped, true
}
