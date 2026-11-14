package marketplace

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/gustaborges/work/internal/pluginid"
)

// CaptureOptions carries test seams for deterministic interleaving.
type CaptureOptions struct {
	// afterCopy runs after content is copied and before the source is
	// re-enumerated, letting tests mutate the source at the exact boundary.
	afterCopy func()
}

// CaptureLocal reads a live local directory (including uncommitted content)
// coherently: it validates and copies the catalog and every listed package
// into stage, then re-enumerates the source and compares content and
// inventories. Any observed difference fails instead of publishing mixed
// content. stage must not exist or be empty and is owned by the caller; on
// error it is emptied.
func CaptureLocal(ctx context.Context, src, stage string, opts CaptureOptions) (*Snapshot, error) {
	root, err := filepath.Abs(src)
	if err != nil {
		return nil, fmt.Errorf("%w", ErrSourceUnavailable)
	}
	if info, err := os.Stat(root); err != nil || !info.IsDir() {
		return nil, fmt.Errorf("%w", ErrSourceUnavailable)
	}
	fail := func(err error) (*Snapshot, error) {
		_ = os.RemoveAll(stage)
		return nil, err
	}
	view := dirView{root: root}
	data, err := view.CatalogBytes()
	if err != nil {
		return fail(err)
	}
	cat, err := ParseCatalog(data)
	if err != nil {
		return fail(err)
	}
	if err := os.MkdirAll(stage, 0o755); err != nil {
		return fail(err)
	}
	if err := os.WriteFile(filepath.Join(stage, CatalogFile), data, 0o644); err != nil {
		return fail(err)
	}
	for _, l := range cat.Plugins {
		if err := ctx.Err(); err != nil {
			return fail(err)
		}
		tree, err := view.PackageTree(l.Path)
		if err != nil {
			return fail(err)
		}
		if err := tree.CopyTo(filepath.Join(stage, filepath.FromSlash(l.Path))); err != nil {
			return fail(invalidf("package %q cannot be captured: %v", l.Name, err))
		}
	}
	if opts.afterCopy != nil {
		opts.afterCopy()
	}
	again, err := view.CatalogBytes()
	if err != nil || string(again) != string(data) {
		return fail(ErrChanged)
	}
	for _, l := range cat.Plugins {
		now, err := view.PackageTree(l.Path)
		if err != nil {
			return fail(ErrChanged)
		}
		captured, err := dirView{root: stage}.PackageTree(l.Path)
		if err != nil {
			return fail(err)
		}
		same, err := captured.Equal(now)
		if err != nil || !same {
			return fail(ErrChanged)
		}
	}
	offer, err := BuildOffer(dirView{root: stage})
	if err != nil {
		return fail(err)
	}
	return &Snapshot{
		Offer: offer, Source: Source{Kind: pluginid.KindLocal, Source: root},
		Revision: offer.CaptureDigest(), Dir: stage, view: dirView{root: stage}, closer: func() { _ = os.RemoveAll(stage) },
	}, nil
}
