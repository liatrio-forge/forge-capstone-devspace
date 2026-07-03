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

func MountWorkspace(ctx context.Context, mountpoint string, opts WorkspaceMountOptions, out io.Writer) error {
	if out == nil {
		out = io.Discard
	}
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
		return fmt.Errorf("FUSE mount failed at %s: %w\n\nFallback: run `devspace mount %s --preview` to inspect tracked mount entries without requiring FUSE", mountpoint, err, mountpoint)
	}
	fmt.Fprintf(out, "Mounted DevSpace workspace at %s\n", mountpoint)
	fmt.Fprintln(out, "Accessing an on-demand Git project through the mount triggers `devspace project hydrate` safety checks.")
	fmt.Fprintln(out, "Press Ctrl-C to unmount.")

	done := make(chan struct{})
	go func() {
		server.Wait()
		close(done)
	}()
	select {
	case <-ctx.Done():
		if err := server.Unmount(); err != nil {
			fmt.Fprintf(out, "warning: unmount failed: %v\n", err)
		}
		<-done
		return nil
	case <-done:
		return nil
	}
}

func PrintMountPreview(out io.Writer, entries []MountEntry) {
	fmt.Fprintln(out, "DevSpace lazy mount preview")
	fmt.Fprintln(out, "FUSE library: github.com/hanwen/go-fuse/v2/fs")
	fmt.Fprintln(out)
	if len(entries) == 0 {
		fmt.Fprintln(out, "(no tracked projects)")
		return
	}
	for _, entry := range entries {
		fmt.Fprintf(out, "%s\t%s\t%s\t%s", entry.Path, entry.Type, entry.HydrateMode, entry.Status)
		if entry.Reason != "" {
			fmt.Fprintf(out, "\t%s", entry.Reason)
		}
		fmt.Fprintln(out)
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
	entry := MountEntry{
		Name:        p.Name,
		Path:        p.Path,
		Type:        p.Type,
		HydrateMode: p.HydrateMode,
		Remote:      p.Remote,
	}
	info := gitInfo(full)
	switch {
	case info.IsRepo:
		entry.Status = "hydrated"
	case exists(full) && p.Type == ProjectTypeGit && isEmptyDir(full):
		entry.Status = "placeholder"
		if p.Remote != "" && p.HydrateMode == HydrateOnDemand {
			entry.Reason = "will hydrate on project lookup"
		}
	case exists(full):
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
	full, _, err := safeWorkspacePath(n.workspace, p.Path)
	if err != nil {
		fmt.Fprintf(n.hydrationFailures, "devspace mount: %s\n", err)
		return nil, syscall.EIO
	}
	if n.shouldHydrate(full, p) {
		if _, err := HydrateProject(p.ID); err != nil {
			fmt.Fprintf(n.hydrationFailures, "devspace mount: hydrate %s failed: %s\n", p.Path, err)
			return nil, syscall.EIO
		}
	}
	if stat, err := os.Stat(full); err == nil && stat.IsDir() {
		loopbackRoot, err := fs.NewLoopbackRoot(full)
		if err != nil {
			fmt.Fprintf(n.hydrationFailures, "devspace mount: loopback %s failed: %s\n", p.Path, err)
			return nil, syscall.EIO
		}
		return n.NewInode(ctx, loopbackRoot, fs.StableAttr{Mode: syscall.S_IFDIR}), fs.OK
	}
	status, err := mountEntryForProject(n.workspace, p)
	if err != nil {
		fmt.Fprintf(n.hydrationFailures, "devspace mount: %s\n", err)
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
	if n.project.Remote != "" {
		fmt.Fprintf(&b, "remote: %s\n", n.project.Remote)
	}
	if n.entry.Reason != "" {
		fmt.Fprintf(&b, "reason: %s\n", n.entry.Reason)
	}
	return b.String()
}
