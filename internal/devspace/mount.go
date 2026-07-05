package devspace

import (
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"
	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
)

const mountStatusFile = ".devspace-status"

type WorkspaceMountOptions struct {
	HydrateOnLookup bool
	Debug           bool
	ErrOut          io.Writer
}

type MountEntry struct {
	Name        string `json:"name"`
	Path        string `json:"path"`
	Type        string `json:"type"`
	HydrateMode string `json:"hydrateMode"`
	Remote      string `json:"remote,omitempty"`
	Status      string `json:"status"`
	Reason      string `json:"reason,omitempty"`
	Dirty       bool   `json:"dirty"`
	EnvPresent  bool   `json:"envPresent"`
	SetupHint   string `json:"setupHint"`
}

func BuildMountEntries() ([]MountEntry, error) {
	cfg, err := LoadConfig()
	if err != nil {
		return nil, err
	}
	m, err := LoadManifest(cfg.WorkspaceRoot)
	if err != nil {
		return nil, err
	}
	entries := make([]MountEntry, 0, len(m.Projects))
	for _, p := range m.Projects {
		entry, err := mountEntryForProject(cfg.WorkspaceRoot, p)
		if err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Path < entries[j].Path
	})
	return entries, nil
}

func MountWorkspace(ctx context.Context, mountpoint string, opts WorkspaceMountOptions) error {
	if opts.ErrOut == nil {
		opts.ErrOut = io.Discard
	}
	mountpoint, err := expandPath(mountpoint)
	if err != nil {
		return err
	}
	if err := ensureMountpointReady(mountpoint); err != nil {
		return err
	}
	cfg, err := LoadConfig()
	if err != nil {
		return err
	}
	m, err := LoadManifest(cfg.WorkspaceRoot)
	if err != nil {
		return err
	}

	sec := time.Second
	root := &workspaceMountNode{
		workspace:         cfg.WorkspaceRoot,
		projects:          m.Projects,
		hydrateOnLookup:   opts.HydrateOnLookup,
		hydrationFailures: opts.ErrOut,
	}
	server, err := fs.Mount(mountpoint, root, &fs.Options{
		AttrTimeout:  &sec,
		EntryTimeout: &sec,
		MountOptions: fuse.MountOptions{
			Debug:  opts.Debug,
			FsName: cfg.WorkspaceRoot,
			Name:   "devspace",
			Options: []string{
				"ro",
			},
		},
	})
	if err != nil {
		return fmt.Errorf("FUSE mount failed at %s: %w\n\nFallback: run `devspace mount %s --preview` to inspect tracked mount entries without requiring FUSE\n\n%s", mountpoint, err, mountpoint, staleMountGuidance(mountpoint))
	}
	logger := newDiagnosticsLogger(opts.ErrOut)
	logger.Info("mounted workspace", "mountpoint", mountpoint)
	logger.Info("accessing an on-demand Git project through the mount triggers `devspace project hydrate` safety checks")
	logger.Info("press ctrl-c to unmount")

	done := make(chan struct{})
	go func() {
		server.Wait()
		close(done)
	}()
	select {
	case <-ctx.Done():
		if err := server.Unmount(); err != nil {
			logger.Warn("unmount failed", "error", err, "guidance", staleMountGuidance(mountpoint))
		}
		<-done
		return nil
	case <-done:
		return nil
	}
}

func PrintMountPreview(out io.Writer, entries []MountEntry) {
	out = styledWriter(out)
	fmt.Fprintln(out, currentTheme.Header.Render("DevSpace lazy mount preview"))
	fmt.Fprintln(out, currentTheme.Muted.Render("FUSE library: github.com/hanwen/go-fuse/v2/fs"))
	fmt.Fprintln(out)
	if len(entries) == 0 {
		fmt.Fprintln(out, "(no tracked projects)")
		return
	}
	rows := make([][]string, 0, len(entries))
	for _, entry := range entries {
		rows = append(rows, []string{entry.Path, entry.Type, entry.HydrateMode, entry.Status, fmt.Sprint(entry.Dirty), fmt.Sprint(entry.EnvPresent), entry.Reason})
	}
	tbl := table.New().
		Headers("PATH", "TYPE", "HYDRATE MODE", "STATUS", "DIRTY", "ENV", "REASON").
		Rows(rows...).
		BorderStyle(currentTheme.Muted).
		StyleFunc(func(row, col int) lipgloss.Style {
			if row == table.HeaderRow {
				return currentTheme.Header.Padding(0, 1)
			}
			if col == 3 {
				return mountStatusStyle(rows[row][3]).Padding(0, 1)
			}
			return lipgloss.NewStyle().Padding(0, 1)
		})
	fmt.Fprintln(out, tbl.Render())
}

// mountStatusStyle colors a mount entry's status column: hydrated is good,
// placeholder/lazy are pending (something will happen on lookup), local is
// informational, and missing is a problem.
func mountStatusStyle(status string) lipgloss.Style {
	switch status {
	case "hydrated":
		return currentTheme.OK
	case "placeholder", "lazy":
		return currentTheme.Warn
	case "missing":
		return currentTheme.Fail
	default: // "local"
		return currentTheme.Info
	}
}

func ensureMountpointReady(mountpoint string) error {
	if err := os.MkdirAll(mountpoint, 0o750); err != nil {
		return err
	}
	stat, err := os.Stat(mountpoint)
	if err != nil {
		return err
	}
	if !stat.IsDir() {
		return fmt.Errorf("mountpoint is not a directory: %s", mountpoint)
	}
	if !isEmptyDir(mountpoint) {
		return fmt.Errorf("mountpoint is non-empty; refusing to hide local files: %s", mountpoint)
	}
	return nil
}

func mountEntryForProject(workspace string, p Project) (MountEntry, error) {
	full, _, err := safeWorkspacePath(workspace, p.Path)
	if err != nil {
		return MountEntry{}, err
	}
	info := gitInfo(full)
	state := stateForProject(full, p, info)
	entry := MountEntry{
		Name:        p.Name,
		Path:        p.Path,
		Type:        p.Type,
		HydrateMode: p.HydrateMode,
		Remote:      p.Remote,
		Dirty:       state.Dirty,
		EnvPresent:  state.EnvFilePresent,
		SetupHint:   setupHint(p.Setup),
	}
	switch {
	case info.IsRepo:
		entry.Status = "hydrated"
	case state.Placeholder:
		entry.Status = "placeholder"
		if p.Remote != "" && p.HydrateMode == HydrateOnDemand {
			entry.Reason = "will hydrate on project lookup"
		}
	case state.Exists:
		entry.Status = "local"
	case p.Type == ProjectTypeGit && p.Remote != "" && p.HydrateMode == HydrateOnDemand:
		entry.Status = "lazy"
		entry.Reason = "will hydrate on project lookup"
	default:
		entry.Status = "missing"
		entry.Reason = "no automatic mount hydration is configured"
	}
	return entry, nil
}

func setupHint(setup Setup) string {
	var parts []string
	if setup.InstallCommand != "" {
		parts = append(parts, "install: "+setup.InstallCommand)
	}
	if setup.DevCommand != "" {
		parts = append(parts, "dev: "+setup.DevCommand)
	}
	return strings.Join(parts, "; ")
}

func staleMountGuidance(mountpoint string) string {
	return fmt.Sprintf("Stale mount cleanup: check for a previous devspace mount holding %s, then run `umount %s` or on Linux `fusermount3 -u %s`.", mountpoint, mountpoint, mountpoint)
}

type workspaceMountNode struct {
	fs.Inode
	workspace         string
	projects          []Project
	prefix            string
	hydrateOnLookup   bool
	hydrationFailures io.Writer
}

func (n *workspaceMountNode) Readdir(ctx context.Context) (fs.DirStream, syscall.Errno) {
	children := n.children()
	entries := make([]fuse.DirEntry, 0, len(children))
	for _, child := range children {
		entries = append(entries, fuse.DirEntry{
			Name: child.name,
			Mode: syscall.S_IFDIR,
		})
	}
	return fs.NewListDirStream(entries), fs.OK
}

func (n *workspaceMountNode) Lookup(ctx context.Context, name string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	for _, child := range n.children() {
		if child.name != name {
			continue
		}
		if child.project != nil {
			return n.projectInode(ctx, *child.project)
		}
		node := &workspaceMountNode{
			workspace:         n.workspace,
			projects:          n.projects,
			prefix:            child.path,
			hydrateOnLookup:   n.hydrateOnLookup,
			hydrationFailures: n.hydrationFailures,
		}
		return n.NewInode(ctx, node, fs.StableAttr{Mode: syscall.S_IFDIR}), fs.OK
	}
	return nil, syscall.ENOENT
}

func (n *workspaceMountNode) children() []mountChild {
	seen := map[string]mountChild{}
	for i := range n.projects {
		p := &n.projects[i]
		rel := filepath.ToSlash(filepath.Clean(p.Path))
		if n.prefix != "" {
			if rel != n.prefix && !strings.HasPrefix(rel, n.prefix+"/") {
				continue
			}
			rel = strings.TrimPrefix(rel, n.prefix+"/")
			if rel == n.prefix {
				rel = ""
			}
		}
		if rel == "" {
			continue
		}
		parts := strings.Split(rel, "/")
		childPath := parts[0]
		if n.prefix != "" {
			childPath = path.Join(n.prefix, parts[0])
		}
		child := mountChild{name: parts[0], path: childPath}
		if len(parts) == 1 {
			child.project = p
		}
		seen[child.name] = child
	}
	children := make([]mountChild, 0, len(seen))
	for _, child := range seen {
		children = append(children, child)
	}
	sort.Slice(children, func(i, j int) bool {
		return children[i].name < children[j].name
	})
	return children
}

func (n *workspaceMountNode) projectInode(ctx context.Context, p Project) (*fs.Inode, syscall.Errno) {
	logger := newDiagnosticsLogger(n.hydrationFailures)
	full, _, err := safeWorkspacePath(n.workspace, p.Path)
	if err != nil {
		logger.Warn("mount lookup failed", "path", p.Path, "error", err)
		return nil, syscall.EIO
	}
	if n.shouldHydrate(full, p) {
		if _, err := HydrateProject(p.ID); err != nil {
			logger.Warn(fmt.Sprintf("hydrate %s failed", p.Path), "error", err)
			return nil, syscall.EIO
		}
	}
	if stat, err := os.Stat(full); err == nil && stat.IsDir() {
		loopbackRoot, err := fs.NewLoopbackRoot(full)
		if err != nil {
			logger.Warn(fmt.Sprintf("loopback %s failed", p.Path), "error", err)
			return nil, syscall.EIO
		}
		return n.NewInode(ctx, loopbackRoot, fs.StableAttr{Mode: syscall.S_IFDIR}), fs.OK
	}
	status, err := mountEntryForProject(n.workspace, p)
	if err != nil {
		logger.Warn("mount lookup failed", "path", p.Path, "error", err)
		return nil, syscall.EIO
	}
	stub := &projectStatusNode{project: p, entry: status}
	return n.NewInode(ctx, stub, fs.StableAttr{Mode: syscall.S_IFDIR}), fs.OK
}

func (n *workspaceMountNode) shouldHydrate(full string, p Project) bool {
	if !n.hydrateOnLookup || p.Type != ProjectTypeGit || p.Remote == "" || p.HydrateMode != HydrateOnDemand {
		return false
	}
	return !exists(full) || isEmptyDir(full)
}

type mountChild struct {
	name    string
	path    string
	project *Project
}

type projectStatusNode struct {
	fs.Inode
	project Project
	entry   MountEntry
}

func (n *projectStatusNode) Readdir(ctx context.Context) (fs.DirStream, syscall.Errno) {
	return fs.NewListDirStream([]fuse.DirEntry{{
		Name: mountStatusFile,
		Mode: syscall.S_IFREG,
	}}), fs.OK
}

func (n *projectStatusNode) Lookup(ctx context.Context, name string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	if name != mountStatusFile {
		return nil, syscall.ENOENT
	}
	data := []byte(n.statusText())
	file := &fs.MemRegularFile{
		Data: data,
		Attr: fuse.Attr{Mode: syscall.S_IFREG | 0o444, Size: uint64(len(data))},
	}
	return n.NewInode(ctx, file, fs.StableAttr{Mode: syscall.S_IFREG}), fs.OK
}

func (n *projectStatusNode) statusText() string {
	var b strings.Builder
	fmt.Fprintf(&b, "name: %s\n", n.project.Name)
	fmt.Fprintf(&b, "path: %s\n", n.project.Path)
	fmt.Fprintf(&b, "type: %s\n", n.project.Type)
	fmt.Fprintf(&b, "hydrateMode: %s\n", n.project.HydrateMode)
	fmt.Fprintf(&b, "status: %s\n", n.entry.Status)
	fmt.Fprintf(&b, "dirty: %t\n", n.entry.Dirty)
	fmt.Fprintf(&b, "envPresent: %t\n", n.entry.EnvPresent)
	fmt.Fprintf(&b, "setupHint: %s\n", n.entry.SetupHint)
	if n.project.Remote != "" {
		fmt.Fprintf(&b, "remote: %s\n", n.project.Remote)
	}
	if n.entry.Reason != "" {
		fmt.Fprintf(&b, "reason: %s\n", n.entry.Reason)
	}
	return b.String()
}
