package devspace

import (
	"fmt"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"
)

const (
	dashboardStatusHydrated    = "Hydrated"
	dashboardStatusPlaceholder = "Placeholder"
	dashboardStatusMissing     = "Missing"

	dashboardEventLimit = 6
)

type dashboardRow struct {
	ref    string
	name   string
	path   string
	typ    string
	status string
	dirty  bool
	branch string
	env    bool
}

type dashboardModel struct {
	rows          []dashboardRow
	summary       ScanSummary
	syncStatus    dashboardSyncStatus
	selected      int
	events        []string
	busy          bool
	errText       string
	noWatch       bool
	workspaceRoot string
	machineID     string
	machineName   string
	syncMode      string
	width         int
	height        int

	statusCache     *syncStatusCache
	watchCmdFactory func(string) tea.Cmd
}

type dashboardSyncStatus struct {
	Configured         bool
	LastSyncAt         string
	LocalDiffers       bool
	DiffAdded          int
	DiffRemoved        int
	DiffChanged        int
	ReconcileSaved     bool
	ConflictCount      int
	GitDiffUnavailable string
	UnavailableReason  string
}

const (
	syncStatusRemoteNotConfigured = "remote not configured"
	syncStatusLoading             = "loading"
	syncStatusHostedUnavailable   = "unavailable-for-hosted"
)

type scanLoadedMsg struct {
	rows    []dashboardRow
	summary ScanSummary
	err     error
}

type actionResultMsg struct {
	label   string
	rows    []dashboardRow
	summary ScanSummary
	plan    Plan
	project Project
	refresh WatchRefresh
	err     error
}

type watchRefreshMsg struct {
	refresh WatchRefresh
	rows    []dashboardRow
	summary ScanSummary
	err     error
}

type syncStatusLoadedMsg struct {
	status dashboardSyncStatus
}

type errMsg struct {
	err error
}

func newDashboardModel(noWatch bool) dashboardModel {
	model := dashboardModel{
		noWatch:         noWatch,
		syncMode:        WatchSyncOff,
		statusCache:     newSyncStatusCache(dashboardSyncStatusCmd()),
		watchCmdFactory: dashboardWatchCmd,
	}
	cfg, err := LoadConfig()
	if err == nil {
		model.workspaceRoot = cfg.WorkspaceRoot
		model.machineID = cfg.MachineID
		model.machineName = cfg.MachineName
		if cfg.ManifestRemote == "" && !hostedSyncConfigured(cfg) {
			model.syncStatus.UnavailableReason = syncStatusRemoteNotConfigured
		} else {
			model.syncStatus.Configured = true
			model.syncStatus.UnavailableReason = syncStatusLoading
		}
	}
	return model
}

func (m dashboardModel) Init() tea.Cmd {
	if m.noWatch {
		return tea.Batch(dashboardScanCmd(), m.syncStatusCmd())
	}
	return tea.Batch(dashboardScanCmd(), m.syncStatusCmd(), m.nextWatchCmd())
}

func (m dashboardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case scanLoadedMsg:
		m.busy = false
		if msg.err != nil {
			m.errText = msg.err.Error()
			return m, nil
		}
		m.errText = ""
		m.rows = msg.rows
		m.summary = msg.summary
		m.clampSelected()
		return m, m.syncStatusCmd()
	case actionResultMsg:
		m.busy = false
		if msg.err != nil {
			m.errText = msg.err.Error()
			return m, nil
		}
		m.errText = ""
		m.rows = msg.rows
		m.summary = msg.summary
		m.prependEvent(fmt.Sprintf("%s complete", msg.label))
		m.clampSelected()
		return m, m.syncStatusCmd()
	case watchRefreshMsg:
		m.busy = false
		if msg.err != nil {
			m.errText = msg.err.Error()
			if m.noWatch {
				return m, nil
			}
			return m, m.nextWatchCmd()
		}
		m.errText = ""
		m.rows = msg.rows
		m.summary = msg.summary
		m.prependEvent(formatWatchEvent(msg.refresh))
		m.clampSelected()
		if m.noWatch {
			return m, nil
		}
		return m, m.nextWatchCmd()
	case syncStatusLoadedMsg:
		m.syncStatus = msg.status
		return m, nil
	case errMsg:
		m.busy = false
		if msg.err != nil {
			m.errText = msg.err.Error()
		}
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "up", "k":
			if m.selected > 0 {
				m.selected--
			}
			return m, nil
		case "down", "j":
			if m.selected < len(m.rows)-1 {
				m.selected++
			}
			return m, nil
		case "r":
			m, cmd := m.startAction("refresh", dashboardRefreshCmd(m.syncMode))
			if cmd == nil {
				return m, nil
			}
			return m, tea.Batch(cmd, m.syncStatusCmd())
		case "s":
			return m.startAction("scan", dashboardScanCmd())
		case "p":
			return m.startAction("plan", dashboardPlanCmd())
		case "a":
			return m.startAction("apply-safe", dashboardApplyCmd())
		case "h":
			if len(m.rows) == 0 {
				return m, nil
			}
			return m.startAction("hydrate", dashboardHydrateCmd(m.rows[m.selected].ref))
		}
	}
	return m, nil
}

func (m dashboardModel) View() tea.View {
	var b strings.Builder
	header := fmt.Sprintf("DevSpace UI  workspace=%s  machine=%s  sync=%s", valueOrDash(m.workspaceRoot), valueOrDash(m.machineLabel()), valueOrDash(m.syncMode))
	b.WriteString(currentTheme.Header.Render(header))
	b.WriteString("\n\n")
	b.WriteString(m.renderTable())
	b.WriteString("\n")
	b.WriteString(m.renderSummary())
	b.WriteString("\n\n")
	b.WriteString(m.renderSyncStatus())
	b.WriteString("\n\n")
	b.WriteString(currentTheme.Header.Render("Events"))
	b.WriteString("\n")
	if len(m.events) == 0 {
		b.WriteString(currentTheme.Muted.Render("none"))
		b.WriteString("\n")
	} else {
		for _, event := range m.events {
			b.WriteString(event)
			b.WriteString("\n")
		}
	}
	if m.errText != "" {
		b.WriteString(currentTheme.Fail.Render("Error: " + m.errText))
		b.WriteString("\n")
	}
	if m.busy {
		b.WriteString(currentTheme.Warn.Render("busy"))
		b.WriteString("\n")
	}
	b.WriteString(currentTheme.Muted.Render("keys: up/down/j/k navigate | r refresh | s scan | p plan | a apply-safe | h hydrate | q quit"))
	view := tea.NewView(b.String())
	view.AltScreen = true
	return view
}

func (m dashboardModel) renderSyncStatus() string {
	var b strings.Builder
	b.WriteString(currentTheme.Header.Render("Sync Status"))
	b.WriteString("\n")
	status := m.syncStatus
	if status.UnavailableReason != "" {
		switch status.UnavailableReason {
		case syncStatusRemoteNotConfigured:
			b.WriteString(currentTheme.Muted.Render(syncStatusRemoteNotConfigured))
		case syncStatusLoading:
			b.WriteString(currentTheme.Muted.Render("loading..."))
		default:
			b.WriteString(currentTheme.Warn.Render("status unavailable: " + status.UnavailableReason))
		}
		return b.String()
	}
	fmt.Fprintf(&b, "Last sync/base: %s\n", valueOrDash(status.LastSyncAt))
	if status.GitDiffUnavailable != "" {
		fmt.Fprintf(&b, "Local differs from remote: %s\n", status.GitDiffUnavailable)
		fmt.Fprintf(&b, "Remote diff: %s\n", status.GitDiffUnavailable)
	} else {
		fmt.Fprintf(&b, "Local differs from remote: %s\n", yesNo(status.LocalDiffers))
		fmt.Fprintf(&b, "Remote diff: added=%d removed=%d changed=%d\n", status.DiffAdded, status.DiffRemoved, status.DiffChanged)
	}
	if status.ReconcileSaved {
		fmt.Fprintf(&b, "Reconcile conflicts: %d", status.ConflictCount)
	} else {
		b.WriteString("Reconcile conflicts: -")
	}
	return b.String()
}

func (m dashboardModel) startAction(label string, cmd tea.Cmd) (dashboardModel, tea.Cmd) {
	if m.busy {
		m.errText = "busy; wait for the current operation to finish"
		return m, nil
	}
	m.busy = true
	m.errText = ""
	if m.statusCache != nil {
		m.statusCache.invalidate()
	}
	return m, cmd
}

func (m dashboardModel) syncStatusCmd() tea.Cmd {
	if m.statusCache == nil {
		return dashboardSyncStatusCmd()
	}
	return m.statusCache.cmd()
}

func (m dashboardModel) nextWatchCmd() tea.Cmd {
	factory := m.watchCmdFactory
	if factory == nil {
		factory = dashboardWatchCmd
	}
	return factory(m.syncMode)
}

func (m *dashboardModel) clampSelected() {
	if len(m.rows) == 0 {
		m.selected = 0
		return
	}
	if m.selected < 0 {
		m.selected = 0
	}
	if m.selected >= len(m.rows) {
		m.selected = len(m.rows) - 1
	}
}

func (m *dashboardModel) prependEvent(event string) {
	if event == "" {
		return
	}
	m.events = append([]string{event}, m.events...)
	if len(m.events) > dashboardEventLimit {
		m.events = m.events[:dashboardEventLimit]
	}
}

func (m dashboardModel) machineLabel() string {
	if m.machineName != "" && m.machineID != "" {
		return m.machineName + " (" + m.machineID + ")"
	}
	if m.machineName != "" {
		return m.machineName
	}
	return m.machineID
}

func (m dashboardModel) renderTable() string {
	rows := make([][]string, 0, len(m.rows))
	for i, row := range m.rows {
		name := row.name
		if i == m.selected {
			name = "> " + name
		} else {
			name = "  " + name
		}
		rows = append(rows, []string{
			truncateCell(name, 32),
			row.typ,
			renderStatus(row.status),
			renderDirty(row.dirty),
			valueOrDash(row.branch),
			renderEnv(row.env),
		})
	}
	if len(rows) == 0 {
		rows = append(rows, []string{"(none)", "-", "-", "-", "-", "-"})
	}
	tbl := table.New().
		Headers("Project", "Type", "Status", "Dirty", "Branch", "Env").
		Rows(rows...).
		BorderStyle(currentTheme.Muted).
		StyleFunc(func(row, col int) lipgloss.Style {
			if row == table.HeaderRow {
				return currentTheme.Header.Padding(0, 1)
			}
			return lipgloss.NewStyle().Padding(0, 1)
		})
	return tbl.Render()
}

func (m dashboardModel) renderSummary() string {
	return fmt.Sprintf("Summary: found=%d git=%d untracked=%d local-only=%d env=%d",
		m.summary.FoundProjects,
		m.summary.GitRepos,
		m.summary.UntrackedFolders,
		m.summary.LocalOnlyProjects,
		m.summary.ProjectsWithEnv,
	)
}

func renderStatus(status string) string {
	switch status {
	case dashboardStatusHydrated:
		return currentTheme.badge(currentTheme.OK, status)
	case dashboardStatusPlaceholder:
		return currentTheme.badge(currentTheme.Warn, status)
	case dashboardStatusMissing:
		return currentTheme.badge(currentTheme.Fail, status)
	default:
		return status
	}
}

func renderDirty(dirty bool) string {
	if dirty {
		return currentTheme.badge(currentTheme.Warn, "Dirty")
	}
	return currentTheme.badge(currentTheme.OK, "Clean")
}

func renderEnv(env bool) string {
	if env {
		return currentTheme.badge(currentTheme.OK, "Present")
	}
	return currentTheme.badge(currentTheme.Warn, "Missing")
}

func valueOrDash(value string) string {
	if strings.TrimSpace(value) == "" {
		return "-"
	}
	return value
}

func yesNo(value bool) string {
	if value {
		return "yes"
	}
	return "no"
}

func truncateCell(value string, max int) string {
	runes := []rune(value)
	if max <= 0 || len(runes) <= max {
		return value
	}
	if max <= 3 {
		return string(runes[:max])
	}
	return string(runes[:max-3]) + "..."
}

func dashboardRowsFromState() ([]dashboardRow, error) {
	cfg, err := LoadConfig()
	if err != nil {
		return nil, err
	}
	m, err := LoadManifest(cfg.WorkspaceRoot)
	if err != nil {
		return nil, err
	}
	st, err := LoadState()
	if err != nil && !missing(err) {
		return nil, err
	}
	rows := make([]dashboardRow, 0, len(m.Projects))
	for _, p := range m.Projects {
		state := st.Projects[p.ID]
		rows = append(rows, dashboardRow{
			ref:    p.Path,
			name:   p.Name,
			path:   p.Path,
			typ:    p.Type,
			status: dashboardStatus(state),
			dirty:  state.Dirty,
			branch: state.CurrentBranch,
			env:    state.EnvFilePresent,
		})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].path == rows[j].path {
			return rows[i].name < rows[j].name
		}
		return rows[i].path < rows[j].path
	})
	return rows, nil
}

func dashboardStatus(state ProjectState) string {
	switch {
	case state.Missing || !state.Exists:
		return dashboardStatusMissing
	case state.Placeholder:
		return dashboardStatusPlaceholder
	case state.Hydrated:
		return dashboardStatusHydrated
	default:
		return dashboardStatusPlaceholder
	}
}

func summaryFromRows(rows []dashboardRow) ScanSummary {
	summary := ScanSummary{FoundProjects: len(rows)}
	for _, row := range rows {
		if row.typ == ProjectTypeGit {
			summary.GitRepos++
		}
		if row.typ == ProjectTypeLocal {
			summary.LocalOnlyProjects++
		}
		if row.env {
			summary.ProjectsWithEnv++
		}
	}
	return summary
}

func formatWatchEvent(refresh WatchRefresh) string {
	scope := "scoped"
	if refresh.FullScan {
		scope = "full"
	}
	return fmt.Sprintf("%s refresh at %s: found=%d git=%d dirs=%d",
		scope,
		valueOrDash(refresh.RefreshStartedAt),
		refresh.Summary.FoundProjects,
		refresh.Summary.GitRepos,
		refresh.WatchedDirCount,
	)
}
