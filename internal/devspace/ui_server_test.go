package devspace

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
)

func TestFindTUIBinaryUsesAppHomeBin(t *testing.T) {
	home := t.TempDir()
	t.Setenv("DEVSPACE_HOME", home)
	t.Setenv("PATH", t.TempDir())
	if got := findTUIBinary(); got != "" {
		t.Fatalf("findTUIBinary with nothing installed = %q, want empty", got)
	}
	bin := filepath.Join(home, "bin", tuiBinaryName)
	if err := os.MkdirAll(filepath.Dir(bin), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bin, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := findTUIBinary(); got != bin {
		t.Fatalf("findTUIBinary = %q, want %q", got, bin)
	}
}

func TestFindTUIBinaryPrefersAdjacentCompanion(t *testing.T) {
	dir := t.TempDir()
	executable := filepath.Join(dir, "devspace")
	adjacent := filepath.Join(dir, tuiBinaryName)
	home := t.TempDir()
	pathDir := t.TempDir()
	t.Setenv("DEVSPACE_HOME", home)
	t.Setenv("PATH", pathDir)
	for _, path := range []string{
		adjacent,
		filepath.Join(home, "bin", tuiBinaryName),
		filepath.Join(pathDir, tuiBinaryName),
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	if got := findTUIBinaryFrom(executable); got != adjacent {
		t.Fatalf("findTUIBinaryFrom = %q, want adjacent %q", got, adjacent)
	}
}

func TestFindTUIBinaryFallsBackToPATH(t *testing.T) {
	pathDir := t.TempDir()
	t.Setenv("DEVSPACE_HOME", t.TempDir())
	t.Setenv("PATH", pathDir)
	bin := filepath.Join(pathDir, tuiBinaryName)
	if err := os.WriteFile(bin, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	if got := findTUIBinaryFrom(filepath.Join(t.TempDir(), "devspace")); got != bin {
		t.Fatalf("findTUIBinaryFrom = %q, want PATH binary %q", got, bin)
	}
}

func TestUICommandDocumentsBundledCompanionAndLegacyFallback(t *testing.T) {
	stdout, _, err := executeCommand(t, "test", "ui", "--help")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"bundled", "next to devspace", "source build", "--legacy"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("ui --help missing %q:\n%s", want, stdout)
		}
	}
	if strings.Contains(stdout, "tui install") {
		t.Fatalf("ui --help recommends removed installer:\n%s", stdout)
	}
}

func uiServerRoundTrip(t *testing.T, opts uiServerOptions, requests []string) []map[string]any {
	t.Helper()
	outR, outW := io.Pipe()
	inR, inW := io.Pipe()
	done := make(chan error, 1)
	go func() {
		err := runUIServer(inR, outW, opts)
		_ = outW.Close()
		done <- err
	}()
	var messages []map[string]any
	dec := json.NewDecoder(outR)
	for _, req := range requests {
		if _, err := fmt.Fprintln(inW, req); err != nil {
			t.Fatalf("write request: %v", err)
		}
		var msg map[string]any
		if err := dec.Decode(&msg); err != nil {
			t.Fatalf("decode server output: %v", err)
		}
		messages = append(messages, msg)
	}
	_ = inW.Close()
	if err := <-done; err != nil {
		t.Fatalf("runUIServer: %v", err)
	}
	return messages
}

func uiResponseResult(t *testing.T, msg map[string]any) map[string]any {
	t.Helper()
	if errObj, ok := msg["error"]; ok && errObj != nil {
		t.Fatalf("unexpected error response: %+v", msg)
	}
	result, ok := msg["result"].(map[string]any)
	if !ok {
		t.Fatalf("response has no object result: %+v", msg)
	}
	return result
}

func uiResponseError(t *testing.T, msg map[string]any) string {
	t.Helper()
	errObj, ok := msg["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected error response, got %+v", msg)
	}
	message, _ := errObj["message"].(string)
	return message
}

func TestUIServerRequestResponseFlow(t *testing.T) {
	workspace := dashboardSeedWorkspace(t)

	messages := uiServerRoundTrip(t, uiServerOptions{Version: "test", NoWatch: true}, []string{
		`{"id":1,"method":"hello"}`,
		`{"id":2,"method":"scan"}`,
		`{"id":3,"method":"projects"}`,
		`{"id":4,"method":"status"}`,
		`{"id":5,"method":"plan"}`,
		`{"id":6,"method":"apply"}`,
		`{"id":7,"method":"lastPlan"}`,
	})
	if len(messages) != 7 {
		t.Fatalf("expected 7 responses, got %d: %+v", len(messages), messages)
	}

	hello := uiResponseResult(t, messages[0])
	if hello["workspaceRoot"] != workspace {
		t.Fatalf("hello workspaceRoot = %v, want %s", hello["workspaceRoot"], workspace)
	}
	if hello["protocol"] != float64(uiProtocolVersion) || hello["watch"] != false || hello["version"] != "test" {
		t.Fatalf("hello = %+v", hello)
	}

	scan := uiResponseResult(t, messages[1])
	rows, ok := scan["rows"].([]any)
	if !ok || len(rows) < 2 {
		t.Fatalf("scan rows = %+v", scan["rows"])
	}
	names := map[string]map[string]any{}
	for _, r := range rows {
		row := r.(map[string]any)
		names[row["name"].(string)] = row
	}
	if api, ok := names["api"]; !ok || api["status"] != dashboardStatusHydrated || api["dirty"] != true || api["env"] != true {
		t.Fatalf("api row = %+v", names["api"])
	}
	if missing, ok := names["missing"]; !ok || missing["status"] != dashboardStatusMissing {
		t.Fatalf("missing row = %+v", names["missing"])
	}
	summary := scan["summary"].(map[string]any)
	if summary["foundProjects"].(float64) < 2 {
		t.Fatalf("summary = %+v", summary)
	}

	projects := uiResponseResult(t, messages[2])
	if len(projects["rows"].([]any)) != len(rows) {
		t.Fatalf("projects rows = %+v", projects["rows"])
	}

	status := uiResponseResult(t, messages[3])
	if status["configured"] != false || status["unavailableReason"] != syncStatusRemoteNotConfigured {
		t.Fatalf("status = %+v", status)
	}

	plan := uiResponseResult(t, messages[4])
	planObj, ok := plan["plan"].(map[string]any)
	if !ok {
		t.Fatalf("plan response missing plan: %+v", plan)
	}
	actions := planObj["actions"].([]any)
	foundPlaceholder := false
	for _, a := range actions {
		action := a.(map[string]any)
		if action["kind"] == "placeholder" && action["path"] == "apps/missing" && action["safety"] == "safe" {
			foundPlaceholder = true
		}
	}
	if !foundPlaceholder {
		t.Fatalf("plan actions = %+v", actions)
	}

	apply := uiResponseResult(t, messages[5])
	if _, ok := apply["plan"].(map[string]any); !ok {
		t.Fatalf("apply response missing plan: %+v", apply)
	}
	applyRows := apply["rows"].([]any)
	for _, r := range applyRows {
		row := r.(map[string]any)
		if row["name"] == "missing" && row["status"] != dashboardStatusPlaceholder {
			t.Fatalf("missing row after apply = %+v", row)
		}
	}

	if errObj, ok := messages[6]["error"]; ok && errObj != nil {
		t.Fatalf("lastPlan errored: %+v", messages[6])
	}
}

func TestUIServerRemoveUntracksProjectAndLeavesFiles(t *testing.T) {
	workspace := initCommandWorkspace(t)
	appPath := filepath.Join(workspace, "apps", "api")
	hardeningWriteFile(t, filepath.Join(appPath, "README.md"), "hello\n", 0o644)
	app, err := AddProject("apps/api")
	if err != nil {
		t.Fatal(err)
	}
	otherPath := filepath.Join(workspace, "apps", "other")
	hardeningWriteFile(t, filepath.Join(otherPath, "README.md"), "hello\n", 0o644)
	other, err := AddProject("apps/other")
	if err != nil {
		t.Fatal(err)
	}

	messages := uiServerRoundTrip(t, uiServerOptions{NoWatch: true}, []string{
		`{"id":1,"method":"remove","params":{"ref":"api"}}`,
	})
	result := uiResponseResult(t, messages[0])
	if project := result["project"].(map[string]any); project["id"] != app.ID {
		t.Fatalf("removed project = %+v, want %s", project, app.ID)
	}
	for _, row := range result["rows"].([]any) {
		if row.(map[string]any)["ref"] == app.Path {
			t.Fatalf("removed project still present in rows: %+v", result["rows"])
		}
	}
	manifest, err := LoadManifest(workspace)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := findProject(manifest, app.ID); ok {
		t.Fatal("removed project still present in manifest")
	}
	if _, ok := findProject(manifest, other.ID); !ok {
		t.Fatal("unrelated project was removed")
	}
	state, err := LoadState()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := state.Projects[app.ID]; ok {
		t.Fatal("removed project state remains")
	}
	if !exists(appPath) {
		t.Fatal("project directory was deleted")
	}
}

func TestUIServerRemoveReturnsAccessAdvisoryWarnings(t *testing.T) {
	_, project := commandWorkspaceWithProjectRole(t, AccessRoleDeveloper)

	messages := uiServerRoundTrip(t, uiServerOptions{NoWatch: true}, []string{
		fmt.Sprintf(`{"id":1,"method":"remove","params":{"ref":%q}}`, project.Name),
	})
	result := uiResponseResult(t, messages[0])
	warnings, ok := result["warnings"].([]any)
	if !ok || len(warnings) == 0 {
		t.Fatalf("warnings = %+v, want advisory warning", result["warnings"])
	}
	if got := warnings[0].(string); !strings.Contains(got, "devspace project untrack") {
		t.Fatalf("warning = %q, want project untrack advisory", got)
	}
}

func TestUIServerErrorPaths(t *testing.T) {
	dashboardSeedWorkspace(t)

	messages := uiServerRoundTrip(t, uiServerOptions{NoWatch: true}, []string{
		`{"id":1,"method":"hydrate"}`,
		`{"id":2,"method":"hydrate","params":{"ref":"nope"}}`,
		`{"id":3,"method":"remove"}`,
		`{"id":4,"method":"remove","params":{"ref":"  "}}`,
		`{"id":5,"method":"remove","params":{"ref":"nope"}}`,
		`{"id":6,"method":"bogus"}`,
		`not json`,
		`{"id":7,"method":"hello"}`,
	})
	if len(messages) != 8 {
		t.Fatalf("expected 8 responses, got %d: %+v", len(messages), messages)
	}
	if msg := uiResponseError(t, messages[0]); !strings.Contains(msg, "requires params.ref") {
		t.Fatalf("hydrate no-ref error = %q", msg)
	}
	if msg := uiResponseError(t, messages[1]); !strings.Contains(msg, "not found") {
		t.Fatalf("hydrate bad-ref error = %q", msg)
	}
	if msg := uiResponseError(t, messages[2]); !strings.Contains(msg, "requires params.ref") {
		t.Fatalf("remove no-ref error = %q", msg)
	}
	if msg := uiResponseError(t, messages[3]); !strings.Contains(msg, "requires params.ref") {
		t.Fatalf("remove blank-ref error = %q", msg)
	}
	if msg := uiResponseError(t, messages[4]); !strings.Contains(msg, "not found") {
		t.Fatalf("remove bad-ref error = %q", msg)
	}
	if msg := uiResponseError(t, messages[5]); !strings.Contains(msg, "unknown method") {
		t.Fatalf("unknown method error = %q", msg)
	}
	if msg := uiResponseError(t, messages[6]); !strings.Contains(msg, "malformed request") {
		t.Fatalf("malformed error = %q", msg)
	}
	uiResponseResult(t, messages[7])
}

func TestUIServerWatchEventPush(t *testing.T) {
	dashboardSeedWorkspace(t)

	calls := 0
	factory := func(syncMode string) tea.Cmd {
		return func() tea.Msg {
			calls++
			if calls > 1 {
				return nil
			}
			return watchRefreshMsg{
				refresh: WatchRefresh{FullScan: true, WatchedDirCount: 3, RefreshStartedAt: "now", SyncMode: syncMode},
				rows:    []dashboardRow{{ref: "apps/api", name: "api", status: dashboardStatusHydrated}},
				summary: ScanSummary{FoundProjects: 1},
			}
		}
	}

	outR, outW := io.Pipe()
	inR, inW := io.Pipe()
	go func() {
		_ = runUIServer(inR, outW, uiServerOptions{watchCmdFactory: factory})
		_ = outW.Close()
	}()

	dec := json.NewDecoder(outR)
	var event map[string]any
	if err := dec.Decode(&event); err != nil {
		t.Fatalf("decode event: %v", err)
	}
	if event["method"] != "event" {
		t.Fatalf("expected event, got %+v", event)
	}
	params := event["params"].(map[string]any)
	if params["type"] != "watch-refresh" {
		t.Fatalf("event params = %+v", params)
	}
	refresh := params["refresh"].(map[string]any)
	if refresh["fullScan"] != true || refresh["watchedDirCount"] != float64(3) {
		t.Fatalf("refresh = %+v", refresh)
	}
	if rows := params["rows"].([]any); len(rows) != 1 {
		t.Fatalf("rows = %+v", params["rows"])
	}
	_ = inW.Close()
}

func TestUIServerWatchErrorEndsLoop(t *testing.T) {
	dashboardSeedWorkspace(t)

	calls := 0
	factory := func(string) tea.Cmd {
		return func() tea.Msg {
			calls++
			return watchRefreshMsg{err: fmt.Errorf("boom %d", calls)}
		}
	}

	outR, outW := io.Pipe()
	inR, inW := io.Pipe()
	go func() {
		_ = runUIServer(inR, outW, uiServerOptions{watchCmdFactory: factory, watchRetryBase: time.Millisecond})
		_ = outW.Close()
	}()

	dec := json.NewDecoder(outR)
	for i := 0; i < watchRetryMaxAttempts; i++ {
		var event map[string]any
		if err := dec.Decode(&event); err != nil {
			t.Fatalf("decode event %d: %v", i, err)
		}
		params := event["params"].(map[string]any)
		if params["type"] != "watch-error" {
			t.Fatalf("event %d = %+v", i, event)
		}
	}
	_ = inW.Close()
	if calls != watchRetryMaxAttempts {
		t.Fatalf("expected loop to stop after %d consecutive errors, calls = %d", watchRetryMaxAttempts, calls)
	}
}

func TestUIServerWatchErrorRecovers(t *testing.T) {
	dashboardSeedWorkspace(t)

	var mu sync.Mutex
	calls := 0
	callSignal := make(chan struct{}, 16)
	factory := func(string) tea.Cmd {
		return func() tea.Msg {
			mu.Lock()
			calls++
			n := calls
			mu.Unlock()
			callSignal <- struct{}{}
			switch n {
			case 1:
				return watchRefreshMsg{err: errors.New("transient")}
			case 2:
				return watchRefreshMsg{
					refresh: WatchRefresh{FullScan: true},
					rows:    []dashboardRow{{ref: "apps/api", name: "api", status: dashboardStatusHydrated}},
					summary: ScanSummary{FoundProjects: 1},
				}
			default:
				return nil
			}
		}
	}

	outR, outW := io.Pipe()
	inR, inW := io.Pipe()
	go func() {
		_ = runUIServer(inR, outW, uiServerOptions{watchCmdFactory: factory, watchRetryBase: time.Millisecond})
		_ = outW.Close()
	}()

	dec := json.NewDecoder(outR)
	var errEvent, refreshEvent map[string]any
	if err := dec.Decode(&errEvent); err != nil {
		t.Fatalf("decode error event: %v", err)
	}
	if err := dec.Decode(&refreshEvent); err != nil {
		t.Fatalf("decode refresh event: %v", err)
	}
	if params := errEvent["params"].(map[string]any); params["type"] != "watch-error" {
		t.Fatalf("first event = %+v", errEvent)
	}
	if params := refreshEvent["params"].(map[string]any); params["type"] != "watch-refresh" {
		t.Fatalf("second event = %+v", refreshEvent)
	}

	// Wait for the third (nil-returning, loop-ending) factory call so the
	// backoff-reset path after a successful refresh is actually exercised.
	for i := 0; i < 3; i++ {
		select {
		case <-callSignal:
		case <-time.After(2 * time.Second):
			t.Fatalf("timed out waiting for factory call %d", i+1)
		}
	}
	_ = inW.Close()

	mu.Lock()
	got := calls
	mu.Unlock()
	if got <= 2 {
		t.Fatalf("expected more than 2 factory calls (backoff reset path), got %d", got)
	}
}

func TestUIServerWatchClosedEmitsEvent(t *testing.T) {
	dashboardSeedWorkspace(t)

	factory := func(string) tea.Cmd {
		return func() tea.Msg { return nil }
	}

	dec, inW, done := startUIServerForTest(t, uiServerOptions{watchCmdFactory: factory})
	event := readUIServerMessage(t, dec)
	params := event["params"].(map[string]any)
	if event["method"] != "event" || params["type"] != "watch-error" || !strings.Contains(params["message"].(string), "watcher closed") {
		t.Fatalf("event = %+v", event)
	}
	closeUIServerForTest(t, inW, done)
}

func TestUIServerScanRestartsDeadWatcher(t *testing.T) {
	dashboardSeedWorkspace(t)

	var calls atomic.Int32
	factory := func(string) tea.Cmd {
		return func() tea.Msg {
			n := calls.Add(1)
			if n <= int32(watchRetryMaxAttempts) {
				return watchRefreshMsg{err: fmt.Errorf("boom %d", n)}
			}
			if n == int32(watchRetryMaxAttempts+1) {
				return watchRefreshMsg{
					refresh: WatchRefresh{FullScan: true, WatchedDirCount: 1},
					rows:    []dashboardRow{{ref: "apps/api", name: "api", status: dashboardStatusHydrated}},
					summary: ScanSummary{FoundProjects: 1},
				}
			}
			return nil
		}
	}

	dec, inW, done := startUIServerForTest(t, uiServerOptions{
		watchCmdFactory: factory,
		watchRetryBase:  time.Millisecond,
		scanCmd: func() tea.Cmd {
			return func() tea.Msg {
				return scanLoadedMsg{
					rows:    []dashboardRow{{ref: "apps/api", name: "api", status: dashboardStatusHydrated}},
					summary: ScanSummary{FoundProjects: 1},
				}
			}
		},
	})

	for i := 0; i < watchRetryMaxAttempts; i++ {
		event := readUIServerMessage(t, dec)
		params := event["params"].(map[string]any)
		if params["type"] != "watch-error" {
			t.Fatalf("event %d = %+v", i, event)
		}
	}

	writeUIServerRequest(t, inW, `{"id":1,"method":"scan"}`)
	sawRefresh := false
	for i := 0; i < 3 && !sawRefresh; i++ {
		msg := readUIServerMessage(t, dec)
		if msg["method"] == "event" {
			params := msg["params"].(map[string]any)
			sawRefresh = params["type"] == "watch-refresh"
			continue
		}
		if msg["id"] == float64(1) {
			uiResponseResult(t, msg)
		}
	}
	if !sawRefresh {
		t.Fatal("restarted watcher did not emit watch-refresh")
	}
	if got := calls.Load(); got < int32(watchRetryMaxAttempts+1) {
		t.Fatalf("watch factory calls = %d, want at least %d", got, watchRetryMaxAttempts+1)
	}
	closeUIServerForTest(t, inW, done)
}

func TestUIServerSyncModeWiring(t *testing.T) {
	dashboardSeedWorkspace(t)

	waitForSyncMode := func(t *testing.T, opts uiServerOptions) string {
		t.Helper()
		gotCh := make(chan string, 1)
		opts.watchCmdFactory = func(syncMode string) tea.Cmd {
			return func() tea.Msg {
				gotCh <- syncMode
				return nil
			}
		}

		outR, outW := io.Pipe()
		inR, inW := io.Pipe()
		go func() {
			_ = runUIServer(inR, outW, opts)
			_ = outW.Close()
		}()

		var got string
		select {
		case got = <-gotCh:
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for watchCmdFactory call")
		}
		_ = inW.Close()
		_, _ = io.Copy(io.Discard, outR)
		return got
	}

	for _, mode := range []string{"git", "hosted"} {
		t.Run(mode, func(t *testing.T) {
			if got := waitForSyncMode(t, uiServerOptions{SyncMode: mode}); got != mode {
				t.Fatalf("watchCmdFactory syncMode = %q, want %q", got, mode)
			}
		})
	}

	t.Run("defaults to off", func(t *testing.T) {
		if got := waitForSyncMode(t, uiServerOptions{}); got != WatchSyncOff {
			t.Fatalf("default syncMode = %q, want %q", got, WatchSyncOff)
		}
	})
}

func TestUIServerHandlesVeryLongRequestLine(t *testing.T) {
	dashboardSeedWorkspace(t)

	longReq, err := json.Marshal(uiServerRequest{
		ID:     1,
		Method: "hello",
		Params: json.RawMessage(fmt.Sprintf(`{"padding":%q}`, strings.Repeat("x", 2*1024*1024))),
	})
	if err != nil {
		t.Fatal(err)
	}

	messages := uiServerRoundTrip(t, uiServerOptions{Version: "test", NoWatch: true}, []string{
		string(longReq),
		`{"id":2,"method":"hello"}`,
	})
	if len(messages) != 2 {
		t.Fatalf("expected 2 responses, got %d: %+v", len(messages), messages)
	}
	first := uiResponseResult(t, messages[0])
	if first["protocol"] != float64(uiProtocolVersion) {
		t.Fatalf("first response = %+v", first)
	}
	uiResponseResult(t, messages[1])
}

func TestUIServerReadsNotBlockedBySlowAction(t *testing.T) {
	dashboardSeedWorkspace(t)

	started := make(chan struct{})
	unblock := make(chan struct{})
	opts := uiServerOptions{
		NoWatch: true,
		hydrateCmd: func(string) tea.Cmd {
			return func() tea.Msg {
				close(started)
				<-unblock
				return actionResultMsg{
					label:   "hydrate",
					rows:    []dashboardRow{{ref: "apps/api", name: "api", status: dashboardStatusHydrated}},
					summary: ScanSummary{FoundProjects: 1},
				}
			}
		},
	}
	dec, inW, done := startUIServerForTest(t, opts)

	writeUIServerRequest(t, inW, `{"id":1,"method":"hydrate","params":{"ref":"apps/api"}}`)
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("hydrate did not start")
	}
	writeUIServerRequest(t, inW, `{"id":2,"method":"hello"}`)

	hello := readUIServerMessage(t, dec)
	if hello["id"] != float64(2) {
		t.Fatalf("first response id = %v, want 2: %+v", hello["id"], hello)
	}
	uiResponseResult(t, hello)

	close(unblock)
	hydrate := readUIServerMessage(t, dec)
	if hydrate["id"] != float64(1) {
		t.Fatalf("second response id = %v, want 1: %+v", hydrate["id"], hydrate)
	}
	uiResponseResult(t, hydrate)
	closeUIServerForTest(t, inW, done)
}

func TestUIServerWorkspaceReadNotBlockedBySlowAction(t *testing.T) {
	dashboardSeedWorkspace(t)

	started := make(chan struct{})
	unblock := make(chan struct{})
	opts := uiServerOptions{
		NoWatch: true,
		hydrateCmd: func(string) tea.Cmd {
			return func() tea.Msg {
				close(started)
				<-unblock
				return actionResultMsg{label: "hydrate"}
			}
		},
	}
	dec, inW, done := startUIServerForTest(t, opts)

	writeUIServerRequest(t, inW, `{"id":1,"method":"hydrate","params":{"ref":"apps/api"}}`)
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("hydrate did not start")
	}
	writeUIServerRequest(t, inW, `{"id":2,"method":"workspace"}`)

	workspace := readUIServerMessage(t, dec)
	if workspace["id"] != float64(2) {
		t.Fatalf("first response id = %v, want 2: %+v", workspace["id"], workspace)
	}
	result := uiResponseResult(t, workspace)
	if result["workspaceRoot"] == "" {
		t.Fatalf("workspace result = %+v", result)
	}

	close(unblock)
	hydrate := readUIServerMessage(t, dec)
	if hydrate["id"] != float64(1) {
		t.Fatalf("second response id = %v, want 1: %+v", hydrate["id"], hydrate)
	}
	uiResponseResult(t, hydrate)
	closeUIServerForTest(t, inW, done)
}

func TestUIServerRejectsConcurrentActions(t *testing.T) {
	dashboardSeedWorkspace(t)

	started := make(chan struct{})
	unblock := make(chan struct{})
	opts := uiServerOptions{
		NoWatch: true,
		hydrateCmd: func(string) tea.Cmd {
			return func() tea.Msg {
				close(started)
				<-unblock
				return actionResultMsg{label: "hydrate"}
			}
		},
	}
	dec, inW, done := startUIServerForTest(t, opts)

	writeUIServerRequest(t, inW, `{"id":1,"method":"hydrate","params":{"ref":"apps/api"}}`)
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("hydrate did not start")
	}
	writeUIServerRequest(t, inW, `{"id":2,"method":"scan"}`)

	scan := readUIServerMessage(t, dec)
	if scan["id"] != float64(2) {
		t.Fatalf("first response id = %v, want 2: %+v", scan["id"], scan)
	}
	if msg := uiResponseError(t, scan); !strings.Contains(msg, "busy: hydrate in progress") {
		t.Fatalf("scan error = %q, want busy hydrate", msg)
	}

	close(unblock)
	hydrate := readUIServerMessage(t, dec)
	if hydrate["id"] != float64(1) {
		t.Fatalf("second response id = %v, want 1: %+v", hydrate["id"], hydrate)
	}
	uiResponseResult(t, hydrate)
	closeUIServerForTest(t, inW, done)
}

func startUIServerForTest(t *testing.T, opts uiServerOptions) (*json.Decoder, *io.PipeWriter, <-chan error) {
	t.Helper()
	outR, outW := io.Pipe()
	inR, inW := io.Pipe()
	done := make(chan error, 1)
	go func() {
		err := runUIServer(inR, outW, opts)
		_ = outW.Close()
		done <- err
	}()
	return json.NewDecoder(outR), inW, done
}

func writeUIServerRequest(t *testing.T, inW *io.PipeWriter, req string) {
	t.Helper()
	if _, err := fmt.Fprintln(inW, req); err != nil {
		t.Fatalf("write request: %v", err)
	}
}

func readUIServerMessage(t *testing.T, dec *json.Decoder) map[string]any {
	t.Helper()
	type result struct {
		msg map[string]any
		err error
	}
	ch := make(chan result, 1)
	go func() {
		var msg map[string]any
		err := dec.Decode(&msg)
		ch <- result{msg: msg, err: err}
	}()
	select {
	case got := <-ch:
		if got.err != nil {
			t.Fatalf("decode server output: %v", got.err)
		}
		return got.msg
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for server output")
		return nil
	}
}

func closeUIServerForTest(t *testing.T, inW *io.PipeWriter, done <-chan error) {
	t.Helper()
	_ = inW.Close()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("runUIServer: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for server shutdown")
	}
}
