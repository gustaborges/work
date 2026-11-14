package pluginadmin

import (
	"errors"
	"testing"

	"github.com/gustaborges/work/internal/adminstate"
	"github.com/gustaborges/work/internal/config"
	"github.com/gustaborges/work/internal/marketplace"
	"github.com/gustaborges/work/internal/pluginid"
	"github.com/gustaborges/work/internal/registry"
)

func availableFixture() *adminstate.Snapshot {
	const id = "0123456789abcdef0123456789abcdef"
	src := marketplace.Source{Kind: pluginid.KindLocal, Source: "/x"}
	cat := marketplace.NewState()
	cat.Upsert(marketplace.Stored{
		RegistrationID: id, Namespace: "team", Source: src, Revision: "r1",
		Entries: []marketplace.Entry{{Name: "tool", Version: "1"}},
	})
	return &adminstate.Snapshot{
		Config:   &config.Config{Marketplaces: []config.Marketplace{{RegistrationID: id, Name: "team", Kind: src.Kind, Source: src.Source}}},
		Registry: registry.New(),
		Catalogs: cat,
	}
}

func TestAvailableReportsAnUnreadableBuiltinOffer(t *testing.T) {
	failing := func() (*marketplace.Snapshot, error) { return nil, errors.New("no assets for this platform") }
	rows, errs := available(availableFixture(), "", failing)

	if len(errs) != 1 || errs[0].Marketplace != pluginid.Builtin || errs[0].Error != "marketplace-entry-unavailable" {
		t.Fatalf("catalog errors = %+v, want one marketplace-entry-unavailable for builtin", errs)
	}
	if errs[0].Message == "" || errs[0].Message == "no assets for this platform" {
		t.Errorf("message = %q, want fixed text that does not echo the build error", errs[0].Message)
	}
	if len(rows) != 1 || rows[0].ID != "tool@team" {
		t.Errorf("rows = %+v, want only the registered source's offer", rows)
	}
}

func TestAvailableFilteredToAnotherSourceNeverConsultsBuiltin(t *testing.T) {
	called := false
	probe := func() (*marketplace.Snapshot, error) { called = true; return nil, errors.New("unused") }
	rows, errs := available(availableFixture(), "team", probe)
	if called || len(errs) != 0 || len(rows) != 1 {
		t.Errorf("called=%v rows=%d errs=%+v", called, len(rows), errs)
	}
}
