package devspace

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func uiServerRoundTrip(t *testing.T, opts uiServerOptions, requests []string) []map[string]any {
	t.Helper()
	input := strings.Join(requests, "\n") + "\n"
	outR, outW := io.Pipe()
	done := make(chan error, 1)
	go func() {
		err := runUIServer(strings.NewReader(input), outW, opts)
		_ = outW.Close()
		done <- err
	}()
	var messages []map[string]any
	dec := json.NewDecoder(outR)
	for {
		var msg map[string]any
		if err := dec.Decode(&msg); err != nil {
			if !errors.Is(err, io.EOF) {
				t.Fatalf("decode server output: %v", err)
			}
			break
		}
		messages = append(messages, msg)
	}
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

func TestUIServerErrorPaths(t *testing.T) {
	dashboardSeedWorkspace(t)

	messages := uiServerRoundTrip(t, uiServerOptions{NoWatch: true}, []string{
		`{"id":1,"method":"hydrate"}`,
		`{"id":2,"method":"hydrate","params":{"ref":"nope"}}`,
		`{"id":3,"method":"bogus"}`,
		`not json`,
		`{"id":4,"method":"hello"}`,
	})
	if len(messages) != 5 {
		t.Fatalf("expected 5 responses, got %d: %+v", len(messages), messages)
	}
	if msg := uiResponseError(t, messages[0]); !strings.Contains(msg, "requires params.ref") {
		t.Fatalf("hydrate no-ref error = %q", msg)
	}
	if msg := uiResponseError(t, messages[1]); !strings.Contains(msg, "not found") {
		t.Fatalf("hydrate bad-ref error = %q", msg)
	}
	if msg := uiResponseError(t, messages[2]); !strings.Contains(msg, "unknown method") {
		t.Fatalf("unknown method error = %q", msg)
	}
	if msg := uiResponseError(t, messages[3]); !strings.Contains(msg, "malformed request") {
		t.Fatalf("malformed error = %q", msg)
	}
	uiResponseResult(t, messages[4])
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
		_ = runUIServer(inR, outW, uiServerOptions{watchCmdFactory: factory})
		_ = outW.Close()
	}()

	dec := json.NewDecoder(outR)
	var event map[string]any
	if err := dec.Decode(&event); err != nil {
		t.Fatalf("decode event: %v", err)
	}
	params := event["params"].(map[string]any)
	if params["type"] != "watch-error" || params["message"] != "boom 1" {
		t.Fatalf("event = %+v", event)
	}
	_ = inW.Close()
	if calls != 1 {
		t.Fatalf("watch loop re-armed after error: calls = %d", calls)
	}
}
