package devspace

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"

	tea "charm.land/bubbletea/v2"
	"github.com/spf13/cobra"
)

// uiServer speaks newline-delimited JSON-RPC over stdio for the external
// devspace-tui client. It wraps the same dashboard command closures the
// Bubble Tea dashboard uses, so both frontends share one domain path.
const uiProtocolVersion = 1

type uiServerRequest struct {
	ID     int64           `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params,omitempty"`
}

type uiServerError struct {
	Message string `json:"message"`
}

type uiServerResponse struct {
	ID     int64          `json:"id"`
	Result any            `json:"result,omitempty"`
	Error  *uiServerError `json:"error,omitempty"`
}

type uiServerEvent struct {
	Method string `json:"method"`
	Params any    `json:"params"`
}

type uiHello struct {
	Protocol      int    `json:"protocol"`
	Version       string `json:"version,omitempty"`
	WorkspaceRoot string `json:"workspaceRoot"`
	MachineID     string `json:"machineId"`
	MachineName   string `json:"machineName"`
	SyncMode      string `json:"syncMode"`
	Watch         bool   `json:"watch"`
}

type uiProjectRow struct {
	Ref    string `json:"ref"`
	Name   string `json:"name"`
	Path   string `json:"path"`
	Type   string `json:"type"`
	Status string `json:"status"`
	Dirty  bool   `json:"dirty"`
	Branch string `json:"branch,omitempty"`
	Env    bool   `json:"env"`
}

type uiScanSummary struct {
	FoundProjects     int `json:"foundProjects"`
	GitRepos          int `json:"gitRepos"`
	UntrackedFolders  int `json:"untrackedFolders"`
	LocalOnlyProjects int `json:"localOnlyProjects"`
	ProjectsWithEnv   int `json:"projectsWithEnv"`
}

type uiSyncStatus struct {
	Configured         bool   `json:"configured"`
	LastSyncAt         string `json:"lastSyncAt,omitempty"`
	LocalDiffers       bool   `json:"localDiffers"`
	DiffAdded          int    `json:"diffAdded"`
	DiffRemoved        int    `json:"diffRemoved"`
	DiffChanged        int    `json:"diffChanged"`
	ReconcileSaved     bool   `json:"reconcileSaved"`
	ConflictCount      int    `json:"conflictCount"`
	GitDiffUnavailable string `json:"gitDiffUnavailable,omitempty"`
	UnavailableReason  string `json:"unavailableReason,omitempty"`
}

type uiWatchRefresh struct {
	FullScan         bool   `json:"fullScan"`
	RefreshStartedAt string `json:"refreshStartedAt,omitempty"`
	WatchedDirCount  int    `json:"watchedDirCount"`
	SyncChanged      bool   `json:"syncChanged"`
	SyncMode         string `json:"syncMode,omitempty"`
}

type uiSnapshot struct {
	Rows    []uiProjectRow `json:"rows"`
	Summary uiScanSummary  `json:"summary"`
	Plan    *Plan          `json:"plan,omitempty"`
	Project *Project       `json:"project,omitempty"`
}

type uiServerOptions struct {
	Version  string
	NoWatch  bool
	SyncMode string

	watchCmdFactory func(string) tea.Cmd // test seam, same as dashboardModel
}

type uiServer struct {
	opts uiServerOptions

	mu  sync.Mutex // guards enc: responses and watch events interleave
	enc *json.Encoder
}

func runUIServer(r io.Reader, w io.Writer, opts uiServerOptions) error {
	if opts.SyncMode == "" {
		opts.SyncMode = WatchSyncOff
	}
	if opts.watchCmdFactory == nil {
		opts.watchCmdFactory = dashboardWatchCmd
	}
	srv := &uiServer{opts: opts, enc: json.NewEncoder(w)}
	if !opts.NoWatch {
		go srv.watchLoop()
	}
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1<<20)
	// ponytail: requests are handled sequentially — that IS the single-flight
	// busy guard the dashboard implements with startAction.
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var req uiServerRequest
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			srv.write(uiServerResponse{Error: &uiServerError{Message: "malformed request: " + err.Error()}})
			continue
		}
		result, err := srv.handle(req)
		resp := uiServerResponse{ID: req.ID}
		if err != nil {
			resp.Error = &uiServerError{Message: err.Error()}
		} else {
			resp.Result = result
		}
		srv.write(resp)
	}
	return scanner.Err()
}

func (s *uiServer) write(v any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	_ = s.enc.Encode(v)
}

func (s *uiServer) event(params any) {
	s.write(uiServerEvent{Method: "event", Params: params})
}

func (s *uiServer) handle(req uiServerRequest) (any, error) {
	switch req.Method {
	case "hello":
		return s.hello()
	case "projects":
		rows, summary, err := dashboardSnapshotFromState()
		if err != nil {
			return nil, err
		}
		return uiSnapshot{Rows: uiRows(rows), Summary: uiSummary(summary)}, nil
	case "scan":
		return snapshotFromMsg("scan", dashboardScanCmd()())
	case "refresh":
		return snapshotFromMsg("refresh", dashboardRefreshCmd(s.opts.SyncMode)())
	case "plan":
		return snapshotFromMsg("plan", dashboardPlanCmd()())
	case "apply":
		return snapshotFromMsg("apply-safe", dashboardApplyCmd()())
	case "hydrate":
		var params struct {
			Ref string `json:"ref"`
		}
		if len(req.Params) > 0 {
			if err := json.Unmarshal(req.Params, &params); err != nil {
				return nil, err
			}
		}
		if strings.TrimSpace(params.Ref) == "" {
			return nil, errors.New("hydrate requires params.ref")
		}
		return snapshotFromMsg("hydrate", dashboardHydrateCmd(params.Ref)())
	case "status":
		msg, ok := dashboardSyncStatusCmd()().(syncStatusLoadedMsg)
		if !ok {
			return nil, errors.New("unexpected sync status result")
		}
		return uiSync(msg.status), nil
	case "lastPlan":
		cfg, err := LoadConfig()
		if err != nil {
			return nil, err
		}
		plan, err := LoadLastPlan(cfg.WorkspaceRoot)
		if err != nil {
			return nil, fmt.Errorf("no saved plan found; run plan first: %w", err)
		}
		return plan, nil
	default:
		return nil, fmt.Errorf("unknown method %q", req.Method)
	}
}

func (s *uiServer) hello() (any, error) {
	hello := uiHello{
		Protocol: uiProtocolVersion,
		Version:  s.opts.Version,
		SyncMode: s.opts.SyncMode,
		Watch:    !s.opts.NoWatch,
	}
	cfg, err := LoadConfig()
	if err != nil {
		return nil, err
	}
	hello.WorkspaceRoot = cfg.WorkspaceRoot
	hello.MachineID = cfg.MachineID
	hello.MachineName = cfg.MachineName
	return hello, nil
}

// watchLoop re-arms the same one-shot watch command the dashboard uses and
// pushes each refresh as an event. A watch error ends the loop (the client is
// told); persistent errors would otherwise re-arm in a tight loop.
func (s *uiServer) watchLoop() {
	for {
		msg := s.opts.watchCmdFactory(s.opts.SyncMode)()
		if msg == nil {
			return
		}
		refresh, ok := msg.(watchRefreshMsg)
		if !ok {
			return
		}
		if refresh.err != nil {
			s.event(map[string]any{"type": "watch-error", "message": refresh.err.Error()})
			return
		}
		s.event(map[string]any{
			"type":    "watch-refresh",
			"rows":    uiRows(refresh.rows),
			"summary": uiSummary(refresh.summary),
			"refresh": uiWatch(refresh.refresh),
		})
	}
}

func snapshotFromMsg(label string, msg tea.Msg) (any, error) {
	switch msg := msg.(type) {
	case scanLoadedMsg:
		if msg.err != nil {
			return nil, msg.err
		}
		return uiSnapshot{Rows: uiRows(msg.rows), Summary: uiSummary(msg.summary)}, nil
	case actionResultMsg:
		if msg.err != nil {
			return nil, msg.err
		}
		snap := uiSnapshot{Rows: uiRows(msg.rows), Summary: uiSummary(msg.summary)}
		if len(msg.plan.Actions) > 0 || msg.plan.GeneratedAt != "" {
			plan := msg.plan
			snap.Plan = &plan
		}
		if msg.project.ID != "" {
			project := msg.project
			snap.Project = &project
		}
		return snap, nil
	default:
		return nil, fmt.Errorf("unexpected %s result", label)
	}
}

func uiRows(rows []dashboardRow) []uiProjectRow {
	out := make([]uiProjectRow, 0, len(rows))
	for _, row := range rows {
		out = append(out, uiProjectRow{
			Ref:    row.ref,
			Name:   row.name,
			Path:   row.path,
			Type:   row.typ,
			Status: row.status,
			Dirty:  row.dirty,
			Branch: row.branch,
			Env:    row.env,
		})
	}
	return out
}

func uiSummary(s ScanSummary) uiScanSummary {
	return uiScanSummary(s)
}

func uiSync(s dashboardSyncStatus) uiSyncStatus {
	return uiSyncStatus(s)
}

func uiWatch(r WatchRefresh) uiWatchRefresh {
	return uiWatchRefresh{
		FullScan:         r.FullScan,
		RefreshStartedAt: r.RefreshStartedAt,
		WatchedDirCount:  r.WatchedDirCount,
		SyncChanged:      r.SyncChanged,
		SyncMode:         r.SyncMode,
	}
}

func newUIServerCommand(version string) *cobra.Command {
	var noWatch bool
	var syncMode string
	cmd := &cobra.Command{
		Use:    "ui-server",
		Short:  "Serve the devspace-tui JSON-RPC protocol over stdio",
		Hidden: true,
		Args:   cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUIServer(cmd.InOrStdin(), cmd.OutOrStdout(), uiServerOptions{
				Version:  version,
				NoWatch:  noWatch,
				SyncMode: syncMode,
			})
		},
	}
	cmd.Flags().BoolVar(&noWatch, "no-watch", false, "disable the live filesystem watcher")
	cmd.Flags().StringVar(&syncMode, "sync", WatchSyncOff, "manifest sync mode for watch refreshes (off|git|hosted)")
	return cmd
}
