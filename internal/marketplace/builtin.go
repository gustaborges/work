package marketplace

import (
	"fmt"

	"github.com/gustaborges/work/internal/pluginid"
	"github.com/gustaborges/work/seed"
)

// BuiltinSource is the descriptor recorded for the binary-owned provider.
var BuiltinSource = Source{Kind: pluginid.KindBuiltin, Source: pluginid.Builtin}

// builtinView projects the embedded distribution as an ordinary catalog view
// held entirely in memory, so reading it never touches disk or Work home.
type builtinView struct{}

func (builtinView) CatalogBytes() ([]byte, error) { return seed.MarketplaceJSON(), nil }

func (builtinView) PackageTree(entryPath string) (*Tree, error) {
	files, err := seed.Package(entryPath)
	if err != nil {
		return nil, invalidf("builtin package %q: %v", entryPath, err)
	}
	mem := map[string]MemFile{}
	for name, f := range files {
		mem[name] = MemFile{Content: f.Content, Exec: f.Exec}
	}
	return NewMemoryTree(mem), nil
}

// BuiltinView returns the code-owned catalog view for the host platform.
func BuiltinView() View { return builtinView{} }

// BuiltinSnapshot validates and projects the current embedded offer. The
// result is a pure function of the binary: no user registration can replace
// or shadow it, and the reserved namespace is asserted here.
func BuiltinSnapshot() (*Snapshot, error) {
	offer, err := BuildOffer(builtinView{})
	if err != nil {
		return nil, fmt.Errorf("builtin distribution is invalid: %w", err)
	}
	if offer.Name != pluginid.Builtin {
		return nil, fmt.Errorf("builtin distribution must publish the %q namespace", pluginid.Builtin)
	}
	return &Snapshot{Offer: offer, Source: BuiltinSource, Revision: offer.CaptureDigest(), view: builtinView{}}, nil
}
