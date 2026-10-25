package adminstate

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/registry"
	"github.com/gustaborges/work/internal/workhome"
)

func TestAdminReadAbsentPure(t *testing.T) {
	h := workhome.At(filepath.Join(t.TempDir(), "absent"))
	for range 20 {
		s, err := Read(context.Background(), h)
		if err != nil {
			t.Fatal(err)
		}
		if s.Epoch != 0 || len(s.Registry.Packages) != 0 {
			t.Fatal(s)
		}
	}
	if _, err := os.Stat(h.Root()); !os.IsNotExist(err) {
		t.Fatalf("read created home: %v", err)
	}
}
func TestAdminReadControl(t *testing.T) {
	for _, data := range []string{`{"schema":1,"epoch":1,"status":"pending","transaction_id":"tx"}`, `{"schema":2}`, `garbage`, `{"schema":1,"epoch":0,"status":"stable","transaction_id":"../bad"}`} {
		t.Run(data, func(t *testing.T) {
			h := workhome.At(t.TempDir())
			if err := os.MkdirAll(h.StateDir(), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(h.AdministrationFile(), []byte(data), 0600); err != nil {
				t.Fatal(err)
			}
			_, err := Read(context.Background(), h)
			if diag.ExitCode(err) != 54 {
				t.Fatalf("%v", err)
			}
		})
	}
}
func TestAdminReadCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := Read(ctx, workhome.At(t.TempDir()))
	if diag.ExitCode(err) != 20 {
		t.Fatal(err)
	}
}

func TestAdminEpochDriftAndLoadError(t *testing.T) {
	h := workhome.At(t.TempDir())
	ctx := context.Background()
	calls := 0
	if err := os.MkdirAll(h.ConfigDir(), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(h.ConfigFile(), []byte("broken"), 0600); err != nil {
		t.Fatal(err)
	}
	s, err := read(ctx, h, func() {
		calls++
		if calls == 1 {
			if err := os.Remove(h.ConfigFile()); err != nil {
				t.Fatal(err)
			}
			if err := saveJSON(h.AdministrationFile(), Control{Schema: 1, Epoch: 2, Status: "stable"}); err != nil {
				t.Fatal(err)
			}
		}
	})
	if err != nil || calls != 2 || s.Epoch != 2 {
		t.Fatalf("intermediate error not retried: %v %d", err, calls)
	}
	calls = 0
	_, err = read(ctx, h, func() {
		calls++
		if err := saveJSON(h.AdministrationFile(), Control{Schema: 1, Epoch: uint64(2 + calls), Status: "stable"}); err != nil {
			t.Fatal(err)
		}
	})
	if diag.ExitCode(err) != 53 || calls != 3 {
		t.Fatalf("retry bound: %v %d", err, calls)
	}
}

func TestAdminLegacyProjectionAndLinkedUnknown(t *testing.T) {
	h := workhome.At(t.TempDir())
	if err := h.EnsureLayout(); err != nil {
		t.Fatal(err)
	}
	reg := &registry.Registry{}
	for _, origin := range []string{registry.OriginLocalPinned, registry.OriginLocalLinked} {
		alias := origin
		root := filepath.Join(h.PluginsDir(), alias, "source")
		if err := os.MkdirAll(root, 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "plugin.json"), []byte(`{"name":"fixture","version":"installed","components":[],"conventions":[]}`), 0600); err != nil {
			t.Fatal(err)
		}
		reg.UpsertPackage(registry.Package{Alias: alias, Origin: origin})
	}
	if err := registry.Save(h.RegistryFile(), reg); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(h.RegistryFile())
	s, err := Read(context.Background(), h)
	if err != nil {
		t.Fatal(err)
	}
	pinned, _ := s.Registry.PackageByAlias(registry.OriginLocalPinned)
	linked, _ := s.Registry.PackageByAlias(registry.OriginLocalLinked)
	if pinned.Version != "installed" || linked.Version != "" {
		t.Fatal("wrong legacy baseline")
	}
	after, _ := os.ReadFile(h.RegistryFile())
	if string(before) != string(after) {
		t.Fatal("read migrated registry")
	}
}
func TestAdminLegacySeedCompleteRegistration(t *testing.T) {
	h := workhome.At(t.TempDir())
	r := &registry.Registry{Components: []registry.Component{{Alias: ReferenceAlias, Name: "local-path-starter", Role: registry.RoleStarter, StarterLayer: registry.LayerFallback}, {Alias: ReferenceAlias, Name: "filesystem-repository-locator", Role: registry.RoleRepositoryLocator}}, Conventions: []registry.Convention{{Name: "freeform"}}}
	if err := registry.Save(h.RegistryFile(), r); err != nil {
		t.Fatal(err)
	}
	s, err := Read(context.Background(), h)
	if err != nil {
		t.Fatal(err)
	}
	p, ok := s.Registry.PackageByAlias(ReferenceAlias)
	if !ok || p.Origin != registry.OriginEmbedded || !s.Lifecycle.Initialized {
		t.Fatal("missing virtual seed ownership")
	}
	raw, _ := registry.Load(h.RegistryFile())
	if len(raw.Packages) != 0 {
		t.Fatal("pure read adopted seed")
	}
}
