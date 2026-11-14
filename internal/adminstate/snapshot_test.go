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

func TestAdminReadRejectsPresentButInvalidState(t *testing.T) {
	cases := map[string]struct {
		file func(workhome.Home) string
		data string
	}{
		"unversioned config":    {func(h workhome.Home) string { return h.ConfigFile() }, `{"workspace":"x"}`},
		"unknown config field":  {func(h workhome.Home) string { return h.ConfigFile() }, `{"schema":1,"nope":1}`},
		"alias-shaped registry": {func(h workhome.Home) string { return h.RegistryFile() }, `{"components":[],"conventions":[],"packages":[{"alias":"p","origin":"local-linked"}]}`},
		"future registry":       {func(h workhome.Home) string { return h.RegistryFile() }, `{"schema":2}`},
		"bare-string memory":    {func(h workhome.Home) string { return h.BranchConventionsFile() }, `{"entries":[{"identity":"r","convention":"x"}]}`},
		"old lifecycle shape":   {func(h workhome.Home) string { return h.LifecycleFile() }, `{"schema":1,"initialized":true,"policy_seeded":true,"removed":false}`},
		"removed uninitialized": {func(h workhome.Home) string { return h.LifecycleFile() }, `{"schema":1,"initialized":false,"policy_handled":false,"removed":true}`},
		"future catalogs":       {func(h workhome.Home) string { return h.CatalogsFile() }, `{"schema":2,"catalogs":[]}`},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			h := workhome.At(t.TempDir())
			path := c.file(h)
			if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(c.data), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := Read(context.Background(), h); err == nil || diag.ExitCode(err) == 0 {
				t.Fatalf("invalid state was accepted: %v", err)
			}
			got, _ := os.ReadFile(path)
			if string(got) != c.data {
				t.Fatal("a read rewrote or repaired the file")
			}
			// A mutation preflight must not convert or delete it either.
			_ = WithWrite(context.Background(), h, "work plugin enable a@m", func(s *Session) error { return nil })
			if got, _ = os.ReadFile(path); string(got) != c.data {
				t.Fatal("a mutation converted or deleted unsupported state")
			}
		})
	}
}

func TestAdminReadReportsInstalledContentStatus(t *testing.T) {
	h := workhome.At(t.TempDir())
	p := prov("a", "m")
	reg := registry.New()
	reg.UpsertPackage(testPackage(p))
	reg.UpsertComponent(registry.Component{Provider: p, Name: "run", Role: "linker", Entrypoint: "bin/run", Runtime: "sh"})
	if err := registry.Save(h.RegistryFile(), reg); err != nil {
		t.Fatal(err)
	}
	s, err := Read(context.Background(), h)
	if err != nil || s.Inspection[p.String()] != "content-unavailable" {
		t.Fatalf("missing content = %v, %v", s.Inspection, err)
	}
	src, _ := h.PackageSource(p)
	if err := os.MkdirAll(filepath.Join(src, "bin"), 0o700); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"plugin.json", "bin/run"} {
		if err := os.WriteFile(filepath.Join(src, f), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if s, _ = Read(context.Background(), h); s.Inspection[p.String()] != "ready" {
		t.Fatalf("present content = %v", s.Inspection)
	}
	// Reads never repair: the registry record is untouched and no state appeared.
	if _, err := os.Stat(h.LifecycleFile()); !os.IsNotExist(err) {
		t.Error("a read created lifecycle state")
	}
}
