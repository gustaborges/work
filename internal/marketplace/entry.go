package marketplace

import (
	"errors"
	"fmt"
	"os"

	"github.com/gustaborges/work/internal/pluginid"
)

// ErrNoEntry marks a requested plugin that the fixed offer does not contain.
var ErrNoEntry = errors.New("the marketplace does not offer that plugin")

// ResolvedEntry is one offered plugin bound to the exact source facts it was
// resolved from: the registration instance, fixed revision and catalog it came
// from, and the validated entry. It is the only input to package publication,
// so no installation can name a path, URL or manifest of its own.
//
// Its content is the fixed view of the offer, never the live source: a source
// that moved or vanished after browsing cannot change what is copied, and an
// unavailable fixed revision fails instead of selecting the branch tip.
type ResolvedEntry struct {
	Provider       pluginid.Provider
	RegistrationID string
	Source         Source
	Revision       string
	CatalogDigest  string
	Entry          Entry

	view   View
	closer func()
}

// Resolve binds the named entry of the snapshot. The returned value takes over
// the snapshot's scratch content, so callers close the ResolvedEntry, not the
// snapshot. registrationID is the source instance that owns the offer.
func (s *Snapshot) Resolve(registrationID, name string) (*ResolvedEntry, error) {
	e, ok := s.Offer.Entry(name)
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrNoEntry, name)
	}
	if s.view == nil {
		return nil, errors.New("snapshot has no fixed content")
	}
	r := &ResolvedEntry{
		Provider:       pluginid.Provider{Marketplace: s.Offer.Name, Plugin: e.Name},
		RegistrationID: registrationID, Source: s.Source, Revision: s.Revision,
		CatalogDigest: s.Offer.CatalogDigest, Entry: e, view: s.view, closer: s.closer,
	}
	s.closer = nil
	return r, nil
}

// Tree reads the selected package from the fixed view and proves it is exactly
// the content the offer promised.
func (r *ResolvedEntry) Tree() (*Tree, error) {
	t, err := r.view.PackageTree(r.Entry.Path)
	if err != nil {
		return nil, err
	}
	d, err := t.Digest()
	if err != nil {
		return nil, invalidf("package %q: %v", r.Entry.Name, err)
	}
	if d != r.Entry.PackageDigest {
		return nil, fmt.Errorf("%w: the fixed content of %q no longer matches its offer", ErrSourceUnavailable, r.Entry.Name)
	}
	return t, nil
}

// Close releases scratch content the entry owns. It is idempotent.
func (r *ResolvedEntry) Close() {
	if r != nil && r.closer != nil {
		r.closer()
		r.closer = nil
	}
}

// OpenCapture reads a published local capture as a fixed view. The directory
// is owned by the Work home and referenced by committed catalog state, so the
// returned snapshot never removes it.
func OpenCapture(dir, source string) (*Snapshot, error) {
	if info, err := os.Lstat(dir); err != nil || !info.IsDir() {
		return nil, fmt.Errorf("%w", ErrSourceUnavailable)
	}
	v := dirView{root: dir}
	offer, err := BuildOffer(v)
	if err != nil {
		return nil, err
	}
	return &Snapshot{
		Offer: offer, Source: Source{Kind: pluginid.KindLocal, Source: source},
		Revision: offer.CaptureDigest(), Dir: dir, view: v,
	}, nil
}

// MatchesStored reports whether the snapshot is exactly the offer a stored
// catalog recorded: the same namespace, revision and catalog digest.
func (s *Snapshot) MatchesStored(c Stored) bool {
	return s.Offer.Name == c.Namespace && s.Revision == c.Revision && s.Offer.CatalogDigest == c.CatalogDigest
}

// StoredEntry returns the entry a stored catalog recorded under name.
func (c Stored) StoredEntry(name string) (Entry, bool) {
	for _, e := range c.Entries {
		if e.Name == name {
			return e, true
		}
	}
	return Entry{}, false
}
