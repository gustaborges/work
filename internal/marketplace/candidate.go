package marketplace

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/gustaborges/work/internal/pluginid"
)

// Candidate is one complete, temporary view of a source's current offer, shared
// by every installed plugin that was copied from that source and branch.
// Maintenance resolves the source once per batch so all targets of a group are
// judged against a single revision, and never publishes discovery: a candidate
// lives in OS temporary storage and disappears with its last user.
//
// Each entry resolved from a Candidate keeps the shared content alive until it
// is closed, so a frozen update can outlive the Candidate handle that produced
// it. Close on the handle and on every entry are all required and idempotent.
type Candidate struct {
	snap *Snapshot

	mu           sync.Mutex
	refs         int
	handleClosed bool
	closer       func()
}

// OpenCandidate resolves the current offer of src without reading or writing
// Work-home state. A Git source resolves its recorded branch tip once; a local
// source is captured coherently into scratch; the builtin source is the
// binary's own offer. scratch must be an OS temporary directory.
func OpenCandidate(ctx context.Context, src Source, scratch string) (*Candidate, error) {
	switch src.Kind {
	case pluginid.KindBuiltin:
		snap, err := BuiltinSnapshot()
		if err != nil {
			return nil, err
		}
		return newCandidate(snap), nil
	case pluginid.KindGit:
		snap, err := OpenGit(ctx, src.Source, src.Branch, "", scratch)
		if err != nil {
			return nil, err
		}
		return newCandidate(snap), nil
	case pluginid.KindLocal:
		dir, err := os.MkdirTemp(scratch, "work-candidate-*")
		if err != nil {
			return nil, err
		}
		snap, err := CaptureLocal(ctx, src.Source, filepath.Join(dir, "capture"), CaptureOptions{})
		if err != nil {
			_ = os.RemoveAll(dir)
			return nil, err
		}
		snap.closer = func() { _ = os.RemoveAll(dir) }
		return newCandidate(snap), nil
	}
	return nil, fmt.Errorf("%w: unsupported source kind %q", ErrSourceUnavailable, src.Kind)
}

func newCandidate(s *Snapshot) *Candidate {
	c := &Candidate{snap: s, refs: 1, closer: s.closer}
	s.closer = nil
	return c
}

// Offer is the validated catalog of the candidate.
func (c *Candidate) Offer() *Offer { return c.snap.Offer }

// Revision is the exact commit (Git), capture digest (local) or distribution
// digest (builtin) every entry of this candidate was resolved at.
func (c *Candidate) Revision() string { return c.snap.Revision }

// Source is the descriptor the candidate was obtained from.
func (c *Candidate) Source() Source { return c.snap.Source }

// Resolve binds the named entry to this candidate's fixed content. The entry
// holds its own reference to the shared content and must be closed.
func (c *Candidate) Resolve(registrationID, name string) (*ResolvedEntry, error) {
	e, ok := c.snap.Offer.Entry(name)
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrNoEntry, name)
	}
	if c.snap.view == nil {
		return nil, fmt.Errorf("%w: the candidate has no fixed content", ErrSourceUnavailable)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.refs == 0 {
		return nil, fmt.Errorf("%w: the candidate was closed", ErrSourceUnavailable)
	}
	c.refs++
	return &ResolvedEntry{
		Provider:       pluginid.Provider{Marketplace: c.snap.Offer.Name, Plugin: e.Name},
		RegistrationID: registrationID, Source: c.snap.Source, Revision: c.snap.Revision,
		CatalogDigest: c.snap.Offer.CatalogDigest, Entry: e, view: c.snap.view, closer: c.release,
	}, nil
}

// Close releases the handle's reference. Content disappears once every
// resolved entry has also been closed. It is idempotent.
func (c *Candidate) Close() {
	if c == nil {
		return
	}
	c.mu.Lock()
	already := c.handleClosed
	c.handleClosed = true
	c.mu.Unlock()
	if !already {
		c.release()
	}
}

func (c *Candidate) release() {
	c.mu.Lock()
	c.refs--
	last := c.refs == 0
	closer := c.closer
	if last {
		c.closer = nil
	}
	c.mu.Unlock()
	if last && closer != nil {
		closer()
	}
}
