package marketplace

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gustaborges/work/internal/gitx"
	"github.com/gustaborges/work/internal/pluginid"
)

// ErrSourceUnavailable marks a source, branch, fixed revision or capture that
// cannot be reached (exit 59). Its text never carries transport detail.
var ErrSourceUnavailable = errors.New("marketplace source is unavailable")

// ErrChanged marks a local source that changed while it was being captured.
var ErrChanged = fmt.Errorf("%w: the source changed during capture", ErrSourceUnavailable)

// Source is the stored, normalized descriptor of where an offer is published.
type Source struct {
	Kind   string `json:"kind"`
	Source string `json:"source"`
	Branch string `json:"branch,omitempty"`
}

// Snapshot is one fixed, fully validated view of a source. Close releases any
// scratch content it owns.
type Snapshot struct {
	Offer    *Offer
	Source   Source
	Revision string // git: exact commit; local: capture digest; builtin: distribution capture digest
	// Dir holds the fixed content (catalog plus listed packages at their
	// paths) for local and Git views; empty for the in-memory builtin view.
	Dir string

	closer func()
}

// Close removes content owned by the snapshot. It is idempotent.
func (s *Snapshot) Close() {
	if s != nil && s.closer != nil {
		s.closer()
		s.closer = nil
	}
}

// Detach transfers ownership of Dir to the caller, which then publishes or
// removes it; Close no longer touches it.
func (s *Snapshot) Detach() { s.closer = nil }

// dirView reads a catalog and packages from a directory. exec, when non-nil,
// is the authoritative recorded executable bit set (Git tree modes).
type dirView struct {
	root string
	exec map[string]bool
}

func (v dirView) CatalogBytes() ([]byte, error) {
	p := filepath.Join(v.root, CatalogFile)
	info, err := os.Lstat(p)
	if err != nil {
		return nil, invalidf("%s is missing", CatalogFile)
	}
	if !info.Mode().IsRegular() {
		return nil, invalidf("%s must be a regular file", CatalogFile)
	}
	return os.ReadFile(p)
}

func (v dirView) PackageTree(entryPath string) (*Tree, error) {
	dir := v.root
	for _, seg := range strings.Split(entryPath, "/") {
		dir = filepath.Join(dir, seg)
		info, err := os.Lstat(dir)
		if err != nil {
			return nil, invalidf("package path %q does not exist", entryPath)
		}
		if !info.IsDir() {
			return nil, invalidf("package path %q must be a real directory without links", entryPath)
		}
	}
	t, err := ReadTree(dir)
	if err != nil {
		return nil, invalidf("package %q: %v", entryPath, err)
	}
	if v.exec != nil {
		for i, n := range t.Nodes {
			if n.Kind == KindFile {
				t.Nodes[i].Exec = v.exec[entryPath+"/"+n.Path]
			}
		}
	}
	return t, nil
}

// OpenGit fetches one exact commit of the recorded branch and validates its
// whole offer. An empty commit resolves the branch tip once; a non-empty
// commit is materialized exactly and never replaced by the tip. scratch is a
// parent directory for temporary content.
func OpenGit(ctx context.Context, source, branch, commit, scratch string) (*Snapshot, error) {
	if commit == "" {
		var err error
		if commit, err = gitx.ResolveBranch(ctx, source, branch); err != nil {
			return nil, sourceError(err)
		}
	}
	fetched, err := gitx.FetchCommit(ctx, source, branch, commit, scratch)
	if err != nil {
		return nil, sourceError(err)
	}
	defer fetched.Close()
	dir, err := os.MkdirTemp(scratch, "work-tree-*")
	if err != nil {
		return nil, err
	}
	fail := func(err error) (*Snapshot, error) {
		_ = os.RemoveAll(dir)
		return nil, err
	}
	exec, err := fetched.Materialize(ctx, dir)
	if err != nil {
		return fail(sourceError(err))
	}
	offer, err := BuildOffer(dirView{root: dir, exec: exec})
	if err != nil {
		return fail(err)
	}
	return &Snapshot{
		Offer: offer, Source: Source{Kind: pluginid.KindGit, Source: source, Branch: branch},
		Revision: commit, Dir: dir, closer: func() { _ = os.RemoveAll(dir) },
	}, nil
}

// sourceError projects retrieval failures onto safe, classified errors.
func sourceError(err error) error {
	switch {
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return err
	case errors.Is(err, gitx.ErrUnsafeContent):
		return invalidf("committed content is unsafe: submodules and special entries are not distributable")
	default:
		return fmt.Errorf("%w", ErrSourceUnavailable)
	}
}

// DiscoverBranch returns the remote's default branch for initial registration.
func DiscoverBranch(ctx context.Context, source string) (string, error) {
	b, err := gitx.DefaultBranch(ctx, source)
	if err != nil {
		return "", sourceError(err)
	}
	return b, nil
}
